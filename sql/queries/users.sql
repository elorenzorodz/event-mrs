-- name: CreateUser :one
INSERT INTO users (id, firstname, lastname, email, password)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, firstname, lastname, email, password, created_at, updated_at;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;