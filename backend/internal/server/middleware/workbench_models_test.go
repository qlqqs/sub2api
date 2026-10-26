package middleware

import "testing"

func TestWorkbenchModelDiscoveryOnlyExemptsReadOnlyModels(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		want         bool
	}{
		{"GET", "/v1/models", true}, {"POST", "/v1/models", false}, {"POST", "/v1/images/generations", false}, {"POST", "/v1/responses", false}, {"GET", "/v1/models/other", false},
	} {
		if got := isWorkbenchModelDiscovery(tc.method, tc.path); got != tc.want {
			t.Errorf("%s %s: %v", tc.method, tc.path, got)
		}
	}
}
