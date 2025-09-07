package event_details

import (
	"fmt"
	"net/http"

	"github.com/elorenzorodz/event-mrs/common"
	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (eventDetailAPIConfig *EventDetailAPIConfig) CreateEventDetail(ginContext *gin.Context) {
	eventId, parseEventIdError := uuid.Parse(ginContext.Param("eventId"))

	if parseEventIdError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid event ID"})

		return
	}

	eventDetailParams := EventDetailParameters{}

	// Bind incoming JSON to struct and check for errors in the process.
	if parameterBindError := ginContext.ShouldBindJSON(&eventDetailParams); parameterBindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "error parsing JSON, please check all required fields are present and/or numbers are not be quoted"})

		return
	}

	showDate, referenceShowDateFormat, parseShowDateError := common.StringToTime(eventDetailParams.ShowDate)

	if parseShowDateError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("error parsing show date, please use the following format: %s", referenceShowDateFormat)})
		
		return
	}

	createEventDetailParams := database.CreateEventDetailParams {
		ID: uuid.New(),
		ShowDate: showDate,
		Price: fmt.Sprintf("%.2f", eventDetailParams.Price),
		NumberOfTickets: eventDetailParams.NumberOfTickets,
		TicketDescription: eventDetailParams.TicketDescription,
		EventID: eventId,
	}

	newEventDetail, createEventDetailError := eventDetailAPIConfig.DB.CreateEventDetail(ginContext.Request.Context(), createEventDetailParams)

	if createEventDetailError != nil {
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error" : "error creating event detail, please try again in a few minutes"})

		return
	}

	ginContext.JSON(http.StatusCreated, gin.H{"event_detail": DatabaseEventDetailToEventDetailJSON(newEventDetail)})
}