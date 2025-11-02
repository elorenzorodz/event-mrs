package events

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type EventAPIConfig struct {
	Service EventService
}

func getOwnerIDFromContext(ginContext *gin.Context) (uuid.UUID, error) {
	ownerID, exists := ginContext.Get("userId")

	if !exists {
		return uuid.Nil, errors.New("user ID not found in context (middleware missing)")
	}
	
	ownerIDUUID, ok := ownerID.(uuid.UUID)

	if !ok {
        if ownerIDStr, isStr := ownerID.(string); isStr {
            return uuid.Parse(ownerIDStr)
        }

        return uuid.Nil, errors.New("user ID in context is not a UUID or a string")
	}

	return ownerIDUUID, nil
}

func (eventAPIConfig *EventAPIConfig) CreateEvent(ginContext *gin.Context) {
	var createEventRequest CreateEventRequest
	
	if err := ginContext.ShouldBindJSON(&createEventRequest); err != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "error parsing JSON, please check all required fields are present"})

		return
	}

	ownerID, getOwnerIDError := getOwnerIDFromContext(ginContext)

	if getOwnerIDError != nil {
		ginContext.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user ID from context"})

		return
	}

	event, createEventError := eventAPIConfig.Service.Create(ginContext.Request.Context(), ownerID, createEventRequest)
	
	if createEventError != nil {
		if strings.Contains(createEventError.Error(), "encountered errors") {
			ginContext.JSON(http.StatusMultiStatus, gin.H{"event": NewEventResponse(event), "error": fmt.Sprintf("error creating some details/tickets: %v", createEventError.Error())})

			return
		}
		
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": "error creating event, please try again in a few minutes"})

		return
	}

	ginContext.JSON(http.StatusCreated, gin.H{"event": NewEventResponse(event)})
}

func (eventAPIConfig *EventAPIConfig) GetUserEvents(ginContext *gin.Context) {
	ownerID, getOwnerIDError := getOwnerIDFromContext(ginContext)

	if getOwnerIDError != nil {
		ginContext.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user ID from context"})

		return
	}

	events, getEventsByOwnerError := eventAPIConfig.Service.GetEventsByOwner(ginContext.Request.Context(), ownerID)

	if getEventsByOwnerError != nil {
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": "error retrieving user events, please try again in a few minutes"})

		return
	}
	
	responseEvents := make([]EventResponse, len(events))

	for index, event := range events {
		responseEvents[index] = NewEventResponse(&event)
	}

	ginContext.JSON(http.StatusOK, gin.H{"events": responseEvents})
}

func (eventAPIConfig *EventAPIConfig) GetUserEventById(ginContext *gin.Context) {
	eventID, parseEventIdError := uuid.Parse(ginContext.Param("eventId"))

	if parseEventIdError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid event ID"})

		return
	}

	ownerID, getEventsByOwnerError := getOwnerIDFromContext(ginContext)

	if getEventsByOwnerError != nil {
		ginContext.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user ID from context"})

		return
	}

	event, getEventByIdError := eventAPIConfig.Service.GetEventByID(ginContext.Request.Context(), eventID, ownerID)

	if getEventByIdError != nil {
		if errors.Is(getEventByIdError, ErrEventNotFound) {
			ginContext.JSON(http.StatusNotFound, gin.H{"error": "event not found or not owned by user"})

			return
		}

		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": "error retrieving user event, please try again in a few minutes"})

		return
	}

	ginContext.JSON(http.StatusOK, gin.H{"event": NewEventResponse(event)})
}

func (eventAPIConfig *EventAPIConfig) UpdateEvent(ginContext *gin.Context) {
	eventID, parseEventIdError := uuid.Parse(ginContext.Param("eventId"))

	if parseEventIdError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid event ID"})

		return
	}

	var updateEventRequest UpdateEventRequest

	if err := ginContext.ShouldBindJSON(&updateEventRequest); err != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "error parsing JSON, please check all required fields are present"})

		return
	}

	ownerID, getEventsByOwnerError := getOwnerIDFromContext(ginContext)

	if getEventsByOwnerError != nil {
		ginContext.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user ID from context"})

		return
	}

	updatedEvent, err := eventAPIConfig.Service.Update(ginContext.Request.Context(), eventID, ownerID, updateEventRequest)

	if err != nil {
		if errors.Is(err, ErrEventNotFound) {
			ginContext.JSON(http.StatusNotFound, gin.H{"error": "event not found or not owned by user"})

			return
		}
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": "error updating event, please try again in a few minutes"})

		return
	}
	
	ginContext.JSON(http.StatusOK, gin.H{"event": NewEventResponse(updatedEvent)})
}

func (eventAPIConfig *EventAPIConfig) DeleteEvent(ginContext *gin.Context) {
	eventID, parseEventIdError := uuid.Parse(ginContext.Param("eventId"))

	if parseEventIdError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid event ID"})
		
		return
	}

	ownerID, getEventsByOwnerError := getOwnerIDFromContext(ginContext)
	if getEventsByOwnerError != nil {
		ginContext.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user ID from context"})

		return
	}
	
	userEmail, ok := ginContext.MustGet("email").(string)

	if !ok {
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": "user email not found in context"})

		return
	}

	summary, deleteEventError := eventAPIConfig.Service.Delete(ginContext.Request.Context(), eventID, ownerID, userEmail)

	if deleteEventError != nil {
		if errors.Is(deleteEventError, ErrEventNotFound) {
			ginContext.JSON(http.StatusNotFound, gin.H{"error": "event not found or not owned by user"})
			return
		}
		
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": "error deleting event, please try again in a few minutes"})

		return
	}

	if len(summary.EventFailedRefundOrCancels) != 0 || len(summary.FailedNotificationEmails) != 0 {
		ginContext.JSON(http.StatusMultiStatus, gin.H{
			"message": "event deleted successfully",
			"payments_failed_refund_or_cancelled": summary.EventFailedRefundOrCancels,
			"failed_refund_cancelled_notif_emails": summary.FailedNotificationEmails,
		})

		return
	}

	ginContext.JSON(http.StatusOK, gin.H{"message": "event deleted successfully"})
}

func (eventAPIConfig *EventAPIConfig) GetEvents(ginContext *gin.Context) {
	searchQuery := ginContext.Query("search")
	startShowDateQuery := ginContext.Query("startShowDate")
	endShowDateQuery := ginContext.Query("endShowDate")

	searchEvents, searchEventsError := eventAPIConfig.Service.SearchEvents(ginContext.Request.Context(), searchQuery, startShowDateQuery, endShowDateQuery)

	if searchEventsError != nil {
		if strings.Contains(searchEventsError.Error(), "invalid") {
			ginContext.JSON(http.StatusBadRequest, gin.H{"error": searchEventsError.Error()})

			return
		}

		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": "error searching events, please try again in a few minutes"})
		
		return
	}

	ginContext.JSON(http.StatusOK, gin.H{"events": searchEvents})
}