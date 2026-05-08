-- Migration: 002_create_transactions
-- Immutable ledger — every monetary event creates a new row.
-- transaction_id is globally unique (enforced at DB + application layers).

CREATE TABLE IF NOT EXISTS transactions (
    id             BIGSERIAL      PRIMARY KEY,
    user_id        TEXT           NOT NULL,
    type           VARCHAR(20)    NOT NULL,
    amount         NUMERIC(20, 8) NOT NULL,
    currency       VARCHAR(3)     NOT NULL,
    transaction_id TEXT           NOT NULL,
    round_id       TEXT           NOT NULL,
    status         VARCHAR(20)    NOT NULL DEFAULT 'completed',
    created_at     TIMESTAMPTZ    NOT NULL DEFAULT NOW(),

    CONSTRAINT transactions_amount_positive
        CHECK (amount > 0),
    CONSTRAINT transactions_type_check
        CHECK (type IN ('bet', 'win', 'rollback')),
    CONSTRAINT transactions_status_check
        CHECK (status IN ('completed', 'rolled_back'))
);

-- Idempotency: duplicate transaction_id rejected at the DB level
CREATE UNIQUE INDEX IF NOT EXISTS idx_transactions_transaction_id
    ON transactions (transaction_id);

-- Common query patterns
CREATE INDEX IF NOT EXISTS idx_transactions_user_id
    ON transactions (user_id);

CREATE INDEX IF NOT EXISTS idx_transactions_round_id
    ON transactions (round_id);

CREATE INDEX IF NOT EXISTS idx_transactions_user_created
    ON transactions (user_id, created_at DESC);
