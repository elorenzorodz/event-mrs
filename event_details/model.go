package event_details

import (
	"context"
	"time"

	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/elorenzorodz/event-mrs/internal/mailer"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v83"
)

type EventDetailAPIConfig struct {
	Service EventDetailService
}

type EventDetailService interface {
	Create(ctx context.Context, eventID uuid.UUID, req EventDetailParameters) (*EventDetail, error)
	Update(ctx context.Context, eventID, eventDetailID uuid.UUID, req EventDetailParameters) (*EventDetail, error)
	Delete(ctx context.Context, eventID, eventDetailID, ownerID uuid.UUID, userEmail string) ([]EventDetailFailedRefundOrCancel, []FailedNotificationEmail, error)
}

type Service struct {
	DBQueries database.Queries
	Stripe StripeClient
	Mailer    *mailer.Mailer
}

type StripeClient interface {
	Refund(amount int64, paymentIntentID string) (*stripe.Refund, error)
	CancelPaymentIntent(paymentIntentID string, cancellationReason string) error
}

type StripeAPIClient struct{}

type EventDetail struct {
	ID                uuid.UUID `json:"id"`
	ShowDate          time.Time `json:"show_date"`
	Price             float32   `json:"price"`
	NumberOfTickets   int32     `json:"number_of_tickets"`
	TicketsRemaining  int32     `json:"tickets_remaining"`
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

type EventDetailFailedRefundOrCancel struct {
	PaymentID uuid.UUID `json:"payment_id"`
	Action    string    `json:"action"`
	Code      string    `json:"code"`
	Message   string    `json:"message"`
}

type FailedNotificationEmail struct {
	SendRefundCancelNotificationError string `json:"send_refund_cancel_notification_error"`
}