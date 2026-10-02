package traveloka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/cliutil"
)

type simulatedTransport func(*http.Request) (*http.Response, error)

func (f simulatedTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func simulatedResponse(r *http.Request, status int, body string, headers http.Header) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: headers, Request: r}
}
func testHTTPClient(t *testing.T) (*Client, string) {
	t.Helper()
	path, _ := importedFixture(t, flightInitial)
	c, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	c.limiter = cliutil.NewAdaptiveLimiter(100000)
	return c, path
}
func TestSimulatedHTTPReplaySanitizedFullEnvelopeAndPrivateRefresh(t *testing.T) {
	c, path := testHTTPClient(t)
	calls := 0
	c.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != origin+flightInitial || r.Method != http.MethodPost {
			t.Fatal("HTTP origin/operation escaped pinned allowlist")
		}
		for k, want := range map[string]string{"Origin": origin, "Sec-Fetch-Site": "same-origin", "Sec-Fetch-Mode": "cors", "Sec-Fetch-Dest": "empty", "Accept": "application/json", "Content-Type": "application/json", "Tv-Country": "SG", "Tv-Language": "en_SG", "Tv-Currency": "USD", "X-Route-Prefix": "en-sg", "X-Domain": "flight", "Fpr-Search-Id": "fresh-id", "T-A-V": "synthetic-header-secret", "User-Agent": "synthetic-UA", "Www-App-Version": "observed-version"} {
			if r.Header.Get(k) != want {
				t.Fatalf("wrong captured/normal shopper header %s", k)
			}
		}
		if !strings.Contains(r.Header.Get("Cookie"), "guest=synthetic-cookie-secret") {
			t.Fatal("scoped jar cookie missing")
		}
		if r.Header.Get("Authorization") != "" || strings.Contains(r.Header.Get("Cookie"), "must-not-import") {
			t.Fatal("uncaptured credentials imported")
		}
		if d, ok := r.Context().Deadline(); !ok || time.Until(d) > requestTimeout {
			t.Fatal("request deadline is unbounded")
		}
		b, _ := io.ReadAll(r.Body)
		var payload map[string]any
		if err := decodeJSON(b, &payload); err != nil {
			t.Fatal(err)
		}
		data := payload["data"].(map[string]any)
		if data["currency"] != "USD" || data["locale"] != "en_SG" || data["searchId"] != "fresh-id" {
			t.Fatal("shopper payload override failed")
		}
		sentinel := payload["sentinel"].(map[string]any)["token"]
		wantSentinel := "synthetic-sentinel-secret"
		if calls > 1 {
			wantSentinel = "synthetic-refreshed-sentinel"
		}
		if sentinel != wantSentinel {
			t.Fatal("legitimate sentinel replay/refresh missing")
		}
		return simulatedResponse(r, 200, `{"data":{"offers":[],"userContext":{"token":"hidden"},"marketingContextCapsule":"hidden","inventoryRateKey":"hidden","echo":"synthetic-cookie-secret synthetic-header-secret synthetic-refreshed-cookie"},"meta":{"searchCompleted":true},"sentinel":{"token":"synthetic-refreshed-sentinel"}}`, http.Header{"Set-Cookie": []string{"refreshed=synthetic-refreshed-cookie; Domain=traveloka.com; Path=/; Secure; HttpOnly"}}), nil
	}))
	input := map[string]any{"searchId": "fresh-id", "currency": "SGD", "locale": "original"}
	result, err := c.Post(context.Background(), flightInitial, input, Shopper{"SG", "en-SG", "USD"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := result["data"]; !ok {
		t.Fatal("full response data envelope missing")
	}
	if result["meta"].(map[string]any)["searchCompleted"] != true {
		t.Fatal("full response metadata missing")
	}
	b, _ := json.Marshal(result)
	for _, secret := range []string{"synthetic-cookie-secret", "synthetic-header-secret", "synthetic-sentinel-secret", "synthetic-refreshed-sentinel", "synthetic-refreshed-cookie", "userContext", "marketingContextCapsule", "inventoryRateKey", "sentinel"} {
		if strings.Contains(string(b), secret) {
			t.Fatalf("public response leaked %s", secret)
		}
	}
	if input["currency"] != "SGD" || input["locale"] != "original" {
		t.Fatal("Post mutated caller data")
	}
	if _, err = c.Post(context.Background(), flightInitial, input, Shopper{"SG", "en-SG", "USD"}); err != nil {
		t.Fatal(err)
	}
	reloaded, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.session.Profiles[flightInitial].Body["sentinel"].(map[string]any)["token"] != "synthetic-refreshed-sentinel" {
		t.Fatal("response sentinel not persisted privately")
	}
	found := false
	for _, cookie := range reloaded.session.Cookies {
		if cookie.Name == "refreshed" && cookie.Value == "synthetic-refreshed-cookie" {
			found = true
		}
	}
	if !found {
		t.Fatal("response cookie not persisted privately")
	}
	st, _ := os.Stat(path)
	if st.Mode().Perm() != 0600 {
		t.Fatal("refresh changed private file permissions")
	}
}
func TestSimulatedHTTPErrorClassificationAndRedaction(t *testing.T) {
	for _, tt := range []struct {
		name       string
		status     int
		body, code string
		rate       bool
	}{
		{"expired", 401, "synthetic-cookie-secret", "AUTH_REQUIRED", false},
		{"forbidden", 403, "synthetic-header-secret", "ACCESS_BLOCKED", false},
		{"empty protection", 202, "", "ACCESS_BLOCKED", false},
		{"nonempty protection", 202, `{"data":[]}`, "ACCESS_BLOCKED", false},
		{"upstream", 500, "synthetic-cookie-secret", "UPSTREAM_ERROR", false},
		{"malformed", 200, "synthetic-sentinel-secret", "MALFORMED_RESPONSE", false},
		{"empty successful HTTP is malformed", 200, "", "MALFORMED_RESPONSE", false},
		{"non-object", 200, `[]`, "MALFORMED_RESPONSE", false},
		{"multiple values", 200, `{} {}`, "MALFORMED_RESPONSE", false},
		{"throttled", 429, "synthetic-header-secret", "RATE_LIMITED", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := testHTTPClient(t)
			c.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
				return simulatedResponse(r, tt.status, tt.body, http.Header{"Retry-After": []string{"5"}}), nil
			}))
			result, err := c.Post(context.Background(), flightInitial, map[string]any{}, Shopper{"SG", "en-SG", "SGD"})
			if err == nil || result != nil {
				t.Fatal("error must never become empty inventory")
			}
			if strings.Contains(err.Error(), "synthetic") {
				t.Fatal("HTTP error leaks secret values")
			}
			var ae *APIError
			if !errors.As(err, &ae) || ae.Code != tt.code || ae.Status != tt.status {
				t.Fatalf("wrong error: %v", err)
			}
			var rate *cliutil.RateLimitError
			if errors.As(err, &rate) != tt.rate {
				t.Fatal("429 must have generated typed error")
			}
			if tt.rate && rate.RetryAfter != 5*time.Second {
				t.Fatal("lost bounded Retry-After")
			}
		})
	}
}
func TestSimulatedHTTPRefusesRedirectAndUnsupportedOperations(t *testing.T) {
	c, _ := testHTTPClient(t)
	calls := 0
	c.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		return simulatedResponse(r, 302, "", http.Header{"Location": []string{"https://evil.example/steal"}}), nil
	}))
	_, err := c.Post(context.Background(), flightInitial, map[string]any{}, Shopper{"SG", "en-SG", "SGD"})
	var ae *APIError
	if !errors.As(err, &ae) || ae.Code != "ACCESS_BLOCKED" || calls != 1 {
		t.Fatalf("redirect was not refused before replay: %v", err)
	}
	for _, path := range []string{"https://evil.example/", "/api/v2/booking", flightInitial + "?token=x"} {
		if _, err = c.Post(context.Background(), path, map[string]any{}, Shopper{"SG", "en-SG", "SGD"}); err == nil {
			t.Fatal("unsupported path accepted")
		}
	}
	if calls != 1 {
		t.Fatal("unsafe operation reached transport")
	}
	if _, err = c.Post(context.Background(), flightInitial, nil, Shopper{"SG", "en-SG", "SGD"}); err == nil {
		t.Fatal("nil data accepted")
	}
	if _, err = c.Post(context.Background(), flightInitial, map[string]any{}, Shopper{}); err == nil {
		t.Fatal("invalid shopper accepted")
	}
}
func TestSimulatedHTTPMissingSentinelIsNotManufactured(t *testing.T) {
	c, _ := testHTTPClient(t)
	p := c.session.Profiles[flightInitial]
	delete(p.Body, "sentinel")
	c.session.Profiles[flightInitial] = p
	c.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
		b, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = decodeJSON(b, &payload)
		if _, ok := payload["sentinel"]; ok {
			t.Fatal("manufactured sentinel")
		}
		return simulatedResponse(r, 200, `{"data":{"offers":[]},"meta":{"searchCompleted":true}}`, http.Header{}), nil
	}))
	result, err := c.Post(context.Background(), flightInitial, map[string]any{}, Shopper{"SG", "en-SG", "SGD"})
	if err != nil || result["meta"].(map[string]any)["searchCompleted"] != true {
		t.Fatal("successful empty response is valid")
	}
}
func TestSimulatedHTTPTransportErrorsRedactExactCredentials(t *testing.T) {
	c, _ := testHTTPClient(t)
	c.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("failure synthetic-cookie-secret synthetic-header-secret synthetic-sentinel-secret")
	}))
	_, err := c.Post(context.Background(), flightInitial, map[string]any{}, Shopper{"SG", "en-SG", "SGD"})
	if err == nil || strings.Contains(err.Error(), "synthetic") || !strings.Contains(err.Error(), "[REDACTED]") {
		t.Fatalf("unsafe HTTP error: %v", err)
	}
}
func TestSimulatedHTTPResponseBoundAndCanceledDeadline(t *testing.T) {
	c, _ := testHTTPClient(t)
	c.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
		return simulatedResponse(r, 200, strings.Repeat("x", maxResponseBytes+1), http.Header{}), nil
	}))
	_, err := c.Post(context.Background(), flightInitial, map[string]any{}, Shopper{"SG", "en-SG", "SGD"})
	var ae *APIError
	if !errors.As(err, &ae) || ae.Code != "MALFORMED_RESPONSE" {
		t.Fatal("oversize response was not rejected")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Post(ctx, flightInitial, map[string]any{}, Shopper{"SG", "en-SG", "SGD"}); err == nil {
		t.Fatal("canceled context ignored")
	}
	c.SetHTTPTransport(nil)
	if c.http.Transport != http.DefaultTransport {
		t.Fatal("nil transport must restore normal HTTP")
	}
}
func TestSimulatedSanitize(t *testing.T) {
	input := map[string]any{"nested": []any{map[string]any{"userContext": "private", "inventory_rate_key": "private", "cancellation": "known"}}, "request_token": "private", "currency": "SGD"}
	value := Sanitize(input).(map[string]any)
	b, _ := json.Marshal(value)
	if strings.Contains(string(b), "private") || !strings.Contains(string(b), "known") || value["currency"] != "SGD" {
		t.Fatal("incorrect recursive public sanitization")
	}
	if input["request_token"] != "private" {
		t.Fatal("sanitizer mutated source")
	}
}

func TestSimulatedShortPreferenceCookiesDoNotCorruptMoney(t *testing.T) {
	c, _ := testHTTPClient(t)
	c.secrets = append(c.secrets, "1", "SGD", "en_SG")
	c.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
		return simulatedResponse(r, 200, `{"data":{"amount":"47181","currency":"SGD","inventoryRateKey":"opaque-runtime-secret","echo":"opaque-runtime-secret"},"meta":{"searchCompleted":true}}`, http.Header{}), nil
	}))
	result, err := c.Post(context.Background(), flightInitial, map[string]any{}, Shopper{"SG", "en-SG", "SGD"})
	if err != nil {
		t.Fatal(err)
	}
	data := result["data"].(map[string]any)
	if data["amount"] != "47181" || data["currency"] != "SGD" || data["echo"] != "[REDACTED]" {
		t.Fatal("opaque redaction must preserve source numeric/currency preference values")
	}
}

func TestSimulatedSanitizePreservesPaymentPreauthorization(t *testing.T) {
	input := map[string]any{"preauthorization": true, "cardPreauthorizationConditions": "Source card guarantee policy", "paymentAuthorizationPolicy": "Source payment policy", "authorization": "private credential", "proxy_authorization": "private proxy credential"}
	value := Sanitize(input).(map[string]any)
	if value["preauthorization"] != true || value["cardPreauthorizationConditions"] != "Source card guarantee policy" || value["paymentAuthorizationPolicy"] != "Source payment policy" {
		t.Fatal("sanitization removed legitimate payment/preauthorization conditions")
	}
	if _, ok := value["authorization"]; ok {
		t.Fatal("authorization credential retained")
	}
	if _, ok := value["proxy_authorization"]; ok {
		t.Fatal("proxy authorization credential retained")
	}
	if Sanitize(nil) != nil {
		t.Fatal("unknown value must remain nil")
	}
	if value := Sanitize(map[string]any{}).(map[string]any); len(value) != 0 {
		t.Fatal("empty value must remain empty")
	}
}

func TestSimulatedPayloadDoesNotInventCurrencyForResolvers(t *testing.T) {
	path, _ := importedFixture(t, "/api/v2/airport/search-nexus")
	b, _ := os.ReadFile(path)
	var session privateSession
	if err := decodeJSON(b, &session); err != nil {
		t.Fatal(err)
	}
	p := session.Profiles["/api/v2/airport/search-nexus"]
	delete(p.Body["data"].(map[string]any), "currency")
	session.Profiles["/api/v2/airport/search-nexus"] = p
	writeFixtureJSON(t, path, session)
	c, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	c.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = decodeJSON(body, &payload)
		if _, exists := payload["data"].(map[string]any)["currency"]; exists {
			t.Fatal("currency field invented for source resolver")
		}
		return simulatedResponse(r, 200, `{"data":{"sections":[]},"meta":{}}`, http.Header{}), nil
	}))
	if _, err := c.Post(context.Background(), "/api/v2/airport/search-nexus", map[string]any{"query": "Singapore"}, Shopper{"SG", "en-SG", "USD"}); err != nil {
		t.Fatal(err)
	}
}

func TestSimulatedBrowserJSONCookieRefreshIsExactAndScoped(t *testing.T) {
	c, path := testHTTPClient(t)
	calls := 0
	value := `{"state":"synthetic-refreshed","escaped":"a\\b"}`
	c.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls > 1 && !strings.Contains(r.Header.Get("Cookie"), "g_state="+value) {
			t.Fatal("refreshed browser JSON value was changed or omitted")
		}
		return simulatedResponse(r, 200, `{"data":{},"meta":{}}`, http.Header{"Set-Cookie": []string{"g_state=" + value + "; Domain=traveloka.com; Path=/; Secure; HttpOnly; SameSite=Lax; Max-Age=120"}}), nil
	}))
	for i := 0; i < 2; i++ {
		if _, err := c.Post(context.Background(), flightInitial, map[string]any{}, Shopper{"SG", "en-SG", "SGD"}); err != nil {
			t.Fatal(err)
		}
	}
	reloaded, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, cookie := range reloaded.session.Cookies {
		if cookie.Name == "g_state" {
			found = true
			if cookie.Value != value || cookie.SameSite != "Lax" || cookie.Expires == "-1" {
				t.Fatal("refreshed browser cookie not privately preserved with attributes")
			}
		}
	}
	if !found {
		t.Fatal("browser cookie refresh discarded")
	}
	foreign, _ := url.Parse("https://evil.example/")
	if len(reloaded.jar.Cookies(foreign)) != 0 {
		t.Fatal("scoped browser values escaped Traveloka host")
	}
	otherPath, _ := url.Parse(origin + "/unrelated")
	scoped := syntheticCookies()[0]
	scoped.Name = "operation-only"
	scoped.Path = "/api/v2/flight"
	reloaded.jar.SetCookies(otherPath, []*http.Cookie{cookieHTTP(scoped)})
	for _, cookie := range reloaded.jar.Cookies(otherPath) {
		if cookie.Name == scoped.Name {
			t.Fatal("operation cookie escaped source path")
		}
	}
}

func TestSimulatedPartialResponseSentinelPreservesCapturedRequestShape(t *testing.T) {
	c, path := testHTTPClient(t)
	captured := map[string]any{"signals": []any{}, "token": "synthetic-sentinel-secret", "context": map[string]any{"required": "source-request"}}
	p := c.session.Profiles[flightInitial]
	p.Body["sentinel"] = captured
	c.session.Profiles[flightInitial] = p
	calls := 0
	c.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		b, _ := io.ReadAll(r.Body)
		var payload map[string]any
		if err := decodeJSON(b, &payload); err != nil {
			t.Fatal(err)
		}
		sentinel := payload["sentinel"].(map[string]any)
		if signals, ok := sentinel["signals"].([]any); !ok || len(signals) != 0 {
			t.Fatal("partial response removed captured request signals")
		}
		context := sentinel["context"].(map[string]any)
		if context["required"] != "source-request" {
			t.Fatal("nested partial response removed or aliased required request context")
		}
		if calls > 1 && (sentinel["token"] != "synthetic-refreshed-sentinel" || context["updated"] != "source-response") {
			t.Fatal("legitimate partial response was not merged into private request sentinel")
		}
		return simulatedResponse(r, 200, `{"data":{},"meta":{},"sentinel":{"token":"synthetic-refreshed-sentinel","context":{"updated":"source-response"}}}`, http.Header{}), nil
	}))
	result, err := c.Post(context.Background(), flightInitial, map[string]any{}, Shopper{"SG", "en-SG", "SGD"})
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := result["sentinel"]; exists {
		t.Fatal("sentinel reached public response")
	}
	if captured["token"] != "synthetic-sentinel-secret" || captured["context"].(map[string]any)["updated"] != nil {
		t.Fatal("merge mutated the captured sentinel object")
	}
	captured["context"].(map[string]any)["required"] = "caller-mutation"
	if _, err = c.Post(context.Background(), flightInitial, map[string]any{}, Shopper{"SG", "en-SG", "SGD"}); err != nil {
		t.Fatal(err)
	}
	reloaded, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	sentinel := reloaded.session.Profiles[flightInitial].Body["sentinel"].(map[string]any)
	if _, ok := sentinel["signals"].([]any); !ok {
		t.Fatal("required request signals were not persisted privately")
	}
}
func TestSimulatedRedirectionRequiresExplicitReadOnlyPrefetch(t *testing.T) {
	const path = "/api/v2/flight/search/redirection"
	file, _ := importedFixture(t, path)
	c, err := NewClient(file)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	c.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = decodeJSON(body, &payload)
		if payload["data"].(map[string]any)["isPrefetch"] != true {
			t.Fatal("non-prefetch redirection reached transport")
		}
		return simulatedResponse(r, 200, `{"data":{},"meta":{}}`, http.Header{}), nil
	}))
	for _, value := range []any{nil, false, "true", json.Number("1"), 1} {
		data := map[string]any{}
		if value != nil {
			data["isPrefetch"] = value
		}
		result, err := c.Post(context.Background(), path, data, Shopper{"SG", "en-SG", "SGD"})
		var ae *APIError
		if result != nil || !errors.As(err, &ae) || ae.Code != "UNSUPPORTED_OPERATION" {
			t.Fatalf("unsafe redirection was not rejected before HTTP: %v", err)
		}
	}
	if calls != 0 {
		t.Fatal("unsafe advanced raw redirection issued remote requests")
	}
	if _, err := c.Post(context.Background(), path, map[string]any{"isPrefetch": true}, Shopper{"SG", "en-SG", "SGD"}); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal("source read-only prefetch was not allowed")
	}
}

func TestSimulatedResponseSentinelCannotInventOrReplaceRequestShape(t *testing.T) {
	for _, update := range []any{nil, "synthetic-response-token", []any{}, json.Number("1")} {
		c, _ := testHTTPClient(t)
		c.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
			body, _ := json.Marshal(map[string]any{"data": map[string]any{}, "meta": map[string]any{}, "sentinel": update})
			return simulatedResponse(r, 200, string(body), http.Header{}), nil
		}))
		if _, err := c.Post(context.Background(), flightInitial, map[string]any{}, Shopper{"SG", "en-SG", "SGD"}); err != nil {
			t.Fatal(err)
		}
		if c.session.Profiles[flightInitial].Body["sentinel"].(map[string]any)["token"] != "synthetic-sentinel-secret" {
			t.Fatal("incompatible response replaced the captured request object")
		}
	}
	c, _ := testHTTPClient(t)
	p := c.session.Profiles[flightInitial]
	delete(p.Body, "sentinel")
	c.session.Profiles[flightInitial] = p
	c.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
		return simulatedResponse(r, 200, `{"data":{},"meta":{},"sentinel":{"token":"synthetic-response-token"}}`, http.Header{}), nil
	}))
	if _, err := c.Post(context.Background(), flightInitial, map[string]any{}, Shopper{"SG", "en-SG", "SGD"}); err != nil {
		t.Fatal(err)
	}
	if _, exists := c.session.Profiles[flightInitial].Body["sentinel"]; exists {
		t.Fatal("response invented a previously uncaptured request sentinel")
	}
}

func TestSimulatedHTTPPublicEnvelopeOverrides(t *testing.T) {
	c, _ := testHTTPClient(t)
	profile := c.session.Profiles[flightInitial]
	profile.Body["fields"] = []any{"captured"}
	profile.Body["clientInterface"] = "SIMULATED-captured-interface"
	c.session.Profiles[flightInitial] = profile
	wantFields := []any{"offers", "meta"}
	wantInterface := "desktop"
	c.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(body["fields"], wantFields) || body["clientInterface"] != wantInterface {
			t.Fatalf("public envelope overrides were dropped: fields=%v interface=%v", body["fields"], body["clientInterface"])
		}
		if object := body["sentinel"].(map[string]any); object["token"] != "synthetic-sentinel-secret" {
			t.Fatal("override replaced private captured sentinel")
		}
		return simulatedResponse(r, 200, `{"data":{"offers":[]}}`, http.Header{}), nil
	}))
	overrides := map[string]any{"fields": []any{"offers", "meta"}, "clientInterface": "desktop"}
	if _, err := c.PostWithEnvelope(context.Background(), flightInitial, map[string]any{}, Shopper{"SG", "en-SG", "SGD"}, overrides); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(overrides["fields"], []any{"offers", "meta"}) {
		t.Fatal("caller projection values were mutated")
	}
	wantFields = []any{"captured"}
	wantInterface = "SIMULATED-captured-interface"
	if _, err := c.Post(context.Background(), flightInitial, map[string]any{}, Shopper{"SG", "en-SG", "SGD"}); err != nil {
		t.Fatal(err)
	}
	wantFields = []any{}
	wantInterface = "desktop"
	if _, err := c.PostWithEnvelope(context.Background(), flightInitial, map[string]any{}, Shopper{"SG", "en-SG", "SGD"}, map[string]any{"fields": []string{}, "clientInterface": "desktop"}); err != nil {
		t.Fatal(err)
	}
}

func TestSimulatedHTTPEnvelopeOverridesRejectBeforeTransport(t *testing.T) {
	c, _ := testHTTPClient(t)
	c.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
		t.Fatal("invalid envelope reached transport")
		return nil, nil
	}))
	many := make([]string, 101)
	for i := range many {
		many[i] = "offers"
	}
	for _, tt := range []struct {
		name      string
		overrides map[string]any
		code      string
	}{
		{"private_sentinel", map[string]any{"sentinel": map[string]any{"token": "SIMULATED"}}, "INVALID_INPUT"},
		{"private_headers", map[string]any{"headers": map[string]any{}}, "INVALID_INPUT"},
		{"field_object", map[string]any{"fields": map[string]any{}}, "INVALID_INPUT"},
		{"field_number", map[string]any{"fields": []any{json.Number("1")}}, "INVALID_INPUT"},
		{"empty_name", map[string]any{"fields": []string{" "}}, "INVALID_INPUT"},
		{"long_name", map[string]any{"fields": []string{strings.Repeat("x", 257)}}, "INVALID_INPUT"},
		{"many_fields", map[string]any{"fields": many}, "INVALID_INPUT"},
		{"field_control", map[string]any{"fields": []string{"offers\n"}}, "INVALID_INPUT"},
		{"interface_number", map[string]any{"clientInterface": json.Number("1")}, "INVALID_INPUT"},
		{"unsupported_interface", map[string]any{"clientInterface": "mobile"}, "UNSUPPORTED_OPERATION"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.PostWithEnvelope(context.Background(), flightInitial, map[string]any{}, Shopper{"SG", "en-SG", "SGD"}, tt.overrides)
			var source *APIError
			if !errors.As(err, &source) || source.Code != tt.code {
				t.Fatalf("unexpected override error: %v", err)
			}
		})
	}
}

type simulatedContextBody struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func (b simulatedContextBody) Read([]byte) (int, error) {
	if b.cancel != nil {
		b.cancel()
	}
	<-b.ctx.Done()
	return 0, fmt.Errorf("synthetic-cookie-secret: %w", b.ctx.Err())
}
func (b simulatedContextBody) Close() error { return nil }

func TestSimulatedHTTPDeadlineAndCancellationPreserveSafeCauses(t *testing.T) {
	for _, stage := range []string{"request", "body"} {
		for _, cancellation := range []bool{false, true} {
			name := stage + "/deadline"
			if cancellation {
				name = stage + "/cancel"
			}
			t.Run(name, func(t *testing.T) {
				c, _ := testHTTPClient(t)
				if err := c.SetRateLimit(0); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
				defer cancel()
				var cancelInFlight context.CancelFunc
				if cancellation {
					cancelInFlight = cancel
				}
				c.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
					if stage == "body" {
						return &http.Response{StatusCode: 200, Header: http.Header{}, Body: simulatedContextBody{r.Context(), cancelInFlight}, Request: r}, nil
					}
					if cancelInFlight != nil {
						cancelInFlight()
					}
					<-r.Context().Done()
					return nil, fmt.Errorf("synthetic-cookie-secret: %w", r.Context().Err())
				}))
				_, err := c.Post(ctx, flightInitial, map[string]any{}, Shopper{"SG", "en-SG", "SGD"})
				wantCode, wantCause := "TIMEOUT", context.DeadlineExceeded
				if cancellation {
					wantCode, wantCause = "CANCELED", context.Canceled
				}
				var source *APIError
				if !errors.As(err, &source) || source.Code != wantCode || !errors.Is(err, wantCause) {
					t.Fatalf("context classification/cause lost: %v", err)
				}
				if source.Retryable == cancellation {
					t.Fatalf("wrong context retryability: %v", err)
				}
				b, marshalErr := json.Marshal(source)
				if marshalErr != nil || strings.Contains(err.Error(), "synthetic-cookie-secret") || strings.Contains(string(b), "synthetic-cookie-secret") || strings.Contains(string(b), "Cause") {
					t.Fatalf("private failure appeared in public error: %s", b)
				}
			})
		}
	}
}

func TestSimulatedSharedSessionReloadsBeforeRefreshing(t *testing.T) {
	first, path := testHTTPClient(t)
	stale, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	shop := Shopper{Market: "SG", Locale: "en-SG", Currency: "SGD"}
	first.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
		return simulatedResponse(r, 200, `{"data":{},"sentinel":{"token":"example-first-refresh"}}`, http.Header{"Set-Cookie": []string{"first=example-first-cookie; Domain=traveloka.com; Path=/; Secure"}}), nil
	}))
	if _, err = first.Post(context.Background(), flightInitial, map[string]any{}, shop); err != nil {
		t.Fatal(err)
	}
	stale.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
		b, _ := io.ReadAll(r.Body)
		var envelope map[string]any
		if err := decodeJSON(b, &envelope); err != nil {
			t.Fatal(err)
		}
		if sourceObject(envelope["sentinel"])["token"] != "example-first-refresh" || !strings.Contains(r.Header.Get("Cookie"), "first=example-first-cookie") {
			t.Fatal("second client replayed stale credentials")
		}
		return simulatedResponse(r, 200, `{"data":{},"sentinel":{"token":"example-second-refresh"}}`, http.Header{"Set-Cookie": []string{"second=example-second-cookie; Domain=traveloka.com; Path=/; Secure"}}), nil
	}))
	if _, err = stale.Post(context.Background(), flightInitial, map[string]any{}, shop); err != nil {
		t.Fatal(err)
	}
	final, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, cookie := range final.session.Cookies {
		names[cookie.Name] = true
	}
	if !names["first"] || !names["second"] || sourceObject(final.session.Profiles[flightInitial].Body["sentinel"])["token"] != "example-second-refresh" {
		t.Fatal("refresh erased another client's credentials")
	}
}

func TestSimulatedSessionTransactionsSerializeAcrossClients(t *testing.T) {
	first, path := testHTTPClient(t)
	second, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	first.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
		entered <- struct{}{}
		<-release
		return simulatedResponse(r, 200, `{"data":{}}`, http.Header{}), nil
	}))
	second.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
		entered <- struct{}{}
		return simulatedResponse(r, 200, `{"data":{}}`, http.Header{}), nil
	}))
	results := make(chan error, 2)
	call := func(c *Client) {
		_, err := c.Post(context.Background(), flightInitial, map[string]any{}, Shopper{Market: "SG", Locale: "en-SG", Currency: "SGD"})
		results <- err
	}
	go call(first)
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first request did not start")
	}
	go call(second)
	overlap := false
	select {
	case <-entered:
		overlap = true
	case <-time.After(40 * time.Millisecond):
	}
	close(release)
	for i := 0; i < 2; i++ {
		select {
		case err := <-results:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			t.Fatal("session transaction did not release lock")
		}
	}
	if overlap {
		t.Fatal("two clients sent requests concurrently using the same private session")
	}
}

func TestSimulatedSessionLockWaitHonorsDeadline(t *testing.T) {
	for _, sharedClient := range []bool{false, true} {
		t.Run(fmt.Sprint(sharedClient), func(t *testing.T) {
			first, path := testHTTPClient(t)
			second, err := NewClient(path)
			if err != nil {
				t.Fatal(err)
			}
			if sharedClient {
				second = first
			}
			entered := make(chan struct{})
			release := make(chan struct{})
			first.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
				close(entered)
				<-release
				return simulatedResponse(r, 200, `{"data":{}}`, http.Header{}), nil
			}))
			done := make(chan error, 1)
			go func() {
				_, err := first.Post(context.Background(), flightInitial, map[string]any{}, Shopper{Market: "SG", Locale: "en-SG", Currency: "SGD"})
				done <- err
			}()
			<-entered
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
			defer cancel()
			start := time.Now()
			_, err = second.Post(ctx, flightInitial, map[string]any{}, Shopper{Market: "SG", Locale: "en-SG", Currency: "SGD"})
			close(release)
			if e := <-done; e != nil {
				t.Fatal(e)
			}
			var api *APIError
			if !errors.As(err, &api) || api.Code != "TIMEOUT" || time.Since(start) > 500*time.Millisecond {
				t.Fatalf("session wait ignored deadline: %v", err)
			}
		})
	}
}
