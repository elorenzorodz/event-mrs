package reservations

import (
	"context"
	"fmt"
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
		mutex           sync.Mutex
		waitGroup       sync.WaitGroup
		errorChannel    = make(chan error, len(reservations.EventDetailReservations))
	)

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

				reserveTicketParams := database.ReserveTicketParams{
					Column1: edReservation.EventDetailID,
					Column2: uuid.New(),
					Column3: emailReservation,
					Column4: userId,
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