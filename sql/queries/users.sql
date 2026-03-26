-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, email, password)
VALUES (
    gen_random_uuid(),
    NOW(),
    NOW(),
    $1,
    $2
)
RETURNING *;

-- name: DeleteUsers :exec
DELETE FROM users;

-- name: GetUser :one
SELECT * FROM users WHERE email = $1;

-- name: UpdateUsers :exec
UPDATE users SET email = $1, password = $2 WHERE id = $3;

-- name: SubscribeUser :exec
UPDATE users SET is_chirpy_red = true WHERE id = $1;

-- name: CheckUser :one
SELECT id FROM users WHERE id = $1;
