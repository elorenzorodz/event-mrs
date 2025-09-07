package events

import (
	"context"
	"fmt"
	"sync"

	"github.com/elorenzorodz/event-mrs/common"
	"github.com/elorenzorodz/event-mrs/event_details"
	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/google/uuid"
)

func DatabaseEventToEventJSON(databaseEvent database.Event, eventDetails []event_details.EventDetail) Event {
	return Event{
		ID:          databaseEvent.ID,
		Title:       databaseEvent.Title,
		Description: databaseEvent.Description,
		Organizer:   databaseEvent.Organizer.String,
		CreatedAt:   databaseEvent.CreatedAt,
		UpdatedAt:   common.NullTimeToString(databaseEvent.UpdatedAt),
		UserID:      databaseEvent.UserID,
		Tickets:	 eventDetails,
	}
}

func DatabaseEventsToEventsJSON(databaseEvents []database.Event, eventDetailsMap map[string][]event_details.EventDetail) []Event {
	events := []Event{}

	for _, databaseEvent := range databaseEvents {
		eventDetails := []event_details.EventDetail{}

		if eventDetailsMap != nil || len(eventDetailsMap) > 0 {
			eventDetails = eventDetailsMap[databaseEvent.ID.String()]
		}

		events = append(events, DatabaseEventToEventJSON(databaseEvent, eventDetails))
	}

	return events
}

func SaveEventTickets(db *database.Queries, context context.Context, eventId uuid.UUID, tickets []event_details.EventDetailParameters) ([]event_details.EventDetail, error) {
	var (
		newTickets []event_details.EventDetail
		mutex      sync.Mutex
		waitGroup  sync.WaitGroup
		errorChannel = make(chan error, len(tickets))
	)

	for _, ticket := range tickets {
		tkt := ticket // capture loop variable

		waitGroup.Go(func() {

			showDate, referenceFormat, parseShowDateError := common.StringToTime(tkt.ShowDate)

			if parseShowDateError != nil {
				errorChannel <- fmt.Errorf("error parsing show date '%s': expected format %s", tkt.ShowDate, referenceFormat)

				return
			}

			createEventDetailParams := database.CreateEventDetailParams{
				ID:               uuid.New(),
				ShowDate:         showDate,
				Price:            fmt.Sprintf("%.2f", tkt.Price),
				NumberOfTickets:  tkt.NumberOfTickets,
				TicketDescription: tkt.TicketDescription,
				EventID:          eventId,
			}

			newEventDetail, createEventDetailError := db.CreateEventDetail(context, createEventDetailParams)

			if createEventDetailError != nil {
				errorChannel <- fmt.Errorf("error creating event detail: %w", createEventDetailError)

				return
			}

			mutex.Lock()
			newTickets = append(newTickets, event_details.DatabaseEventDetailToEventDetailJSON(newEventDetail))
			mutex.Unlock()
		})
	}

	go func() {
		waitGroup.Wait()
		close(errorChannel)
	}()

	for err := range errorChannel {
		if err != nil {
			// Return empty map and error if any occurred.
			return newTickets, err
		}
	}

	return newTickets, nil
}