package config

import (
	"context"
	"database/sql"

	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/google/uuid"
)

type DBQueries interface {
	CreateEvent(ctx context.Context, arg database.CreateEventParams) (database.Event, error)
	CreateEventDetail(ctx context.Context, arg database.CreateEventDetailParams) (database.EventDetail, error)
	CreatePayment(ctx context.Context, arg database.CreatePaymentParams) (database.Payment, error)
	CreatePaymentLog(ctx context.Context, arg database.CreatePaymentLogParams) (database.PaymentLog, error)
	CreateUser(ctx context.Context, arg database.CreateUserParams) (database.User, error)
	DeleteEvent(ctx context.Context, arg database.DeleteEventParams) error
	DeleteEventDetail(ctx context.Context, arg database.DeleteEventDetailParams) error
	GetEventConfirmedUserReservations(ctx context.Context, id uuid.UUID) ([]database.GetEventConfirmedUserReservationsRow, error)
	GetEventDetailsByEventId(ctx context.Context, eventID []uuid.UUID) ([]database.EventDetail, error)
	GetEventDetailsById(ctx context.Context, id uuid.UUID) (database.EventDetail, error)
	GetEventDetailsWithTitleByIds(ctx context.Context, id []uuid.UUID) ([]database.GetEventDetailsWithTitleByIdsRow, error)
	GetEvents(ctx context.Context, arg database.GetEventsParams) ([]database.GetEventsRow, error)
	GetMultiplePayments(ctx context.Context, id []uuid.UUID) ([]database.Payment, error)
	GetPaidEventDetailForRefund(ctx context.Context, arg database.GetPaidEventDetailForRefundParams) ([]database.GetPaidEventDetailForRefundRow, error)
	GetPaidEventForRefund(ctx context.Context, arg database.GetPaidEventForRefundParams) ([]database.GetPaidEventForRefundRow, error)
	GetPaymentAndReservationDetails(ctx context.Context, arg database.GetPaymentAndReservationDetailsParams) ([]database.GetPaymentAndReservationDetailsRow, error)
	GetPaymentById(ctx context.Context, arg database.GetPaymentByIdParams) (database.Payment, error)
	GetPaymentByIdOnly(ctx context.Context, id uuid.UUID) (database.Payment, error)
	GetPaymentByPaymentIntentId(ctx context.Context, paymentIntentID sql.NullString) (database.Payment, error)
	GetUserByEmail(ctx context.Context, email string) (database.User, error)
	GetUserById(ctx context.Context, id uuid.UUID) (database.User, error)
	GetUserEventById(ctx context.Context, arg database.GetUserEventByIdParams) (database.Event, error)
	GetUserEvents(ctx context.Context, userID uuid.UUID) ([]database.Event, error)
	GetUserPayments(ctx context.Context, userID uuid.UUID) ([]database.Payment, error)
	GetUserReservationById(ctx context.Context, arg database.GetUserReservationByIdParams) (database.Reservation, error)
	GetUserReservations(ctx context.Context, userID uuid.UUID) ([]database.Reservation, error)
	GetUserReservationsByPaymentId(ctx context.Context, arg database.GetUserReservationsByPaymentIdParams) ([]database.Reservation, error)
	RefundPaymentAndRestoreTickets(ctx context.Context, arg database.RefundPaymentAndRestoreTicketsParams) error
	ReserveTicket(ctx context.Context, arg database.ReserveTicketParams) (database.Reservation, error)
	RestoreTicketsAndDeletePayment(ctx context.Context, arg database.RestoreTicketsAndDeletePaymentParams) error
	UpdateEvent(ctx context.Context, arg database.UpdateEventParams) (database.Event, error)
	UpdateEventDetail(ctx context.Context, arg database.UpdateEventDetailParams) (database.EventDetail, error)
	UpdatePayment(ctx context.Context, arg database.UpdatePaymentParams) (database.Payment, error)
	UpdateTicketsRemaining(ctx context.Context, arg database.UpdateTicketsRemainingParams) (database.EventDetail, error)
	UpdateUserReservationEmail(ctx context.Context, arg database.UpdateUserReservationEmailParams) (database.Reservation, error)
}