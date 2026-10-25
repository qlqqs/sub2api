//go:build unit

package handler

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

type chatAuthUsers struct {
	service.UserRepository
	user *service.User
}

func (r *chatAuthUsers) GetByID(context.Context, int64) (*service.User, error) { return r.user, nil }
func (r *chatAuthUsers) GetUserAvatar(context.Context, int64) (*service.UserAvatar, error) {
	return nil, nil
}
func TestChatBackendRealJWTValidation(t *testing.T) {
	cfg := &config.Config{}
	cfg.JWT.Secret = "chat-test-secret-not-used-outside-unit-tests"
	users := &chatAuthUsers{user: &service.User{ID: 42, Status: service.StatusActive, TokenVersion: 3, TokenVersionResolved: true}}
	auth := service.NewAuthService(nil, users, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	b := NewChatBackend(&Handlers{Auth: &AuthHandler{authService: auth, userService: service.NewUserService(users, nil, nil, nil)}})
	for _, tc := range []struct {
		name, secret, status string
		expiry               time.Time
		version              int64
		valid                bool
	}{
		{"valid", cfg.JWT.Secret, service.StatusActive, time.Now().Add(time.Hour), 3, true},
		{"signature", "wrong-secret", service.StatusActive, time.Now().Add(time.Hour), 3, false},
		{"expired", cfg.JWT.Secret, service.StatusActive, time.Now().Add(-time.Hour), 3, false},
		{"banned", cfg.JWT.Secret, "inactive", time.Now().Add(time.Hour), 3, false},
		{"revoked", cfg.JWT.Secret, service.StatusActive, time.Now().Add(time.Hour), 2, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			users.user.Status = tc.status
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, service.JWTClaims{UserID: 42, TokenVersion: tc.version, RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(tc.expiry)}}).SignedString([]byte(tc.secret))
			require.NoError(t, err)
			u, expiry, err := b.Authenticate(context.Background(), token)
			if tc.valid {
				require.NoError(t, err)
				require.Equal(t, int64(42), u.ID)
				require.WithinDuration(t, tc.expiry, expiry, time.Second)
			} else {
				require.Error(t, err)
				require.Nil(t, u)
			}
		})
	}
}
