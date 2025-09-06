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