package events

import (
	"net/http"

	"github.com/elorenzorodz/event-mrs/common"
	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (eventAPIConfig *EventAPIConfig) CreateEvent(ginContext *gin.Context) {
	type parameters struct {
		Title       string `json:"title" binding:"required"`
		Description string `json:"description" binding:"required"`
		Organizer   string `json:"organizer"`
	}

	params := parameters{}

	// Bind incoming JSON to struct and check for errors in the process.
	if parameterBindError := ginContext.ShouldBindJSON(&params); parameterBindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "error parsing JSON, please check all required fields are present"})

		return
	}

	userId, parseUserIdError := uuid.Parse(ginContext.MustGet("userId").(string))

	if parseUserIdError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})

		return
	}

	createEventParams := database.CreateEventParams {
		ID: uuid.New(),
		Title: params.Title,
		Description: params.Description,
		Organizer: common.StringToNullString(params.Organizer),
		UserID: userId,
	}

	newEvent, createEventError := eventAPIConfig.DB.CreateEvent(ginContext, createEventParams)

	if createEventError != nil {
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": "error creating event, please try again in a few minutes"})

		return
	}

	ginContext.JSON(http.StatusCreated, gin.H{"event": newEvent})
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

	ginContext.JSON(http.StatusOK, gin.H{"events": getUserEvents})
}