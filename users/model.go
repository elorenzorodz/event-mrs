package users

import "github.com/elorenzorodz/event-mrs/internal/database"

type UserAPIConfig struct {
	DB *database.Queries
}

type UserAuthorized struct {
	Email string `json:"email"`
	AccessToken string `json:"access_token"`
}