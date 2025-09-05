package events

import (
	"time"

	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/google/uuid"
)

type EventAPIConfig struct {
	DB *database.Queries
}

type Event struct {
	ID uuid.UUID `json:"id"`
	Title string `json:"title"`
	Description string `json:"description"`
	Organizer string `json:"organizer"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	UserID uuid.UUID `json:"user_id"`
}