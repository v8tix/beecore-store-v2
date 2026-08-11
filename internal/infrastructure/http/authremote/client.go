// Package authremote is the kawa-based HTTP client implementing
// resource.AuthRemote, ported 1:1 from
// internal/business/core/repository/auth_repository.go and the
// auth-relevant methods of internal/business/core/repository/user_repository.go
// in the source repo (see port/resource.AuthRemote's doc comment for the
// full mapping, including the borrowed HasAddresses method).
package authremote

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/v8tix/beecore-eda/config"
	users "github.com/v8tix/beecore-http/messages/users/v1"
	"github.com/v8tix/kawa"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/port/resource"
)

// Client is the outbound HTTP boundary for the auth vertical slice. It
// holds *config.Cfg directly (same as source repo's BaseRepositoryImpl
// struct) — the URLs, shared *http.Client, retry/deadline profile, admin
// token cache and admin/app credentials it needs all live there already.
type Client struct {
	cfg *config.Cfg
}

var _ resource.AuthRemote = (*Client)(nil)

func NewClient(cfg *config.Cfg) *Client {
	return &Client{cfg: cfg}
}

// deadline and retryPolicy mirror BaseRepositoryImpl.deadline/retryPolicy
// in the source repo: every call here is a single-record lookup, so they
// use the config-tunable "fast" HTTP client profile.
func (c *Client) deadline() time.Duration {
	return c.cfg.HTTPClient.GetFast().Timeout.Duration()
}

func (c *Client) retryPolicy() kawa.RetryPolicy {
	profile := c.cfg.HTTPClient.GetFast()
	return kawa.NewExponentialRetryPolicy(profile.MaxRetries, profile.RetryInterval.Duration())
}

func buildAuthHeader(token string) map[string]string {
	return map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
	}
}

// errMessage mirrors model.ErrMessage in the source repo — the shape of a
// downstream error response body: {"error": "..."}.
type errMessage struct {
	Error string `json:"error"`
}

// decodeHTTPError extracts a kawa.ErrInvalidHTTPStatus's status code and
// parsed error message from err. ok is false when err isn't an HTTP
// status error (e.g. a network failure) — callers must propagate err as-is
// in that case. parseErr is non-nil when the body isn't valid
// errMessage JSON.
func decodeHTTPError(err error) (statusCode int, message string, parseErr error, ok bool) {
	var errHTTP kawa.ErrInvalidHTTPStatus
	if !errors.As(err, &errHTTP) {
		return 0, "", nil, false
	}

	var em errMessage
	if unmarshalErr := json.Unmarshal(errHTTP.Body, &em); unmarshalErr != nil {
		return errHTTP.StatusCode, "", unmarshalErr, true
	}

	return errHTTP.StatusCode, em.Error, nil, true
}

// translateHTTPError turns a kawa.ErrInvalidHTTPStatus into a plain error
// carrying the downstream response body as its message, for call sites
// that don't need to distinguish specific downstream rejection reasons.
// Non-HTTP errors (network failures) pass through unchanged.
func translateHTTPError(err error) error {
	var errHTTP kawa.ErrInvalidHTTPStatus
	if errors.As(err, &errHTTP) {
		return fmt.Errorf("downstream request failed with status %d: %s", errHTTP.StatusCode, string(errHTTP.Body))
	}

	return err
}

func safeString(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}

func toDomainUser(u users.User) domain.User {
	return domain.User{
		ID:        u.ID,
		RoleID:    u.RoleID,
		DNI:       safeString(u.DNI),
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Email:     u.Email,
		Birthday:  safeString(u.Birthday),
		Genre:     safeString(u.Genre),
		Phone:     safeString(u.Phone),
		ImgURL:    safeString(u.ImgURL),
		Website:   safeString(u.Website),
		Enabled:   u.Enabled,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}
