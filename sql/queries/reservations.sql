-- name: CreateReservation :one
INSERT INTO reservations (id, email, event_detail_id)
VALUES ($1, $2, $3)
RETURNING id, email, created_at, updated_at, event_detail_id;

-- name: DeleteReservation :exec
DELETE FROM reservations WHERE id = $1;