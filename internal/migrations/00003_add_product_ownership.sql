-- +goose Up
ALTER TABLE products ADD COLUMN seller_id BIGINT REFERENCES sellers(id);

UPDATE products
SET seller_id = (SELECT id FROM sellers WHERE is_admin = true ORDER BY id LIMIT 1)
WHERE seller_id IS NULL;

-- +goose Down
ALTER TABLE products DROP COLUMN seller_id;
