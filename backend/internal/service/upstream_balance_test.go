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
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
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

// --- service-level tests with minimal repository surface ---

// upstreamBalanceAccountRepoStub implements only GetByID; remaining methods panic.
// Compile-time assertion is done by assigning to AccountRepository in the helper.
type upstreamBalanceAccountRepoStub struct {
	account      *Account
	err          error
	mu           sync.Mutex
	getByIDCalls int
	writeCalls   int
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

func (stub *upstreamBalanceAccountRepoStub) recordWrite(methodName string) error {
	stub.mu.Lock()
	stub.writeCalls++
	stub.mu.Unlock()
	panic("unexpected repository write: " + methodName)
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

// upstreamBalanceRepoAdapter wraps the GetByID-only stub so NewUpstreamBalanceService
// can accept it. We implement the interface by embedding and overriding GetByID via
// a dedicated type that panics on unused methods - generated below if compile fails.
//
// For unit tests we construct UpstreamBalanceService directly with the stub field typed
// as the package's AccountRepository only if the stub implements the full interface.
// To keep the test file maintainable, service tests use a local constructor that sets
// the unexported-equivalent fields via New + type assertion workaround:
//
//	service := &UpstreamBalanceService{accountRepository: ..., httpClient: ..., now: ...}
//
// AccountRepository is an interface; the stub must implement all methods.
// We use a panic-default embedding approach via code generation from the interface.

// NOTE: The following methods satisfy AccountRepository with panics/no-ops so the
// service tests can run. Only GetByID is used by QueryAccountBalance.
func (stub *upstreamBalanceAccountRepoStub) Create(ctx context.Context, account *Account) error {
	_ = stub.recordWrite("Create")
	return nil
}

func (stub *upstreamBalanceAccountRepoStub) GetByIDs(ctx context.Context, ids []int64) ([]*Account, error) {
	panic("unexpected GetByIDs call")
	return nil, nil
}

func (stub *upstreamBalanceAccountRepoStub) ExistsByID(ctx context.Context, id int64) (bool, error) {
	panic("unexpected ExistsByID call")
	return false, nil
}

func (stub *upstreamBalanceAccountRepoStub) GetByCRSAccountID(ctx context.Context, crsAccountID string) (*Account, error) {
	panic("unexpected GetByCRSAccountID call")
	return nil, nil
}

func (stub *upstreamBalanceAccountRepoStub) FindByExtraField(ctx context.Context, key string, value any) ([]Account, error) {
	panic("unexpected FindByExtraField call")
	return nil, nil
}

func (stub *upstreamBalanceAccountRepoStub) ListCRSAccountIDs(ctx context.Context) (map[string]int64, error) {
	panic("unexpected ListCRSAccountIDs call")
	return nil, nil
}

func (stub *upstreamBalanceAccountRepoStub) Update(ctx context.Context, account *Account) error {
	_ = stub.recordWrite("Update")
	return nil
}

func (stub *upstreamBalanceAccountRepoStub) Delete(ctx context.Context, id int64) error {
	_ = stub.recordWrite("Delete")
	return nil
}

func (stub *upstreamBalanceAccountRepoStub) List(ctx context.Context, params pagination.PaginationParams) ([]Account, *pagination.PaginationResult, error) {
	panic("unexpected List call")
	return nil, nil, nil
}

func (stub *upstreamBalanceAccountRepoStub) ListWithFilters(ctx context.Context, params pagination.PaginationParams, platform, accountType, status, search string, groupID int64, privacyMode string) ([]Account, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
	return nil, nil, nil
}

func (stub *upstreamBalanceAccountRepoStub) ListAllWithFilters(ctx context.Context, platform, accountType, status, search string, groupID int64, privacyMode string) ([]Account, error) {
	panic("unexpected ListAllWithFilters call")
	return nil, nil
}

func (stub *upstreamBalanceAccountRepoStub) ListByGroup(ctx context.Context, groupID int64) ([]Account, error) {
	panic("unexpected ListByGroup call")
	return nil, nil
}

func (stub *upstreamBalanceAccountRepoStub) ListActive(ctx context.Context) ([]Account, error) {
	panic("unexpected ListActive call")
	return nil, nil
}

func (stub *upstreamBalanceAccountRepoStub) ListByPlatform(ctx context.Context, platform string) ([]Account, error) {
	panic("unexpected ListByPlatform call")
	return nil, nil
}

func (stub *upstreamBalanceAccountRepoStub) UpdateLastUsed(ctx context.Context, id int64) error {
	_ = stub.recordWrite("UpdateLastUsed")
	return nil
}

func (stub *upstreamBalanceAccountRepoStub) BatchUpdateLastUsed(ctx context.Context, updates map[int64]time.Time) error {
	_ = stub.recordWrite("BatchUpdateLastUsed")
	return nil
}

func (stub *upstreamBalanceAccountRepoStub) SetError(ctx context.Context, id int64, errorMsg string) error {
	_ = stub.recordWrite("SetError")
	return nil
}

func (stub *upstreamBalanceAccountRepoStub) ClearError(ctx context.Context, id int64) error {
	_ = stub.recordWrite("ClearError")
	return nil
}

func (stub *upstreamBalanceAccountRepoStub) SetSchedulable(ctx context.Context, id int64, schedulable bool) error {
	_ = stub.recordWrite("SetSchedulable")
	return nil
}

func (stub *upstreamBalanceAccountRepoStub) AutoPauseExpiredAccounts(ctx context.Context, now time.Time) (int64, error) {
	_ = stub.recordWrite("AutoPauseExpiredAccounts")
	return 0, nil
}

func (stub *upstreamBalanceAccountRepoStub) BindGroups(ctx context.Context, accountID int64, groupIDs []int64) error {
	_ = stub.recordWrite("BindGroups")
	return nil
}

func (stub *upstreamBalanceAccountRepoStub) ListSchedulable(ctx context.Context) ([]Account, error) {
	panic("unexpected ListSchedulable call")
	return nil, nil
}

func (stub *upstreamBalanceAccountRepoStub) ListSchedulableByGroupID(ctx context.Context, groupID int64) ([]Account, error) {
	panic("unexpected ListSchedulableByGroupID call")
	return nil, nil
}

func (stub *upstreamBalanceAccountRepoStub) ListSchedulableByPlatform(ctx context.Context, platform string) ([]Account, error) {
	panic("unexpected ListSchedulableByPlatform call")
	return nil, nil
}

func (stub *upstreamBalanceAccountRepoStub) ListSchedulableByGroupIDAndPlatform(ctx context.Context, groupID int64, platform string) ([]Account, error) {
	panic("unexpected ListSchedulableByGroupIDAndPlatform call")
	return nil, nil
}

func (stub *upstreamBalanceAccountRepoStub) ListSchedulableByPlatforms(ctx context.Context, platforms []string) ([]Account, error) {
	panic("unexpected ListSchedulableByPlatforms call")
	return nil, nil
}

func (stub *upstreamBalanceAccountRepoStub) ListSchedulableByGroupIDAndPlatforms(ctx context.Context, groupID int64, platforms []string) ([]Account, error) {
	panic("unexpected ListSchedulableByGroupIDAndPlatforms call")
	return nil, nil
}

func (stub *upstreamBalanceAccountRepoStub) ListSchedulableUngroupedByPlatform(ctx context.Context, platform string) ([]Account, error) {
	panic("unexpected ListSchedulableUngroupedByPlatform call")
	return nil, nil
}

func (stub *upstreamBalanceAccountRepoStub) ListSchedulableUngroupedByPlatforms(ctx context.Context, platforms []string) ([]Account, error) {
	panic("unexpected ListSchedulableUngroupedByPlatforms call")
	return nil, nil
}

func (stub *upstreamBalanceAccountRepoStub) SetRateLimited(ctx context.Context, id int64, resetAt time.Time) error {
	_ = stub.recordWrite("SetRateLimited")
	return nil
}

func (stub *upstreamBalanceAccountRepoStub) SetModelRateLimit(ctx context.Context, id int64, scope string, resetAt time.Time, reason ...string) error {
	_ = stub.recordWrite("SetModelRateLimit")
	return nil
}

func (stub *upstreamBalanceAccountRepoStub) SetOverloaded(ctx context.Context, id int64, until time.Time) error {
	_ = stub.recordWrite("SetOverloaded")
	return nil
}

func (stub *upstreamBalanceAccountRepoStub) SetTempUnschedulable(ctx context.Context, id int64, until time.Time, reason string) error {
	_ = stub.recordWrite("SetTempUnschedulable")
	return nil
}

func (stub *upstreamBalanceAccountRepoStub) ClearTempUnschedulable(ctx context.Context, id int64) error {
	_ = stub.recordWrite("ClearTempUnschedulable")
	return nil
}

func (stub *upstreamBalanceAccountRepoStub) ClearRateLimit(ctx context.Context, id int64) error {
	_ = stub.recordWrite("ClearRateLimit")
	return nil
}

func (stub *upstreamBalanceAccountRepoStub) ClearAntigravityQuotaScopes(ctx context.Context, id int64) error {
	_ = stub.recordWrite("ClearAntigravityQuotaScopes")
	return nil
}

func (stub *upstreamBalanceAccountRepoStub) ClearModelRateLimits(ctx context.Context, id int64) error {
	_ = stub.recordWrite("ClearModelRateLimits")
	return nil
}

func (stub *upstreamBalanceAccountRepoStub) UpdateSessionWindow(ctx context.Context, id int64, start, end *time.Time, status string) error {
	_ = stub.recordWrite("UpdateSessionWindow")
	return nil
}

func (stub *upstreamBalanceAccountRepoStub) UpdateSessionWindowEnd(ctx context.Context, id int64, end time.Time) error {
	_ = stub.recordWrite("UpdateSessionWindowEnd")
	return nil
}

func (stub *upstreamBalanceAccountRepoStub) UpdateExtra(ctx context.Context, id int64, updates map[string]any) error {
	_ = stub.recordWrite("UpdateExtra")
	return nil
}

func (stub *upstreamBalanceAccountRepoStub) BulkUpdate(ctx context.Context, ids []int64, updates AccountBulkUpdate) (int64, error) {
	panic("unexpected BulkUpdate call")
	return 0, nil
}

func (stub *upstreamBalanceAccountRepoStub) IncrementQuotaUsed(ctx context.Context, id int64, amount float64) error {
	panic("unexpected IncrementQuotaUsed call")
	return nil
}

func (stub *upstreamBalanceAccountRepoStub) ResetQuotaUsed(ctx context.Context, id int64) error {
	panic("unexpected ResetQuotaUsed call")
	return nil
}

func (stub *upstreamBalanceAccountRepoStub) RevertProxyFallback(ctx context.Context, accountID int64) error {
	panic("unexpected RevertProxyFallback call")
	return nil
}

func (stub *upstreamBalanceAccountRepoStub) ListShadowsByParent(ctx context.Context, parentID int64) ([]*Account, error) {
	panic("unexpected ListShadowsByParent call")
	return nil, nil
}

func newUpstreamBalanceServiceForTest(account *Account, httpClient UpstreamBalanceHTTPClient) (*UpstreamBalanceService, *upstreamBalanceAccountRepoStub) {
	repoStub := &upstreamBalanceAccountRepoStub{account: account}
	// Compile-time interface check.
	var _ AccountRepository = repoStub
	serviceInstance := NewUpstreamBalanceService(repoStub, httpClient)
	serviceInstance.now = func() time.Time {
		return time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
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
	require.Equal(t, 0, repoStub.writeCalls)

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
	require.Equal(t, 0, repoStub.writeCalls)
	require.Equal(t, "inactive", disabledAccount.Status)

	// Redirect/transport failure does not write repository state.
	redirectHTTP := &upstreamBalanceHTTPClientStub{
		errors: []error{errors.New("redirect to different host rejected")},
	}
	account := sampleUpstreamAccount(UpstreamBalancePlatformSub2API, map[string]any{"api_key": "sk-test"})
	serviceInstance, repoStub := newUpstreamBalanceServiceForTest(account, redirectHTTP)
	_, err = serviceInstance.QueryAccountBalance(context.Background(), 42)
	require.Error(t, err)
	require.Equal(t, 0, repoStub.writeCalls)
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
			require.Equal(t, 0, repositoryStub.writeCalls)
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
