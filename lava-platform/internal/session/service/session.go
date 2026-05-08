package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lava-platform/internal/domain"
	"github.com/lava-platform/internal/session/repository"
)

type SessionService interface {
	Create(ctx context.Context, operatorID int64, playerID, currency, ip string) (*domain.OperatorSession, error)
	Validate(ctx context.Context, token string) (*domain.OperatorSession, error)
	Revoke(ctx context.Context, token string) error
}

type sessionService struct {
	repo       repository.SessionRepository
	sessionTTL time.Duration
}

func New(repo repository.SessionRepository, ttl time.Duration) SessionService {
	return &sessionService{repo: repo, sessionTTL: ttl}
}

func (s *sessionService) Create(ctx context.Context, operatorID int64, playerID, currency, ip string) (*domain.OperatorSession, error) {
	if playerID == "" || currency == "" {
		return nil, fmt.Errorf("player_id and currency are required")
	}

	token := uuid.NewString()

	sess, err := s.repo.Create(ctx, &domain.CreateSessionRequest{
		OperatorID: operatorID,
		PlayerID:   playerID,
		Currency:   currency,
		IP:         ip,
		TTL:        s.sessionTTL,
	}, token)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return sess, nil
}

func (s *sessionService) Validate(ctx context.Context, token string) (*domain.OperatorSession, error) {
	sess, err := s.repo.GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}

	if !sess.Active {
		return nil, domain.ErrSessionRevoked
	}
	if time.Now().After(sess.ExpiresAt) {
		return nil, domain.ErrSessionExpired
	}

	return sess, nil
}

func (s *sessionService) Revoke(ctx context.Context, token string) error {
	return s.repo.Revoke(ctx, token)
}
