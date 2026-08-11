package user_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/port/resource/mocks"
	"github.com/v8tix/beecore-store-v2/internal/core/user"
)

func newDeps(userRemote *mocks.UserRemote, authRemote *mocks.AuthRemote) user.Dependencies {
	return user.Dependencies{
		UserRemote: userRemote,
		AuthRemote: authRemote,
	}
}

func TestUpdateProfile(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(ur *mocks.UserRemote, ar *mocks.AuthRemote)
		wantErr   error
	}{
		{
			name: "success prefixes phone with Ecuador country code",
			setupMock: func(ur *mocks.UserRemote, ar *mocks.AuthRemote) {
				ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
				ur.On("UpdateUser", mock.Anything, "u1", "0102030405", "Jane", "Doe", "1990-01-01", "+593999999999", "", "", "admin-tok").
					Return(nil)
			},
		},
		{
			name: "admin token failure propagates and skips UpdateUser",
			setupMock: func(ur *mocks.UserRemote, ar *mocks.AuthRemote) {
				ar.On("GetAdminToken", mock.Anything).Return("", errors.New("boom"))
			},
			wantErr: errors.New("boom"),
		},
		{
			name: "downstream not-found sentinel propagates",
			setupMock: func(ur *mocks.UserRemote, ar *mocks.AuthRemote) {
				ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
				ur.On("UpdateUser", mock.Anything, "u1", "0102030405", "Jane", "Doe", "1990-01-01", "+593999999999", "", "", "admin-tok").
					Return(domain.ErrUserNotFound)
			},
			wantErr: domain.ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ur := mocks.NewUserRemote(t)
			ar := mocks.NewAuthRemote(t)
			tt.setupMock(ur, ar)

			svc := user.NewUserService(newDeps(ur, ar))

			err := svc.UpdateProfile(t.Context(), "u1", "Jane", "Doe", "0102030405", "1990-01-01", "999999999")

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				if errors.Is(tt.wantErr, domain.ErrUserNotFound) {
					if !errors.Is(err, domain.ErrUserNotFound) {
						t.Fatalf("got error %v, want domain.ErrUserNotFound", err)
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
