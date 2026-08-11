package product_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/port/resource/mocks"
	"github.com/v8tix/beecore-store-v2/internal/core/product"
)

func newDeps(productRemote *mocks.ProductRemote, authRemote *mocks.AuthRemote) product.Dependencies {
	return product.Dependencies{
		ProductRemote: productRemote,
		AuthRemote:    authRemote,
	}
}

func TestFindByID(t *testing.T) {
	ar := mocks.NewAuthRemote(t)
	ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)

	pr := mocks.NewProductRemote(t)
	want := domain.Product{ID: "p1", Name: "Widget"}
	pr.On("FindProduct", mock.Anything, "p1", "admin-tok").Return(want, nil)

	svc := product.NewProductService(newDeps(pr, ar))

	got, err := svc.FindByID(t.Context(), "p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestFindByID_AdminTokenFailurePropagates(t *testing.T) {
	ar := mocks.NewAuthRemote(t)
	ar.On("GetAdminToken", mock.Anything).Return("", errors.New("boom"))

	pr := mocks.NewProductRemote(t)

	svc := product.NewProductService(newDeps(pr, ar))

	_, err := svc.FindByID(t.Context(), "p1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSearch(t *testing.T) {
	ar := mocks.NewAuthRemote(t)
	ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)

	pr := mocks.NewProductRemote(t)
	want := domain.ProductPage{
		Products: []domain.Product{{ID: "p1"}, {ID: "p2"}},
		Total:    2,
	}
	pr.On("FindPaginatedProducts", mock.Anything, "widget", int64(0), "admin-tok").Return(want, nil)

	svc := product.NewProductService(newDeps(pr, ar))

	got, err := svc.Search(t.Context(), "widget", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Total != 2 || len(got.Products) != 2 {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestSearch_AdminTokenFailurePropagates(t *testing.T) {
	ar := mocks.NewAuthRemote(t)
	ar.On("GetAdminToken", mock.Anything).Return("", errors.New("boom"))

	pr := mocks.NewProductRemote(t)

	svc := product.NewProductService(newDeps(pr, ar))

	_, err := svc.Search(t.Context(), "widget", 0)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
