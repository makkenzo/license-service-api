CREATE TABLE IF NOT EXISTS api_keys (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key_hash      VARCHAR(255) NOT NULL UNIQUE,
    prefix        VARCHAR(16) NOT NULL UNIQUE,
    description   TEXT NOT NULL DEFAULT '',
    product_id    UUID,
    is_enabled    BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at  TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_api_keys_prefix ON api_keys (prefix);
CREATE INDEX IF NOT EXISTS idx_api_keys_is_enabled ON api_keys (is_enabled);