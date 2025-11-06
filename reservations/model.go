package reservations

import (
	"context"
	"database/sql"
	"time"

	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/elorenzorodz/event-mrs/internal/mailer"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v83"
)

type ReservationAPIConfig struct {
	Service ReservationService
}

type Reservation struct {
	ID            uuid.UUID `json:"id"`
	Email         string    `json:"email"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     string    `json:"updated_at"`
	EventDetailID uuid.UUID `json:"event_detail_id"`
	UserID        uuid.UUID `json:"user_id"`
	PaymentID     uuid.UUID `json:"payment_id"`
}

// Note: If email isn't provided here, try to get from current user.
type ReservationParameters struct {
	Email                   string                   `json:"email"`
	Currency                string                   `json:"currency"`
	PaymentMethodID         string                   `json:"payment_method_id" binding:"required"`
	EventDetailReservations []EventDetailReservation `json:"event_detail_reservations" binding:"required"`
}

// Note: If email isn't provided here, try to get from ReservationParameters.
type EventDetailReservation struct {
	EventDetailID uuid.UUID `json:"event_detail_id" binding:"required"`
	Quantity      int32     `json:"quantity" binding:"required"`
	Email         string    `json:"email"`
}

type PatchReservationParameters struct {
	Email string `json:"email" binding:"required"`
}

type ReservationResponse struct {
	Reservations   []Reservation `json:"reservations"`
	PaymentID      uuid.UUID     `json:"payment_id"`
	ClientSecret   string        `json:"client_secret"`
	PaymentStatus  string        `json:"payment_status"`
	PaymentMessage string        `json:"payment_message"`
}

type PaymentResponse struct {
	ID           uuid.UUID `json:"id"`
	NextAction   string    `json:"next_action"`
	Status       string    `json:"status"`
	ClientSecret string    `json:"client_secret"`
	Message      string    `json:"message"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type StripeClient interface {
	CreatePaymentIntent(amount int64, currency string, paymentMethodID string, paymentId uuid.UUID) (*stripe.PaymentIntent, error)
}

type ReservationService interface {
	CreateReservations(ctx context.Context, userId uuid.UUID, userEmail string, reservations ReservationParameters) ([]Reservation, PaymentResponse, error)
	GetUserReservations(ctx context.Context, userID uuid.UUID) ([]Reservation, error)
	GetUserReservationByID(ctx context.Context, reservationID, userID uuid.UUID) (*Reservation, error)
	UpdateReservationEmail(ctx context.Context, reservationID, userID uuid.UUID, email string) (*Reservation, error)
}

type Service struct {
	DBQueries    database.Queries
	DBConnection *sql.DB
	Stripe       StripeClient
	Mailer       *mailer.Mailer
}

type StripeAPIClient struct{}