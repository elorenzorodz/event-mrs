-- name: CreatePayment :one
INSERT INTO payments (id, amount, currency, status, expires_at, user_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, payment_intent_id, amount, currency, status, expires_at, created_at, updated_at, user_id;

-- name: UpdatePayment :one
UPDATE payments
SET amount = $1, status = $2, updated_at = NOW(), payment_intent_id = $3
WHERE id = $4 AND user_id = $5
RETURNING id, payment_intent_id, amount, currency, status, expires_at, created_at, updated_at, user_id;

-- name: RestoreTicketsAndDeletePayment :exec
WITH counts AS (
  SELECT event_detail_id, COUNT(*) AS cnt
  FROM reservations
  WHERE payment_id = @payment_id::uuid
  GROUP BY event_detail_id
),
updated AS (
  UPDATE event_details ed
  SET tickets_remaining = ed.tickets_remaining + c.cnt
  FROM counts c
  WHERE ed.id = c.event_detail_id
  RETURNING ed.id
)
DELETE FROM payments
WHERE id = @payment_id::uuid
  AND user_id = @user_id::uuid
  AND EXISTS (SELECT 1 FROM updated);

-- name: GetPaymentById :one
SELECT * FROM payments WHERE id = $1 AND user_id = $2;

-- name: GetPaymentByIdOnly :one
SELECT * FROM payments WHERE id = $1;

-- name: GetUserPayments :many
SELECT * FROM payments WHERE user_id = $1;

-- name: GetPaymentAndReservationDetails :many
SELECT 
	p.id AS payment_id,
	p.payment_intent_id,
  	p.user_id, 
	p.amount,
	p.status,
	r.id AS reservation_id,
	r.email,
	ed.id AS event_detail_id,
	e.title,
	ed.ticket_description,
	ed.show_date,
	ed.price 
FROM payments AS p 
LEFT JOIN reservations AS r
ON r.payment_id = p.id 
LEFT JOIN event_details AS ed
ON ed.id = r.event_detail_id 
LEFT JOIN events AS e
ON e.id = ed.event_id 
WHERE 
	p.id = $1
	AND p.user_id = $2;

-- name: RefundPaymentAndRestoreTickets :exec
WITH event_details_update AS (
	UPDATE event_details AS ed
	SET tickets_remaining = ed.tickets_remaining + 1 
	WHERE id = @event_detail_id::uuid
  RETURNING ed.id
),
payment_update AS (
	UPDATE payments AS p
	SET amount = p.amount - @amount::numeric
	WHERE id = @payment_id::uuid AND user_id = @user_id::uuid
  RETURNING p.id
)
DELETE FROM reservations 
WHERE id = @reservation_id::uuid
	AND EXISTS (SELECT 1 FROM event_details_update)
	AND EXISTS (SELECT 1 FROM payment_update);