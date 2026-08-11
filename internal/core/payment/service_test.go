package payment_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/payment"
	"github.com/v8tix/beecore-store-v2/internal/core/port/resource/mocks"
)

func newDeps(payPhoneRemote *mocks.PayPhoneRemote) payment.Dependencies {
	return payment.Dependencies{PayPhoneRemote: payPhoneRemote}
}

func newDepsWithAuth(payPhoneRemote *mocks.PayPhoneRemote, authRemote *mocks.AuthRemote) payment.Dependencies {
	return payment.Dependencies{PayPhoneRemote: payPhoneRemote, AuthRemote: authRemote}
}

func TestProcessPayment(t *testing.T) {
	pr := mocks.NewPayPhoneRemote(t)
	req := domain.PaymentRequest{OrderID: "o1", ClientTransactionID: "ctx-1"}
	want := domain.PaymentResult{PaymentID: "ctx-1", OrderID: "o1", Status: "CREATED"}
	pr.On("ProcessPayment", mock.Anything, req).Return(want, nil)

	svc := payment.NewPaymentService(newDeps(pr))

	got, err := svc.ProcessPayment(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestProcessPayment_RemoteFailurePropagates(t *testing.T) {
	pr := mocks.NewPayPhoneRemote(t)
	req := domain.PaymentRequest{OrderID: "o1", ClientTransactionID: "ctx-1"}
	pr.On("ProcessPayment", mock.Anything, req).Return(domain.PaymentResult{}, errors.New("boom"))

	svc := payment.NewPaymentService(newDeps(pr))

	_, err := svc.ProcessPayment(t.Context(), req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestConfirmPayment(t *testing.T) {
	pr := mocks.NewPayPhoneRemote(t)
	want := domain.PaymentConfirmation{TransactionID: "123", PaymentID: "ctx-1", Status: "Approved"}
	pr.On("ConfirmPayment", mock.Anything, "123", "ctx-1").Return(want, nil)

	svc := payment.NewPaymentService(newDeps(pr))

	got, err := svc.ConfirmPayment(t.Context(), "123", "ctx-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestConfirmPayment_RemoteFailurePropagates(t *testing.T) {
	pr := mocks.NewPayPhoneRemote(t)
	pr.On("ConfirmPayment", mock.Anything, "123", "ctx-1").Return(domain.PaymentConfirmation{}, errors.New("boom"))

	svc := payment.NewPaymentService(newDeps(pr))

	_, err := svc.ConfirmPayment(t.Context(), "123", "ctx-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCancelPayment(t *testing.T) {
	pr := mocks.NewPayPhoneRemote(t)
	pr.On("CancelPayment", mock.Anything, "123", "ctx-1").Return(nil)

	svc := payment.NewPaymentService(newDeps(pr))

	if err := svc.CancelPayment(t.Context(), "123", "ctx-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCancelPayment_RemoteFailurePropagates(t *testing.T) {
	pr := mocks.NewPayPhoneRemote(t)
	pr.On("CancelPayment", mock.Anything, "123", "ctx-1").Return(errors.New("boom"))

	svc := payment.NewPaymentService(newDeps(pr))

	if err := svc.CancelPayment(t.Context(), "123", "ctx-1"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGeneratePayPhoneConfigWithTransactionID(t *testing.T) {
	pr := mocks.NewPayPhoneRemote(t)
	ar := mocks.NewAuthRemote(t)

	req := domain.PaymentRequest{UserID: "u1"}
	user := domain.User{ID: "u1", Phone: "0999999999"}
	want := map[string]any{"token": "ph-token"}

	ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
	ar.On("FindUserByID", mock.Anything, "u1", "admin-tok").Return(user, nil)
	pr.On("GeneratePayPhoneConfig", req, "ctx-1", user).Return(want)

	svc := payment.NewPaymentService(newDepsWithAuth(pr, ar))

	got, err := svc.GeneratePayPhoneConfigWithTransactionID(t.Context(), req, "ctx-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestGeneratePayPhoneConfigWithTransactionID_AdminTokenFailurePropagates(t *testing.T) {
	pr := mocks.NewPayPhoneRemote(t)
	ar := mocks.NewAuthRemote(t)

	ar.On("GetAdminToken", mock.Anything).Return("", errors.New("boom"))

	svc := payment.NewPaymentService(newDepsWithAuth(pr, ar))

	_, err := svc.GeneratePayPhoneConfigWithTransactionID(t.Context(), domain.PaymentRequest{UserID: "u1"}, "ctx-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGeneratePayPhoneConfigWithTransactionID_UserLookupFailurePropagates(t *testing.T) {
	pr := mocks.NewPayPhoneRemote(t)
	ar := mocks.NewAuthRemote(t)

	ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
	ar.On("FindUserByID", mock.Anything, "u1", "admin-tok").Return(domain.User{}, errors.New("boom"))

	svc := payment.NewPaymentService(newDepsWithAuth(pr, ar))

	_, err := svc.GeneratePayPhoneConfigWithTransactionID(t.Context(), domain.PaymentRequest{UserID: "u1"}, "ctx-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGeneratePayPhoneConfig_MintsTransactionIDWhenEmpty(t *testing.T) {
	pr := mocks.NewPayPhoneRemote(t)
	ar := mocks.NewAuthRemote(t)

	req := domain.PaymentRequest{UserID: "u1"}
	user := domain.User{ID: "u1"}

	ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
	ar.On("FindUserByID", mock.Anything, "u1", "admin-tok").Return(user, nil)
	pr.On("GeneratePayPhoneConfig", req, mock.MatchedBy(func(id string) bool { return id != "" }), user).
		Return(map[string]any{"token": "ph-token"})

	svc := payment.NewPaymentService(newDepsWithAuth(pr, ar))

	got, err := svc.GeneratePayPhoneConfig(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["token"] != "ph-token" {
		t.Fatalf("got %+v", got)
	}
}

func TestGeneratePayPhoneConfig_PreservesExistingTransactionID(t *testing.T) {
	pr := mocks.NewPayPhoneRemote(t)
	ar := mocks.NewAuthRemote(t)

	req := domain.PaymentRequest{UserID: "u1", ClientTransactionID: "ctx-existing"}
	user := domain.User{ID: "u1"}

	ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
	ar.On("FindUserByID", mock.Anything, "u1", "admin-tok").Return(user, nil)
	pr.On("GeneratePayPhoneConfig", req, "ctx-existing", user).Return(map[string]any{"token": "ph-token"})

	svc := payment.NewPaymentService(newDepsWithAuth(pr, ar))

	if _, err := svc.GeneratePayPhoneConfig(t.Context(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
