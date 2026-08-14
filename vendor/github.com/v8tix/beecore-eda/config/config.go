package config

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	kms "cloud.google.com/go/kms/apiv1"
	"cloud.google.com/go/storage"
	"github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/sessions"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/redis/go-redis/v9"
	"github.com/samber/lo"
	"github.com/v8tix/beecore-eda/smtp"
	"github.com/v8tix/beecore-http/worker"
	"github.com/v8tix/kawa"
	"github.com/v8tix/kawa/policy"
	"github.com/v8tix/kawa/transport"
)

var (
	// ErrStorageClientCreationFailed indicates that creating storage client failed.
	ErrStorageClientCreationFailed = errors.New("storage client creation failed")
	// ErrMailerCreationFailed indicates that creating mailer failed.
	ErrMailerCreationFailed = errors.New("mailer creation failed")
	// ErrPostgresConnectionFailed indicates that connecting to postgres failed.
	ErrPostgresConnectionFailed = errors.New("postgres connection failed")
	// ErrPostgresPingFailed indicates that pinging postgres failed.
	ErrPostgresPingFailed = errors.New("postgres ping failed")
	// ErrRedisPingFailed indicates that pinging redis failed.
	ErrRedisPingFailed = errors.New("redis ping failed")
	// ErrNATSConnectionFailed indicates that connecting to NATS failed.
	ErrNATSConnectionFailed = errors.New("nats connection failed")
	// ErrJetStreamCreationFailed indicates that creating JetStream failed.
	ErrJetStreamCreationFailed = errors.New("jetstream creation failed")
	// ErrStreamInfoFailed indicates that getting stream info failed.
	ErrStreamInfoFailed = errors.New("stream info failed")
	// ErrStreamAddFailed indicates that adding stream failed.
	ErrStreamAddFailed = errors.New("stream add failed")
	// ErrKMSClientCreationFailed indicates that creating KMS client failed.
	ErrKMSClientCreationFailed = errors.New("kms client creation failed")
	// ErrUnknownService indicates that service is unknown.
	ErrUnknownService = errors.New("unknown service")
	// ErrInitMessageWriteFailed indicates that writing initialization message failed.
	ErrInitMessageWriteFailed = errors.New("initialization message write failed")
	// ErrInvalidCookieSecretKey indicates that cookie_secret_key is set but not
	// a valid AES key length (16, 24, or 32 bytes) — securecookie requires an
	// exact length and silently skips encryption on a mismatch, so this is
	// checked explicitly rather than left to fail invisibly.
	ErrInvalidCookieSecretKey = errors.New("cookie_secret_key must be 16, 24, or 32 bytes for AES-128/192/256")
	// ErrEmailSendFailed indicates that sending email failed.
	ErrEmailSendFailed = errors.New("email send failed")
)

type OnceWithErr struct {
	once sync.Once
	err  error
}

func (o *OnceWithErr) Do(fn func() error) error {
	o.once.Do(func() {
		o.err = fn()
	})

	return o.err
}

type Service string

const (
	ServiceLogger       Service = "logger"
	ServiceJWT          Service = "jwt"
	ServiceWeb          Service = "web"
	ServiceWorker       Service = "worker"
	ServiceRedis        Service = "redis"
	ServicePostgres     Service = "postgres"
	ServiceNATS         Service = "nats"
	ServiceJetStream    Service = "jetstream"
	ServiceKMS          Service = "kms"
	ServiceSMTP         Service = "smtp"
	ServiceCORS         Service = "cors"
	ServiceHTTPClient   Service = "HTTP client"
	ServiceAuth                 = "auth"
	ServiceCloudStorage Service = "cloud storage"
	ServiceSessionStore Service = "session store"
)

// parseDuration parses a duration string, returning defaultVal on empty or error.
func parseDuration(s string, defaultVal time.Duration) time.Duration {
	if s == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return defaultVal
	}
	return d
}

type App struct {
	Name                      string   `json:"name,omitempty"`
	SupportEmail              string   `json:"support_email,omitempty"`
	ActivationTokenExpiration Duration `json:"activation_token_expiration"`
}

// GetActivationTokenExpiration returns the configured activation-token TTL,
// defaulting to 72 hours if unset.
func (a App) GetActivationTokenExpiration() time.Duration {
	if a.ActivationTokenExpiration.Duration() == 0 {
		return 72 * time.Hour
	}

	return a.ActivationTokenExpiration.Duration()
}

// Log holds logging configuration.
type Log struct {
	// Level sets the minimum log level: "debug", "info", "warn", "error". Default: "info".
	Level string `json:"level,omitempty"`
	// Format sets the log output format: "json" or "text". Default: "json".
	Format string `json:"format,omitempty"`
}

// JWT represents JWT configuration settings.
type JWT struct {
	ExpirationDuration string `json:"expiration_duration,omitempty"` // e.g., "24h", "15m"
	RefreshInterval    string `json:"refresh_interval,omitempty"`    // e.g., "24h" — how often a session's access token is refreshed
}

// GetDuration parses and returns the expiration duration.
func (j *JWT) GetDuration() (time.Duration, error) {
	if j.ExpirationDuration == "" {
		return 24 * time.Hour, nil // Default to 72 hours
	}

	return time.ParseDuration(j.ExpirationDuration)
}

// GetRefreshInterval parses and returns the session refresh interval.
func (j *JWT) GetRefreshInterval() (time.Duration, error) {
	if j.RefreshInterval == "" {
		return 24 * time.Hour, nil
	}

	return time.ParseDuration(j.RefreshInterval)
}

type Cors struct {
	TrustedOrigins []string `json:"trusted_origins,omitempty"`
}

type Pagination struct {
	PerPage int `json:"per_page,omitempty"`
}

type UserRoles struct {
	StoreAdminID string `json:"store_admin_id,omitempty"`
	StoreBuyerID string `json:"store_buyer_id,omitempty"`
}

type Web struct {
	Host              string `json:"host,omitempty"`
	AdminPort         string `json:"admin_port,omitempty"`
	StorePort         string `json:"store_port,omitempty"`
	UsersPort         string `json:"users_port,omitempty"`
	StoresPort        string `json:"stores_port,omitempty"`
	BasketsPort       string `json:"baskets_port,omitempty"`
	PaymentsPort      string `json:"payments_port,omitempty"`
	OrdersPort        string `json:"orders_port,omitempty"`
	NotificationsPort string `json:"notifications_port,omitempty"`
	Cors              Cors   `json:"cors"`
	AdminBaseURL      string `json:"admin_base_url,omitempty"`
	ShutdownTimeout   int    `json:"shutdown_timeout,omitempty"`
	// HTTPClientTimeout is the total request timeout for the shared HTTP client (e.g. "5s"). Default: 5s.
	HTTPClientTimeout string `json:"http_client_timeout,omitempty"`
	// HTTPClientIdleConnTimeout is the idle connection keep-alive timeout (e.g. "15s"). Default: 15s.
	HTTPClientIdleConnTimeout string `json:"http_client_idle_conn_timeout,omitempty"`
	HTTPClient                *http.Client
}

type Session struct {
	CookieSecretKey     string `json:"cookie_secret_key,omitempty"`
	SessionSecretKey    string `json:"session_secret_key,omitempty"`
	OldSessionSecretKey string `json:"old_session_secret_key,omitempty"`
	// MaxAge is the session cookie lifetime in seconds. Default: 604800 (7 days).
	MaxAge       int `json:"max_age,omitempty"`
	SessionStore *sessions.CookieStore
}

// PostgresPool holds connection pool tuning parameters.
type PostgresPool struct {
	// MaxConns is the maximum number of connections in the pool. Default: 25.
	MaxConns int `json:"max_conns,omitempty"`
	// MinConns is the minimum number of idle connections maintained. Default: 5.
	MinConns int `json:"min_conns,omitempty"`
	// MaxConnLifetime is the maximum lifetime of a connection (e.g. "1h"). Default: 1h.
	MaxConnLifetime string `json:"max_conn_lifetime,omitempty"`
	// MaxConnIdleTime is the maximum idle time before a connection is closed (e.g. "30m"). Default: 30m.
	MaxConnIdleTime string `json:"max_conn_idle_time,omitempty"`
}

type Postgres struct {
	Dsn  string       `json:"dsn,omitempty"`
	Pool PostgresPool `json:"pool"`
	DB   *pgxpool.Pool
	init OnceWithErr
}

type Nats struct {
	URL       string `json:"url,omitempty"`
	Stream    string `json:"stream,omitempty"`
	Conn      *nats.Conn
	JetStream nats.JetStreamContext
	init      OnceWithErr
	jsInit    OnceWithErr
}

type KMS struct {
	Credentials string `json:"credentials_path,omitempty"`
	ProjectID   string `json:"project_id,omitempty"`
	Location    string `json:"location,omitempty"`
	KeyRing     string `json:"key_ring,omitempty"`
	KeyID       string `json:"key_id,omitempty"`
	Version     int    `json:"version,omitempty"`
	Client      *kms.KeyManagementClient
	init        OnceWithErr
}

type CloudStorage struct {
	BucketName string `json:"bucket_name,omitempty"`
	BucketKey  string `json:"bucket_key,omitempty"` // path to JSON key
	Client     *storage.Client
	init       OnceWithErr
}

type Redis struct {
	Addr      string `json:"addr,omitempty"`
	Password  string `json:"password,omitempty"`
	DB        int    `json:"db,omitempty"`
	BufferTTL string `json:"buffer_ttl,omitempty"` // TTL for event buffer (default: 1h), e.g., "24h", "1h"
	Client    *redis.Client
	init      OnceWithErr
}

// GetBufferTTL parses and returns the buffer TTL duration.
func (r *Redis) GetBufferTTL() (time.Duration, error) {
	if r.BufferTTL == "" {
		return 1 * time.Hour, nil // Default to 1 hour
	}

	return time.ParseDuration(r.BufferTTL)
}

type SMTP struct {
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Sender   string `json:"sender,omitempty"`
	Client   *smtp.Mailer
	init     OnceWithErr
}

type PayPal struct {
	ClientID        string `json:"client_id,omitempty"`
	Secret          string `json:"secret,omitempty"`
	AuthURL         string `json:"auth_url,omitempty"`
	OrdersURL       string `json:"orders_url,omitempty"`
	OrdersReturnURL string `json:"orders_return_url,omitempty"`
	OrdersCancelURL string `json:"orders_cancel_url,omitempty"`
}
type Payphone struct {
	Token       string `json:"token,omitempty"`
	StoreID     string `json:"store_id,omitempty"`
	BaseURL     string `json:"base_url,omitempty"`
	Environment string `json:"environment,omitempty"`
	TimeZone    string `json:"time_zone,omitempty"`
	Latitude    string `json:"latitude,omitempty"`
	Longitude   string `json:"longitude,omitempty"`
	Language    string `json:"language,omitempty"`
}

type Admin struct {
	Email    string `json:"email,omitempty"`
	Password string `json:"password,omitempty"`
}

// Worker represents worker configuration settings.
type Worker struct {
	Concurrency int `json:"concurrency,omitempty"`
	QueueSize   int `json:"queue_size,omitempty"`
}

// ISO4217 represents ISO 4217 currency code information.
type ISO4217 struct {
	Alpha   string `json:"alpha,omitempty"`
	Numeric int    `json:"numeric,omitempty"`
}

// Currency represents currency configuration.
type Currency struct {
	Name    string  `json:"name,omitempty"`
	Iso4217 ISO4217 `json:"iso_4217"`
}

// Shipping represents shipping configuration.
type Shipping struct {
	Amount   int    `json:"amount,omitempty"`
	Currency string `json:"currency,omitempty"`
}

// Taxes represents tax configuration.
type Taxes struct {
	Percentage int    `json:"percentage,omitempty"`
	Type       string `json:"type,omitempty"`
}

// Country represents country-specific business configuration.
type Country struct {
	Name            string   `json:"name,omitempty"`
	PaymentStrategy string   `json:"payment_strategy,omitempty"`
	Currency        Currency `json:"currency"`
	Shipping        Shipping `json:"shipping"`
	Taxes           Taxes    `json:"taxes"`
}

// BusinessParameters represents business-specific configuration parameters.
type BusinessParameters struct {
	Country Country `json:"country"`
}

// OutboxRetryPolicy configures the per-aggregate poison-pill retry behaviour.
// Leave all fields at zero to use the processor defaults (3 retries, 5s→10m exponential).
type OutboxRetryPolicy struct {
	// MaxRetries is the maximum number of publish attempts before marking the aggregate as failed. Default: 3.
	MaxRetries int `json:"max_retries,omitempty"`
	// InitialInterval is the first backoff wait (e.g. "5s"). Default: 5s.
	InitialInterval string `json:"initial_interval,omitempty"`
	// MaxInterval is the ceiling for exponential backoff (e.g. "10m"). Default: 10m.
	MaxInterval string `json:"max_interval,omitempty"`
	// Multiplier grows the interval each retry (e.g. 2.0). Default: 2.0.
	Multiplier float64 `json:"multiplier,omitempty"`
}

// OutboxPartition holds outbox partition settings for horizontal scaling.
// When running multiple outbox processors against the same table (e.g. a
// dedicated outbox service alongside the API service), assign each process a
// unique PartitionID in the range [0, TotalPartitions).
//
// Example – two processors sharing "users.outbox":
//
//	API service config:     {"partition_id": 0, "total_partitions": 2}
//	Outbox service config:  {"partition_id": 1, "total_partitions": 2}
//
// Leave TotalPartitions as 0 or 1 (or omit the block entirely) to use the
// standard non-partitioned store – this is the default and is fully backward
// compatible with existing deployments.
type OutboxPartition struct {
	PartitionID     int               `json:"partition_id"`
	TotalPartitions int               `json:"total_partitions"`
	RetryPolicy     OutboxRetryPolicy `json:"retry_policy"`
}

// IsPartitioned reports whether this configuration describes a partitioned
// outbox setup (i.e. TotalPartitions > 1).
func (o OutboxPartition) IsPartitioned() bool {
	return o.TotalPartitions > 1
}

// IntegrationV1 represents version 1 integration configuration.
type IntegrationV1 struct {
	AuthURL              string `json:"auth_url,omitempty"`
	StoresURL            string `json:"stores_url,omitempty"`
	BasketsURL           string `json:"baskets_url,omitempty"`
	PaymentsURL          string `json:"payments_url,omitempty"`
	OrdersURL            string `json:"orders_url,omitempty"`
	ResetPasswdMerchants string `json:"merchants_passwd_url,omitempty"`
	ResetPasswdBuyers    string `json:"buyers_passwd_url,omitempty"`
}

// Integration represents integration configuration.
type Integration struct {
	V1 IntegrationV1 `json:"v1"`
}

// Cfg represents the main application configuration.
type Cfg struct {
	App                App                `json:"app"`
	Log                Log                `json:"log"`
	JWT                JWT                `json:"jwt"`
	Web                Web                `json:"web"`
	Postgres           Postgres           `json:"postgres"`
	Nats               Nats               `json:"nats"`
	KMS                KMS                `json:"kms"`
	Redis              Redis              `json:"redis"`
	SMTP               SMTP               `json:"smtp"`
	Integration        Integration        `json:"integration"`
	HTTPClient         HTTPClient         `json:"http_client"`
	Admin              Admin              `json:"admin"`
	Pagination         Pagination         `json:"pagination"`
	UserRoles          UserRoles          `json:"user_roles"`
	CloudStorage       CloudStorage       `json:"cloud_storage"`
	Session            Session            `json:"session"`
	Worker             Worker             `json:"worker"`
	PayPal             PayPal             `json:"paypal"`
	Payphone           Payphone           `json:"payphone"`
	BusinessParameters BusinessParameters `json:"business_parameters"`
	Outbox             OutboxPartition    `json:"outbox"`
	AuthService
	WorkerCfg *worker.Cfg
	*slog.Logger
}

// defaultSessionMaxAge is the session cookie/Redis-session/refresh-token
// ceiling used whenever session.max_age is unset — shared by
// initSessionStore (cookie MaxAge) and AuthService.GetRefreshTokenTTL
// (refresh-token TTL) so the two can't silently drift apart.
const defaultSessionMaxAge = 7 * 24 * time.Hour

// sessionBlockKey validates and returns the AES block key derived from
// cookie_secret_key. Empty is allowed (encryption is opt-in — cookies stay
// signed-only, today's behavior) but a *set* key of the wrong length is
// rejected outright: securecookie's own New() swallows an invalid-length
// key by silently skipping encryption, which would otherwise fail closed
// (or rather, fail *open* — an operator who thinks they've enabled
// encryption but made a typo would never find out).
func sessionBlockKey(cookieSecretKey string) ([]byte, error) {
	if cookieSecretKey == "" {
		return nil, nil
	}

	key := []byte(cookieSecretKey)
	switch len(key) {
	case 16, 24, 32:
		return key, nil
	default:
		return nil, ErrInvalidCookieSecretKey
	}
}

func (s *Session) initSessionStore() error {
	blockKey, err := sessionBlockKey(s.CookieSecretKey)
	if err != nil {
		return err
	}

	keyPairs := [][]byte{[]byte(s.SessionSecretKey), blockKey}
	if s.OldSessionSecretKey != "" {
		keyPairs = append(keyPairs, []byte(s.OldSessionSecretKey), blockKey)
	}

	maxAge := s.MaxAge
	if maxAge == 0 {
		maxAge = int(defaultSessionMaxAge.Seconds())
	}

	sessionStore := sessions.NewCookieStore(keyPairs...)
	sessionStore.Options = &sessions.Options{
		HttpOnly: true,
		MaxAge:   maxAge,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
		Secure:   true,
	}

	s.SessionStore = sessionStore

	return nil
}

func (c *CloudStorage) initCloudStorage(ctx context.Context) error {
	return c.init.Do(func() error {
		// Set GOOGLE_APPLICATION_CREDENTIALS environment variable
		// Both GCS and KMS will use this unified credential file
		if err := os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", c.BucketKey); err != nil {
			return fmt.Errorf("%w: failed to set credentials: %v", ErrStorageClientCreationFailed, err)
		}

		// Create client without explicit credentials - uses ADC from environment variable
		client, err := storage.NewClient(ctx)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrStorageClientCreationFailed, err)
		}

		c.Client = client

		return nil
	})
}

func (c *Cfg) initAuthService(ctx context.Context) error {
	svc := newAuthService(ctx, c)

	c.AuthService = svc

	return nil
}

func (s *SMTP) initMailer() error {
	return s.init.Do(func() error {
		mailer, err := smtp.NewMailer(s.Host, s.Username, s.Password, s.Sender, s.Port)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrMailerCreationFailed, err)
		}

		s.Client = mailer

		return nil
	})
}

func (w *Web) initCors() error {
	w.Cors.TrustedOrigins = lo.Map(w.Cors.TrustedOrigins, func(origin string, _ int) string {
		return origin
	})

	return nil
}

func (w *Web) initHTTPClient() error {
	timeout := parseDuration(w.HTTPClientTimeout, 5*time.Second)
	idleConnTimeout := parseDuration(w.HTTPClientIdleConnTimeout, 15*time.Second)

	w.HTTPClient = kawa.NewHTTPClient(
		timeout,
		policy.OneRedirect,
		transport.IdleConnectionTimeout(idleConnTimeout),
	)

	return nil
}

func (c *Cfg) initWorker() error {
	var wg sync.WaitGroup

	var mu sync.Mutex

	c.WorkerCfg = worker.NewWorkerCfg(&wg, &mu)

	return nil
}

func (p *Postgres) initPostgres(ctx context.Context) error {
	return p.init.Do(func() error {
		poolConfig, err := pgxpool.ParseConfig(p.Dsn)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrPostgresConnectionFailed, err)
		}

		maxConns := int32(p.Pool.MaxConns)
		if maxConns == 0 {
			maxConns = 25
		}
		minConns := int32(p.Pool.MinConns)
		if minConns == 0 {
			minConns = 5
		}

		poolConfig.MaxConns = maxConns
		poolConfig.MinConns = minConns
		poolConfig.MaxConnLifetime = parseDuration(p.Pool.MaxConnLifetime, time.Hour)
		poolConfig.MaxConnIdleTime = parseDuration(p.Pool.MaxConnIdleTime, 30*time.Minute)

		db, err := pgxpool.NewWithConfig(ctx, poolConfig)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrPostgresConnectionFailed, err)
		}

		if err := db.Ping(ctx); err != nil {
			return fmt.Errorf("%w: %v", ErrPostgresPingFailed, err)
		}

		p.DB = db

		return nil
	})
}

func (r *Redis) initRedis(ctx context.Context) error {
	return r.init.Do(func() error {
		rdb := redis.NewClient(&redis.Options{
			Addr:     r.Addr,
			Password: r.Password,
			DB:       r.DB,
		})
		if err := rdb.Ping(ctx).Err(); err != nil {
			return fmt.Errorf("%w: %v", ErrRedisPingFailed, err)
		}

		r.Client = rdb

		return nil
	})
}

func (n *Nats) initNATS() error {
	return n.init.Do(func() error {
		nc, err := nats.Connect(n.URL)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrNATSConnectionFailed, err)
		}

		n.Conn = nc

		return nil
	})
}

func (n *Nats) initJetStream() error {
	return n.jsInit.Do(func() error {
		// Create new JetStream instance using new API
		js, err := jetstream.New(n.Conn)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrJetStreamCreationFailed, err)
		}

		// Check if stream exists using new API
		ctx := context.Background()
		_, err = js.Stream(ctx, n.Stream)
		if err != nil {
			if errors.Is(err, jetstream.ErrStreamNotFound) {
				// Create stream with new API
				_, err := js.CreateStream(ctx, jetstream.StreamConfig{
					Name:     n.Stream,
					Subjects: []string{n.Stream + ".>"},
				})
				if err != nil {
					return fmt.Errorf("%w: %v", ErrStreamAddFailed, err)
				}
			} else {
				return fmt.Errorf("%w: %v", ErrStreamInfoFailed, err)
			}
		}

		// Store old API context for backward compatibility (if needed)
		oldJS, err := n.Conn.JetStream()
		if err != nil {
			return fmt.Errorf("%w: %v", ErrJetStreamCreationFailed, err)
		}
		n.JetStream = oldJS

		return nil
	})
}

func (g *KMS) initKMS(ctx context.Context) error {
	return g.init.Do(func() error {
		// Set GOOGLE_APPLICATION_CREDENTIALS if not already set by Cloud Storage
		if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" {
			if err := os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", g.Credentials); err != nil {
				return fmt.Errorf("%w: failed to set credentials: %v", ErrKMSClientCreationFailed, err)
			}
		}

		// Create client without explicit credentials - uses ADC from environment variable
		client, err := kms.NewKeyManagementClient(ctx)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrKMSClientCreationFailed, err)
		}

		g.Client = client

		return nil
	})
}

// GetKMSKeyName returns the fully qualified KMS key name.
func (g *KMS) GetKMSKeyName() string {
	return fmt.Sprintf(
		"projects/%s/locations/%s/keyRings/%s/cryptoKeys/%s/cryptoKeyVersions/%d",
		g.ProjectID,
		g.Location,
		g.KeyRing,
		g.KeyID,
		g.Version,
	)
}

func (c *Cfg) initLogger() error {
	level := slog.LevelInfo
	switch strings.ToLower(c.Log.Level) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if strings.ToLower(c.Log.Format) == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	c.Logger = slog.New(handler)
	slog.SetDefault(c.Logger)

	return nil
}

func (c *Cfg) initJWT() error {
	jwt.TimeFunc = func() time.Time {
		return time.Now().UTC()
	}

	return nil
}

func (w *Web) initWeb() error {
	return w.initCors()
}

// InitSelected initializes the selected services in the configuration.
func (c *Cfg) InitSelected(ctx context.Context, services []Service) error {
	available := map[Service]func() error{
		ServiceLogger:       func() error { return c.initLogger() },
		ServiceJWT:          func() error { return c.initJWT() },
		ServiceWeb:          func() error { return c.Web.initWeb() },
		ServiceWorker:       func() error { return c.initWorker() },
		ServiceRedis:        func() error { return c.Redis.initRedis(ctx) },
		ServicePostgres:     func() error { return c.Postgres.initPostgres(ctx) },
		ServiceNATS:         c.Nats.initNATS,
		ServiceJetStream:    c.Nats.initJetStream,
		ServiceKMS:          func() error { return c.KMS.initKMS(ctx) },
		ServiceSMTP:         func() error { return c.SMTP.initMailer() },
		ServiceCORS:         func() error { return c.Web.initCors() },
		ServiceHTTPClient:   func() error { return c.Web.initHTTPClient() },
		ServiceAuth:         func() error { return c.initAuthService(ctx) },
		ServiceCloudStorage: func() error { return c.CloudStorage.initCloudStorage(ctx) },
		ServiceSessionStore: func() error { return c.Session.initSessionStore() },
	}

	for _, service := range services {
		initFn, ok := available[service]
		if !ok {
			return fmt.Errorf("%w: %s", ErrUnknownService, service)
		}

		if err := initFn(); err != nil {
			return fmt.Errorf("%s initialization failed: %w", service, err)
		}

		if c.Logger != nil {
			c.Info(fmt.Sprintf("%s initialized", service))
		} else {
			if _, err := fmt.Fprintf(os.Stderr, "%s initialized\n", service); err != nil {
				// If we can't write to stderr, there's not much we can do
				// This is a rare case, but we acknowledge the error
				return fmt.Errorf("%w: %v", ErrInitMessageWriteFailed, err)
			}
		}
	}

	return nil
}

// SendEmail sends an email using the configured SMTP client.
func (s *SMTP) SendEmail(
	email,
	subject,
	plainBody,
	htmlBody string,
) error {
	if err := s.Client.Send(email, subject, plainBody, htmlBody); err != nil {
		return fmt.Errorf("%w: %v", ErrEmailSendFailed, err)
	}

	return nil
}

// CloseAllClients closes all initialized clients and connections.
func (c *Cfg) CloseAllClients() {
	if c.Redis.Client != nil {
		if err := c.Redis.Client.Close(); err != nil {
			c.Error("Error closing Redis client", "error", err)
		} else {
			c.Info("Redis client closed")
		}
	}

	if c.Postgres.DB != nil {
		c.Postgres.DB.Close()
		c.Info("Postgres connection pool closed")
	}

	if c.Nats.Conn != nil {
		c.Nats.Conn.Close()
		c.Info("NATS connection closed")
	}

	if c.KMS.Client != nil {
		if err := c.KMS.Client.Close(); err != nil {
			c.Error("Error closing KMS client", "error", err)
		} else {
			c.Info("KMS client closed")
		}
	}

	if c.Web.HTTPClient != nil {
		c.Web.HTTPClient.CloseIdleConnections()
		c.Info("HTTP client idle connections closed")
	}

	if c.CloudStorage.Client != nil {
		if err := c.CloudStorage.Client.Close(); err != nil {
			c.Error("Error closing Cloud Storage client", "error", err)
		} else {
			c.Info("Cloud Storage client closed")
		}
	}
}
