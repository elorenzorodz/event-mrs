-- name: CreateEventDetail :one
INSERT INTO event_details (id, show_date, price, number_of_tickets, ticket_description, event_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, show_date, price, number_of_tickets, ticket_description, created_at, updated_at, event_id;

-- name: GetEventDetailsByEventId :many
SELECT * 
FROM event_details 
WHERE event_id = ANY($1);