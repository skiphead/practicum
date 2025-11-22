-- +goose Up
CREATE TABLE IF NOT EXISTS shorts_url
(
    id             SERIAL PRIMARY KEY,
    created_at     TIMESTAMPTZ      DEFAULT NOW(),
    expires_at     TIMESTAMPTZ,
    correlation_id VARCHAR,
    short_code     VARCHAR NOT NULL,
    original_url   VARCHAR NOT NULL,
    user_id        VARCHAR NOT NULL DEFAULT 0,
    is_active      BOOLEAN          DEFAULT true,
    click_count    BIGINT           DEFAULT 0
);

-- +goose Down
DROP TABLE shorts_url;

