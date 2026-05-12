-- Migration: 010_create_testers
-- Stores test-access users who authenticated via Telegram Login Widget.

CREATE TABLE IF NOT EXISTS testers (
    telegram_id       BIGINT         NOT NULL PRIMARY KEY,
    first_name        TEXT           NOT NULL DEFAULT '',
    telegram_username TEXT,
    game_username     TEXT,
    is_active         BOOLEAN        NOT NULL DEFAULT TRUE,
    created_at        TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    last_login        TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_testers_telegram_id ON testers (telegram_id);
