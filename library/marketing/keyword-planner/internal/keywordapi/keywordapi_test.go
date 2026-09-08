package keywordapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/keyword-planner/internal/cliutil"
)

type testHandlers struct {
	token func(http.ResponseWriter, *http.Request)
	ads   func(http.ResponseWriter, *http.Request)
}

func newTestClient(t *testing.T, handlers testHandlers) (*Client, *httptest.Server, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	var tokenCalls atomic.Int32
	var adsCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			tokenCalls.Add(1)
			if handlers.token != nil {
				handlers.token(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, "{\"access_token\":\"access-token\",\"expires_in\":3600}")
		default:
			adsCalls.Add(1)
			if handlers.ads != nil {
				handlers.ads(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, "{\"ok\":true}")
		}
	}))
	t.Cleanup(server.Close)

	cfg := Config{
		ClientID:           "client-id",
		ClientSecret:       "client-secret",
		RefreshToken:       "refresh-token",
		DeveloperToken:     "developer-token",
		CustomerID:         "1234567890",
		LoginCustomerID:    "0987654321",
		BaseURL:            server.URL,
		TokenURL:           server.URL + "/token",
		HTTPClient:         server.Client(),
		RateLockDir:        t.TempDir(),
		MinRequestInterval: time.Nanosecond,
		Limiter:            cliutil.NewAdaptiveLimiter(1000),
		MaxAttempts:        3,
		RetryBase:          time.Nanosecond,
		Timeout:            3 * time.Second,
		Sleep:              func(context.Context, time.Duration) error { return nil },
	}
	return NewClient(cfg), server, &tokenCalls, &adsCalls
}

func TestLoadConfigFileAndEnvironmentOverride(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), "credentials.env")
	content := strings.Join([]string{
		"GOOGLE_ADS_CLIENT_ID=file-client",
		"GOOGLE_ADS_CLIENT_SECRET=file-secret",
		"GOOGLE_ADS_REFRESH_TOKEN=file-refresh",
		"GOOGLE_ADS_DEVELOPER_TOKEN=file-developer",
		"GOOGLE_ADS_LOGIN_CUSTOMER_ID=file-login",
		"GOOGLE_ADS_CUSTOMER_ID=file-customer",
		"UNRELATED_SHOULD_BE_IGNORED=do-not-load",
	}, "\n")
	if err := os.WriteFile(envPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOOGLE_ADS_CLIENT_ID", "env-client")
	t.Setenv("GOOGLE_ADS_CUSTOMER_ID", "env-customer")

	cfg, err := LoadConfig(envPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ClientID != "env-client" || cfg.CustomerID != "env-customer" {
		t.Fatalf("environment override not applied: client=%q customer=%q", cfg.ClientID, cfg.CustomerID)
	}
	if cfg.ClientSecret != "file-secret" || cfg.RefreshToken != "file-refresh" ||
		cfg.DeveloperToken != "file-developer" || cfg.LoginCustomerID != "file-login" {
		t.Fatalf("selected env file values not loaded: %#v", cfg)
	}
}

func TestLoadConfigUsesEnvironmentFileFallback(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), "fallback.env")
	if err := os.WriteFile(envPath, []byte(strings.Join([]string{
		"GOOGLE_ADS_CLIENT_ID=fallback-client",
		"GOOGLE_ADS_CLIENT_SECRET=fallback-secret",
		"GOOGLE_ADS_REFRESH_TOKEN=fallback-refresh",
		"GOOGLE_ADS_DEVELOPER_TOKEN=fallback-developer",
		"GOOGLE_ADS_CUSTOMER_ID=1234567890",
	}, "\n")), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(envFileEnv, envPath)
	for _, key := range configEnvNames {
		oldValue, wasSet := os.LookupEnv(key)
		_ = os.Unsetenv(key)
		t.Cleanup(func() {
			if wasSet {
				_ = os.Setenv(key, oldValue)
			} else {
				_ = os.Unsetenv(key)
			}
		})
	}
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.EnvPath != envPath || cfg.ClientID != "fallback-client" ||
		cfg.RefreshToken != "fallback-refresh" || cfg.CustomerID != "1234567890" {
		t.Fatalf("fallback config = %#v", cfg)
	}
}

func TestLoadConfigRequiresProtectedRegularEnvFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mode 0644 is not a portable Windows ACL fixture")
	}
	clearConfigEnvForTest(t)
	dir := t.TempDir()
	content := []byte("GOOGLE_ADS_CLIENT_ID=file-client\n")
	safePath := filepath.Join(dir, "safe.env")
	if err := os.WriteFile(safePath, content, 0o600); err != nil {
		t.Fatal(err)
	}
	unsafePath := filepath.Join(dir, "unsafe.env")
	if err := os.WriteFile(unsafePath, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(unsafePath, 0o644); err != nil {
		t.Fatal(err)
	}
	safeLink := filepath.Join(dir, "safe-link.env")
	if err := os.Symlink(safePath, safeLink); err != nil {
		t.Fatal(err)
	}
	unsafeLink := filepath.Join(dir, "unsafe-link.env")
	if err := os.Symlink(unsafePath, unsafeLink); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		path      string
		wantError bool
	}{
		{name: "0600 regular file", path: safePath},
		{name: "0600 symlink target", path: safeLink},
		{name: "0644 regular file", path: unsafePath, wantError: true},
		{name: "0644 symlink target", path: unsafeLink, wantError: true},
		{name: "directory", path: dir, wantError: true},
		{name: "explicit missing", path: filepath.Join(dir, "missing.env"), wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg, err := LoadConfig(test.path)
			if test.wantError {
				if CodeOf(err) != CodeConfigFile {
					t.Fatalf("error code=%q err=%v, want %q", CodeOf(err), err, CodeConfigFile)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.EnvPath != test.path || cfg.ClientID != "file-client" {
				t.Fatalf("config=%#v", cfg)
			}
		})
	}
}

func TestLoadConfigAllowsMissingImplicitDefaultForEnvironmentOnly(t *testing.T) {
	clearConfigEnvForTest(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(envFileEnv, "")
	t.Setenv("GOOGLE_ADS_CLIENT_ID", "environment-client")

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.EnvPath != filepath.Join(home, ".env") || cfg.ClientID != "environment-client" {
		t.Fatalf("config=%#v", cfg)
	}
}

func TestLoadConfigWithDefaultPreservesImplicitAndExplicitPaths(t *testing.T) {
	clearConfigEnvForTest(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(envFileEnv, "")
	t.Setenv("GOOGLE_ADS_CLIENT_ID", "environment-client")
	fallback := filepath.Join(t.TempDir(), ".env")
	// A real HOME file must not be read when the caller chooses a fallback.
	if err := os.WriteFile(filepath.Join(home, ".env"), []byte("malformed"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfigWithDefault("", fallback)
	if err != nil || cfg.EnvPath != fallback || cfg.ClientID != "environment-client" {
		t.Fatalf("isolated implicit default: path=%q error=%v", cfg.EnvPath, err)
	}
	if _, err := LoadConfigWithDefault(fallback, filepath.Join(home, ".env")); err == nil {
		t.Fatal("explicit missing path was treated as optional")
	}
	t.Setenv(envFileEnv, fallback)
	if _, err := LoadConfigWithDefault("", filepath.Join(home, ".env")); err == nil {
		t.Fatal("environment-selected missing path was treated as optional")
	}
}

func TestCallExactPathHeadersAndNestedBody(t *testing.T) {
	wantBody := []byte("{\"keywordSeed\":{\"keywords\":[\"alpha\"],\"nested\":{\"limit\":2}}}")
	client, _, _, _ := newTestClient(t, testHandlers{
		ads: func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %s, want POST", r.Method)
			}
			if r.URL.Path != "/v25/customers/1234567890:generateKeywordIdeas" {
				t.Errorf("path = %s", r.URL.Path)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer access-token" {
				t.Errorf("authorization = %q", got)
			}
			if got := r.Header.Get("developer-token"); got != "developer-token" {
				t.Errorf("developer-token = %q", got)
			}
			if got := r.Header.Get("login-customer-id"); got != "0987654321" {
				t.Errorf("login-customer-id = %q", got)
			}
			got, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("read body: %v", err)
			}
			if !bytes.Equal(got, wantBody) {
				t.Errorf("body = %q, want %q", got, wantBody)
			}
			w.Header().Set("request-id", "request-1")
			_, _ = io.WriteString(w, "{\"results\":[]}")
		},
	})
	resp, err := client.Call(context.Background(), http.MethodPost,
		"/v25/customers/1234567890:generateKeywordIdeas", wantBody, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != http.StatusOK || resp.StatusCode != http.StatusOK ||
		resp.RequestID != "request-1" || !bytes.Equal(resp.Body, []byte("{\"results\":[]}")) {
		t.Fatalf("response receipt mismatch: %#v", resp)
	}
}

func TestPlannerPathMustMatchConfiguredCustomerAndUsesWireID(t *testing.T) {
	client, _, _, adsCalls := newTestClient(t, testHandlers{
		ads: func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v25/customers/1234567890:generateKeywordIdeas" {
				t.Errorf("wire path = %s", r.URL.Path)
			}
			_, _ = io.WriteString(w, "{\"ok\":true}")
		},
	})
	client.cfg.CustomerID = "123-456-7890"
	if _, err := client.Call(context.Background(), http.MethodPost,
		"/v25/customers/123-456-7890:generateKeywordIdeas", []byte("{}"), nil); err != nil {
		t.Fatal(err)
	}
	client.cfg.CustomerID = "9999999999"
	_, err := client.Call(context.Background(), http.MethodPost,
		"/v25/customers/1234567890:generateKeywordIdeas", []byte("{}"), nil)
	if CodeOf(err) != CodeInvalidRequest || adsCalls.Load() != 1 {
		t.Fatalf("customer mismatch code=%q calls=%d err=%v", CodeOf(err), adsCalls.Load(), err)
	}
}

func TestCallCallbackSeesRawBodyBeforeRetryClassification(t *testing.T) {
	var calls atomic.Int32
	client, _, _, _ := newTestClient(t, testHandlers{
		ads: func(w http.ResponseWriter, r *http.Request) {
			if calls.Add(1) == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = io.WriteString(w, "{\"unknown\":\"first-body\"}")
				return
			}
			_, _ = io.WriteString(w, "{\"unknown\":\"second-body\"}")
		},
	})
	var attempts []int
	var statuses []int
	var bodies []string
	resp, err := client.Call(context.Background(), http.MethodPost,
		"/v25/customers/1234567890:generateKeywordIdeas", []byte("{\"keywordSeed\":{\"keywords\":[\"alpha\"]}}"),
		func(receipt Response) error {
			attempts = append(attempts, receipt.Attempt)
			statuses = append(statuses, receipt.Status)
			bodies = append(bodies, string(receipt.Body))
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(attempts, []int{1, 2}) ||
		!reflect.DeepEqual(statuses, []int{500, 200}) ||
		!reflect.DeepEqual(bodies, []string{"{\"unknown\":\"first-body\"}", "{\"unknown\":\"second-body\"}"}) {
		t.Fatalf("callback receipts = %#v %#v %#v", attempts, statuses, bodies)
	}
	if resp.Attempt != 2 {
		t.Fatalf("final attempt = %d", resp.Attempt)
	}
}

func TestCallbackBodyIsolatedFromReturnedReceipt(t *testing.T) {
	client, _, _, _ := newTestClient(t, testHandlers{
		ads: func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, "{\"original\":true}")
		},
	})
	resp, err := client.Call(context.Background(), http.MethodPost,
		"/v25/customers/1234567890:generateKeywordIdeas", []byte("{}"),
		func(receipt Response) error {
			receipt.Body[2] = 'X'
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if string(resp.Body) != "{\"original\":true}" {
		t.Fatalf("callback mutated returned body: %q", resp.Body)
	}
}

func TestAccountReturnsCurrencyAndProvenanceAfterCallback(t *testing.T) {
	client, _, _, _ := newTestClient(t, testHandlers{
		ads: func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v25/customers/1234567890/googleAds:search" {
				t.Errorf("account path = %s", r.URL.Path)
			}
			var body struct {
				Query string `json:"query"`
			}
			raw, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(raw, &body); err != nil {
				t.Errorf("account body: %v", err)
			}
			if body.Query != "SELECT customer.id, customer.currency_code FROM customer LIMIT 1" {
				t.Errorf("query = %q", body.Query)
			}
			_, _ = io.WriteString(w, "{\"results\":[{\"customer\":{\"id\":\"1234567890\",\"currencyCode\":\"EUR\"}}]}")
		},
	})
	var callbacks int
	info, err := client.Account(context.Background(), func(receipt Response) error {
		callbacks++
		if receipt.Status != http.StatusOK {
			t.Errorf("callback status = %d", receipt.Status)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if callbacks != 1 || info.CustomerID != "1234567890" ||
		info.CurrencyCode != "EUR" || info.Provenance != "google-ads.customer.currency_code" {
		t.Fatalf("account result = %#v callbacks=%d", info, callbacks)
	}
}

func TestAccountNormalizesHyphenatedIDsAndAdsHeader(t *testing.T) {
	client, _, _, _ := newTestClient(t, testHandlers{
		ads: func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v25/customers/1234567890/googleAds:search" {
				t.Errorf("account path = %s", r.URL.Path)
			}
			if got := r.Header.Get("login-customer-id"); got != "0987654321" {
				t.Errorf("login-customer-id = %q", got)
			}
			_, _ = io.WriteString(w, "{\"results\":[{\"customer\":{\"id\":\"1234567890\",\"currencyCode\":\"EUR\"}}]}")
		},
	})
	client.cfg.CustomerID = "123-456-7890"
	client.cfg.LoginCustomerID = "098-765-4321"

	info, err := client.Account(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if info.CustomerID != "1234567890" || info.CurrencyCode != "EUR" {
		t.Fatalf("account result = %#v", info)
	}
}

func TestMalformedAccountCallsCallbackBeforeDecodeError(t *testing.T) {
	client, _, _, _ := newTestClient(t, testHandlers{
		ads: func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, "{\"results\":[{\"customer\":{}}]}")
		},
	})
	var callbackBody string
	_, err := client.Account(context.Background(), func(receipt Response) error {
		callbackBody = string(receipt.Body)
		return nil
	})
	if CodeOf(err) != CodeResponseMalformed {
		t.Fatalf("code = %q, err=%v", CodeOf(err), err)
	}
	if callbackBody != "{\"results\":[{\"customer\":{}}]}" {
		t.Fatalf("callback body = %q", callbackBody)
	}
}

func TestAccountRequiresMatchingIDAndUppercaseCurrency(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "missing id", body: "{\"results\":[{\"customer\":{\"currencyCode\":\"EUR\"}}]}"},
		{name: "different id", body: "{\"results\":[{\"customer\":{\"id\":\"9999999999\",\"currencyCode\":\"EUR\"}}]}"},
		{name: "lowercase currency", body: "{\"results\":[{\"customer\":{\"id\":\"1234567890\",\"currencyCode\":\"eur\"}}]}"},
		{name: "wrong currency length", body: "{\"results\":[{\"customer\":{\"id\":\"1234567890\",\"currencyCode\":\"EURO\"}}]}"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, _, _, _ := newTestClient(t, testHandlers{
				ads: func(w http.ResponseWriter, r *http.Request) {
					_, _ = io.WriteString(w, test.body)
				},
			})
			_, err := client.Account(context.Background(), nil)
			if CodeOf(err) != CodeResponseMalformed {
				t.Fatalf("code=%q err=%v", CodeOf(err), err)
			}
		})
	}
}

func TestNoResponseFailureCallsCallback(t *testing.T) {
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/token" {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("{\"access_token\":\"access-token\",\"expires_in\":3600}")),
				Request:    req,
			}, nil
		}
		return nil, errors.New("network transport failure")
	})
	client := NewClient(Config{
		ClientID:           "client-id",
		ClientSecret:       "client-secret",
		RefreshToken:       "refresh-token",
		DeveloperToken:     "developer-token",
		CustomerID:         "1234567890",
		BaseURL:            "http://example.invalid",
		TokenURL:           "http://example.invalid/token",
		HTTPClient:         &http.Client{Transport: transport},
		RateLockDir:        t.TempDir(),
		MinRequestInterval: time.Nanosecond,
		Limiter:            cliutil.NewAdaptiveLimiter(1000),
		MaxAttempts:        1,
		Timeout:            time.Second,
	})
	var receipt Response
	got, err := client.Call(context.Background(), http.MethodPost,
		"/v25/customers/1234567890:generateKeywordIdeas", []byte("{}"),
		func(value Response) error {
			receipt = value
			return nil
		})
	if CodeOf(err) != CodeTransport || receipt.Status != 0 || receipt.Attempt != 1 ||
		receipt.Err == nil || got.Attempt != 1 {
		t.Fatalf("transport result=%#v receipt=%#v err=%v", got, receipt, err)
	}
}

func TestInvalidGrantIsPermanentAndRedacted(t *testing.T) {
	const secretBody = "secret-body-value"
	client, _, tokenCalls, adsCalls := newTestClient(t, testHandlers{
		token: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, "{\"error\":\"invalid_grant\",\"error_description\":\""+secretBody+"\"}")
		},
	})
	_, err := client.Call(context.Background(), http.MethodPost,
		"/v25/customers/1234567890:generateKeywordIdeas", []byte("{}"), nil)
	if CodeOf(err) != CodeOAuthInvalidGrant || ExitCodeOf(err) != 3 {
		t.Fatalf("invalid grant result code=%q exit=%d err=%v", CodeOf(err), ExitCodeOf(err), err)
	}
	if tokenCalls.Load() != 1 || adsCalls.Load() != 0 {
		t.Fatalf("token calls=%d ads calls=%d", tokenCalls.Load(), adsCalls.Load())
	}
	assertErrorDoesNotContain(t, err, secretBody)
}

func TestOAuthRedirectsAreNotFollowedOrPersisted(t *testing.T) {
	for _, status := range []int{http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			var destinationCalls atomic.Int32
			destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				destinationCalls.Add(1)
				w.WriteHeader(http.StatusTeapot)
			}))
			t.Cleanup(destination.Close)

			var sourceCalls atomic.Int32
			const responseBody = "oauth-redirect-response"
			source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				sourceCalls.Add(1)
				if r.URL.Path != "/token" {
					t.Errorf("OAuth path = %s", r.URL.Path)
				}
				w.Header().Set("Location", destination.URL+"/token-target")
				w.WriteHeader(status)
				_, _ = io.WriteString(w, responseBody)
			}))
			t.Cleanup(source.Close)

			ledgerPath := filepath.Join(t.TempDir(), "oauth-ledger.jsonl")
			t.Setenv("KEYWORD_PLANNER_CALL_LEDGER", ledgerPath)
			injected := source.Client()
			client := NewClient(Config{
				ClientID:           "client-id",
				ClientSecret:       "client-secret",
				RefreshToken:       "refresh-token",
				DeveloperToken:     "developer-token",
				CustomerID:         "1234567890",
				BaseURL:            source.URL,
				TokenURL:           source.URL + "/token",
				HTTPClient:         injected,
				RateLockDir:        t.TempDir(),
				MinRequestInterval: time.Nanosecond,
				Limiter:            cliutil.NewAdaptiveLimiter(1000),
				MaxAttempts:        1,
				Timeout:            time.Second,
			})
			if injected.CheckRedirect != nil || client.httpClient == injected {
				t.Fatal("NewClient mutated or reused the injected HTTP client")
			}

			_, err := client.Call(context.Background(), http.MethodPost,
				"/v25/customers/1234567890:generateKeywordIdeas", []byte("{}"), nil)
			if CodeOf(err) != CodeOAuthEndpoint {
				t.Fatalf("status=%d code=%q err=%v", status, CodeOf(err), err)
			}
			if sourceCalls.Load() != 1 || destinationCalls.Load() != 0 {
				t.Fatalf("status=%d source calls=%d destination calls=%d", status, sourceCalls.Load(), destinationCalls.Load())
			}
			if _, statErr := os.Stat(ledgerPath); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("OAuth response was persisted: stat error=%v", statErr)
			}
			assertErrorDoesNotContain(t, err, responseBody)
		})
	}
}

func TestAdsRedirectsPreserveRaw3xxReceiptWithoutFollowing(t *testing.T) {
	for _, status := range []int{http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			var destinationCalls atomic.Int32
			destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				destinationCalls.Add(1)
			}))
			t.Cleanup(destination.Close)

			var sourceCalls atomic.Int32
			responseBody := []byte(`{"redirect":true,"status":` + strconv.Itoa(status) + `}`)
			source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				sourceCalls.Add(1)
				if r.URL.Path != "/v25/customers/1234567890:generateKeywordIdeas" {
					t.Errorf("Ads path = %s", r.URL.Path)
				}
				w.Header().Set("Location", destination.URL+"/ads-target")
				w.WriteHeader(status)
				_, _ = w.Write(responseBody)
			}))
			t.Cleanup(source.Close)

			injected := source.Client()
			client := NewClient(Config{
				AccessToken:        "access-token",
				DeveloperToken:     "developer-token",
				CustomerID:         "1234567890",
				BaseURL:            source.URL,
				HTTPClient:         injected,
				RateLockDir:        t.TempDir(),
				MinRequestInterval: time.Nanosecond,
				Limiter:            cliutil.NewAdaptiveLimiter(1000),
				MaxAttempts:        1,
				Timeout:            time.Second,
			})
			if injected.CheckRedirect != nil || client.httpClient == injected {
				t.Fatal("NewClient mutated or reused the injected HTTP client")
			}

			var callback Response
			response, err := client.Call(context.Background(), http.MethodPost,
				"/v25/customers/1234567890:generateKeywordIdeas", []byte("{}"),
				func(receipt Response) error {
					callback = receipt
					return nil
				})
			if CodeOf(err) != CodeUpstream {
				t.Fatalf("status=%d code=%q err=%v", status, CodeOf(err), err)
			}
			if sourceCalls.Load() != 1 || destinationCalls.Load() != 0 {
				t.Fatalf("status=%d source calls=%d destination calls=%d", status, sourceCalls.Load(), destinationCalls.Load())
			}
			if response.Status != status || response.StatusCode != status || !bytes.Equal(response.Body, responseBody) {
				t.Fatalf("returned receipt = %#v", response)
			}
			if callback.Status != status || !bytes.Equal(callback.Body, responseBody) {
				t.Fatalf("callback receipt = %#v", callback)
			}
		})
	}
}

func TestZeroAccessTokenBootstrapsRefreshOnly(t *testing.T) {
	client, _, tokenCalls, _ := newTestClient(t, testHandlers{
		ads: func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("Authorization"); got != "Bearer access-token" {
				t.Errorf("authorization = %q", got)
			}
			_, _ = io.WriteString(w, "{\"ok\":true}")
		},
	})
	client.cfg.AccessToken = ""
	client.accessToken = ""
	_, err := client.Call(context.Background(), http.MethodPost,
		"/v25/customers/1234567890:generateKeywordIdeas", []byte("{}"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if tokenCalls.Load() != 1 {
		t.Fatalf("token calls = %d", tokenCalls.Load())
	}
}

func TestRateLimitRetriesBoundedAndReturnsTypedError(t *testing.T) {
	const secretBody = "{\"error\":\"secret-body-value\"}"
	client, _, _, adsCalls := newTestClient(t, testHandlers{
		ads: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", "7")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, secretBody)
		},
	})
	var waits []time.Duration
	client.cfg.Sleep = func(_ context.Context, delay time.Duration) error {
		waits = append(waits, delay)
		return nil
	}
	_, err := client.Call(context.Background(), http.MethodPost,
		"/v25/customers/1234567890:generateKeywordIdeas", []byte("{}"), nil)
	var rateErr *cliutil.RateLimitError
	if !errors.As(err, &rateErr) || CodeOf(err) != ErrorCode("rate_limit") ||
		ExitCodeOf(err) != 5 || adsCalls.Load() != 3 {
		t.Fatalf("rate result calls=%d waits=%v code=%q exit=%d err=%v", adsCalls.Load(), waits, CodeOf(err), ExitCodeOf(err), err)
	}
	if len(waits) != 2 || rateErr.RetryAfter != 7*time.Second {
		t.Fatalf("retry waits=%v retry-after=%s", waits, rateErr.RetryAfter)
	}
	assertErrorDoesNotContain(t, err, "secret-body-value")
}

func TestFiveHundredRetriesBounded(t *testing.T) {
	client, _, _, adsCalls := newTestClient(t, testHandlers{
		ads: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = io.WriteString(w, "{\"error\":\"secret-body-value\"}")
		},
	})
	_, err := client.Call(context.Background(), http.MethodPost,
		"/v25/customers/1234567890:generateKeywordIdeas", []byte("{}"), nil)
	if CodeOf(err) != CodeUpstream5xx || adsCalls.Load() != 3 {
		t.Fatalf("500 result calls=%d code=%q err=%v", adsCalls.Load(), CodeOf(err), err)
	}
	assertErrorDoesNotContain(t, err, "secret-body-value")
}

func TestPermanentErrorsDoNotRetry(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     string
		wantCode ErrorCode
		wantRate bool
	}{
		{name: "unauthorized", status: http.StatusUnauthorized, body: "{\"error\":\"access\"}", wantCode: CodeAuth},
		{name: "forbidden", status: http.StatusForbidden, body: "{\"error\":\"permission\"}", wantCode: CodeAccessDenied},
		{name: "version", status: http.StatusBadRequest, body: "{\"error\":\"unsupported version\"}", wantCode: CodeAPIVersion},
		{name: "daily quota", status: http.StatusTooManyRequests, body: "{\"error\":\"daily quota exceeded\"}", wantCode: CodeDailyQuota, wantRate: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, _, _, adsCalls := newTestClient(t, testHandlers{
				ads: func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(test.status)
					_, _ = io.WriteString(w, test.body)
				},
			})
			_, err := client.Call(context.Background(), http.MethodPost,
				"/v25/customers/1234567890:generateKeywordIdeas", []byte("{}"), nil)
			if adsCalls.Load() != 1 || CodeOf(err) != test.wantCode {
				t.Fatalf("calls=%d code=%q err=%v", adsCalls.Load(), CodeOf(err), err)
			}
			if test.wantRate {
				var rateErr *cliutil.RateLimitError
				if !errors.As(err, &rateErr) {
					t.Fatalf("daily quota error type=%T", err)
				}
			}
		})
	}
}

func TestTwoClientsShareCustomerPacing(t *testing.T) {
	var mu sync.Mutex
	var times []time.Time
	clientOne, _, _, _ := newTestClient(t, testHandlers{
		ads: func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			times = append(times, time.Now())
			mu.Unlock()
			_, _ = io.WriteString(w, "{\"ok\":true}")
		},
	})
	clientTwo, _, _, _ := newTestClient(t, testHandlers{
		ads: func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			times = append(times, time.Now())
			mu.Unlock()
			_, _ = io.WriteString(w, "{\"ok\":true}")
		},
	})
	sharedDir := t.TempDir()
	clientOne.cfg.RateLockDir = sharedDir
	clientTwo.cfg.RateLockDir = sharedDir
	clientOne.cfg.MinRequestInterval = 30 * time.Millisecond
	clientTwo.cfg.MinRequestInterval = 30 * time.Millisecond
	clientOne.cfg.Sleep = sleepContext
	clientTwo.cfg.Sleep = sleepContext
	clientOne.cfg.MaxAttempts = 1
	clientTwo.cfg.MaxAttempts = 1
	if _, err := clientOne.Call(context.Background(), http.MethodPost,
		"/v25/customers/1234567890:generateKeywordIdeas", []byte("{}"), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := clientTwo.Call(context.Background(), http.MethodPost,
		"/v25/customers/1234567890:generateKeywordIdeas", []byte("{}"), nil); err != nil {
		t.Fatal(err)
	}
	if len(times) != 2 {
		t.Fatalf("request times=%d", len(times))
	}
	if delta := times[1].Sub(times[0]); delta < 25*time.Millisecond {
		t.Fatalf("shared customer spacing=%s", delta)
	}
}

func TestKeywordAPILockHelper(t *testing.T) {
	if os.Getenv("KEYWORD_API_LOCK_HELPER") != "1" {
		return
	}
	lockPath := os.Getenv("KEYWORD_API_LOCK_PATH")
	readyPath := os.Getenv("KEYWORD_API_LOCK_READY")
	if lockPath == "" || readyPath == "" {
		os.Exit(2)
	}
	err := cliutil.WithFileLock(lockPath, func() error {
		if err := os.WriteFile(readyPath, []byte("ready"), 0o600); err != nil {
			return err
		}
		_, _ = io.Copy(io.Discard, os.Stdin)
		return nil
	})
	if err != nil {
		os.Exit(3)
	}
}

func TestRateLockSurvivesOwnerProcessExit(t *testing.T) {
	customerID := "1234567890"
	rateDir := t.TempDir()
	lockPath, statePath := customerRateLockPaths(rateDir, customerID)
	client, _, _, _ := newTestClient(t, testHandlers{})
	client.cfg.RateLockDir = rateDir
	client.cfg.MinRequestInterval = 120 * time.Millisecond
	client.cfg.Sleep = sleepContext
	client.cfg.MaxAttempts = 1
	if _, err := client.Call(context.Background(), http.MethodPost,
		"/v25/customers/1234567890:generateKeywordIdeas", []byte("{}"), nil); err != nil {
		t.Fatal(err)
	}

	readyPath := filepath.Join(rateDir, "helper.ready")
	command := exec.Command(os.Args[0], "-test.run=TestKeywordAPILockHelper", "-test.count=1")
	command.Env = append(os.Environ(),
		"KEYWORD_API_LOCK_HELPER=1",
		"KEYWORD_API_LOCK_PATH="+lockPath,
		"KEYWORD_API_LOCK_READY="+readyPath,
	)
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	waitDone := make(chan error, 1)
	go func() { waitDone <- command.Wait() }()
	helperExited := false
	t.Cleanup(func() {
		if !helperExited {
			_ = command.Process.Kill()
			_ = stdin.Close()
			<-waitDone
		}
	})

	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, err := os.Stat(readyPath); err == nil {
			break
		}
		select {
		case err := <-waitDone:
			helperExited = true
			t.Fatalf("lock helper exited before acquiring lock: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			_ = command.Process.Kill()
			_ = stdin.Close()
			helperExited = true
			<-waitDone
			t.Fatal("timed out waiting for lock helper")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err := os.WriteFile(statePath,
		[]byte(strconv.FormatInt(time.Now().UnixNano(), 10)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := command.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = stdin.Close()
	helperExited = true
	if err := <-waitDone; err == nil {
		t.Fatal("lock helper was expected to be interrupted")
	}

	start := time.Now()
	if _, err := client.Call(context.Background(), http.MethodPost,
		"/v25/customers/1234567890:generateKeywordIdeas", []byte("{}"), nil); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed < 90*time.Millisecond {
		t.Fatalf("subsequent caller bypassed retained pacing after owner exit: %s", elapsed)
	}
}

func TestAllowlistRejectsArbitraryPathAndMutation(t *testing.T) {
	client, _, _, adsCalls := newTestClient(t, testHandlers{})
	_, err := client.Call(context.Background(), http.MethodDelete,
		"/v25/customers/1234567890/campaigns", nil, nil)
	if CodeOf(err) != CodeUnsupportedPath || adsCalls.Load() != 0 {
		t.Fatalf("arbitrary path code=%q calls=%d err=%v", CodeOf(err), adsCalls.Load(), err)
	}
	_, err = client.Call(context.Background(), http.MethodPost,
		"/v25/customers/1234567890/googleAds:search",
		[]byte("{\"query\":\"UPDATE campaign SET status = 'PAUSED'\"}"), nil)
	if CodeOf(err) != CodeInvalidRequest || adsCalls.Load() != 0 {
		t.Fatalf("mutation query code=%q calls=%d err=%v", CodeOf(err), adsCalls.Load(), err)
	}
	_, err = client.Call(context.Background(), http.MethodPost,
		"/v25/customers/1234567890/googleAds:search",
		[]byte("{\"query\":\"SELECT customer.id FROM customer LIMIT 1\"}"), nil)
	if CodeOf(err) != CodeInvalidRequest || adsCalls.Load() != 0 {
		t.Fatalf("non-approved account query code=%q calls=%d err=%v", CodeOf(err), adsCalls.Load(), err)
	}
}

func TestGoogleAdsFailureClassificationIsPermanentAndActionable(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		wantCode  ErrorCode
		wantRate  bool
		wantCalls int32
	}{
		{
			name:      "user permission",
			status:    http.StatusForbidden,
			body:      "{\"error\":{\"status\":\"PERMISSION_DENIED\",\"details\":[{\"errors\":[{\"errorCode\":{\"authorizationError\":\"USER_PERMISSION_DENIED\"}}]}]}}",
			wantCode:  CodeUserPermissionDenied,
			wantCalls: 1,
		},
		{
			name:      "developer token not approved",
			status:    http.StatusForbidden,
			body:      "{\"error\":{\"details\":[{\"errors\":[{\"errorCode\":{\"authorizationError\":\"DEVELOPER_TOKEN_NOT_APPROVED\"}}]}]}}",
			wantCode:  CodeDeveloperTokenNotApproved,
			wantCalls: 1,
		},
		{
			name:      "developer token prohibited",
			status:    http.StatusForbidden,
			body:      "{\"error\":{\"details\":[{\"errors\":[{\"errorCode\":{\"authorizationError\":\"DEVELOPER_TOKEN_PROHIBITED\"}}]}]}}",
			wantCode:  CodeDeveloperTokenProhibited,
			wantCalls: 1,
		},
		{
			name:      "customer not enabled",
			status:    http.StatusForbidden,
			body:      "{\"error\":{\"details\":[{\"errors\":[{\"errorCode\":{\"authorizationError\":\"CUSTOMER_NOT_ENABLED\"}}]}]}}",
			wantCode:  CodeCustomerNotEnabled,
			wantCalls: 1,
		},
		{
			name:      "daily resource exhausted",
			status:    http.StatusTooManyRequests,
			body:      "{\"error\":{\"status\":\"RESOURCE_EXHAUSTED\",\"message\":\"daily quota exhausted\"}}",
			wantCode:  CodeDailyQuota,
			wantRate:  true,
			wantCalls: 1,
		},
		{
			name:      "transient resource exhausted",
			status:    http.StatusTooManyRequests,
			body:      "{\"error\":{\"status\":\"RESOURCE_EXHAUSTED\",\"message\":\"temporary service exhaustion\"}}",
			wantCode:  CodeRateLimit,
			wantRate:  true,
			wantCalls: 3,
		},
		{
			name:      "unsupported version",
			status:    http.StatusBadRequest,
			body:      "{\"error\":{\"status\":\"UNSUPPORTED_VERSION\",\"message\":\"retired API version\"}}",
			wantCode:  CodeAPIVersion,
			wantCalls: 1,
		},
		{
			name:      "unimplemented",
			status:    http.StatusNotImplemented,
			body:      "{\"error\":{\"status\":\"UNIMPLEMENTED\",\"message\":\"method unavailable\"}}",
			wantCode:  CodeAPIVersion,
			wantCalls: 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, _, _, adsCalls := newTestClient(t, testHandlers{
				ads: func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(test.status)
					_, _ = io.WriteString(w, test.body)
				},
			})
			_, err := client.Call(context.Background(), http.MethodPost,
				"/v25/customers/1234567890:generateKeywordIdeas", []byte("{}"), nil)
			if CodeOf(err) != test.wantCode || adsCalls.Load() != test.wantCalls {
				t.Fatalf("calls=%d code=%q want=%q err=%v", adsCalls.Load(), CodeOf(err), test.wantCode, err)
			}
			if test.wantRate {
				var rateErr *cliutil.RateLimitError
				if !errors.As(err, &rateErr) {
					t.Fatalf("rate error type=%T", err)
				}
			}
			if test.name == "user permission" {
				message := err.Error()
				for _, fragment := range []string{"API v25", "Discovery 20260831", "transport REST", "OAuth user's access"} {
					if !strings.Contains(message, fragment) {
						t.Fatalf("diagnostic %q missing %q", message, fragment)
					}
				}
			}
		})
	}
}

func TestErrorDoesNotRetainUntrustedBody(t *testing.T) {
	const secretBody = "upstream-secret-body-value"
	client, _, _, _ := newTestClient(t, testHandlers{
		ads: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, secretBody)
		},
	})
	_, err := client.Call(context.Background(), http.MethodPost,
		"/v25/customers/1234567890:generateKeywordIdeas", []byte("{}"), nil)
	if err == nil {
		t.Fatal("expected error")
	}
	assertErrorDoesNotContain(t, err, secretBody)
}

func TestCredentialEchoWithholdsBodyAndReportsSafeFailure(t *testing.T) {
	client, _, _, _ := newTestClient(t, testHandlers{
		ads: func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, "{\"error\":\"client-secret\"}")
		},
	})
	var callback Response
	resp, err := client.Call(context.Background(), http.MethodPost,
		"/v25/customers/1234567890:generateKeywordIdeas", []byte("{}"),
		func(receipt Response) error {
			callback = receipt
			return nil
		})
	if CodeOf(err) != CodeCredentialEcho || ExitCodeOf(err) != 7 {
		t.Fatalf("code=%q exit=%d err=%v", CodeOf(err), ExitCodeOf(err), err)
	}
	if len(resp.Body) != 0 || !resp.BodyWithheld || resp.FailureCode != CodeCredentialEcho {
		t.Fatalf("returned receipt retained body: %#v", resp)
	}
	if len(callback.Body) != 0 || !callback.BodyWithheld ||
		callback.FailureCode != CodeCredentialEcho {
		t.Fatalf("callback receipt retained body: %#v", callback)
	}
	assertErrorDoesNotContain(t, err, "client-secret")
	assertErrorDoesNotContain(t, callback.Err, "client-secret")
}

func TestCallLedgerContainsOnlyAttemptMetadata(t *testing.T) {
	ledgerPath := filepath.Join(t.TempDir(), "attempts.jsonl")
	t.Setenv("KEYWORD_PLANNER_CALL_LEDGER", ledgerPath)
	client, _, _, _ := newTestClient(t, testHandlers{})
	if _, err := client.Call(context.Background(), http.MethodPost,
		"/v25/customers/1234567890:generateKeywordIdeas", []byte("{\"keywordSeed\":{\"keywords\":[\"alpha\"]}}"), nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(data), &record); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(record["status"], float64(http.StatusOK)) ||
		record["kind"] != "response" {
		t.Fatalf("ledger record=%#v", record)
	}
	if len(record) != 3 || record["time"] == nil {
		t.Fatalf("ledger fields=%#v", record)
	}
}

func TestProductionDefaultsKeepOneQPSGuard(t *testing.T) {
	cfg := withDefaults(Config{})
	if cfg.MinRequestInterval != time.Second {
		t.Fatalf("default interval=%s", cfg.MinRequestInterval)
	}
	if cfg.RatePerSecond != 1 || cfg.MaxAttempts != 3 {
		t.Fatalf("defaults rate=%v attempts=%d", cfg.RatePerSecond, cfg.MaxAttempts)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func assertErrorDoesNotContain(t *testing.T, err error, forbidden string) {
	t.Helper()
	if strings.Contains(err.Error(), forbidden) {
		t.Fatalf("error string contains forbidden body: %q", err.Error())
	}
	if valueContains(reflect.ValueOf(err), forbidden, make(map[visit]bool)) {
		t.Fatalf("error value retains forbidden body: %#v", err)
	}
}

func clearConfigEnvForTest(t *testing.T) {
	t.Helper()
	for _, key := range configEnvNames {
		key := key
		oldValue, wasSet := os.LookupEnv(key)
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if wasSet {
				_ = os.Setenv(key, oldValue)
			} else {
				_ = os.Unsetenv(key)
			}
		})
	}
}

type visit struct {
	typ reflect.Type
	ptr uintptr
}

func valueContains(value reflect.Value, forbidden string, seen map[visit]bool) bool {
	if !value.IsValid() {
		return false
	}
	switch value.Kind() {
	case reflect.String:
		return strings.Contains(value.String(), forbidden)
	case reflect.Interface, reflect.Pointer:
		if value.IsNil() {
			return false
		}
		if value.Kind() == reflect.Pointer {
			key := visit{value.Type(), value.Pointer()}
			if key.ptr != 0 && seen[key] {
				return false
			}
			if key.ptr != 0 {
				seen[key] = true
			}
		}
		return valueContains(value.Elem(), forbidden, seen)
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			if valueContains(value.Field(i), forbidden, seen) {
				return true
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			if valueContains(value.Index(i), forbidden, seen) {
				return true
			}
		}
	case reflect.Map:
		iter := value.MapRange()
		for iter.Next() {
			if valueContains(iter.Key(), forbidden, seen) ||
				valueContains(iter.Value(), forbidden, seen) {
				return true
			}
		}
	}
	return false
}
