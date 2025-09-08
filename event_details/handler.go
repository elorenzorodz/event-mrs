package event_details

import (
	"github.com/elorenzorodz/event-mrs/common"
	"github.com/elorenzorodz/event-mrs/internal/database"
)

func DatabaseEventDetailToEventDetailJSON(databaseEventDetail database.EventDetail) EventDetail {
	return EventDetail{
		ID:                databaseEventDetail.ID,
		ShowDate:          databaseEventDetail.ShowDate,
		Price:             common.StringToFloat32(databaseEventDetail.Price),
		NumberOfTickets:   databaseEventDetail.NumberOfTickets,
		TicketDescription: databaseEventDetail.TicketDescription,
		CreatedAt:         databaseEventDetail.CreatedAt,
		UpdatedAt:         common.NullTimeToString(databaseEventDetail.UpdatedAt),
		EventID:           databaseEventDetail.EventID,
	}
}

func DatabaseEventDetailsToEventDetailsJSON(databaseEventDetails []database.EventDetail) []EventDetail {
	eventDetails := []EventDetail{}

	for _, databaseEventDetail := range databaseEventDetails {
		eventDetails = append(eventDetails, DatabaseEventDetailToEventDetailJSON(databaseEventDetail))
	}

	return eventDetails
}