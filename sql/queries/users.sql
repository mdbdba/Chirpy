-- name: CreateUser :one
INSERT INTO users (created_at, updated_at, email, hashed_password)
VALUES (
    now(), now(), $1, $2
)
RETURNING *;

-- name: GetUserById :one
Select * from users where id = $1;

-- name: GetUserByEmail :one
Select * from users where email = $1;

-- name: DeleteUsers :exec
DELETE FROM users;

-- name: UpdateUserById :one
Update users set email = $2, hashed_password = $3,
                 updated_at = now()
where id = $1
returning *;