package payments

import (
	"context"
	"time"

	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/elorenzorodz/event-mrs/internal/mailer"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v83"
)

type PaymentAPIConfig struct {
	Service PaymentService
}

type Payment struct {
	ID              uuid.UUID `json:"id"`
	PaymentIntentID string    `json:"payment_intent_id"`
	Amount          float64   `json:"amount"`
	Currency        string    `json:"currency"`
	Status          string    `json:"status"`
	ExpiresAt       time.Time `json:"expires_at"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       string    `json:"updated_at"`
	UserID          uuid.UUID `json:"user_id"`
}

type PaymentResponse struct {
	ID           uuid.UUID `json:"id"`
	NextAction   string    `json:"next_action"`
	Status       string    `json:"status"`
	ClientSecret string    `json:"client_secret"`
	Message      string    `json:"message"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type PaymentParameters struct {
	PaymentMethodID string `json:"payment_method_id" binding:"required"`
}

type PaymentRefundResponse struct {
	Message        string            `json:"message"`
	PaymentRefunds []PaymentRefunded `json:"payment_refunded"`
}

type PaymentRefunded struct {
	PaymentID         uuid.UUID `json:"payment_id"`
	Amount            string    `json:"amount_refunded"`
	Title             string    `json:"title"`
	TicketDescription string    `json:"ticket_description"`
	ShowDate          time.Time `json:"show_date"`
}

type StripeClient interface {
	UpdatePaymentIntent(paymentIntentID string, params *stripe.PaymentIntentParams) (*stripe.PaymentIntent, error)
	CreateRefund(params *stripe.RefundParams) (*stripe.Refund, error)
	ConstructEvent(payload []byte, signature string, secret string) (stripe.Event, error)
}

type PaymentService interface {
	GetUserPayments(ctx context.Context, userID uuid.UUID) ([]*Payment, error)
	GetUserPaymentById(ctx context.Context, paymentID, userID uuid.UUID) (*Payment, error)
	UpdatePayment(ctx context.Context, paymentID, userID uuid.UUID, paymentMethodID string) (*PaymentResponse, error)
	RefundPayment(ctx context.Context, paymentID, userID uuid.UUID) (*PaymentRefundResponse, error)
	HandleWebhook(ctx context.Context, body []byte, signature string, webhookType string) error
}

type Service struct {
	DB                        *database.Queries
	Stripe                    StripeClient
	Mailer                    *mailer.Mailer
	StripeSigningSecret       string
	StripeRefundSigningSecret string
}

type StripeAPIClient struct{}