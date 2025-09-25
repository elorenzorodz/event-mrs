package reservations

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (reservationAPIConfig *ReservationAPIConfig) CreateReservation(ginContext *gin.Context) {
	userId, parseUserIdError := uuid.Parse(ginContext.MustGet("userId").(uuid.UUID).String())

	if parseUserIdError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})

		return
	}

	reservationParams := ReservationParameters{}

	// Bind incoming JSON to struct and check for errors in the process.
	if parameterBindError := ginContext.ShouldBindJSON(&reservationParams); parameterBindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "error parsing JSON, please check all required fields are present"})

		return
	}

	userEmail := reservationParams.Email

	// Use current user's email address of email is empty in the params.
	if strings.TrimSpace(userEmail) == "" {
		userEmail = ginContext.MustGet("email").(string)
	}

	newReservations, createTicketsError := SaveReservations(reservationAPIConfig.DB, ginContext.Request.Context(), userId, userEmail, reservationParams)

	if createTicketsError != nil {
		ginContext.JSON(http.StatusMultiStatus, gin.H{"events_reserved": newReservations, "error": createTicketsError.Error()})

		return
	}

	ginContext.JSON(http.StatusCreated, gin.H{"events_reserved": newReservations})
}