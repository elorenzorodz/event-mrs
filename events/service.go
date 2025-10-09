package events

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/elorenzorodz/event-mrs/common"
	"github.com/elorenzorodz/event-mrs/event_details"
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

	createEventParams := database.CreateEventParams{
		ID:          uuid.New(),
		Title:       eventParams.Title,
		Description: eventParams.Description,
		Organizer:   common.StringToNullString(eventParams.Organizer),
		UserID:      userId,
	}

	newEvent, createEventError := eventAPIConfig.DB.CreateEvent(ginContext.Request.Context(), createEventParams)

	if createEventError != nil {
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": "error creating event, please try again in a few minutes"})

		return
	}

	// Save ticket details.
	newTickets, createTicketsError := SaveEventTickets(eventAPIConfig.DB, ginContext.Request.Context(), newEvent.ID, eventParams.Tickets)

	if createTicketsError != nil {
		ginContext.JSON(http.StatusMultiStatus, gin.H{"event": DatabaseEventToEventJSON(newEvent, newTickets), "error": "error creating some details/tickets, please create separately the event details/tickets"})

		return
	}

	ginContext.JSON(http.StatusCreated, gin.H{"event": DatabaseEventToEventJSON(newEvent, newTickets)})
}

func (eventAPIConfig *EventAPIConfig) GetUserEvents(ginContext *gin.Context) {
	userId, parseUserIdError := uuid.Parse(ginContext.MustGet("userId").(uuid.UUID).String())

	if parseUserIdError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})

		return
	}

	userEvents, getUserEventsError := eventAPIConfig.DB.GetUserEvents(ginContext.Request.Context(), userId)

	if getUserEventsError != nil {
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": "error retrieving user events, please try again in a few minutes"})

		return
	}

	eventIdArray := []uuid.UUID{}

	for _, event := range userEvents {
		eventIdArray = append(eventIdArray, event.ID)
	}

	eventDetails, getEventDetailsError := eventAPIConfig.DB.GetEventDetailsByEventId(ginContext.Request.Context(), eventIdArray)
	
	// Get event details from all fetched events.
	eventDetailsMap := make(map[string][]event_details.EventDetail)

	if getEventDetailsError != nil {
		log.Printf("error retrieving event details: %v", getEventDetailsError)
	} else {
		for _, detail := range eventDetails {
			eventDetailsMap[detail.EventID.String()] = append(eventDetailsMap[detail.EventID.String()], event_details.EventDetail{
				ID:                detail.ID,
				ShowDate:          detail.ShowDate,
				Price:             common.StringToFloat32(detail.Price),
				NumberOfTickets:   detail.NumberOfTickets,
				TicketDescription: detail.TicketDescription,
				CreatedAt:         detail.CreatedAt,
				UpdatedAt:         common.NullTimeToString(detail.UpdatedAt),
				EventID:           detail.EventID,
			})
		}
	}

	ginContext.JSON(http.StatusOK, gin.H{"events": DatabaseEventsToEventsJSON(userEvents, eventDetailsMap)})
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

	getUserEventByIdParams := database.GetUserEventByIdParams{
		ID:     eventId,
		UserID: userId,
	}

	getUserEvent, getUserEventByIdError := eventAPIConfig.DB.GetUserEventById(ginContext.Request.Context(), getUserEventByIdParams)

	if getUserEventByIdError != nil {
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": "error retrieving user event, please try again in a few minutes"})

		return
	}

	eventIdArray := []uuid.UUID{getUserEvent.ID}

	eventDetails, getEventDetailsError := eventAPIConfig.DB.GetEventDetailsByEventId(ginContext.Request.Context(), eventIdArray)
	
	// Get event details from all fetched events.
	eventDetailsList := []event_details.EventDetail{}

	if getEventDetailsError != nil {
		log.Printf("error retrieving event details: %v", getEventDetailsError)
	} else {
		for _, detail := range eventDetails {
			eventDetailsList = append(eventDetailsList, event_details.EventDetail{
				ID:                detail.ID,
				ShowDate:          detail.ShowDate,
				Price:             common.StringToFloat32(detail.Price),
				NumberOfTickets:   detail.NumberOfTickets,
				TicketDescription: detail.TicketDescription,
				CreatedAt:         detail.CreatedAt,
				UpdatedAt:         common.NullTimeToString(detail.UpdatedAt),
				EventID:           detail.EventID,
			})
		}
	}

	ginContext.JSON(http.StatusOK, gin.H{"event": DatabaseEventToEventJSON(getUserEvent, eventDetailsList)})
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

	updateEventParams := database.UpdateEventParams{
		ID:          eventId,
		Title:       eventParams.Title,
		Description: eventParams.Description,
		Organizer:   common.StringToNullString(eventParams.Organizer),
		UserID:      userId,
	}

	updatedEvent, updatedEventError := eventAPIConfig.DB.UpdateEvent(ginContext.Request.Context(), updateEventParams)

	if updatedEventError != nil {
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": "error updating event, please try again in a few minutes"})

		return
	}

	ginContext.JSON(http.StatusOK, gin.H{"event": DatabaseEventToEventJSON(updatedEvent, []event_details.EventDetail{})})
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

	userEmail := ginContext.MustGet("email").(string)

	eventFailedRefundOrCancels, failedNotificationEmails, refundCancelPaymentErrors := RefundOrCancelPayment(eventAPIConfig.DB, ginContext.Request.Context(), eventId, userId, userEmail)

	if refundCancelPaymentErrors != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": refundCancelPaymentErrors})

		return
	}

	deleteEventParams := database.DeleteEventParams {
		ID:     eventId,
		UserID: userId,
	}

	deleteEventError := eventAPIConfig.DB.DeleteEvent(ginContext.Request.Context(), deleteEventParams)

	if deleteEventError != nil {
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": "error deleting event, please try again in a few minutes"})

		return
	}

	if len(eventFailedRefundOrCancels) != 0 || len(failedNotificationEmails) != 0 {
		ginContext.JSON(http.StatusMultiStatus, gin.H{"message": "event deleted successfully", "payments_failed_refund_or_cancelled": eventFailedRefundOrCancels, "failed_refund_cancelled_notif_emails": failedNotificationEmails})

		return
	}

	ginContext.JSON(http.StatusOK, gin.H{"message": "event deleted successfully"})
}

func (eventAPIConfig *EventAPIConfig) GetEvents(ginContext *gin.Context) {
	searchQuery := ginContext.Query("search")

	if strings.TrimSpace(searchQuery) == "" {
		searchQuery = "%%"
	} else {
		searchQuery = fmt.Sprintf("%s%s%s", "%", strings.ToLower(searchQuery), "%")
	}
	
	startShowDateQuery := ginContext.Query("startShowDate")
	endShowDateQuery := ginContext.Query("endShowDate")

	var startShowDate time.Time
	var endShowDate time.Time

	currentDateTime := time.Now().UTC()

	if strings.TrimSpace(startShowDateQuery) == "" {
		startShowDate = time.Date(currentDateTime.Year(), currentDateTime.Month(), currentDateTime.Day(), 0, 0, 0, 0, currentDateTime.Location())
	} else {
		// Append HH:mm:ss to avoid error with common.StringToTime()
		startShowDateQuery = fmt.Sprintf("%s 00:00", startShowDateQuery)

		parsedShowDate, referenceShowDateFormat, parseShowDateError := common.StringToTime(startShowDateQuery)

		if parseShowDateError != nil {
			ginContext.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("error parsing show date '%s': expected format %s", startShowDateQuery, referenceShowDateFormat)})

			return
		}

		startShowDate = parsedShowDate
	}

	if strings.TrimSpace(endShowDateQuery) == "" {
		// First day of next month
		firstDayOfNextMonth := time.Date(currentDateTime.Year(), currentDateTime.Month() + 1, 1, 0, 0, 0, 0, currentDateTime.Location())

		// Subtract one day to get the last day of current month.
		lastDayOfCurrentMonth := firstDayOfNextMonth.AddDate(0, 0, -1)

		endShowDate = time.Date(currentDateTime.Year(), currentDateTime.Month(), lastDayOfCurrentMonth.Day(), 23, 59, 59, 0, currentDateTime.Location())
	} else {
		// Append HH:mm:ss to avoid error with common.StringToTime()
		endShowDateQuery = fmt.Sprintf("%s 23:59", endShowDateQuery)

		parsedEndDate, referenceShowDateFormat, parseEndDateError := common.StringToTime(endShowDateQuery)

		if parseEndDateError != nil {
			ginContext.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("error parsing show date '%s': expected format %s", endShowDateQuery, referenceShowDateFormat)})

			return
		}

		endShowDate = parsedEndDate
	}

	getEventsParam := database.GetEventsParams {
		Title: searchQuery,
		Description: searchQuery,
		Organizer: common.StringToNullString(searchQuery),
		ShowDate: startShowDate,
		ShowDate_2: endShowDate,
	}

	getSearchEvents, getEventsError := eventAPIConfig.DB.GetEvents(ginContext.Request.Context(), getEventsParam)

	if getEventsError != nil {
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": "error searching events, please try again in a few minutes"})

		return
	}

	ginContext.JSON(http.StatusOK, DatabaseSearchEventsToSearchEventsJSON(getSearchEvents))
}