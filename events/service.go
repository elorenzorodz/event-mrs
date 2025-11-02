package events

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/elorenzorodz/event-mrs/event_details"
	"github.com/elorenzorodz/event-mrs/internal/convert"
	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/elorenzorodz/event-mrs/internal/mailer"
	"github.com/elorenzorodz/event-mrs/internal/sqlutil"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v83"
	"github.com/stripe/stripe-go/v83/paymentintent"
	"github.com/stripe/stripe-go/v83/refund"
)

var (
	ErrEventNotFound = errors.New("event not found or unauthorized")
	ErrDatabase      = errors.New("internal database error")
)

type StripeClient interface {
	Refund(amount int64, paymentIntentID string) (*stripe.Refund, error)
	CancelPaymentIntent(paymentIntentID string) error
}

type StripeAPIClient struct{}

func (stripeAPIClient *StripeAPIClient) Refund(amount int64, paymentIntentID string) (*stripe.Refund, error) {
	refundParams := &stripe.RefundParams{
		Amount:        stripe.Int64(amount),
		PaymentIntent: stripe.String(paymentIntentID),
	}
	return refund.New(refundParams)
}

func (stripeAPIClient *StripeAPIClient) CancelPaymentIntent(paymentIntentID string) error {
	paymentIntentCancelParams := &stripe.PaymentIntentCancelParams{
		CancellationReason: stripe.String("abandoned"),
	}

	_, err := paymentintent.Cancel(paymentIntentID, paymentIntentCancelParams)

	return err
}

type EventService interface {
	Create(ctx context.Context, ownerID uuid.UUID, req CreateEventRequest) (*Event, error)
	GetEventsByOwner(ctx context.Context, ownerID uuid.UUID) ([]Event, error)
	GetEventByID(ctx context.Context, eventID, ownerID uuid.UUID) (*Event, error)
	Update(ctx context.Context, eventID, ownerID uuid.UUID, req UpdateEventRequest) (*Event, error)
	Delete(ctx context.Context, eventID, ownerID uuid.UUID, userEmail string) (*DeleteSummary, error)
	SearchEvents(ctx context.Context, searchQuery, startShowDateQuery, endShowDateQuery string) ([]SearchEventResponse, error)
}

type Service struct {
	DBQueries database.Queries
	Mailer    *mailer.Mailer
	Stripe    StripeClient
}

func NewService(dbQueries database.Queries, mMailer *mailer.Mailer, stripeClient StripeClient) EventService {
	return &Service{
		DBQueries: dbQueries,
		Mailer:    mMailer,
		Stripe:    stripeClient,
	}
}

func (service *Service) Create(ctx context.Context, userID uuid.UUID, createEventRequest CreateEventRequest) (*Event, error) {
	createEventParams := database.CreateEventParams{
		ID:          uuid.New(),
		UserID:      userID,
		Title:       createEventRequest.Title,
		Description: createEventRequest.Description,
		Organizer:   sqlutil.StringToNullString(createEventRequest.Organizer),
	}

	newEvent, createEventError := service.DBQueries.CreateEvent(ctx, createEventParams)

	if createEventError != nil {
		log.Printf("Error creating event: %v", createEventError)

		return nil, ErrDatabase
	}

	newTickets, createTicketsError := service.saveEventTickets(ctx, newEvent.ID, createEventRequest.Tickets)

	if createTicketsError != nil {
		log.Printf("Partial error creating event tickets for event %s: %v", newEvent.ID, createTicketsError)
	}

	event := databaseEventToDomain(newEvent)
	event.Tickets = newTickets

	return event, createTicketsError
}

func (service *Service) GetEventsByOwner(ctx context.Context, userID uuid.UUID) ([]Event, error) {
	userEvents, getUserEventsError := service.DBQueries.GetUserEvents(ctx, userID)

	if getUserEventsError != nil {
		return nil, ErrDatabase
	}

	eventIdArray := make([]uuid.UUID, len(userEvents))

	for i, event := range userEvents {
		eventIdArray[i] = event.ID
	}

	eventDetails, getEventDetailsError := service.DBQueries.GetEventDetailsByEventId(ctx, eventIdArray)

	eventDetailsMap := make(map[uuid.UUID][]event_details.EventDetail)

	if getEventDetailsError != nil {
		log.Printf("error retrieving event details: %v", getEventDetailsError)
	} else {
		for _, detail := range eventDetails {
			eventDetailJSON := databaseEventDetailToEventDetailJSON(detail)
			eventDetailsMap[detail.EventID] = append(eventDetailsMap[detail.EventID], eventDetailJSON)
		}
	}

	events := make([]Event, len(userEvents))

	for i, dbEvent := range userEvents {
		event := databaseEventToDomain(dbEvent)
		event.Tickets = eventDetailsMap[dbEvent.ID]
		events[i] = *event
	}

	return events, nil
}

func (service *Service) GetEventByID(ctx context.Context, eventID, userID uuid.UUID) (*Event, error) {
	getUserEventByIdParams := database.GetUserEventByIdParams{
		ID:     eventID,
		UserID: userID,
	}

	getUserEvent, getUserEventByIdError := service.DBQueries.GetUserEventById(ctx, getUserEventByIdParams)

	if getUserEventByIdError == sql.ErrNoRows {
		return nil, ErrEventNotFound
	}

	if getUserEventByIdError != nil {
		return nil, ErrDatabase
	}

	eventDetails, getEventDetailsError := service.DBQueries.GetEventDetailsByEventId(ctx, []uuid.UUID{getUserEvent.ID})

	eventDetailsList := []event_details.EventDetail{}

	if getEventDetailsError != nil {
		log.Printf("error retrieving event details: %v", getEventDetailsError)
	} else {
		for _, detail := range eventDetails {
			eventDetailsList = append(eventDetailsList, databaseEventDetailToEventDetailJSON(detail))
		}
	}

	event := databaseEventToDomain(getUserEvent)
	event.Tickets = eventDetailsList

	return event, nil
}

func (service *Service) Update(ctx context.Context, eventID, ownerID uuid.UUID, req UpdateEventRequest) (*Event, error) {
	updateEventParams := database.UpdateEventParams{
		ID:          eventID,
		Title:       req.Title,
		Description: req.Description,
		Organizer:   sqlutil.StringToNullString(req.Organizer),
		UserID:      ownerID,
	}

	updatedEvent, updatedEventError := service.DBQueries.UpdateEvent(ctx, updateEventParams)

	if updatedEventError == sql.ErrNoRows {
		return nil, ErrEventNotFound
	}

	if updatedEventError != nil {
		return nil, ErrDatabase
	}

	eventConfirmedUserReservations, getEventConfirmedUserReservations := service.DBQueries.GetEventConfirmedUserReservations(ctx, eventID)

	var waitGroup sync.WaitGroup

	if getEventConfirmedUserReservations != nil {
		log.Printf("error getting users reserved for the event: %v", getEventConfirmedUserReservations)
	} else {
		for _, res := range eventConfirmedUserReservations {
			reservation := res
			recipientName := reservation.Fullname.String

			waitGroup.Go(func() {
				sendUpdatedEventNotificationError := service.Mailer.SendUpdatedEventNotification(
					recipientName,
					reservation.Email,
					updatedEvent.Title,
					updatedEvent.Description,
					updatedEvent.Organizer.String,
				)

				if sendUpdatedEventNotificationError != nil {
					log.Printf("error sending updated event notification to %s: %v", reservation.Email, sendUpdatedEventNotificationError)
				}
			})
		}

		waitGroup.Wait()
	}

	return databaseEventToDomain(updatedEvent), nil
}

func (service *Service) Delete(ctx context.Context, eventID, ownerID uuid.UUID, userEmail string) (*DeleteSummary, error) {
	eventFailedRefundOrCancels, failedNotificationEmails, refundCancelPaymentErrors := service.eventRefundOrCancelPayment(ctx, eventID, ownerID, userEmail)

	if refundCancelPaymentErrors != nil {
		log.Printf("Error during refund/cancellation process: %v", refundCancelPaymentErrors)
	}

	deleteEventParams := database.DeleteEventParams{
		ID:     eventID,
		UserID: ownerID,
	}

	deleteEventError := service.DBQueries.DeleteEvent(ctx, deleteEventParams)

	if deleteEventError == sql.ErrNoRows {
		return nil, ErrEventNotFound
	}
	if deleteEventError != nil {
		log.Printf("Error deleting event %s: %v", eventID, deleteEventError)
		return nil, ErrDatabase
	}

	return &DeleteSummary{
		EventFailedRefundOrCancels: eventFailedRefundOrCancels,
		FailedNotificationEmails:   failedNotificationEmails,
	}, nil
}

func (service *Service) SearchEvents(ctx context.Context, searchQuery, startShowDateQuery, endShowDateQuery string) ([]SearchEventResponse, error) {
	if strings.TrimSpace(searchQuery) == "" {
		searchQuery = "%%"
	} else {
		searchQuery = fmt.Sprintf("%s%s%s", "%", strings.ToLower(searchQuery), "%")
	}

	currentDateTime := time.Now().UTC()

	var startShowDate time.Time
	if strings.TrimSpace(startShowDateQuery) == "" {
		startShowDate = time.Date(currentDateTime.Year(), currentDateTime.Month(), currentDateTime.Day(), 0, 0, 0, 0, currentDateTime.Location())
	} else {
		startShowDateQuery = fmt.Sprintf("%s 00:00", startShowDateQuery)
		parsedShowDate, _, parseShowDateError := convert.StringToTime(startShowDateQuery)

		if parseShowDateError != nil {
			return nil, errors.New("invalid start show date format")
		}

		startShowDate = parsedShowDate
	}

	var endShowDate time.Time
	if strings.TrimSpace(endShowDateQuery) == "" {
		firstDayOfNextMonth := time.Date(currentDateTime.Year(), currentDateTime.Month()+1, 1, 0, 0, 0, 0, currentDateTime.Location())
		lastDayOfCurrentMonth := firstDayOfNextMonth.AddDate(0, 0, -1)
		endShowDate = time.Date(currentDateTime.Year(), currentDateTime.Month(), lastDayOfCurrentMonth.Day(), 23, 59, 59, 0, currentDateTime.Location())
	} else {
		endShowDateQuery = fmt.Sprintf("%s 23:59", endShowDateQuery)
		parsedEndDate, _, parseEndDateError := convert.StringToTime(endShowDateQuery)

		if parseEndDateError != nil {
			return nil, errors.New("invalid end show date format")
		}

		endShowDate = parsedEndDate
	}

	getEventsParam := database.GetEventsParams{
		Title:       searchQuery,
		Description: searchQuery,
		Organizer:   sqlutil.StringToNullString(searchQuery),
		ShowDate:    startShowDate,
		ShowDate_2:  endShowDate,
	}

	getSearchEvents, getEventsError := service.DBQueries.GetEvents(ctx, getEventsParam)

	if getEventsError != nil {
		log.Printf("Error searching events: %v", getEventsError)

		return nil, ErrDatabase
	}

	return databaseSearchEventsToSearchEventsResponse(getSearchEvents), nil
}

func databaseEventToDomain(databaseEvent database.Event) *Event {
	var updatedAt *time.Time

	if databaseEvent.UpdatedAt.Valid {
		updatedAt = &databaseEvent.UpdatedAt.Time
	}

	return &Event{
		ID:          databaseEvent.ID,
		UserID:      databaseEvent.UserID,
		Title:       databaseEvent.Title,
		Description: databaseEvent.Description,
		Organizer:   databaseEvent.Organizer.String,
		CreatedAt:   databaseEvent.CreatedAt,
		UpdatedAt:   updatedAt,
	}
}

func databaseEventDetailToEventDetailJSON(detail database.EventDetail) event_details.EventDetail {
	priceFloat, _ := convert.StringToFloat32(detail.Price)

	return event_details.EventDetail{
		ID:                detail.ID,
		ShowDate:          detail.ShowDate,
		Price:             priceFloat,
		NumberOfTickets:   detail.NumberOfTickets,
		TicketDescription: detail.TicketDescription,
		CreatedAt:         detail.CreatedAt,
		UpdatedAt:         sqlutil.NullTimeToString(detail.UpdatedAt),
		EventID:           detail.EventID,
	}
}

func databaseSearchEventsToSearchEventsResponse(databaseSearchEvents []database.GetEventsRow) []SearchEventResponse {
	searchEvents := make([]SearchEventResponse, len(databaseSearchEvents))

	for i, databaseSearchEvent := range databaseSearchEvents {

		price, _ := convert.StringToFloat32(databaseSearchEvent.Price.String)

		searchEvents[i] = SearchEventResponse{
			EventID:           databaseSearchEvent.EventID,
			Title:             databaseSearchEvent.Title,
			Description:       databaseSearchEvent.Description,
			Organizer:         databaseSearchEvent.Organizer.String,
			EventDetailID:     databaseSearchEvent.EventDetailID.UUID,
			ShowDate:          databaseSearchEvent.ShowDate.Time,
			Price:             price,
			NumberOfTickets:   databaseSearchEvent.NumberOfTickets.Int32,
			TicketDescription: databaseSearchEvent.TicketDescription.String,
		}
	}

	return searchEvents
}

func (service *Service) saveEventTickets(ctx context.Context, eventId uuid.UUID, tickets []event_details.EventDetailParameters) ([]event_details.EventDetail, error) {
	var (
		newTickets   []event_details.EventDetail
		mutex        sync.Mutex
		waitGroup    sync.WaitGroup
		errorChannel = make(chan error, len(tickets))
	)

	for _, ticket := range tickets {
		tkt := ticket

		waitGroup.Go(func() {
			showDate, referenceFormat, parseShowDateError := convert.StringToTime(tkt.ShowDate)

			if parseShowDateError != nil {
				errorChannel <- fmt.Errorf("error parsing show date '%s': expected format %s", tkt.ShowDate, referenceFormat)

				return
			}

			createEventDetailParams := database.CreateEventDetailParams{
				ID:                uuid.New(),
				ShowDate:          showDate,
				Price:             fmt.Sprintf("%.2f", tkt.Price),
				NumberOfTickets:   tkt.NumberOfTickets,
				TicketsRemaining:  tkt.NumberOfTickets,
				TicketDescription: tkt.TicketDescription,
				EventID:           eventId,
			}

			newEventDetail, createEventDetailError := service.DBQueries.CreateEventDetail(ctx, createEventDetailParams)

			if createEventDetailError != nil {
				errorChannel <- fmt.Errorf("error creating event detail: %w", createEventDetailError)

				return
			}

			mutex.Lock()
			newTickets = append(newTickets, databaseEventDetailToEventDetailJSON(newEventDetail))
			mutex.Unlock()
		})
	}

	waitGroup.Wait()
	close(errorChannel)

	var allErrors []string

	for err := range errorChannel {
		if err != nil {
			allErrors = append(allErrors, err.Error())
		}
	}

	if len(allErrors) > 0 {
		return newTickets, fmt.Errorf("encountered errors:\n%s", strings.Join(allErrors, "\n"))
	}

	return newTickets, nil
}

func (service *Service) eventRefundOrCancelPayment(ctx context.Context, eventId uuid.UUID, userId uuid.UUID, userEmail string) ([]EventFailedRefundOrCancel, []FailedNotificationEmail, error) {
	getPaidEventForRefundParams := database.GetPaidEventForRefundParams{
		EventID: eventId,
		UserID:  userId,
	}

	paidEventForRefunds, getRefundEventPaymentError := service.DBQueries.GetPaidEventForRefund(ctx, getPaidEventForRefundParams)

	if getRefundEventPaymentError != nil {
		return nil, nil, fmt.Errorf("failed to get payment and reservations for the event: %w", getRefundEventPaymentError)
	}

	if len(paidEventForRefunds) == 0 {
		return nil, nil, nil
	}

	var (
		PaymentIDs                 []uuid.UUID
		mutex                      sync.Mutex
		waitGroup                  sync.WaitGroup
		eventFailedRefundOrCancels []EventFailedRefundOrCancel
	)

	for _, paidEvent := range paidEventForRefunds {
		paidEventForRefund := paidEvent
		amount, _ := convert.PriceStringToCents(paidEventForRefund.Amount)
		ticketPrice, _ := convert.PriceStringToCents(paidEventForRefund.TicketPrice)
		isErrorOccured := false

		if paidEventForRefund.Status == "refunded" || paidEventForRefund.Status == "cancelled" {
			continue
		}

		if amount == 0 {
			PaymentIDs = append(PaymentIDs, paidEventForRefund.PaymentID)
		} else {
			if amount != ticketPrice {
				amount = ticketPrice
			}
		}

		waitGroup.Go(func() {
			eventFailedRefundOrCancel := EventFailedRefundOrCancel{}
			createPaymentLogParams := database.CreatePaymentLogParams{
				ID:              uuid.New(),
				PaymentIntentID: paidEventForRefund.PaymentIntentID.String,
				Amount:          fmt.Sprintf("%.2f", float64(amount)/100.0),
				UserEmail:       userEmail,
				PaymentID:       paidEventForRefund.PaymentID,
			}

			updatePaymentParams := database.UpdatePaymentParams{
				ID:              paidEventForRefund.PaymentID,
				Amount:          fmt.Sprintf("%.2f", float64(amount)/100.0),
				PaymentIntentID: paidEventForRefund.PaymentIntentID,
				UserID:          userId,
			}

			if paidEventForRefund.Status == string(stripe.PaymentIntentStatusSucceeded) {
				refundResult, refundError := service.Stripe.Refund(amount, paidEventForRefund.PaymentIntentID.String)

				if refundError != nil {
					log.Printf("Stripe refund error: %v", refundError)

					if stripeError, ok := refundError.(*stripe.Error); ok {
						createPaymentLogParams.Status = string(stripeError.Code)
						createPaymentLogParams.Description = sqlutil.StringToNullString(stripeError.Msg)

						eventFailedRefundOrCancel.PaymentID = paidEventForRefund.PaymentID
						eventFailedRefundOrCancel.Action = "stripe refund request"
						eventFailedRefundOrCancel.Code = string(stripeError.Code)
						eventFailedRefundOrCancel.Message = stripeError.Msg

						isErrorOccured = true
					}
				} else {
					createPaymentLogParams.Status = string(refundResult.Status)

					switch refundResult.Status {
						case stripe.RefundStatusFailed:
							createPaymentLogParams.Description = sqlutil.StringToNullString(string(refundResult.FailureReason))
						case stripe.RefundStatusPending:
							createPaymentLogParams.Description = sqlutil.StringToNullString("refund pending")
							updatePaymentParams.Status = "refund pending"
						case stripe.RefundStatusSucceeded:
							createPaymentLogParams.Description = sqlutil.StringToNullString("refund succeeded")
							updatePaymentParams.Status = "refunded"
					}
				}
			} else {
				paymentIntentCancelError := service.Stripe.CancelPaymentIntent(paidEventForRefund.PaymentIntentID.String)

				if paymentIntentCancelError != nil {
					if stripeError, ok := paymentIntentCancelError.(*stripe.Error); ok {
						createPaymentLogParams.Status = string(stripeError.Code)
						createPaymentLogParams.Description = sqlutil.StringToNullString(stripeError.Msg)

						eventFailedRefundOrCancel.PaymentID = paidEventForRefund.PaymentID
						eventFailedRefundOrCancel.Action = "stripe cancel request"
						eventFailedRefundOrCancel.Code = string(stripeError.Code)
						eventFailedRefundOrCancel.Message = stripeError.Msg
						isErrorOccured = true
					}
				} else {
					updatePaymentParams.Status = "cancelled"
					createPaymentLogParams.Status = "cancelled"
					createPaymentLogParams.Description = sqlutil.StringToNullString("event deleted")
				}
			}

			_, createPaymentLogError := service.DBQueries.CreatePaymentLog(ctx, createPaymentLogParams)

			if createPaymentLogError != nil {
				log.Printf("error: create payment log - %s", createPaymentLogError)
			}

			if isErrorOccured {
				mutex.Lock()
				eventFailedRefundOrCancels = append(eventFailedRefundOrCancels, eventFailedRefundOrCancel)
				mutex.Unlock()

				return
			}

			_, updatePaymentError := service.DBQueries.UpdatePayment(ctx, updatePaymentParams)

			if updatePaymentError != nil {
				log.Printf("error: update payment - %s", updatePaymentError)
			}

			mutex.Lock()
			PaymentIDs = append(PaymentIDs, paidEventForRefund.PaymentID)
			mutex.Unlock()
		})
	}

	waitGroup.Wait()

	if len(PaymentIDs) == 0 {
		return eventFailedRefundOrCancels, nil, nil
	}

	payments, _ := service.DBQueries.GetMultiplePayments(ctx, PaymentIDs)

	var (
		sendRefundCanceWaitGroup          sync.WaitGroup
		sendRefundCancelNotifErrorChannel = make(chan error, len(payments))
	)

	for _, pymnt := range payments {
		payment := pymnt

		sendRefundCanceWaitGroup.Go(func() { 

			user, getUserByIdError := service.DBQueries.GetUserById(ctx, payment.UserID)

			if getUserByIdError != nil {
				sendRefundCancelNotifErrorChannel <- fmt.Errorf("failed to get user email for payment %s: %w", payment.ID, getUserByIdError)
				return
			}

			eventTitle := paidEventForRefunds[0].Title
			recipientName := fmt.Sprintf("%s %s", user.Firstname, user.Lastname)

			sendRefundCancelError := service.Mailer.SendRefundOrCancelledEmail(
				recipientName,
				user.Email,
				eventTitle,
				fmt.Sprintf("The event: %s, that you booked was cancelled and your payment was refunded. If you didn't pay yet, the pending payment is now cancelled.", eventTitle),
			)

			if sendRefundCancelError != nil {
				sendRefundCancelNotifErrorChannel <- sendRefundCancelError
			}
		})
	}

	sendRefundCanceWaitGroup.Wait()
	close(sendRefundCancelNotifErrorChannel)

	failedNotificationEmails := []FailedNotificationEmail{}

	for errorMessage := range sendRefundCancelNotifErrorChannel {
		if errorMessage != nil {
			failedNotificationEmails = append(failedNotificationEmails, FailedNotificationEmail{
				SendRefundCancelNotificationError: errorMessage.Error(),
			})
		}
	}

	return eventFailedRefundOrCancels, failedNotificationEmails, nil
}