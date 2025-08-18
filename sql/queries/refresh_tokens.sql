-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (token, created_at, updated_at, user_id, expires_at)
VALUES (
           $1,now(), now(), $2, $3
       )
RETURNING *;

-- name: GetRefreshTokenByToken :one
SELECT * from refresh_tokens
where token = $1;

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens set revoked_at = now(),
    updated_at = now()
WHERE token = $1;