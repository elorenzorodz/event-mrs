package event_details

import (
	"log"
	"strconv"

	"github.com/elorenzorodz/event-mrs/common"
	"github.com/elorenzorodz/event-mrs/internal/database"
)

func DatabaseEventDetailToEventDetailJSON(databaseEventDetail database.EventDetail) EventDetail {
	price, priceParseFloatError := strconv.ParseFloat(databaseEventDetail.Price, 32)

	if priceParseFloatError != nil {
		log.Printf("error parsing event detail price: %v", priceParseFloatError)

		price = 0.00
	}

	price32 := float32(price)

	return EventDetail{
		ID:                databaseEventDetail.ID,
		ShowDate:          databaseEventDetail.ShowDate,
		Price:             price32,
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