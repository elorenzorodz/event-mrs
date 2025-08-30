-- name: CreateEvent :one
INSERT INTO events (id, title, description, organizer, user_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, title, description, organizer, created_at, updated_at, user_id;