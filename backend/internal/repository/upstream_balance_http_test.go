//go:build unit

// CUSTOM: Verifies balance HTTP redirects never forward management credentials.
package repository

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpstreamBalanceHTTPClientDoesNotFollowRedirects(t *testing.T) {
	t.Parallel()

	var sameOriginTargetCalls atomic.Int32
	sameOriginServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/target" {
			sameOriginTargetCalls.Add(1)
			require.Empty(t, request.Header.Get("Authorization"))
			require.Empty(t, request.Header.Get("New-Api-User"))
			responseWriter.WriteHeader(http.StatusNoContent)
			return
		}
		http.Redirect(responseWriter, request, "/target", http.StatusFound)
	}))
	defer sameOriginServer.Close()

	var crossOriginTargetCalls atomic.Int32
	crossOriginTarget := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		crossOriginTargetCalls.Add(1)
		require.Empty(t, request.Header.Get("Authorization"))
		require.Empty(t, request.Header.Get("New-Api-User"))
		responseWriter.WriteHeader(http.StatusNoContent)
	}))
	defer crossOriginTarget.Close()

	crossOriginSource := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		http.Redirect(responseWriter, request, crossOriginTarget.URL+"/target", http.StatusFound)
	}))
	defer crossOriginSource.Close()

	testCases := []struct {
		name       string
		requestURL string
	}{
		{name: "same origin", requestURL: sameOriginServer.URL + "/source"},
		{name: "cross origin", requestURL: crossOriginSource.URL + "/source"},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, testCase.requestURL, nil)
			require.NoError(t, err)
			request.Header.Set("Authorization", "Bearer balance-token-secret")
			request.Header.Set("New-Api-User", "balance-user-secret")

			client := &upstreamBalanceHTTPClient{}
			response, err := client.Do(request, "")
			require.NoError(t, err)
			defer response.Body.Close()
			_, err = io.Copy(io.Discard, response.Body)
			require.NoError(t, err)
			require.Equal(t, http.StatusFound, response.StatusCode, fmt.Sprintf("redirect Location: %s", response.Header.Get("Location")))
		})
	}

	require.Equal(t, int32(0), sameOriginTargetCalls.Load())
	require.Equal(t, int32(0), crossOriginTargetCalls.Load())
}
