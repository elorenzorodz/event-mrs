-- name: CreateEvent :one
INSERT INTO events (id, title, description, organizer, user_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, title, description, organizer, created_at, updated_at, user_id;

-- name: GetUserEvents :many
SELECT * 
FROM events
WHERE user_id = $1;

-- name: GetUserEventById :one
SELECT *
FROM events
WHERE id = $1 AND user_id = $2;

-- name: UpdateEvent :one
UPDATE events
SET title = $1, description = $2, organizer = $3, updated_at = NOW()
WHERE id = $4 AND user_id= $5
RETURNING id, title, description, organizer, created_at, updated_at, user_id;

-- name: DeleteEvent :exec
DELETE FROM events WHERE id = $1 AND user_id = $2;

-- name: GetEvents :many
SELECT 
	e.id AS event_id,
	e.title,
	e.description,
	e.organizer,
	ed.id AS event_detail_id,
	ed.show_date,
	ed.price,
	ed.number_of_tickets,
	ed.ticket_description
FROM events AS e
LEFT JOIN event_details AS ed
ON ed.event_id = e.id
WHERE 
(LOWER(e.title) LIKE $1 
OR LOWER(e.description) LIKE $2 
OR LOWER(e.organizer) LIKE $3) 
AND (ed.show_date >= $4 AND ed.show_date <= $5);

-- name: GetPaidEventForRefund :many
SELECT
    p.id AS payment_id,
    p.payment_intent_id,
    p.amount,
    p.status,
	e.title,
	SUM(ed.price) AS ticket_price 
FROM events AS e
JOIN event_details AS ed
    ON ed.event_id = e.id
JOIN reservations AS r
    ON r.event_detail_id = ed.id
JOIN payments AS p
    ON p.id = r.payment_id
JOIN users AS u
    ON u.id = p.user_id
WHERE e.id = @event_id::uuid AND e.user_id = @user_id::uuid
GROUP BY p.id, p.payment_intent_id, p.amount, p.status, e.title, ed.price;

-- name: GetEventConfirmedUserReservations :many
SELECT 
	p.user_id,
	CONCAT(u.firstName, ' ', u.lastName) AS fullName, 
	u.email
FROM events as e
JOIN event_details AS ed
	ON ed.event_id = e.id
JOIN reservations AS r
	ON r.event_detail_id = ed.id
JOIN payments AS p
	ON p.id = r.payment_id
JOIN users AS u
	ON u.id = p.user_id 
WHERE e.id = $1
	AND p.status = 'succeeded'
GROUP BY p.user_id, u.firstName, u.lastName, u.email;