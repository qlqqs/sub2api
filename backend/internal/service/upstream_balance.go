// CUSTOM: Provides read-only upstream account balance queries for administrators.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	UpstreamBalancePlatformAuto    = "auto"
	UpstreamBalancePlatformSub2API = "sub2api"
	UpstreamBalancePlatformNewAPI  = "new_api"
	UpstreamBalancePlatformUnknown = "unknown"

	UpstreamBalanceStatusAvailable   = "available"
	UpstreamBalanceStatusUnsupported = "unsupported"

	UpstreamBalanceScopeUser    = "user"
	UpstreamBalanceScopeAPIKey  = "api_key"
	UpstreamBalanceScopeUnknown = "unknown"

	upstreamBalancePlatformExtraKey = "upstream_platform_type"
	upstreamBalanceAccessTokenKey   = "balance_access_token"
	upstreamBalanceUserIDKey        = "balance_user_id"
	upstreamBalanceBaseURLKey       = "base_url"
	upstreamBalanceAPIKeyKey        = "api_key"

	upstreamBalanceRequestTimeout = 10 * time.Second
	upstreamBalanceResponseLimit  = 1 << 20

	// New API stores quota in token-like units where 500000 units equal one USD.
	newAPIQuotaUnitsPerUSD = int64(500000)
)

var errUpstreamBalanceProtocolMismatch = errors.New("upstream balance protocol mismatch")

type UpstreamBalanceResult struct {
	Status       string    `json:"status"`
	PlatformType string    `json:"platform_type"`
	Scope        string    `json:"scope"`
	Remaining    string    `json:"remaining,omitempty"`
	Used         *string   `json:"used,omitempty"`
	Total        *string   `json:"total,omitempty"`
	Unit         string    `json:"unit,omitempty"`
	APIKeyRate   *string   `json:"api_key_rate,omitempty"`
	QueriedAt    time.Time `json:"queried_at"`
}

type UpstreamBalanceHTTPClient interface {
	Do(request *http.Request, proxyURL string) (*http.Response, error)
}

// UpstreamBalanceAccountLookup is the minimal account dependency for balance queries.
// Full AccountRepository satisfies this interface at the wiring layer.
type UpstreamBalanceAccountLookup interface {
	GetByID(ctx context.Context, id int64) (*Account, error)
}

type UpstreamBalanceService struct {
	accountLookup UpstreamBalanceAccountLookup
	httpClient    UpstreamBalanceHTTPClient
	now           func() time.Time
}

func NewUpstreamBalanceService(
	accountLookup UpstreamBalanceAccountLookup,
	httpClient UpstreamBalanceHTTPClient,
) *UpstreamBalanceService {
	return &UpstreamBalanceService{
		accountLookup: accountLookup,
		httpClient:    httpClient,
		now:           time.Now,
	}
}

func (service *UpstreamBalanceService) QueryAccountBalance(
	ctx context.Context,
	accountID int64,
) (*UpstreamBalanceResult, error) {
	if service == nil || service.accountLookup == nil || service.httpClient == nil {
		return nil, infraerrors.New(http.StatusServiceUnavailable, "UPSTREAM_BALANCE_UNAVAILABLE", "upstream balance service is unavailable")
	}

	account, err := service.accountLookup.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, infraerrors.New(http.StatusNotFound, "ACCOUNT_NOT_FOUND", "account not found")
	}
	if account.Type != AccountTypeUpstream {
		return nil, infraerrors.BadRequest("UPSTREAM_BALANCE_ACCOUNT_TYPE_UNSUPPORTED", "balance query is only available for upstream accounts")
	}

	baseURL := strings.TrimSpace(account.GetCredential(upstreamBalanceBaseURLKey))
	if baseURL == "" {
		return nil, infraerrors.BadRequest("UPSTREAM_BALANCE_BASE_URL_REQUIRED", "upstream base URL is required")
	}
	parsedBaseURL, err := parseUpstreamBalanceBaseURL(baseURL)
	if err != nil {
		return nil, infraerrors.BadRequest("UPSTREAM_BALANCE_BASE_URL_INVALID", "upstream base URL is invalid")
	}

	platformType := normalizeUpstreamBalancePlatform(account.GetExtraString(upstreamBalancePlatformExtraKey))
	queryContext, cancel := context.WithTimeout(ctx, upstreamBalanceRequestTimeout)
	defer cancel()

	configuration := upstreamBalanceQueryConfiguration{
		account:     account,
		baseURL:     parsedBaseURL,
		accessToken: strings.TrimSpace(account.GetCredential(upstreamBalanceAccessTokenKey)),
		apiKey:      strings.TrimSpace(account.GetCredential(upstreamBalanceAPIKeyKey)),
		userID:      strings.TrimSpace(account.GetCredential(upstreamBalanceUserIDKey)),
		proxyURL:    upstreamBalanceProxyURL(account),
	}

	switch platformType {
	case UpstreamBalancePlatformSub2API:
		result, queryErr := service.querySub2APIBalance(queryContext, configuration)
		if queryErr == nil {
			return result, nil
		}
		// Protocol mismatch on an explicit platform is a stable unsupported result,
		// not an internal error leaking the private sentinel.
		if errors.Is(queryErr, errUpstreamBalanceProtocolMismatch) {
			return service.unsupportedResultWithPlatform(UpstreamBalancePlatformSub2API), nil
		}
		return nil, queryErr
	case UpstreamBalancePlatformNewAPI:
		result, queryErr := service.queryNewAPIBalance(queryContext, configuration)
		if queryErr == nil {
			return result, nil
		}
		if errors.Is(queryErr, errUpstreamBalanceProtocolMismatch) {
			return service.unsupportedResultWithPlatform(UpstreamBalancePlatformNewAPI), nil
		}
		return nil, queryErr
	case UpstreamBalancePlatformAuto:
		result, queryErr := service.querySub2APIBalance(queryContext, configuration)
		if queryErr == nil {
			return result, nil
		}
		if !errors.Is(queryErr, errUpstreamBalanceProtocolMismatch) {
			return nil, queryErr
		}

		result, queryErr = service.queryNewAPIBalance(queryContext, configuration)
		if queryErr == nil {
			return result, nil
		}
		if errors.Is(queryErr, errUpstreamBalanceProtocolMismatch) {
			return service.unsupportedResult(), nil
		}
		return nil, queryErr
	default:
		return nil, infraerrors.BadRequest("UPSTREAM_BALANCE_PLATFORM_INVALID", "upstream balance platform type is invalid")
	}
}

type upstreamBalanceQueryConfiguration struct {
	account     *Account
	baseURL     *url.URL
	accessToken string
	apiKey      string
	userID      string
	proxyURL    string
}

func (service *UpstreamBalanceService) querySub2APIBalance(
	ctx context.Context,
	configuration upstreamBalanceQueryConfiguration,
) (*UpstreamBalanceResult, error) {
	credential := configuration.accessToken
	if credential == "" {
		credential = configuration.apiKey
	}
	if credential == "" {
		return nil, infraerrors.BadRequest("UPSTREAM_BALANCE_CREDENTIAL_REQUIRED", "configure an API key or balance query access token")
	}

	requestURL := buildSub2APIUsageURL(configuration.baseURL)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return nil, infraerrors.BadRequest("UPSTREAM_BALANCE_BASE_URL_INVALID", "upstream base URL is invalid")
	}
	request.Header.Set("Authorization", "Bearer "+credential)
	request.Header.Set("Accept", "application/json")

	response, err := service.httpClient.Do(request, configuration.proxyURL)
	if err != nil {
		return nil, classifyUpstreamBalanceTransportError(ctx, err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusMethodNotAllowed {
		return nil, errUpstreamBalanceProtocolMismatch
	}
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return nil, upstreamBalanceCredentialError()
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, infraerrors.New(http.StatusBadGateway, "UPSTREAM_BALANCE_RESPONSE_ERROR", "upstream balance service returned an error")
	}

	payload, err := readUpstreamBalanceJSON(response.Body)
	if err != nil {
		return nil, err
	}
	result, err := parseSub2APIUsage(payload)
	if err != nil {
		if errors.Is(err, errUpstreamBalanceProtocolMismatch) {
			return nil, err
		}
		// Preserve structured application errors (e.g. credential required) from the parser.
		var applicationError *infraerrors.ApplicationError
		if errors.As(err, &applicationError) {
			return nil, err
		}
		return nil, infraerrors.New(http.StatusBadGateway, "UPSTREAM_BALANCE_RESPONSE_INVALID", "upstream balance response is invalid").WithCause(err)
	}
	result.PlatformType = UpstreamBalancePlatformSub2API
	result.QueriedAt = service.now().UTC()
	return result, nil
}

func (service *UpstreamBalanceService) queryNewAPIBalance(
	ctx context.Context,
	configuration upstreamBalanceQueryConfiguration,
) (*UpstreamBalanceResult, error) {
	credential := configuration.accessToken
	if credential == "" {
		credential = configuration.apiKey
	}
	if credential == "" || configuration.userID == "" {
		return nil, infraerrors.BadRequest("UPSTREAM_BALANCE_CREDENTIAL_REQUIRED", "configure a balance query access token and user ID")
	}

	requestURL := buildNewAPIUserURL(configuration.baseURL)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return nil, infraerrors.BadRequest("UPSTREAM_BALANCE_BASE_URL_INVALID", "upstream base URL is invalid")
	}
	request.Header.Set("Authorization", "Bearer "+credential)
	request.Header.Set("New-Api-User", configuration.userID)
	request.Header.Set("Accept", "application/json")

	response, err := service.httpClient.Do(request, configuration.proxyURL)
	if err != nil {
		return nil, classifyUpstreamBalanceTransportError(ctx, err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusMethodNotAllowed {
		return nil, errUpstreamBalanceProtocolMismatch
	}
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return nil, upstreamBalanceCredentialError()
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, infraerrors.New(http.StatusBadGateway, "UPSTREAM_BALANCE_RESPONSE_ERROR", "upstream balance service returned an error")
	}

	payload, err := readUpstreamBalanceJSON(response.Body)
	if err != nil {
		return nil, err
	}
	result, err := parseNewAPIUser(payload)
	if err != nil {
		if errors.Is(err, errUpstreamBalanceProtocolMismatch) {
			return nil, err
		}
		// Preserve structured application errors (e.g. credential required) from the parser.
		var applicationError *infraerrors.ApplicationError
		if errors.As(err, &applicationError) {
			return nil, err
		}
		return nil, infraerrors.New(http.StatusBadGateway, "UPSTREAM_BALANCE_RESPONSE_INVALID", "upstream balance response is invalid").WithCause(err)
	}
	result.PlatformType = UpstreamBalancePlatformNewAPI
	result.QueriedAt = service.now().UTC()
	return result, nil
}

func (service *UpstreamBalanceService) unsupportedResult() *UpstreamBalanceResult {
	return service.unsupportedResultWithPlatform(UpstreamBalancePlatformUnknown)
}

func (service *UpstreamBalanceService) unsupportedResultWithPlatform(platformType string) *UpstreamBalanceResult {
	resolvedPlatformType := strings.TrimSpace(platformType)
	if resolvedPlatformType == "" {
		resolvedPlatformType = UpstreamBalancePlatformUnknown
	}
	return &UpstreamBalanceResult{
		Status:       UpstreamBalanceStatusUnsupported,
		PlatformType: resolvedPlatformType,
		Scope:        UpstreamBalanceScopeUnknown,
		QueriedAt:    service.now().UTC(),
	}
}

func normalizeUpstreamBalancePlatform(platformType string) string {
	switch strings.ToLower(strings.TrimSpace(platformType)) {
	case "", UpstreamBalancePlatformAuto:
		return UpstreamBalancePlatformAuto
	case UpstreamBalancePlatformSub2API:
		return UpstreamBalancePlatformSub2API
	case UpstreamBalancePlatformNewAPI:
		return UpstreamBalancePlatformNewAPI
	default:
		return strings.ToLower(strings.TrimSpace(platformType))
	}
}

func parseUpstreamBalanceBaseURL(rawBaseURL string) (*url.URL, error) {
	parsedURL, err := url.Parse(strings.TrimSpace(rawBaseURL))
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" || parsedURL.Hostname() == "" {
		return nil, errors.New("invalid upstream base URL")
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, errors.New("unsupported upstream base URL scheme")
	}
	if parsedURL.User != nil || parsedURL.RawQuery != "" || parsedURL.Fragment != "" {
		return nil, errors.New("upstream base URL cannot contain user information, query, or fragment")
	}
	return parsedURL, nil
}

func buildSub2APIUsageURL(baseURL *url.URL) *url.URL {
	requestURL := cloneURL(baseURL)
	trimmedPath := strings.TrimSuffix(requestURL.Path, "/")
	if strings.HasSuffix(trimmedPath, "/v1") {
		requestURL.Path = trimmedPath + "/usage"
	} else {
		requestURL.Path = path.Join(trimmedPath, "v1", "usage")
	}
	return requestURL
}

func buildNewAPIUserURL(baseURL *url.URL) *url.URL {
	requestURL := cloneURL(baseURL)
	trimmedPath := strings.TrimSuffix(requestURL.Path, "/")
	trimmedPath = strings.TrimSuffix(trimmedPath, "/v1")
	trimmedPath = strings.TrimSuffix(trimmedPath, "/api")
	requestURL.Path = path.Join(trimmedPath, "api", "user", "self")
	return requestURL
}

func cloneURL(source *url.URL) *url.URL {
	clonedURL := *source
	return &clonedURL
}

func upstreamBalanceProxyURL(account *Account) string {
	if account == nil || account.ProxyID == nil || account.Proxy == nil {
		return ""
	}
	return account.Proxy.URL()
}

func readUpstreamBalanceJSON(reader io.Reader) ([]byte, error) {
	limitedReader := io.LimitReader(reader, upstreamBalanceResponseLimit+1)
	payload, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, infraerrors.New(http.StatusBadGateway, "UPSTREAM_BALANCE_RESPONSE_INVALID", "upstream balance response is invalid").WithCause(err)
	}
	if len(payload) == 0 || len(payload) > upstreamBalanceResponseLimit {
		return nil, infraerrors.New(http.StatusBadGateway, "UPSTREAM_BALANCE_RESPONSE_INVALID", "upstream balance response is invalid")
	}
	trimmedPayload := strings.TrimSpace(string(payload))
	if strings.HasPrefix(strings.ToLower(trimmedPayload), "<!doctype html") || strings.HasPrefix(strings.ToLower(trimmedPayload), "<html") {
		return nil, infraerrors.New(http.StatusBadGateway, "UPSTREAM_BALANCE_RESPONSE_INVALID", "upstream balance response is invalid")
	}
	return payload, nil
}

func classifyUpstreamBalanceTransportError(ctx context.Context, err error) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return infraerrors.New(http.StatusGatewayTimeout, "UPSTREAM_BALANCE_TIMEOUT", "upstream balance query timed out").WithCause(err)
	}
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
		return infraerrors.New(http.StatusRequestTimeout, "UPSTREAM_BALANCE_CANCELED", "upstream balance query was canceled").WithCause(err)
	}
	return infraerrors.New(http.StatusBadGateway, "UPSTREAM_BALANCE_NETWORK_ERROR", "unable to reach upstream balance service").WithCause(err)
}

func upstreamBalanceCredentialError() error {
	return infraerrors.BadRequest("UPSTREAM_BALANCE_CREDENTIAL_REQUIRED", "upstream rejected the balance query credentials; configure a dedicated access token and user ID if required")
}

type sub2APIUsageResponse struct {
	Mode      string          `json:"mode"`
	IsValid   *bool           `json:"isValid"`
	Remaining json.Number     `json:"remaining"`
	Balance   json.Number     `json:"balance"`
	Unit      string          `json:"unit"`
	Quota     *sub2APIQuota   `json:"quota"`
	Rate      json.RawMessage `json:"api_key_rate"`
}

type sub2APIQuota struct {
	Limit     json.Number `json:"limit"`
	Used      json.Number `json:"used"`
	Remaining json.Number `json:"remaining"`
	Unit      string      `json:"unit"`
}

func parseSub2APIUsage(payload []byte) (*UpstreamBalanceResult, error) {
	var response sub2APIUsageResponse
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.UseNumber()
	if err := decoder.Decode(&response); err != nil {
		return nil, fmt.Errorf("decode sub2api usage response: %w", err)
	}
	if err := requireJSONDocumentEnd(decoder); err != nil {
		return nil, fmt.Errorf("decode sub2api usage response: %w", err)
	}
	// isValid:false is a recognized sub2api status/credential failure, not a protocol mismatch
	// and not a generic invalid response body.
	if response.IsValid != nil && !*response.IsValid {
		return nil, upstreamBalanceCredentialError()
	}

	result := &UpstreamBalanceResult{Status: UpstreamBalanceStatusAvailable, Unit: strings.TrimSpace(response.Unit)}
	switch strings.ToLower(strings.TrimSpace(response.Mode)) {
	case "quota_limited":
		if response.Quota == nil {
			return nil, errors.New("sub2api quota response is missing quota")
		}
		remaining, err := normalizeDecimal(response.Quota.Remaining)
		if err != nil {
			return nil, err
		}
		used, err := normalizeOptionalDecimal(response.Quota.Used)
		if err != nil {
			return nil, err
		}
		total, err := normalizeOptionalDecimal(response.Quota.Limit)
		if err != nil {
			return nil, err
		}
		result.Scope = UpstreamBalanceScopeAPIKey
		result.Remaining = remaining
		result.Used = used
		result.Total = total
		if strings.TrimSpace(response.Quota.Unit) != "" {
			result.Unit = strings.TrimSpace(response.Quota.Unit)
		}
	case "unrestricted":
		remainingNumber := response.Remaining
		if remainingNumber == "" {
			remainingNumber = response.Balance
		}
		remaining, err := normalizeDecimal(remainingNumber)
		if err != nil {
			return nil, err
		}
		result.Scope = UpstreamBalanceScopeUser
		result.Remaining = remaining
	default:
		return nil, errUpstreamBalanceProtocolMismatch
	}
	if result.Unit == "" {
		result.Unit = "USD"
	}
	result.APIKeyRate = parseOptionalDecimalJSON(response.Rate)
	return result, nil
}

type newAPIUserEnvelope struct {
	Success *bool           `json:"success"`
	Data    json.RawMessage `json:"data"`
}

type newAPIUserResponse struct {
	Quota     json.Number `json:"quota"`
	UsedQuota json.Number `json:"used_quota"`
}

func parseNewAPIUser(payload []byte) (*UpstreamBalanceResult, error) {
	var envelope newAPIUserEnvelope
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.UseNumber()
	if err := decoder.Decode(&envelope); err != nil {
		// Unrecognized / non-JSON envelope shape is a protocol mismatch for auto probing.
		return nil, errUpstreamBalanceProtocolMismatch
	}
	if err := requireJSONDocumentEnd(decoder); err != nil {
		return nil, fmt.Errorf("decode new api user response: %w", err)
	}
	// Missing success field means the envelope shape is not New API.
	if envelope.Success == nil {
		return nil, errUpstreamBalanceProtocolMismatch
	}
	// Recognized New API envelope with success:false is an auth/credential or upstream error,
	// never "unsupported".
	if !*envelope.Success {
		return nil, upstreamBalanceCredentialError()
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil, fmt.Errorf("new api user data is missing")
	}

	var response newAPIUserResponse
	dataDecoder := json.NewDecoder(strings.NewReader(string(envelope.Data)))
	dataDecoder.UseNumber()
	if err := dataDecoder.Decode(&response); err != nil {
		return nil, fmt.Errorf("decode new api user data: %w", err)
	}
	if err := requireJSONDocumentEnd(dataDecoder); err != nil {
		return nil, fmt.Errorf("decode new api user data: %w", err)
	}
	remaining, err := quotaUnitsToUSD(response.Quota)
	if err != nil {
		return nil, err
	}
	used, err := quotaUnitsToUSD(response.UsedQuota)
	if err != nil {
		return nil, err
	}
	totalRat := new(big.Rat)
	remainingRat, _ := new(big.Rat).SetString(remaining)
	usedRat, _ := new(big.Rat).SetString(used)
	totalRat.Add(remainingRat, usedRat)
	total := formatRatDecimal(totalRat)

	return &UpstreamBalanceResult{
		Status:    UpstreamBalanceStatusAvailable,
		Scope:     UpstreamBalanceScopeUser,
		Remaining: remaining,
		Used:      &used,
		Total:     &total,
		Unit:      "USD",
	}, nil
}

func requireJSONDocumentEnd(decoder *json.Decoder) error {
	var trailingValue any
	if err := decoder.Decode(&trailingValue); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return fmt.Errorf("invalid trailing JSON content: %w", err)
	}
	return errors.New("multiple JSON values are not allowed")
}

func normalizeDecimal(number json.Number) (string, error) {
	value := strings.TrimSpace(number.String())
	if value == "" {
		return "", errors.New("missing decimal value")
	}
	rational, ok := new(big.Rat).SetString(value)
	if !ok {
		return "", errors.New("invalid decimal value")
	}
	return formatRatDecimal(rational), nil
}

func normalizeOptionalDecimal(number json.Number) (*string, error) {
	if strings.TrimSpace(number.String()) == "" {
		return nil, nil
	}
	value, err := normalizeDecimal(number)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func quotaUnitsToUSD(number json.Number) (string, error) {
	value := strings.TrimSpace(number.String())
	if value == "" {
		return "", errors.New("missing quota value")
	}
	rational, ok := new(big.Rat).SetString(value)
	if !ok {
		return "", errors.New("invalid quota value")
	}
	rational.Quo(rational, new(big.Rat).SetInt64(newAPIQuotaUnitsPerUSD))
	return formatRatDecimal(rational), nil
}

func formatRatDecimal(value *big.Rat) string {
	formatted := value.FloatString(12)
	formatted = strings.TrimRight(formatted, "0")
	formatted = strings.TrimRight(formatted, ".")
	if formatted == "" || formatted == "-0" {
		return "0"
	}
	return formatted
}

func parseOptionalDecimalJSON(payload json.RawMessage) *string {
	if len(payload) == 0 || string(payload) == "null" {
		return nil
	}
	var number json.Number
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.UseNumber()
	if err := decoder.Decode(&number); err != nil {
		return nil
	}
	value, err := normalizeDecimal(number)
	if err != nil {
		return nil
	}
	return &value
}
