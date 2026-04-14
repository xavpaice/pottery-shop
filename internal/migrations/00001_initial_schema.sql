-- +goose Up
CREATE TABLE IF NOT EXISTS products (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    price       NUMERIC(10,2) NOT NULL DEFAULT 0,
    is_sold     BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS images (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    product_id   BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    filename     TEXT NOT NULL,
    thumbnail_fn TEXT NOT NULL DEFAULT '',
    sort_order   INTEGER NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS images;
DROP TABLE IF EXISTS products;
