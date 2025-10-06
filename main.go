package main

import (
	"log"
	"net/http"

	"github.com/elorenzorodz/event-mrs/common"
	"github.com/elorenzorodz/event-mrs/event_details"
	"github.com/elorenzorodz/event-mrs/events"
	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/elorenzorodz/event-mrs/middleware"
	"github.com/elorenzorodz/event-mrs/payments"
	"github.com/elorenzorodz/event-mrs/reservations"
	"github.com/elorenzorodz/event-mrs/users"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	// TODO: Refund payments for deleted events.
	// TODO: Send email confirmation for payment and reservation.
	// TODO: Add email alert whenever the event is updated or deleted.
	// TODO: Add email alert whenever the event detail is updated or deleted.

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

	routerAPIPrefix := router.Group("/api/" + apiVersion)

	routerAPIPrefix.GET("/ping", func(ginContext *gin.Context) {
		ginContext.JSON(http.StatusOK, gin.H { "message": "pong" })
	})

	userAPIConfig := users.UserAPIConfig {
		DB: database,
	}

	routerAPIPrefix.POST("/account/register", userAPIConfig.RegisterUser)
	routerAPIPrefix.POST("/account/login", userAPIConfig.LoginUser)

	routerWithAuthorization := routerAPIPrefix.Group("")
	routerWithAuthorization.Use(middleware.AuthorizationMiddleware(&userAPIConfig))

	eventAPIConfig := events.EventAPIConfig {
		DB: database,
	}

	routerWithAuthorization.GET("/events", eventAPIConfig.GetUserEvents)
	routerWithAuthorization.GET("/events/:eventId", eventAPIConfig.GetUserEventById)
	routerWithAuthorization.GET("/events/filter", eventAPIConfig.GetEvents)
	routerWithAuthorization.POST("/events", eventAPIConfig.CreateEvent)
	routerWithAuthorization.PUT("/events/:eventId", eventAPIConfig.UpdateEvent)
	routerWithAuthorization.DELETE("/events/:eventId", eventAPIConfig.DeleteEvent)

	eventDetailAPIConfig := event_details.EventDetailAPIConfig {
		DB: database,
	}

	routerWithAuthorization.POST("/events/:eventId/details", eventDetailAPIConfig.CreateEventDetail)
	routerWithAuthorization.PUT("/events/:eventId/details/:eventDetailId", eventDetailAPIConfig.UpdateEventDetail)
	routerWithAuthorization.DELETE("/events/:eventId/details/:eventDetailId", eventDetailAPIConfig.DeleteEventDetail)

	reservationAPIConfig := reservations.ReservationAPIConfig {
		DB: database,
	}

	routerWithAuthorization.GET("/reservations", reservationAPIConfig.GetUserReservations)
	routerWithAuthorization.GET("/reservations/:reservationId", reservationAPIConfig.GetUserReservationById)
	routerWithAuthorization.POST("/reservations", reservationAPIConfig.CreateReservation)
	routerWithAuthorization.PATCH("/reservations/:reservationId", reservationAPIConfig.UpdateReservationEmail)

	paymentAPIConfig := payments.PaymentAPIConfig {
		DB: database,
	}

	routerAPIPrefix.POST("/payments/webhook", paymentAPIConfig.StripeWebhook)
	routerWithAuthorization.PATCH("/payments/:paymentId", paymentAPIConfig.UpdatePayment)
	routerWithAuthorization.GET("/payments", paymentAPIConfig.GetUserPayments)

	log.Printf("Server starting on port %s in %s mode", port, ginMode)

	routerRunError := router.Run(":" + port)

	if routerRunError != nil {
		log.Fatal(routerRunError)
	}
}
