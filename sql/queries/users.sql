-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, email, hashed_password)
values(
    $1,
    $2,
    $3,
    $4,
    $5
)
RETURNING *;

-- name: GetUserById :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: DeleteAllUsers :exec
DELETE FROM users;

-- name: UpdateUser :one
UPDATE users
SET updated_at = $2, email = $3, hashed_password = $4
WHERE id = $1
RETURNING *;

-- name: UpgradeUserToChirpyRed :one
UPDATE users
SET is_chirpy_red = true, updated_at = $2
WHERE id = $1
RETURNING *;