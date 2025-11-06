package reservations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/elorenzorodz/event-mrs/internal/convert"
	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/elorenzorodz/event-mrs/internal/mailer"
	"github.com/elorenzorodz/event-mrs/internal/sqlutil"
	"github.com/elorenzorodz/event-mrs/payments"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v83"
	"github.com/stripe/stripe-go/v83/paymentintent"
)

var (
	ErrInsufficientTickets = errors.New("insufficient tickets remaining for one or more requested events")
	ErrTicketNotFound      = errors.New("event detail not found")
	ErrPaymentFailed       = errors.New("payment failed, please check your method or try again")
	ErrInternalError       = errors.New("an internal error occurred")
)

func NewService(dbQueries database.Queries, dbConn *sql.DB, mMailer *mailer.Mailer, stripeClient StripeClient) ReservationService {
	return &Service{
		DBQueries:    dbQueries,
		DBConnection: dbConn,
		Mailer:       mMailer,
		Stripe:       stripeClient,
	}
}

func (stripeAPIClient *StripeAPIClient) CreatePaymentIntent(amount int64, currency string, paymentMethodID string, paymentId uuid.UUID) (*stripe.PaymentIntent, error) {
	paymentIntentParams := &stripe.PaymentIntentParams{
		Amount:        stripe.Int64(amount),
		Currency:      stripe.String(strings.ToLower(currency)),
		Confirm:       stripe.Bool(true),
		PaymentMethod: stripe.String(paymentMethodID),
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled:        stripe.Bool(true),
			AllowRedirects: stripe.String("never"),
		},
		Metadata: map[string]string{"payment_id": paymentId.String()},
	}

	return paymentintent.New(paymentIntentParams)
}

func (service *Service) CreateReservations(ctx context.Context, userId uuid.UUID, userEmail string, reservations ReservationParameters) ([]Reservation, PaymentResponse, error) {
	var totalTickets int32 = 0

	for _, eventDetailReservation := range reservations.EventDetailReservations {
		totalTickets += eventDetailReservation.Quantity
	}

	if totalTickets == 0 {
		return nil, PaymentResponse{}, fmt.Errorf("no tickets being reserved")
	}

	var (
		newReservations []Reservation
		totalPrice      int64
	)

	currency := reservations.Currency

	if strings.TrimSpace(currency) == "" {
		currency = "usd"
	}

	eventDetails, totalPrice, priceError := validateAndCalculatePrice(&service.DBQueries, ctx, reservations)

	if priceError != nil {
		// If validation fails (e.g., tickets sold out), exit immediately with the error.
		return nil, PaymentResponse{}, priceError
	}

	var newPayment database.Payment
	var createPaymentError error

	// Atomically create the Payment record and all Reservations.
	tx, beginTxError := service.DBConnection.BeginTx(ctx, nil)

	if beginTxError != nil {
		return nil, PaymentResponse{}, fmt.Errorf("failed to begin transaction: %w", beginTxError)
	}

	defer tx.Rollback()
	qtx := service.DBQueries.WithTx(tx)

	// Create PENDING Payment record with the now known FINAL price.
	createPaymentParams := database.CreatePaymentParams{
		ID:        uuid.New(),
		Amount:    fmt.Sprintf("%.2f", float64(totalPrice)/100.0),
		Currency:  currency,
		Status:    "pending",
		UserID:    userId,
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}

	// Capture the result to get the final Payment ID.
	newPayment, createPaymentError = qtx.CreatePayment(ctx, createPaymentParams)

	if createPaymentError != nil {
		log.Printf("Error creating payment record: %v", createPaymentError)

		return nil, PaymentResponse{}, fmt.Errorf("internal database error creating payment")
	}

	// Reserve tickets sequentially.
	for _, edReservation := range reservations.EventDetailReservations {
		emailReservation := edReservation.Email

		if strings.TrimSpace(emailReservation) == "" {
			emailReservation = userEmail
		}

		// Loop once for each ticket quantity requested for this event detail.
		for x := 0; x < int(edReservation.Quantity); x++ {
			reserveTicketParams := database.ReserveTicketParams{
				EventDetailID: edReservation.EventDetailID,
				ReservationID: uuid.New(),
				Email:         emailReservation,
				UserID:        userId,
				PaymentID:     newPayment.ID,
			}

			// The database's ReserveTicket SQL query handles the `tickets_remaining > 0` check.
			reservedTicket, reserveTicketError := qtx.ReserveTicket(ctx, reserveTicketParams)

			if reserveTicketError != nil {
				log.Printf("Error reserving ticket for event detail %s: %v", edReservation.EventDetailID, reserveTicketError)

				return nil, PaymentResponse{}, fmt.Errorf("error reserving ticket, transaction rolled back: %w", reserveTicketError)
			}

			// Collect the successfully created reservation.
			newReservations = append(newReservations, DatabaseReservationToReservationJSON(reservedTicket))
		}
	}

	// Commit if everything succeeded.
	if err := tx.Commit(); err != nil {
		return nil, PaymentResponse{}, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Convert the successfully created database payment record to the external JSON type.
	userPayment := payments.DatabasePaymentToPaymentJSON(newPayment)

	paymentIntentId := ""
	paymentResponse := PaymentResponse{
		ID:        userPayment.ID,
		ExpiresAt: userPayment.ExpiresAt,
	}

	if totalPrice > 0 {
		// Tickets reserved are not free.
		// Log creation (before calling Stripe).
		createPaymentLogParams := database.CreatePaymentLogParams{
			ID:              uuid.New(),
			PaymentMethodID: sqlutil.StringToNullString(reservations.PaymentMethodID),
			Amount:          fmt.Sprintf("%.2f", float64(totalPrice)/100.0),
			UserEmail:       userEmail,
			PaymentID:       userPayment.ID,
		}

		paymentIntentResult, paymentIntentError := service.Stripe.CreatePaymentIntent(totalPrice, userPayment.Currency, reservations.PaymentMethodID, userPayment.ID)

		if paymentIntentError != nil {
			if stripeErr, ok := paymentIntentError.(*stripe.Error); ok {
				paymentResponse.Status = *stripe.String(stripeErr.Code)
				paymentResponse.Message = *stripe.String(stripeErr.Msg)

				if stripeErr.PaymentIntent != nil {
					paymentIntentId = stripeErr.PaymentIntent.ID
				}
			}
		} else {
			if paymentIntentResult != nil {
				paymentResponse.Status = string(paymentIntentResult.Status)
				paymentIntentId = paymentIntentResult.ID
				paymentResponse.Message = "payment successful"

				if paymentResponse.Status == string(stripe.PaymentIntentStatusRequiresAction) {
					paymentResponse.ClientSecret = paymentIntentResult.ClientSecret

					if paymentIntentResult.NextAction != nil {
						paymentResponse.NextAction = string(paymentIntentResult.NextAction.Type)
					}
				} else if paymentResponse.Status != string(stripe.PaymentIntentStatusSucceeded) {
					// Keep the default success message unless the status indicates failure/pending.
					paymentResponse.Message = "please refer to next action and status"
				}
			}
		}

		// Log result.
		createPaymentLogParams.Status = paymentResponse.Status
		createPaymentLogParams.Description = sqlutil.StringToNullString(paymentResponse.Message)
		createPaymentLogParams.PaymentIntentID = paymentIntentId

		// Log the outcome to the payment_logs table
		_, createPaymentLogError := service.DBQueries.CreatePaymentLog(ctx, createPaymentLogParams)

		if createPaymentLogError != nil {
			log.Printf("error: create payment log - %s", createPaymentLogError)
		}

	} else {
		// For free tickets, explicitly mark the payment record as succeeded.
		// This status will be used to update the payments table later.
		paymentResponse.Status = string(stripe.PaymentIntentStatusSucceeded)
		paymentResponse.Message = "free reservation successful"
	}

	// Handle non-succeeded payment statuses
	if paymentResponse.Status != string(stripe.PaymentIntentStatusSucceeded) {
		switch paymentResponse.Status {
		case string(stripe.PaymentIntentStatusRequiresAction):
			paymentResponse.Message = "complete payment within next 15 minutes"

		case string(stripe.PaymentIntentStatusCanceled):
			paymentResponse.Message = "payment expired, please rebook your tickets"

			// Perform necessary cleanup: Restore tickets and delete the payment record.
			deletePaymentParams := database.RestoreTicketsAndDeletePaymentParams{
				PaymentID: userPayment.ID,
				UserID:    userId,
			}

			deletePaymentError := service.DBQueries.RestoreTicketsAndDeletePayment(ctx, deletePaymentParams)

			if deletePaymentError != nil {
				log.Printf("error restoring tickets and deleting payment after cancellation: %v", deletePaymentError)

				return nil, paymentResponse, fmt.Errorf("payment canceled, failed to clean up database: %w", deletePaymentError)
			}

			// Return immediately on confirmed failure/cancellation after cleanup.
			return nil, paymentResponse, fmt.Errorf("payment has been canceled, please rebook")

		case string(stripe.PaymentIntentStatusProcessing):
			paymentResponse.Message = "payment processing, we'll send you an email once payment succeeded"

		case string(stripe.PaymentIntentStatusRequiresPaymentMethod):
			paymentResponse.Message = "please submit new payment method"
		}
	} else {
		// Handle Succeeded Payment - send confirmation email.
		// Fetch user details for the email.
		user, getUserError := service.DBQueries.GetUserById(ctx, userId)

		fullName := userEmail
		if getUserError == nil {
			fullName = fmt.Sprintf("%s %s", user.Firstname, user.Lastname)
		} else {
			log.Printf("error fetching user for email: %v", getUserError)
		}

		sendEmailError := service.Mailer.SendPaymentConfirmationAndTicketReservation(fullName, userEmail, eventDetails)

		if sendEmailError != nil {
			log.Printf("error sending confirmation email: %v", sendEmailError)
		}
	}

	// This ensures the payments table reflects the final status and payment intent ID.
	updatePaymentParams := database.UpdatePaymentParams{
		Amount:          fmt.Sprintf("%.2f", float64(totalPrice)/100.0),
		Status:          paymentResponse.Status,
		PaymentIntentID: sqlutil.StringToNullString(paymentIntentId),
		ID:              userPayment.ID,
		UserID:          userId,
	}

	_, updatePaymentError := service.DBQueries.UpdatePayment(ctx, updatePaymentParams)

	if updatePaymentError != nil {
		// Log this error, but do not fail the function, as tickets are reserved.
		log.Printf("error updating final payment status in DB: %v", updatePaymentError)
	}

	return newReservations, paymentResponse, nil
}

func (service *Service) GetUserReservations(ctx context.Context, userID uuid.UUID) ([]Reservation, error) {
	databaseReservations, err := service.DBQueries.GetUserReservations(ctx, userID)
	
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {

			return []Reservation{}, nil
		}

		log.Printf("Database error fetching user reservations: %v", err)
		
		return nil, ErrInternalError
	}
	return DatabaseReservationsToReservationsJSON(databaseReservations), nil
}

func (service *Service) GetUserReservationByID(ctx context.Context, reservationID, userID uuid.UUID) (*Reservation, error) {
	databaseReservation, err := service.DBQueries.GetUserReservationById(ctx, database.GetUserReservationByIdParams{
		ID:     reservationID,
		UserID: userID,
	})

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		log.Printf("Database error fetching user reservation by ID: %v", err)

		return nil, ErrInternalError
	}
	reservation := DatabaseReservationToReservationJSON(databaseReservation)

	return &reservation, nil
}

func (service *Service) UpdateReservationEmail(ctx context.Context, reservationID, userID uuid.UUID, email string) (*Reservation, error) {
	updatedDBReservation, err := service.DBQueries.UpdateUserReservationEmail(ctx, database.UpdateUserReservationEmailParams{
		Email:  email,
		ID:     reservationID,
		UserID: userID,
	})

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}

		log.Printf("Database error updating reservation email: %v", err)

		return nil, ErrInternalError
	}
	updatedReservation := DatabaseReservationToReservationJSON(updatedDBReservation)

	return &updatedReservation, nil
}

func validateAndCalculatePrice(dbQueries *database.Queries, ctx context.Context, reservationParams ReservationParameters) ([]database.GetEventDetailsWithTitleByIdsRow, int64, error) {
	if len(reservationParams.EventDetailReservations) == 0 {
		return nil, 0, errors.New("reservations list cannot be empty")
	}

	eventDetailIDs := make([]uuid.UUID, 0, len(reservationParams.EventDetailReservations))

	for _, eventDetailReservation := range reservationParams.EventDetailReservations {
		eventDetailIDs = append(eventDetailIDs, eventDetailReservation.EventDetailID)
	}

	// Fetch all event details that needs to be booked.
	eventDetails, getEventDetailsWithTitleByIdsError := dbQueries.GetEventDetailsWithTitleByIds(ctx, eventDetailIDs)

	if getEventDetailsWithTitleByIdsError != nil {
		log.Printf("Error fetching event details for reservation: %v", getEventDetailsWithTitleByIdsError)

		return nil, 0, ErrInternalError
	}

	detailsMap := make(map[uuid.UUID]database.GetEventDetailsWithTitleByIdsRow)

	for _, eventDetail := range eventDetails {
		detailsMap[eventDetail.ID] = eventDetail
	}

	var totalCents int64
	currentDateTime := time.Now()

	// Iterate through requested reservations and perform validation.
	for _, eventDetailReservation := range reservationParams.EventDetailReservations {
		detail, ok := detailsMap[eventDetailReservation.EventDetailID]

		if !ok {
			return nil, 0, fmt.Errorf("event detail with ID %s not found", eventDetailReservation.EventDetailID)
		}

		// Ticket availability check.
		if detail.TicketsRemaining < int32(eventDetailReservation.Quantity) {
			return nil, 0, fmt.Errorf("%w: only %d tickets remaining for %s", ErrInsufficientTickets, detail.TicketsRemaining, detail.Title)
		}

		// Show date check. Must not be currently showing.
		if currentDateTime.After(detail.ShowDate) {
			return nil, 0, fmt.Errorf("error booking ticket, show date is already past: %s, show date: %s", detail.TicketDescription, detail.ShowDate)
		}

		// Price Calculation.
		priceCents, priceToCentsError := convert.PriceStringToCents(detail.Price)

		if priceToCentsError != nil {
			log.Printf("Error converting price to cents for %s: %v", detail.Price, priceToCentsError)

			return nil, 0, fmt.Errorf("error processing price for ticket: %s", detail.Title)
		}

		totalCents += priceCents * int64(eventDetailReservation.Quantity)
	}

	if totalCents < 0 {
		return nil, 0, errors.New("calculated total price is invalid")
	}

	// Return the fetched details (for email preparation later) and the final price.
	return eventDetails, totalCents, nil
}

func DatabaseReservationToReservationJSON(databaseReservation database.Reservation) Reservation {
	return Reservation{
		ID:            databaseReservation.ID,
		Email:         databaseReservation.Email,
		CreatedAt:     databaseReservation.CreatedAt,
		UpdatedAt:     sqlutil.NullTimeToString(databaseReservation.UpdatedAt),
		EventDetailID: databaseReservation.EventDetailID,
		UserID:        databaseReservation.UserID,
		PaymentID:     databaseReservation.PaymentID,
	}
}

func DatabaseReservationsToReservationsJSON(databaseReservations []database.Reservation) []Reservation {
	reservations := make([]Reservation, len(databaseReservations))

	for i, databaseReservation := range databaseReservations {
		reservations[i] = DatabaseReservationToReservationJSON(databaseReservation)
	}

	return reservations
}