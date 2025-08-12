-- +goose Up
alter table chirps alter column user_id set not null;

-- +goose Down
alter table chirps alter column user_id drop not null;