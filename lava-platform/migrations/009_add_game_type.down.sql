-- 009_add_game_type.down.sql
ALTER TABLE rounds     DROP COLUMN IF EXISTS game_type;
ALTER TABLE round_bets DROP COLUMN IF EXISTS game_type;
