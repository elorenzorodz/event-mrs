package middleware

import (
	"net/http"

	"github.com/elorenzorodz/event-mrs/common"
	"github.com/elorenzorodz/event-mrs/users"
	"github.com/gin-gonic/gin"
)

func AuthorizationMiddleware(userAPIConfig *users.UserAPIConfig) gin.HandlerFunc {
	return func(ginContext *gin.Context) {
		// Get the Bearer token from the Authorization header.
		bearerToken, getJWTError := common.GetJWT(ginContext.GetHeader("Authorization"))

		if getJWTError != nil {
			ginContext.JSON(http.StatusUnauthorized, gin.H{"error": getJWTError.Error()})
			ginContext.Abort()

			return
		}
		email, extractEmailClaimError := common.ValidateJWTAndGetEmailClaim(bearerToken)

		if extractEmailClaimError != nil {
			ginContext.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			ginContext.Abort()

			return
		}

		getUser, getUserError := userAPIConfig.DB.GetUserByEmail(ginContext, email)

		if getUserError != nil {
			ginContext.JSON(http.StatusInternalServerError, gin.H{"error": "error retrieving user information"})
			ginContext.Abort()

			return
		}

		ginContext.Set("userId", getUser.ID)
		ginContext.Set("email", getUser.Email)
		ginContext.Next()
	}
}