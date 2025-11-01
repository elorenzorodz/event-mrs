package users

import "time"

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