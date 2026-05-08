-- Migration: 004_create_operators
-- One row per casino/aggregator that integrates with this game provider.
--
-- Security notes:
--   secret_key  — stored as-is; encrypt at-rest with AES-GCM in production.
--                 The application never returns this column in API responses.
--   api_key     — public identifier sent in X-API-KEY header.
--   Both are rotatable; a rotation endpoint should invalidate the Redis cache.

CREATE TABLE IF NOT EXISTS operators (
    id                     BIGSERIAL       PRIMARY KEY,
    name                   TEXT            NOT NULL,
    api_key                TEXT            NOT NULL UNIQUE,
    secret_key             TEXT            NOT NULL,
    status                 VARCHAR(20)     NOT NULL DEFAULT 'active',
    allowed_origins        TEXT[]          NOT NULL DEFAULT '{}',
    callback_url           TEXT            NOT NULL DEFAULT '',
    default_rtp_profile_id BIGINT          REFERENCES rtp_profiles(id) ON DELETE SET NULL,
    created_at             TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMPTZ     NOT NULL DEFAULT NOW(),

    CONSTRAINT operators_status_check
        CHECK (status IN ('active', 'inactive', 'suspended'))
);

CREATE INDEX IF NOT EXISTS idx_operators_api_key ON operators (api_key);
CREATE INDEX IF NOT EXISTS idx_operators_status  ON operators (status);

-- Trigger: keep updated_at current automatically
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER operators_set_updated_at
BEFORE UPDATE ON operators
FOR EACH ROW EXECUTE FUNCTION set_updated_at();
