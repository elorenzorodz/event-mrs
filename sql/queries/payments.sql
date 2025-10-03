-- name: CreatePayment :one
INSERT INTO payments (id, amount, currency, status, user_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, amount, currency, status, user_id;

-- name: UpdatePayment :one
UPDATE payments
SET amount = $1, status = $2, updated_at = NOW()
WHERE id = $3 AND user_id = $4
RETURNING id, amount, currency, status, user_id;

-- name: DeletePayment :exec
DELETE FROM payments
WHERE id = $1 AND user_id = $2;