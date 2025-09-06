package events

import (
	"net/http"

	"github.com/elorenzorodz/event-mrs/common"
	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (eventAPIConfig *EventAPIConfig) CreateEvent(ginContext *gin.Context) {
	eventParams := EventParameters{}

	// Bind incoming JSON to struct and check for errors in the process.
	if parameterBindError := ginContext.ShouldBindJSON(&eventParams); parameterBindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "error parsing JSON, please check all required fields are present"})

		return
	}

	userId, parseUserIdError := uuid.Parse(ginContext.MustGet("userId").(uuid.UUID).String())

	if parseUserIdError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})

		return
	}

	createEventParams := database.CreateEventParams {
		ID: uuid.New(),
		Title: eventParams.Title,
		Description: eventParams.Description,
		Organizer: common.StringToNullString(eventParams.Organizer),
		UserID: userId,
	}

	newEvent, createEventError := eventAPIConfig.DB.CreateEvent(ginContext, createEventParams)

	if createEventError != nil {
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": "error creating event, please try again in a few minutes"})

		return
	}

	ginContext.JSON(http.StatusCreated, gin.H{"event": DatabaseEventToEventJSON(newEvent)})
}

func (eventAPIConfig *EventAPIConfig) GetUserEvents(ginContext *gin.Context) {
	userId, parseUserIdError := uuid.Parse(ginContext.MustGet("userId").(uuid.UUID).String())

	if parseUserIdError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})

		return
	}

	getUserEvents, getUserEventsError := eventAPIConfig.DB.GetUserEvents(ginContext, userId)

	if getUserEventsError != nil {
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": "error retrieving user events, please try again in a few minutes"})

		return
	}

	ginContext.JSON(http.StatusOK, gin.H{"events": DatabaseEventsToEventsJSON(getUserEvents)})
}

func (eventAPIConfig *EventAPIConfig) GetUserEventById(ginContext *gin.Context) {
	eventId, parseEventIdError := uuid.Parse(ginContext.Param("eventId"))

	if parseEventIdError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid event ID"})

		return
	}

	userId, parseUserIdError := uuid.Parse(ginContext.MustGet("userId").(uuid.UUID).String())

	if parseUserIdError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})

		return
	}

	getUserEventByIdParams := database.GetUserEventByIdParams {
		ID: eventId,
		UserID: userId,
	}

	getUserEvent, getUserEventByIdError := eventAPIConfig.DB.GetUserEventById(ginContext, getUserEventByIdParams)

	if getUserEventByIdError != nil {
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": "error retrieving user event, please try again in a few minutes"})

		return
	}

	ginContext.JSON(http.StatusOK, gin.H{"event": DatabaseEventToEventJSON(getUserEvent)})
}

func (eventAPIConfig *EventAPIConfig) UpdateEvent(ginContext *gin.Context) {
	eventId, parseEventIdError := uuid.Parse(ginContext.Param("eventId"))

	if parseEventIdError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid event ID"})

		return
	}

	eventParams := EventParameters{}

	// Bind incoming JSON to struct and check for errors in the process.
	if parameterBindError := ginContext.ShouldBindJSON(&eventParams); parameterBindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "error parsing JSON, please check all required fields are present"})

		return
	}

	userId, parseUserIdError := uuid.Parse(ginContext.MustGet("userId").(uuid.UUID).String())

	if parseUserIdError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})

		return
	}

	updateEventParams := database.UpdateEventParams {
		ID: eventId,
		Title: eventParams.Title,
		Description: eventParams.Description,
		Organizer: common.StringToNullString(eventParams.Organizer),
		UserID: userId,
	}

	updatedEvent, updatedEventError := eventAPIConfig.DB.UpdateEvent(ginContext, updateEventParams)

	if updatedEventError != nil {
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": "error updating event, please try again in a few minutes"})

		return
	}

	ginContext.JSON(http.StatusOK, gin.H{"event": DatabaseEventToEventJSON(updatedEvent)})
}

func (eventAPIConfig *EventAPIConfig) DeleteEvent(ginContext *gin.Context) {
	eventId, parseEventIdError := uuid.Parse(ginContext.Param("eventId"))

	if parseEventIdError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid event ID"})

		return
	}

	userId, parseUserIdError := uuid.Parse(ginContext.MustGet("userId").(uuid.UUID).String())

	if parseUserIdError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})

		return
	}

	deleteEventParams := database.DeleteEventParams {
		ID: eventId,
		UserID: userId,
	}

	deleteEventError := eventAPIConfig.DB.DeleteEvent(ginContext, deleteEventParams)

	if deleteEventError != nil {
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": "error deleting event, please try again in a few minutes"})

		return
	}

	ginContext.JSON(http.StatusOK, gin.H{"message": "event deleted successfully"})
}