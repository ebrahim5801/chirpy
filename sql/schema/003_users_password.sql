-- +goose UP
ALTER TABLE users ADD COLUMN password TEXT NOT NULL DEFAULT 'unset';

-- +goose DOWN
ALTER TABLE users DROP COLUMN password;
