-- name: CreateUser :one
INSERT INTO users (created_at, updated_at, email)
VALUES (
    now(), now(), $1
)
RETURNING *;

-- name: GetUserId :one
Select id from users where email = '$1';

-- name: DeleteUsers :exec
DELETE FROM users;
