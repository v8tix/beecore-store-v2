// Package confirm holds wire-message DTOs for confirming a PayPhone order transaction.
package confirm

type (
	// PayPhoneConfirmReq requests confirmation of a PayPhone transaction.
	PayPhoneConfirmReq struct {
		ID         int    `json:"id,omitempty"`
		ClientTxID string `json:"clientTxId,omitempty"`
	}

	// PayPhoneConfirmRes is PayPhone's transaction confirmation response.
	PayPhoneConfirmRes struct {
		Email               string  `json:"email,omitempty"`
		CardType            string  `json:"cardType,omitempty"`
		Bin                 string  `json:"bin,omitempty"`
		LastDigits          string  `json:"lastDigits,omitempty"`
		DeferredCode        string  `json:"deferredCode,omitempty"`
		DeferredMessage     string  `json:"deferredMessage,omitempty"`
		Deferred            bool    `json:"deferred,omitempty"`
		CardBrandCode       string  `json:"cardBrandCode,omitempty"`
		CardBrand           string  `json:"cardBrand,omitempty"`
		Amount              int     `json:"amount,omitempty"`
		ClientTransactionID string  `json:"clientTransactionId,omitempty"`
		PhoneNumber         string  `json:"phoneNumber,omitempty"`
		StatusCode          int     `json:"statusCode,omitempty"`
		TransactionStatus   string  `json:"transactionStatus,omitempty"`
		AuthorizationCode   string  `json:"authorizationCode,omitempty"`
		Message             *string `json:"message,omitempty"`
		MessageCode         int     `json:"messageCode,omitempty"`
		TransactionID       int     `json:"transactionId,omitempty"`
		Document            string  `json:"document,omitempty"`
		Taxes               []any   `json:"taxes,omitempty"`
		Currency            string  `json:"currency,omitempty"`
		OptionalParameter1  string  `json:"optionalParameter1,omitempty"`
		OptionalParameter2  string  `json:"optionalParameter2,omitempty"`
		OptionalParameter3  string  `json:"optionalParameter3,omitempty"`
		OptionalParameter4  string  `json:"optionalParameter4,omitempty"`
		StoreName           string  `json:"storeName,omitempty"`
		Date                string  `json:"date,omitempty"`
		RegionISO           string  `json:"regionIso,omitempty"`
		TransactionType     string  `json:"transactionType,omitempty"`
		Recap               *string `json:"recap,omitempty"`
		Reference           string  `json:"reference,omitempty"`
		Pan                 *string `json:"pan,omitempty"`
	}
)

// Req marks PayPhoneConfirmReq as a request payload.
func (p PayPhoneConfirmReq) Req() {}

// Res marks PayPhoneConfirmRes as a response payload.
func (p PayPhoneConfirmRes) Res() {}
