package event_details

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/elorenzorodz/event-mrs/common"
	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
	eventDetails := make([]EventDetail, len(databaseEventDetails))

	for i, databaseEventDetail := range databaseEventDetails {
		eventDetails[i] = DatabaseEventDetailToEventDetailJSON(databaseEventDetail)
	}

	return eventDetails
}

func (eventDetailAPIConfig *EventDetailAPIConfig) CreateEventDetail(ginContext *gin.Context) {
	eventID, parseEventIDError := uuid.Parse(ginContext.Param("eventId"))

	if parseEventIDError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid event ID"})

		return
	}

	eventDetailParams := EventDetailParameters{}
	if parameterBindError := ginContext.ShouldBindJSON(&eventDetailParams); parameterBindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "error parsing JSON, please check all required fields are present and/or numbers are not be quoted"})

		return
	}

	createdEventDetail, createEventDetailError := eventDetailAPIConfig.Service.Create(ginContext.Request.Context(), eventID, eventDetailParams)

	if createEventDetailError != nil {
		if _, _, parseError := common.StringToTime(eventDetailParams.ShowDate); parseError != nil {
			ginContext.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("error parsing show date: %v", parseError.Error())})

			return
		}
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": createEventDetailError.Error()})

		return
	}

	ginContext.JSON(http.StatusCreated, createdEventDetail)
}

func (eventDetailAPIConfig *EventDetailAPIConfig) UpdateEventDetail(ginContext *gin.Context) {
	eventID, parseEventIDError := uuid.Parse(ginContext.Param("eventId"))

	if parseEventIDError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid event ID"})

		return
	}

	eventDetailID, parseEventDetailIDError := uuid.Parse(ginContext.Param("eventDetailId"))

	if parseEventDetailIDError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid event detail ID"})

		return
	}

	eventDetailParams := EventDetailParameters{}

	if parameterBindError := ginContext.ShouldBindJSON(&eventDetailParams); parameterBindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "error parsing JSON, please check all required fields are present and/or numbers are not be quoted"})

		return
	}

	updatedEventDetail, updateEventDetailError := eventDetailAPIConfig.Service.Update(ginContext.Request.Context(), eventID, eventDetailID, eventDetailParams)

	if updateEventDetailError != nil {
		if _, _, parseError := common.StringToTime(eventDetailParams.ShowDate); parseError != nil {
			ginContext.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("error parsing show date: %v", parseError.Error())})

			return
		}

		if updateEventDetailError == sql.ErrNoRows {
			ginContext.JSON(http.StatusNotFound, gin.H{"error": "event detail not found"})

			return
		}

		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": updateEventDetailError.Error()})

		return
	}

	ginContext.JSON(http.StatusOK, updatedEventDetail)
}

func (eventDetailAPIConfig *EventDetailAPIConfig) DeleteEventDetail(ginContext *gin.Context) {
	eventID, parseEventIDError := uuid.Parse(ginContext.Param("eventId"))
	if parseEventIDError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid event ID"})
		return
	}

	eventDetailID, parseEventDetailIDError := uuid.Parse(ginContext.Param("eventDetailId"))
	if parseEventDetailIDError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid event detail ID"})
		return
	}

	userID, _ := ginContext.MustGet("userId").(uuid.UUID)
	userEmail := ginContext.MustGet("email").(string)

	eventDetailFailedRefundOrCancels, failedNotificationEmails, deleteError := eventDetailAPIConfig.Service.Delete(ginContext.Request.Context(), eventID, eventDetailID, userID, userEmail)

	if deleteError != nil {
		if deleteError == sql.ErrNoRows {
			ginContext.JSON(http.StatusNotFound, gin.H{"error": "event detail not found"})
			
			return
		}

		// Check if delete was unsuccessful but async ops occurred (MultiStatus scenario for failure)
		if len(eventDetailFailedRefundOrCancels) != 0 || len(failedNotificationEmails) != 0 {
			ginContext.JSON(http.StatusMultiStatus,
				gin.H{
					"message":                             "error deleting event detail",
					"error":                               deleteError.Error(),
					"payments_failed_refund_or_cancelled": eventDetailFailedRefundOrCancels,
					"failed_notification_emails":          failedNotificationEmails,
				})

			return
		}

		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": deleteError.Error()})

		return
	}

	if len(eventDetailFailedRefundOrCancels) != 0 || len(failedNotificationEmails) != 0 {
		ginContext.JSON(http.StatusMultiStatus,
			gin.H{
				"message":                             "event detail deleted successfully",
				"payments_failed_refund_or_cancelled": eventDetailFailedRefundOrCancels,
				"failed_notification_emails":          failedNotificationEmails,
			})

		return
	}

	ginContext.JSON(http.StatusOK, gin.H{"message": "event detail deleted successfully"})
}