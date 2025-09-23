package events

import (
	"time"

	"github.com/elorenzorodz/event-mrs/event_details"
	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/google/uuid"
)

type EventAPIConfig struct {
	DB *database.Queries
}

type Event struct {
	ID          uuid.UUID                   `json:"id"`
	Title       string                      `json:"title"`
	Description string                      `json:"description"`
	Organizer   string                      `json:"organizer"`
	CreatedAt   time.Time                   `json:"created_at"`
	UpdatedAt   string                      `json:"updated_at"`
	UserID      uuid.UUID                   `json:"user_id"`
	Tickets     []event_details.EventDetail `json:"tickets"`
}

type SearchEvent struct {
	EventID           uuid.UUID `json:"event_id"`
	Title             string    `json:"title"`
	Description       string    `json:"desription"`
	Organizer         string    `json:"organizer"`
	EventDetailID     uuid.UUID `json:"event_detail_id"`
	ShowDate          string `json:"show_date"`
	Price             float32   `json:"price"`
	NumberOfTickets   int32     `json:"number_of_tickets"`
	TicketDescription string    `json:"ticket_description"`
}

type EventParameters struct {
	Title       string                                `json:"title" binding:"required"`
	Description string                                `json:"description" binding:"required"`
	Organizer   string                                `json:"organizer"`
	Tickets     []event_details.EventDetailParameters `json:"tickets"  binding:"required"`
}