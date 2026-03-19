-- name: CreateChirp :one
INSERT INTO chirps (id, created_at, updated_at, body, user_id)
values(
    $1,
    $2,
    $3,
    $4,
    $5
)
RETURNING *;

-- name: GetAllChirps :many
SELECT * FROM chirps;

-- name: GetChirpById :one
SELECT * FROM chirps WHERE id = $1;

-- name: DeleteAllChirps :exec
DELETE FROM chirps;

-- name: DeleteChirpById :exec
DELETE FROM chirps WHERE id = $1;

-- name: GetChirpsByAuthorIdSort :many
SELECT * FROM chirps WHERE user_id = $1 ORDER BY created_at ASC;
