-- +goose Up
CREATE TABLE sellers (
    id             BIGSERIAL PRIMARY KEY,
    email          TEXT UNIQUE NOT NULL,
    password_hash  TEXT NOT NULL,
    name           TEXT NOT NULL,
    bio            TEXT DEFAULT '',
    order_email    TEXT DEFAULT '',
    is_active      BOOLEAN DEFAULT false,
    is_admin       BOOLEAN DEFAULT false,
    approval_token TEXT DEFAULT '',
    created_at     TIMESTAMPTZ DEFAULT now(),
    updated_at     TIMESTAMPTZ DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS sellers;
