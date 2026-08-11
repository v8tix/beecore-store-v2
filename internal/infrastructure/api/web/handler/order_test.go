// Package handler (internal test package, not handler_test) — Orders
// needs the unexported currentSession helper, so these tests live inside
// the package alongside basket_test.go's/user_test.go's/address_test.go's.
package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/sessions"
	"github.com/stretchr/testify/mock"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/port/resource/mocks"
	servicemocks "github.com/v8tix/beecore-store-v2/internal/core/port/service/mocks"
)

func newOrderHandler(orderService *servicemocks.Order, sessionStore *mocks.SessionStore, store *sessions.CookieStore) *OrderHandler {
	return NewOrderHandler(orderService, sessionStore, store, discardTestLogger())
}

func TestOrderHandler_ConfirmOrder(t *testing.T) {
	h := newOrderHandler(servicemocks.NewOrder(t), mocks.NewSessionStore(t), newTestCookieStore())

	w := httptest.NewRecorder()
	h.ConfirmOrder(w, httptest.NewRequest(http.MethodGet, "/orders/confirm", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusOK)
	}
}

func TestOrderHandler_CancelOrder(t *testing.T) {
	h := newOrderHandler(servicemocks.NewOrder(t), mocks.NewSessionStore(t), newTestCookieStore())

	w := httptest.NewRecorder()
	h.CancelOrder(w, httptest.NewRequest(http.MethodGet, "/orders/cancel", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusOK)
	}
}

func TestOrderHandler_Orders(t *testing.T) {
	store := newTestCookieStore()

	t.Run("no session is a 500", func(t *testing.T) {
		h := newOrderHandler(servicemocks.NewOrder(t), mocks.NewSessionStore(t), store)

		w := httptest.NewRecorder()
		h.Orders(w, httptest.NewRequest(http.MethodGet, "/orders", nil))

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("GET renders the order history page", func(t *testing.T) {
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Load", mock.Anything, "sess-1").Return(domain.Session{UserID: "u1"}, nil)

		orderService := servicemocks.NewOrder(t)
		updatedAt := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
		orderService.On("GetByUserID", mock.Anything, "u1").Return([]domain.Order{
			{
				ID:        "o1",
				PaymentID: "p1",
				Status:    "completed",
				UpdatedAt: updatedAt,
				FinancialItem: domain.FinancialLineItem{
					Total:    11.5,
					Products: []domain.BasketProduct{{ID: "prod-1", Quantity: 2}, {ID: "prod-2", Quantity: 1}},
				},
			},
		}, nil)

		h := newOrderHandler(orderService, sessionStore, store)

		r := sessionCookieRequest(t, store, http.MethodGet, "/orders", nil, "sess-1")
		w := httptest.NewRecorder()
		h.Orders(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("service failure is a 500", func(t *testing.T) {
		sessionStore := mocks.NewSessionStore(t)
		sessionStore.On("Load", mock.Anything, "sess-1").Return(domain.Session{UserID: "u1"}, nil)

		orderService := servicemocks.NewOrder(t)
		orderService.On("GetByUserID", mock.Anything, "u1").Return(nil, errors.New("boom"))

		h := newOrderHandler(orderService, sessionStore, store)

		r := sessionCookieRequest(t, store, http.MethodGet, "/orders", nil, "sess-1")
		w := httptest.NewRecorder()
		h.Orders(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got status %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

// TestOrderHandler_Orders_ItemCountAndAmount proves toOrderDisplay mirrors
// dto.toOrderDisplay in the source repo: Amount comes straight from
// FinancialItem.Total, and ItemCount sums every product line's Quantity —
// checked indirectly via a successful render (the page's own field access
// would panic/fail to compile a bad mapping, but the real proof is a unit
// test on the pure conversion helper itself, exercised directly here since
// it's unexported).
func TestOrderHandler_Orders_ItemCountAndAmount(t *testing.T) {
	updatedAt := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	got := toOrderDisplay(domain.Order{
		ID:        "o1",
		PaymentID: "p1",
		Status:    "completed",
		UpdatedAt: updatedAt,
		FinancialItem: domain.FinancialLineItem{
			Total:    11.5,
			Products: []domain.BasketProduct{{ID: "prod-1", Quantity: 2}, {ID: "prod-2", Quantity: 1}},
		},
	})

	if got.OrderID != "o1" || got.PaymentID != "p1" || got.Status != "completed" {
		t.Fatalf("unexpected display: %+v", got)
	}
	if got.Amount != 11.5 {
		t.Fatalf("got Amount %v, want %v", got.Amount, 11.5)
	}
	if got.ItemCount != 3 {
		t.Fatalf("got ItemCount %d, want %d", got.ItemCount, 3)
	}
	if got.UpdatedAt != "02/01/2026" {
		t.Fatalf("got UpdatedAt %q, want %q", got.UpdatedAt, "02/01/2026")
	}
}
