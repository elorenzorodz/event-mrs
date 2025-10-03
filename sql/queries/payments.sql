-- name: CreatePayment :one
INSERT INTO payments (id, amount, currency, status, user_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, amount, currency, status, user_id;

-- name: UpdatePayment :one
UPDATE payments
SET amount = $1, status = $2, updated_at = NOW(), payment_intent_id = $3
WHERE id = $4 AND user_id = $5
RETURNING id, amount, currency, status, user_id;

-- name: RestoreTicketsAndDeletePayment :exec
WITH counts AS (
  SELECT event_detail_id, COUNT(*) AS cnt
  FROM reservations
  WHERE payment_id = @payment_id::uuid
  GROUP BY event_detail_id
),
updated AS (
  -- add back reserved tickets
  UPDATE event_details ed
  SET tickets_remaining = ed.tickets_remaining + c.cnt
  FROM counts c
  WHERE ed.id = c.event_detail_id
  RETURNING ed.id
)
DELETE FROM payments
WHERE id = @payment_id::uuid AND user_id = @user_id::uuid;