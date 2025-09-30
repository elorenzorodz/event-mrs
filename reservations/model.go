package reservations

import (
	"time"

	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/google/uuid"
)

type ReservationAPIConfig struct {
	DB *database.Queries
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
	Currency				string					 `json:"currency"`
	EventDetailReservations []EventDetailReservation `json:"event_detail_reservations" binding:"required"`
}

// Note: If email isn't provided here, try to get from ReservationParameters.
type EventDetailReservation struct {
	EventDetailID uuid.UUID `json:"event_detail_id" binding:"required"`
	Quantity      int       `json:"quantity" binding:"required"`
	Email         string    `json:"email"`
}

type PatchReservationParameters struct {
	Email string `json:"email" binding:"required"`
}