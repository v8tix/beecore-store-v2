package resource

import (
	"context"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
)

//go:generate mockery --name AuthRemote

// AuthRemote is the outbound HTTP boundary for the auth slice, ported from
// internal/business/core/repository/auth_repository.go and the
// auth-relevant methods of internal/business/core/repository/user_repository.go
// in the source repo (both are just methods on the shared BaseRepositoryImpl
// struct there — this interface is the auth vertical slice's share of that
// same set of downstream calls). Every method name below matches its
// source counterpart; signatures gain a leading ctx and drop the
// kawa.Envelope/*_V1Req/*_V1Res wire-shape in favor of domain types or
// primitives — translating the downstream JSON error body into one of the
// domain.Err* sentinels (or a generic wrapped error) is this adapter's
// job, so core/auth and the handler never need to know kawa exists.
//
// HasAddresses is the one exception to the "auth_repository.go/
// user_repository.go only" scope: it's defined in address_repository.go in
// the source repo, but the login handler calls it to populate
// domain.Session's UserShippingAddress field (which decides the
// post-login redirect), and the Address vertical slice doesn't exist yet
// in this repo (Task 8 in the plan). Duplicating it here for now preserves
// login's behavior 1:1; when the Address slice lands, consider whether
// AuthRemote should keep its own copy or Login should take an
// AddressRemote dependency instead — same relocation beecore-admin-v2 did
// for its borrowed HasAddresses/FindStoreByUserID methods.
//
// FindTokenByEmailAndPassword and RefreshUserToken both decode the
// downstream POST auth/token and POST auth/refresh responses into the
// shared users.GetTokenEnvV1Res type, which — now that this repo's go.mod
// enables the beecore-http replace pointing at the fixed local checkout —
// declares RefreshToken alongside Token. Neither method reads that field
// (FindTokenByEmailAndPassword only ever needs the access token;
// RefreshUserToken's downstream endpoint doesn't rotate the refresh
// token), but decoding into the fixed shared type avoids
// DisallowUnknownFields choking on the refresh_token key that
// beecore-customers' real responses always include — the same class of
// bug that broke /password/forgot in beecore-admin before its fix.
// FindUserTokenByEmailAndPassword is the only method that actually reads
// RefreshToken.
type AuthRemote interface {
	// GetAdminToken returns the cached admin service-account token used to
	// authorize the calls below, refreshing it as needed. Mirrors
	// BaseRepositoryImpl.GetAdminToken.
	GetAdminToken(ctx context.Context) (string, error)

	// FindTokenByEmailAndPassword exchanges email/password for an access
	// token via POST auth/token, decoding into the shared (now-fixed)
	// users.GetTokenEnvV1Res but returning only the access token. Mirrors
	// BaseRepositoryImpl.FindTokenByEmailAndPassword.
	FindTokenByEmailAndPassword(ctx context.Context, email, password, token string) (accessToken string, err error)

	// FindUserByEmail looks up a user by email. err is
	// domain.ErrUserNotFound on a downstream 404. Mirrors
	// BaseRepositoryImpl.FindUserByEmail.
	FindUserByEmail(ctx context.Context, email, token string) (domain.User, error)

	// FindUserByEmailAndPassword verifies credentials and returns the
	// matching user. err is domain.ErrInvalidCredentials or
	// domain.ErrUserNotFound depending on the downstream rejection
	// reason. Mirrors BaseRepositoryImpl.FindUserByEmailAndPassword.
	FindUserByEmailAndPassword(ctx context.Context, email, password, token string) (domain.User, error)

	// FindUserTokenByEmailAndPassword exchanges the real user's own
	// email/password for their access+refresh token pair via POST
	// auth/token — same endpoint as FindTokenByEmailAndPassword, but
	// decoding the refresh_token field too. Mirrors
	// BaseRepositoryImpl.FindUserTokenByEmailAndPassword — unlike the
	// source method, this decodes directly into the shared (now-fixed)
	// users.GetTokenEnvV1Res rather than a local TokenPairEnvV1Res
	// workaround type, since the shared type now declares RefreshToken.
	FindUserTokenByEmailAndPassword(ctx context.Context, email, password, token string) (accessToken, refreshToken string, err error)

	// RegisterUser creates a new user account. domain.ErrEmailInUse is NOT
	// decided here (the source repo relies on a prior FindUserByEmail
	// check) — a downstream rejection surfaces as a generic error. Mirrors
	// BaseRepositoryImpl.RegisterUser.
	RegisterUser(ctx context.Context, firstName, lastName, email, password, confirmPassword, token, roleID string) (userID string, err error)

	// SendResetPasswdEmail triggers the downstream password-reset email.
	// Mirrors BaseRepositoryImpl.SendResetPasswdEmail.
	SendResetPasswdEmail(ctx context.Context, email, token string) error

	// ActivateUser confirms a user's account with their activation code.
	// err is domain.ErrInvalidActivationToken when the code doesn't
	// match. Mirrors BaseRepositoryImpl.ActivateUser.
	ActivateUser(ctx context.Context, token, userID, activationToken string) error

	// UpdateUserPassword sets a new password for userID. Mirrors
	// BaseRepositoryImpl.UpdateUserPassword.
	UpdateUserPassword(ctx context.Context, userID, password, confirmPassword, token string) error

	// HasAddresses reports whether userID has at least one registered
	// address, and its first address's ID if so (Login only needs the
	// first one, matching addresses[0].ID in the source handler). Mirrors
	// BaseRepositoryImpl.HasAddresses — see the type-level doc comment
	// above for why this lives on AuthRemote rather than an
	// AddressRemote, including the preserved quirk of swallowing any
	// downstream error whose body parses as valid JSON.
	HasAddresses(ctx context.Context, userID, token string) (hasAddresses bool, firstAddressID string, err error)

	// RefreshUserToken exchanges a session's refresh token for a new
	// access token via POST auth/refresh, without the user's password —
	// mirrors BaseRepositoryImpl.RefreshUserToken, used by the source
	// repo's session-refresh helper (cmd/web/helpers.go's
	// refreshIfNeeded) once a session's access token is close to its own
	// expiry. err is domain.ErrSessionRejected when the downstream
	// response is 401/403 (a revoked/expired/mismatched refresh token, or
	// a disabled user) — a future session-refresh path can use that to
	// decide the session is genuinely dead and should be deleted, rather
	// than just transiently unrefreshable. Not yet called from
	// core/auth's use-case (no session-refresh middleware exists in this
	// repo yet), but ported alongside FindTokenByEmailAndPassword since
	// both are the two refresh_token strict-decode-failure risks the
	// Foundation phase's beecore-http fix resolves.
	RefreshUserToken(ctx context.Context, userID, refreshToken, token string) (accessToken string, err error)
}
