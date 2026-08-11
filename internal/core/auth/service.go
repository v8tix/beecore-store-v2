// Package auth is the login/signup/activation/password-reset use-case,
// ported from the login/signup/activate/forgottenPassword/passwordReset
// sections of cmd/web/handlers.go in the source repo (see
// port/service.Auth's doc comment for the full mapping). It owns every
// downstream call and business decision those handlers made; the
// infrastructure/api/web/handler/auth.go handler now only decodes/
// validates requests, calls one method here, and renders the result.
package auth

import (
	"context"
	"errors"
	"time"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/port/resource"
	"github.com/v8tix/beecore-store-v2/internal/core/port/service"
	"github.com/v8tix/beecore-store-v2/internal/token"
)

// Dependencies are authService's collaborators, all resolved once at
// composition-root time rather than read from *config.Cfg here — core
// stays free of any infra/config import.
type Dependencies struct {
	AuthRemote   resource.AuthRemote
	SessionStore resource.SessionStore

	// AddressRemote is used only for its HasAddresses method, to populate
	// domain.Session's UserShippingAddress field during Login — relocated
	// here from AuthRemote now that the Address vertical slice exists
	// (plan Task 8); see resource.AddressRemote's doc comment for why.
	AddressRemote resource.AddressRemote

	// AdminEmail/AdminPassword are the admin service account's own
	// credentials (app.Admin.Email/Password in the source repo),
	// used by ForgotPassword/ResetPassword to mint a token via
	// AuthRemote.FindTokenByEmailAndPassword rather than
	// AuthRemote.GetAdminToken's cached path — matching the source
	// handlers' (inconsistent, but preserved) behavior exactly.
	AdminEmail    string
	AdminPassword string

	// BuyerRoleID is the role ID assigned to every self-signup user
	// (app.UserRoles.StoreBuyerID in the source repo).
	BuyerRoleID string

	// AccessTokenDuration is how long a freshly issued user access token
	// is considered valid (JWT.GetDuration() in the source repo) —
	// resolved once at startup from config.Cfg.JWT.
	AccessTokenDuration time.Duration

	// SessionTTL is the Redis session record's lifetime (sessionTTL() in
	// the source repo's cmd/web/helpers.go) — resolved once at startup
	// from config.Cfg.Session.MaxAge.
	SessionTTL time.Duration
}

type authService struct {
	deps Dependencies
}

var _ service.Auth = (*authService)(nil)

func NewAuthService(d Dependencies) *authService {
	return &authService{deps: d}
}

// Login mirrors the login handler in the source repo: authenticate,
// resolve the DNI/address state that decides the post-login redirect,
// exchange for the user's own access+refresh token, assemble the session
// record, and persist it under a freshly generated opaque session ID.
//
// The source handler also looks up the user's basket at this point
// (FindBasketByUserID/FindBasket, to seed the cookie's basket-quantity
// display) — that's browser-cookie state (session.Values), not part of
// domain.Session, so it isn't done here: infrastructure/api/web/handler's
// AuthHandler.Login owns that step instead, via port/service.Basket (plan
// Task 13/14; see its doc comment). Login here only owns what belongs in
// the persisted session record.
func (s *authService) Login(ctx context.Context, email, password string) (string, domain.Session, domain.User, error) {
	at, err := s.deps.AuthRemote.GetAdminToken(ctx)
	if err != nil {
		return "", domain.Session{}, domain.User{}, err
	}

	user, err := s.deps.AuthRemote.FindUserByEmailAndPassword(ctx, email, password, at)
	if err != nil {
		return "", domain.Session{}, domain.User{}, err
	}

	var sess domain.Session
	sess.UserID = user.ID
	if user.DNI != "" {
		sess.UserDNI = user.DNI
	}

	if sess.UserID == "" {
		return "", domain.Session{}, domain.User{}, errors.New("login succeeded but no user ID was returned")
	}

	hasAddresses, firstAddressID, err := s.deps.AddressRemote.HasAddresses(ctx, user.ID, at)
	if err != nil {
		return "", domain.Session{}, domain.User{}, err
	}
	if hasAddresses {
		sess.UserShippingAddress = firstAddressID
	}

	accessToken, refreshToken, err := s.deps.AuthRemote.FindUserTokenByEmailAndPassword(ctx, email, password, at)
	if err != nil {
		return "", domain.Session{}, domain.User{}, err
	}

	sess.UserAccessToken = accessToken
	sess.UserAccessExpiry = time.Now().Add(s.deps.AccessTokenDuration)
	sess.RefreshToken = refreshToken

	// A response with no error but an empty token is just as unusable as
	// an outright error above — don't persist a session that looks
	// logged-in but can't authenticate any subsequent backend call.
	if sess.UserAccessToken == "" {
		return "", domain.Session{}, domain.User{}, errors.New("login succeeded but no user access token was returned")
	}

	sessionID, err := token.New()
	if err != nil {
		return "", domain.Session{}, domain.User{}, err
	}

	if err := s.deps.SessionStore.Save(ctx, sessionID, sess, s.deps.SessionTTL); err != nil {
		return "", domain.Session{}, domain.User{}, err
	}

	return sessionID, sess, user, nil
}

// Signup mirrors the signup handler: check the email isn't already
// registered, then register the new storefront buyer user.
func (s *authService) Signup(ctx context.Context, firstName, lastName, email, password, confirmPassword string) (string, error) {
	at, err := s.deps.AuthRemote.GetAdminToken(ctx)
	if err != nil {
		return "", err
	}

	_, err = s.deps.AuthRemote.FindUserByEmail(ctx, email, at)
	switch {
	case err == nil:
		return "", domain.ErrEmailInUse
	case errors.Is(err, domain.ErrUserNotFound):
		// Not registered yet — proceed.
	default:
		return "", err
	}

	userID, err := s.deps.AuthRemote.RegisterUser(ctx, firstName, lastName, email, password, confirmPassword, at, s.deps.BuyerRoleID)
	if err != nil {
		return "", err
	}

	return userID, nil
}

// Activate mirrors the activate handler.
func (s *authService) Activate(ctx context.Context, userID, activationCode string) error {
	at, err := s.deps.AuthRemote.GetAdminToken(ctx)
	if err != nil {
		return err
	}

	return s.deps.AuthRemote.ActivateUser(ctx, at, userID, activationCode)
}

// ForgotPassword mirrors the forgottenPassword handler: it fetches its own
// token via FindTokenByEmailAndPassword (not GetAdminToken's cached path —
// matches the source handler exactly), confirms the email is registered,
// then triggers the reset email.
func (s *authService) ForgotPassword(ctx context.Context, email string) error {
	at, err := s.deps.AuthRemote.FindTokenByEmailAndPassword(ctx, s.deps.AdminEmail, s.deps.AdminPassword, "")
	if err != nil {
		return err
	}

	_, err = s.deps.AuthRemote.FindUserByEmail(ctx, email, at)
	switch {
	case err == nil:
		// Registered — proceed.
	case errors.Is(err, domain.ErrUserNotFound):
		return domain.ErrUserNotFound
	default:
		return err
	}

	return s.deps.AuthRemote.SendResetPasswdEmail(ctx, email, at)
}

// ResetPassword mirrors the passwordReset handler.
func (s *authService) ResetPassword(ctx context.Context, userID, password, confirmPassword string) error {
	at, err := s.deps.AuthRemote.FindTokenByEmailAndPassword(ctx, s.deps.AdminEmail, s.deps.AdminPassword, "")
	if err != nil {
		return err
	}

	return s.deps.AuthRemote.UpdateUserPassword(ctx, userID, password, confirmPassword, at)
}
