package users

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/elorenzorodz/event-mrs/common"
	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
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
		Firstname: params.FirstName,
		Lastname: params.LastName,
		Email: params.Email,
		Password: hashedPassword,
	}

	newUser, createUserError := userAPIConfig.DB.CreateUser(ginContext.Request.Context(), createUserParams)

	if createUserError != nil {
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": createUserError.Error()})

		return
	}

	ginContext.JSON(http.StatusOK, gin.H{"user": newUser})
}

func (userAPIConfig *UserAPIConfig) LoginUser(ginContext *gin.Context) {
	type parameters struct {
		Email     string `json:"email" binding:"required"`
		Password  string `json:"password" binding:"required"`
	}

	params := parameters{}

	// Bind incoming JSON to struct and check for errors in the process.
	if parameterBindError := ginContext.ShouldBindJSON(&params); parameterBindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "error parsing JSON, please check all required fields are present"})
		
		return
	}

	getUser, getUserError := userAPIConfig.DB.GetUserByEmail(ginContext.Request.Context(), params.Email)

	if getUserError != nil {
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": "failed to login, please try again in a few minutes"})

		return
	}

	// Verify password.
	verifyPasswordError := common.VerifyPassword(params.Password, getUser.Password)

	if verifyPasswordError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid email or password"})

		return
	}

	// Private and public keys used the following settings for this project:
	// Curve: SECG secp256r1 / X9.62 prime256v1 / NIST P-256
	// Output Type: PEM text
	// Format: PKCS#8
	newAccessToken := jwt.NewWithClaims(
		jwt.SigningMethodES256,
		jwt.MapClaims{
			"email": getUser.Email,
			"exp": time.Now().Add(time.Hour * 1).Unix(),
		})

	privateBytes, readPrivateKeyError := os.ReadFile("private.pem")

	if readPrivateKeyError != nil {
		log.Printf("Private key read file error %v", readPrivateKeyError)
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": "failed to login, please try again in a few minutes"})

		return
	}

	parsedPrivateKey, parsePrivateKeyError := jwt.ParseECPrivateKeyFromPEM(privateBytes)

	if parsePrivateKeyError != nil {
		log.Printf("Parse private key error %v", parsePrivateKeyError)
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": "failed to login, please try again in a few minutes"})

		return
	}

	signedToken, signedTokenError := newAccessToken.SignedString(parsedPrivateKey)

	if signedTokenError != nil {
		log.Printf("Signing token error %v", signedTokenError)
		ginContext.JSON(http.StatusInternalServerError, gin.H{"error": "failed to login, please try again in a few minutes"})

		return
	}

	userAuthorized := DatabaseUserToUserAuthorizedJSON(getUser, signedToken)

	ginContext.JSON(http.StatusOK, gin.H{"user": userAuthorized})
}