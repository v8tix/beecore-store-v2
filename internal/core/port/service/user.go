package service

import "context"

//go:generate mockery --name User

// User is the profile/wizard use-case, ported from the updateUserWizard
// section of cmd/web/handlers.go in the source repo. The other flows
// touching a user's own record — signup, activation — are Login/Signup/
// Activate's concern and already live behind port/service.Auth; this
// interface only covers what's left: the DNI/phone/birthday profile
// update reached via the post-login "/user/update" wizard step.
type User interface {
	// UpdateProfile mirrors updateUserWizard's POST handler: it fetches
	// its own admin token, then updates userID's DNI, birthday and phone
	// (formatted with the Ecuador country-code prefix the source handler
	// hardcodes onto every phone number) — firstName/lastName are passed
	// through unchanged from the caller (the source handler reuses the
	// already-authenticated user's own values), image URL and website
	// are left unset, matching the source call site.
	//
	// err is domain.ErrUserNotFound when the downstream user record is
	// gone — mirrors the source handler's 404 special-case, which treats
	// it as a silent no-op rather than a failure — or a generic error
	// otherwise.
	UpdateProfile(ctx context.Context, userID, firstName, lastName, dni, birthday, phone string) error
}
