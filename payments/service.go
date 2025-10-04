package payments

import (
	"log"
	"net/http"
	"time"

	"github.com/elorenzorodz/event-mrs/common"
	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v83"
	"github.com/stripe/stripe-go/v83/paymentintent"
)

func (paymentAPIConfig *PaymentAPIConfig) UpdatePayment(ginContext *gin.Context) {
	userId, parseUserIdError := uuid.Parse(ginContext.MustGet("userId").(uuid.UUID).String())

	if parseUserIdError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})

		return
	}

	paymentId, parsePaymentIdError := uuid.Parse(ginContext.Param("paymentId"))

	if parsePaymentIdError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment ID"})

		return
	}

	paymentParams := PaymentParameters{}

	// Bind incoming JSON to struct and check for errors in the process.
	if parameterBindError := ginContext.ShouldBindJSON(&paymentParams); parameterBindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "error parsing JSON, please check all required fields are present"})

		return
	}

	getPaymentByIdParams := database.GetPaymentByIdParams {
		ID: paymentId,
		UserID: userId,
	}

	currentPayment, getPaymentByIdError := paymentAPIConfig.DB.GetPaymentById(ginContext.Request.Context(), getPaymentByIdParams)
	
	if getPaymentByIdError != nil {
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get payment details, please try again in a few minutes"})

		return
	}

	currentDateTime := time.Now()

	if currentPayment.ExpiresAt.After(currentDateTime) {
		// User failed to process payment before expiration.
		deletePaymentParams := database.RestoreTicketsAndDeletePaymentParams {
			PaymentID: currentPayment.ID,
			UserID: userId,
		}

		deletePaymentError := paymentAPIConfig.DB.RestoreTicketsAndDeletePayment(ginContext.Request.Context(), deletePaymentParams)

		if deletePaymentError != nil {
			ginContext.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process your payment, please try again in a few minutes"})

			return
		}

		stripe.Key = common.GetEnvVariable("STRIPE_SECRET_KEY")

		paymentIntentCancelParams := &stripe.PaymentIntentCancelParams {
			CancellationReason: stripe.String("abandoned"),
		}

		paymentintent.Cancel(currentPayment.PaymentIntentID.String, paymentIntentCancelParams)

		ginContext.JSON(http.StatusOK, gin.H{"message": "your payment have expired, please rebook your tickets again"})

		return
	}

	paymentIntentConfirmParams := &stripe.PaymentIntentConfirmParams {
		PaymentMethod: stripe.String(paymentParams.PaymentMethodID),
	}

	paymentIntentResult, paymentIntentError := paymentintent.Confirm(currentPayment.PaymentIntentID.String, paymentIntentConfirmParams)

	var paymentIntentId string
	paymentResponse := PaymentResponse {
		ID: currentPayment.ID,
	}

	if paymentIntentError != nil {
		if stripeErr, ok := paymentIntentError.(*stripe.Error); ok {
			paymentResponse.Status = *stripe.String(stripeErr.Code)
			paymentResponse.Message = *stripe.String(stripeErr.Msg)

			if stripeErr.PaymentIntent != nil {
				paymentIntentId = stripeErr.PaymentIntent.ID
			}
		}
	} else {
		if paymentIntentResult != nil {
			paymentResponse.Status = string(paymentIntentResult.Status)
			paymentIntentId = paymentIntentResult.ID
			paymentResponse.Message = "payment successful"

			if paymentResponse.Status == string(stripe.PaymentIntentStatusRequiresAction) {
				paymentResponse.ClientSecret = paymentIntentResult.ClientSecret

				if paymentIntentResult.NextAction != nil {
					paymentResponse.NextAction = string(paymentIntentResult.NextAction.Type)
				}
			} else if paymentResponse.Status != string(stripe.PaymentIntentStatusSucceeded) {
				paymentResponse.Message = "please refer to next action and status"
			}
		}
	}

	if paymentResponse.Status != string(stripe.PaymentIntentStatusSucceeded) {
		switch paymentResponse.Status {
			case string(stripe.PaymentIntentStatusRequiresAction):
				paymentResponse.Message = "complete payment within next 15 minutes"

			case string(stripe.PaymentIntentStatusCanceled):
				paymentResponse.Message = "payment expired, please rebook your tickets"

				deletePaymentParams := database.RestoreTicketsAndDeletePaymentParams {
					PaymentID: currentPayment.ID,
					UserID: userId,
				}

				deletePaymentError := paymentAPIConfig.DB.RestoreTicketsAndDeletePayment(ginContext.Request.Context(), deletePaymentParams)

				if deletePaymentError != nil {
					ginContext.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process your payment, please try again in a few minutes"})
					
					return
				}

			case string(stripe.PaymentIntentStatusProcessing):
				paymentResponse.Message = "payment processing, we'll send you an email once payment succeeded"

			case string(stripe.PaymentIntentStatusRequiresPaymentMethod):
				paymentResponse.Message = "please submit new payment method"
		}
	}

	updatePaymentParams := database.UpdatePaymentParams {
		Amount: currentPayment.Amount,
		Status: paymentResponse.Status,
		PaymentIntentID: common.StringToNullString(paymentIntentId),
		ID: currentPayment.ID,
		UserID: userId,
	}

	_, updatePaymentError := paymentAPIConfig.DB.UpdatePayment(ginContext.Request.Context(), updatePaymentParams)

	if updatePaymentError != nil {
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update your payment, please try again in a few minutes"})

		return
	}

	ginContext.JSON(http.StatusOK, gin.H{"payment": paymentResponse})
}

func (paymentAPIConfig *PaymentAPIConfig) ProcessStripePayment(ginContext *gin.Context) {
	stripePayloadParams := StripePayloadParameters{}

	// Bind incoming JSON to struct and check for errors in the process.
	if parameterBindError := ginContext.ShouldBindJSON(&stripePayloadParams); parameterBindError != nil {
		log.Printf("error: failed to bind payload to parameters, %s", parameterBindError)

		return
	}

	// TODO: Check if payment already succeeded. Webhook triggered by paymentintent.Confirm() from ticket reservation function.
	
}