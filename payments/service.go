package payments

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/elorenzorodz/event-mrs/internal/convert"
	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/elorenzorodz/event-mrs/internal/mailer"
	"github.com/elorenzorodz/event-mrs/internal/sqlutil"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v83"
	"github.com/stripe/stripe-go/v83/paymentintent"
	"github.com/stripe/stripe-go/v83/refund"
	"github.com/stripe/stripe-go/v83/webhook"
)

var (
	ErrNotFound = errors.New("payment not found")
)

func NewService(dbQueries *database.Queries, stripeClient StripeClient, mMailer *mailer.Mailer, stripeSigningSecret string, stripeRefundSigningSecret string) PaymentService {
	return &Service{
		DB:                        dbQueries,
		Stripe:                    stripeClient,
		Mailer:                    mMailer,
		StripeSigningSecret:       stripeSigningSecret,
		StripeRefundSigningSecret: stripeRefundSigningSecret,
	}
}

func (stripeAPIClient *StripeAPIClient) UpdatePaymentIntent(paymentIntentID string, params *stripe.PaymentIntentParams) (*stripe.PaymentIntent, error) {
	return paymentintent.Update(paymentIntentID, params)
}

func (stripeAPIClient *StripeAPIClient) CreateRefund(params *stripe.RefundParams) (*stripe.Refund, error) {
	return refund.New(params)
}

func (stripeAPIClient *StripeAPIClient) ConstructEvent(payload []byte, signature string, secret string) (stripe.Event, error) {
	event, err := webhook.ConstructEvent(payload, signature, secret)

	if err != nil {
		return stripe.Event{}, fmt.Errorf("webhook signature verification failed: %w", err)
	}

	return event, nil
}

func DatabasePaymentToPaymentJSON(databasePayment database.Payment) *Payment {
	amount, _ := convert.StringToFloat32(databasePayment.Amount)

	return &Payment{
		ID:              databasePayment.ID,
		PaymentIntentID: databasePayment.PaymentIntentID.String,
		Amount:          float64(amount),
		Currency:        databasePayment.Currency,
		Status:          databasePayment.Status,
		ExpiresAt:       databasePayment.ExpiresAt,
		CreatedAt:       databasePayment.CreatedAt,
		UpdatedAt:       sqlutil.NullTimeToString(databasePayment.UpdatedAt),
		UserID:          databasePayment.UserID,
	}
}

func DatabasePaymentsToPaymentsJSON(databasePayments []database.Payment) []*Payment {
	payments := make([]*Payment, len(databasePayments))

	for i, databasePayment := range databasePayments {
		payments[i] = DatabasePaymentToPaymentJSON(databasePayment)
	}

	return payments
}

func (service *Service) GetUserPayments(ctx context.Context, userID uuid.UUID) ([]*Payment, error) {
	dbPayments, err := service.DB.GetUserPayments(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []*Payment{}, nil
		}

		return nil, fmt.Errorf("failed to retrieve user payments: %w", err)
	}

	return DatabasePaymentsToPaymentsJSON(dbPayments), nil
}

func (service *Service) GetUserPaymentById(ctx context.Context, paymentID, userID uuid.UUID) (*Payment, error) {
	dbPayment, err := service.DB.GetPaymentById(ctx, database.GetPaymentByIdParams{ID: paymentID, UserID: userID})

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("failed to retrieve payment: %w", err)
	}

	return DatabasePaymentToPaymentJSON(dbPayment), nil
}

func (service *Service) UpdatePayment(ctx context.Context, paymentID, userID uuid.UUID, paymentMethodID string) (*PaymentResponse, error) {
	// Get payment record.
	payment, err := service.DB.GetPaymentById(ctx, database.GetPaymentByIdParams{
		ID:     paymentID,
		UserID: userID,
	})

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("failed to retrieve payment: %w", err)
	}

	// Prepare PaymentIntent params.
	if !payment.PaymentIntentID.Valid {
		return nil, errors.New("payment record has no linked payment intent")
	}

	params := &stripe.PaymentIntentParams{
		PaymentMethod: stripe.String(paymentMethodID),
		Confirm:       stripe.Bool(true),
	}

	// Call Stripe API using injected client.
	paymentIntentResult, paymentIntentError := service.Stripe.UpdatePaymentIntent(payment.PaymentIntentID.String, params)

	paymentResponse := PaymentResponse{ID: paymentID, ExpiresAt: payment.ExpiresAt}

	// Handle Stripe result.
	if paymentIntentError != nil {
		if stripeErr, ok := paymentIntentError.(*stripe.Error); ok {
			paymentResponse.Status = *stripe.String(stripeErr.Code)
			paymentResponse.Message = *stripe.String(stripeErr.Msg)
		} else {
			paymentResponse.Status = "error"
			paymentResponse.Message = "an unknown error occurred with payment update"
		}
	} else {
		paymentResponse.Status = string(paymentIntentResult.Status)
		paymentResponse.Message = "payment updated"

		if paymentIntentResult.ClientSecret != "" {
			paymentResponse.ClientSecret = paymentIntentResult.ClientSecret
		}
		if paymentIntentResult.NextAction != nil {
			paymentResponse.NextAction = string(paymentIntentResult.NextAction.Type)
		}
	}

	// Update DB record.
	_, updateDbError := service.DB.UpdatePayment(ctx, database.UpdatePaymentParams{
		Amount:          payment.Amount,
		Status:          paymentResponse.Status,
		PaymentIntentID: payment.PaymentIntentID,
		ID:              paymentID,
		UserID:          userID,
	})

	if updateDbError != nil {
		log.Printf("CRITICAL: Failed to update payment status for %s: %v", paymentID, updateDbError)
	}

	return &paymentResponse, nil
}

func (service *Service) RefundPayment(ctx context.Context, paymentID, userID uuid.UUID) (*PaymentRefundResponse, error) {
	// 1. Retrieve payment and reservation details.
	paymentDetails, err := service.DB.GetPaymentAndReservationDetails(ctx, database.GetPaymentAndReservationDetailsParams{
		PaymentID: paymentID,
		UserID:    userID,
	})

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || len(paymentDetails) == 0 {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to retrieve payment details for refund: %w", err)
	}

	type ReservationForRefund struct {
		EventTitle        string
		TicketDescription string
		ShowDate          time.Time
		PaymentID         uuid.UUID
		ReservationID     uuid.UUID
		EventDetailID     uuid.UUID
		Amount            int64
	}

	var (
		reservationsToBeRefunded []ReservationForRefund
		mutex                    sync.Mutex
		filterWaitGroup          sync.WaitGroup 
		allErrors                []string      
		paymentIntentID          = paymentDetails[0].PaymentIntentID.String
		originalPaymentDetails   = paymentDetails[0]
	)

	for _, paymentAndReservationDetail := range paymentDetails {
		paymentReservationDetail := paymentAndReservationDetail

		if paymentIntentID == "" {
			mutex.Lock()
			allErrors = append(allErrors, "payment intent ID is missing")
			mutex.Unlock()

			continue
		}

		filterWaitGroup.Go(func() {
			if !paymentReservationDetail.ReservationID.Valid || !paymentReservationDetail.ShowDate.Valid || !paymentReservationDetail.Price.Valid {
				mutex.Lock()
				allErrors = append(allErrors, "incomplete reservation details found")
				mutex.Unlock()

				return
			}

			currentDateTime := time.Now()
			showDate := paymentReservationDetail.ShowDate.Time
			dateTimeDifference := showDate.Sub(currentDateTime)

			if dateTimeDifference < 0 {
				dateTimeDifference = -dateTimeDifference
			}

			// Allow refund for show dates with more than 48 hours difference.
			if dateTimeDifference.Hours() > 48 {
				// Convert price string (e.g., "15.00") to amount in cents (1500).
				amount, amountParseError := convert.PriceStringToCents(paymentReservationDetail.Price.String)

				if amountParseError != nil {
					mutex.Lock()
					allErrors = append(allErrors, fmt.Sprintf("error processing price for ticket %s, for event: %s: %s", 
						paymentReservationDetail.TicketDescription.String, paymentReservationDetail.Title.String, amountParseError))
					mutex.Unlock()

					return
				}

				reservation := ReservationForRefund{
					EventTitle:        paymentReservationDetail.Title.String,
					TicketDescription: paymentReservationDetail.TicketDescription.String,
					ShowDate:          showDate,
					PaymentID:         paymentReservationDetail.PaymentID,
					ReservationID:     paymentReservationDetail.ReservationID.UUID,
					EventDetailID:     paymentReservationDetail.EventDetailID.UUID,
					Amount:            amount,
				}

				mutex.Lock()
				reservationsToBeRefunded = append(reservationsToBeRefunded, reservation)
				mutex.Unlock()
			}
		})
	}

	filterWaitGroup.Wait()

	if len(allErrors) > 0 {
		return nil, fmt.Errorf("one or more errors occurred during reservation filtering:\n%s", strings.Join(allErrors, "\n"))
	}

	if len(reservationsToBeRefunded) == 0 {
		return nil, errors.New("no reservations were eligible for refund")
	}

	var (
		paymentRefundResponse PaymentRefundResponse
		refundMutex           sync.Mutex
		restoreWaitGroup      sync.WaitGroup
		totalRefundAmount     int64
		allRefundErrors       []string
	)
    
	for _, refundReservation := range reservationsToBeRefunded {
		reservationToBeRefunded := refundReservation

		restoreWaitGroup.Go(func() { 
			refundPaymentAndRestoreTicketsParams := database.RefundPaymentAndRestoreTicketsParams{
				EventDetailID: reservationToBeRefunded.EventDetailID,
				Amount:        fmt.Sprintf("%.2f", float64(reservationToBeRefunded.Amount)/100.0),
				PaymentID:     reservationToBeRefunded.PaymentID,
				UserID:        userID,
				ReservationID: reservationToBeRefunded.ReservationID,
			}

			refundPaymentAndRestoreTicketsError := service.DB.RefundPaymentAndRestoreTickets(ctx, refundPaymentAndRestoreTicketsParams)

			if refundPaymentAndRestoreTicketsError != nil {
				refundMutex.Lock()
				allRefundErrors = append(allRefundErrors, fmt.Sprintf("error updating payment refund and restoring tickets for %s - %s: %s", 
					reservationToBeRefunded.EventTitle, reservationToBeRefunded.TicketDescription, refundPaymentAndRestoreTicketsError))
				refundMutex.Unlock()

				return
			}

			paymentRefunded := PaymentRefunded {
				PaymentID:         reservationToBeRefunded.PaymentID,
				Amount:            fmt.Sprintf("%.2f", float64(reservationToBeRefunded.Amount)/100.0),
				Title:             reservationToBeRefunded.EventTitle,
				TicketDescription: reservationToBeRefunded.TicketDescription,
				ShowDate:          reservationToBeRefunded.ShowDate,
			}

			refundMutex.Lock()
			paymentRefundResponse.PaymentRefunds = append(paymentRefundResponse.PaymentRefunds, paymentRefunded)
			totalRefundAmount += reservationToBeRefunded.Amount
			refundMutex.Unlock()
		})
	}

	restoreWaitGroup.Wait()

	if len(allRefundErrors) > 0 {
		log.Printf("CRITICAL: Partial refund processing error. Check DB for inconsistencies. Errors:\n%s", strings.Join(allRefundErrors, "\n"))
	}

	if totalRefundAmount == 0 {
		return nil, errors.New("no refund amount calculated after processing reservations")
	}

	refundParams := &stripe.RefundParams {
		Amount: stripe.Int64(totalRefundAmount),
		PaymentIntent: stripe.String(paymentIntentID),
	}

	refundResult, stripeRefundError := service.Stripe.CreateRefund(refundParams)
	
	finalStatus := "refund pending" 
	finalMsg := "Refund initiated. Status is pending, confirmation will be sent via webhook."
	var returnError error = nil
	
	if stripeRefundError != nil {
		finalStatus = "refund_failed" 
		finalMsg = fmt.Sprintf("Refund initiation failed: %v", stripeRefundError)
		returnError = fmt.Errorf("failed to initiate Stripe refund: %w", stripeRefundError)
	} else if refundResult.Status == stripe.RefundStatusFailed {
        finalStatus = "refund_failed"
		finalMsg = fmt.Sprintf("Refund failed: %s", string(refundResult.FailureReason))
		returnError = errors.New("refund failed synchronously")
    } else if refundResult.Status == stripe.RefundStatusSucceeded {
        finalStatus = string(stripe.RefundStatusSucceeded)
		finalMsg = "Refund succeeded immediately."
    }
	
	_, updateDbError := service.DB.UpdatePayment(ctx, database.UpdatePaymentParams{
		Amount:          originalPaymentDetails.Amount,
		Status:          finalStatus, // Set to "refund pending", "refund_succeeded", or "refund_failed"
		PaymentIntentID: originalPaymentDetails.PaymentIntentID,
		ID:              originalPaymentDetails.PaymentID,
		UserID:          originalPaymentDetails.UserID,
	})

	if updateDbError != nil {
		log.Printf("CRITICAL: Failed to update payment status to %s for %s: %v", finalStatus, paymentID, updateDbError)
	}

	if len(allRefundErrors) > 0 {
		errorContext := strings.Join(allRefundErrors, "\n")
		if returnError == nil {
			returnError = fmt.Errorf("partial DB failure during refund processing:\n%s", errorContext)
		} else {
			returnError = fmt.Errorf("%w\nPartial DB failure during refund processing:\n%s", returnError, errorContext)
		}
	}

	if returnError != nil {
		log.Printf("Refund failure for Payment ID %s: %v", paymentID, returnError)

		return nil, returnError
	}

	response := &PaymentRefundResponse{
		PaymentRefunds: paymentRefundResponse.PaymentRefunds,
		Message: finalMsg,
	}

	return response, nil
}

func (service *Service) HandleWebhook(ctx context.Context, body []byte, signature string, webhookType string) error {
	signingSecret := service.StripeSigningSecret

	if strings.Contains(strings.ToLower(webhookType), "refund") {
		signingSecret = service.StripeRefundSigningSecret
	}

	event, err := service.Stripe.ConstructEvent(body, signature, signingSecret)

	if err != nil {
		return fmt.Errorf("error verifying Stripe signature: %w", err)
	}

	switch event.Type {
	case "payment_intent.succeeded":
		var paymentIntent stripe.PaymentIntent

		if err := json.Unmarshal(event.Data.Raw, &paymentIntent); err != nil {
			return fmt.Errorf("error unmarshaling payment_intent.succeeded: %w", err)
		}

		service.handlePaymentIntentSuccess(ctx, paymentIntent)

	case "payment_intent.payment_failed":
		var paymentIntent stripe.PaymentIntent

		if err := json.Unmarshal(event.Data.Raw, &paymentIntent); err != nil {
			return fmt.Errorf("error unmarshaling payment_intent.payment_failed: %w", err)
		}

		service.handlePaymentIntentFailure(ctx, paymentIntent)

	case "payment_intent.requires_action":
		var paymentIntent stripe.PaymentIntent

		if err := json.Unmarshal(event.Data.Raw, &paymentIntent); err != nil {
			return fmt.Errorf("error unmarshaling payment_intent.requires_action: %w", err)
		}

		service.handlePaymentIntentRequiresAction(ctx, paymentIntent)

	case "charge.refunded":
		var charge stripe.Charge
		if err := json.Unmarshal(event.Data.Raw, &charge); err != nil {
			return fmt.Errorf("error unmarshaling charge.refunded: %w", err)
		}

		service.handleChargeRefunded(ctx, charge)

	case "refund.failed":
		var refundEvent stripe.Refund

		if err := json.Unmarshal(event.Data.Raw, &refundEvent); err != nil {
			return fmt.Errorf("error unmarshaling refund.failed: %w", err)
		}

		service.handleRefundFailed(ctx, refundEvent)

	default:
		log.Printf("Webhook: Unhandled event type: %s", event.Type)
	}

	return nil
}

func (service *Service) handlePaymentIntentSuccess(ctx context.Context, intent stripe.PaymentIntent) error {
	payment, user, err := service.getPaymentAndUserFromIntent(ctx, intent)

	if err != nil {
		return err 
	}

	amountStr := fmt.Sprintf("%.2f", float64(intent.Amount)/100.0)

	updatedPayment, err := service.updatePaymentStatus(ctx, payment, string(stripe.PaymentIntentStatusSucceeded), intent.ID, amountStr)

	if err != nil {
		return err
	}

	// This retrieves the ticket/event details linked to the reservations for the email.
	userReservations, err := service.DB.GetUserReservationsByPaymentId(ctx, database.GetUserReservationsByPaymentIdParams{
		UserID: user.ID,
		PaymentID: updatedPayment.ID,
	})

	if err == nil && len(userReservations) > 0 {
		eventDetailIds := make([]uuid.UUID, len(userReservations))

		for i, ur := range userReservations {
			eventDetailIds[i] = ur.EventDetailID
		}

		eventDetails, err := service.DB.GetEventDetailsWithTitleByIds(ctx, eventDetailIds)
		if err == nil {
			fullName := fmt.Sprintf("%s %s", user.Firstname, user.Lastname)
			sendEmailError := service.Mailer.SendPaymentConfirmationAndTicketReservation(fullName, user.Email, eventDetails) 

			if sendEmailError != nil {
				log.Printf("Error sending confirmation email for payment %s: %v", payment.ID, sendEmailError)
			}
		}
	} else if err != nil {
		log.Printf("Webhook Warning: Failed to retrieve reservations for successful payment %s: %v", payment.ID, err)
	}

	service.createPaymentLog(
		ctx,
		updatedPayment,
		stripe.PaymentIntentStatusSucceeded,
		"Payment succeeded.",
		intent.ID,
		intent.PaymentMethod.ID,
		intent.Amount,
	)

	return nil
}

func (service *Service) handlePaymentIntentFailure(ctx context.Context, intent stripe.PaymentIntent) error {
	payment, user, err := service.getPaymentAndUserFromIntent(ctx, intent)

	if err != nil {
		return err
	}

	amountStr := fmt.Sprintf("%.2f", float64(intent.Amount)/100.0)
	errorMessage := "Payment failed."

	if intent.LastPaymentError != nil {
		errorMessage = intent.LastPaymentError.Msg
	}

	updatedPayment, err := service.updatePaymentStatus(ctx, payment, "payment_failed", intent.ID, amountStr)
	if err != nil {
		return err
	}

	// Fetch reservations and send failure email.
	userReservations, err := service.DB.GetUserReservationsByPaymentId(ctx, database.GetUserReservationsByPaymentIdParams{
		UserID: user.ID,
		PaymentID: updatedPayment.ID,
	})

	if err == nil && len(userReservations) > 0 {
		eventDetailIds := make([]uuid.UUID, len(userReservations))

		for i, ur := range userReservations {
			eventDetailIds[i] = ur.EventDetailID
		}

		eventDetails, err := service.DB.GetEventDetailsWithTitleByIds(ctx, eventDetailIds)
		if err == nil {
			fullName := fmt.Sprintf("%s %s", user.Firstname, user.Lastname)
			sendEmailError := service.Mailer.SendPaymentFailedNotification(fullName, user.Email, errorMessage, eventDetails) 

			if sendEmailError != nil {
				log.Printf("Error sending payment failed email for payment %s: %v", payment.ID, sendEmailError)
			}
		}
	} else if err != nil {
		log.Printf("Webhook Warning: Failed to retrieve reservations for failed payment %s: %v", payment.ID, err)
	}

	service.createPaymentLog(
		ctx,
		updatedPayment,
		"payment_failed",
		errorMessage,
		intent.ID,
		intent.PaymentMethod.ID,
		intent.Amount,
	)

	return nil
}

func (service *Service) handlePaymentIntentRequiresAction(ctx context.Context, intent stripe.PaymentIntent) error {
	payment, _, err := service.getPaymentAndUserFromIntent(ctx, intent)

	if err != nil {
		return err
	}

	amountStr := fmt.Sprintf("%.2f", float64(intent.Amount)/100.0)

	// Use payment's expiration time from DB for the message.
	message := fmt.Sprintf("Payment requires action. Please complete the action before %s", payment.ExpiresAt.Format("2006-01-02 15:04:05"))

	updatedPayment, err := service.updatePaymentStatus(ctx, payment, string(stripe.PaymentIntentStatusRequiresAction), intent.ID, amountStr)

	if err != nil {
		return err
	}

	service.createPaymentLog(
		ctx,
		updatedPayment,
		stripe.PaymentIntentStatusRequiresAction,
		message,
		intent.ID,
		intent.PaymentMethod.ID,
		intent.Amount,
	)

	return nil
}

func (service *Service) handleChargeRefunded(ctx context.Context, charge stripe.Charge) {
	paymentIntentID := charge.PaymentIntent.ID

	if paymentIntentID == "" {
		log.Printf("Webhook Warning: charge.refunded event received without PaymentIntent ID.")

		return
	}

	dbPayment, err := service.DB.GetPaymentByPaymentIntentId(ctx, sqlutil.StringToNullString(paymentIntentID))

	if err != nil {
		log.Printf("Webhook Error: Failed to find payment by intent ID %s for refund confirmation: %v", paymentIntentID, err)

		return
	}

	if dbPayment.Status != "refund pending" {
		log.Printf("Webhook Warning: charge.refunded received for Payment ID %s but status is %s, skipping final update.", dbPayment.ID, dbPayment.Status)

		return
	}

	updatePaymentParams := database.UpdatePaymentParams{
		Amount:          "0.00", 
		Status:          "refunded",
		PaymentIntentID: dbPayment.PaymentIntentID,
		ID:              dbPayment.ID,
		UserID:          dbPayment.UserID,
	}

	_, updatePaymentError := service.DB.UpdatePayment(ctx, updatePaymentParams)

	if updatePaymentError != nil {
		log.Printf("Webhook CRITICAL: Failed to update payment status for %s to refunded: %v", dbPayment.ID, updatePaymentError)
	}

	service.createPaymentLog(
		ctx,
		dbPayment,
		stripe.PaymentIntentStatus(stripe.RefundStatusSucceeded),
		"Refund confirmed by Stripe webhook. Amount set to 0.00.",
		paymentIntentID,
		"",
		charge.AmountRefunded,
	)

	log.Printf("Webhook: Refund Confirmed for Payment ID: %s (Intent: %s)", dbPayment.ID, paymentIntentID)
}

func (service *Service) handleRefundFailed(ctx context.Context, refundEvent stripe.Refund) {
	paymentIntentID := refundEvent.PaymentIntent.ID

	if paymentIntentID == "" {
		log.Printf("Webhook Warning: refund.failed event received without PaymentIntent ID.")

		return
	}

	dbPayment, err := service.DB.GetPaymentByPaymentIntentId(ctx, sqlutil.StringToNullString(paymentIntentID))

	if err != nil {
		log.Printf("Webhook Error: Failed to find payment by intent ID %s for refund failure: %v", paymentIntentID, err)

		return
	}

	failureReason := string(refundEvent.FailureReason)
	if failureReason == "" {
		failureReason = "Unknown reason"
	}
	logDescription := fmt.Sprintf("Refund failed: %s", failureReason)

	updatePaymentParams := database.UpdatePaymentParams{
		Amount:          dbPayment.Amount,
		Status:          "refund_failed",
		PaymentIntentID: dbPayment.PaymentIntentID,
		ID:              dbPayment.ID,
		UserID:          dbPayment.UserID,
	}

	_, updatePaymentError := service.DB.UpdatePayment(ctx, updatePaymentParams)

	if updatePaymentError != nil {
		log.Printf("Webhook CRITICAL: Failed to update payment status for %s to refund_failed: %v", dbPayment.ID, updatePaymentError)
	}

	service.createPaymentLog(
		ctx,
		dbPayment,
		stripe.PaymentIntentStatus(stripe.RefundStatusFailed),
		logDescription,
		paymentIntentID,
		"",
		refundEvent.Amount,
	)

	sendRefundErrorEmailError := service.Mailer.SendRefundErrorNotification()

	if sendRefundErrorEmailError != nil {
		log.Printf("Error sending refund error notification")
	}

	log.Printf("Webhook: Refund Failed for Payment ID: %s (Intent: %s). Reason: %s", dbPayment.ID, paymentIntentID, failureReason)
}

func (service *Service) getPaymentAndUserFromIntent(ctx context.Context, intent stripe.PaymentIntent) (database.Payment, database.User, error) {
	paymentIDStr := intent.Metadata["payment_id"]

	if paymentIDStr == "" {
		err := errors.New("stripe payment intent missing 'payment_id' metadata")
		log.Printf("Webhook Error: %v (Intent: %s)", err, intent.ID)

		return database.Payment{}, database.User{}, err
	}

	paymentID, parseErr := uuid.Parse(paymentIDStr)

	if parseErr != nil {
		log.Printf("Webhook Error: Failed to parse payment id '%s' from intent %s: %v", paymentIDStr, intent.ID, parseErr)

		return database.Payment{}, database.User{}, parseErr
	}

	payment, getPaymentErr := service.DB.GetPaymentByIdOnly(ctx, paymentID)

	if getPaymentErr != nil {
		log.Printf("Webhook Error: Failed to get payment details for ID %s: %v", paymentID, getPaymentErr)

		return database.Payment{}, database.User{}, getPaymentErr
	}

	user, getUserErr := service.DB.GetUserById(ctx, payment.UserID)

	if getUserErr != nil {
		log.Printf("Webhook Error: Failed to get user details for user ID %s: %v", payment.UserID, getUserErr)

		return database.Payment{}, database.User{}, getUserErr
	}

	return payment, user, nil
}

func (service *Service) updatePaymentStatus(ctx context.Context, currentPayment database.Payment, newStatus, intentID, amount string) (database.Payment, error) {
	updatePaymentParams := database.UpdatePaymentParams{
		Amount: amount,
		Status: newStatus,
		PaymentIntentID: sqlutil.StringToNullString(intentID),
		ID: currentPayment.ID,
		UserID: currentPayment.UserID,
	}

	updatedPayment, updatePaymentError := service.DB.UpdatePayment(ctx, updatePaymentParams)

	if updatePaymentError != nil {
		log.Printf("Webhook CRITICAL: Failed to update payment status for %s to %s: %v", currentPayment.ID, newStatus, updatePaymentError)
		return database.Payment{}, fmt.Errorf("failed to update payment in database: %w", updatePaymentError)
	}

	return updatedPayment, nil
}

func (service *Service) createPaymentLog(
	ctx context.Context,
	dbPayment database.Payment,
	status stripe.PaymentIntentStatus,
	description string,
	intentID string,
	paymentMethodID string,
	amount int64,
) {
	user, err := service.DB.GetUserById(ctx, dbPayment.UserID)

	userEmail := "unknown@example.com"
	
	if err == nil {
		userEmail = user.Email
	} else {
		log.Printf("Webhook Auditing Error: Failed to fetch user %s for payment log: %v", dbPayment.UserID, err)
	}

	params := database.CreatePaymentLogParams{
		ID:              uuid.New(),
		Status:          string(status),
		Description:     sqlutil.StringToNullString(description),
		PaymentIntentID: intentID,
		PaymentMethodID: sqlutil.StringToNullString(paymentMethodID),
		Amount:          fmt.Sprintf("%.2f", float64(amount)/100.0),
		UserEmail:       userEmail,
		PaymentID:       dbPayment.ID,
	}

	if _, err := service.DB.CreatePaymentLog(ctx, params); err != nil {
		log.Printf("Webhook CRITICAL: Failed to create payment log for status %s: %v", status, err)
	}
}