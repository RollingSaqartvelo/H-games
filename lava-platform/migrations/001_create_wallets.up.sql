-- Migration: 001_create_wallets
-- Stores one balance record per player.

CREATE TABLE IF NOT EXISTS wallets (
    user_id    TEXT           NOT NULL PRIMARY KEY,
    balance    NUMERIC(20, 8) NOT NULL DEFAULT 0,
    currency   VARCHAR(3)     NOT NULL,
    updated_at TIMESTAMPTZ    NOT NULL DEFAULT NOW(),

    CONSTRAINT wallets_balance_non_negative CHECK (balance >= 0)
);

-- Fast lookup when acquiring the row lock
CREATE INDEX IF NOT EXISTS idx_wallets_user_id ON wallets (user_id);
