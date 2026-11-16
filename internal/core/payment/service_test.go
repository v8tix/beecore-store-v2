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

func newDepsFull(
	payPhoneRemote *mocks.PayPhoneRemote, authRemote *mocks.AuthRemote,
	orderRemote *mocks.OrderRemote, basketRemote *mocks.BasketRemote,
	paymentRecordRemote *mocks.PaymentRecordRemote,
) payment.Dependencies {
	return payment.Dependencies{
		PayPhoneRemote:      payPhoneRemote,
		AuthRemote:          authRemote,
		OrderRemote:         orderRemote,
		BasketRemote:        basketRemote,
		PaymentRecordRemote: paymentRecordRemote,
	}
}

func newDepsForProcessPayment(
	payPhoneRemote *mocks.PayPhoneRemote, authRemote *mocks.AuthRemote, paymentRecordRemote *mocks.PaymentRecordRemote,
) payment.Dependencies {
	return payment.Dependencies{
		PayPhoneRemote:      payPhoneRemote,
		AuthRemote:          authRemote,
		PaymentRecordRemote: paymentRecordRemote,
	}
}

func TestProcessPayment(t *testing.T) {
	pr := mocks.NewPayPhoneRemote(t)
	ar := mocks.NewAuthRemote(t)
	prr := mocks.NewPaymentRecordRemote(t)

	req := domain.PaymentRequest{
		OrderID: "o1", ClientTransactionID: "ctx-1", UserID: "user-1",
		Financials: domain.Financials{Total: 14.50},
	}
	want := domain.PaymentResult{PaymentID: "ctx-1", OrderID: "o1", Status: "CREATED"}
	pr.On("ProcessPayment", mock.Anything, req).Return(want, nil)
	ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
	prr.On("CreatePayment", mock.Anything, "ctx-1", "user-1", "payphone", "PENDING", mock.Anything, "", 14.50, "admin-tok").
		Return(nil)

	svc := payment.NewPaymentService(newDepsForProcessPayment(pr, ar, prr))

	got, err := svc.ProcessPayment(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestProcessPayment_CreatePaymentRecordFailurePropagates(t *testing.T) {
	pr := mocks.NewPayPhoneRemote(t)
	ar := mocks.NewAuthRemote(t)
	prr := mocks.NewPaymentRecordRemote(t)

	req := domain.PaymentRequest{OrderID: "o1", ClientTransactionID: "ctx-1", UserID: "user-1"}
	want := domain.PaymentResult{PaymentID: "ctx-1", OrderID: "o1", Status: "CREATED"}
	pr.On("ProcessPayment", mock.Anything, req).Return(want, nil)
	ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
	prr.On("CreatePayment", mock.Anything, "ctx-1", "user-1", "payphone", "PENDING", mock.Anything, "", float64(0), "admin-tok").
		Return(errors.New("boom"))

	svc := payment.NewPaymentService(newDepsForProcessPayment(pr, ar, prr))

	if _, err := svc.ProcessPayment(t.Context(), req); err == nil {
		t.Fatal("expected error, got nil")
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

func TestConfirmPayment_ApprovedCreatesOrdersChecksOutAndOpensFreshBasket(t *testing.T) {
	pr := mocks.NewPayPhoneRemote(t)
	ar := mocks.NewAuthRemote(t)
	or := mocks.NewOrderRemote(t)
	br := mocks.NewBasketRemote(t)
	prr := mocks.NewPaymentRecordRemote(t)

	want := domain.PaymentConfirmation{TransactionID: "123", PaymentID: "pay-1", Status: "Approved"}
	ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
	prr.On("FindPayment", mock.Anything, "pay-1", "admin-tok").
		Return(domain.Payment{Status: "PENDING", Request: "original-request"}, nil)
	pr.On("ConfirmPayment", mock.Anything, "123", "ctx-1").Return(want, nil)
	prr.On("UpdatePayment", mock.Anything, "pay-1", "CONFIRMED", "original-request", mock.Anything, "admin-tok").
		Return(nil)

	financials := domain.Financials{Items: []domain.FinancialLineItem{
		{Store: domain.Store{ID: "store-1"}, Total: 10},
		{Store: domain.Store{ID: "store-2"}, Total: 20},
	}}
	br.On("ComputeFinancials", mock.Anything, "basket-1", "admin-tok").Return(financials, nil)

	or.On("CreateOrder", mock.Anything, domain.CreateOrderRequest{
		UserID: "user-1", PaymentID: "pay-1", BasketID: "basket-1",
		ShippingAddressID: "addr-1", FinancialItem: financials.Items[0],
	}, "admin-tok").Return("order-1", nil)
	or.On("CreateOrder", mock.Anything, domain.CreateOrderRequest{
		UserID: "user-1", PaymentID: "pay-1", BasketID: "basket-1",
		ShippingAddressID: "addr-1", FinancialItem: financials.Items[1],
	}, "admin-tok").Return("order-2", nil)

	br.On("CheckoutBasket", mock.Anything, "basket-1", "pay-1", "addr-1", "admin-tok").Return(nil)
	br.On("CreateBasket", mock.Anything, "user-1", "admin-tok").Return("basket-2", nil)

	svc := payment.NewPaymentService(newDepsFull(pr, ar, or, br, prr))

	got, newBasketID, err := svc.ConfirmPayment(t.Context(), "123", "ctx-1", "user-1", "basket-1", "pay-1", "addr-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if newBasketID != "basket-2" {
		t.Fatalf("got newBasketID %q, want %q", newBasketID, "basket-2")
	}
}

func TestConfirmPayment_NotPendingRecordRejectedOutright(t *testing.T) {
	pr := mocks.NewPayPhoneRemote(t)
	ar := mocks.NewAuthRemote(t)
	or := mocks.NewOrderRemote(t)
	br := mocks.NewBasketRemote(t)
	prr := mocks.NewPaymentRecordRemote(t)

	ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
	prr.On("FindPayment", mock.Anything, "pay-1", "admin-tok").
		Return(domain.Payment{Status: "CONFIRMED"}, nil)
	// No PayPhoneRemote/OrderRemote/BasketRemote/UpdatePayment expectations
	// set — mockery fails the test if any of them get called, which is
	// exactly the assertion: a replayed/non-pending callback must be
	// rejected before even calling PayPhone again.

	svc := payment.NewPaymentService(newDepsFull(pr, ar, or, br, prr))

	_, newBasketID, err := svc.ConfirmPayment(t.Context(), "123", "ctx-1", "user-1", "basket-1", "pay-1", "addr-1")
	if !errors.Is(err, domain.ErrPaymentNotPending) {
		t.Fatalf("got err %v, want domain.ErrPaymentNotPending", err)
	}
	if newBasketID != "" {
		t.Fatalf("got newBasketID %q, want empty", newBasketID)
	}
}

func TestConfirmPayment_NotApprovedSkipsOrderAndBasketSideEffects(t *testing.T) {
	pr := mocks.NewPayPhoneRemote(t)
	ar := mocks.NewAuthRemote(t)
	or := mocks.NewOrderRemote(t)
	br := mocks.NewBasketRemote(t)
	prr := mocks.NewPaymentRecordRemote(t)

	want := domain.PaymentConfirmation{TransactionID: "123", ErrorMessage: "declined"}
	ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
	prr.On("FindPayment", mock.Anything, "pay-1", "admin-tok").
		Return(domain.Payment{Status: "PENDING", Request: "original-request"}, nil)
	pr.On("ConfirmPayment", mock.Anything, "123", "ctx-1").Return(want, nil)
	prr.On("UpdatePayment", mock.Anything, "pay-1", "FAILED", "original-request", mock.Anything, "admin-tok").
		Return(nil)
	// No OrderRemote/BasketRemote expectations set — mockery fails the
	// test if either gets called, which is exactly the assertion: a
	// non-approved confirmation must short-circuit before touching
	// orders/baskets at all (after still recording it as FAILED).

	svc := payment.NewPaymentService(newDepsFull(pr, ar, or, br, prr))

	got, newBasketID, err := svc.ConfirmPayment(t.Context(), "123", "ctx-1", "user-1", "basket-1", "pay-1", "addr-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if newBasketID != "" {
		t.Fatalf("got newBasketID %q, want empty", newBasketID)
	}
}

func TestConfirmPayment_RemoteFailurePropagates(t *testing.T) {
	pr := mocks.NewPayPhoneRemote(t)
	ar := mocks.NewAuthRemote(t)
	prr := mocks.NewPaymentRecordRemote(t)

	ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
	prr.On("FindPayment", mock.Anything, "pay-1", "admin-tok").
		Return(domain.Payment{Status: "PENDING"}, nil)
	pr.On("ConfirmPayment", mock.Anything, "123", "ctx-1").Return(domain.PaymentConfirmation{}, errors.New("boom"))

	svc := payment.NewPaymentService(newDepsForProcessPayment(pr, ar, prr))

	_, _, err := svc.ConfirmPayment(t.Context(), "123", "ctx-1", "user-1", "basket-1", "pay-1", "addr-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestConfirmPayment_CheckoutBasketFailurePropagates(t *testing.T) {
	pr := mocks.NewPayPhoneRemote(t)
	ar := mocks.NewAuthRemote(t)
	or := mocks.NewOrderRemote(t)
	br := mocks.NewBasketRemote(t)
	prr := mocks.NewPaymentRecordRemote(t)

	want := domain.PaymentConfirmation{TransactionID: "123", Status: "Approved"}
	ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
	prr.On("FindPayment", mock.Anything, "pay-1", "admin-tok").
		Return(domain.Payment{Status: "PENDING", Request: "original-request"}, nil)
	pr.On("ConfirmPayment", mock.Anything, "123", "ctx-1").Return(want, nil)
	prr.On("UpdatePayment", mock.Anything, "pay-1", "CONFIRMED", "original-request", mock.Anything, "admin-tok").
		Return(nil)
	br.On("ComputeFinancials", mock.Anything, "basket-1", "admin-tok").Return(domain.Financials{}, nil)
	br.On("CheckoutBasket", mock.Anything, "basket-1", "pay-1", "addr-1", "admin-tok").
		Return(errors.New("boom"))

	svc := payment.NewPaymentService(newDepsFull(pr, ar, or, br, prr))

	_, newBasketID, err := svc.ConfirmPayment(t.Context(), "123", "ctx-1", "user-1", "basket-1", "pay-1", "addr-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if newBasketID != "" {
		t.Fatalf("got newBasketID %q, want empty", newBasketID)
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
