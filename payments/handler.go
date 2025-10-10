package payments

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/elorenzorodz/event-mrs/common"
	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v83"
	"github.com/stripe/stripe-go/v83/paymentintent"
	"github.com/stripe/stripe-go/v83/refund"
)

func DatabasePaymentToPaymentJSON(databasePayment database.Payment) Payment {
	return Payment{
		ID:              databasePayment.ID,
		PaymentIntentID: databasePayment.PaymentIntentID.String,
		Amount:          float64(common.StringToFloat32(databasePayment.Amount)),
		Currency:        databasePayment.Currency,
		Status:          databasePayment.Status,
		ExpiresAt:       databasePayment.ExpiresAt,
		CreatedAt:       databasePayment.CreatedAt,
		UpdatedAt:       common.NullTimeToString(databasePayment.UpdatedAt),
		UserID:          databasePayment.UserID,
	}
}

func DatabasePaymentsToPaymentsJSON(databasePayments []database.Payment) []Payment {
	payments := []Payment{}

	for _, databasePayment := range databasePayments {
		payments = append(payments, DatabasePaymentToPaymentJSON(databasePayment))
	}

	return payments
}

func ProcessExpiredPayment(payment *database.Payment, db *database.Queries, ctx context.Context, userEmail string) string {
	currentDateTime := time.Now()

	if currentDateTime.After(payment.ExpiresAt) {
		// User failed to process payment before expiration.
		deletePaymentParams := database.RestoreTicketsAndDeletePaymentParams{
			PaymentID: payment.ID,
			UserID:    payment.UserID,
		}

		deletePaymentError := db.RestoreTicketsAndDeletePayment(ctx, deletePaymentParams)

		if deletePaymentError != nil {
			return fmt.Sprintf("error: failed to delete expired payment | ID: %s", payment.ID)
		}

		// Skip Stripe payment intent cancellation of payment_intent_id is null.
		if payment.PaymentIntentID.Valid {
			createPaymentLogParams := database.CreatePaymentLogParams {
				ID: uuid.New(),
				PaymentIntentID: payment.PaymentIntentID.String,
				Amount: payment.Amount,
				UserEmail: userEmail,
				PaymentID: payment.ID,
			}

			stripe.Key = common.GetEnvVariable("STRIPE_SECRET_KEY")

			paymentIntentParams := &stripe.PaymentIntentParams{}

			paymentIntent, retrievePaymentIntentError := paymentintent.Get(payment.PaymentIntentID.String, paymentIntentParams)

			if retrievePaymentIntentError != nil {
				return fmt.Sprintln("error: failed to get stripe payment details")
			}

			if string(paymentIntent.Status) != string(stripe.PaymentIntentStatusCanceled) {
				paymentIntentCancelParams := &stripe.PaymentIntentCancelParams{
					CancellationReason: stripe.String("abandoned"),
				}

				_, paymentIntentCancelError := paymentintent.Cancel(payment.PaymentIntentID.String, paymentIntentCancelParams)

				if paymentIntentCancelError != nil {
					if stripeError, ok := paymentIntentCancelError.(*stripe.Error); ok {
						createPaymentLogParams.Status = string(stripeError.Code)
						createPaymentLogParams.Description = common.StringToNullString(stripeError.Msg)
					}
				} else {
					createPaymentLogParams.Status = "cancelled"
					createPaymentLogParams.Description = common.StringToNullString("payment expired")

					_, createPaymentLogError := db.CreatePaymentLog(ctx, createPaymentLogParams)

					if createPaymentLogError != nil {
						log.Printf("error: create payment log - %s", createPaymentLogError)
					}
				}
			}
		}

		return fmt.Sprintf("expired payment successfully deleted and restored tickets | ID: %s", payment.ID)
	}

	return ""
}

func ProcessRefund(db *database.Queries, ctx context.Context, paymentReservationDetails []database.GetPaymentAndReservationDetailsRow, userEmail string) (PaymentRefundResponse, error) {
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
		waitGroup                sync.WaitGroup
		errorChannel             = make(chan error, len(paymentReservationDetails))
	)

	// Create array of reservations to be refunded and total refund amount.
	for _, paymentAndReservationDetail := range paymentReservationDetails {
		paymentReservationDetail := paymentAndReservationDetail

		waitGroup.Go(func() {
			currentDateTime := time.Now()

			dateTimeDifference := paymentReservationDetail.ShowDate.Time.Sub(currentDateTime)

			if dateTimeDifference < 0 {
				dateTimeDifference = -dateTimeDifference
			}

			// Allow refund for show dates with more than 2 days difference.
			if dateTimeDifference.Hours() > 48 {
				amount, amountParseError := common.PriceStringToCents(paymentReservationDetail.Price.String)

				if amountParseError != nil {
					errorChannel <- fmt.Errorf("error processing refund for price of ticket %s, for event: %s, showing on: %s", paymentReservationDetail.TicketDescription.String, paymentReservationDetail.Title.String, paymentReservationDetail.ShowDate.Time)

					return
				}

				reservation := ReservationForRefund{
					EventTitle: 		paymentReservationDetail.Title.String,
					TicketDescription: 	paymentReservationDetail.TicketDescription.String,
					ShowDate: 			paymentReservationDetail.ShowDate.Time,
					PaymentID:     		paymentReservationDetail.PaymentID,
					ReservationID: 		paymentReservationDetail.ReservationID.UUID,
					EventDetailID: 		paymentReservationDetail.EventDetailID.UUID,
					Amount:        		amount,
				}

				mutex.Lock()
				reservationsToBeRefunded = append(reservationsToBeRefunded, reservation)
				mutex.Unlock()
			}
		})
	}

	waitGroup.Wait()
	close(errorChannel)

	allErrors := []string{}

	for err := range errorChannel {
		if err != nil {
			allErrors = append(allErrors, err.Error())
		}
	}

	if len(reservationsToBeRefunded) == 0 {
		return PaymentRefundResponse{}, fmt.Errorf("errors: \n%s", strings.Join(allErrors, ",\n"))
	}
	
	userId, _ := uuid.Parse(paymentReservationDetails[0].UserID.String())

	var (
		paymentRefundResponse PaymentRefundResponse
		refundMutex           sync.Mutex
		refundWaitGroup       sync.WaitGroup
		totalRefundAmount     int64
		refundErrorChannel    = make(chan error, len(paymentReservationDetails))
	)

	for _, refundReservation := range reservationsToBeRefunded {
		reservationToBeRefunded := refundReservation

		refundWaitGroup.Go(func() {
			refundPaymentAndRestoreTicketsParams := database.RefundPaymentAndRestoreTicketsParams{
				EventDetailID: reservationToBeRefunded.EventDetailID,
				Amount:        fmt.Sprintf("%.2f", float64(reservationToBeRefunded.Amount)/100.0),
				PaymentID:     reservationToBeRefunded.PaymentID,
				UserID:        userId,
				ReservationID: reservationToBeRefunded.ReservationID,
			}

			refundPaymentAndRestoreTicketsError := db.RefundPaymentAndRestoreTickets(ctx, refundPaymentAndRestoreTicketsParams)

			if refundPaymentAndRestoreTicketsError != nil {
				refundErrorChannel <- fmt.Errorf("error updating payment refund for %s - %s, price: %v, show date: %s", reservationToBeRefunded.EventTitle, reservationToBeRefunded.TicketDescription, reservationToBeRefunded.Amount, reservationToBeRefunded.ShowDate)

				return
			}

			paymentRefunded := PaymentRefunded {
				PaymentID: reservationToBeRefunded.PaymentID,
				Amount: fmt.Sprintf("%.2f", float64(reservationToBeRefunded.Amount)/100.0),
				Title: reservationToBeRefunded.EventTitle,
				TicketDescription: reservationToBeRefunded.TicketDescription,
				ShowDate: reservationToBeRefunded.ShowDate,
			}

			refundMutex.Lock()
			paymentRefundResponse.PaymentRefunds = append(paymentRefundResponse.PaymentRefunds, paymentRefunded)
			totalRefundAmount += reservationToBeRefunded.Amount
			refundMutex.Unlock()
		})
	}

	refundWaitGroup.Wait()
	close(refundErrorChannel)

	allRefundErrors := []string{}

	for err := range refundErrorChannel {
		if err != nil {
			allRefundErrors = append(allRefundErrors, err.Error())
		}
	}

	if len(paymentRefundResponse.PaymentRefunds) == 0 {
		return PaymentRefundResponse{}, fmt.Errorf("errors: \n%s, %s", strings.Join(allErrors, ",\n"), strings.Join(allRefundErrors, ",\n"))
	}

	paymentIntentId := paymentReservationDetails[0].PaymentIntentID.String
	createPaymentLogParams := database.CreatePaymentLogParams {
		ID: uuid.New(),
		PaymentIntentID: paymentIntentId,
		Amount: fmt.Sprintf("%.2f", float64(totalRefundAmount)/100.0),
		UserEmail: userEmail,
		PaymentID: paymentReservationDetails[0].PaymentID,
	}

	stripe.Key = common.GetEnvVariable("STRIPE_SECRET_KEY")

	refundParams := &stripe.RefundParams {
		Amount: stripe.Int64(totalRefundAmount),
		PaymentIntent: stripe.String(paymentIntentId),
	}

	refundResult, refundError := refund.New(refundParams)

	if refundError != nil {
		sendRefundErrorEmailError := common.SendRefundErrorNotification()

		if sendRefundErrorEmailError != nil {
			log.Printf("error sending refund error notification")
		}
		
		if stripeError, ok := refundError.(*stripe.Error); ok {
			createPaymentLogParams.Status = string(stripeError.Code)
			createPaymentLogParams.Description = common.StringToNullString(stripeError.Msg)

			_, createPaymentLogError := db.CreatePaymentLog(ctx, createPaymentLogParams)

			if createPaymentLogError != nil {
				log.Printf("error: create payment log - %s", createPaymentLogError)
			}
		}

		return PaymentRefundResponse{}, fmt.Errorf("refund failed, we have notified our team to manually refund the amount")
	}

	var aggregatedErrors error = nil

	if len(allErrors) > 0 {
		aggregatedErrors = fmt.Errorf("%s", strings.Join(allErrors, ",\n"))
	}

	if len(allRefundErrors) > 0 {
		if aggregatedErrors == nil {
			aggregatedErrors = fmt.Errorf("%s", strings.Join(allRefundErrors, ",\n"))
		} else {
			aggregatedErrors = fmt.Errorf("%s\n%s", aggregatedErrors, strings.Join(allRefundErrors, ",\n"))
		}
	}

	createPaymentLogParams.Status = string(refundResult.Status)

	switch refundResult.Status {
		case stripe.RefundStatusFailed:
			paymentRefundResponse.Message = fmt.Sprintf("%s, please contact our support for further assistance", string(refundResult.FailureReason))
			createPaymentLogParams.Description = common.StringToNullString(string(refundResult.FailureReason))

		case stripe.RefundStatusPending:
			paymentRefundResponse.Message = "your refund is on the way, we'll notify you once it succeeded"
			createPaymentLogParams.Description = common.StringToNullString("refund pending")

		case stripe.RefundStatusSucceeded:
			paymentRefundResponse.Message = "refund succeeded"
			createPaymentLogParams.Description = common.StringToNullString(paymentRefundResponse.Message)
	}

	_, createPaymentLogError := db.CreatePaymentLog(ctx, createPaymentLogParams)

	if createPaymentLogError != nil {
		log.Printf("error: create payment log - %s", createPaymentLogError)
	}
	
	return paymentRefundResponse, aggregatedErrors
}