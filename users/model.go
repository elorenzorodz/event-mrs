package users

import (
	"context"
	"time"

	"github.com/elorenzorodz/event-mrs/internal/auth"
	"github.com/elorenzorodz/event-mrs/internal/database"
)

type User struct {
	ID        string
	FirstName string
	LastName  string
	Email     string
	CreatedAt time.Time
	UpdatedAt *time.Time
}

type RegisterRequest struct {
	FirstName string `json:"firstname" binding:"required"`
	LastName  string `json:"lastname" binding:"required"`
	Email     string `json:"email" binding:"required"`
	Password  string `json:"password" binding:"required"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type UserResponse struct {
	ID        string    `json:"id"`
	FirstName string    `json:"firstname"`
	LastName  string    `json:"lastname"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type UserAuthorized struct {
	Email       string `json:"email"`
	AccessToken string `json:"access_token"`
}

func NewUserResponse(user *User) UserResponse {
	return UserResponse{
		ID:        user.ID,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
	}
}

type UserService interface {
	Register(ctx context.Context, req RegisterRequest) (*User, error)
	Login(ctx context.Context, req LoginRequest) (*UserAuthorized, error)
}

type Service struct {
	DBQueries *database.Queries 
	TokenGenerator auth.TokenGenerator 
}

type UserAPIConfig struct {
	Service UserService
}