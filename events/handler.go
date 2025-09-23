package events

import (
	"context"
	"fmt"
	"strings"
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

func DatabaseSearchEventsToSearchEventsJSON(databaseSearchEvents []database.GetEventsRow) []SearchEvent {
	searchEvents := []SearchEvent{}

	for _, databaseSearchEvent := range databaseSearchEvents {
		searchEvent := SearchEvent {
			EventID: databaseSearchEvent.EventID,
			Title: databaseSearchEvent.Title,
			Description: databaseSearchEvent.Description,
			Organizer: databaseSearchEvent.Organizer.String,
			EventDetailID: databaseSearchEvent.EventID,
			ShowDate: common.NullTimeToString(databaseSearchEvent.ShowDate),
			Price: common.StringToFloat32(databaseSearchEvent.Price.String),
			NumberOfTickets: databaseSearchEvent.NumberOfTickets.Int32,
			TicketDescription: databaseSearchEvent.TicketDescription.String,
		}

		searchEvents = append(searchEvents, searchEvent)
	}

	return searchEvents
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

	var allErrors []string

	for err := range errorChannel {
		if err != nil {
			allErrors = append(allErrors, err.Error())
		}
	}

	// This ensures that if there were any errors, we still return the tickets that were created successfully.
	if len(allErrors) > 0 {
		return newTickets, fmt.Errorf("encountered errors:\n%s", strings.Join(allErrors, "\n"))
	}

	return newTickets, nil
}