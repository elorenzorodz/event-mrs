package event_details

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/elorenzorodz/event-mrs/common"
	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v83"
	"github.com/stripe/stripe-go/v83/paymentintent"
	"github.com/stripe/stripe-go/v83/refund"
)

func DatabaseEventDetailToEventDetailJSON(databaseEventDetail database.EventDetail) EventDetail {
	return EventDetail{
		ID:                databaseEventDetail.ID,
		ShowDate:          databaseEventDetail.ShowDate,
		Price:             common.StringToFloat32(databaseEventDetail.Price),
		NumberOfTickets:   databaseEventDetail.NumberOfTickets,
		TicketsRemaining:  databaseEventDetail.TicketsRemaining,
		TicketDescription: databaseEventDetail.TicketDescription,
		CreatedAt:         databaseEventDetail.CreatedAt,
		UpdatedAt:         common.NullTimeToString(databaseEventDetail.UpdatedAt),
		EventID:           databaseEventDetail.EventID,
	}
}

func DatabaseEventDetailsToEventDetailsJSON(databaseEventDetails []database.EventDetail) []EventDetail {
	eventDetails := []EventDetail{}

	for _, databaseEventDetail := range databaseEventDetails {
		eventDetails = append(eventDetails, DatabaseEventDetailToEventDetailJSON(databaseEventDetail))
	}

	return eventDetails
}

func EventDetailRefundOrCancelPayment(db *database.Queries, ctx context.Context, eventDetailId uuid.UUID, userId uuid.UUID, userEmail string) ([]EventDetailFailedRefundOrCancel, []FailedNotificationEmail, error) {
	getPaidEventDetailForRefundParams := database.GetPaidEventDetailForRefundParams {
		EventDetailID: eventDetailId,
		UserID:  userId,
	}

	paidEventDetailForRefunds, getRefundEventDetailPaymentError := db.GetPaidEventDetailForRefund(ctx, getPaidEventDetailForRefundParams)

	if getRefundEventDetailPaymentError != nil {
		return []EventDetailFailedRefundOrCancel{}, []FailedNotificationEmail{}, fmt.Errorf("failed to get payment and reservations for the event detail")
	}

	if len(paidEventDetailForRefunds) == 0 {
		return []EventDetailFailedRefundOrCancel{}, []FailedNotificationEmail{}, nil
	}

	var (
		PaymentIDs                 []uuid.UUID
		mutex                      sync.Mutex
		waitGroup                  sync.WaitGroup
		eventDetailFailedRefundOrCancels []EventDetailFailedRefundOrCancel
	)

	stripe.Key = common.GetEnvVariable("STRIPE_SECRET_KEY")

	for _, paidEventDetail := range paidEventDetailForRefunds {
		paidEventDetailForRefund := paidEventDetail
		amount, _ := common.PriceStringToCents(paidEventDetailForRefund.Amount)
		ticketPrice, _ := common.PriceStringToCents(paidEventDetailForRefund.TicketPrice)
		isErrorOccured := false

		if paidEventDetailForRefund.Status == "refunded" || paidEventDetailForRefund.Status == "cancelled" {
			continue
		}

		if amount == 0 {
			PaymentIDs = append(PaymentIDs, paidEventDetailForRefund.PaymentID)
		} else {
			if amount != ticketPrice {
				// If paid amount is different from ticket price, user might have been booked different events in a single payment.
				amount = ticketPrice
			}
		}

		waitGroup.Go(func() {
			eventFailedRefundOrCancel := EventDetailFailedRefundOrCancel{}

			updatePaymentParams := database.UpdatePaymentParams{
				ID:              paidEventDetailForRefund.PaymentID,
				Amount:          fmt.Sprintf("%.2f", float64(amount)/100.0),
				PaymentIntentID: paidEventDetailForRefund.PaymentIntentID,
				UserID:          userId,
			}

			createPaymentLogParams := database.CreatePaymentLogParams{
				ID:              uuid.New(),
				PaymentIntentID: paidEventDetailForRefund.PaymentID.String(),
				Amount:          fmt.Sprintf("%.2f", float64(amount)/100.0),
				UserEmail:       userEmail,
				PaymentID:       paidEventDetailForRefund.PaymentID,
			}

			if paidEventDetailForRefund.Status == string(stripe.PaymentIntentStatusSucceeded) {
				refundParams := &stripe.RefundParams{
					Amount:        stripe.Int64(amount),
					PaymentIntent: stripe.String(paidEventDetailForRefund.PaymentIntentID.String),
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

						eventFailedRefundOrCancel.PaymentID = paidEventDetailForRefund.PaymentID
						eventFailedRefundOrCancel.Action = "stripe refund request"
						eventFailedRefundOrCancel.Code = string(stripeError.Code)
						eventFailedRefundOrCancel.Message = stripeError.Msg

						isErrorOccured = true
					}
				}

				createPaymentLogParams.Status = string(refundResult.Status)

				switch refundResult.Status {
				case stripe.RefundStatusFailed:
					createPaymentLogParams.Description = common.StringToNullString(string(refundResult.FailureReason))

				case stripe.RefundStatusPending:
					createPaymentLogParams.Description = common.StringToNullString("refund pending")
					updatePaymentParams.Status = "refund pending"

				case stripe.RefundStatusSucceeded:
					createPaymentLogParams.Description = common.StringToNullString("refund succeeded")
					updatePaymentParams.Status = "refunded"
				}
			} else {
				paymentIntentCancelParams := &stripe.PaymentIntentCancelParams{
					CancellationReason: stripe.String("abandoned"),
				}

				_, paymentIntentCancelError := paymentintent.Cancel(paidEventDetailForRefund.PaymentIntentID.String, paymentIntentCancelParams)

				if paymentIntentCancelError != nil {
					if stripeError, ok := paymentIntentCancelError.(*stripe.Error); ok {
						createPaymentLogParams.Status = string(stripeError.Code)
						createPaymentLogParams.Description = common.StringToNullString(stripeError.Msg)

						eventFailedRefundOrCancel.PaymentID = paidEventDetailForRefund.PaymentID
						eventFailedRefundOrCancel.Action = "stripe cancel request"
						eventFailedRefundOrCancel.Code = string(stripeError.Code)
						eventFailedRefundOrCancel.Message = stripeError.Msg
						isErrorOccured = true
					}
				} else {
					updatePaymentParams.Status = "cancelled"
					createPaymentLogParams.Status = "cancelled"
					createPaymentLogParams.Description = common.StringToNullString("event detail deleted")
				}
			}

			_, createPaymentLogError := db.CreatePaymentLog(ctx, createPaymentLogParams)

			if createPaymentLogError != nil {
				log.Printf("error: create payment log - %s", createPaymentLogError)
			}

			if isErrorOccured {
				mutex.Lock()
				eventDetailFailedRefundOrCancels = append(eventDetailFailedRefundOrCancels, eventFailedRefundOrCancel)
				mutex.Unlock()

				return
			}

			_, updatePaymentError := db.UpdatePayment(ctx, updatePaymentParams)

			if updatePaymentError != nil {
				log.Printf("error: update payment - %s", updatePaymentError)
			}

			mutex.Lock()
			PaymentIDs = append(PaymentIDs, paidEventDetailForRefund.PaymentID)
			mutex.Unlock()
		})
	}

	waitGroup.Wait()

	if len(PaymentIDs) == 0 {
		return eventDetailFailedRefundOrCancels, []FailedNotificationEmail{}, nil
	}

	// Get payments for deletion.
	payments, _ := db.GetMultiplePayments(ctx, PaymentIDs)

	var (
		sendRefundCanceWaitGroup          sync.WaitGroup
		sendRefundCancelNotifErrorChannel = make(chan error, len(payments))
	)

	for _, pymnt := range payments {
		payment := pymnt
		eventDetailFailedRefundOrCancel := EventDetailFailedRefundOrCancel{}

		sendRefundCanceWaitGroup.Go(func() {
			user, getUserByIdError := db.GetUserById(ctx, payment.UserID)

			if getUserByIdError != nil {
				eventDetailFailedRefundOrCancel.PaymentID = payment.ID
				eventDetailFailedRefundOrCancel.Action = "get user email for email notification"
				eventDetailFailedRefundOrCancel.Message = getUserByIdError.Error()
			}

			eventTitle := paidEventDetailForRefunds[0].Title

			recipientName := fmt.Sprintf("%s %s", user.Firstname, user.Lastname)

			refundOrCancelledNotifMessage := fmt.Sprintf(`Hi %s,

The event reservation you've booked: %s, was cancelled and your payment was refunded. 
If you didn't pay yet, the pending payment is now cancelled.
Sorry for the inconvencience.

- Event - MRS Team`, recipientName, eventTitle)

			sendRefundCancelError := common.SendRefundOrCancelledEmail(recipientName, user.Email, eventTitle, refundOrCancelledNotifMessage)

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
			failedNotificationEmail := FailedNotificationEmail{
				SendRefundCancelNotificationError: errorMessage.Error(),
			}

			failedNotificationEmails = append(failedNotificationEmails, failedNotificationEmail)
		}
	}

	return eventDetailFailedRefundOrCancels, failedNotificationEmails, nil
}