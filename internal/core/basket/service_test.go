package basket_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/v8tix/beecore-store-v2/internal/core/basket"
	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/port/resource/mocks"
)

func newDeps(basketRemote *mocks.BasketRemote, authRemote *mocks.AuthRemote) basket.Dependencies {
	return basket.Dependencies{
		BasketRemote: basketRemote,
		AuthRemote:   authRemote,
	}
}

func TestComputeFinancials(t *testing.T) {
	ar := mocks.NewAuthRemote(t)
	ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)

	br := mocks.NewBasketRemote(t)
	want := domain.Financials{Total: 11.5}
	br.On("ComputeFinancials", mock.Anything, "b1", "admin-tok").Return(want, nil)

	svc := basket.NewBasketService(newDeps(br, ar))

	got, err := svc.ComputeFinancials(t.Context(), "b1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Total != want.Total {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestComputeFinancials_AdminTokenFailurePropagates(t *testing.T) {
	ar := mocks.NewAuthRemote(t)
	ar.On("GetAdminToken", mock.Anything).Return("", errors.New("boom"))

	br := mocks.NewBasketRemote(t)

	svc := basket.NewBasketService(newDeps(br, ar))

	_, err := svc.ComputeFinancials(t.Context(), "b1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAddItem(t *testing.T) {
	ar := mocks.NewAuthRemote(t)
	ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)

	br := mocks.NewBasketRemote(t)
	br.On("BasketAddItem", mock.Anything, "b1", "prod-1", 1, "admin-tok").Return(nil)
	want := domain.Basket{ID: "b1", Items: []domain.BasketItem{{Product: domain.BasketProduct{ID: "prod-1", Quantity: 1}}}}
	br.On("FindBasket", mock.Anything, "b1", "admin-tok").Return(want, nil)

	svc := basket.NewBasketService(newDeps(br, ar))

	got, err := svc.AddItem(t.Context(), "b1", "prod-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != want.ID || len(got.Items) != 1 {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestAddItem_AddFailurePropagatesWithoutFindBasket(t *testing.T) {
	ar := mocks.NewAuthRemote(t)
	ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)

	br := mocks.NewBasketRemote(t)
	br.On("BasketAddItem", mock.Anything, "b1", "prod-1", 1, "admin-tok").Return(errors.New("boom"))

	svc := basket.NewBasketService(newDeps(br, ar))

	_, err := svc.AddItem(t.Context(), "b1", "prod-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	br.AssertNotCalled(t, "FindBasket", mock.Anything, mock.Anything, mock.Anything)
}

func TestRemoveItem(t *testing.T) {
	ar := mocks.NewAuthRemote(t)
	ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)

	br := mocks.NewBasketRemote(t)
	br.On("BasketRemoveItem", mock.Anything, "b1", "prod-1", 1, "admin-tok").Return(nil)
	want := domain.Basket{ID: "b1"}
	br.On("FindBasket", mock.Anything, "b1", "admin-tok").Return(want, nil)

	svc := basket.NewBasketService(newDeps(br, ar))

	got, err := svc.RemoveItem(t.Context(), "b1", "prod-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != want.ID {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestAddItemFromSearch_NoConflict(t *testing.T) {
	ar := mocks.NewAuthRemote(t)
	ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)

	br := mocks.NewBasketRemote(t)
	br.On("BasketAddItem", mock.Anything, "b1", "prod-1", 1, "admin-tok").Return(nil)
	want := domain.Basket{ID: "b1"}
	br.On("FindBasket", mock.Anything, "b1", "admin-tok").Return(want, nil)

	svc := basket.NewBasketService(newDeps(br, ar))

	newID, got, err := svc.AddItemFromSearch(t.Context(), "b1", "prod-1", "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newID != "b1" {
		t.Fatalf("got basketID %q, want %q", newID, "b1")
	}
	if got.ID != want.ID {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	br.AssertNotCalled(t, "CreateBasket", mock.Anything, mock.Anything, mock.Anything)
}

// TestAddItemFromSearch_ConflictStartsFreshBasket proves the source repo's
// searchAddItem conflict-retry behavior: a domain.ErrBasketCheckedOut from
// the first BasketAddItem call triggers CreateBasket, then the add is
// retried against the new basket.
func TestAddItemFromSearch_ConflictStartsFreshBasket(t *testing.T) {
	ar := mocks.NewAuthRemote(t)
	ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)

	br := mocks.NewBasketRemote(t)
	br.On("BasketAddItem", mock.Anything, "stale-basket", "prod-1", 1, "admin-tok").
		Return(domain.ErrBasketCheckedOut).Once()
	br.On("CreateBasket", mock.Anything, "u1", "admin-tok").Return("fresh-basket", nil)
	br.On("BasketAddItem", mock.Anything, "fresh-basket", "prod-1", 1, "admin-tok").Return(nil).Once()
	want := domain.Basket{ID: "fresh-basket"}
	br.On("FindBasket", mock.Anything, "fresh-basket", "admin-tok").Return(want, nil)

	svc := basket.NewBasketService(newDeps(br, ar))

	newID, got, err := svc.AddItemFromSearch(t.Context(), "stale-basket", "prod-1", "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newID != "fresh-basket" {
		t.Fatalf("got basketID %q, want %q", newID, "fresh-basket")
	}
	if got.ID != want.ID {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestAddItemFromSearch_CreateBasketFailurePropagates(t *testing.T) {
	ar := mocks.NewAuthRemote(t)
	ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)

	br := mocks.NewBasketRemote(t)
	br.On("BasketAddItem", mock.Anything, "stale-basket", "prod-1", 1, "admin-tok").
		Return(domain.ErrBasketCheckedOut)
	br.On("CreateBasket", mock.Anything, "u1", "admin-tok").Return("", errors.New("boom"))

	svc := basket.NewBasketService(newDeps(br, ar))

	_, _, err := svc.AddItemFromSearch(t.Context(), "stale-basket", "prod-1", "u1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFindByUserID(t *testing.T) {
	ar := mocks.NewAuthRemote(t)
	ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)

	br := mocks.NewBasketRemote(t)
	br.On("FindBasketByUserID", mock.Anything, "u1", "admin-tok").Return("b1", nil)

	svc := basket.NewBasketService(newDeps(br, ar))

	id, err := svc.FindByUserID(t.Context(), "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "b1" {
		t.Fatalf("got %q, want %q", id, "b1")
	}
}

func TestCreate(t *testing.T) {
	ar := mocks.NewAuthRemote(t)
	ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)

	br := mocks.NewBasketRemote(t)
	br.On("CreateBasket", mock.Anything, "u1", "admin-tok").Return("b1", nil)

	svc := basket.NewBasketService(newDeps(br, ar))

	id, err := svc.Create(t.Context(), "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "b1" {
		t.Fatalf("got %q, want %q", id, "b1")
	}
}

func TestFindByID(t *testing.T) {
	ar := mocks.NewAuthRemote(t)
	ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)

	br := mocks.NewBasketRemote(t)
	want := domain.Basket{ID: "b1"}
	br.On("FindBasket", mock.Anything, "b1", "admin-tok").Return(want, nil)

	svc := basket.NewBasketService(newDeps(br, ar))

	got, err := svc.FindByID(t.Context(), "b1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != want.ID {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}
