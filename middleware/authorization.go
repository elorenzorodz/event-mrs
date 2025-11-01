package middleware

import (
	"net/http"

	"github.com/elorenzorodz/event-mrs/internal/auth"
	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/gin-gonic/gin"
)

func AuthorizationMiddleware(dbQueries *database.Queries, tokenValidator *auth.TokenValidator) gin.HandlerFunc {
	return func(ginContext *gin.Context) {
		bearerToken, getJWTError := auth.GetTokenFromHeader(ginContext.GetHeader("Authorization"))

		if getJWTError != nil {
			ginContext.JSON(http.StatusUnauthorized, gin.H{"error": getJWTError.Error()})
			ginContext.Abort()
			return
		}
		
		email, extractEmailClaimError := tokenValidator.ValidateAndGetEmailClaim(bearerToken)

		if extractEmailClaimError != nil {
			ginContext.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			ginContext.Abort()
			return
		}

		getUser, getUserError := dbQueries.GetUserByEmail(ginContext.Request.Context(), email)

		if getUserError != nil {
			ginContext.JSON(http.StatusUnauthorized, gin.H{"error": "error retrieving user information or invalid session"})
			ginContext.Abort()
			return
		}

		ginContext.Set("userId", getUser.ID)
		ginContext.Set("email", getUser.Email)
		
		ginContext.Next()
	}
}