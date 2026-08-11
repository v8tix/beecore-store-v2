package address_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/v8tix/beecore-store-v2/internal/core/address"
	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/port/resource/mocks"
)

func newDeps(addressRemote *mocks.AddressRemote, authRemote *mocks.AuthRemote) address.Dependencies {
	return address.Dependencies{
		AddressRemote: addressRemote,
		AuthRemote:    authRemote,
	}
}

func TestFindByUserID(t *testing.T) {
	ar := mocks.NewAuthRemote(t)
	ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)

	adr := mocks.NewAddressRemote(t)
	want := []domain.Address{{ID: "addr-1"}}
	adr.On("FindAddressesByUserID", mock.Anything, "u1", "admin-tok").Return(want, nil)

	svc := address.NewAddressService(newDeps(adr, ar))

	got, err := svc.FindByUserID(t.Context(), "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "addr-1" {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestHasAddresses(t *testing.T) {
	ar := mocks.NewAuthRemote(t)
	ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)

	adr := mocks.NewAddressRemote(t)
	adr.On("HasAddresses", mock.Anything, "u1", "admin-tok").Return(true, "addr-1", nil)

	svc := address.NewAddressService(newDeps(adr, ar))

	has, firstID, err := svc.HasAddresses(t.Context(), "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !has || firstID != "addr-1" {
		t.Fatalf("got (%v, %q), want (true, %q)", has, firstID, "addr-1")
	}
}

func TestInsert(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(adr *mocks.AddressRemote, ar *mocks.AuthRemote)
		wantErr   error
		wantID    string
	}{
		{
			name: "success",
			setupMock: func(adr *mocks.AddressRemote, ar *mocks.AuthRemote) {
				ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
				adr.On("InsertAddress", mock.Anything, "u1", domain.ShippingAddress, "EC", "Quito", "Pichincha", "170101", "Main St", "2nd St", "123", "+593999999999", "admin-tok").
					Return("new-addr", nil)
			},
			wantID: "new-addr",
		},
		{
			name: "admin token failure propagates",
			setupMock: func(adr *mocks.AddressRemote, ar *mocks.AuthRemote) {
				ar.On("GetAdminToken", mock.Anything).Return("", errors.New("boom"))
			},
			wantErr: errors.New("boom"),
		},
		{
			name: "bad address type propagates sentinel",
			setupMock: func(adr *mocks.AddressRemote, ar *mocks.AuthRemote) {
				ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
				adr.On("InsertAddress", mock.Anything, "u1", domain.ShippingAddress, "EC", "Quito", "Pichincha", "170101", "Main St", "2nd St", "123", "+593999999999", "admin-tok").
					Return("", domain.ErrBadAddressType)
			},
			wantErr: domain.ErrBadAddressType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adr := mocks.NewAddressRemote(t)
			ar := mocks.NewAuthRemote(t)
			tt.setupMock(adr, ar)

			svc := address.NewAddressService(newDeps(adr, ar))

			id, err := svc.Insert(t.Context(), "u1", domain.ShippingAddress, "EC", "Quito", "Pichincha", "170101", "Main St", "2nd St", "123", "999999999")

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				if errors.Is(tt.wantErr, domain.ErrBadAddressType) && !errors.Is(err, domain.ErrBadAddressType) {
					t.Fatalf("got error %v, want domain.ErrBadAddressType", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if id != tt.wantID {
				t.Fatalf("got id %q, want %q", id, tt.wantID)
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	ar := mocks.NewAuthRemote(t)
	ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)

	adr := mocks.NewAddressRemote(t)
	adr.On("UpdateAddress", mock.Anything, "addr-1", "u1", domain.ShippingAddress, "EC", "Quito", "Pichincha", "170101", "Main St", "2nd St", "123", "+593999999999", "admin-tok").
		Return(nil)

	svc := address.NewAddressService(newDeps(adr, ar))

	err := svc.Update(t.Context(), "addr-1", "u1", domain.ShippingAddress, "EC", "Quito", "Pichincha", "170101", "Main St", "2nd St", "123", "999999999")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
