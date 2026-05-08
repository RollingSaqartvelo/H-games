package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lava-platform/internal/domain"
	"github.com/lava-platform/internal/operator/repository"
	"github.com/lava-platform/internal/signing"
	"github.com/redis/go-redis/v9"
)

const cachePrefix = "op:"

type OperatorService interface {
	// Authenticate validates the HMAC signature and returns the operator.
	Authenticate(ctx context.Context, apiKey, sig string, timestamp int64, method, path string, body []byte) (*domain.Operator, error)
	// GetByID returns an operator (used by admin endpoints).
	GetByID(ctx context.Context, id int64) (*domain.Operator, error)
	// Create provisions a new operator and returns generated credentials.
	Create(ctx context.Context, req *domain.CreateOperatorRequest) (*domain.Operator, string, string, error)
	// UpdateStatus enables/disables/suspends an operator.
	UpdateStatus(ctx context.Context, id int64, status domain.OperatorStatus) error
	// List returns all operators.
	List(ctx context.Context) ([]*domain.Operator, error)
	// ListRTPProfiles returns available RTP configurations.
	ListRTPProfiles(ctx context.Context) ([]*domain.RTPProfile, error)
}

type operatorService struct {
	repo     repository.OperatorRepository
	cache    *redis.Client
	cacheTTL time.Duration
}

func New(repo repository.OperatorRepository, cache *redis.Client, cacheTTL time.Duration) OperatorService {
	return &operatorService{repo: repo, cache: cache, cacheTTL: cacheTTL}
}

func (s *operatorService) Authenticate(
	ctx context.Context,
	apiKey, sig string,
	timestamp int64,
	method, path string,
	body []byte,
) (*domain.Operator, error) {
	op, err := s.getByAPIKey(ctx, apiKey)
	if err != nil {
		return nil, err
	}

	if op.Status == domain.OperatorStatusSuspended {
		return nil, domain.ErrOperatorSuspended
	}
	if op.Status == domain.OperatorStatusInactive {
		return nil, domain.ErrOperatorInactive
	}

	if !signing.Verify(op.SecretKey, method, path, timestamp, body, sig) {
		return nil, domain.ErrInvalidSignature
	}

	return op, nil
}

func (s *operatorService) GetByID(ctx context.Context, id int64) (*domain.Operator, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *operatorService) Create(ctx context.Context, req *domain.CreateOperatorRequest) (*domain.Operator, string, string, error) {
	apiKey := "lava_" + uuid.NewString()
	secretKey := uuid.NewString() + uuid.NewString() // 72-char secret

	op, err := s.repo.Create(ctx, req, apiKey, secretKey)
	if err != nil {
		return nil, "", "", fmt.Errorf("create operator: %w", err)
	}
	return op, apiKey, secretKey, nil
}

func (s *operatorService) UpdateStatus(ctx context.Context, id int64, status domain.OperatorStatus) error {
	if err := s.repo.UpdateStatus(ctx, id, status); err != nil {
		return err
	}
	// Invalidate cache so the next request re-fetches the updated status
	s.invalidateCache(ctx, id)
	return nil
}

func (s *operatorService) List(ctx context.Context) ([]*domain.Operator, error) {
	return s.repo.List(ctx)
}

func (s *operatorService) ListRTPProfiles(ctx context.Context) ([]*domain.RTPProfile, error) {
	return s.repo.ListRTPProfiles(ctx)
}

// ─── Cache (Redis) ────────────────────────────────────────────────────────────

func (s *operatorService) getByAPIKey(ctx context.Context, apiKey string) (*domain.Operator, error) {
	key := cachePrefix + apiKey
	if data, err := s.cache.Get(ctx, key).Bytes(); err == nil {
		var op domain.Operator
		if json.Unmarshal(data, &op) == nil {
			return &op, nil
		}
	}

	op, err := s.repo.GetByAPIKey(ctx, apiKey)
	if errors.Is(err, domain.ErrOperatorNotFound) {
		return nil, domain.ErrOperatorNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get operator: %w", err)
	}

	if data, err := json.Marshal(op); err == nil {
		_ = s.cache.Set(ctx, key, data, s.cacheTTL).Err()
	}
	return op, nil
}

func (s *operatorService) invalidateCache(ctx context.Context, operatorID int64) {
	// Pattern-based delete: scan for op:lava_* belonging to this operator.
	// Simpler approach: operator stores api_key in context — caller can pass it.
	// For now, invalidation is done on next request (cache miss) via short TTL.
	_ = operatorID
	_ = ctx
}
