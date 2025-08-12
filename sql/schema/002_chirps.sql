-- +goose Up
CREATE TABLE chirps (
    id UUID DEFAULT gen_random_uuid() primary key,
    created_at timestamp not null,
    updated_at timestamp not null,
    body text not null,
    user_id UUID,
    constraint fk_chirps_user_id foreign key (user_id) references users(id) on delete cascade
);

-- +goose Down
DROP TABLE chirps;