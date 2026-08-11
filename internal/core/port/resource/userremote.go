package resource

import "context"

//go:generate mockery --name UserRemote

// UserRemote is the outbound HTTP boundary for the user profile/wizard
// use-case, ported from the UpdateUser method of
// internal/business/core/repository/user_repository.go in the source
// repo — the one user_repository.go method the updateUserWizard handler
// calls that isn't already covered by resource.AuthRemote. AuthRemote
// already owns every other user_repository.go method the auth slice's
// handlers touch: FindUserByEmail, FindUserByEmailAndPassword,
// FindUserTokenByEmailAndPassword, RegisterUser, SendResetPasswdEmail,
// ActivateUser, UpdateUserPassword, RefreshUserToken (see its doc
// comment).
//
// user_repository.go also defines FindUserByID, used by the source repo's
// auth middleware (cmd/web/middleware.go's authenticate/
// requireAuthenticatedUser) to resolve the currently logged-in user on
// every request — not by any user-profile handler. It isn't ported here:
// this repo's session-resolution middleware doesn't exist yet
// (composition-root wiring, plan Task 19); when it lands, FindUserByID
// most naturally belongs on resource.AuthRemote instead (an auth "who's
// making the request" concern, not a profile-management one), matching
// the precedent set by HasAddresses relocating onto its true owner
// (resource.AddressRemote — see that file's doc comment) rather than
// staying wherever the source repo happened to define it.
type UserRemote interface {
	// UpdateUser updates a user's DNI, name, birthday, phone, image URL
	// and website. err is domain.ErrUserNotFound on a downstream 404 —
	// the source updateUserWizard handler treats that case as a silent
	// no-op rather than a failure; this adapter surfaces it as a
	// sentinel so the use-case/handler can replicate that decision
	// without needing to know kawa exists. Mirrors
	// BaseRepositoryImpl.UpdateUser.
	UpdateUser(ctx context.Context, id, dni, firstName, lastName, birthday, phone, imageURL, website, token string) error
}
