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
}

type ReservationParameters struct {
	Email                   string                   `json:"email"`
	EventDetailReservations []EventDetailReservation `json:"event_detail_reservations" binding:"required"`
}

type EventDetailReservation struct {
	EventDetailID uuid.UUID `json:"event_detail_id" binding:"required"`
	Quantity      int     	`json:"quantity" binding:"required"`
}