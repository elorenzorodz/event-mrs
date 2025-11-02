package events

import (
	"time"

	"github.com/elorenzorodz/event-mrs/event_details"
	"github.com/google/uuid"
)

type Event struct {
	ID          uuid.UUID
	UserID     uuid.UUID
	Title       string
	Description string
	Organizer   string
	CreatedAt   time.Time
	UpdatedAt   *time.Time
	Tickets     []event_details.EventDetail
}

type CreateEventRequest struct {
	Title       string                                `json:"title" binding:"required"`
	Description string                                `json:"description" binding:"required"`
	Organizer   string                                `json:"organizer"`
	Tickets     []event_details.EventDetailParameters `json:"tickets" binding:"required"`
}

type UpdateEventRequest struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description" binding:"required"`
	Organizer   string `json:"organizer"`
}

type EventResponse struct {
	ID          uuid.UUID                   `json:"id"`
	Title       string                      `json:"title"`
	Description string                      `json:"description"`
	Organizer   string                      `json:"organizer"`
	CreatedAt   time.Time                   `json:"created_at"`
	UpdatedAt   *time.Time                  `json:"updated_at"`
	UserID      uuid.UUID                   `json:"user_id"`
	Tickets     []event_details.EventDetail `json:"tickets"`
}

type SearchEventResponse struct {
	EventID           uuid.UUID `json:"event_id"`
	Title             string    `json:"title"`
	Description       string    `json:"desription"`
	Organizer         string    `json:"organizer"`
	EventDetailID     uuid.UUID `json:"event_detail_id"`
	ShowDate          time.Time `json:"show_date"`
	Price             float32   `json:"price"`
	NumberOfTickets   int32     `json:"number_of_tickets"`
	TicketDescription string    `json:"ticket_description"`
}

type EventFailedRefundOrCancel struct {
	PaymentID uuid.UUID `json:"payment_id"`
	Action    string    `json:"action"`
	Code      string    `json:"code"`
	Message   string    `json:"message"`
}

type FailedNotificationEmail struct {
	SendRefundCancelNotificationError string `json:"send_refund_cancel_notification_error"`
}

type DeleteSummary struct {
	EventFailedRefundOrCancels []EventFailedRefundOrCancel
	FailedNotificationEmails   []FailedNotificationEmail
}

func NewEventResponse(e *Event) EventResponse {
	return EventResponse{
		ID:          e.ID,
		UserID:      e.UserID,
		Title:       e.Title,
		Description: e.Description,
		Organizer:   e.Organizer,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
		Tickets:     e.Tickets,
	}
}