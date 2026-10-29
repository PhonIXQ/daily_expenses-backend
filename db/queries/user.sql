-- name: CreateUser :one
INSERT INTO users (id, email, display_name) VALUES ($1, $2, $3) RETURNING *;

-- name: GetUser :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: UpdateUser :one
UPDATE users
SET display_name = COALESCE(sqlc.narg('display_name'), display_name),
    locale       = COALESCE(sqlc.narg('locale'), locale),
    updated_at   = now()
WHERE id = $1
RETURNING *;
