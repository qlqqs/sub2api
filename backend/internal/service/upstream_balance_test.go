//go:build unit

// CUSTOM: Unit tests for upstream balance adapters, error classification, and credential redaction.
package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestParseSub2APIUsage_SuccessZeroAndInvalid(t *testing.T) {
	t.Parallel()

	successPayload := []byte(`{
		"mode": "unrestricted",
		"isValid": true,
		"balance": "12.50",
		"unit": "USD",
		"api_key_rate": "1.5"
	}`)
	result, err := parseSub2APIUsage(successPayload)
	require.NoError(t, err)
	require.Equal(t, UpstreamBalanceStatusAvailable, result.Status)
	require.Equal(t, UpstreamBalanceScopeUser, result.Scope)
	require.Equal(t, "12.5", result.Remaining)
	require.Equal(t, "USD", result.Unit)
	require.NotNil(t, result.APIKeyRate)
	require.Equal(t, "1.5", *result.APIKeyRate)

	zeroPayload := []byte(`{
		"mode": "unrestricted",
		"isValid": true,
		"balance": "0",
		"unit": "USD"
	}`)
	zeroResult, err := parseSub2APIUsage(zeroPayload)
	require.NoError(t, err)
	require.Equal(t, "0", zeroResult.Remaining)

	quotaPayload := []byte(`{
		"mode": "quota_limited",
		"isValid": true,
		"unit": "USD",
		"quota": {"limit": "100", "used": "25", "remaining": "75", "unit": "USD"}
	}`)
	quotaResult, err := parseSub2APIUsage(quotaPayload)
	require.NoError(t, err)
	require.Equal(t, UpstreamBalanceScopeAPIKey, quotaResult.Scope)
	require.Equal(t, "75", quotaResult.Remaining)

	// isValid:false is credential/status failure, not protocol mismatch.
	_, err = parseSub2APIUsage([]byte(`{"mode":"unrestricted","isValid":false,"balance":"9"}`))
	require.Error(t, err)
	var appErr *infraerrors.ApplicationError
	require.True(t, errors.As(err, &appErr))
	require.Equal(t, "UPSTREAM_BALANCE_CREDENTIAL_REQUIRED", appErr.Reason)
	require.False(t, errors.Is(err, errUpstreamBalanceProtocolMismatch))

	// Unknown mode is protocol mismatch for auto probing.
	_, err = parseSub2APIUsage([]byte(`{"mode":"something_else","balance":"1"}`))
	require.ErrorIs(t, err, errUpstreamBalanceProtocolMismatch)
}

func TestParseNewAPIUser_SuccessZeroFalseAndMalformed(t *testing.T) {
	t.Parallel()

	successPayload := []byte(`{"success":true,"data":{"quota":500000,"used_quota":0}}`)
	result, err := parseNewAPIUser(successPayload)
	require.NoError(t, err)
	require.Equal(t, UpstreamBalanceStatusAvailable, result.Status)
	require.Equal(t, UpstreamBalanceScopeUser, result.Scope)
	require.Equal(t, "1", result.Remaining)
	require.Equal(t, "USD", result.Unit)

	zeroPayload := []byte(`{"success":true,"data":{"quota":0,"used_quota":250000}}`)
	zeroResult, err := parseNewAPIUser(zeroPayload)
	require.NoError(t, err)
	require.Equal(t, "0", zeroResult.Remaining)
	require.NotNil(t, zeroResult.Used)
	require.Equal(t, "0.5", *zeroResult.Used)

	// Recognized envelope with success:false is credential/auth, not unsupported.
	_, err = parseNewAPIUser([]byte(`{"success":false,"message":"unauthorized","data":null}`))
	require.Error(t, err)
	var appErr *infraerrors.ApplicationError
	require.True(t, errors.As(err, &appErr))
	require.Equal(t, "UPSTREAM_BALANCE_CREDENTIAL_REQUIRED", appErr.Reason)
	require.False(t, errors.Is(err, errUpstreamBalanceProtocolMismatch))

	// Missing success field is protocol mismatch for auto probing.
	_, err = parseNewAPIUser([]byte(`{"data":{"quota":1}}`))
	require.ErrorIs(t, err, errUpstreamBalanceProtocolMismatch)

	// success:true with null data is invalid response, not protocol mismatch.
	_, err = parseNewAPIUser([]byte(`{"success":true,"data":null}`))
	require.Error(t, err)
	require.False(t, errors.Is(err, errUpstreamBalanceProtocolMismatch))
	require.Contains(t, err.Error(), "missing")
}

func TestSensitiveCredentialKeys_IncludeBalanceFields(t *testing.T) {
	t.Parallel()

	require.True(t, IsSensitiveCredentialKey("balance_access_token"))
	require.True(t, IsSensitiveCredentialKey("balance_user_id"))

	existing := map[string]any{
		"base_url":             "https://upstream.example.com",
		"api_key":              "sk-keep",
		"balance_access_token": "token-keep",
		"balance_user_id":      "99",
	}
	// Omitting sensitive keys preserves them.
	mergedPreserve := MergePreservingSensitiveCreds(existing, map[string]any{
		"base_url": "https://new.example.com",
	})
	require.Equal(t, "token-keep", mergedPreserve["balance_access_token"])
	require.Equal(t, "99", mergedPreserve["balance_user_id"])
	require.Equal(t, "https://new.example.com", mergedPreserve["base_url"])

	// Present empty balance_user_id overwrites (explicit clear).
	mergedClear := MergePreservingSensitiveCreds(existing, map[string]any{
		"balance_user_id": "",
	})
	require.Equal(t, "", mergedClear["balance_user_id"])
	require.Equal(t, "token-keep", mergedClear["balance_access_token"])
}

// --- service-level tests with minimal account lookup surface ---

type upstreamBalanceAccountRepoStub struct {
	account      *Account
	err          error
	mu           sync.Mutex
	getByIDCalls int
}

func (stub *upstreamBalanceAccountRepoStub) GetByID(ctx context.Context, id int64) (*Account, error) {
	stub.mu.Lock()
	stub.getByIDCalls++
	stub.mu.Unlock()
	if stub.err != nil {
		return nil, stub.err
	}
	return stub.account, nil
}

type upstreamBalanceHTTPClientStub struct {
	mu        sync.Mutex
	responses []*http.Response
	errors    []error
	requests  []*http.Request
}

func (stub *upstreamBalanceHTTPClientStub) Do(request *http.Request, proxyURL string) (*http.Response, error) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	stub.requests = append(stub.requests, request)
	var response *http.Response
	var err error
	if len(stub.responses) > 0 {
		response = stub.responses[0]
		stub.responses = stub.responses[1:]
	}
	if len(stub.errors) > 0 {
		err = stub.errors[0]
		stub.errors = stub.errors[1:]
	}
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, errors.New("no scripted response")
	}
	return response, nil
}

func jsonHTTPResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}
}

func sampleUpstreamAccount(platformType string, credentials map[string]any) *Account {
	if credentials == nil {
		credentials = map[string]any{}
	}
	if _, ok := credentials["base_url"]; !ok {
		credentials["base_url"] = "https://upstream.example.com"
	}
	return &Account{
		ID:          42,
		Name:        "upstream-balance-test",
		Type:        AccountTypeUpstream,
		Status:      "inactive",
		Credentials: credentials,
		Extra: map[string]any{
			"upstream_platform_type": platformType,
		},
	}
}


func newUpstreamBalanceServiceForTest(
	account *Account,
	httpClient UpstreamBalanceHTTPClient,
) (*UpstreamBalanceService, *upstreamBalanceAccountRepoStub) {
	repoStub := &upstreamBalanceAccountRepoStub{account: account}
	serviceInstance := &UpstreamBalanceService{
		accountLookup: repoStub,
		httpClient:    httpClient,
		now:           func() time.Time { return time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC) },
	}
	return serviceInstance, repoStub
}

func TestQueryAccountBalance_ExplicitVsAutoProbing(t *testing.T) {
	t.Parallel()

	// Explicit sub2api: 404 protocol mismatch becomes unsupported (not internal error).
	explicitSub2APIHTTP := &upstreamBalanceHTTPClientStub{
		responses: []*http.Response{jsonHTTPResponse(http.StatusNotFound, `{"error":"missing"}`)},
	}
	explicitSub2APIAccount := sampleUpstreamAccount(UpstreamBalancePlatformSub2API, map[string]any{
		"api_key": "sk-test",
	})
	explicitService, _ := newUpstreamBalanceServiceForTest(explicitSub2APIAccount, explicitSub2APIHTTP)
	result, err := explicitService.QueryAccountBalance(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, UpstreamBalanceStatusUnsupported, result.Status)
	require.Equal(t, UpstreamBalancePlatformSub2API, result.PlatformType)

	// Explicit new_api with success:false must not become unsupported.
	explicitNewAPIHTTP := &upstreamBalanceHTTPClientStub{
		responses: []*http.Response{jsonHTTPResponse(http.StatusOK, `{"success":false,"message":"auth"}`)},
	}
	explicitNewAPIAccount := sampleUpstreamAccount(UpstreamBalancePlatformNewAPI, map[string]any{
		"balance_access_token": "mgmt-token",
		"balance_user_id":      "7",
	})
	explicitNewAPIService, _ := newUpstreamBalanceServiceForTest(explicitNewAPIAccount, explicitNewAPIHTTP)
	_, err = explicitNewAPIService.QueryAccountBalance(context.Background(), 42)
	require.Error(t, err)
	var appErr *infraerrors.ApplicationError
	require.True(t, errors.As(err, &appErr))
	require.Equal(t, "UPSTREAM_BALANCE_CREDENTIAL_REQUIRED", appErr.Reason)

	// Auto: first protocol mismatch, second success.
	autoHTTP := &upstreamBalanceHTTPClientStub{
		responses: []*http.Response{
			jsonHTTPResponse(http.StatusNotFound, `not found`),
			jsonHTTPResponse(http.StatusOK, `{"success":true,"data":{"quota":1000000,"used_quota":0}}`),
		},
	}
	autoAccount := sampleUpstreamAccount(UpstreamBalancePlatformAuto, map[string]any{
		"balance_access_token": "mgmt-token",
		"balance_user_id":      "7",
		"api_key":              "sk-test",
	})
	autoService, _ := newUpstreamBalanceServiceForTest(autoAccount, autoHTTP)
	autoResult, err := autoService.QueryAccountBalance(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, UpstreamBalanceStatusAvailable, autoResult.Status)
	require.Equal(t, UpstreamBalancePlatformNewAPI, autoResult.PlatformType)
	require.Equal(t, "2", autoResult.Remaining)
	require.Len(t, autoHTTP.requests, 2)
}

func TestQueryAccountBalance_CredentialRequiredDisabledAndNoWrites(t *testing.T) {
	t.Parallel()

	// Missing credentials returns stable credential-required error.
	missingCredsAccount := sampleUpstreamAccount(UpstreamBalancePlatformNewAPI, map[string]any{
		"base_url": "https://upstream.example.com",
	})
	missingCredsService, repoStub := newUpstreamBalanceServiceForTest(missingCredsAccount, &upstreamBalanceHTTPClientStub{})
	_, err := missingCredsService.QueryAccountBalance(context.Background(), 42)
	require.Error(t, err)
	var appErr *infraerrors.ApplicationError
	require.True(t, errors.As(err, &appErr))
	require.Equal(t, "UPSTREAM_BALANCE_CREDENTIAL_REQUIRED", appErr.Reason)

	// Disabled upstream account is still queryable without repository writes.
	disabledAccount := sampleUpstreamAccount(UpstreamBalancePlatformSub2API, map[string]any{
		"api_key": "sk-test",
	})
	disabledAccount.Status = "inactive"
	disabledHTTP := &upstreamBalanceHTTPClientStub{
		responses: []*http.Response{jsonHTTPResponse(http.StatusOK, `{"mode":"unrestricted","isValid":true,"balance":"3.25","unit":"USD"}`)},
	}
	disabledService, repoStub := newUpstreamBalanceServiceForTest(disabledAccount, disabledHTTP)
	result, err := disabledService.QueryAccountBalance(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, UpstreamBalanceStatusAvailable, result.Status)
	require.Equal(t, "3.25", result.Remaining)
	require.Equal(t, 1, repoStub.getByIDCalls)
	require.Equal(t, "inactive", disabledAccount.Status)

	// Redirect/transport failure does not write repository state.
	redirectHTTP := &upstreamBalanceHTTPClientStub{
		errors: []error{errors.New("redirect to different host rejected")},
	}
	account := sampleUpstreamAccount(UpstreamBalancePlatformSub2API, map[string]any{"api_key": "sk-test"})
	serviceInstance, repoStub := newUpstreamBalanceServiceForTest(account, redirectHTTP)
	_, err = serviceInstance.QueryAccountBalance(context.Background(), 42)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "sk-test")
}

func TestQueryAccountBalance_DoesNotLeakTokensInErrors(t *testing.T) {
	t.Parallel()

	secretToken := "super-secret-balance-token-xyz"
	account := sampleUpstreamAccount(UpstreamBalancePlatformNewAPI, map[string]any{
		"balance_access_token": secretToken,
		"balance_user_id":      "7",
	})
	httpClient := &upstreamBalanceHTTPClientStub{
		responses: []*http.Response{jsonHTTPResponse(http.StatusUnauthorized, `{"success":false}`)},
	}
	serviceInstance, _ := newUpstreamBalanceServiceForTest(account, httpClient)
	_, err := serviceInstance.QueryAccountBalance(context.Background(), 42)
	require.Error(t, err)
	require.NotContains(t, err.Error(), secretToken)
}

func TestUpstreamBalanceParsersRejectTrailingJSONContent(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		payload string
		parse   func([]byte) (*UpstreamBalanceResult, error)
	}{
		{
			name:    "sub2api trailing text",
			payload: `{"mode":"unrestricted","isValid":true,"remaining":"4"} trailing`,
			parse:   parseSub2APIUsage,
		},
		{
			name:    "sub2api second document",
			payload: `{"mode":"unrestricted","isValid":true,"remaining":"4"}{"mode":"unrestricted","remaining":"9"}`,
			parse:   parseSub2APIUsage,
		},
		{
			name:    "new api trailing text",
			payload: `{"success":true,"data":{"quota":500000,"used_quota":0}} trailing`,
			parse:   parseNewAPIUser,
		},
		{
			name:    "new api second document",
			payload: `{"success":true,"data":{"quota":500000,"used_quota":0}}{"success":true,"data":{"quota":0,"used_quota":0}}`,
			parse:   parseNewAPIUser,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			_, err := testCase.parse([]byte(testCase.payload))
			require.Error(t, err)
			require.False(t, errors.Is(err, errUpstreamBalanceProtocolMismatch))
		})
	}

	_, err := parseSub2APIUsage([]byte("  \n" + `{"mode":"unrestricted","isValid":true,"remaining":"4"}` + "\t\n"))
	require.NoError(t, err)
	_, err = parseNewAPIUser([]byte("  \n" + `{"success":true,"data":{"quota":500000,"used_quota":0}}` + "\t\n"))
	require.NoError(t, err)
}

func TestQueryAccountBalance_TrailingContentKeepsInvalidResponseReason(t *testing.T) {
	t.Parallel()

	account := sampleUpstreamAccount(UpstreamBalancePlatformNewAPI, map[string]any{
		"balance_access_token": "management-token",
		"balance_user_id":      "7",
	})
	httpClient := &upstreamBalanceHTTPClientStub{
		responses: []*http.Response{jsonHTTPResponse(
			http.StatusOK,
			`{"success":true,"data":{"quota":500000,"used_quota":0}} trailing`,
		)},
	}
	serviceInstance, _ := newUpstreamBalanceServiceForTest(account, httpClient)

	_, err := serviceInstance.QueryAccountBalance(context.Background(), account.ID)
	requireApplicationErrorReason(t, err, "UPSTREAM_BALANCE_RESPONSE_INVALID")
}

func TestQueryAccountBalance_ValidationAndReadOnlyBoundaries(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		account        *Account
		repositoryErr  error
		expectedReason string
	}{
		{
			name:           "account not found",
			account:        nil,
			expectedReason: "ACCOUNT_NOT_FOUND",
		},
		{
			name: "non upstream account",
			account: &Account{
				ID:   42,
				Type: AccountTypeOAuth,
			},
			expectedReason: "UPSTREAM_BALANCE_ACCOUNT_TYPE_UNSUPPORTED",
		},
		{
			name: "missing base url",
			account: sampleUpstreamAccount(UpstreamBalancePlatformSub2API, map[string]any{
				"base_url": "",
				"api_key":  "key",
			}),
			expectedReason: "UPSTREAM_BALANCE_BASE_URL_REQUIRED",
		},
		{
			name: "invalid base url",
			account: sampleUpstreamAccount(UpstreamBalancePlatformSub2API, map[string]any{
				"base_url": "file:///tmp/upstream",
				"api_key":  "key",
			}),
			expectedReason: "UPSTREAM_BALANCE_BASE_URL_INVALID",
		},
		{
			name: "invalid platform",
			account: sampleUpstreamAccount("guessed-platform", map[string]any{
				"api_key": "key",
			}),
			expectedReason: "UPSTREAM_BALANCE_PLATFORM_INVALID",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			repositoryStub := &upstreamBalanceAccountRepoStub{account: testCase.account, err: testCase.repositoryErr}
			serviceInstance := NewUpstreamBalanceService(repositoryStub, &upstreamBalanceHTTPClientStub{})

			_, err := serviceInstance.QueryAccountBalance(context.Background(), 42)
			requireApplicationErrorReason(t, err, testCase.expectedReason)
		})
	}
}

func TestQueryAccountBalance_AutoStopsAfterCredentialAndGeneralHTTPFailures(t *testing.T) {
	t.Parallel()

	for _, statusCode := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusInternalServerError} {
		statusCode := statusCode
		t.Run(http.StatusText(statusCode), func(t *testing.T) {
			t.Parallel()
			account := sampleUpstreamAccount(UpstreamBalancePlatformAuto, map[string]any{
				"api_key":              "key",
				"balance_access_token": "management-token",
				"balance_user_id":      "7",
			})
			httpClient := &upstreamBalanceHTTPClientStub{
				responses: []*http.Response{jsonHTTPResponse(statusCode, `{"error":"redacted"}`)},
			}
			serviceInstance, _ := newUpstreamBalanceServiceForTest(account, httpClient)

			_, err := serviceInstance.QueryAccountBalance(context.Background(), account.ID)
			if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden {
				requireApplicationErrorReason(t, err, "UPSTREAM_BALANCE_CREDENTIAL_REQUIRED")
			} else {
				requireApplicationErrorReason(t, err, "UPSTREAM_BALANCE_RESPONSE_ERROR")
			}
			require.Len(t, httpClient.requests, 1)
		})
	}
}

func TestUpstreamBalanceTransportAndBodyErrorClassification(t *testing.T) {
	t.Parallel()

	timeoutContext, timeoutCancel := context.WithCancel(context.Background())
	timeoutCancel()
	requireApplicationErrorReason(
		t,
		classifyUpstreamBalanceTransportError(timeoutContext, context.DeadlineExceeded),
		"UPSTREAM_BALANCE_TIMEOUT",
	)

	canceledContext, cancel := context.WithCancel(context.Background())
	cancel()
	requireApplicationErrorReason(
		t,
		classifyUpstreamBalanceTransportError(canceledContext, context.Canceled),
		"UPSTREAM_BALANCE_CANCELED",
	)

	oversizedBody := strings.NewReader(strings.Repeat("x", upstreamBalanceResponseLimit+1))
	_, err := readUpstreamBalanceJSON(oversizedBody)
	requireApplicationErrorReason(t, err, "UPSTREAM_BALANCE_RESPONSE_INVALID")
}

func TestUnsupportedResultUsesPlatformUnknown(t *testing.T) {
	t.Parallel()

	serviceInstance := &UpstreamBalanceService{now: time.Now}
	result := serviceInstance.unsupportedResult()
	require.Equal(t, UpstreamBalancePlatformUnknown, result.PlatformType)
	require.Equal(t, UpstreamBalanceScopeUnknown, result.Scope)
}

func requireApplicationErrorReason(t *testing.T, err error, expectedReason string) {
	t.Helper()
	require.Error(t, err)
	var applicationError *infraerrors.ApplicationError
	require.True(t, errors.As(err, &applicationError))
	require.Equal(t, expectedReason, applicationError.Reason)
}
