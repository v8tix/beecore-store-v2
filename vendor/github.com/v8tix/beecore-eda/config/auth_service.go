package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/v8tix/beecore-auth-v3-mod/model"
	"github.com/v8tix/beecore-auth-v3-mod/repository"
	"golang.org/x/sync/singleflight"
)

const (
	cacheExpiration = 24 * time.Hour

	// tokenRefreshMargin shrinks a cached service-account token's stored TTL
	// relative to its real JWT expiry, so callers never race a token that
	// expires mid-request.
	tokenRefreshMargin = 30 * time.Second

	// tokenRefreshAheadWindow triggers a non-blocking background regeneration
	// when Get finds a cached token still valid but within this long of its
	// (already-margined) expiry — refresh-ahead caching, the same idea as
	// Guava's CacheBuilder.refreshAfterWrite or HTTP's stale-while-revalidate
	// (RFC 5861). The caller that notices always gets the still-valid cached
	// token back immediately; the point is only to make it far less likely
	// that some *later* caller hits a synchronous miss right at expiry.
	tokenRefreshAheadWindow = 5 * time.Minute
)

// serviceTokenCache pairs a TokenCache (see token_cache.go) with a
// singleflight.Group behind a pointer, so that copying an AuthService value
// (it's returned by value from newAuthService and assigned into Cfg) never
// copies live lock state — the same class of bug as embedding sync.Once
// directly in a struct that gets copied.
//
// group coalesces concurrent cache misses for the same key WITHIN THIS
// PROCESS: when a token expires and several requests notice at once, only
// one actually pays the bcrypt-compare + KMS-sign round trip — the rest
// block on the same in-flight call and share its result. store's own
// PutIfAbsent provides the equivalent guarantee ACROSS PROCESSES when store
// is a RedisTokenCache — the two are complementary, not redundant.
type serviceTokenCache struct {
	group singleflight.Group
	store TokenCache
}

// generateTokenFunc is the shape of the call GetOrGenerateAuthenticationToken
// makes to actually mint a token. It's a package-level swappable var (not a
// struct field or interface) so tests can substitute a fake without touching
// AuthService's or Repository's signatures — GenerateAuthenticationToken
// itself does a real Postgres bcrypt compare + Google KMS sign, neither of
// which is available in a unit test.
type generateTokenFunc func(s *AuthService, email, password, app string, duration time.Duration) (string, error)

var generateAuthToken generateTokenFunc = func(s *AuthService, email, password, app string, duration time.Duration) (string, error) {
	return s.GenerateAuthenticationToken(email, password, app, duration)
}

// SetGenerateAuthTokenFunc overrides generateAuthToken for tests and returns
// the previous value so callers can restore it:
//
//	old := SetGenerateAuthTokenFunc(fake)
//	defer SetGenerateAuthTokenFunc(old)
func SetGenerateAuthTokenFunc(fn generateTokenFunc) generateTokenFunc {
	old := generateAuthToken
	generateAuthToken = fn

	return old
}

var (
	// ErrAuthTokenGenerationFailed indicates that authentication token generation failed.
	ErrAuthTokenGenerationFailed = errors.New("authentication token generation failed")
	// ErrPermissionsFindFailed indicates that finding permissions by user ID failed.
	ErrPermissionsFindFailed = errors.New("permissions find failed")
	// ErrTokenFindFailed indicates that finding token by user ID and scope failed.
	ErrTokenFindFailed = errors.New("token find failed")
	// ErrDatabaseConnectionAcquireFailed indicates that acquiring database connection failed.
	ErrDatabaseConnectionAcquireFailed = errors.New("database connection acquire failed")
	// ErrDatabaseListenCommandFailed indicates that executing database LISTEN command failed.
	ErrDatabaseListenCommandFailed = errors.New("database listen command failed")
	// ErrPostgresConnectionNotAvailable indicates that PostgreSQL connection is not available for LISTEN/NOTIFY.
	ErrPostgresConnectionNotAvailable = errors.New("postgres connection not available for LISTEN/NOTIFY")
)

// AuthService provides authentication and permission services.
// It embeds repository.Repository for token/permission DB access
// and holds only the minimal dependencies it requires.
type AuthService struct {
	repository.Repository
	db      *pgxpool.Pool
	redis   *redis.Client
	jwt     *JWT
	app     *App
	session *Session
	logger  *slog.Logger
	cancel  context.CancelFunc

	tokens *serviceTokenCache
}

func newAuthService(ctx context.Context, cfg *Cfg) AuthService {
	ctx, cancel := context.WithCancel(ctx)

	// Redis-backed when available, so the token cache is shared across every
	// microservice instance pointed at the same Redis instead of each one
	// caching (and separately paying bcrypt+KMS for) its own copy. Falls back
	// to in-process only if Redis isn't configured for this service.
	var tokenStore TokenCache
	if cfg.Redis.Client != nil {
		tokenStore = NewRedisTokenCache(cfg.Redis.Client)
	} else {
		tokenStore = NewInMemoryTokenCache()
	}

	svc := AuthService{
		Repository: repository.NewRepository(cfg.Postgres.DB, cfg.KMS.GetKMSKeyName(), cfg.KMS.Client),
		db:         cfg.Postgres.DB,
		redis:      cfg.Redis.Client,
		jwt:        &cfg.JWT,
		app:        &cfg.App,
		session:    &cfg.Session,
		logger:     cfg.Logger,
		cancel:     cancel,
		tokens:     &serviceTokenCache{store: tokenStore},
	}

	// Start LISTEN/NOTIFY listener for role changes (PostgreSQL-only feature).
	go svc.listenForRoleChanges(ctx)

	return svc
}

// GetDuration delegates JWT expiration duration to the configured JWT settings.
// This allows AuthService to satisfy application.JWTDuration.
func (s *AuthService) GetDuration() (time.Duration, error) {
	return s.jwt.GetDuration()
}

// GetRefreshTokenTTL returns the configured session lifetime, used as the
// refresh token's own expiry ceiling — distinct from jwt.refresh_interval,
// which governs how often the access token is refreshed within that ceiling.
// Falls back to defaultSessionMaxAge (config.go) when session.max_age is
// unset — the same default initSessionStore uses for the cookie's own
// MaxAge, so the two can't silently drift apart.
func (s *AuthService) GetRefreshTokenTTL() time.Duration {
	if s.session == nil || s.session.MaxAge <= 0 {
		return defaultSessionMaxAge
	}

	return time.Duration(s.session.MaxAge) * time.Second
}

// GenerateRefreshToken issues and persists an opaque refresh token for
// userID under the configured session TTL. It shadows the embedded
// repository.Repository method of the same name, hiding the ttl parameter
// behind config so callers don't need to know about session.max_age.
func (s *AuthService) GenerateRefreshToken(userID string) (string, error) {
	return s.Repository.GenerateRefreshToken(userID, s.GetRefreshTokenTTL())
}

// GetOrGenerateAuthenticationToken returns a cached JWT for (email, app) while
// it remains valid, avoiding the bcrypt compare + KMS signing round trip that
// GenerateAuthenticationToken performs on every call. Concurrent callers for
// the same key share one GenerateAuthenticationToken call — via singleflight
// within this process, and via the token store's PutIfAbsent across processes
// when the store is Redis-backed (see newAuthService).
//
// A cached token found within tokenRefreshAheadWindow of its own expiry is
// still returned immediately, but also triggers a non-blocking background
// refresh (see refreshInBackground) so a later caller is unlikely to hit the
// synchronous miss path right at the moment of expiry.
func (s *AuthService) GetOrGenerateAuthenticationToken(ctx context.Context, email, password, app string) (string, error) {
	duration, err := s.jwt.GetDuration()
	if err != nil {
		return "", err
	}

	// NUL-separated, not "|" — email/app are trusted config values today, but a
	// plain delimiter is ambiguous: email="a|b",app="c" and email="a",app="b|c"
	// would otherwise collide on the same cache/singleflight key.
	key := email + "\x00" + app

	token, expiry, ok, err := s.tokens.store.Get(ctx, key)
	if err != nil {
		return "", err
	}
	if ok {
		if time.Until(expiry) <= tokenRefreshAheadWindow {
			s.refreshInBackground(ctx, key, email, password, app, duration)
		}

		return token, nil
	}

	return s.refreshAndWait(ctx, key, email, password, app, duration)
}

// refreshInBackground kicks off a non-blocking regeneration for key, deduped
// with any concurrent miss-path or other refresh-ahead trigger via the same
// singleflight key refreshAndWait uses — so if the cache entry fully expires
// while this is still in flight, the caller that hits the miss path joins
// this same call instead of starting a redundant second generation.
func (s *AuthService) refreshInBackground(ctx context.Context, key, email, password, app string, duration time.Duration) {
	// Detached from the triggering request's context/deadline — this refresh
	// is meant to outlive that request, not get canceled when it finishes.
	detached := context.WithoutCancel(ctx)

	go func() {
		defer afterBackgroundRefresh()

		_, _, _ = s.tokens.group.Do(key, func() (any, error) {
			// Re-check: another goroutine/process may have already refreshed
			// this since we decided to trigger — if it's no longer within
			// the refresh-ahead window, there's nothing left to do here.
			if token, expiry, ok, err := s.tokens.store.Get(detached, key); err != nil {
				return "", err
			} else if ok && time.Until(expiry) > tokenRefreshAheadWindow {
				return token, nil
			}

			return s.regenerateAndReplace(detached, key, email, password, app, duration)
		})
	}()
}

// afterBackgroundRefresh runs (on the background goroutine) after each
// refreshInBackground call completes. It's a no-op in production; tests
// override it (via SetAfterBackgroundRefreshFunc) to deterministically wait
// for a background refresh to finish before asserting on its effects —
// without this, a test could return (and restore its generateAuthToken fake)
// while the goroutine it triggered is still pending, which then runs later
// against whatever fake a *different* test has since installed.
var afterBackgroundRefresh = func() {}

// SetAfterBackgroundRefreshFunc overrides afterBackgroundRefresh for tests
// and returns the previous value so callers can restore it:
//
//	old := SetAfterBackgroundRefreshFunc(fn)
//	defer SetAfterBackgroundRefreshFunc(old)
func SetAfterBackgroundRefreshFunc(fn func()) func() {
	old := afterBackgroundRefresh
	afterBackgroundRefresh = fn

	return old
}

// refreshAndWait is the blocking miss path: the caller has no usable cached
// token at all (never cached, or its entry has fully expired) and must wait
// for a fresh one.
func (s *AuthService) refreshAndWait(ctx context.Context, key, email, password, app string, duration time.Duration) (string, error) {
	v, err, _ := s.tokens.group.Do(key, func() (any, error) {
		// Re-check after entering the in-process singleflight critical
		// section — another goroutine here (or another process, if the
		// store is shared, or an in-flight refreshInBackground call this one
		// just joined) may have already populated it while we waited.
		if token, _, ok, err := s.tokens.store.Get(ctx, key); err != nil {
			return "", err
		} else if ok {
			return token, nil
		}

		return s.regenerateAndCache(ctx, key, email, password, app, duration)
	})
	if err != nil {
		return "", err
	}

	return v.(string), nil
}

// regenerateAndCache mints a fresh token and stores it via PutIfAbsent,
// preferring an existing value if another process's PutIfAbsent wins the
// race first. Used by the blocking miss path (refreshAndWait), where
// there's no valid entry yet and avoiding duplicate concurrent generation
// across processes actually matters, since it'd otherwise block N callers
// on N redundant bcrypt+KMS round trips.
func (s *AuthService) regenerateAndCache(ctx context.Context, key, email, password, app string, duration time.Duration) (string, error) {
	token, err := generateAuthToken(s, email, password, app, duration)
	if err != nil {
		return "", err
	}

	stored, err := s.tokens.store.PutIfAbsent(ctx, key, token, computeTTL(duration))
	if err != nil {
		return "", err
	}
	if !stored {
		// Another process's PutIfAbsent won the race — prefer its value over
		// ours so the fleet converges on one cached token per key, not one
		// per process. Our freshly generated token is still valid; we just
		// don't keep it if a shared copy already exists.
		if existing, _, ok, getErr := s.tokens.store.Get(ctx, key); getErr == nil && ok {
			return existing, nil
		}
	}

	return token, nil
}

// regenerateAndReplace mints a fresh token and unconditionally overwrites
// the cached entry. Used by the background refresh-ahead path
// (refreshInBackground), where an entry already exists — just near expiry —
// and the whole point is to replace it; PutIfAbsent would never take effect
// there since the old entry hasn't lapsed yet. Two processes racing to
// refresh-ahead the same key at once may both pay the generation cost
// (whichever Put lands last wins) — accepted, since this only ever runs in
// the background and never blocks a caller, unlike the miss path above
// where avoiding duplicate work matters more.
func (s *AuthService) regenerateAndReplace(ctx context.Context, key, email, password, app string, duration time.Duration) (string, error) {
	token, err := generateAuthToken(s, email, password, app, duration)
	if err != nil {
		return "", err
	}

	if err := s.tokens.store.Put(ctx, key, token, computeTTL(duration)); err != nil {
		return "", err
	}

	return token, nil
}

// computeTTL shrinks duration by tokenRefreshMargin, the safety buffer that
// keeps a token pulled from cache from expiring mid-use — unless duration is
// already shorter than the margin, in which case there's nothing to shrink.
func computeTTL(duration time.Duration) time.Duration {
	if duration > tokenRefreshMargin {
		return duration - tokenRefreshMargin
	}

	return duration
}

func (s *AuthService) AuthenticateSVC(email, password string) (string, error) {
	duration, err := s.jwt.GetDuration()
	if err != nil {
		return "", err
	}

	token, err := s.GenerateAuthenticationToken(email, password, s.app.Name, duration)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrAuthTokenGenerationFailed, err)
	}

	return token, nil
}

func (s *AuthService) GetPermissionsSVC(ctx context.Context, userID string) ([]model.Permission, error) {
	cacheKeyStr := "permissions:user:" + strings.ToLower(userID)

	val, err := s.redis.Get(ctx, cacheKeyStr).Result()
	if err == nil {
		s.logger.Info("Redis Cache hit", slog.String("key", cacheKeyStr))

		var permissions []model.Permission

		unmarshalErr := json.Unmarshal([]byte(val), &permissions)
		if unmarshalErr == nil {
			return permissions, nil
		}

		s.logger.Warn("Failed to unmarshal permissions",
			slog.String("key", cacheKeyStr),
			slog.Any("error", unmarshalErr),
		)
	} else if !errors.Is(err, redis.Nil) {
		s.logger.Error("Redis error on GET",
			slog.String("key", cacheKeyStr),
			slog.Any("error", err),
		)
	}

	s.logger.Info("Redis Cache miss", slog.String("key", cacheKeyStr))

	permissions, dbErr := s.FindPermissionsByUserID(ctx, userID)
	if dbErr != nil {
		return nil, fmt.Errorf("%w: %v", ErrPermissionsFindFailed, dbErr)
	}

	data, marshalErr := json.Marshal(permissions)
	if marshalErr != nil {
		s.logger.Error("Failed to marshal permissions", slog.Any("error", marshalErr))
	} else {
		if setErr := s.redis.Set(ctx, cacheKeyStr, data, cacheExpiration).Err(); setErr != nil {
			s.logger.Error("Failed to set Redis cache",
				slog.String("key", cacheKeyStr),
				slog.Any("error", setErr),
			)
		}
	}

	return permissions, nil
}

func (s *AuthService) GetTokenByUserIDAndScopeSVC(ctx context.Context, userID string, scope repository.TokenScope) (*model.Token, error) {
	token, err := s.FindTokenByUserIDAndScope(ctx, userID, scope)
	if err != nil {
		// %w (not %v) on both: middleware.validateTokenStatus does
		// errors.Is(err, repository.ErrTokenNotFound/ErrTokenScope) on this
		// error to distinguish a clean 401/403 from a genuine 500 — %v here
		// broke that chain and leaked raw internal errors as 500s.
		return nil, fmt.Errorf("%w: %w", ErrTokenFindFailed, err)
	}

	return token, nil
}

func (s *AuthService) Stop() {
	s.cancel()
	s.logger.Info("Stopping pg_notify listener...")
}

func (s *AuthService) listenForRoleChanges(ctx context.Context) {
	conn, err := s.setupRoleChangeListener(ctx)
	if err != nil {
		return
	}
	defer conn.Release()

	s.logger.Info("Listening for role changes...")

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Stopping role change listener...")

			return
		default:
			if err := s.handleRoleChangeNotification(ctx, conn); err != nil {
				// The leased conn is dead (or ctx was canceled) — WaitForNotification
				// fails instantly on either, so looping via `default` again would busy-spin
				// with no backoff. Stop instead; the pool connection is released above.
				if errors.Is(err, context.Canceled) {
					s.logger.Info("Stopping role change listener...")
				} else {
					s.logger.Error("listenForRoleChanges: stopping after notification error", "error", err)
				}

				return
			}
		}
	}
}

func (s *AuthService) setupRoleChangeListener(ctx context.Context) (*pgxpool.Conn, error) {
	if s.db == nil {
		return nil, ErrPostgresConnectionNotAvailable
	}

	conn, err := s.db.Acquire(ctx)
	if err != nil {
		s.logger.Error("listenForRoleChanges: Failed to acquire connection", "error", err)

		return nil, fmt.Errorf("%w: %v", ErrDatabaseConnectionAcquireFailed, err)
	}

	_, err = conn.Exec(ctx, "LISTEN role_changes;")
	if err != nil {
		s.logger.Error("listenForRoleChanges: Failed to LISTEN", "error", err)
		conn.Release()

		return nil, fmt.Errorf("%w: %v", ErrDatabaseListenCommandFailed, err)
	}

	return conn, nil
}

func (s *AuthService) handleRoleChangeNotification(ctx context.Context, conn *pgxpool.Conn) error {
	notification, err := conn.Conn().WaitForNotification(ctx)
	if err != nil {
		return err
	}

	roleID := strings.ToLower(notification.Payload)
	s.logger.Info("Role changed, scanning for matching keys", "role", roleID)

	s.clearCachedPermissionsForRole(ctx, roleID)

	return nil
}

func (s *AuthService) clearCachedPermissionsForRole(ctx context.Context, roleID string) {
	iter := s.redis.Scan(ctx, 0, "permissions:*", 0).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		if strings.Contains(key, roleID) {
			s.deleteCachedPermission(ctx, key)
		}
	}

	if err := iter.Err(); err != nil {
		s.logger.Error("Error scanning Redis keys", "error", err)
	}
}

func (s *AuthService) deleteCachedPermission(ctx context.Context, key string) {
	if err := s.redis.Del(ctx, key).Err(); err != nil {
		s.logger.Error("Failed to delete Redis key", "key", key, "error", err)
	} else {
		s.logger.Info("Deleted cached permission", "key", key)
	}
}
