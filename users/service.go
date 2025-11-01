package users

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/elorenzorodz/event-mrs/internal/auth"
	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/elorenzorodz/event-mrs/internal/validation"
	"github.com/google/uuid"
)

var (
	ErrPasswordInvalid = errors.New("invalid email or password")
	ErrEmailExists     = errors.New("email address is already registered")
	ErrPasswordWeak    = errors.New("invalid password, password must contain at least 1 upper case letter, 1 lower case letter, 1 digit and must be 12 to 20 characters long")
)

type UserService interface {
	Register(ctx context.Context, req RegisterRequest) (*User, error)
	Login(ctx context.Context, req LoginRequest) (*UserAuthorized, error)
}

type Service struct {
	DBQueries *database.Queries 
	TokenGenerator auth.TokenGenerator 
}

func NewService(db *database.Queries, tg auth.TokenGenerator) UserService {
	return &Service{
		DBQueries: db,
		TokenGenerator: tg,
	}
}

func (service *Service) Register(ctx context.Context, req RegisterRequest) (*User, error) {
	if !validation.IsEmailValid(req.Email) {
		return nil, ErrPasswordInvalid
	}
	if !validation.IsPasswordValid(req.Password) {
		return nil, ErrPasswordWeak
	}
	
	hashedPassword, hashPasswordError := auth.HashPassword(req.Password)
	if hashPasswordError != nil {
		log.Printf("Error hashing password: %s", hashPasswordError)
		return nil, errors.New("internal server error")
	}

	createUserParams := database.CreateUserParams {
		ID: uuid.New(),
		Firstname: req.FirstName,
		Lastname: req.LastName,
		Email: req.Email,
		Password: hashedPassword,
	}
	
	newUserDB, createUserError := service.DBQueries.CreateUser(ctx, createUserParams)

	if createUserError != nil {
		return nil, ErrEmailExists 
	}

	return databaseUserToDomainUser(newUserDB), nil
}

func (service *Service) Login(ctx context.Context, req LoginRequest) (*UserAuthorized, error) {
	getUserDB, getUserError := service.DBQueries.GetUserByEmail(ctx, req.Email)

	if getUserError != nil {
		return nil, ErrPasswordInvalid 
	}

	verifyPasswordError := auth.VerifyPassword(req.Password, getUserDB.Password)

	if verifyPasswordError != nil {
		return nil, ErrPasswordInvalid 
	}

	signedToken, signedTokenError := service.TokenGenerator.Generate(getUserDB.Email)

	if signedTokenError != nil {
		log.Printf("Token generation error: %v", signedTokenError)
		return nil, errors.New("internal token generation error")
	}

	return &UserAuthorized{
		Email: getUserDB.Email,
		AccessToken: signedToken,
	}, nil
}

func databaseUserToDomainUser(dbUser database.User) *User {
	var updatedAt *time.Time
	if dbUser.UpdatedAt.Valid {
		updatedAt = &dbUser.UpdatedAt.Time
	}

	return &User{
		ID:        dbUser.ID.String(), 
		FirstName: dbUser.Firstname,
		LastName:  dbUser.Lastname,
		Email:     dbUser.Email,
		CreatedAt: dbUser.CreatedAt,
		UpdatedAt: updatedAt,
	}
}