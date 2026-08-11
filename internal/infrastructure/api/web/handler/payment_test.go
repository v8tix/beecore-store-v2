// Package handler (internal test package, not handler_test) — reuses
// basket_test.go's basketCookieRequest/newTestCookieStore/discardTestLogger
// helpers, same as basket_test.go/user_test.go/order_test.go/address_test.go.
package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gorilla/sessions"
	"github.com/stretchr/testify/mock"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/port/resource/mocks"
	servicemocks "github.com/v8tix/beecore-store-v2/internal/core/port/service/mocks"
	"github.com/v8tix/beecore-store-v2/internal/foundation/keys"
)

func newPaymentHandler(t *testing.T, paymentService *servicemocks.Payment, basketService *servicemocks.Basket, store *sessions.CookieStore) *PaymentHandler {
	return NewPaymentHandler(paymentService, basketService, servicemocks.NewAddress(t), mocks.NewSessionStore(t), "USD", 12, store, discardTestLogger())
}

func newCheckoutHandler(
	t *testing.T,
	paymentService *servicemocks.Payment,
	basketService *servicemocks.Basket,
	addressService *servicemocks.Address,
	sessionStore *mocks.SessionStore,
	store *sessions.CookieStore,
) *PaymentHandler {
	return NewPaymentHandler(paymentService, basketService, addressService, sessionStore, "USD", 12, store, discardTestLogger())
}

func TestPaymentHandler_ProcessPayment(t *testing.T) {
	store := newTestCookieStore()

	t.Run("GET is not allowed", func(t *testing.T) {
		h := newPaymentHandler(t, servicemocks.NewPayment(t), servicemocks.NewBasket(t), store)

		w := httptest.NewRecorder()
		h.ProcessPayment(w, httptest.NewRequest(http.MethodGet, "/payphone/process", nil))

		if w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusMethodNotAllowed)
		}
	})

	t.Run("no basket ID in cookie is a 500", func(t *testing.T) {
		h := newPaymentHandler(t, servicemocks.NewPayment(t), servicemocks.NewBasket(t), store)

		r := basketCookieRequest(t, store, http.MethodPost, "/payphone/process", nil, map[any]any{})
		w := httptest.NewRecorder()
		h.ProcessPayment(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("financials failure is a 500", func(t *testing.T) {
		basketService := servicemocks.NewBasket(t)
		basketService.On("ComputeFinancials", mock.Anything, "b1").Return(domain.Financials{}, errors.New("boom"))

		h := newPaymentHandler(t, servicemocks.NewPayment(t), basketService, store)

		r := basketCookieRequest(t, store, http.MethodPost, "/payphone/process", nil, map[any]any{keys.SessionBasketID: "b1"})
		w := httptest.NewRecorder()
		h.ProcessPayment(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("success computes financials, calls PaymentService and responds with JSON", func(t *testing.T) {
		basketService := servicemocks.NewBasket(t)
		financials := domain.Financials{Total: 23}
		basketService.On("ComputeFinancials", mock.Anything, "b1").Return(financials, nil)

		paymentService := servicemocks.NewPayment(t)
		paymentService.On("ProcessPayment", mock.Anything, mock.MatchedBy(func(req domain.PaymentRequest) bool {
			return req.Financials.Total == 23 && req.OrderID != "" && req.ClientTransactionID != "" && req.OrderID != req.ClientTransactionID
		})).Return(domain.PaymentResult{PaymentID: "ctx-1", OrderID: "o1", Status: "CREATED", ConfirmURL: "https://x/payphone/confirm"}, nil)

		h := newPaymentHandler(t, paymentService, basketService, store)

		r := basketCookieRequest(t, store, http.MethodPost, "/payphone/process", nil, map[any]any{keys.SessionBasketID: "b1"})
		w := httptest.NewRecorder()
		h.ProcessPayment(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("got status %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
		}
		if got := w.Header().Get("Content-Type"); got != "application/json" {
			t.Fatalf("got Content-Type %q, want application/json", got)
		}
		if len(w.Result().Cookies()) == 0 {
			t.Fatal("expected the client-transaction-ID/payment-ID cookie to be saved")
		}
	})

	t.Run("PaymentService failure is a 500", func(t *testing.T) {
		basketService := servicemocks.NewBasket(t)
		basketService.On("ComputeFinancials", mock.Anything, "b1").Return(domain.Financials{Total: 23}, nil)

		paymentService := servicemocks.NewPayment(t)
		paymentService.On("ProcessPayment", mock.Anything, mock.Anything).Return(domain.PaymentResult{}, errors.New("boom"))

		h := newPaymentHandler(t, paymentService, basketService, store)

		r := basketCookieRequest(t, store, http.MethodPost, "/payphone/process", nil, map[any]any{keys.SessionBasketID: "b1"})
		w := httptest.NewRecorder()
		h.ProcessPayment(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestPaymentHandler_ConfirmPayment(t *testing.T) {
	store := newTestCookieStore()

	t.Run("approved confirmation redirects to /orders/confirm", func(t *testing.T) {
		paymentService := servicemocks.NewPayment(t)
		paymentService.On("ConfirmPayment", mock.Anything, "123", "ctx-1").
			Return(domain.PaymentConfirmation{TransactionID: "123", Status: "Approved"}, nil)

		h := newPaymentHandler(t, paymentService, servicemocks.NewBasket(t), store)

		r := basketCookieRequest(t, store, http.MethodGet, "/payphone/confirm?id=123&clientTransactionId=ctx-1", nil, map[any]any{})
		w := httptest.NewRecorder()
		h.ConfirmPayment(w, r)

		if w.Code != http.StatusSeeOther {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusSeeOther)
		}
		if got := w.Header().Get("Location"); got != "/orders/confirm" {
			t.Fatalf("got redirect %q, want %q", got, "/orders/confirm")
		}
	})

	t.Run("PaymentService failure is a 500", func(t *testing.T) {
		paymentService := servicemocks.NewPayment(t)
		paymentService.On("ConfirmPayment", mock.Anything, "123", "ctx-1").
			Return(domain.PaymentConfirmation{}, errors.New("boom"))

		h := newPaymentHandler(t, paymentService, servicemocks.NewBasket(t), store)

		r := basketCookieRequest(t, store, http.MethodGet, "/payphone/confirm?id=123&clientTransactionId=ctx-1", nil, map[any]any{})
		w := httptest.NewRecorder()
		h.ConfirmPayment(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("non-approved confirmation is a 500, not a redirect", func(t *testing.T) {
		paymentService := servicemocks.NewPayment(t)
		paymentService.On("ConfirmPayment", mock.Anything, "123", "ctx-1").
			Return(domain.PaymentConfirmation{TransactionID: "123", Status: "Rejected", ErrorMessage: "Payment was not approved"}, nil)

		h := newPaymentHandler(t, paymentService, servicemocks.NewBasket(t), store)

		r := basketCookieRequest(t, store, http.MethodGet, "/payphone/confirm?id=123&clientTransactionId=ctx-1", nil, map[any]any{})
		w := httptest.NewRecorder()
		h.ConfirmPayment(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestPaymentHandler_CancelPayment(t *testing.T) {
	store := newTestCookieStore()

	t.Run("success redirects home", func(t *testing.T) {
		paymentService := servicemocks.NewPayment(t)
		paymentService.On("CancelPayment", mock.Anything, "123", "ctx-1").Return(nil)

		h := newPaymentHandler(t, paymentService, servicemocks.NewBasket(t), store)

		r := basketCookieRequest(t, store, http.MethodGet, "/payphone/cancel?id=123&clientTransactionId=ctx-1", nil, map[any]any{})
		w := httptest.NewRecorder()
		h.CancelPayment(w, r)

		if w.Code != http.StatusSeeOther {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusSeeOther)
		}
		if got := w.Header().Get("Location"); got != "/" {
			t.Fatalf("got redirect %q, want %q", got, "/")
		}
	})

	t.Run("PaymentService failure is a 500", func(t *testing.T) {
		paymentService := servicemocks.NewPayment(t)
		paymentService.On("CancelPayment", mock.Anything, "123", "ctx-1").Return(errors.New("boom"))

		h := newPaymentHandler(t, paymentService, servicemocks.NewBasket(t), store)

		r := basketCookieRequest(t, store, http.MethodGet, "/payphone/cancel?id=123&clientTransactionId=ctx-1", nil, map[any]any{})
		w := httptest.NewRecorder()
		h.CancelPayment(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

// TestPaymentHandler_Checkout mirrors the checkoutPayPhone handler in the
// source repo — see PaymentHandler.Checkout's doc comment.
func TestPaymentHandler_Checkout(t *testing.T) {
	store := newTestCookieStore()

	t.Run("no user access token is a 500", func(t *testing.T) {
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Load", mock.Anything, "sess-1").Return(domain.Session{UserID: "u1"}, nil)

		h := newCheckoutHandler(t, servicemocks.NewPayment(t), servicemocks.NewBasket(t), servicemocks.NewAddress(t), sessionStore, store)

		r := basketCookieRequest(t, store, http.MethodGet, "/checkout", nil, map[any]any{keys.SessionID: "sess-1"})
		w := httptest.NewRecorder()
		h.Checkout(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("no basket ID in cookie is a 500", func(t *testing.T) {
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Load", mock.Anything, "sess-1").Return(domain.Session{UserID: "u1", UserAccessToken: "tok"}, nil)

		h := newCheckoutHandler(t, servicemocks.NewPayment(t), servicemocks.NewBasket(t), servicemocks.NewAddress(t), sessionStore, store)

		r := basketCookieRequest(t, store, http.MethodGet, "/checkout", nil, map[any]any{keys.SessionID: "sess-1"})
		w := httptest.NewRecorder()
		h.Checkout(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("GET renders the checkout page and generates the PayPhone config", func(t *testing.T) {
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Load", mock.Anything, "sess-1").Return(domain.Session{UserID: "u1", UserAccessToken: "tok"}, nil)

		addressService := servicemocks.NewAddress(t)
		addressService.On("FindByUserID", mock.Anything, "u1").Return([]domain.Address{{ID: "a1"}}, nil)

		basketService := servicemocks.NewBasket(t)
		financials := domain.Financials{
			Items:               []domain.FinancialLineItem{{Store: domain.Store{ID: "s1"}, Products: []domain.BasketProduct{{ID: "p1"}}}},
			Subtotal:            20,
			TotalTaxesAmount:    2,
			TotalShippingAmount: 1,
			Total:               23,
		}
		basketService.On("ComputeFinancials", mock.Anything, "b1").Return(financials, nil)

		paymentService := servicemocks.NewPayment(t)
		paymentService.On("GeneratePayPhoneConfigWithTransactionID", mock.Anything, mock.MatchedBy(func(req domain.PaymentRequest) bool {
			return req.UserID == "u1" && req.Currency == "USD" && req.Financials.Total == 23
		}), mock.MatchedBy(func(id string) bool { return id != "" })).Return(map[string]any{"token": "ph-token"}, nil)

		h := newCheckoutHandler(t, paymentService, basketService, addressService, sessionStore, store)

		r := basketCookieRequest(t, store, http.MethodGet, "/checkout", nil, map[any]any{keys.SessionID: "sess-1", keys.SessionBasketID: "b1"})
		w := httptest.NewRecorder()
		h.Checkout(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("got status %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
		}
		if len(w.Result().Cookies()) == 0 {
			t.Fatal("expected the client-transaction-ID cookie to be saved")
		}
	})

	t.Run("GET with an empty basket skips PayPhone config generation", func(t *testing.T) {
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Load", mock.Anything, "sess-1").Return(domain.Session{UserID: "u1", UserAccessToken: "tok"}, nil)

		addressService := servicemocks.NewAddress(t)
		addressService.On("FindByUserID", mock.Anything, "u1").Return(nil, nil)

		basketService := servicemocks.NewBasket(t)
		basketService.On("ComputeFinancials", mock.Anything, "b1").Return(domain.Financials{}, nil)

		paymentService := servicemocks.NewPayment(t)

		h := newCheckoutHandler(t, paymentService, basketService, addressService, sessionStore, store)

		r := basketCookieRequest(t, store, http.MethodGet, "/checkout", nil, map[any]any{keys.SessionID: "sess-1", keys.SessionBasketID: "b1"})
		w := httptest.NewRecorder()
		h.Checkout(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("got status %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
		}
		paymentService.AssertNotCalled(t, "GeneratePayPhoneConfigWithTransactionID", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("POST without a shipping address re-renders with a regenerated PayPhone config", func(t *testing.T) {
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Load", mock.Anything, "sess-1").Return(domain.Session{UserID: "u1", UserAccessToken: "tok"}, nil)

		addressService := servicemocks.NewAddress(t)
		addressService.On("FindByUserID", mock.Anything, "u1").Return([]domain.Address{{ID: "a1"}}, nil)

		basketService := servicemocks.NewBasket(t)
		financials := domain.Financials{
			Items: []domain.FinancialLineItem{{Products: []domain.BasketProduct{{ID: "p1", Quantity: 1}}}},
			Total: 23,
		}
		basketService.On("ComputeFinancials", mock.Anything, "b1").Return(financials, nil)

		paymentService := servicemocks.NewPayment(t)
		paymentService.On("GeneratePayPhoneConfig", mock.Anything, mock.MatchedBy(func(req domain.PaymentRequest) bool {
			return req.ClientTransactionID == "ctx-1"
		})).Return(map[string]any{"token": "ph-token"}, nil)

		h := newCheckoutHandler(t, paymentService, basketService, addressService, sessionStore, store)

		body := strings.NewReader(url.Values{"BasketItems": {"1"}}.Encode())
		r := basketCookieRequest(t, store, http.MethodPost, "/checkout", body, map[any]any{
			keys.SessionID:                  "sess-1",
			keys.SessionBasketID:            "b1",
			keys.SessionBasketItems:         1,
			keys.SessionClientTransactionID: "ctx-1",
		})
		w := httptest.NewRecorder()
		h.Checkout(w, r)

		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("got status %d, want %d: %s", w.Code, http.StatusUnprocessableEntity, w.Body.String())
		}
	})

	t.Run("POST success redirects to /checkout", func(t *testing.T) {
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Load", mock.Anything, "sess-1").Return(domain.Session{UserID: "u1", UserAccessToken: "tok"}, nil)

		addressService := servicemocks.NewAddress(t)
		addressService.On("FindByUserID", mock.Anything, "u1").Return([]domain.Address{{ID: "a1"}}, nil)

		basketService := servicemocks.NewBasket(t)
		financials := domain.Financials{
			Items: []domain.FinancialLineItem{{Products: []domain.BasketProduct{{ID: "p1", Quantity: 1}}}},
			Total: 23,
		}
		basketService.On("ComputeFinancials", mock.Anything, "b1").Return(financials, nil)

		paymentService := servicemocks.NewPayment(t)

		h := newCheckoutHandler(t, paymentService, basketService, addressService, sessionStore, store)

		body := strings.NewReader(url.Values{"ShippingAddress": {"a1"}, "BasketItems": {"1"}}.Encode())
		r := basketCookieRequest(t, store, http.MethodPost, "/checkout", body, map[any]any{
			keys.SessionID:                  "sess-1",
			keys.SessionBasketID:            "b1",
			keys.SessionBasketItems:         1,
			keys.SessionClientTransactionID: "ctx-1",
		})
		w := httptest.NewRecorder()
		h.Checkout(w, r)

		if w.Code != http.StatusSeeOther {
			t.Fatalf("got status %d, want %d: %s", w.Code, http.StatusSeeOther, w.Body.String())
		}
		if got := w.Header().Get("Location"); got != "/checkout" {
			t.Fatalf("got redirect %q, want %q", got, "/checkout")
		}
	})
}
