-- 007_create_round_bets.sql
-- Records every bet placed in a crash round and its settlement outcome.

CREATE TABLE IF NOT EXISTS round_bets (
    id              TEXT         NOT NULL PRIMARY KEY,
    round_id        TEXT         NOT NULL REFERENCES rounds (id),
    operator_id     BIGINT       NOT NULL,
    wallet_user_id  TEXT         NOT NULL,               -- "op{id}:{player_id}"
    player_id       TEXT         NOT NULL,
    bet_amount      NUMERIC(18,8) NOT NULL,
    currency        TEXT         NOT NULL,
    auto_cashout    NUMERIC(10,2) NOT NULL DEFAULT 0,    -- 0 = manual only
    cashout_at      NUMERIC(10,2),                       -- multiplier at cashout
    payout_amount   NUMERIC(18,8),
    transaction_id  TEXT         NOT NULL UNIQUE,        -- idempotency key (debit tx)
    payout_tx_id    TEXT,                                -- credit tx id (set on cashout)
    status          TEXT         NOT NULL DEFAULT 'PENDING'
                                 CHECK (status IN ('PENDING','ACTIVE','WON','LOST','CANCELLED')),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_round_bets_round_id       ON round_bets (round_id);
CREATE INDEX IF NOT EXISTS idx_round_bets_wallet_user_id ON round_bets (wallet_user_id);
CREATE INDEX IF NOT EXISTS idx_round_bets_status         ON round_bets (round_id, status);
