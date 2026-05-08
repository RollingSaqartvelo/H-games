package lock

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const defaultTTL = 5 * time.Second

var ErrNotAcquired = errors.New("lock not acquired")

// releaseScript atomically checks ownership before deleting (safe unlock).
var releaseScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
else
    return 0
end
`)

type Locker struct {
	client *redis.Client
}

func New(client *redis.Client) *Locker {
	return &Locker{client: client}
}

type Lock struct {
	locker *Locker
	key    string
	token  string
}

// Acquire sets a distributed lock keyed by resource (e.g. user_id).
// Returns ErrNotAcquired if the lock is already held.
func (l *Locker) Acquire(ctx context.Context, resource string) (*Lock, error) {
	key := fmt.Sprintf("wallet:lock:%s", resource)
	token := uuid.NewString()

	ok, err := l.client.SetNX(ctx, key, token, defaultTTL).Result()
	if err != nil {
		return nil, fmt.Errorf("acquire lock: %w", err)
	}
	if !ok {
		return nil, ErrNotAcquired
	}
	return &Lock{locker: l, key: key, token: token}, nil
}

// Release deletes the lock only if we still own it.
func (lk *Lock) Release(ctx context.Context) {
	_ = releaseScript.Run(ctx, lk.locker.client, []string{lk.key}, lk.token).Err()
}
