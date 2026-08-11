package keys

import "errors"

var (
	ErrAccessTokenRequired         = errors.New("access token is required")
	ErrSessionUserIDRequired       = errors.New("user ID is required")
	ErrSessionBasketIDRequired     = errors.New("basket ID is required")
	ErrUserAccessTokenRequired     = errors.New("user access token is required")
	ErrClientTransactionIDRequired = errors.New("client transaction ID not found in session")
)

const (
	Session = "session"
	// SessionID is the single key holding identity in the browser cookie: an
	// opaque, high-entropy ID pointing at the server-side session.Blob in
	// Redis. It replaces AccessToken, UserAccessToken, SessionUserID,
	// SessionUserDNI, and SessionUserShippingAddress — those now live in
	// session.Blob, never in the cookie. Every other key below is unrelated
	// to identity (basket/payment/pagination/search state) and is
	// deliberately left in the cookie's session.Values, unchanged.
	SessionID = "sessionID"

	SessionUserIN                    = "userIN"
	SessionShippingAddressID         = "shippingAddressID"
	SessionBasketItems               = "basketItems"
	SessionBasketID                  = "basketID"
	SessionPaymentID                 = "paymentID"
	SessionOrderID                   = "orderID"
	SessionClientTransactionID       = "clientTransactionID"
	SessionPayPhoneConfirmation      = "payphoneConfirmation"
	SessionUserInput                 = "userInput"
	SessionSelfPage                  = "selfPage"
	SessionNextPage                  = "nextPage"
	SessionPreviousPage              = "previousPage"
	PayPhoneClientTransactionIDParam = "clientTransactionId"
)
