package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/config"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/traveloka"
	"github.com/spf13/cobra"
)

type sharedSecurityTransport func(*http.Request) (*http.Response, error)

func (fn sharedSecurityTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func isolateSharedSecurityHome(t *testing.T) {
	t.Helper()
	restore, err := cliutil.SetHomeOverride(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restore)
	t.Setenv("TRAVELOKA_SESSION_FILE", "")
}

func TestTravelokaSharedNoSessionRefusesRealEndpointHTTP(t *testing.T) {
	isolateSharedSecurityHome(t)
	for _, mode := range []struct{ name, verify, live string }{
		{"ordinary", "", ""}, {"verify", "1", ""}, {"verify_live_www", "1", "1"},
	} {
		t.Run(mode.name, func(t *testing.T) {
			t.Setenv("PRINTING_PRESS_VERIFY", mode.verify)
			t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", mode.live)
			cfg := &config.Config{BaseURL: "https://www.traveloka.com"}
			c := client.New(cfg, time.Second, 0)
			calls := 0
			c.HTTPClient.Transport = sharedSecurityTransport(func(req *http.Request) (*http.Response, error) {
				calls++
				t.Fatal("sessionless endpoint reached fallback HTTP")
				return nil, nil
			})
			if err := ApplyClientHooks(c); err != nil {
				t.Fatal(err)
			}
			for _, data := range []map[string]any{{}, {"isPrefetch": false}, {"isPrefetch": true}} {
				out, status, err := c.PostQueryWithParams(context.Background(), "/api/v2/flight/search/redirection", nil, map[string]any{"data": data})
				var upstream *client.APIError
				if !errors.As(err, &upstream) || upstream.StatusCode != http.StatusUnauthorized || status != http.StatusUnauthorized || len(out) != 0 {
					t.Fatalf("missing-session refusal did not remain safe HTTP401: output=%s status=%d err=%v", out, status, err)
				}
				if ExitCode(classifyAPIErrorOnly(err)) != 4 {
					t.Fatalf("missing-session refusal lost auth exit: %v", err)
				}
			}
			if calls != 0 {
				t.Fatalf("fallback calls=%d", calls)
			}
			if c.HTTPClient.Jar != nil || !c.NoCache {
				t.Fatal("sessionless endpoint retained cookie jar or cached output")
			}
		})
	}
}

func TestTravelokaSharedVerifierAllowsOnlyExplicitLoopbackMock(t *testing.T) {
	isolateSharedSecurityHome(t)
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "1")
	for _, baseURL := range []string{"http://127.0.0.1:12345", "http://localhost:12345", "https://[::1]:12345"} {
		if !travelokaVerifierMock(baseURL) {
			t.Fatalf("explicit loopback mock refused: %s", baseURL)
		}
	}
	for _, baseURL := range []string{"https://www.traveloka.com", "https://localhost.example", "http://192.0.2.1", "https://user@localhost"} {
		if travelokaVerifierMock(baseURL) {
			t.Fatalf("non-mock bypass allowed: %s", baseURL)
		}
	}
	c := client.New(&config.Config{BaseURL: "http://127.0.0.1:12345"}, time.Second, 0)
	calls := 0
	c.HTTPClient.Transport = sharedSecurityTransport(func(req *http.Request) (*http.Response, error) {
		calls++
		return travelokaReplayResponse(req, http.StatusOK, map[string]any{"data": map[string]any{}})
	})
	if err := ApplyClientHooks(c); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.PostQueryWithParams(context.Background(), "/api/v2/airport/search-nexus", nil, map[string]any{"data": map[string]any{"query": "SIMULATED"}}); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("mock calls=%d", calls)
	}
}

func TestTravelokaSharedHomepageRemainsReachableWithoutCredentials(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://www.traveloka.com/en-sg", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "SIMULATED-auth")
	req.Header.Set("Cookie", "SIMULATED-cookie")
	calls := 0
	adapter := &travelokaReplayTransport{fallback: sharedSecurityTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Fatal("homepage carried credentials")
		}
		return travelokaReplayResponse(r, http.StatusOK, map[string]any{"ok": true})
	})}
	resp, err := adapter.RoundTrip(req)
	if err != nil || calls != 1 {
		t.Fatalf("homepage probe unavailable: %v calls=%d", err, calls)
	}
	resp.Body.Close()
	if req.Header.Get("Authorization") == "" {
		t.Fatal("adapter mutated caller headers")
	}
}

func sharedSecuritySession(t *testing.T, path string) {
	t.Helper()
	dir := t.TempDir()
	cookies := filepath.Join(dir, "SIMULATED-cookies.json")
	requests := filepath.Join(dir, "SIMULATED-requests.json")
	if err := os.WriteFile(cookies, []byte(`[{"name":"guest","value":"SIMULATED-guest-cookie","domain":".traveloka.com","path":"/","expires":-1,"secure":true}]`), 0600); err != nil {
		t.Fatal(err)
	}
	profile := []map[string]any{{"method": "POST", "url": "https://www.traveloka.com/api/v2/airport/search-nexus", "headers": map[string]string{}, "body": `{"data":{"query":"SIMULATED"},"fields":["capturedField"],"clientInterface":"desktop","sentinel":{"token":"SIMULATED-captured-token"}}`}}
	encoded, err := json.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(requests, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = traveloka.ImportSession(cookies, requests, path); err != nil {
		t.Fatal(err)
	}
}

func TestTravelokaSharedDefaultSessionAndPublicEnvelopeReplay(t *testing.T) {
	isolateSharedSecurityHome(t)
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	sharedSecuritySession(t, travelokaDefaultSessionFile())
	c := client.New(&config.Config{BaseURL: "https://www.traveloka.com"}, time.Second, 0)
	var recorded map[string]any
	c.HTTPClient.Transport = sharedSecurityTransport(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://www.traveloka.com/api/v2/airport/search-nexus" {
			t.Fatal("wrong replay origin")
		}
		if !strings.Contains(req.Header.Get("Cookie"), "SIMULATED-guest-cookie") {
			t.Fatal("default source session was not loaded")
		}
		if err := json.NewDecoder(req.Body).Decode(&recorded); err != nil {
			t.Fatal(err)
		}
		return travelokaReplayResponse(req, http.StatusOK, map[string]any{"data": map[string]any{"sections": []any{}}, "sentinel": map[string]any{"token": "SIMULATED-response-token"}})
	})
	if err := ApplyClientHooks(c); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"data": map[string]any{"query": "Singapore"}, "fields": []any{"sourceName"}, "clientInterface": "desktop", "sentinel": map[string]any{"token": "SIMULATED-caller-token"}}
	out, _, err := c.PostQueryWithParams(context.Background(), "/api/v2/airport/search-nexus", nil, body)
	if err != nil {
		t.Fatal(err)
	}
	if recorded["fields"].([]any)[0] != "sourceName" || recorded["clientInterface"] != "desktop" {
		t.Fatalf("public envelope flags lost: %v", recorded)
	}
	if recorded["sentinel"].(map[string]any)["token"] != "SIMULATED-captured-token" {
		t.Fatal("caller replaced captured private envelope")
	}
	if strings.Contains(string(out), "SIMULATED-response-token") || strings.Contains(string(out), "sentinel") {
		t.Fatalf("public replay leaked source credentials: %s", out)
	}
}

func TestTravelokaSharedMissingHistoryRespectsExplicitID(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.db")
	value, err := travelokaLoad(context.Background(), missing, "", "SIMULATED-missing", "flights")
	var typed *traveloka.APIError
	if value != nil || !errors.As(err, &typed) || typed.Code != "NOT_FOUND" {
		t.Fatalf("explicit missing ID hidden: %v %v", value, err)
	}
	if value, err = travelokaLoad(context.Background(), missing, "", "", "flights"); value != nil || err != nil {
		t.Fatalf("ordinary empty cache failed: %v %v", value, err)
	}
	if _, err = os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("local miss created database")
	}
}

func TestTravelokaSharedErrorFormattingPreservesTypedExit(t *testing.T) {
	source := &traveloka.APIError{Code: "AUTH_REQUIRED", Message: "SIMULATED missing scoped session", Status: 401}
	for _, tc := range []struct {
		name        string
		flags       rootFlags
		has, absent string
	}{
		{"selected", rootFlags{asJSON: true, selectFields: "error.code"}, "AUTH_REQUIRED", "message"},
		{"csv", rootFlags{csv: true, selectFields: "error.code"}, "AUTH_REQUIRED", "\"error\""},
		{"plain", rootFlags{plain: true, selectFields: "error.code"}, "AUTH_REQUIRED", "\"error\""},
		{"quiet", rootFlags{quiet: true}, "", "AUTH_REQUIRED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&out)
			err := travelokaFail(cmd, &tc.flags, source)
			var typed *traveloka.APIError
			if !errors.As(err, &typed) || typed != source || ExitCode(err) != 4 {
				t.Fatalf("typed failure lost: %v", err)
			}
			if !strings.Contains(out.String(), tc.has) || strings.Contains(out.String(), tc.absent) {
				t.Fatalf("error formatting ignored flags: %q", out.String())
			}
		})
	}
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	pathErr := &os.PathError{Op: "open", Path: "SIMULATED-missing.json", Err: os.ErrNotExist}
	err := travelokaFail(cmd, &rootFlags{asJSON: true}, pathErr)
	var preserved *os.PathError
	if ExitCode(err) != 6 || !errors.As(err, &preserved) || preserved != pathErr || !strings.Contains(out.String(), "LOCAL_IO_ERROR") {
		t.Fatalf("local path exit/type lost: %v %q", err, out.String())
	}
}

func TestTravelokaSharedAirportValidationRejectsFailureAndMalformedData(t *testing.T) {
	cases := []struct{ name, input, code string }{
		{"failed envelope", `{"success":false,"data":{"sections":[]}}`, "UPSTREAM_ERROR"},
		{"failed data", `{"data":{"success":false,"sections":[]}}`, "UPSTREAM_ERROR"},
		{"failed envelope status", `{"status":"FAILED","data":{"sections":[]}}`, "UPSTREAM_ERROR"},
		{"failed data status", `{"data":{"status":"FAILED","sections":[]}}`, "UPSTREAM_ERROR"},
		{"envelope error", `{"error":{"message":"SIMULATED invalid session"},"data":{"sections":[]}}`, "UPSTREAM_ERROR"},
		{"data error", `{"data":{"error":{"message":"SIMULATED invalid session"},"sections":[]}}`, "UPSTREAM_ERROR"},
		{"bad success", `{"success":"yes","data":{"sections":[]}}`, "MALFORMED_RESPONSE"},
		{"missing data", `{}`, "MALFORMED_RESPONSE"},
		{"missing sections", `{"data":{}}`, "MALFORMED_RESPONSE"},
		{"null sections", `{"data":{"sections":null}}`, "MALFORMED_RESPONSE"},
		{"malformed section", `{"data":{"sections":["bad"]}}`, "MALFORMED_RESPONSE"},
		{"missing results", `{"data":{"sections":[{}]}}`, "MALFORMED_RESPONSE"},
		{"empty source inventory", `{"success":true,"data":{"sections":[]}}`, ""},
		{"no source inventory", `{"status":"NO_INVENTORY","error":null,"data":{"status":"NO_RESULTS","error":{},"sections":[]}}`, ""},
		{"valid source inventory", `{"data":{"sections":[{"type":"RECOMMENDED","results":[]}]}}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var value map[string]any
			if err := json.Unmarshal([]byte(tc.input), &value); err != nil {
				t.Fatal(err)
			}
			err := travelokaCheckAirportValidationReply(value)
			var typed *traveloka.APIError
			if tc.code == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if !errors.As(err, &typed) || typed.Code != tc.code {
				t.Fatalf("unusable validation response accepted: %v", err)
			}
		})
	}
}

func TestTravelokaSharedSourceAuthFailuresDoNotRetryOrLoseStatus(t *testing.T) {
	isolateSharedSecurityHome(t)
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	t.Setenv("PRINTING_PRESS_DOGFOOD", "")
	path := travelokaDefaultSessionFile()
	sharedSecuritySession(t, path)
	for _, tc := range []struct {
		name                       string
		sourceStatus, bridgeStatus int
		code                       string
	}{
		{"protection", http.StatusAccepted, http.StatusForbidden, "ACCESS_BLOCKED"},
		{"expired", http.StatusUnauthorized, http.StatusUnauthorized, "AUTH_REQUIRED"},
		{"forbidden", http.StatusForbidden, http.StatusForbidden, "ACCESS_BLOCKED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := client.New(&config.Config{BaseURL: "https://www.traveloka.com", TravelokaSessionFile: path}, time.Second, 0)
			calls := 0
			c.HTTPClient.Transport = sharedSecurityTransport(func(req *http.Request) (*http.Response, error) {
				calls++
				return travelokaReplayResponse(req, tc.sourceStatus, map[string]any{"upstreamEcho": "SIMULATED-guest-cookie SIMULATED-captured-token"})
			})
			if err := ApplyClientHooks(c); err != nil {
				t.Fatal(err)
			}
			_, status, err := c.PostQueryWithParams(context.Background(), "/api/v2/airport/search-nexus", nil, map[string]any{"data": map[string]any{"query": "Singapore"}})
			var native *client.APIError
			if calls != 1 || status != tc.bridgeStatus || !errors.As(err, &native) {
				t.Fatalf("source auth failure was retried or lost HTTP classification: calls=%d status=%d err=%v", calls, status, err)
			}
			var body struct {
				Error traveloka.APIError `json:"error"`
			}
			if e := json.Unmarshal([]byte(native.Body), &body); e != nil {
				t.Fatal(e)
			}
			if body.Error.Code != tc.code || body.Error.Status != tc.sourceStatus || body.Error.Retryable {
				t.Fatalf("source auth metadata changed: %s", native.Body)
			}
			if strings.Contains(native.Body, "SIMULATED-") {
				t.Fatalf("source auth body exposed private response echo: %s", native.Body)
			}
			var out bytes.Buffer
			classified := classifyAPIError(&out, err, &rootFlags{asJSON: true})
			if ExitCode(classified) != 4 || !strings.Contains(out.String(), tc.code) {
				t.Fatalf("native classifier lost source auth failure: exit=%d output=%s", ExitCode(classified), out.String())
			}
		})
	}
}

func TestTravelokaSharedSourceRateAndTimeoutTypesSurviveBridge(t *testing.T) {
	isolateSharedSecurityHome(t)
	// Disable generic retry loops while retaining the real pinned-source adapter.
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "")
	path := travelokaDefaultSessionFile()
	sharedSecuritySession(t, path)
	for _, name := range []string{"rate", "deadline"} {
		t.Run(name, func(t *testing.T) {
			c := client.New(&config.Config{BaseURL: "https://www.traveloka.com", TravelokaSessionFile: path}, time.Second, 0)
			calls := 0
			c.HTTPClient.Transport = sharedSecurityTransport(func(req *http.Request) (*http.Response, error) {
				calls++
				if name == "deadline" {
					return nil, context.DeadlineExceeded
				}
				return travelokaReplayResponse(req, http.StatusTooManyRequests, map[string]any{"error": "SIMULATED source rate limit"})
			})
			if err := ApplyClientHooks(c); err != nil {
				t.Fatal(err)
			}
			_, _, err := c.PostQueryWithParams(context.Background(), "/api/v2/airport/search-nexus", nil, map[string]any{"data": map[string]any{"query": "Singapore"}})
			if calls != 1 || err == nil {
				t.Fatalf("typed failure disappeared: calls=%d err=%v", calls, err)
			}
			if name == "rate" {
				var rate *cliutil.RateLimitError
				if !errors.As(err, &rate) || ExitCode(classifyAPIErrorOnly(err)) != 7 {
					t.Fatalf("rate-limit type/exit lost: %v", err)
				}
			} else {
				var source *traveloka.APIError
				if !errors.Is(err, context.DeadlineExceeded) || !errors.As(err, &source) || source.Code != "TIMEOUT" {
					t.Fatalf("deadline type/cause lost: %v", err)
				}
			}
		})
	}
}
