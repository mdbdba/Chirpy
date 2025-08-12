-- name: CreateChirp :one
insert into chirps (created_at, updated_at, body, user_id)
values (now(),now(), $1, $2)
returning *;
