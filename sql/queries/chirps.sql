-- name: CreateChirp :one
insert into chirps (created_at, updated_at, body, user_id)
values (now(),now(), $1, $2)
returning *;

-- name: GetChirps :many
select * from chirps order by created_at asc;

-- name: GetChirpById :one
select * from chirps where id = $1;

-- name: DeleteChirp :exec
delete from chirps where id = $1 and user_id = $2;

-- name: GetChirpsByUserId :many
select * from chirps where user_id = $1 order by created_at asc;