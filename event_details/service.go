package event_details

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/elorenzorodz/event-mrs/internal/convert"
	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/elorenzorodz/event-mrs/internal/mailer"
	"github.com/elorenzorodz/event-mrs/internal/sqlutil"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v83"
	"github.com/stripe/stripe-go/v83/paymentintent"
	"github.com/stripe/stripe-go/v83/refund"
)

func NewService(dbQueries database.Queries, mMailer *mailer.Mailer, stripeClient StripeClient) EventDetailService {
	return &Service{
		DBQueries: dbQueries,
		Mailer:    mMailer,
		Stripe:    stripeClient,
	}
}

func (stripeAPIClient *StripeAPIClient) Refund(amount int64, paymentIntentID string) (*stripe.Refund, error) {
	refundParams := &stripe.RefundParams{
		Amount:        stripe.Int64(amount),
		PaymentIntent: stripe.String(paymentIntentID),
	}

	return refund.New(refundParams)
}

func (stripeAPIClient *StripeAPIClient) CancelPaymentIntent(paymentIntentID string, cancellationReason string) error {
	paymentIntentCancelParams := &stripe.PaymentIntentCancelParams{
		CancellationReason: stripe.String("abandoned"),
	}

	_, err := paymentintent.Cancel(paymentIntentID, paymentIntentCancelParams)

	return err
}

func (service *Service) Create(ctx context.Context, eventID uuid.UUID, req EventDetailParameters) (*EventDetail, error) {
	showDate, _, parseShowDateError := convert.StringToTime(req.ShowDate)

	if parseShowDateError != nil {
		return nil, parseShowDateError
	}

	priceString := fmt.Sprintf("%.2f", req.Price)

	createEventDetailParams := database.CreateEventDetailParams{
		ID:                uuid.New(),
		ShowDate:          showDate,
		Price:             priceString,
		NumberOfTickets:   req.NumberOfTickets,
		TicketsRemaining:  req.NumberOfTickets,
		TicketDescription: req.TicketDescription,
		EventID:           eventID,
	}

	createdEventDetail, createEventDetailError := service.DBQueries.CreateEventDetail(ctx, createEventDetailParams)

	if createEventDetailError != nil {
		log.Printf("error creating event detail: %v", createEventDetailError)

		return nil, errors.New("error creating event detail")
	}

	eventDetail := DatabaseEventDetailToEventDetailJSON(createdEventDetail)

	return &eventDetail, nil
}

func (service *Service) Update(ctx context.Context, eventID, eventDetailID uuid.UUID, req EventDetailParameters) (*EventDetail, error) {
	showDate, _, parseShowDateError := convert.StringToTime(req.ShowDate)

	if parseShowDateError != nil {
		return nil, parseShowDateError
	}

	priceString := fmt.Sprintf("%.2f", req.Price)

	updateEventDetailParams := database.UpdateEventDetailParams{
		ShowDate:          showDate,
		Price:             priceString,
		NumberOfTickets:   req.NumberOfTickets,
		TicketDescription: req.TicketDescription,
		ID:                eventDetailID,
		EventID:           eventID,
	}

	updatedEventDetail, updateEventDetailError := service.DBQueries.UpdateEventDetail(ctx, updateEventDetailParams)

	if updateEventDetailError != nil {
		log.Printf("error updating event detail: %v", updateEventDetailError)

		return nil, updateEventDetailError
	}

	eventDetail := DatabaseEventDetailToEventDetailJSON(updatedEventDetail)

	return &eventDetail, nil
}

func (service *Service) Delete(ctx context.Context, eventID, eventDetailID, ownerID uuid.UUID, userEmail string) ([]EventDetailFailedRefundOrCancel, []FailedNotificationEmail, error) {
	failedRefunds, failedEmails, refundCancelPaymentErrors := service.EventDetailRefundOrCancelPayment(ctx, eventDetailID, ownerID, userEmail)

	if refundCancelPaymentErrors != nil {
		return []EventDetailFailedRefundOrCancel{}, []FailedNotificationEmail{}, refundCancelPaymentErrors
	}

	deleteEventDetailParams := database.DeleteEventDetailParams{
		ID:      eventDetailID,
		EventID: eventID,
	}

	deleteEventError := service.DBQueries.DeleteEventDetail(ctx, deleteEventDetailParams)
	if deleteEventError != nil {

		if deleteEventError == sql.ErrNoRows {
			return []EventDetailFailedRefundOrCancel{}, []FailedNotificationEmail{}, deleteEventError
		}

		log.Printf("error deleting event detail: %v", deleteEventError)
		
		return failedRefunds, failedEmails, deleteEventError
	}

	return failedRefunds, failedEmails, nil
}

func (service *Service) EventDetailRefundOrCancelPayment(ctx context.Context, eventDetailID uuid.UUID, userID uuid.UUID, userEmail string) ([]EventDetailFailedRefundOrCancel, []FailedNotificationEmail, error) {
	getPaidEventDetailForRefundParams := database.GetPaidEventDetailForRefundParams {
		EventDetailID: eventDetailID,
		UserID: userID,
	}

	paidEventDetailForRefunds, getRefundEventDetailPaymentError := service.DBQueries.GetPaidEventDetailForRefund(ctx, getPaidEventDetailForRefundParams)

	if getRefundEventDetailPaymentError != nil {
		return nil, nil, fmt.Errorf("failed to get payment and reservations for the event detail")
	}

	if len(paidEventDetailForRefunds) == 0 {
		return nil, nil, nil
	}

	uniquePaymentsToProcess := make(map[string]database.GetPaidEventDetailForRefundRow)

	for _, detail := range paidEventDetailForRefunds {
		// Use the PaymentIntentID as the unique key for Stripe actions.
		if detail.PaymentIntentID.Valid {
			uniquePaymentsToProcess[detail.PaymentIntentID.String] = detail
		}
	}

	if len(uniquePaymentsToProcess) == 0 {
		return nil, nil, nil
	}

	var (
		PaymentIDs []uuid.UUID
		mutex sync.Mutex
		waitGroup sync.WaitGroup
		eventDetailFailedRefundOrCancels []EventDetailFailedRefundOrCancel
	)

	// Iterate over the map of unique payment intents.
	for _, eventDetailForRefund := range uniquePaymentsToProcess {
		paidEventDetailForRefund := eventDetailForRefund
		
		amount, _ := convert.PriceStringToCents(paidEventDetailForRefund.Amount)
		ticketPrice, _ := convert.PriceStringToCents(paidEventDetailForRefund.TicketPrice)
		isErrorOccured := false

		if paidEventDetailForRefund.Status == "refunded" || paidEventDetailForRefund.Status == "cancelled" {
			continue
		}

		// Handle free/unpaid tickets outside the goroutine as it involves no Stripe action.
		if amount == 0 || ticketPrice == 0 {
			mutex.Lock()
			PaymentIDs = append(PaymentIDs, paidEventDetailForRefund.PaymentID)
			mutex.Unlock()

			continue
		} else {
			// This is the partial refund amount for this specific ticket price.
			if amount != ticketPrice {
				amount = ticketPrice
			}
		}

		waitGroup.Go(func() {
			eventFailedRefundOrCancel := EventDetailFailedRefundOrCancel{}

			updatePaymentParams := database.UpdatePaymentParams{
				ID: paidEventDetailForRefund.PaymentID,
				Amount: fmt.Sprintf("%.2f", float64(amount)/100.0),
				PaymentIntentID: paidEventDetailForRefund.PaymentIntentID,
				UserID: userID,
			}

			createPaymentLogParams := database.CreatePaymentLogParams{
				ID: uuid.New(),
				PaymentIntentID: paidEventDetailForRefund.PaymentIntentID.String, 
				Amount: fmt.Sprintf("%.2f", float64(amount)/100.0),
				UserEmail: userEmail,
				PaymentID: paidEventDetailForRefund.PaymentID,
			}

			if paidEventDetailForRefund.Status == string(stripe.PaymentIntentStatusSucceeded) {
				refundResult, refundError := service.Stripe.Refund(amount, paidEventDetailForRefund.PaymentIntentID.String)

				if refundError != nil {
					service.Mailer.SendRefundErrorNotification()

					if stripeError, ok := refundError.(*stripe.Error); ok {
						createPaymentLogParams.Status = string(stripeError.Code)
						createPaymentLogParams.Description = sqlutil.StringToNullString(stripeError.Msg)

						eventFailedRefundOrCancel.PaymentID = paidEventDetailForRefund.PaymentID
						eventFailedRefundOrCancel.Action = "stripe refund request"
						eventFailedRefundOrCancel.Code = string(stripeError.Code)
						eventFailedRefundOrCancel.Message = stripeError.Msg

						isErrorOccured = true
					}
				}

				if refundResult != nil {
					createPaymentLogParams.Status = string(refundResult.Status)

					switch refundResult.Status {
						case stripe.RefundStatusFailed:
							createPaymentLogParams.Description = sqlutil.StringToNullString(string(refundResult.FailureReason))
						case stripe.RefundStatusPending:
							createPaymentLogParams.Description = sqlutil.StringToNullString("refund pending")
							updatePaymentParams.Status = "refund pending"
						case stripe.RefundStatusSucceeded:
							createPaymentLogParams.Description = sqlutil.StringToNullString("refund succeeded")
							updatePaymentParams.Status = "refunded"
					}
				}
			} else {
				paymentIntentCancelError := service.Stripe.CancelPaymentIntent(paidEventDetailForRefund.PaymentIntentID.String, "abandoned")

				if paymentIntentCancelError != nil {
					if stripeError, ok := paymentIntentCancelError.(*stripe.Error); ok {
						createPaymentLogParams.Status = string(stripeError.Code)
						createPaymentLogParams.Description = sqlutil.StringToNullString(stripeError.Msg)

						eventFailedRefundOrCancel.PaymentID = paidEventDetailForRefund.PaymentID
						eventFailedRefundOrCancel.Action = "stripe cancel request"
						eventFailedRefundOrCancel.Code = string(stripeError.Code)
						eventFailedRefundOrCancel.Message = stripeError.Msg
						isErrorOccured = true
					}
				} else {
					updatePaymentParams.Status = "cancelled"
					createPaymentLogParams.Status = "cancelled"
					createPaymentLogParams.Description = sqlutil.StringToNullString("event detail deleted")
				}
			}

			// Create payment log.
			service.DBQueries.CreatePaymentLog(ctx, createPaymentLogParams)

			if isErrorOccured {
				mutex.Lock()
				eventDetailFailedRefundOrCancels = append(eventDetailFailedRefundOrCancels, eventFailedRefundOrCancel)
				mutex.Unlock()
				return
			}

			// Update payment.
			service.DBQueries.UpdatePayment(ctx, updatePaymentParams)

			mutex.Lock()
			PaymentIDs = append(PaymentIDs, paidEventDetailForRefund.PaymentID)
			mutex.Unlock()
		})
	}

	waitGroup.Wait()
	
	if len(PaymentIDs) == 0 {
		return eventDetailFailedRefundOrCancels, nil, nil
	}

	// Get payments for notification. Only pass unique IDs.
	payments, _ := service.DBQueries.GetMultiplePayments(ctx, PaymentIDs)

	var (
		sendRefundCanceWaitGroup sync.WaitGroup
		sendRefundCancelNotifErrorChannel = make(chan error, len(payments))
	)

	for _, pymnt := range payments {
		payment := pymnt

		sendRefundCanceWaitGroup.Go(func() {
			
			user, getUserByIdError := service.DBQueries.GetUserById(ctx, payment.UserID)

			if getUserByIdError != nil {
				sendRefundCancelNotifErrorChannel <- fmt.Errorf("failed to get user email for payment %s: %w", payment.ID, getUserByIdError)

				return
			}

			// NOTE: We rely on the first entry in paidEventDetailForRefunds for event/ticket details for the email,
			// which is potentially inaccurate if the refund covers multiple events.
			// Using the first entry from the original query results for generic details.
			eventTitle := paidEventDetailForRefunds[0].Title
			ticketDescription := paidEventDetailForRefunds[0].TicketDescription
			recipientName := fmt.Sprintf("%s %s", user.Firstname, user.Lastname)
			refundOrCancelledNotifMessage := fmt.Sprintf(`Hi %s,

The event reservation you've booked: %s - %s, was cancelled and your payment was refunded. 
If you didn't pay yet, the pending payment is now cancelled.
Sorry for the inconvencience.

- Event - MRS Team`, recipientName, eventTitle, ticketDescription)

			sendRefundCancelError := service.Mailer.SendRefundOrCancelledEmail(recipientName, user.Email, eventTitle, refundOrCancelledNotifMessage)

			if sendRefundCancelError != nil {
				sendRefundCancelNotifErrorChannel <- sendRefundCancelError
			}
		})
	}

	sendRefundCanceWaitGroup.Wait()
	close(sendRefundCancelNotifErrorChannel)

	failedNotificationEmails := []FailedNotificationEmail{}

	for errorMessage := range sendRefundCancelNotifErrorChannel {
		if errorMessage != nil {
			failedNotificationEmails = append(failedNotificationEmails, FailedNotificationEmail{
				SendRefundCancelNotificationError: errorMessage.Error(),
			})
		}
	}

	return eventDetailFailedRefundOrCancels, failedNotificationEmails, nil
}

func DatabaseEventDetailToEventDetailJSON(databaseEventDetail database.EventDetail) EventDetail {
	priceFloat, _ := convert.StringToFloat32(databaseEventDetail.Price)

	return EventDetail{
		ID:                databaseEventDetail.ID,
		ShowDate:          databaseEventDetail.ShowDate,
		Price:             priceFloat,
		NumberOfTickets:   databaseEventDetail.NumberOfTickets,
		TicketsRemaining:  databaseEventDetail.TicketsRemaining,
		TicketDescription: databaseEventDetail.TicketDescription,
		CreatedAt:         databaseEventDetail.CreatedAt,
		UpdatedAt:         sqlutil.NullTimeToString(databaseEventDetail.UpdatedAt),
		EventID:           databaseEventDetail.EventID,
	}
}

func DatabaseEventDetailsToEventDetailsJSON(databaseEventDetails []database.EventDetail) []EventDetail {
	eventDetails := make([]EventDetail, len(databaseEventDetails))

	for i, databaseEventDetail := range databaseEventDetails {
		eventDetails[i] = DatabaseEventDetailToEventDetailJSON(databaseEventDetail)
	}

	return eventDetails
}