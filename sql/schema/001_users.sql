-- +goose Up
CREATE TABLE users (
  id UUID DEFAULT gen_random_uuid() primary key,
  created_at timestamp not null,
  updated_at timestamp not null,
  email text not null unique
);

-- +goose Down
DROP TABLE users;
