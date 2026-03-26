-- +goose UP
ALTER TABLE users ADD COLUMN is_chirpy_red BOOLEAN DEFAULT false;

-- +goose DOWN
ALTER TABLE users DROP COLUMN is_chirpy_red;
