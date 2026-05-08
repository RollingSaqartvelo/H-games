-- 006_create_rounds.sql
-- Stores the lifecycle of every crash game round.
-- server_seed is NULL until the round crashes (provably fair reveal).
-- crash_point is pre-computed and stored sealed; revealed via server_seed after crash.

CREATE TABLE IF NOT EXISTS rounds (
    id               TEXT        NOT NULL PRIMARY KEY,
    state            TEXT        NOT NULL DEFAULT 'CREATED'
                                 CHECK (state IN ('CREATED','STARTING','RUNNING','CRASHED','FINISHED')),
    server_seed      TEXT,                               -- NULL until round ends
    server_seed_hash TEXT        NOT NULL,               -- published before round starts
    client_seed      TEXT        NOT NULL,
    nonce            BIGINT      NOT NULL,
    rtp_profile      INT         NOT NULL DEFAULT 96,
    house_edge       NUMERIC(6,4) NOT NULL DEFAULT 0.04,
    crash_point      NUMERIC(10,2) NOT NULL,
    started_at       TIMESTAMPTZ,
    crashed_at       TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_rounds_state       ON rounds (state);
CREATE INDEX IF NOT EXISTS idx_rounds_created_at  ON rounds (created_at DESC);

-- Auto-update updated_at
CREATE OR REPLACE FUNCTION rounds_set_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_rounds_updated_at ON rounds;
CREATE TRIGGER trg_rounds_updated_at
    BEFORE UPDATE ON rounds
    FOR EACH ROW EXECUTE FUNCTION rounds_set_updated_at();
