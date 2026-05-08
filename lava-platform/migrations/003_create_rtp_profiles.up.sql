-- Migration: 003_create_rtp_profiles
-- RTP (Return to Player) configurations assigned per operator.

CREATE TABLE IF NOT EXISTS rtp_profiles (
    id         BIGSERIAL      PRIMARY KEY,
    name       TEXT           NOT NULL UNIQUE,
    target_rtp NUMERIC(5, 2)  NOT NULL DEFAULT 96.00,
    created_at TIMESTAMPTZ    NOT NULL DEFAULT NOW(),

    CONSTRAINT rtp_target_range CHECK (target_rtp BETWEEN 50.00 AND 99.99)
);

-- Seed default profiles
INSERT INTO rtp_profiles (name, target_rtp) VALUES
    ('Standard 96%', 96.00),
    ('Premium 97%',  97.00),
    ('VIP 98%',      98.00),
    ('Demo 99.9%',   99.90)
ON CONFLICT (name) DO NOTHING;
