package service

import (
	"context"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
)

//go:generate mockery --name Auth

// Auth is the login/signup/activation/password-reset use-case, ported from
// the login/signup/activate/forgottenPassword/passwordReset sections of
// cmd/web/handlers.go in the source repo. Every downstream call and
// business decision those handlers made (which admin-token path to use,
// how to interpret a downstream "invalid password"/"user not found"/"token
// does not match" response, session assembly) lives behind this interface
// now — handlers only decode/validate the request, call one of these
// methods, and render a response.
type Auth interface {
	// Login authenticates a user by email/password, assembles their
	// server-side domain.Session (mirroring session.Blob in the source
	// repo's login handler — including the address lookup that decides
	// the post-login redirect), persists it via resource.SessionStore,
	// and returns the opaque session ID the handler must put in the
	// browser's session cookie, alongside the session record and the
	// authenticated domain.User (used for template data).
	//
	// err is domain.ErrInvalidCredentials or domain.ErrUserNotFound for a
	// rejected login, or a generic error for anything else (network,
	// downstream 5xx, etc).
	Login(ctx context.Context, email, password string) (sessionID string, session domain.Session, user domain.User, err error)

	// Signup registers a new storefront buyer user. err is
	// domain.ErrEmailInUse if the email is already registered, or a
	// generic error otherwise. userID is the new user's ID, used by the
	// handler to build the /activate/{userID} redirect.
	Signup(ctx context.Context, firstName, lastName, email, password, confirmPassword string) (userID string, err error)

	// Activate confirms a user's account with the activation code emailed
	// to them. err is domain.ErrInvalidActivationToken if the code doesn't
	// match, or a generic error otherwise.
	Activate(ctx context.Context, userID, activationCode string) error

	// ForgotPassword triggers a password-reset email for the given
	// address. err is domain.ErrUserNotFound if no account matches, or a
	// generic error otherwise.
	ForgotPassword(ctx context.Context, email string) error

	// ResetPassword sets a new password for userID (reached via the
	// password-reset email's link). err is a generic error; the source
	// handler does not distinguish specific downstream rejections here.
	ResetPassword(ctx context.Context, userID, password, confirmPassword string) error
}
