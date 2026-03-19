-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (token, created_at, updated_at, expires_at, user_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetRefreshTokenByToken :one
SELECT * FROM refresh_tokens WHERE token = $1 and revoked_at IS NULL;

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens SET revoked_at = NOW() WHERE token = $1;
