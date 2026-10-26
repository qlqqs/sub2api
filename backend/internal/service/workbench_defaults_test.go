package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type workbenchRepoStub struct {
	mu      sync.Mutex
	claimed bool
	groupID int64
	saved   json.RawMessage
}

func (r *workbenchRepoStub) Target(context.Context) (int64, error) { return r.groupID, nil }
func (r *workbenchRepoStub) ClaimSignup(context.Context, int64, int64) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.claimed {
		return false, nil
	}
	r.claimed = true
	return true, nil
}
func (r *workbenchRepoStub) State(context.Context, int64, string) (*WorkbenchState, error) {
	return &WorkbenchState{}, nil
}
func (r *workbenchRepoStub) Save(_ context.Context, _ int64, _ string, p json.RawMessage, _ bool) (json.RawMessage, error) {
	r.saved = p
	return p, nil
}

type workbenchKeysStub struct {
	groups   []Group
	requests []CreateAPIKeyRequest
	fail     bool
}

func (s *workbenchKeysStub) GetAvailableGroups(context.Context, int64) ([]Group, error) {
	return s.groups, nil
}
func (s *workbenchKeysStub) Create(_ context.Context, _ int64, r CreateAPIKeyRequest) (*APIKey, error) {
	s.requests = append(s.requests, r)
	if s.fail {
		return nil, errors.New("create failed")
	}
	return &APIKey{}, nil
}
func TestWorkbenchSignupUsesIDAndCurrentNameOnce(t *testing.T) {
	r := &workbenchRepoStub{groupID: 2}
	k := &workbenchKeysStub{groups: []Group{{ID: 7, Name: "Codex 稳定分组", Status: StatusActive}, {ID: 2, Name: "Codex 稳定分组", Status: StatusActive}}}
	s := &AuthService{workbenchRepo: r, workbenchKeys: k}
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); s.bootstrapWorkbench(context.Background(), &User{ID: 1}) }()
	}
	wg.Wait()
	require.Len(t, k.requests, 1)
	require.Equal(t, int64(2), *k.requests[0].GroupID)
	require.Equal(t, "Codex 稳定分组", k.requests[0].Name)
}
func TestWorkbenchFailureDoesNotPanicOrRetry(t *testing.T) {
	for _, tc := range []struct {
		name   string
		id     int64
		groups []Group
		fail   bool
	}{
		{"missing", 0, nil, false}, {"forbidden", 2, nil, false}, {"inactive", 2, []Group{{ID: 2, Status: "disabled"}}, false},
		{"create", 2, []Group{{ID: 2, Name: "Codex 稳定分组", Status: StatusActive}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &workbenchRepoStub{groupID: tc.id}
			k := &workbenchKeysStub{groups: tc.groups, fail: tc.fail}
			s := &AuthService{workbenchRepo: r, workbenchKeys: k}
			require.NotPanics(t, func() {
				s.bootstrapWorkbench(context.Background(), &User{ID: 1})
				s.bootstrapWorkbench(context.Background(), &User{ID: 1})
			})
			require.True(t, r.claimed)
			require.LessOrEqual(t, len(k.requests), 1)
		})
	}
}
func TestWorkbenchPreferencesNeverPersistSecrets(t *testing.T) {
	r := &workbenchRepoStub{}
	s := &AuthService{workbenchRepo: r}
	_, err := s.SaveWorkbench(context.Background(), 1, "studio", json.RawMessage(`{"text":{"keyId":12,"model":"gpt-6.1-sol","key":"secret"},"apiKey":"secret"}`), true)
	require.NoError(t, err)
	require.NotContains(t, string(r.saved), "secret")
	_, err = s.SaveWorkbench(context.Background(), 1, "wrong", json.RawMessage(`{}`), true)
	require.Error(t, err)
	state, err := s.WorkbenchState(context.Background(), 1, "chat")
	require.NoError(t, err)
	require.Equal(t, "medium", state.ReasoningEffort)
}

func TestWorkbenchPreferenceErrorsAreClassified(t *testing.T) {
	s := &AuthService{}
	for _, tc := range []struct {
		kind, prefs, reason string
		status              int
	}{
		{"wrong", "{}", "WORKBENCH_INVALID", 400},
		{"studio", "{", "WORKBENCH_PREFERENCES_INVALID", 400},
		{"studio", "{}", "WORKBENCH_UNAVAILABLE", 503},
	} {
		_, err := s.SaveWorkbench(context.Background(), 1, tc.kind, json.RawMessage(tc.prefs), false)
		status, detail := infraerrors.ToHTTP(err)
		require.Equal(t, tc.status, status)
		require.Equal(t, tc.reason, detail.Reason)
	}
}
