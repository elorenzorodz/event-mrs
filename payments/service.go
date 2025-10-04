package payments

import (
	"log"

	"github.com/gin-gonic/gin"
)

func (paymentAPIConfig *PaymentAPIConfig) ProcessStripePayment(ginContext *gin.Context) {
	stripePayloadParams := StripePayloadParameters{}

	// Bind incoming JSON to struct and check for errors in the process.
	if parameterBindError := ginContext.ShouldBindJSON(&stripePayloadParams); parameterBindError != nil {
		log.Printf("error: failed to bind payload to parameters, %s", parameterBindError)

		return
	}

	// TODO: Check if payment already succeeded. Webhook triggered by paymentintent.Confirm() from ticket reservation function.
	
}