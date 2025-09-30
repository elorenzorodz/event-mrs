package reservations

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/elorenzorodz/event-mrs/common"
	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/google/uuid"
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
				// Do not proceed if remaining tickets is already zero.
				eventDetail, getEventDetailByIdError := db.GetEventDetailsById(context, edReservation.EventDetailID)

				if getEventDetailByIdError != nil {
					errorChannel <- fmt.Errorf("error checking event detail tickets remaining: %w", getEventDetailByIdError)

					return
				}

				if eventDetail.TicketsRemaining < 1 {
					errorChannel <- fmt.Errorf("no remaining tickets for: %s, showing on: %s", eventDetail.TicketDescription, eventDetail.ShowDate)

					return
				}

				price, priceParseError := strconv.ParseFloat(eventDetail.Price, 64)

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
		// TODO: Process Stripe payment here.

		updatePaymentParams := database.UpdatePaymentParams {
			Amount: strconv.FormatFloat(totalAmount, 'f', -1, 64),
			Status: "",
		}

		_, updatePaymentError := db.UpdatePayment(context, updatePaymentParams)

		if updatePaymentError != nil {
			errorChannel <- fmt.Errorf("failed to update payment record: %w", updatePaymentError)

			return
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