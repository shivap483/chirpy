-- +goose Up
ALTER TABLE users
ADD COLUMN IF NOT EXISTS hashed_password VARCHAR(255) NOT NULL DEFAULT 'unset';

-- +goose Down
ALTER TABLE users
DROP COLUMN IF EXISTS hashed_password;