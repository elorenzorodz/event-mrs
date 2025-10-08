package events

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/elorenzorodz/event-mrs/common"
	"github.com/elorenzorodz/event-mrs/event_details"
	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v83"
	"github.com/stripe/stripe-go/v83/paymentintent"
	"github.com/stripe/stripe-go/v83/refund"
)

func DatabaseEventToEventJSON(databaseEvent database.Event, eventDetails []event_details.EventDetail) Event {
	return Event{
		ID:          databaseEvent.ID,
		Title:       databaseEvent.Title,
		Description: databaseEvent.Description,
		Organizer:   databaseEvent.Organizer.String,
		CreatedAt:   databaseEvent.CreatedAt,
		UpdatedAt:   common.NullTimeToString(databaseEvent.UpdatedAt),
		UserID:      databaseEvent.UserID,
		Tickets:	 eventDetails,
	}
}

func DatabaseEventsToEventsJSON(databaseEvents []database.Event, eventDetailsMap map[string][]event_details.EventDetail) []Event {
	events := []Event{}

	for _, databaseEvent := range databaseEvents {
		eventDetails := []event_details.EventDetail{}

		if eventDetailsMap != nil || len(eventDetailsMap) > 0 {
			eventDetails = eventDetailsMap[databaseEvent.ID.String()]
		}

		events = append(events, DatabaseEventToEventJSON(databaseEvent, eventDetails))
	}

	return events
}

func DatabaseSearchEventsToSearchEventsJSON(databaseSearchEvents []database.GetEventsRow) []SearchEvent {
	searchEvents := []SearchEvent{}

	for _, databaseSearchEvent := range databaseSearchEvents {
		searchEvent := SearchEvent {
			EventID: databaseSearchEvent.EventID,
			Title: databaseSearchEvent.Title,
			Description: databaseSearchEvent.Description,
			Organizer: databaseSearchEvent.Organizer.String,
			EventDetailID: databaseSearchEvent.EventID,
			ShowDate: common.NullTimeToString(databaseSearchEvent.ShowDate),
			Price: common.StringToFloat32(databaseSearchEvent.Price.String),
			NumberOfTickets: databaseSearchEvent.NumberOfTickets.Int32,
			TicketDescription: databaseSearchEvent.TicketDescription.String,
		}

		searchEvents = append(searchEvents, searchEvent)
	}

	return searchEvents
}

func SaveEventTickets(db *database.Queries, ctx context.Context, eventId uuid.UUID, tickets []event_details.EventDetailParameters) ([]event_details.EventDetail, error) {
	var (
		newTickets []event_details.EventDetail
		mutex      sync.Mutex
		waitGroup  sync.WaitGroup
		errorChannel = make(chan error, len(tickets))
	)

	for _, ticket := range tickets {
		tkt := ticket // capture loop variable

		waitGroup.Go(func() {

			showDate, referenceFormat, parseShowDateError := common.StringToTime(tkt.ShowDate)

			if parseShowDateError != nil {
				errorChannel <- fmt.Errorf("error parsing show date '%s': expected format %s", tkt.ShowDate, referenceFormat)

				return
			}

			createEventDetailParams := database.CreateEventDetailParams{
				ID:               uuid.New(),
				ShowDate:         showDate,
				Price:            fmt.Sprintf("%.2f", tkt.Price),
				NumberOfTickets:  tkt.NumberOfTickets,
				TicketsRemaining: tkt.NumberOfTickets,
				TicketDescription: tkt.TicketDescription,
				EventID:          eventId,
			}

			newEventDetail, createEventDetailError := db.CreateEventDetail(ctx, createEventDetailParams)

			if createEventDetailError != nil {
				errorChannel <- fmt.Errorf("error creating event detail: %w", createEventDetailError)

				return
			}

			mutex.Lock()
			newTickets = append(newTickets, event_details.DatabaseEventDetailToEventDetailJSON(newEventDetail))
			mutex.Unlock()
		})
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
		return newTickets, fmt.Errorf("encountered errors:\n%s", strings.Join(allErrors, "\n"))
	}

	return newTickets, nil
}

func RefundOrCancelPayment(db *database.Queries, ctx context.Context, eventId uuid.UUID, userId uuid.UUID, userEmail string) (error) {
	getPaidEventForRefundParams := database.GetPaidEventForRefundParams {
		EventID: eventId,
		UserID: userId,
	}

	paidEventForRefunds, getRefundEventPaymentError := db.GetPaidEventForRefund(ctx, getPaidEventForRefundParams)

	if getRefundEventPaymentError != nil {
		return fmt.Errorf("failed to get payment and reservations for the event")
	}

	if len(paidEventForRefunds) == 0 {
		return fmt.Errorf("no tickets reserved for this event")
	}

	var(
		PaymentIDs []uuid.UUID
		mutex      sync.Mutex
		waitGroup  sync.WaitGroup
		errorChannel = make(chan error, len(paidEventForRefunds))
	)

	stripe.Key = common.GetEnvVariable("STRIPE_SECRET_KEY")

	for _, paidEvent := range paidEventForRefunds {
		paidEventForRefund := paidEvent
		amount, _ := common.PriceStringToCents(paidEventForRefund.Amount)

		if amount == 0 {
			PaymentIDs = append(PaymentIDs, paidEventForRefund.PaymentID)
		}

		waitGroup.Go(func() {
			createPaymentLogParams := database.CreatePaymentLogParams {
				ID: uuid.New(),
				PaymentIntentID: paidEventForRefund.PaymentID.String(),
				Amount: fmt.Sprintf("%.2f", float64(amount)/100.0),
				UserEmail: userEmail,
				PaymentID: paidEventForRefund.PaymentID,
			}

			if paidEventForRefund.Status == string(stripe.PaymentIntentStatusSucceeded) || paidEventForRefund.Status == string(stripe.PaymentIntentStatusProcessing) {
				refundParams := &stripe.RefundParams {
					Amount: stripe.Int64(amount),
					PaymentIntent: stripe.String(paidEventForRefund.PaymentIntentID.String),
				}

				refundResult, refundError := refund.New(refundParams)

				if refundError != nil {
					// TODO: Send email to team.
					if stripeError, ok := refundError.(*stripe.Error); ok {
						createPaymentLogParams.Status = string(stripeError.Code)
						createPaymentLogParams.Description = common.StringToNullString(stripeError.Msg)
					}
				}

				createPaymentLogParams.Status = string(refundResult.Status)

				switch refundResult.Status {
					case stripe.RefundStatusFailed:
						createPaymentLogParams.Description = common.StringToNullString(string(refundResult.FailureReason))

					case stripe.RefundStatusPending:
						createPaymentLogParams.Description = common.StringToNullString("refund pending")

					case stripe.RefundStatusSucceeded:
						createPaymentLogParams.Description = common.StringToNullString("refund succeeded")
				}
			} else {
				paymentIntentCancelParams := &stripe.PaymentIntentCancelParams{
					CancellationReason: stripe.String("abandoned"),
				}

				_, paymentIntentCancelError := paymentintent.Cancel(paidEventForRefund.PaymentIntentID.String, paymentIntentCancelParams)

				if paymentIntentCancelError != nil {
					log.Printf("error payment intent cancel: %s", paymentIntentCancelError)
				} else {
					createPaymentLogParams.Status = "cancelled"
					createPaymentLogParams.Description = common.StringToNullString("payment expired")

					_, createPaymentLogError := db.CreatePaymentLog(ctx, createPaymentLogParams)

					if createPaymentLogError != nil {
						if stripeError, ok := createPaymentLogError.(*stripe.Error); ok {
							createPaymentLogParams.Status = string(stripeError.Code)
							createPaymentLogParams.Description = common.StringToNullString(stripeError.Msg)
						}

						log.Printf("error: create payment log - %s", createPaymentLogError)
					}
				}
			}

			_, createPaymentLogError := db.CreatePaymentLog(ctx, createPaymentLogParams)

			if createPaymentLogError != nil {
				log.Printf("error: create payment log - %s", createPaymentLogError)
			}

			mutex.Lock()
			PaymentIDs = append(PaymentIDs, paidEventForRefund.PaymentID)
			mutex.Unlock()
		})
	}

	waitGroup.Wait()
	close(errorChannel)
}