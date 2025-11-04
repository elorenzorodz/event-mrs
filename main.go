package main

import (
	"log"
	"net/http"

	"github.com/elorenzorodz/event-mrs/config"
	"github.com/elorenzorodz/event-mrs/event_details"
	"github.com/elorenzorodz/event-mrs/events"
	"github.com/elorenzorodz/event-mrs/internal/auth"
	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/elorenzorodz/event-mrs/internal/mailer"
	"github.com/elorenzorodz/event-mrs/middleware"
	"github.com/elorenzorodz/event-mrs/payments"
	"github.com/elorenzorodz/event-mrs/reservations"
	"github.com/elorenzorodz/event-mrs/users"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/stripe/stripe-go/v83"
)

func main() {
	godotenv.Load(".env.dev")

	envConfig, loadEnvironmentVariablesError := config.LoadEnvironmentVariables()

	if loadEnvironmentVariablesError != nil {
		log.Fatalf("Fatal: Failed to load configuration: %v", loadEnvironmentVariablesError)
	}

	dbConnection, dbConnectionError := database.OpenConnection(envConfig.DBURL)

	if dbConnectionError != nil {
		log.Fatalf("Fatal: Could not connect to database: %v", dbConnectionError)
	}

	gin.SetMode(envConfig.GinMode)

	dbQueries := database.New(dbConnection)

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	routerAPIPrefix := router.Group("/api/" + envConfig.APIVersion)

	routerAPIPrefix.GET("/ping", func(ginContext *gin.Context) {
		ginContext.JSON(http.StatusOK, gin.H{"message": "pong"})
	})

	signingKey, tokenValidator := auth.LoadKeys("private.pem", "public.pem")

	tokenGenerator := auth.NewTokenGenerator(signingKey)

	userService := users.NewService(dbQueries, tokenGenerator)

	userAPIConfig := users.UserAPIConfig{
		Service: userService,
	}

	routerAPIPrefix.POST("/account/register", userAPIConfig.RegisterUser)
	routerAPIPrefix.POST("/account/login", userAPIConfig.LoginUser)

	routerWithAuthorization := routerAPIPrefix.Group("")
	routerWithAuthorization.Use(middleware.AuthorizationMiddleware(dbQueries, tokenValidator))

	mailerConfig := mailer.MailerConfig{
		Domain: envConfig.MailgunSendingDomain,
		APIKey: envConfig.MailgunAPIKey,
		SenderName: envConfig.SenderName,
		SenderEmail: envConfig.SenderEmail,
		TeamName: envConfig.TeamName,
		TeamEmail: envConfig.TeamEmail,
	}
	newMailer := mailer.NewMailer(mailerConfig)
	stripe.Key = envConfig.StripeSecretKey
	stripeClient := &events.StripeAPIClient{}

	eventService := events.NewService(*dbQueries, newMailer, stripeClient)

	eventAPIConfig := events.EventAPIConfig{
		Service: eventService,
	}

	routerWithAuthorization.GET("/events", eventAPIConfig.GetUserEvents)
	routerWithAuthorization.GET("/events/:eventId", eventAPIConfig.GetUserEventById)
	routerWithAuthorization.GET("/events/filter", eventAPIConfig.GetEvents)
	routerWithAuthorization.POST("/events", eventAPIConfig.CreateEvent)
	routerWithAuthorization.PUT("/events/:eventId", eventAPIConfig.UpdateEvent)
	routerWithAuthorization.DELETE("/events/:eventId", eventAPIConfig.DeleteEvent)

	stripeClientEventDetails := &event_details.StripeAPIClient{}
	eventDetailService := event_details.NewService(*dbQueries, newMailer, stripeClientEventDetails)
	
	eventDetailAPIConfig := event_details.EventDetailAPIConfig{
		Service: eventDetailService,
	}

	routerWithAuthorization.POST("/events/:eventId/details", eventDetailAPIConfig.CreateEventDetail)
	routerWithAuthorization.PUT("/events/:eventId/details/:eventDetailId", eventDetailAPIConfig.UpdateEventDetail)
	routerWithAuthorization.DELETE("/events/:eventId/details/:eventDetailId", eventDetailAPIConfig.DeleteEventDetail)

	reservationAPIConfig := reservations.ReservationAPIConfig{
		DB: dbQueries,
	}

	routerWithAuthorization.GET("/reservations", reservationAPIConfig.GetUserReservations)
	routerWithAuthorization.GET("/reservations/:reservationId", reservationAPIConfig.GetUserReservationById)
	routerWithAuthorization.POST("/reservations", reservationAPIConfig.CreateReservation)
	routerWithAuthorization.PATCH("/reservations/:reservationId", reservationAPIConfig.UpdateReservationEmail)

	paymentAPIConfig := payments.PaymentAPIConfig{
		DB: dbQueries,
	}

	routerAPIPrefix.POST("/payments/webhook", paymentAPIConfig.StripeWebhook)
	routerAPIPrefix.POST("/payments/refund-webhook", paymentAPIConfig.StripeRefundWebhook)
	routerWithAuthorization.GET("/payments", paymentAPIConfig.GetUserPayments)
	routerWithAuthorization.GET("/payments/:paymentId", paymentAPIConfig.GetUserPaymentById)
	routerWithAuthorization.PATCH("/payments/:paymentId", paymentAPIConfig.UpdatePayment)
	routerWithAuthorization.POST("/payments/:paymentId/refund", paymentAPIConfig.RefundPayment)

	log.Printf("Server starting on port %s in %s mode", envConfig.Port, envConfig.GinMode)

	routerRunError := router.Run(":" + envConfig.Port)

	if routerRunError != nil {
		log.Fatal(routerRunError)
	}
}
