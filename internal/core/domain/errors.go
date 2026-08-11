package domain

import "errors"

// Sentinel errors returned by core/auth's use-case (and, upstream of it,
// infrastructure/http/authremote's adapter) so a handler can decide which
// form field to flag without importing any transport-specific error type
// (e.g. kawa.ErrInvalidHTTPStatus) — see cmd/web/handlers.go in the source
// repo, which inspects the downstream JSON error body directly today. The
// adapter is responsible for translating a downstream response into one of
// these; everything above it works with plain Go errors only.
var (
	// ErrUserNotFound mirrors the downstream user service's "user not
	// found" response — used both for a failed credentials lookup (login)
	// and a failed email lookup (signup's email-in-use check, forgotten
	// password's email check).
	ErrUserNotFound = errors.New("user not found")

	// ErrInvalidCredentials mirrors the downstream user service's
	// "invalid password" response for an existing user.
	ErrInvalidCredentials = errors.New("invalid password")

	// ErrEmailInUse indicates a signup attempt used an email already
	// registered to another user.
	ErrEmailInUse = errors.New("email already in use")

	// ErrInvalidActivationToken mirrors the downstream user service's
	// "token does not match" response from ActivateUser.
	ErrInvalidActivationToken = errors.New("token does not match")

	// ErrSessionRejected mirrors isSessionRejected in the source repo's
	// cmd/web/helpers.go: the downstream auth service explicitly rejected
	// an access-token refresh (401/403 — a revoked/expired/mismatched
	// refresh token, or a disabled user), as opposed to a transient
	// failure (network error, 5xx). resource.AuthRemote.RefreshUserToken
	// returns this sentinel instead of a kawa-specific status check so a
	// future session-resolution middleware can decide whether to
	// invalidate the session outright, without needing to know kawa
	// exists.
	ErrSessionRejected = errors.New("session refresh rejected")
)
