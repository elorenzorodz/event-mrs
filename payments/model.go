package payments

import (
	"time"

	"github.com/google/uuid"
)

type Payment struct {
	ID              uuid.UUID `json:"id"`
	PaymentIntentID string    `json:"payment_intent_id"`
	Amount          float64   `json:"amount"`
	Currency        string    `json:"currency"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       string    `json:"updated_at"`
	UserID          uuid.UUID `json:"user_id"`
}