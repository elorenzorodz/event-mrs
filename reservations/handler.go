package reservations

import (
	"context"
	"fmt"
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

func SaveReservations(db *database.Queries, ctx context.Context, userId uuid.UUID, userEmail string, reservations ReservationParameters) ([]Reservation, string, string, error) {
	totalTickets := 0

	for _, eventDetailReservation := range reservations.EventDetailReservations {
		totalTickets += eventDetailReservation.Quantity
	}

	if totalTickets == 0 {
		return nil, "", "", fmt.Errorf("no tickets being reserved")
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
	paymentId := reservations.PaymentID

	if strings.TrimSpace(paymentId) != "" {
		paymentuuid, parsePaymentIdError := uuid.Parse(paymentId)

		if parsePaymentIdError != nil {
			return newReservations, "", "", fmt.Errorf("invalid payment id")
		}

		getPaymentByIdParams := database.GetPaymentByIdParams {
			ID: paymentuuid,
			UserID: userId,
		}

		currentPayment, getPaymentByIdError := db.GetPaymentById(ctx, getPaymentByIdParams)
		
		if getPaymentByIdError != nil {
			return newReservations, "", "", fmt.Errorf("cannot get payment: %w", getPaymentByIdError)
		}

		userPayment = payments.DatabasePaymentToPaymentJSON(currentPayment)
	} else {
		// Create initial payment and default amount to zero first.
		createPaymentParams := database.CreatePaymentParams {
			ID: uuid.New(),
			Amount: "0.00",
			Currency: currency,
			Status: "pending",
			UserID: userId,
		}

		newPayment, createPaymentError := db.CreatePayment(ctx, createPaymentParams)

		if createPaymentError != nil {
			return newReservations, "", "", fmt.Errorf("cannot process payment")
		}

		userPayment = payments.DatabasePaymentToPaymentJSON(newPayment)
	}

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

	var allErrors []string

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

		return nil, "", "", fmt.Errorf("encountered errors:\n%s", strings.Join(allErrors, "\n"))
	}

	paymentMessages := []string{}
	paymentIntentId := ""
	paymentNextAction := ""
	paymentClientSecret := ""
	paymentStatus := "succeeded"

	if totalPrice > 0 {
		// Tickets reserved are not free.
		stripe.Key = common.GetEnvVariable("STRIPE_SECRET_KEY")
		var paymentIntentResult *stripe.PaymentIntent
		var paymentIntentError error

		paymentIntentParams := &stripe.PaymentIntentParams{
				Amount: stripe.Int64(totalPrice),
				Currency: stripe.String(strings.ToLower(userPayment.Currency)),
				Confirm: stripe.Bool(true),
				PaymentMethod: stripe.String(reservations.PaymentMethodID),
				Metadata: map[string]string{"payment_id": userPayment.ID.String()},
			}

		if strings.TrimSpace(userPayment.PaymentIntentID) == "" {
			paymentIntentResult, paymentIntentError = paymentintent.New(paymentIntentParams)
		} else {
			paymentIntentResult, paymentIntentError = paymentintent.Update(userPayment.PaymentIntentID, paymentIntentParams)
		}

		if paymentIntentError != nil {
			paymentStatus = "stripe payment error"
			paymentMessages = append(paymentMessages, paymentIntentError.Error())
		} else {
			if paymentIntentResult != nil {
				paymentStatus = string(paymentIntentResult.Status)
				paymentIntentId = paymentIntentResult.ID
				paymentMessages = append(paymentMessages, "payment successful")

				if paymentStatus == string(stripe.PaymentIntentStatusRequiresAction) {
					paymentClientSecret = paymentIntentResult.ClientSecret

					if paymentIntentResult.NextAction != nil {
						paymentNextAction = string(paymentIntentResult.NextAction.Type)
					}
				}
			}
		}
	}
	
	if paymentStatus != "succeeded" {
		// TODO: Handle other status.
		switch paymentStatus {
			case string(stripe.PaymentIntentStatusRequiresAction):
				paymentMessages = append(paymentMessages, fmt.Sprintf("complete payment within next 15 minutes, next_action: %s", paymentNextAction))

			case string(stripe.PaymentIntentStatusCanceled):
				paymentMessages = append(paymentMessages, fmt.Sprintf("payment expired, please rebook your tickets, status: %s", paymentStatus))

				deletePaymentParams := database.RestoreTicketsAndDeletePaymentParams {
					PaymentID: userPayment.ID,
					UserID: userId,
				}

				deletePaymentError := db.RestoreTicketsAndDeletePayment(ctx, deletePaymentParams)

				if deletePaymentError != nil {
					allErrors = append(allErrors, deletePaymentError.Error())
				}

				return nil, strings.Join(paymentMessages, "\n"), "", fmt.Errorf("encountered errors:\n%s", strings.Join(allErrors, "\n"))

			case string(stripe.PaymentIntentStatusProcessing):
				paymentMessages = append(paymentMessages, "payment processing, we'll send you an email once payment succeeded")

			case string(stripe.PaymentIntentStatusRequiresPaymentMethod):
				paymentMessages = append(paymentMessages, fmt.Sprintf("please submit new payment method, status: %s", paymentStatus))
			
			default:
				paymentMessages = append(paymentMessages, fmt.Sprintf("next action: %s, status: %s", paymentNextAction, paymentStatus))
		}
	}

	updatePaymentParams := database.UpdatePaymentParams {
		Amount: fmt.Sprintf("%.2f", float64(totalPrice)/100.0),
		Status: paymentStatus,
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
		return newReservations, strings.Join(paymentMessages, "\n"), paymentClientSecret, fmt.Errorf("encountered errors:\n%s", strings.Join(allErrors, "\n"))
	}

	return newReservations, strings.Join(paymentMessages, "\n"), paymentClientSecret, nil
}