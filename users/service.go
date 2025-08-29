package users

import (
	"log"
	"net/http"

	"github.com/elorenzorodz/event-mrs/common"
	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (userAPIConfig *UserAPIConfig) RegisterUser(ginContext *gin.Context) {
	type parameters struct {
		FirstName string `json:"firstname" binding:"required"`
		LastName  string `json:"lastname" binding:"required"`
		Email     string `json:"email" binding:"required"`
		Password  string `json:"password" binding:"required"`
	}

	params := parameters{}

	// Bind incoming JSON to struct and check for errors in the process.
	if parameterBindError := ginContext.ShouldBindJSON(&params); parameterBindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "error parsing JSON, please check all required fields are present"})
		
		return
	}

	isEmailValid := common.IsEmailValid(params.Email)

	// Validate email address.
	if !isEmailValid {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid email address"})

		return
	}

	isPasswordValid := common.IsPasswordValid(params.Password)

	// Validate password.
	if !isPasswordValid {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid password, password must contain at least 1 upper case letter, 1 lower case letter, 1 digit and must be 12 to 20 characters long"})

		return
	}

	// Create hashed password for new user.
	hashedPassword, hashPasswordError := common.HashPassword(params.Password)

	if hashPasswordError != nil {
		log.Printf("Error hashing password: %s", hashPasswordError)
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "error creating user"})

		return
	}

	createUserParams := database.CreateUserParams {
		ID: uuid.New(),
		FirstName: params.FirstName,
		LastName: params.LastName,
		Email: params.Email,
		Password: hashedPassword,
	}

	newUser, createUserError := userAPIConfig.DB.CreateUser(ginContext,createUserParams)

	if createUserError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": createUserError.Error()})

		return
	}

	ginContext.JSON(http.StatusOK, gin.H{"message": newUser})
}