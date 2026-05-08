-- Migration: 005_create_operator_sessions
-- Tracks player game sessions per operator.
--
-- Flow:
--   1. Operator calls POST /api/v1/provider/session/create
--   2. We INSERT here + write to Redis (session:{token} → JSON, TTL = expires_at)
--   3. Player's game calls wallet endpoints with session token
--   4. We validate from Redis (fast path) or this table (cold path)
--   5. Session is revoked on game end or timeout

CREATE TABLE IF NOT EXISTS operator_sessions (
    id            BIGSERIAL     PRIMARY KEY,
    operator_id   BIGINT        NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
    player_id     TEXT          NOT NULL,
    session_token TEXT          NOT NULL UNIQUE,
    currency      VARCHAR(3)    NOT NULL,
    ip            TEXT          NOT NULL,
    active        BOOLEAN       NOT NULL DEFAULT TRUE,
    expires_at    TIMESTAMPTZ   NOT NULL,
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

-- Fast lookup by token (most common query pattern)
CREATE INDEX IF NOT EXISTS idx_sessions_token
    ON operator_sessions (session_token)
    WHERE active = TRUE;

-- Player history per operator
CREATE INDEX IF NOT EXISTS idx_sessions_operator_player
    ON operator_sessions (operator_id, player_id);

-- Cleanup job: find expired sessions
CREATE INDEX IF NOT EXISTS idx_sessions_expires
    ON operator_sessions (expires_at)
    WHERE active = TRUE;
