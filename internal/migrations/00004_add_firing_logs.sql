-- +goose Up
CREATE TABLE firing_logs (
    id          BIGSERIAL PRIMARY KEY,
    seller_id   BIGINT NOT NULL REFERENCES sellers(id),
    title       TEXT NOT NULL,
    firing_date DATE,
    clay_body   TEXT DEFAULT '',
    glaze_notes TEXT DEFAULT '',
    outcome     TEXT DEFAULT '',
    notes       TEXT DEFAULT '',
    created_at  TIMESTAMPTZ DEFAULT now(),
    updated_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE firing_readings (
    id              BIGSERIAL PRIMARY KEY,
    firing_log_id   BIGINT NOT NULL REFERENCES firing_logs(id) ON DELETE CASCADE,
    elapsed_minutes INTEGER NOT NULL,
    temperature     NUMERIC(6,1) NOT NULL,
    gas_setting     TEXT DEFAULT '',
    flue_setting    TEXT DEFAULT '',
    notes           TEXT DEFAULT '',
    created_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_firing_readings_log ON firing_readings(firing_log_id, elapsed_minutes);

-- +goose Down
DROP INDEX IF EXISTS idx_firing_readings_log;
DROP TABLE IF EXISTS firing_readings;
DROP TABLE IF EXISTS firing_logs;
