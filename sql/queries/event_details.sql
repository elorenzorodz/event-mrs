-- name: CreateEventDetail :one
INSERT INTO event_details (id, show_date, price, number_of_tickets, tickets_remaining, ticket_description, event_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, show_date, price, number_of_tickets, tickets_remaining, ticket_description, created_at, updated_at, event_id;

-- name: GetEventDetailsByEventId :many
SELECT * FROM event_details WHERE event_id = ANY($1);

-- name: UpdateEventDetail :one
UPDATE event_details
SET show_date = $1, price = $2, number_of_tickets = $3, ticket_description = $4, updated_at = NOW()
WHERE id = $5 AND event_id = $6
RETURNING id, show_date, price, number_of_tickets, tickets_remaining, ticket_description, created_at, updated_at, event_id;

-- name: DeleteEventDetail :exec
DELETE FROM event_details WHERE id = $1 AND event_id = $2;

-- name: UpdateTicketsRemaining :one
UPDATE event_details
SET tickets_remaining = $1, updated_at = NOW()
WHERE id = $2
RETURNING id, show_date, price, number_of_tickets, tickets_remaining, ticket_description, created_at, updated_at, event_id;

-- name: GetEventDetailsById :one
SELECT * FROM event_details WHERE id = $1;

-- name: GethEventDetailsWithTitleByIds :many
SELECT 
	e.title,
	ed.ticket_description,
	ed.show_date
FROM event_details AS ed
JOIN events AS e
	ON e.id = ed.event_id
WHERE ed.id = ANY($1);