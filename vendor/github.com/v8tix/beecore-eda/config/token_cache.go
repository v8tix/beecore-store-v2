package config

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// TokenCache is a minimal cache abstraction shaped after JSR-107 (JCache)'s
// core Cache<K,V> operations — Get/Put-style access with TTL — scoped to
// what a shared, expiring token cache actually needs. It deliberately
// excludes the rest of the JSR-107 surface (CacheManager, CachingProvider,
// entry listeners, getAll/iterator) since nothing here needs it.
//
// Get returns ok=false for both a missing entry and an expired one — callers
// never need to distinguish the two. expiry is only meaningful when ok is
// true; it lets a caller detect a still-valid-but-soon-expiring entry and
// decide to refresh it ahead of time instead of waiting for it to lapse.
//
// Put unconditionally overwrites key — use it to replace an entry that's
// still present but near expiry (refresh-ahead); PutIfAbsent would never
// take effect there since the old entry hasn't lapsed yet.
//
// PutIfAbsent stores atomically: if two callers race to populate a key with
// no valid entry, exactly one call's value wins (reported via the returned
// stored bool) — whether those callers are goroutines in one process
// (InMemoryTokenCache) or separate microservice instances (RedisTokenCache),
// which is what lets every service see the same admin token instead of each
// generating and caching its own. Use it for the populate-from-nothing case;
// use Put to replace an entry that already exists.
type TokenCache interface {
	Get(ctx context.Context, key string) (value string, expiry time.Time, ok bool, err error)
	Put(ctx context.Context, key, value string, ttl time.Duration) error
	PutIfAbsent(ctx context.Context, key, value string, ttl time.Duration) (stored bool, err error)
	Remove(ctx context.Context, key string) error
}

type inMemoryEntry struct {
	value  string
	expiry time.Time
}

// InMemoryTokenCache is a process-local TokenCache. Entries are visible only
// within this process — use RedisTokenCache to share entries across
// microservice instances.
type InMemoryTokenCache struct {
	mu    sync.Mutex
	items map[string]inMemoryEntry
}

var _ TokenCache = (*InMemoryTokenCache)(nil)

// NewInMemoryTokenCache returns a TokenCache backed by an in-process map.
func NewInMemoryTokenCache() *InMemoryTokenCache {
	return &InMemoryTokenCache{items: make(map[string]inMemoryEntry)}
}

func (c *InMemoryTokenCache) Get(_ context.Context, key string) (string, time.Time, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.items[key]
	if !ok || !time.Now().Before(entry.expiry) {
		return "", time.Time{}, false, nil
	}

	return entry.value, entry.expiry, true, nil
}

func (c *InMemoryTokenCache) Put(_ context.Context, key, value string, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = inMemoryEntry{value: value, expiry: time.Now().Add(ttl)}

	return nil
}

func (c *InMemoryTokenCache) PutIfAbsent(_ context.Context, key, value string, ttl time.Duration) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, ok := c.items[key]; ok && time.Now().Before(entry.expiry) {
		return false, nil
	}

	c.items[key] = inMemoryEntry{value: value, expiry: time.Now().Add(ttl)}

	return true, nil
}

func (c *InMemoryTokenCache) Remove(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)

	return nil
}

// RedisTokenCache is a TokenCache backed by Redis, shared across every
// microservice instance pointed at the same Redis — the first instance to
// generate a value for a given key makes it visible to all the others.
type RedisTokenCache struct {
	client *redis.Client
	prefix string
}

var _ TokenCache = (*RedisTokenCache)(nil)

// NewRedisTokenCache returns a TokenCache backed by Redis, namespaced under
// "authcache:" so it doesn't collide with the unrelated "permissions:*" keys
// AuthService already stores in the same Redis instance.
func NewRedisTokenCache(client *redis.Client) *RedisTokenCache {
	return &RedisTokenCache{client: client, prefix: "authcache:"}
}

func (c *RedisTokenCache) Get(ctx context.Context, key string) (string, time.Time, bool, error) {
	fullKey := c.prefix + key

	val, err := c.client.Get(ctx, fullKey).Result()
	if errors.Is(err, redis.Nil) {
		return "", time.Time{}, false, nil
	}
	if err != nil {
		return "", time.Time{}, false, err
	}

	// A second round trip, not a pipeline: the tiny race window (the key
	// expiring between these two calls) only ever produces an already-past
	// expiry, which callers correctly treat as "refresh now" — same outcome
	// as if we'd read it a moment later.
	ttl, err := c.client.PTTL(ctx, fullKey).Result()
	if err != nil {
		return "", time.Time{}, false, err
	}
	if ttl <= 0 {
		// -2 (key gone) or -1 (no TTL set, which shouldn't happen — every
		// PutIfAbsent sets one) — either way, treat as a miss.
		return "", time.Time{}, false, nil
	}

	return val, time.Now().Add(ttl), true, nil
}

func (c *RedisTokenCache) Put(ctx context.Context, key, value string, ttl time.Duration) error {
	return c.client.Set(ctx, c.prefix+key, value, ttl).Err()
}

func (c *RedisTokenCache) PutIfAbsent(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
	stored, err := c.client.SetNX(ctx, c.prefix+key, value, ttl).Result()
	if err != nil {
		return false, err
	}

	return stored, nil
}

func (c *RedisTokenCache) Remove(ctx context.Context, key string) error {
	return c.client.Del(ctx, c.prefix+key).Err()
}
