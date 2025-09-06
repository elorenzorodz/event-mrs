package events

import (
	"github.com/elorenzorodz/event-mrs/common"
	"github.com/elorenzorodz/event-mrs/internal/database"
)

func DatabaseEventToEventJSON(databaseEvent database.Event) Event {
	return Event{
		ID:          databaseEvent.ID,
		Title:       databaseEvent.Title,
		Description: databaseEvent.Description,
		Organizer:   databaseEvent.Organizer.String,
		CreatedAt:   databaseEvent.CreatedAt,
		UpdatedAt:   common.NullTimeToString(databaseEvent.UpdatedAt),
		UserID:      databaseEvent.UserID,
	}
}

func DatabaseEventsToEventsJSON(databaseEvents []database.Event) []Event {
	events := []Event{}

	for _, databaseEvent := range databaseEvents {
		events = append(events, DatabaseEventToEventJSON(databaseEvent))
	}

	return events
}

// func SaveEventTickets(eventAPIConfig *EventAPIConfig, eventId string, tickets []event_details.EventDetailParameters) error {
// 	newTickets = []event_details.EventDetail{}

// 	waitGroup := &sync.WaitGroup{}

// 	for _, ticket := range tickets {
// 		waitGroup.Add(1)

// 		go func() {
// 			defer waitGroup.Done()


// 		}
// 	}

// 	waitGroup.Wait()
// }