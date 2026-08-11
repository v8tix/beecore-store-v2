// Package user is the profile/wizard use-case, ported from the
// updateUserWizard section of cmd/web/handlers.go in the source repo (see
// port/service.User's doc comment for the full mapping, and
// port/resource.UserRemote's doc comment for why register/activate and
// the auth-middleware-only FindUserByID method aren't this slice's
// concern). It owns the one downstream call and business decision that
// handler made; infrastructure/api/web/handler/user.go now only decodes/
// validates the request, loads/saves the session record, calls this
// use-case, and renders the result.
package user

import (
	"context"
	"fmt"

	"github.com/v8tix/beecore-store-v2/internal/core/port/resource"
	"github.com/v8tix/beecore-store-v2/internal/core/port/service"
)

// phoneCountryCode is the Ecuador country-code prefix the source
// updateUserWizard handler hardcodes onto every phone number before
// sending it downstream (fmt.Sprintf("+593%s", form.PhoneNumber) in
// cmd/web/handlers.go) — preserved as-is per the ticket's 1:1 port rule;
// simplification/configuration is a later phase.
const phoneCountryCode = "+593"

// Dependencies are userService's collaborators, all resolved once at
// composition-root time rather than read from *config.Cfg here — core
// stays free of any infra/config import.
type Dependencies struct {
	UserRemote resource.UserRemote

	// AuthRemote is used only for its GetAdminToken method — reused from
	// the auth slice's resource.AuthRemote rather than duplicated here,
	// matching the "reuse, don't redesign" rule for gaps already covered
	// by an existing port.
	AuthRemote resource.AuthRemote
}

type userService struct {
	deps Dependencies
}

var _ service.User = (*userService)(nil)

func NewUserService(d Dependencies) *userService {
	return &userService{deps: d}
}

// UpdateProfile mirrors updateUserWizard's POST handler.
func (s *userService) UpdateProfile(ctx context.Context, userID, firstName, lastName, dni, birthday, phone string) error {
	at, err := s.deps.AuthRemote.GetAdminToken(ctx)
	if err != nil {
		return err
	}

	return s.deps.UserRemote.UpdateUser(
		ctx,
		userID,
		dni,
		firstName,
		lastName,
		birthday,
		fmt.Sprintf("%s%s", phoneCountryCode, phone),
		"",
		"",
		at,
	)
}
