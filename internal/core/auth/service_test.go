package auth_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/v8tix/beecore-store-v2/internal/core/auth"
	"github.com/v8tix/beecore-store-v2/internal/core/domain"
	"github.com/v8tix/beecore-store-v2/internal/core/port/resource/mocks"
)

func newDeps(authRemote *mocks.AuthRemote, sessionStore *mocks.SessionStore, addressRemote *mocks.AddressRemote) auth.Dependencies {
	return auth.Dependencies{
		AuthRemote:          authRemote,
		SessionStore:        sessionStore,
		AddressRemote:       addressRemote,
		AdminEmail:          "admin@example.com",
		AdminPassword:       "admin-pw",
		BuyerRoleID:         "role-1",
		AccessTokenDuration: 24 * time.Hour,
		SessionTTL:          7 * 24 * time.Hour,
	}
}

func TestLogin(t *testing.T) {
	tests := []struct {
		name      string
		email     string
		password  string
		setupMock func(ar *mocks.AuthRemote, ss *mocks.SessionStore, adr *mocks.AddressRemote)
		wantErr   error
		wantUser  domain.User
	}{
		{
			name:     "success",
			email:    "a@b.com",
			password: "pw",
			setupMock: func(ar *mocks.AuthRemote, ss *mocks.SessionStore, adr *mocks.AddressRemote) {
				ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
				ar.On("FindUserByEmailAndPassword", mock.Anything, "a@b.com", "pw", "admin-tok").
					Return(domain.User{ID: "u1", DNI: "12345"}, nil)
				adr.On("HasAddresses", mock.Anything, "u1", "admin-tok").Return(true, "addr-1", nil)
				ar.On("FindUserTokenByEmailAndPassword", mock.Anything, "a@b.com", "pw", "admin-tok").
					Return("access-tok", "refresh-tok", nil)
				ss.On("Save", mock.Anything, mock.AnythingOfType("string"), mock.MatchedBy(func(s domain.Session) bool {
					return s.UserID == "u1" && s.UserDNI == "12345" && s.UserShippingAddress == "addr-1" &&
						s.UserAccessToken == "access-tok" && s.RefreshToken == "refresh-tok"
				}), 7*24*time.Hour).Return(nil)
			},
			wantUser: domain.User{ID: "u1", DNI: "12345"},
		},
		{
			name:     "invalid credentials propagates sentinel",
			email:    "a@b.com",
			password: "wrong",
			setupMock: func(ar *mocks.AuthRemote, ss *mocks.SessionStore, adr *mocks.AddressRemote) {
				ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
				ar.On("FindUserByEmailAndPassword", mock.Anything, "a@b.com", "wrong", "admin-tok").
					Return(domain.User{}, domain.ErrInvalidCredentials)
			},
			wantErr: domain.ErrInvalidCredentials,
		},
		{
			name:     "user not found propagates sentinel",
			email:    "missing@b.com",
			password: "pw",
			setupMock: func(ar *mocks.AuthRemote, ss *mocks.SessionStore, adr *mocks.AddressRemote) {
				ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
				ar.On("FindUserByEmailAndPassword", mock.Anything, "missing@b.com", "pw", "admin-tok").
					Return(domain.User{}, domain.ErrUserNotFound)
			},
			wantErr: domain.ErrUserNotFound,
		},
		{
			name:     "no addresses yet leaves UserShippingAddress empty",
			email:    "a@b.com",
			password: "pw",
			setupMock: func(ar *mocks.AuthRemote, ss *mocks.SessionStore, adr *mocks.AddressRemote) {
				ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
				ar.On("FindUserByEmailAndPassword", mock.Anything, "a@b.com", "pw", "admin-tok").
					Return(domain.User{ID: "u1"}, nil)
				adr.On("HasAddresses", mock.Anything, "u1", "admin-tok").Return(false, "", nil)
				ar.On("FindUserTokenByEmailAndPassword", mock.Anything, "a@b.com", "pw", "admin-tok").
					Return("access-tok", "refresh-tok", nil)
				ss.On("Save", mock.Anything, mock.AnythingOfType("string"), mock.MatchedBy(func(s domain.Session) bool {
					return s.UserID == "u1" && s.UserShippingAddress == "" && s.UserAccessToken == "access-tok"
				}), 7*24*time.Hour).Return(nil)
			},
			wantUser: domain.User{ID: "u1"},
		},
		{
			name:     "empty access token is an error",
			email:    "a@b.com",
			password: "pw",
			setupMock: func(ar *mocks.AuthRemote, ss *mocks.SessionStore, adr *mocks.AddressRemote) {
				ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
				ar.On("FindUserByEmailAndPassword", mock.Anything, "a@b.com", "pw", "admin-tok").
					Return(domain.User{ID: "u1"}, nil)
				adr.On("HasAddresses", mock.Anything, "u1", "admin-tok").Return(false, "", nil)
				ar.On("FindUserTokenByEmailAndPassword", mock.Anything, "a@b.com", "pw", "admin-tok").
					Return("", "", nil)
			},
			wantErr: errors.New("login succeeded but no user access token was returned"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ar := mocks.NewAuthRemote(t)
			ss := mocks.NewSessionStore(t)
			adr := mocks.NewAddressRemote(t)
			tt.setupMock(ar, ss, adr)

			svc := auth.NewAuthService(newDeps(ar, ss, adr))

			sessionID, sess, user, err := svc.Login(t.Context(), tt.email, tt.password)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				if errors.Is(tt.wantErr, domain.ErrInvalidCredentials) || errors.Is(tt.wantErr, domain.ErrUserNotFound) {
					if !errors.Is(err, tt.wantErr) {
						t.Fatalf("got error %v, want %v", err, tt.wantErr)
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sessionID == "" {
				t.Fatal("expected non-empty session ID")
			}
			if sess.UserID != tt.wantUser.ID {
				t.Fatalf("got session UserID %q, want %q", sess.UserID, tt.wantUser.ID)
			}
			if user.ID != tt.wantUser.ID {
				t.Fatalf("got user ID %q, want %q", user.ID, tt.wantUser.ID)
			}
		})
	}
}

func TestSignup(t *testing.T) {
	tests := []struct {
		name       string
		setupMock  func(ar *mocks.AuthRemote)
		wantErr    error
		wantUserID string
	}{
		{
			name: "success",
			setupMock: func(ar *mocks.AuthRemote) {
				ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
				ar.On("FindUserByEmail", mock.Anything, "new@b.com", "admin-tok").
					Return(domain.User{}, domain.ErrUserNotFound)
				ar.On("RegisterUser", mock.Anything, "Jane", "Doe", "new@b.com", "pw", "pw", "admin-tok", "role-1").
					Return("new-user-id", nil)
			},
			wantUserID: "new-user-id",
		},
		{
			name: "email already in use",
			setupMock: func(ar *mocks.AuthRemote) {
				ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
				ar.On("FindUserByEmail", mock.Anything, "existing@b.com", "admin-tok").
					Return(domain.User{ID: "u1"}, nil)
			},
			wantErr: domain.ErrEmailInUse,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ar := mocks.NewAuthRemote(t)
			tt.setupMock(ar)

			svc := auth.NewAuthService(newDeps(ar, mocks.NewSessionStore(t), mocks.NewAddressRemote(t)))

			email := "new@b.com"
			if tt.wantErr != nil {
				email = "existing@b.com"
			}

			userID, err := svc.Signup(t.Context(), "Jane", "Doe", email, "pw", "pw")

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("got error %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if userID != tt.wantUserID {
				t.Fatalf("got userID %q, want %q", userID, tt.wantUserID)
			}
		})
	}
}

func TestActivate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ar := mocks.NewAuthRemote(t)
		ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
		ar.On("ActivateUser", mock.Anything, "admin-tok", "u1", "good-code").Return(nil)

		svc := auth.NewAuthService(newDeps(ar, mocks.NewSessionStore(t), mocks.NewAddressRemote(t)))

		if err := svc.Activate(t.Context(), "u1", "good-code"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("invalid activation token", func(t *testing.T) {
		ar := mocks.NewAuthRemote(t)
		ar.On("GetAdminToken", mock.Anything).Return("admin-tok", nil)
		ar.On("ActivateUser", mock.Anything, "admin-tok", "u1", "bad-code").
			Return(domain.ErrInvalidActivationToken)

		svc := auth.NewAuthService(newDeps(ar, mocks.NewSessionStore(t), mocks.NewAddressRemote(t)))

		err := svc.Activate(t.Context(), "u1", "bad-code")
		if !errors.Is(err, domain.ErrInvalidActivationToken) {
			t.Fatalf("got %v, want domain.ErrInvalidActivationToken", err)
		}
	})
}

func TestForgotPassword(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ar := mocks.NewAuthRemote(t)
		ar.On("FindTokenByEmailAndPassword", mock.Anything, "admin@example.com", "admin-pw", "").
			Return("admin-tok", nil)
		ar.On("FindUserByEmail", mock.Anything, "a@b.com", "admin-tok").Return(domain.User{ID: "u1"}, nil)
		ar.On("SendResetPasswdEmail", mock.Anything, "a@b.com", "admin-tok").Return(nil)

		svc := auth.NewAuthService(newDeps(ar, mocks.NewSessionStore(t), mocks.NewAddressRemote(t)))

		if err := svc.ForgotPassword(t.Context(), "a@b.com"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("email not found does not send reset email", func(t *testing.T) {
		ar := mocks.NewAuthRemote(t)
		ar.On("FindTokenByEmailAndPassword", mock.Anything, "admin@example.com", "admin-pw", "").
			Return("admin-tok", nil)
		ar.On("FindUserByEmail", mock.Anything, "missing@b.com", "admin-tok").
			Return(domain.User{}, domain.ErrUserNotFound)

		svc := auth.NewAuthService(newDeps(ar, mocks.NewSessionStore(t), mocks.NewAddressRemote(t)))

		err := svc.ForgotPassword(t.Context(), "missing@b.com")
		if !errors.Is(err, domain.ErrUserNotFound) {
			t.Fatalf("got %v, want domain.ErrUserNotFound", err)
		}
		ar.AssertNotCalled(t, "SendResetPasswdEmail", mock.Anything, mock.Anything, mock.Anything)
	})
}

func TestResetPassword(t *testing.T) {
	ar := mocks.NewAuthRemote(t)
	ar.On("FindTokenByEmailAndPassword", mock.Anything, "admin@example.com", "admin-pw", "").
		Return("admin-tok", nil)
	ar.On("UpdateUserPassword", mock.Anything, "u1", "newpw", "newpw", "admin-tok").Return(nil)

	svc := auth.NewAuthService(newDeps(ar, mocks.NewSessionStore(t), mocks.NewAddressRemote(t)))

	if err := svc.ResetPassword(t.Context(), "u1", "newpw", "newpw"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
