package reservations

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/elorenzorodz/event-mrs/common"
	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/elorenzorodz/event-mrs/payments"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v83"
	"github.com/stripe/stripe-go/v83/paymentintent"
)

func DatabaseReservationToReservationJSON(databaseReservation database.Reservation) Reservation {
	return Reservation{
		ID:            databaseReservation.ID,
		Email:         databaseReservation.Email,
		CreatedAt:     databaseReservation.CreatedAt,
		UpdatedAt:     common.NullTimeToString(databaseReservation.UpdatedAt),
		EventDetailID: databaseReservation.EventDetailID,
		UserID:        databaseReservation.UserID,
		PaymentID:     databaseReservation.PaymentID,
	}
}

func DatabaseReservationsToReservationsJSON(databaseReservations []database.Reservation) []Reservation {
	reservations := []Reservation{}

	for _, databaseReservation := range databaseReservations {
		reservations = append(reservations, DatabaseReservationToReservationJSON(databaseReservation))
	}

	return reservations
}

func SaveReservations(db *database.Queries, ctx context.Context, userId uuid.UUID, userEmail string, reservations ReservationParameters) ([]Reservation, payments.PaymentResponse, error) {
	totalTickets := 0

	for _, eventDetailReservation := range reservations.EventDetailReservations {
		totalTickets += eventDetailReservation.Quantity
	}

	if totalTickets == 0 {
		return nil, payments.PaymentResponse{}, fmt.Errorf("no tickets being reserved")
	}

	var (
		newReservations []Reservation
		totalPrice		int64
		mutex           sync.Mutex
		waitGroup       sync.WaitGroup
		errorChannel    = make(chan error, totalTickets)
	)

	currency := reservations.Currency

	if strings.TrimSpace(currency) == "" {
		currency = "usd"
	}

	userPayment := payments.Payment{}

	// Create initial payment and default amount to zero first.
	createPaymentParams := database.CreatePaymentParams {
		ID: uuid.New(),
		Amount: "0.00",
		Currency: currency,
		Status: "pending",
		UserID: userId,
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}

	newPayment, createPaymentError := db.CreatePayment(ctx, createPaymentParams)

	if createPaymentError != nil {
		return newReservations, payments.PaymentResponse{}, fmt.Errorf("cannot process ticket reservation, please try again in a few minutes")
	}

	userPayment = payments.DatabasePaymentToPaymentJSON(newPayment)

	for _, eventDetailReservation := range reservations.EventDetailReservations {
		// Capture loop variable
		edReservation := eventDetailReservation
		emailReservation := edReservation.Email

		if strings.TrimSpace(emailReservation) == "" {
			emailReservation = userEmail
		}

		for x := 0; x < edReservation.Quantity; x++ {
			waitGroup.Go(func(edRsrvtion EventDetailReservation, emailForReservation string) func () {
				return func() {
					// Get event details for additional checking.
					eventDetail, getEventDetailByIdError := db.GetEventDetailsById(ctx, edRsrvtion.EventDetailID)

					if getEventDetailByIdError != nil {
						errorChannel <- fmt.Errorf("error checking event detail tickets remaining: %w", getEventDetailByIdError)

						return
					}

					currentDateTime := time.Now()

					// Event already started or over.
					if currentDateTime.After(eventDetail.ShowDate) {
						errorChannel <- fmt.Errorf("error booking ticket, show date is already past: %s, show date: %s", eventDetail.TicketDescription, eventDetail.ShowDate)

						return
					}

					// Remaining tickets is already zero.
					if eventDetail.TicketsRemaining < 1 {
						errorChannel <- fmt.Errorf("no remaining tickets for: %s, showing on: %s", eventDetail.TicketDescription, eventDetail.ShowDate)

						return
					}

					price, priceParseError := common.PriceStringToCents(eventDetail.Price)

					// Cannot process the price.
					if priceParseError != nil {
						errorChannel <- fmt.Errorf("error processing price of ticket %s, price: %s, showing on: %s", eventDetail.TicketDescription, eventDetail.Price, eventDetail.ShowDate)

						return
					}

					reserveTicketParams := database.ReserveTicketParams{
						Column1: edRsrvtion.EventDetailID,
						Column2: uuid.New(),
						Column3: emailForReservation,
						Column4: userId,
						Column5: userPayment.ID,
					}

					newReservation, reserveTicketError := db.ReserveTicket(ctx, reserveTicketParams)

					if reserveTicketError != nil {
						errorChannel <- fmt.Errorf("error reserving ticket: %w", reserveTicketError)

						return
					}

					mutex.Lock()
					newReservations = append(newReservations, DatabaseReservationToReservationJSON(newReservation))
					totalPrice += price
					mutex.Unlock()
				}
			}(edReservation, emailReservation))
		}
	}

	waitGroup.Wait()
	close(errorChannel)

	allErrors := []string{}

	for err := range errorChannel {
		if err != nil {
			allErrors = append(allErrors, err.Error())
		}
	}

	if len(newReservations) == 0 {
		// No tickets reserved.
		deletePaymentParams := database.RestoreTicketsAndDeletePaymentParams {
			PaymentID: userPayment.ID,
			UserID: userId,
		}

		deletePaymentError := db.RestoreTicketsAndDeletePayment(ctx, deletePaymentParams)

		if deletePaymentError != nil {
			allErrors = append(allErrors, deletePaymentError.Error())
		}

		return nil, payments.PaymentResponse{}, fmt.Errorf("encountered errors:\n%s", strings.Join(allErrors, "\n"))
	}

	paymentIntentId := ""
	paymentResponse := payments.PaymentResponse{
		ID: userPayment.ID,
		ExpiresAt: userPayment.ExpiresAt,
	}

	if totalPrice > 0 {
		// Tickets reserved are not free.
		createPaymentLogParams := database.CreatePaymentLogParams {
			ID: uuid.New(),
			PaymentMethodID: common.StringToNullString(reservations.PaymentMethodID),
			Amount: fmt.Sprintf("%.2f", float64(totalPrice)/100.0),
			UserEmail: userEmail,
			PaymentID: userPayment.ID,
		}

		stripe.Key = common.GetEnvVariable("STRIPE_SECRET_KEY")

		paymentIntentParams := &stripe.PaymentIntentParams {
			Amount: stripe.Int64(totalPrice),
			Currency: stripe.String(strings.ToLower(userPayment.Currency)),
			Confirm: stripe.Bool(true),
			PaymentMethod: stripe.String(reservations.PaymentMethodID),
			AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
				Enabled: stripe.Bool(true),
				AllowRedirects: stripe.String("never"),
			},
			Metadata: map[string]string{"payment_id": userPayment.ID.String()},
		}

		paymentIntentResult, paymentIntentError := paymentintent.New(paymentIntentParams)

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
					paymentResponse.Message = "please refer to next action and status"
				}
			}
		}

		createPaymentLogParams.Status = paymentResponse.Status
		createPaymentLogParams.Description = common.StringToNullString(paymentResponse.Message)
		createPaymentLogParams.PaymentIntentID = paymentIntentId

		_, createPaymentLogError := db.CreatePaymentLog(ctx, createPaymentLogParams)

		if createPaymentLogError != nil {
			log.Printf("error: create payment log - %s", createPaymentLogError)
		}
	}
	
	if paymentResponse.Status != string(stripe.PaymentIntentStatusSucceeded) {
		switch paymentResponse.Status {
			case string(stripe.PaymentIntentStatusRequiresAction):
				paymentResponse.Message = "complete payment within next 15 minutes"

			case string(stripe.PaymentIntentStatusCanceled):
				paymentResponse.Message = "payment expired, please rebook your tickets"
				deletePaymentParams := database.RestoreTicketsAndDeletePaymentParams {
					PaymentID: userPayment.ID,
					UserID: userId,
				}

				deletePaymentError := db.RestoreTicketsAndDeletePayment(ctx, deletePaymentParams)

				if deletePaymentError != nil {
					allErrors = append(allErrors, deletePaymentError.Error())
				}

				return nil, paymentResponse, fmt.Errorf("encountered errors:\n%s", strings.Join(allErrors, "\n"))

			case string(stripe.PaymentIntentStatusProcessing):
				paymentResponse.Message = "payment processing, we'll send you an email once payment succeeded"

			case string(stripe.PaymentIntentStatusRequiresPaymentMethod):
				paymentResponse.Message = "please submit new payment method"
		}
	} else {
		// TODO: Send email for payment confirmation with ticket reservations.
	}

	updatePaymentParams := database.UpdatePaymentParams {
		Amount: fmt.Sprintf("%.2f", float64(totalPrice)/100.0),
		Status: paymentResponse.Status,
		PaymentIntentID: common.StringToNullString(paymentIntentId),
		ID: userPayment.ID,
		UserID: userId,
	}

	_, updatePaymentError := db.UpdatePayment(ctx, updatePaymentParams)

	if updatePaymentError != nil {
		allErrors = append(allErrors, updatePaymentError.Error())
	}

	// This ensures that if there were any errors, we still return the tickets that were created successfully.
	if len(allErrors) > 0 {
		return newReservations, paymentResponse, fmt.Errorf("encountered errors:\n%s", strings.Join(allErrors, "\n"))
	}

	return newReservations, paymentResponse, nil
}