package reservations

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v83"
)

func (reservationAPIConfig *ReservationAPIConfig) CreateReservation(ginContext *gin.Context) {
	reservationParams := ReservationParameters{}

	if parameterBindError := ginContext.ShouldBindJSON(&reservationParams); parameterBindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "error parsing JSON, please check all required fields are present"})

		return
	}

	userID := ginContext.MustGet("userId").(uuid.UUID)
	userEmail := ginContext.MustGet("email").(string)

	reqEmail := reservationParams.Email

	if strings.TrimSpace(reqEmail) == "" {
		reqEmail = userEmail
	}

	reservations, paymentResponse, createError := reservationAPIConfig.Service.CreateReservations(ginContext.Request.Context(), userID, reqEmail, reservationParams)

	if createError != nil {
		status := http.StatusInternalServerError

		if errors.Is(createError, ErrInsufficientTickets) || strings.Contains(createError.Error(), "not found") {
			status = http.StatusConflict
		} else if strings.Contains(createError.Error(), "required") || strings.Contains(createError.Error(), "invalid") {
			status = http.StatusBadRequest
		}

		ginContext.JSON(status, gin.H{"error": createError.Error()})

		return
	}

	responseStatus := http.StatusCreated

	switch paymentResponse.Status {
	case string(stripe.PaymentIntentStatusRequiresAction), string(stripe.PaymentIntentStatusRequiresPaymentMethod):
		responseStatus = http.StatusAccepted
	}

	ginContext.JSON(responseStatus, gin.H{"reservations": reservations, "payment_status": paymentResponse})
}

func (reservationAPIConfig *ReservationAPIConfig) GetUserReservations(ginContext *gin.Context) {
	userID := ginContext.MustGet("userId").(uuid.UUID)

	reservations, getReservationsError := reservationAPIConfig.Service.GetUserReservations(ginContext.Request.Context(), userID)

	if getReservationsError != nil {
		if errors.Is(getReservationsError, ErrInternalError) {
			ginContext.JSON(http.StatusInternalServerError, gin.H{"error": "error fetching reservations"})

			return
		}
		
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": getReservationsError.Error()})

		return
	}

	ginContext.JSON(http.StatusOK, reservations)
}

func (reservationAPIConfig *ReservationAPIConfig) GetUserReservationById(ginContext *gin.Context) {
	userID := ginContext.MustGet("userId").(uuid.UUID)

	reservationID, parseReservationIDError := uuid.Parse(ginContext.Param("reservationId"))

	if parseReservationIDError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid reservation ID"})

		return
	}

	reservation, getReservationError := reservationAPIConfig.Service.GetUserReservationByID(ginContext.Request.Context(), reservationID, userID)

	if getReservationError != nil {
		if errors.Is(getReservationError, sql.ErrNoRows) {
			ginContext.JSON(http.StatusNotFound, gin.H{"error": "reservation not found"})

			return
		}

		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": getReservationError.Error()})

		return
	}

	ginContext.JSON(http.StatusOK, reservation)
}

func (reservationAPIConfig *ReservationAPIConfig) UpdateReservationEmail(ginContext *gin.Context) {
	userID := ginContext.MustGet("userId").(uuid.UUID)

	reservationID, parseReservationIDError := uuid.Parse(ginContext.Param("reservationId"))

	if parseReservationIDError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid reservation ID"})

		return
	}

	patchReservationParams := PatchReservationParameters{}
	if parameterBindError := ginContext.ShouldBindJSON(&patchReservationParams); parameterBindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "error parsing JSON, please check all required fields are present"})

		return
	}

	updatedReservation, updateError := reservationAPIConfig.Service.UpdateReservationEmail(
		ginContext.Request.Context(),
		reservationID,
		userID,
		patchReservationParams.Email,
	)

	if updateError != nil {
		if errors.Is(updateError, sql.ErrNoRows) {
			ginContext.JSON(http.StatusNotFound, gin.H{"error": "reservation not found or unauthorized"})

			return
		}

		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": updateError.Error()})

		return
	}

	ginContext.JSON(http.StatusOK, updatedReservation)
}