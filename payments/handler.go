package payments

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (paymentAPIConfig *PaymentAPIConfig) UpdatePayment(ginContext *gin.Context) {
	userID := ginContext.MustGet("userId").(uuid.UUID)

	paymentID, parsePaymentIDError := uuid.Parse(ginContext.Param("paymentId"))

	if parsePaymentIDError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment ID"})

		return
	}

	paymentParams := PaymentParameters{}

	if parameterBindError := ginContext.ShouldBindJSON(&paymentParams); parameterBindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "error parsing JSON"})

		return
	}

	paymentResponse, updateError := paymentAPIConfig.Service.UpdatePayment(
		ginContext.Request.Context(),
		paymentID,
		userID,
		paymentParams.PaymentMethodID,
	)

	if updateError != nil {
		if errors.Is(updateError, ErrNotFound) {
			ginContext.JSON(http.StatusNotFound, gin.H{"error": "payment not found or unauthorized"})

			return
		}

		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": updateError.Error()})

		return
	}

	ginContext.JSON(http.StatusOK, paymentResponse)
}

func (paymentAPIConfig *PaymentAPIConfig) RefundPayment(ginContext *gin.Context) {
	userID := ginContext.MustGet("userId").(uuid.UUID)

	paymentID, parsePaymentIDError := uuid.Parse(ginContext.Param("paymentId"))

	if parsePaymentIDError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment ID"})

		return
	}

	refundResponse, refundError := paymentAPIConfig.Service.RefundPayment(ginContext.Request.Context(), paymentID, userID)

	if refundError != nil {
		if errors.Is(refundError, ErrNotFound) {
			ginContext.JSON(http.StatusNotFound, gin.H{"error": "payment not found or unauthorized"})

			return
		}
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": refundError.Error()})

		return
	}

	ginContext.JSON(http.StatusOK, refundResponse)
}

func (paymentAPIConfig *PaymentAPIConfig) HandleStripeWebhook(ginContext *gin.Context) {
	const MaxBodyBytes = int64(65536) // 64KB limit.
	ginContext.Request.Body = http.MaxBytesReader(ginContext.Writer, ginContext.Request.Body, MaxBodyBytes)
	
	payload, err := io.ReadAll(ginContext.Request.Body)

	if err != nil {
		ginContext.JSON(http.StatusServiceUnavailable, gin.H{"error": fmt.Sprintf("failed to read request body: %v", err)})
		
		return
	}

	signature := ginContext.GetHeader("Stripe-Signature")
	
	if err := paymentAPIConfig.Service.HandleWebhook(ginContext.Request.Context(), payload, signature, "payment"); err != nil {
		if strings.Contains(err.Error(), "Stripe signature") {
			ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid signature"})
			
			return
		}
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})

		return
	}

	ginContext.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (paymentAPIConfig *PaymentAPIConfig) HandleStripeRefundWebhook(ginContext *gin.Context) {
	const MaxBodyBytes = int64(65536) // 64KB limit.
	ginContext.Request.Body = http.MaxBytesReader(ginContext.Writer, ginContext.Request.Body, MaxBodyBytes)
	
	payload, err := io.ReadAll(ginContext.Request.Body)

	if err != nil {
		ginContext.JSON(http.StatusServiceUnavailable, gin.H{"error": fmt.Sprintf("failed to read request body: %v", err)})
		
		return
	}

	signature := ginContext.GetHeader("Stripe-Signature")
	
	if err := paymentAPIConfig.Service.HandleWebhook(ginContext.Request.Context(), payload, signature, "refund"); err != nil {
		if strings.Contains(err.Error(), "Stripe signature") {
			ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid signature"})
			
			return
		}
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})

		return
	}

	ginContext.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (paymentAPIConfig *PaymentAPIConfig) GetUserPayments(ginContext *gin.Context) {
	userID := ginContext.MustGet("userId").(uuid.UUID)

	payments, err := paymentAPIConfig.Service.GetUserPayments(ginContext.Request.Context(), userID)

	if err != nil {
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})

		return
	}

	ginContext.JSON(http.StatusOK, payments)
}

func (paymentAPIConfig *PaymentAPIConfig) GetUserPaymentById(ginContext *gin.Context) {
	userID := ginContext.MustGet("userId").(uuid.UUID)

	paymentID, parsePaymentIDError := uuid.Parse(ginContext.Param("paymentId"))

	if parsePaymentIDError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment ID"})

		return
	}

	payment, err := paymentAPIConfig.Service.GetUserPaymentById(ginContext.Request.Context(), paymentID, userID)

	if err != nil {
		if errors.Is(err, ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
			ginContext.JSON(http.StatusNotFound, gin.H{"error": "payment not found or unauthorized"})

			return
		}
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})

		return
	}

	ginContext.JSON(http.StatusOK, payment)
}