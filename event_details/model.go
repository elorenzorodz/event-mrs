package event_details

import (
	"time"

	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/google/uuid"
)

type EventDetailAPIConfig struct {
	DB *database.Queries
}

type EventDetail struct {
	ID                uuid.UUID `json:"id"`
	ShowDate          time.Time `json:"show_date"`
	Price             float32   `json:"price"`
	NumberOfTickets   int32     `json:"number_of_tickets"`
	TicketDescription string    `json:"ticket_description"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         string    `json:"updated_at"`
	EventID           uuid.UUID `json:"event_id"`
}

type EventDetailParameters struct {
	ShowDate          string  `json:"show_date" binding:"required"`
	TicketDescription string  `json:"description" binding:"required"`
	Price             float32 `json:"price"`
	NumberOfTickets   int32   `json:"number_of_tickets" binding:"required"`
}