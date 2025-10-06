package payments

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/elorenzorodz/event-mrs/common"
	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v83"
	"github.com/stripe/stripe-go/v83/paymentintent"
	"github.com/stripe/stripe-go/v83/webhook"
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

	resultMessage := ProcessExpiredPayment(&currentPayment, paymentAPIConfig.DB, ginContext.Request.Context())

	if strings.TrimSpace(resultMessage) != "" {
		if strings.Contains(resultMessage, "error") {
			ginContext.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process your payment, please try again in a few minutes"})
		} else {
			ginContext.JSON(http.StatusOK, gin.H{"message": "your payment have expired, please rebook your tickets again"})
		}

		return
	}

	stripe.Key = common.GetEnvVariable("STRIPE_SECRET_KEY")

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

func (paymentAPIConfig *PaymentAPIConfig) StripeWebhook(ginContext *gin.Context) {
	const MaxBodyBytes = int64(65536)
	ginContext.Request.Body = http.MaxBytesReader(ginContext.Writer, ginContext.Request.Body, MaxBodyBytes)

	payload, readPayloadError := io.ReadAll(ginContext.Request.Body)

	if readPayloadError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "failed to read stripe webhook request body"})

		return
	}

	stripeSignature := ginContext.GetHeader("Stripe-Signature")

	if strings.TrimSpace(stripeSignature) == "" {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "missing Stripe-Signature header"})

		return
	}

	stripeSigningSecret := strings.TrimSpace(common.GetEnvVariable("STRIPE_SIGNING_SECRET"))

	stripeEvent, signatureVerificationError := webhook.ConstructEvent(payload, stripeSignature, stripeSigningSecret)

	if signatureVerificationError != nil {
		log.Printf("%s", signatureVerificationError)
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "stripe signature verification failed"})

		return
	}

	var paymentIntent stripe.PaymentIntent

	unmarshalPaymentIntentError := json.Unmarshal(stripeEvent.Data.Raw, &paymentIntent)
	
	if unmarshalPaymentIntentError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "failed to unmarshal payment intent"})

		return
	}

	paymentId, parsePaymentIdError := uuid.Parse(paymentIntent.Metadata["payment_id"])

	if parsePaymentIdError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse payment id"})

		return
	}

	payment, getPaymentByIdAndPaymentIntentIdError := paymentAPIConfig.DB.GetPaymentByIdOnly(ginContext.Request.Context(), paymentId)

	if getPaymentByIdAndPaymentIntentIdError != nil {
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get payment details"})

		return
	}

	paymentResponse := PaymentResponse {
		ID: payment.ID,
		Status: payment.Status,
		ExpiresAt: payment.ExpiresAt,
	}

	if payment.Status == string(stripe.PaymentIntentStatusProcessing) {
		resultMessage := ProcessExpiredPayment(&payment, paymentAPIConfig.DB, ginContext.Request.Context())

		if strings.TrimSpace(resultMessage) != "" {
			ginContext.JSON(http.StatusMultiStatus, gin.H{"message": resultMessage})

			return
		}

		updatePaymentParams := database.UpdatePaymentParams {
			Amount: fmt.Sprintf("%.2f", float64(paymentIntent.Amount)/100.0),
			Status: string(paymentIntent.Status),
			PaymentIntentID: common.StringToNullString(paymentIntent.ID),
			ID: payment.ID,
			UserID: payment.UserID,
		}

		_, updatePaymentError := paymentAPIConfig.DB.UpdatePayment(ginContext.Request.Context(), updatePaymentParams)

		if updatePaymentError != nil {
			ginContext.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update payment"})

			return
		}

		switch stripeEvent.Type {
			case "payment_intent.succeeded":
				// TODO: Send email notification with details of ticket reservations.
				paymentResponse.Message = "payment succeeded"
			
			case "payment_intent.payment_failed":
				paymentResponse.ClientSecret = paymentIntent.ClientSecret
				paymentResponse.NextAction = string(paymentIntent.NextAction.Type)
				paymentResponse.Message = paymentIntent.LastPaymentError.Msg

			case "payment_intent.requires_action":
				paymentResponse.ClientSecret = paymentIntent.ClientSecret
				paymentResponse.NextAction = string(paymentIntent.NextAction.Type)
				paymentResponse.Message = fmt.Sprintf("please settle payment before %s", updatedPayment.ExpiresAt)
		}
	}

	log.Println(stripeEvent.Type)

	ginContext.JSON(http.StatusOK, gin.H{"payment": paymentResponse})
}