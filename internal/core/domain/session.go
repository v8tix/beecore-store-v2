package domain

import "time"

// Session is the full server-side session record for a logged-in
// storefront user, as held by internal/session.Blob in the source repo.
// The browser cookie holds only an opaque session ID (see internal/token
// for its generation); everything below lives in Redis, addressed by that
// ID.
//
//   - UserAccessToken is the real logged-in user's own signed JWT,
//     obtained via beecore-customers' /auth/token with the user's actual
//     credentials. It never reaches the browser cookie.
//   - The admin service-account token used for privileged downstream
//     calls is deliberately not part of this per-session record — it is
//     one shared credential for the whole app, not per-user state;
//     callers fetch it fresh (and cached) via resource.AuthRemote's
//     GetAdminToken instead.
//   - Basket/payment/pagination/search session state deliberately stays
//     in the browser cookie's session.Values, unchanged — it isn't
//     sensitive, and moving it server-side wasn't part of this
//     migration's scope (mirrors the source repo's own Decision 2).
type Session struct {
	UserAccessToken     string    `json:"user_access_token"`
	UserAccessExpiry    time.Time `json:"user_access_expiry"`
	RefreshToken        string    `json:"refresh_token"`
	UserID              string    `json:"user_id"`
	UserDNI             string    `json:"user_dni,omitempty"`
	UserShippingAddress string    `json:"user_shipping_address,omitempty"`
}
