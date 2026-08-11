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
