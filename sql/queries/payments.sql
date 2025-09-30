-- name: CreatePayment :one
INSERT INTO payments (id, amount, currency, status, user_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, amount, currency, status, user_id;