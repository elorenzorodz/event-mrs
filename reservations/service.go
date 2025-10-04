package reservations

import (
	"net/http"
	"strings"

	"github.com/elorenzorodz/event-mrs/internal/database"
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

	newReservations, paymentResponse, createTicketsError := SaveReservations(reservationAPIConfig.DB, ginContext.Request.Context(), userId, userEmail, reservationParams)

	if createTicketsError != nil {
		ginContext.JSON(http.StatusMultiStatus, gin.H{"events_reserved": newReservations, "payment": paymentResponse, "error": createTicketsError.Error()})

		return
	}

	ginContext.JSON(http.StatusCreated, gin.H{"events_reserved": newReservations, "payment": paymentResponse})
}

func (reservationAPIConfig *ReservationAPIConfig) GetUserReservations(ginContext *gin.Context) {
	userId, parseUserIdError := uuid.Parse(ginContext.MustGet("userId").(uuid.UUID).String())

	if parseUserIdError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})

		return
	}

	userReservations, getUserReservationsError := reservationAPIConfig.DB.GetUserReservations(ginContext.Request.Context(), userId)

	if getUserReservationsError != nil {
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": getUserReservationsError.Error()})

		return
	}

	ginContext.JSON(http.StatusOK, gin.H{"reservations": DatabaseReservationsToReservationsJSON(userReservations)})
}

func (reservationAPIConfig *ReservationAPIConfig) GetUserReservationById(ginContext *gin.Context) {
	userId, parseUserIdError := uuid.Parse(ginContext.MustGet("userId").(uuid.UUID).String())

	if parseUserIdError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})

		return
	}

	reservationId, parseReservationIdError := uuid.Parse(ginContext.Param("reservationId"))

	if parseReservationIdError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid reservation ID"})

		return
	}

	getUserReservationByIdParams := database.GetUserReservationByIdParams {
		ID: reservationId,
		UserID: userId,
	}

	userReservation, getUserReservationByIdError := reservationAPIConfig.DB.GetUserReservationById(ginContext.Request.Context(), getUserReservationByIdParams)

	if getUserReservationByIdError != nil {
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": getUserReservationByIdError.Error()})

		return
	}

	ginContext.JSON(http.StatusOK, gin.H{"reservation": DatabaseReservationToReservationJSON(userReservation)})
}

func (reservationAPIConfig *ReservationAPIConfig) UpdateReservationEmail(ginContext *gin.Context) {
	userId, parseUserIdError := uuid.Parse(ginContext.MustGet("userId").(uuid.UUID).String())

	if parseUserIdError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})

		return
	}

	reservationId, parseReservationIdError := uuid.Parse(ginContext.Param("reservationId"))

	if parseReservationIdError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid reservation ID"})

		return
	}

	patchReservationParams := PatchReservationParameters{}

	// Bind incoming JSON to struct and check for errors in the process.
	if parameterBindError := ginContext.ShouldBindJSON(&patchReservationParams); parameterBindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "error parsing JSON, please check all required fields are present"})

		return
	}

	updateUserReservationEmailParams := database.UpdateUserReservationEmailParams {
		Email: patchReservationParams.Email,
		ID: reservationId,
		UserID: userId,
	}

	updatedUserReservation, updateReservationEmailError := reservationAPIConfig.DB.UpdateUserReservationEmail(ginContext.Request.Context(), updateUserReservationEmailParams)

	if updateReservationEmailError != nil {
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": updateReservationEmailError.Error()})

		return
	}

	ginContext.JSON(http.StatusOK, gin.H{"reservation": DatabaseReservationToReservationJSON(updatedUserReservation)})
}