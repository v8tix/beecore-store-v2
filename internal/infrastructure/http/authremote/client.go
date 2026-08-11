// Package authremote is the kawa-based HTTP client implementing
// resource.AuthRemote, ported 1:1 from
// internal/business/core/repository/auth_repository.go and the
// auth-relevant methods of internal/business/core/repository/user_repository.go
// in the source repo (see port/resource.AuthRemote's doc comment for the
// full mapping, including the borrowed HasAddresses method).
package authremote

import (
	"errors"
	"fmt"
	"time"

	"github.com/v8tix/beecore-eda/config"
	users "github.com/v8tix/beecore-http/messages/users/v1"
	"github.com/v8tix/kawa"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/port/resource"
	"github.com/v8tix/beecore-store-v2/internal/infrastructure/http/httpshared"
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
	return httpshared.Deadline(c.cfg)
}

func (c *Client) retryPolicy() kawa.RetryPolicy {
	return httpshared.RetryPolicy(c.cfg)
}

// translateHTTPError turns a kawa.ErrInvalidHTTPStatus into a plain error
// carrying the downstream response body as its message, for call sites
// that don't need to distinguish specific downstream rejection reasons.
// Non-HTTP errors (network failures) pass through unchanged.
//
// TODO(httpshared): still duplicated per-adapter — addressremote's and
// basketremote's versions diverge (they extract only the parsed message
// and drop the status code on a JSON-parse failure); unifying that is
// deferred to its own follow-up commit rather than folded into this
// dedup pass.
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
