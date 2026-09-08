// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.

// Package keywordapi contains the deliberately small, policy-bound transport
// used by the Keyword Planner commands. The generated HTTP client remains a
// scaffold; this package owns Google Ads OAuth refresh, required headers,
// request allowlisting, response receipts, and retry pacing.
package keywordapi

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/keyword-planner/internal/cliutil"
)

const (
	defaultBaseURL           = "https://googleads.googleapis.com"
	defaultTokenURL          = "https://oauth2.googleapis.com/token" // #nosec G101 -- public OAuth endpoint URL, never a credential.
	defaultUserAgent         = "keyword-planner-pp-cli"
	defaultAttempts          = 3
	defaultRequestInterval   = time.Second
	defaultRetryBase         = 100 * time.Millisecond
	defaultHTTPTimeout       = 30 * time.Second
	defaultRateLockDirectory = "keyword-planner/rate-limits"
	envFileEnv               = "KEYWORD_PLANNER_ENV_FILE"

	// These values identify the immutable generated contract in diagnostics.
	// They are safe metadata, never credential or upstream response content.
	APIVersion        = "v25"
	DiscoveryRevision = "20260831"
	RESTTransport     = "REST"

	ideasSuffix      = ":generateKeywordIdeas"
	historicalSuffix = ":generateKeywordHistoricalMetrics"
	accountSuffix    = "/googleAds:search"
	accountQuery     = "SELECT customer.id, customer.currency_code FROM customer LIMIT 1"
)

var customerPathPattern = regexp.MustCompile("^/v25/customers/([0-9]+(?:-[0-9]+)*)(:generateKeywordIdeas|:generateKeywordHistoricalMetrics)$")
var accountPathPattern = regexp.MustCompile("^/v25/customers/([0-9]+(?:-[0-9]+)*)/googleAds:search$")

// Config contains the non-secret runtime settings and the credentials loaded
// by LoadConfig. Credential values are retained only in memory by Client.
//
// BaseURL and HTTPClient are intended for test injection. The CLI must not
// expose arbitrary endpoint overrides to users.
type Config struct {
	ClientID       string
	ClientSecret   string
	RefreshToken   string
	DeveloperToken string

	CustomerID      string
	LoginCustomerID string

	AccessToken       string
	AccessTokenExpiry time.Time
	// TokenExpiry is accepted as the generated config's spelling for the
	// same in-memory access-token expiry.
	TokenExpiry time.Time

	BaseURL   string
	TokenURL  string
	UserAgent string
	EnvPath   string

	HTTPClient *http.Client

	// Limiter is an optional injected AdaptiveLimiter. A nil value creates the
	// production limiter at RatePerSecond (one request per second by default).
	Limiter       *cliutil.AdaptiveLimiter
	RatePerSecond float64

	// RateLockDir is the cross-process pacing directory. The default is under
	// the user's cache directory. MinRequestInterval defaults to one second and
	// is kept separate from the in-process AdaptiveLimiter so two processes
	// cannot interleave requests for the same customer.
	RateLockDir        string
	MinRequestInterval time.Duration
	MaxAttempts        int
	RetryBase          time.Duration
	Timeout            time.Duration
	Now                func() time.Time
	Sleep              func(context.Context, time.Duration) error
}

// LoadConfig reads only the selected Google Ads variables from envPath and
// then applies process-environment overrides. It never prints or returns
// credential values in an error.
func LoadConfig(envPath string) (Config, error) {
	return LoadConfigWithDefault(envPath, "")
}

// LoadConfigWithDefault uses defaultPath only when neither an explicit path nor
// KEYWORD_PLANNER_ENV_FILE is supplied. A missing fallback remains optional for
// environment-only bindings, including callers with an isolated home directory.
func LoadConfigWithDefault(envPath, defaultPath string) (Config, error) {
	sourcePath, explicitPath := resolveEnvPath(envPath)
	if !explicitPath && strings.TrimSpace(defaultPath) != "" {
		sourcePath = strings.TrimSpace(defaultPath)
	}
	cfg := Config{
		BaseURL:            defaultBaseURL,
		TokenURL:           defaultTokenURL,
		UserAgent:          defaultUserAgent,
		EnvPath:            sourcePath,
		RatePerSecond:      1,
		MinRequestInterval: defaultRequestInterval,
		MaxAttempts:        defaultAttempts,
		RetryBase:          defaultRetryBase,
		Timeout:            defaultHTTPTimeout,
	}

	values := make(map[string]string, len(configEnvNames))
	if sourcePath != "" {
		data, ok, err := readConfigEnvFile(sourcePath, explicitPath)
		if err != nil {
			return Config{}, newError(CodeConfigFile, 0, "", "", 0)
		}
		if ok {
			parsed, err := parseEnv(data)
			if err != nil {
				return Config{}, newError(CodeConfigFile, 0, "", "", 0)
			}
			for key, value := range parsed {
				values[key] = value
			}
		}
	}
	for _, key := range configEnvNames {
		if value, ok := os.LookupEnv(key); ok {
			values[key] = value
		}
	}

	cfg.ClientID = values["GOOGLE_ADS_CLIENT_ID"]
	cfg.ClientSecret = values["GOOGLE_ADS_CLIENT_SECRET"]
	cfg.RefreshToken = values["GOOGLE_ADS_REFRESH_TOKEN"]
	cfg.DeveloperToken = values["GOOGLE_ADS_DEVELOPER_TOKEN"]
	cfg.LoginCustomerID = values["GOOGLE_ADS_LOGIN_CUSTOMER_ID"]
	cfg.CustomerID = values["GOOGLE_ADS_CUSTOMER_ID"]
	return cfg, nil
}

// readConfigEnvFile validates the selected environment file before reading it.
// The implicit default may be absent because process environment bindings can
// supply the complete configuration. Any explicit path, including a dangling
// symlink, is required to resolve to a protected regular file.
func readConfigEnvFile(path string, explicitPath bool) ([]byte, bool, error) {
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		if !explicitPath && errors.Is(err, os.ErrNotExist) {
			// A truly absent implicit default is optional. Lstat distinguishes
			// that case from a dangling symlink, which must fail closed.
			if _, lstatErr := os.Lstat(path); errors.Is(lstatErr, os.ErrNotExist) {
				return nil, false, nil
			}
		}
		return nil, false, err
	}
	info, err := os.Stat(realPath)
	if err != nil {
		return nil, false, err
	}
	if !info.Mode().IsRegular() {
		return nil, false, errors.New("configuration environment path is not a regular file")
	}
	if err := cliutil.VerifyCredsPerms(realPath); err != nil {
		return nil, false, err
	}
	data, err := os.ReadFile(realPath)
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

func resolveEnvPath(envPath string) (string, bool) {
	if path := strings.TrimSpace(envPath); path != "" {
		return path, true
	}
	if path := strings.TrimSpace(os.Getenv(envFileEnv)); path != "" {
		return path, true
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return ".env", false
	}
	return filepath.Join(home, ".env"), false
}

var configEnvNames = []string{
	"GOOGLE_ADS_CLIENT_ID",
	"GOOGLE_ADS_CLIENT_SECRET",
	"GOOGLE_ADS_REFRESH_TOKEN",
	"GOOGLE_ADS_DEVELOPER_TOKEN",
	"GOOGLE_ADS_LOGIN_CUSTOMER_ID",
	"GOOGLE_ADS_CUSTOMER_ID",
}

func parseEnv(data []byte) (map[string]string, error) {
	values := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if !isConfigEnvName(key) {
			continue
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func isConfigEnvName(name string) bool {
	for _, allowed := range configEnvNames {
		if name == allowed {
			return true
		}
	}
	return false
}

// Response is the immutable receipt metadata supplied to the callback for
// every Google Ads attempt. Body is the exact response bytes. Err is populated
// only when no response was received or the response body could not be read;
// it is excluded from JSON so transport diagnostics cannot accidentally carry
// an untrusted error string into an archived receipt.
type Response struct {
	Status     int
	StatusCode int
	Body       []byte
	// BodyWithheld is true only when the upstream body contained a configured
	// credential binding. In that case Body is nil and FailureCode is stable.
	BodyWithheld bool
	FailureCode  ErrorCode
	RequestID    string
	FetchedAt    time.Time
	Attempt      int
	Err          error `json:"-"`
}

// AccountInfo is the read-only account metadata used to establish currency
// provenance for a snapshot.
type AccountInfo struct {
	CustomerID   string
	CurrencyCode string
	Provenance   string
	Response     Response
}

// ErrorCode is a stable machine-readable class for transport and validation
// failures. Error values deliberately omit upstream response bodies.
type ErrorCode string

const (
	CodeConfigFile                ErrorCode = "config_file"
	CodeCredentialsMissing        ErrorCode = "credentials_missing"
	CodeOAuthInvalidGrant         ErrorCode = "oauth_invalid_grant"
	CodeOAuthEndpoint             ErrorCode = "oauth_endpoint"
	CodeUnsupportedPath           ErrorCode = "unsupported_path"
	CodeInvalidRequest            ErrorCode = "invalid_request"
	CodeAuth                      ErrorCode = "auth"
	CodeAccessDenied              ErrorCode = "access_denied"
	CodeAPIVersion                ErrorCode = "api_version"
	CodeDailyQuota                ErrorCode = "daily_quota"
	CodeUpstream                  ErrorCode = "upstream"
	CodeUpstream5xx               ErrorCode = "upstream_5xx"
	CodeTransport                 ErrorCode = "transport"
	CodeResponseRead              ErrorCode = "response_read"
	CodeResponseMalformed         ErrorCode = "response_malformed"
	CodeResponseCallback          ErrorCode = "response_callback"
	CodeRateLock                  ErrorCode = "rate_lock"
	CodeContext                   ErrorCode = "context"
	CodeCredentialEcho            ErrorCode = "credential_echo_blocked" // #nosec G101 -- diagnostic error code, never a credential value.
	CodeRateLimit                 ErrorCode = "rate_limit"
	CodeUserPermissionDenied      ErrorCode = "user_permission_denied"
	CodeDeveloperTokenNotApproved ErrorCode = "developer_token_not_approved"
	CodeDeveloperTokenProhibited  ErrorCode = "developer_token_prohibited"
	CodeCustomerNotEnabled        ErrorCode = "customer_not_enabled"
)

// Error is the safe, typed error returned by this package. It contains no
// upstream response body, credential, request body, or authorization header.
type Error struct {
	Code              ErrorCode
	ExitCode          int
	StatusCode        int
	Method            string
	Path              string
	Attempt           int
	RetryAfter        time.Duration
	Hint              string
	APIVersion        string
	DiscoveryRevision string
	Transport         string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	msg := "keyword planner request failed: " + string(e.Code)
	if e.StatusCode > 0 {
		msg += fmt.Sprintf(" (HTTP %d)", e.StatusCode)
	}
	if e.Hint != "" {
		msg += "; " + e.Hint
	}
	if e.APIVersion != "" || e.DiscoveryRevision != "" || e.Transport != "" {
		msg += fmt.Sprintf(" [API %s; Discovery %s; transport %s]",
			e.APIVersion, e.DiscoveryRevision, e.Transport)
	}
	return msg
}

func newError(code ErrorCode, status int, method, path string, attempt int) *Error {
	return &Error{
		Code:              code,
		ExitCode:          exitCodeFor(code),
		StatusCode:        status,
		Method:            method,
		Path:              path,
		Attempt:           attempt,
		Hint:              errorHint(code),
		APIVersion:        APIVersion,
		DiscoveryRevision: DiscoveryRevision,
		Transport:         RESTTransport,
	}
}

func errorHint(code ErrorCode) string {
	switch code {
	case CodeConfigFile:
		return "check the selected environment-file path and permissions"
	case CodeCredentialsMissing:
		return "set the required Google Ads credential bindings and operating customer ID"
	case CodeOAuthInvalidGrant:
		return "the refresh token is invalid or revoked; re-authorize the approved OAuth client"
	case CodeOAuthEndpoint:
		return "check OAuth endpoint reachability and response configuration"
	case CodeUnsupportedPath:
		return "use one of the approved read-only Keyword Planner methods"
	case CodeInvalidRequest:
		return "check request fields, customer routing, and the read-only account query"
	case CodeAuth:
		return "check OAuth refresh-token validity and developer-token credentials"
	case CodeUserPermissionDenied:
		return "check the OAuth user's access to the operating customer and manager routing"
	case CodeDeveloperTokenNotApproved:
		return "check the developer-token access level and permitted Google Ads use"
	case CodeDeveloperTokenProhibited:
		return "the developer token is prohibited for this API; stop and inspect its access status"
	case CodeCustomerNotEnabled:
		return "enable the operating customer for Google Ads API access or choose an approved target"
	case CodeAccessDenied:
		return "check customer permissions, manager routing, and developer-token access"
	case CodeAPIVersion:
		return "use the pinned v25 REST endpoint and refresh the Discovery artifact before changing versions"
	case CodeDailyQuota:
		return "stop retries and wait for the developer-token daily quota window"
	case CodeRateLimit:
		return "the per-customer request rate was exhausted; bounded retries have stopped"
	case CodeUpstream5xx:
		return "the bounded retry budget ended; inspect the preserved request ID and Google Ads status"
	case CodeUpstream:
		return "inspect the preserved request ID and Google Ads status before retrying deliberately"
	case CodeTransport:
		return "check network reachability and the configured Google Ads endpoint"
	case CodeResponseRead:
		return "the response was incomplete; preserve the receipt and retry deliberately"
	case CodeResponseMalformed:
		return "Google returned an unexpected response shape; inspect the raw receipt against the pinned schema"
	case CodeResponseCallback:
		return "the receipt callback failed; inspect local portfolio persistence"
	case CodeRateLock:
		return "check the local per-customer rate-limit state directory and permissions"
	case CodeContext:
		return "the request deadline or cancellation expired"
	case CodeCredentialEcho:
		return "the response contained a configured credential; its body was withheld for safety"
	default:
		return ""
	}
}

func (e *Error) Unwrap() error { return nil }

func exitCodeFor(code ErrorCode) int {
	switch code {
	case CodeCredentialsMissing, CodeOAuthInvalidGrant, CodeOAuthEndpoint, CodeAuth:
		return 3
	case CodeRateLock, CodeContext, CodeTransport, CodeResponseRead:
		return 4
	case CodeUpstream5xx, CodeUpstream, CodeDailyQuota, CodeRateLimit:
		return 5
	case CodeAccessDenied, CodeUserPermissionDenied, CodeDeveloperTokenNotApproved,
		CodeDeveloperTokenProhibited, CodeCustomerNotEnabled, CodeAPIVersion:
		return 6
	case CodeResponseMalformed, CodeResponseCallback, CodeCredentialEcho:
		return 7
	default:
		return 2
	}
}

// CodeOf returns the stable code of an Error or a rate-limit error.
func CodeOf(err error) ErrorCode {
	if err == nil {
		return ""
	}
	var rate *cliutil.RateLimitError
	if errors.As(err, &rate) {
		var safe *Error
		if errors.As(rate.Cause, &safe) && safe.Code == CodeDailyQuota {
			return safe.Code
		}
		return CodeRateLimit
	}
	var safe *Error
	if errors.As(err, &safe) {
		return safe.Code
	}
	return ErrorCode("unknown")
}

// ExitCodeOf returns the stable process exit class of an Error.
func ExitCodeOf(err error) int {
	if err == nil {
		return 0
	}
	var rate *cliutil.RateLimitError
	if errors.As(err, &rate) {
		return 5
	}
	var safe *Error
	if errors.As(err, &safe) {
		return safe.ExitCode
	}
	return 1
}

type Client struct {
	cfg        Config
	httpClient *http.Client
	limiter    *cliutil.AdaptiveLimiter

	tokenMu      sync.Mutex
	accessToken  string
	tokenExpiry  time.Time
	tokenFetched time.Time
}

// NewClient applies production defaults without performing I/O. OAuth is
// refreshed lazily on the first Ads request.
func NewClient(cfg Config) *Client {
	cfg = withDefaults(cfg)
	var httpClientValue http.Client
	if cfg.HTTPClient == nil {
		httpClientValue.Timeout = cfg.Timeout
	} else {
		// Copy the injected client before installing the private redirect policy;
		// callers retain their own CheckRedirect behavior and client state.
		httpClientValue = *cfg.HTTPClient
	}
	httpClientValue.CheckRedirect = rejectRedirect
	httpClient := &httpClientValue
	limiter := cfg.Limiter
	if limiter == nil {
		rate := cfg.RatePerSecond
		if rate <= 0 {
			rate = 1
		}
		limiter = cliutil.NewAdaptiveLimiter(rate)
	}
	return &Client{
		cfg:         cfg,
		httpClient:  httpClient,
		limiter:     limiter,
		accessToken: cfg.AccessToken,
		tokenExpiry: firstTime(cfg.AccessTokenExpiry, cfg.TokenExpiry),
	}
}

func firstTime(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value
		}
	}
	return time.Time{}
}

func withDefaults(cfg Config) Config {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = defaultBaseURL
	}
	if strings.TrimSpace(cfg.TokenURL) == "" {
		cfg.TokenURL = defaultTokenURL
	}
	if strings.TrimSpace(cfg.UserAgent) == "" {
		cfg.UserAgent = defaultUserAgent
	}
	if cfg.RatePerSecond <= 0 {
		cfg.RatePerSecond = 1
	}
	if cfg.MinRequestInterval < 0 {
		cfg.MinRequestInterval = 0
	}
	if cfg.MinRequestInterval == 0 {
		cfg.MinRequestInterval = defaultRequestInterval
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = defaultAttempts
	}
	if cfg.RetryBase <= 0 {
		cfg.RetryBase = defaultRetryBase
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultHTTPTimeout
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Sleep == nil {
		cfg.Sleep = sleepContext
	}
	if strings.TrimSpace(cfg.RateLockDir) == "" {
		cfg.RateLockDir = defaultRateLockDir()
	}
	return cfg
}

func defaultRateLockDir() string {
	if dir, err := os.UserCacheDir(); err == nil && strings.TrimSpace(dir) != "" {
		return filepath.Join(dir, defaultRateLockDirectory)
	}
	return filepath.Join(os.TempDir(), defaultRateLockDirectory)
}

// Call sends one allowlisted read-only Google Ads request. The path is
// intentionally restricted to the two Planner methods and the read-only
// account GAQL helper. The callback runs after raw bytes are read but before
// any response classification, retry decision, or decoding.
func (c *Client) Call(ctx context.Context, method, path string, body []byte, callback func(Response) error) (Response, error) {
	if c == nil {
		return Response{}, newError(CodeInvalidRequest, 0, method, path, 0)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	path = normalizeCustomerPath(strings.TrimSpace(path))
	customerID, _, err := c.validateCall(method, path, body)
	if err != nil {
		return Response{}, err
	}
	callCtx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()

	if strings.TrimSpace(c.cfg.DeveloperToken) == "" {
		return Response{}, newError(CodeCredentialsMissing, 0, method, path, 0)
	}
	token, err := c.getAccessToken(callCtx)
	if err != nil {
		return Response{}, err
	}

	var last Response
	for attempt := 1; attempt <= c.cfg.MaxAttempts; attempt++ {
		release, err := c.acquireTurn(callCtx, customerID)
		if err != nil {
			return last, err
		}

		request, err := c.newRequest(callCtx, method, path, body, token)
		if err != nil {
			release()
			return last, err
		}
		start := c.cfg.Now()
		httpResponse, doErr := c.httpClient.Do(request)
		if doErr != nil {
			release()
			last = Response{
				FetchedAt: start,
				Attempt:   attempt,
				Err:       doErr,
			}
			c.recordAttempt(last)
			if callback != nil {
				callbackResponse := last
				if callbackErr := callback(callbackResponse); callbackErr != nil {
					return last, newError(CodeResponseCallback, 0, method, path, attempt)
				}
			}
			if callCtx.Err() != nil {
				return last, newError(CodeContext, 0, method, path, attempt)
			}
			if attempt < c.cfg.MaxAttempts {
				if err := c.cfg.Sleep(callCtx, c.retryDelay(attempt, 0)); err != nil {
					return last, newError(CodeContext, 0, method, path, attempt)
				}
				continue
			}
			return last, newError(CodeTransport, 0, method, path, attempt)
		}

		rawBody, readErr := io.ReadAll(httpResponse.Body)
		_ = httpResponse.Body.Close()
		release()
		last = Response{
			Status:     httpResponse.StatusCode,
			StatusCode: httpResponse.StatusCode,
			Body:       append([]byte(nil), rawBody...),
			RequestID:  requestID(httpResponse.Header),
			FetchedAt:  c.cfg.Now(),
			Attempt:    attempt,
			Err:        readErr,
		}
		if c.credentialEcho(rawBody, token) {
			last.Body = nil
			last.BodyWithheld = true
			last.FailureCode = CodeCredentialEcho
			last.Err = newError(CodeCredentialEcho, httpResponse.StatusCode, method, path, attempt)
			c.recordAttempt(last)
			if callback != nil {
				if callbackErr := callback(last); callbackErr != nil {
					return last, newError(CodeResponseCallback, httpResponse.StatusCode, method, path, attempt)
				}
			}
			return last, last.Err
		}
		c.recordAttempt(last)
		if callback != nil {
			callbackResponse := last
			callbackResponse.Body = append([]byte(nil), last.Body...)
			if callbackErr := callback(callbackResponse); callbackErr != nil {
				return last, newError(CodeResponseCallback, httpResponse.StatusCode, method, path, attempt)
			}
		}
		if readErr != nil {
			return last, newError(CodeResponseRead, httpResponse.StatusCode, method, path, attempt)
		}

		if remaining, resetAt, ok := cliutil.ParseRateLimitHeaders(httpResponse.Header); ok {
			c.limiter.ObserveHeaders(remaining, resetAt)
		}
		if httpResponse.StatusCode >= 200 && httpResponse.StatusCode < 300 {
			c.limiter.OnSuccess()
			return last, nil
		}

		retryAfter := parseRetryAfter(httpResponse.Header)
		failureCode := googleAdsFailureCode(rawBody)
		if isPermanentGoogleAdsCode(failureCode) && failureCode != "RESOURCE_EXHAUSTED" {
			return last, classifyHTTPError(httpResponse.StatusCode, rawBody, method, path, attempt)
		}
		if httpResponse.StatusCode == http.StatusTooManyRequests {
			if containsDailyQuota(rawBody) {
				return last, rateLimitError(path, attempt, 0, CodeDailyQuota)
			}
			c.limiter.OnRateLimit()
			if attempt < c.cfg.MaxAttempts {
				if err := c.cfg.Sleep(callCtx, c.retryDelay(attempt, retryAfter)); err != nil {
					return last, newError(CodeContext, httpResponse.StatusCode, method, path, attempt)
				}
				continue
			}
			return last, rateLimitError(path, attempt, retryAfter, CodeRateLimit)
		}

		if httpResponse.StatusCode >= 500 && httpResponse.StatusCode <= 599 {
			if attempt < c.cfg.MaxAttempts {
				if err := c.cfg.Sleep(callCtx, c.retryDelay(attempt, retryAfter)); err != nil {
					return last, newError(CodeContext, httpResponse.StatusCode, method, path, attempt)
				}
				continue
			}
			return last, newError(CodeUpstream5xx, httpResponse.StatusCode, method, path, attempt)
		}
		return last, classifyHTTPError(httpResponse.StatusCode, rawBody, method, path, attempt)
	}
	return last, newError(CodeTransport, 0, method, path, c.cfg.MaxAttempts)
}

func (c *Client) credentialEcho(body []byte, accessToken string) bool {
	if len(body) == 0 {
		return false
	}
	for _, credential := range []string{
		c.cfg.ClientID,
		c.cfg.ClientSecret,
		c.cfg.RefreshToken,
		c.cfg.DeveloperToken,
		c.cfg.AccessToken,
		accessToken,
	} {
		credential = strings.TrimSpace(credential)
		if credential != "" && bytes.Contains(body, []byte(credential)) {
			return true
		}
	}
	return false
}

func (c *Client) recordAttempt(response Response) {
	path := strings.TrimSpace(os.Getenv("KEYWORD_PLANNER_CALL_LEDGER"))
	if path == "" {
		return
	}
	kind := "response"
	switch {
	case response.Status == 0:
		kind = "transport"
	case response.BodyWithheld:
		kind = string(response.FailureCode)
	case response.Status == http.StatusTooManyRequests:
		kind = "rate_limit"
	case response.Status >= 500 && response.Status <= 599:
		kind = "upstream_5xx"
	}
	record := struct {
		Status int    `json:"status"`
		Kind   string `json:"kind"`
		Time   string `json:"time"`
	}{
		Status: response.Status,
		Kind:   kind,
		Time:   response.FetchedAt.UTC().Format(time.RFC3339Nano),
	}
	data, err := json.Marshal(record)
	if err != nil {
		return
	}
	file, err := os.OpenFile(filepath.Clean(path), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600) // #nosec G304 G703 -- optional operator-selected call ledger path; the caller already controls this local output destination.
	if err != nil {
		return
	}
	_, _ = file.Write(append(data, '\n'))
	_ = file.Close()
}

func (c *Client) validateCall(method, path string, body []byte) (string, string, error) {
	if method != http.MethodPost {
		return "", "", newError(CodeUnsupportedPath, 0, method, path, 0)
	}
	if strings.Contains(path, "://") || !strings.HasPrefix(path, "/") || strings.Contains(path, "?") || strings.Contains(path, "#") {
		return "", "", newError(CodeUnsupportedPath, 0, method, path, 0)
	}
	if match := customerPathPattern.FindStringSubmatch(path); match != nil {
		if c.cfg.CustomerID != "" &&
			normalizeCustomerID(c.cfg.CustomerID) != normalizeCustomerID(match[1]) {
			return "", "", newError(CodeInvalidRequest, 0, method, path, 0)
		}
		if strings.HasSuffix(path, ideasSuffix) {
			return match[1], "ideas", nil
		}
		if strings.HasSuffix(path, historicalSuffix) {
			return match[1], "historical", nil
		}
	}
	if match := accountPathPattern.FindStringSubmatch(path); match != nil {
		if c.cfg.CustomerID != "" &&
			normalizeCustomerID(c.cfg.CustomerID) != normalizeCustomerID(match[1]) {
			return "", "", newError(CodeInvalidRequest, 0, method, path, 0)
		}
		if !validAccountQuery(body) {
			return "", "", newError(CodeInvalidRequest, 0, method, path, 0)
		}
		return match[1], "account", nil
	}
	return "", "", newError(CodeUnsupportedPath, 0, method, path, 0)
}

func validAccountQuery(body []byte) bool {
	var request struct {
		Query string `json:"query"`
	}
	if json.Unmarshal(body, &request) != nil {
		return false
	}
	return strings.TrimSpace(request.Query) == accountQuery
}

func normalizeCustomerID(value string) string {
	return strings.ReplaceAll(strings.TrimSpace(value), "-", "")
}

func normalizeCustomerPath(path string) string {
	match := customerPathPattern.FindStringSubmatch(path)
	if match == nil {
		match = accountPathPattern.FindStringSubmatch(path)
	}
	if match == nil {
		return path
	}
	return strings.Replace(path, match[1], normalizeCustomerID(match[1]), 1)
}

func (c *Client) newRequest(ctx context.Context, method, path string, body []byte, accessToken string) (*http.Request, error) {
	base, err := url.Parse(strings.TrimRight(c.cfg.BaseURL, "/"))
	if err != nil || base.Scheme == "" || base.Host == "" || base.User != nil || base.RawQuery != "" {
		return nil, newError(CodeInvalidRequest, 0, method, path, 0)
	}
	relative, err := url.Parse(path)
	if err != nil || relative.IsAbs() || relative.RawQuery != "" || relative.Fragment != "" {
		return nil, newError(CodeInvalidRequest, 0, method, path, 0)
	}
	target := base.ResolveReference(relative)
	request, err := http.NewRequestWithContext(ctx, method, target.String(), bytes.NewReader(body))
	if err != nil {
		return nil, newError(CodeInvalidRequest, 0, method, path, 0)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("developer-token", c.cfg.DeveloperToken)
	if strings.TrimSpace(c.cfg.LoginCustomerID) != "" {
		request.Header.Set("login-customer-id", normalizeCustomerID(c.cfg.LoginCustomerID))
	}
	request.Header.Set("User-Agent", c.cfg.UserAgent)
	return request, nil
}

func requestID(header http.Header) string {
	for _, name := range []string{"request-id", "google-ads-request-id", "x-request-id"} {
		if value := strings.TrimSpace(header.Get(name)); value != "" {
			return value
		}
	}
	return ""
}

// Account performs the one allowlisted read-only GAQL helper query and returns
// currency with its source provenance. The callback is passed through to Call
// and therefore receives the raw account response before JSON decoding.
func (c *Client) Account(ctx context.Context, callback func(Response) error) (AccountInfo, error) {
	if c == nil || strings.TrimSpace(c.cfg.CustomerID) == "" {
		return AccountInfo{}, newError(CodeCredentialsMissing, 0, http.MethodPost, accountSuffix, 0)
	}
	query := struct {
		Query string `json:"query"`
	}{
		Query: accountQuery,
	}
	body, err := json.Marshal(query)
	if err != nil {
		return AccountInfo{}, newError(CodeInvalidRequest, 0, http.MethodPost, accountSuffix, 0)
	}
	path := "/v25/customers/" + normalizeCustomerID(c.cfg.CustomerID) + accountSuffix
	response, err := c.Call(ctx, http.MethodPost, path, body, callback)
	if err != nil {
		return AccountInfo{Response: response}, err
	}

	var payload struct {
		Results []struct {
			Customer struct {
				ID               string `json:"id"`
				CurrencyCode     string `json:"currencyCode"`
				CurrencyCodeREST string `json:"currency_code"`
			} `json:"customer"`
		} `json:"results"`
	}
	if json.Unmarshal(response.Body, &payload) != nil || len(payload.Results) == 0 {
		return AccountInfo{Response: response}, newError(CodeResponseMalformed, response.Status, http.MethodPost, path, response.Attempt)
	}
	customer := payload.Results[0].Customer
	currency := strings.TrimSpace(customer.CurrencyCode)
	if currency == "" {
		currency = strings.TrimSpace(customer.CurrencyCodeREST)
	}
	if currency == "" {
		return AccountInfo{Response: response}, newError(CodeResponseMalformed, response.Status, http.MethodPost, path, response.Attempt)
	}
	customerID := normalizeCustomerID(customer.ID)
	if customerID == "" || customerID != normalizeCustomerID(c.cfg.CustomerID) {
		return AccountInfo{Response: response}, newError(CodeResponseMalformed, response.Status, http.MethodPost, path, response.Attempt)
	}
	if !validCurrencyCode(currency) {
		return AccountInfo{Response: response}, newError(CodeResponseMalformed, response.Status, http.MethodPost, path, response.Attempt)
	}
	return AccountInfo{
		CustomerID:   customerID,
		CurrencyCode: currency,
		Provenance:   "google-ads.customer.currency_code",
		Response:     response,
	}, nil
}

// rejectRedirect makes OAuth and Google Ads requests fail closed at the first
// redirect. ErrUseLastResponse preserves the exact 3xx response for Ads
// receipts while ensuring no redirected request is issued.
func rejectRedirect(_ *http.Request, _ []*http.Request) error {
	return http.ErrUseLastResponse
}

func validCurrencyCode(value string) bool {
	if len(value) != 3 {
		return false
	}
	for _, character := range value {
		if character < 'A' || character > 'Z' {
			return false
		}
	}
	return true
}

func (c *Client) getAccessToken(ctx context.Context) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	now := c.cfg.Now()
	if strings.TrimSpace(c.accessToken) != "" &&
		(c.tokenExpiry.IsZero() || c.tokenExpiry.After(now.Add(30*time.Second))) {
		return c.accessToken, nil
	}
	if strings.TrimSpace(c.cfg.ClientID) == "" ||
		strings.TrimSpace(c.cfg.ClientSecret) == "" ||
		strings.TrimSpace(c.cfg.RefreshToken) == "" {
		return "", newError(CodeCredentialsMissing, 0, http.MethodPost, "", 0)
	}

	form := url.Values{}
	form.Set("client_id", c.cfg.ClientID)
	form.Set("client_secret", c.cfg.ClientSecret)
	form.Set("refresh_token", c.cfg.RefreshToken)
	form.Set("grant_type", "refresh_token")
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", newError(CodeOAuthEndpoint, 0, http.MethodPost, "", 0)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", c.cfg.UserAgent)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", newError(CodeOAuthEndpoint, 0, http.MethodPost, "", 0)
	}
	rawBody, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil {
		return "", newError(CodeOAuthEndpoint, response.StatusCode, http.MethodPost, "", 0)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		code := tokenErrorCode(rawBody)
		if code == "" {
			code = CodeOAuthEndpoint
		}
		return "", newError(code, response.StatusCode, http.MethodPost, "", 0)
	}
	var token struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if json.Unmarshal(rawBody, &token) != nil || strings.TrimSpace(token.AccessToken) == "" {
		return "", newError(CodeOAuthEndpoint, response.StatusCode, http.MethodPost, "", 0)
	}
	c.accessToken = token.AccessToken
	if token.ExpiresIn <= 0 {
		token.ExpiresIn = int64((time.Hour).Seconds())
	}
	c.tokenExpiry = now.Add(time.Duration(token.ExpiresIn) * time.Second)
	c.tokenFetched = now
	return c.accessToken, nil
}

func tokenErrorCode(body []byte) ErrorCode {
	var payload struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &payload) != nil {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(payload.Error)) {
	case "invalid_grant":
		return CodeOAuthInvalidGrant
	case "invalid_client", "unauthorized_client", "access_denied":
		return CodeAuth
	default:
		return ""
	}
}

func classifyHTTPError(status int, body []byte, method, path string, attempt int) error {
	switch googleAdsFailureCode(body) {
	case "USER_PERMISSION_DENIED":
		return newError(CodeUserPermissionDenied, status, method, path, attempt)
	case "DEVELOPER_TOKEN_NOT_APPROVED":
		return newError(CodeDeveloperTokenNotApproved, status, method, path, attempt)
	case "DEVELOPER_TOKEN_PROHIBITED":
		return newError(CodeDeveloperTokenProhibited, status, method, path, attempt)
	case "CUSTOMER_NOT_ENABLED":
		return newError(CodeCustomerNotEnabled, status, method, path, attempt)
	case "RESOURCE_EXHAUSTED":
		if containsDailyQuota(body) {
			return newError(CodeDailyQuota, status, method, path, attempt)
		}
		return newError(CodeRateLimit, status, method, path, attempt)
	case "UNSUPPORTED_VERSION", "UNIMPLEMENTED":
		return newError(CodeAPIVersion, status, method, path, attempt)
	case "UNAUTHENTICATED", "AUTHENTICATION_ERROR":
		return newError(CodeAuth, status, method, path, attempt)
	case "PERMISSION_DENIED":
		return newError(CodeAccessDenied, status, method, path, attempt)
	case "INVALID_CUSTOMER_ID", "INVALID_ARGUMENT":
		return newError(CodeInvalidRequest, status, method, path, attempt)
	}
	lower := strings.ToLower(string(body))
	switch {
	case status == http.StatusUnauthorized:
		if strings.Contains(lower, "invalid_grant") {
			return newError(CodeOAuthInvalidGrant, status, method, path, attempt)
		}
		return newError(CodeAuth, status, method, path, attempt)
	case status == http.StatusForbidden:
		if containsDailyQuota(body) {
			return newError(CodeDailyQuota, status, method, path, attempt)
		}
		return newError(CodeAccessDenied, status, method, path, attempt)
	case strings.Contains(lower, "unsupported version"), strings.Contains(lower, "version is not supported"):
		return newError(CodeAPIVersion, status, method, path, attempt)
	case containsDailyQuota(body):
		return newError(CodeDailyQuota, status, method, path, attempt)
	default:
		return newError(CodeUpstream, status, method, path, attempt)
	}
}

func googleAdsFailureCode(body []byte) string {
	text := strings.ToUpper(string(body))
	for _, code := range []string{
		"DEVELOPER_TOKEN_NOT_APPROVED",
		"DEVELOPER_TOKEN_PROHIBITED",
		"USER_PERMISSION_DENIED",
		"CUSTOMER_NOT_ENABLED",
		"UNSUPPORTED_VERSION",
		"UNIMPLEMENTED",
		"INVALID_CUSTOMER_ID",
		"AUTHENTICATION_ERROR",
		"UNAUTHENTICATED",
		"PERMISSION_DENIED",
		"INVALID_ARGUMENT",
		"RESOURCE_EXHAUSTED",
	} {
		if strings.Contains(text, code) {
			return code
		}
	}
	return ""
}

func isPermanentGoogleAdsCode(code string) bool {
	switch code {
	case "USER_PERMISSION_DENIED",
		"DEVELOPER_TOKEN_NOT_APPROVED",
		"DEVELOPER_TOKEN_PROHIBITED",
		"CUSTOMER_NOT_ENABLED",
		"UNSUPPORTED_VERSION",
		"UNIMPLEMENTED",
		"UNAUTHENTICATED",
		"AUTHENTICATION_ERROR",
		"PERMISSION_DENIED",
		"INVALID_CUSTOMER_ID",
		"INVALID_ARGUMENT":
		return true
	default:
		return false
	}
}

func containsDailyQuota(body []byte) bool {
	lower := strings.ToLower(string(body))
	return (strings.Contains(lower, "daily") && strings.Contains(lower, "quota")) ||
		strings.Contains(lower, "dailylimit") ||
		strings.Contains(lower, "daily_limit")
}

func rateLimitError(path string, attempt int, retryAfter time.Duration, code ErrorCode) error {
	safe := newError(code, http.StatusTooManyRequests, http.MethodPost, path, attempt)
	safe.RetryAfter = retryAfter
	return &cliutil.RateLimitError{
		URL:        path,
		RetryAfter: retryAfter,
		Cause:      safe,
	}
}

func parseRetryAfter(header http.Header) time.Duration {
	value := strings.TrimSpace(header.Get("Retry-After"))
	if value == "" {
		return 0
	}
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		if seconds <= 0 {
			return 0
		}
		wait := time.Duration(seconds) * time.Second
		if wait > cliutil.MaxRetryWait {
			return cliutil.MaxRetryWait
		}
		return wait
	}
	if date, err := http.ParseTime(value); err == nil {
		wait := time.Until(date)
		if wait <= 0 {
			return 0
		}
		if wait > cliutil.MaxRetryWait {
			return cliutil.MaxRetryWait
		}
		return wait
	}
	return 0
}

func (c *Client) retryDelay(attempt int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		return retryAfter
	}
	delay := c.cfg.RetryBase
	for i := 1; i < attempt; i++ {
		if delay >= 5*time.Second {
			return 5 * time.Second
		}
		delay *= 2
	}
	if delay > 5*time.Second {
		return 5 * time.Second
	}
	return delay
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Client) acquireTurn(ctx context.Context, customerID string) (func(), error) {
	if strings.TrimSpace(customerID) == "" {
		return func() {}, newError(CodeInvalidRequest, 0, http.MethodPost, "", 0)
	}
	if err := os.MkdirAll(c.cfg.RateLockDir, 0o700); err != nil {
		return func() {}, newError(CodeRateLock, 0, "", "", 0)
	}
	// The lock is an advisory file lock. The operating system releases it if
	// the process is interrupted, so a killed caller cannot leave a stale
	// directory that blocks every later request. The distinct suffix also
	// avoids directories left by older mkdir-based versions.
	lockPath, statePath := customerRateLockPaths(c.cfg.RateLockDir, customerID)

	err := cliutil.WithFileLock(lockPath, func() error {
		if c.cfg.MinRequestInterval > 0 {
			data, readErr := os.ReadFile(filepath.Clean(statePath)) // #nosec G304 -- path is derived from the app-owned rate-lock directory and hashed customer identifier.
			if readErr == nil {
				nanos, parseErr := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
				if parseErr != nil {
					return newError(CodeRateLock, 0, "", "", 0)
				}
				wait := c.cfg.MinRequestInterval - c.cfg.Now().Sub(time.Unix(0, nanos))
				if wait > 0 {
					if sleepErr := c.cfg.Sleep(ctx, wait); sleepErr != nil {
						return newError(CodeContext, 0, "", "", 0)
					}
				}
			} else if !errors.Is(readErr, os.ErrNotExist) {
				return newError(CodeRateLock, 0, "", "", 0)
			}
		}
		if err := c.limiter.Wait(ctx); err != nil {
			return newError(CodeContext, 0, "", "", 0)
		}
		now := c.cfg.Now()
		if err := writeRateState(statePath, now); err != nil {
			return newError(CodeRateLock, 0, "", "", 0)
		}
		return nil
	})
	if err != nil {
		return func() {}, err
	}
	// WithFileLock closes and unlocks before the HTTP request begins. The
	// state timestamp reserves this request's one-second slot while allowing
	// the next process to wait without serializing the full network round trip.
	return func() {}, nil
}

func customerRateLockPaths(directory, customerID string) (string, string) {
	sum := sha256.Sum256([]byte(customerID))
	prefix := hex.EncodeToString(sum[:8])
	base := filepath.Join(directory, "customer-"+prefix)
	return base + ".flock", base + ".last"
}

func writeRateState(path string, now time.Time) error {
	temp, err := os.CreateTemp(filepath.Dir(path), ".keyword-planner-rate-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() {
		_ = temp.Close()
		_ = os.Remove(tempPath)
	}()
	if err := temp.Chmod(0o600); err != nil {
		return err
	}
	if _, err := io.WriteString(temp, strconv.FormatInt(now.UnixNano(), 10)); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err == nil {
		return nil
	} else {
		// Windows cannot replace an existing file with Rename. Removal is
		// safe while the advisory lock is held; failure still fails closed.
		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return err
		}
		return os.Rename(tempPath, path)
	}
}
