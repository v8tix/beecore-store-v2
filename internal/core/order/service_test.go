package order_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/order"
	"github.com/v8tix/beecore-store-v2/internal/core/port/resource/mocks"
)

func newDeps(orderRemote *mocks.OrderRemote, authRemote *mocks.AuthRemote) order.Dependencies {
	return order.Dependencies{
		OrderRemote: orderRemote,
		AuthRemote:  authRemote,
	}
}

func TestGetByUserID(t *testing.T) {
	ar := mocks.NewAuthRemote(t)
	ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)

	or := mocks.NewOrderRemote(t)
	want := []domain.Order{{ID: "o1", UserID: "u1", Status: "completed"}}
	or.On("GetOrdersByUserID", mock.Anything, "u1", "admin-tok").Return(want, nil)

	svc := order.NewOrderService(newDeps(or, ar))

	got, err := svc.GetByUserID(t.Context(), "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != want[0].ID {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestGetByUserID_AdminTokenFailurePropagates(t *testing.T) {
	ar := mocks.NewAuthRemote(t)
	ar.On("GetAdminToken", mock.Anything).Return("", errors.New("boom"))

	or := mocks.NewOrderRemote(t)

	svc := order.NewOrderService(newDeps(or, ar))

	_, err := svc.GetByUserID(t.Context(), "u1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	or.AssertNotCalled(t, "GetOrdersByUserID", mock.Anything, mock.Anything, mock.Anything)
}

func TestGetByUserID_OrderRemoteFailurePropagates(t *testing.T) {
	ar := mocks.NewAuthRemote(t)
	ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)

	or := mocks.NewOrderRemote(t)
	or.On("GetOrdersByUserID", mock.Anything, "u1", "admin-tok").Return(nil, errors.New("boom"))

	svc := order.NewOrderService(newDeps(or, ar))

	_, err := svc.GetByUserID(t.Context(), "u1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
