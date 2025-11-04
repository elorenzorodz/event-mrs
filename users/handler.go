package users

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (userAPIConfig *UserAPIConfig) RegisterUser(ginContext *gin.Context) {
	var registerRequest RegisterRequest

	if parameterBindError := ginContext.ShouldBindJSON(&registerRequest); parameterBindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "error parsing JSON, please check all required fields are present"})

		return
	}

	user, registerError := userAPIConfig.Service.Register(ginContext.Request.Context(), registerRequest)

	if registerError != nil {
		switch registerError {
		case ErrPasswordWeak:
			ginContext.JSON(http.StatusBadRequest, gin.H{"error": registerError.Error()})

		case ErrEmailExists:
			ginContext.JSON(http.StatusConflict, gin.H{"error": "email address already registered"})

		case ErrPasswordInvalid:
			ginContext.JSON(http.StatusBadRequest, gin.H{"error": "invalid email format or missing fields"})

		default:
			ginContext.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register user due to an internal error"})
		}

		return
	}

	userResponse := NewUserResponse(user)

	ginContext.JSON(http.StatusCreated, gin.H{"user": userResponse})
}

func (userAPIConfig *UserAPIConfig) LoginUser(ginContext *gin.Context) {
	var loginRequest LoginRequest

	if parameterBindError := ginContext.ShouldBindJSON(&loginRequest); parameterBindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"error": "error parsing JSON, please check all required fields are present"})

		return
	}

	userAuth, loginError := userAPIConfig.Service.Login(ginContext.Request.Context(), loginRequest)

	if loginError != nil {
		switch loginError {
		case ErrPasswordInvalid:
			ginContext.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})

		default:
			ginContext.JSON(http.StatusInternalServerError, gin.H{"error": "failed to login, please try again in a few minutes"})

		}
		return
	}

	ginContext.JSON(http.StatusOK, gin.H{"user": userAuth})
}