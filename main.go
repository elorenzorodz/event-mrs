package main

import (
	"log"
	"net/http"

	"github.com/elorenzorodz/event-mrs/common"
	"github.com/elorenzorodz/event-mrs/events"
	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/elorenzorodz/event-mrs/middleware"
	"github.com/elorenzorodz/event-mrs/users"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	godotenv.Load(".env.dev")

	port := common.GetEnvVariable("PORT")
	apiVersion := common.GetEnvVariable("API_VERSION")
	ginMode := common.GetEnvVariable("GIN_MODE")
	dbConnectionString := common.GetDBConnectionSettings()
	dbConnection := common.OpenDBConnection(dbConnectionString)
	
	gin.SetMode(ginMode)

	database := database.New(dbConnection)

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	v1 := router.Group("/api/" + apiVersion)

	v1.GET("/ping", func(ginContext *gin.Context) {
		ginContext.JSON(http.StatusOK, gin.H { "message": "pong" })
	})

	userAPIConfig := users.UserAPIConfig {
		DB: database,
	}

	v1.POST("/account/register", userAPIConfig.RegisterUser)
	v1.POST("/account/login", userAPIConfig.LoginUser)

	eventAPIConfig := events.EventAPIConfig {
		DB: database,
	}
	
	v1WithAuthorization := v1.Group("")
	v1WithAuthorization.Use(middleware.AuthorizationMiddleware(&userAPIConfig))

	v1WithAuthorization.GET("/events", eventAPIConfig.GetUserEvents)
	v1WithAuthorization.POST("/events", eventAPIConfig.CreateEvent)

	log.Printf("Server starting on port %s in %s mode", port, ginMode)

	routerRunError := router.Run(":" + port)

	if routerRunError != nil {
		log.Fatal(routerRunError)
	}
}
