-- 009_add_game_type.up.sql
-- Adds game_type column to rounds and round_bets for multi-game support.
-- All existing rows default to 'outlaw_escape' (the original game).

ALTER TABLE rounds
    ADD COLUMN IF NOT EXISTS game_type TEXT NOT NULL DEFAULT 'outlaw_escape';

ALTER TABLE round_bets
    ADD COLUMN IF NOT EXISTS game_type TEXT NOT NULL DEFAULT 'outlaw_escape';

CREATE INDEX IF NOT EXISTS idx_rounds_game_type        ON rounds (game_type);
CREATE INDEX IF NOT EXISTS idx_rounds_game_state       ON rounds (game_type, state);
CREATE INDEX IF NOT EXISTS idx_rounds_game_created     ON rounds (game_type, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_round_bets_game_type    ON round_bets (game_type);
