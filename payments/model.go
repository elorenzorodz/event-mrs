package payments

import (
	"time"

	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/google/uuid"
)

type PaymentAPIConfig struct {
	DB *database.Queries
}

type Payment struct {
	ID              uuid.UUID `json:"id"`
	PaymentIntentID string    `json:"payment_intent_id"`
	Amount          float64   `json:"amount"`
	Currency        string    `json:"currency"`
	Status          string    `json:"status"`
	ExpiresAt       time.Time `json:"expires_at"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       string    `json:"updated_at"`
	UserID          uuid.UUID `json:"user_id"`
}

type PaymentResponse struct {
	ID           uuid.UUID `json:"id"`
	NextAction   string    `json:"next_action"`
	Status       string    `json:"status"`
	ClientSecret string    `json:"client_secret"`
	Message      string    `json:"message"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type PaymentParameters struct {
	PaymentMethodID string `json:"payment_method_id" binding:"required"`
}