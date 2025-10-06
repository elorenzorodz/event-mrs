package payments

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/elorenzorodz/event-mrs/common"
	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/stripe/stripe-go/v83"
	"github.com/stripe/stripe-go/v83/paymentintent"
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

func ProcessExpiredPayment(payment *database.Payment, db *database.Queries, ctx context.Context) string {
	currentDateTime := time.Now()

	if currentDateTime.After(payment.ExpiresAt) {
		// User failed to process payment before expiration.
		deletePaymentParams := database.RestoreTicketsAndDeletePaymentParams {
			PaymentID: payment.ID,
			UserID: payment.UserID,
		}

		deletePaymentError := db.RestoreTicketsAndDeletePayment(ctx, deletePaymentParams)

		if deletePaymentError != nil {
			return fmt.Sprintf("error: failed to delete expired payment | ID: %s", payment.ID)
		}

		// Skip Stripe payment intent cancellation of payment_intent_id is null.
		if payment.PaymentIntentID.Valid {
			stripe.Key = common.GetEnvVariable("STRIPE_SECRET_KEY")

			paymentIntentParams := &stripe.PaymentIntentParams {}

			paymentIntent, retrievePaymentIntentError := paymentintent.Get(payment.PaymentIntentID.String, paymentIntentParams)

			if retrievePaymentIntentError != nil {
				return fmt.Sprintln("error: failed to get stripe payment details")
			}

			if string(paymentIntent.Status) != string(stripe.PaymentIntentStatusCanceled) {
				paymentIntentCancelParams := &stripe.PaymentIntentCancelParams {
					CancellationReason: stripe.String("abandoned"),
				}

				_, paymentIntentCancelError := paymentintent.Cancel(payment.PaymentIntentID.String, paymentIntentCancelParams)

				if paymentIntentCancelError != nil {
					log.Printf("error payment intent cancel: %s", paymentIntentCancelError)
				}
			}
		}

		return fmt.Sprintf("expired payment successfully deleted and restored tickets | ID: %s", payment.ID)
	}

	return ""
}