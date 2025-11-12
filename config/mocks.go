package config

import (
	"context"
	"database/sql"

	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/google/uuid"
)

type UserMock struct{}

func (userMock *UserMock) CreateUser(ctx context.Context, arg database.CreateUserParams) (database.User, error) {
	panic("CreateUser not implemented for this test (BaseMock)")
}

func (userMock *UserMock) GetUserByEmail(ctx context.Context, email string) (database.User, error) {
	return database.User{}, sql.ErrNoRows
}

func (userMock *UserMock) GetUserById(ctx context.Context, id uuid.UUID) (database.User, error) {
	return database.User{}, sql.ErrNoRows
}

type EventMock struct{}

func (eventMock *EventMock) CreateEvent(ctx context.Context, arg database.CreateEventParams) (database.Event, error) {
	panic("CreateEvent not implemented for this test (BaseMock)")
}

func (eventMock *EventMock) DeleteEvent(ctx context.Context, arg database.DeleteEventParams) error {
	panic("DeleteEvent not implemented for this test (BaseMock)")
}

func (eventMock *EventMock) GetEvents(ctx context.Context, arg database.GetEventsParams) ([]database.GetEventsRow, error) {
	return []database.GetEventsRow{}, nil
}

func (eventMock *EventMock) GetUserEventById(ctx context.Context, arg database.GetUserEventByIdParams) (database.Event, error) {
	return database.Event{}, sql.ErrNoRows
}

func (eventMock *EventMock) GetUserEvents(ctx context.Context, userID uuid.UUID) ([]database.Event, error) {
	return []database.Event{}, nil
}

func (eventMock *EventMock) UpdateEvent(ctx context.Context, arg database.UpdateEventParams) (database.Event, error) {
	panic("UpdateEvent not implemented for this test (BaseMock)")
}

type EventDetailMock struct{}

func (eventDetailMock *EventDetailMock) CreateEventDetail(ctx context.Context, arg database.CreateEventDetailParams) (database.EventDetail, error) {
	panic("CreateEventDetail not implemented for this test (BaseMock)")
}

func (eventDetailMock *EventDetailMock) DeleteEventDetail(ctx context.Context, arg database.DeleteEventDetailParams) error {
	panic("DeleteEventDetail not implemented for this test (BaseMock)")
}

func (eventDetailMock *EventDetailMock) GetEventDetailsByEventId(ctx context.Context, eventID []uuid.UUID) ([]database.EventDetail, error) {
	return []database.EventDetail{}, nil
}

func (eventDetailMock *EventDetailMock) GetEventDetailsById(ctx context.Context, id uuid.UUID) (database.EventDetail, error) {
	return database.EventDetail{}, sql.ErrNoRows
}

func (eventDetailMock *EventDetailMock) GetEventDetailsWithTitleByIds(ctx context.Context, id []uuid.UUID) ([]database.GetEventDetailsWithTitleByIdsRow, error) {
	return []database.GetEventDetailsWithTitleByIdsRow{}, nil
}

func (eventDetailMock *EventDetailMock) UpdateEventDetail(ctx context.Context, arg database.UpdateEventDetailParams) (database.EventDetail, error) {
	panic("UpdateEventDetail not implemented for this test (BaseMock)")
}

func (eventDetailMock *EventDetailMock) UpdateTicketsRemaining(ctx context.Context, arg database.UpdateTicketsRemainingParams) (database.EventDetail, error) {
	panic("UpdateTicketsRemaining not implemented for this test (BaseMock)")
}

type PaymentMock struct{}

func (paymentMock *PaymentMock) CreatePayment(ctx context.Context, arg database.CreatePaymentParams) (database.Payment, error) {
	panic("CreatePayment not implemented for this test (BaseMock)")
}

func (paymentMock *PaymentMock) CreatePaymentLog(ctx context.Context, arg database.CreatePaymentLogParams) (database.PaymentLog, error) {
	panic("CreatePaymentLog not implemented for this test (BaseMock)")
}

func (paymentMock *PaymentMock) GetMultiplePayments(ctx context.Context, id []uuid.UUID) ([]database.Payment, error) {
	return []database.Payment{}, nil
}

func (paymentMock *PaymentMock) GetPaidEventDetailForRefund(ctx context.Context, arg database.GetPaidEventDetailForRefundParams) ([]database.GetPaidEventDetailForRefundRow, error) {
	return []database.GetPaidEventDetailForRefundRow{}, nil
}

func (paymentMock *PaymentMock) GetPaidEventForRefund(ctx context.Context, arg database.GetPaidEventForRefundParams) ([]database.GetPaidEventForRefundRow, error) {
	return []database.GetPaidEventForRefundRow{}, nil
}

func (paymentMock *PaymentMock) GetPaymentAndReservationDetails(ctx context.Context, arg database.GetPaymentAndReservationDetailsParams) ([]database.GetPaymentAndReservationDetailsRow, error) {
	return []database.GetPaymentAndReservationDetailsRow{}, nil
}

func (paymentMock *PaymentMock) GetPaymentById(ctx context.Context, arg database.GetPaymentByIdParams) (database.Payment, error) {
	return database.Payment{}, sql.ErrNoRows
}

func (paymentMock *PaymentMock) GetPaymentByIdOnly(ctx context.Context, id uuid.UUID) (database.Payment, error) {
	return database.Payment{}, sql.ErrNoRows
}

func (paymentMock *PaymentMock) GetPaymentByPaymentIntentId(ctx context.Context, paymentIntentID sql.NullString) (database.Payment, error) {
	return database.Payment{}, sql.ErrNoRows
}

func (paymentMock *PaymentMock) GetUserPayments(ctx context.Context, userID uuid.UUID) ([]database.Payment, error) {
	return []database.Payment{}, nil
}

func (paymentMock *PaymentMock) UpdatePayment(ctx context.Context, arg database.UpdatePaymentParams) (database.Payment, error) {
	panic("UpdatePayment not implemented for this test (BaseMock)")
}

type ReservationMock struct{}

func (reservationMock *ReservationMock) GetEventConfirmedUserReservations(ctx context.Context, id uuid.UUID) ([]database.GetEventConfirmedUserReservationsRow, error) {
	return []database.GetEventConfirmedUserReservationsRow{}, nil
}

func (reservationMock *ReservationMock) GetUserReservationById(ctx context.Context, arg database.GetUserReservationByIdParams) (database.Reservation, error) {
	return database.Reservation{}, sql.ErrNoRows
}

func (reservationMock *ReservationMock) GetUserReservations(ctx context.Context, userID uuid.UUID) ([]database.Reservation, error) {
	return []database.Reservation{}, nil
}

func (reservationMock *ReservationMock) GetUserReservationsByPaymentId(ctx context.Context, arg database.GetUserReservationsByPaymentIdParams) ([]database.Reservation, error) {
	return []database.Reservation{}, nil
}

func (reservationMock *ReservationMock) RefundPaymentAndRestoreTickets(ctx context.Context, arg database.RefundPaymentAndRestoreTicketsParams) error {
	panic("RefundPaymentAndRestoreTickets not implemented for this test (BaseMock)")
}

func (reservationMock *ReservationMock) ReserveTicket(ctx context.Context, arg database.ReserveTicketParams) (database.Reservation, error) {
	panic("ReserveTicket not implemented for this test (BaseMock)")
}

func (reservationMock *ReservationMock) RestoreTicketsAndDeletePayment(ctx context.Context, arg database.RestoreTicketsAndDeletePaymentParams) error {
	panic("RestoreTicketsAndDeletePayment not implemented for this test (BaseMock)")
}

func (reservationMock *ReservationMock) UpdateUserReservationEmail(ctx context.Context, arg database.UpdateUserReservationEmailParams) (database.Reservation, error) {
	panic("UpdateUserReservationEmail not implemented for this test (BaseMock)")
}

type BaseMock struct {
	*UserMock
	*EventMock
	*EventDetailMock
	*ReservationMock
	*PaymentMock
}

func NewBaseMock() *BaseMock {
	return &BaseMock{
		UserMock: &UserMock{},
		EventMock: &EventMock{},
		EventDetailMock: &EventDetailMock{},
		ReservationMock: &ReservationMock{},
		PaymentMock: &PaymentMock{},
	}
}