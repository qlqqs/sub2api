//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWorkbenchRegistrationHooksFailOpen(t *testing.T) {
	for _, fail := range []bool{false, true} {
		for _, path := range []string{"email", "oauth_tokens", "email_oauth"} {
			t.Run(path+map[bool]string{false: "_success", true: "_key_failure"}[fail], func(t *testing.T) {
				users := &userRepoStub{nextID: 91}
				s := newAuthService(users, map[string]string{SettingKeyRegistrationEnabled: "true"}, nil, nil)
				s.refreshTokenCache = &refreshTokenCacheStub{}
				repo := &workbenchRepoStub{groupID: 2}
				keys := &workbenchKeysStub{fail: fail, groups: []Group{{ID: 2, Name: "Codex 稳定分组", Status: StatusActive}}}
				s.workbenchRepo, s.workbenchKeys = repo, keys
				var user *User
				var err error
				switch path {
				case "email":
					_, user, err = s.Register(context.Background(), "new-workbench@example.com", "password")
				case "oauth_tokens":
					_, user, err = s.LoginOrRegisterOAuthWithTokenPair(context.Background(), "linuxdo-456@linuxdo-connect.invalid", "workbench", "", "", "linuxdo")
				case "email_oauth":
					user, err = s.createEmailOAuthUser(context.Background(), "new-oauth@example.com", "workbench", "github", "", "")
				}
				require.NoError(t, err, "auxiliary key failure must not fail registration")
				require.NotNil(t, user)
				require.True(t, repo.claimed)
				require.Len(t, keys.requests, 1)
				require.Equal(t, int64(2), *keys.requests[0].GroupID)
			})
		}
	}
}

func TestWorkbenchExistingOAuthLoginDoesNotBootstrap(t *testing.T) {
	existing := &User{ID: 88, Email: "linuxdo-123@linuxdo-connect.invalid", Role: RoleUser, Status: StatusActive}
	s := newAuthService(&userRepoStub{user: existing}, map[string]string{SettingKeyRegistrationEnabled: "true"}, nil, nil)
	s.refreshTokenCache = &refreshTokenCacheStub{}
	repo, keys := &workbenchRepoStub{groupID: 2}, &workbenchKeysStub{}
	s.workbenchRepo, s.workbenchKeys = repo, keys
	_, user, err := s.LoginOrRegisterOAuthWithTokenPair(context.Background(), existing.Email, "existing", "", "", "linuxdo")
	require.NoError(t, err)
	require.Equal(t, existing.ID, user.ID)
	require.False(t, repo.claimed)
	require.Empty(t, keys.requests)
}
