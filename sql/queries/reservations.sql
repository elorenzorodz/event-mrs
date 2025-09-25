-- name: ReserveTicket :one 
WITH params AS (
    SELECT 
        $1::uuid AS event_detail_id, 
        $2::uuid AS reservation_id, 
        $3::text AS email,
        $4::uuid AS user_id), 
updated_event_detail AS ( 
    UPDATE event_details ed 
    SET tickets_remaining = ed.tickets_remaining - 1 
    FROM params p 
    WHERE ed.id = p.event_detail_id AND ed.tickets_remaining > 0 
    RETURNING ed.id AS event_detail_id ) 

INSERT INTO reservations (id, email, event_detail_id, user_id) 
SELECT 
    p.reservation_id AS id, 
    p.email AS email, 
    u.event_detail_id,
    p.user_id AS user_id
FROM params p 
CROSS JOIN updated_event_detail u 
RETURNING id AS id, email AS email, created_at AS created_at, updated_at AS updated_at, event_detail_id AS event_detail_id, user_id AS user_id;

-- name: GetUserReservations :many
SELECT * FROM reservations WHERE user_id = $1;

-- name: GetUserReservationById :one
SELECT * FROM reservations WHERE id = $1 AND user_id = $2;