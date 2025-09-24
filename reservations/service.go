package reservations

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func (reservationAPIConfig *ReservationAPIConfig) CreateReservation(ginContext *gin.Context) {
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

	// Save ticket details.
	newReservations, createTicketsError := SaveReservations(reservationAPIConfig.DB, ginContext.Request.Context(), userEmail, reservationParams)

	if createTicketsError != nil {
		ginContext.JSON(http.StatusMultiStatus, gin.H{"events_reserved": newReservations, "error": "error creating to some reservations, please reserve separately the failed reservations"})

		return
	}

	ginContext.JSON(http.StatusCreated, gin.H{"events_reserved": newReservations})
}