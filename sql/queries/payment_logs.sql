-- name: CreatePaymentLog :one
INSERT INTO payment_logs (id, status, description, payment_intent_id, payment_method_id, amount, user_email, payment_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, status, description, payment_intent_id, payment_method_id, amount, created_at, user_email, payment_id;