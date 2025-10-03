package reservations

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/elorenzorodz/event-mrs/common"
	"github.com/elorenzorodz/event-mrs/internal/database"
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

func SaveReservations(db *database.Queries, context context.Context, userId uuid.UUID, userEmail string, reservations ReservationParameters) ([]Reservation, error) {
	var (
		newReservations []Reservation
		totalAmount		float64
		mutex           sync.Mutex
		waitGroup       sync.WaitGroup
		errorChannel    = make(chan error, len(reservations.EventDetailReservations))
	)

	currentDateTime := time.Now()
	currency := reservations.Currency

	if strings.TrimSpace(currency) == "" {
		currency = "usd"
	}

	// Create initial payment and default amount to zero first.
	createPaymentParams := database.CreatePaymentParams {
		ID: uuid.New(),
		Amount: "0.00",
		Currency: currency,
		Status: "pending",
		UserID: userId,
	}

	newPayment, createPaymentError := db.CreatePayment(context, createPaymentParams)

	if createPaymentError != nil {
		return newReservations, fmt.Errorf("cannot process payment")
	}

	for _, eventDetailReservation := range reservations.EventDetailReservations {
		// Capture loop variable
		edReservation := eventDetailReservation

		emailReservation := edReservation.Email

		if strings.TrimSpace(emailReservation) == "" {
			emailReservation = userEmail
		}

		for x := 0; x < edReservation.Quantity; x++ {
			waitGroup.Go(func() {
				// Get event details for additional checking.
				eventDetail, getEventDetailByIdError := db.GetEventDetailsById(context, edReservation.EventDetailID)

				if getEventDetailByIdError != nil {
					errorChannel <- fmt.Errorf("error checking event detail tickets remaining: %w", getEventDetailByIdError)

					return
				}

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

				price, priceParseError := strconv.ParseFloat(eventDetail.Price, 64)

				// Cannot process the price.
				if priceParseError != nil {
					errorChannel <- fmt.Errorf("error processing price of ticket %s, price: %s, showing on: %s", eventDetail.TicketDescription, eventDetail.Price, eventDetail.ShowDate)

					return
				}

				totalAmount += price

				reserveTicketParams := database.ReserveTicketParams{
					Column1: edReservation.EventDetailID,
					Column2: uuid.New(),
					Column3: emailReservation,
					Column4: userId,
					Column5: newPayment.ID,
				}

				newReservation, reserveTicketError := db.ReserveTicket(context, reserveTicketParams)

				if reserveTicketError != nil {
					errorChannel <- fmt.Errorf("error reserving ticket: %w", reserveTicketError)

					return
				}

				mutex.Lock()
				newReservations = append(newReservations, DatabaseReservationToReservationJSON(newReservation))
				mutex.Unlock()
			})
		}
	}

	waitGroup.Go(func() {
		var totalAmountString string
		paymentStatus := "succeeded"

		if totalAmount > 0 {
			// Tickets reserved are free.
			totalAmountString = strconv.FormatFloat(totalAmount, 'f', -1, 64)
			stripeTotalAmount, totalAmountParseError := strconv.ParseInt(strings.ReplaceAll(totalAmountString, ".", ""), 10, 64)
			
			if totalAmountParseError != nil {
				errorChannel <- fmt.Errorf("failed to process payment: %w", totalAmountParseError)

				return
			}

			stripe.Key = common.GetEnvVariable("STRIPE_SECRET_KEY")
			confirmed := true

			paymentIntentParams := &stripe.PaymentIntentParams{
				Amount: &stripeTotalAmount,
				Currency: &newPayment.Currency,
				Confirm: &confirmed,
				PaymentMethod: &reservations.PaymentMethodId,
			}

			newPaymentIntentResult, createPaymentIntentError := paymentintent.New(paymentIntentParams)

			if createPaymentIntentError != nil {
				errorChannel <- fmt.Errorf("failed to process payment: %w", createPaymentIntentError)

				return
			}

			paymentStatus = string(newPaymentIntentResult.Status)
		}
		
		if len(newReservations) == 0 {
			// No tickets reserved.
			deletePaymentParams := database.DeletePaymentParams {
				ID: newPayment.ID,
				UserID: userId,
			}

			deletePaymentError := db.DeletePayment(context, deletePaymentParams)

			if deletePaymentError != nil {
				errorChannel <- fmt.Errorf("failed to clean up payment record: %w", deletePaymentError)

				return
			}
		} else {
			updatePaymentParams := database.UpdatePaymentParams {
				Amount: totalAmountString,
				Status: paymentStatus,
				ID: newPayment.ID,
				UserID: userId,
			}

			_, updatePaymentError := db.UpdatePayment(context, updatePaymentParams)

			if updatePaymentError != nil {
				errorChannel <- fmt.Errorf("failed to update payment record: %w", updatePaymentError)

				return
			}
		}
		
	})

	go func() {
		waitGroup.Wait()
		close(errorChannel)
	}()

	var allErrors []string

	for err := range errorChannel {
		if err != nil {
			allErrors = append(allErrors, err.Error())
		}
	}

	// This ensures that if there were any errors, we still return the tickets that were created successfully.
	if len(allErrors) > 0 {
		return newReservations, fmt.Errorf("encountered errors:\n%s", strings.Join(allErrors, "\n"))
	}

	return newReservations, nil
}