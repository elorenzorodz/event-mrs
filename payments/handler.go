package payments

import (
	"github.com/elorenzorodz/event-mrs/common"
	"github.com/elorenzorodz/event-mrs/internal/database"
)

func DatabasePaymentToPaymentJSON(databasePayment database.Payment) Payment {
	return Payment{
		ID:              databasePayment.ID,
		PaymentIntentID: databasePayment.PaymentIntentID.String,
		Amount:          float64(common.StringToFloat32(databasePayment.Amount)),
		Currency:        databasePayment.Currency,
		Status:          databasePayment.Status,
		ExpiresAt:       databasePayment.ExpiresAt,
		CreatedAt:       databasePayment.CreatedAt,
		UpdatedAt:       common.NullTimeToString(databasePayment.UpdatedAt),
		UserID:          databasePayment.UserID,
	}
}