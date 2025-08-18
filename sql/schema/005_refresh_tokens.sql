-- +goose Up
CREATE TABLE refresh_tokens (
    token text primary key,
    created_at timestamp not null,
    updated_at timestamp not null,
    user_id UUID not null,
    Foreign Key (user_id) references users(id) on delete cascade,
    expires_at timestamp not null,
    revoked_at timestamp
);

-- +goose Down
DROP TABLE refresh_tokens;