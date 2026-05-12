package telegram

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type testerRepository struct {
	pool *pgxpool.Pool
}

func NewTesterRepository(pool *pgxpool.Pool) testerDB {
	return &testerRepository{pool: pool}
}

func (r *testerRepository) Upsert(telegramID int64, firstName, username string) error {
	ctx := context.Background()
	var un *string
	if username != "" {
		un = &username
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO testers (telegram_id, first_name, telegram_username, last_login)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (telegram_id) DO UPDATE
		  SET first_name        = EXCLUDED.first_name,
		      telegram_username = EXCLUDED.telegram_username,
		      last_login        = EXCLUDED.last_login
	`, telegramID, firstName, un, time.Now().UTC())
	return err
}

func (r *testerRepository) GetGameUsername(telegramID int64) (string, bool, error) {
	ctx := context.Background()
	var gameUsername *string
	err := r.pool.QueryRow(ctx,
		`SELECT game_username FROM testers WHERE telegram_id = $1`,
		telegramID,
	).Scan(&gameUsername)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if gameUsername == nil || *gameUsername == "" {
		return "", false, nil
	}
	return *gameUsername, true, nil
}

func (r *testerRepository) SetGameUsername(telegramID int64, gameUsername string) error {
	ctx := context.Background()
	_, err := r.pool.Exec(ctx,
		`UPDATE testers SET game_username = $1 WHERE telegram_id = $2`,
		gameUsername, telegramID,
	)
	return err
}

