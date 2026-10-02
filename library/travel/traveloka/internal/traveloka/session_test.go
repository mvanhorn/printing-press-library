package traveloka

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const flightInitial = "/api/v2/flight/search/initial"

func syntheticCookies() []scopedCookie {
	return []scopedCookie{{Name: "guest", Value: "synthetic-cookie-secret", Domain: ".traveloka.com", Path: "/", Expires: "-1", HTTPOnly: true, Secure: true, SameSite: "Lax"}}
}
func syntheticRequest(path string) requestCapture {
	body, _ := json.Marshal(map[string]any{"clientInterface": "desktop", "fields": []any{}, "data": map[string]any{"currency": "SGD", "locale": "en_SG", "searchId": "captured-old-id", "original": "template", "nested": map[string]any{"mutable": "original"}}, "sentinel": map[string]any{"token": "synthetic-sentinel-secret"}})
	return requestCapture{Method: "POST", URL: origin + path, Headers: map[string]string{"user-agent": "synthetic-UA", "www-app-version": "observed-version", "t-a-v": "synthetic-header-secret", "tv-clientsessionid": "synthetic-session-secret", "x-did": "synthetic-device-secret", "Origin": "https://evil.example", "Cookie": "bad=header-must-not-import", "Authorization": "Bearer must-not-import"}, Body: string(body)}
}
func writeFixtureJSON(t *testing.T, path string, value any) {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
}
func importedFixture(t *testing.T, paths ...string) (string, *SessionInfo) {
	t.Helper()
	d := t.TempDir()
	cp, rp, sp := filepath.Join(d, "cookies.json"), filepath.Join(d, "requests.json"), filepath.Join(d, "session.json")
	writeFixtureJSON(t, cp, syntheticCookies())
	requests := []requestCapture{}
	for _, path := range paths {
		requests = append(requests, syntheticRequest(path))
	}
	writeFixtureJSON(t, rp, requests)
	info, err := ImportSession(cp, rp, sp)
	if err != nil {
		t.Fatal(err)
	}
	return sp, info
}
func TestSimulatedImportSessionScopeAndPermissions(t *testing.T) {
	path, info := importedFixture(t, flightInitial, "/api/v1/hotel/autocomplete")
	if info.Path != path || info.Cookies != 1 || info.Profiles != 2 || info.CapturedAt == "" {
		t.Fatal("incorrect safe metadata")
	}
	b, _ := json.Marshal(info)
	if strings.Contains(string(b), "synthetic") {
		t.Fatal("session metadata leaks secrets")
	}
	st, err := os.Stat(path)
	if err != nil || st.Mode().Perm() != 0600 {
		t.Fatal("private session mode must be 0600")
	}
	c, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	data, err := c.TemplateData(flightInitial)
	if err != nil || data["original"] != "template" {
		t.Fatal("captured data missing")
	}
	data["nested"].(map[string]any)["mutable"] = "changed"
	again, _ := c.TemplateData(flightInitial)
	if again["nested"].(map[string]any)["mutable"] != "original" {
		t.Fatal("template copy must be defensive")
	}
	if _, err = c.TemplateData("/api/v2/booking"); err == nil {
		t.Fatal("unsupported operation accepted")
	}
	if _, err = c.TemplateData("/api/v2/hotel/search/rooms"); err == nil {
		t.Fatal("uncaptured profile accepted")
	}
}
func TestSimulatedImportRejectsUnscopedSources(t *testing.T) {
	tests := []struct {
		name   string
		change func(*[]scopedCookie, *[]requestCapture)
	}{
		{"foreign cookie", func(c *[]scopedCookie, r *[]requestCapture) { (*c)[0].Domain = ".evil.example" }},
		{"subdomain cookie", func(c *[]scopedCookie, r *[]requestCapture) { (*c)[0].Domain = "sub.traveloka.com" }},
		{"suffix cookie", func(c *[]scopedCookie, r *[]requestCapture) { (*c)[0].Domain = "traveloka.com.evil.example" }},
		{"foreign request", func(c *[]scopedCookie, r *[]requestCapture) { (*r)[0].URL = "https://evil.example" + flightInitial }},
		{"credential URL", func(c *[]scopedCookie, r *[]requestCapture) {
			(*r)[0].URL = "https://user:pass@www.traveloka.com" + flightInitial
		}},
		{"plain HTTP", func(c *[]scopedCookie, r *[]requestCapture) { (*r)[0].URL = "http://www.traveloka.com" + flightInitial }},
		{"GET", func(c *[]scopedCookie, r *[]requestCapture) { (*r)[0].Method = "GET" }},
		{"booking path", func(c *[]scopedCookie, r *[]requestCapture) { (*r)[0].URL = origin + "/api/v2/booking" }},
		{"encoded path", func(c *[]scopedCookie, r *[]requestCapture) { (*r)[0].URL = origin + "/api/v2/flight/search/%69nitial" }},
		{"URL query", func(c *[]scopedCookie, r *[]requestCapture) { (*r)[0].URL += "?token=secret" }},
		{"cookie path", func(c *[]scopedCookie, r *[]requestCapture) { (*c)[0].Path = "relative" }},
		{"expiry", func(c *[]scopedCookie, r *[]requestCapture) { (*c)[0].Expires = "-2" }},
		{"malformed body", func(c *[]scopedCookie, r *[]requestCapture) { (*r)[0].Body = "not-json synthetic-body-secret" }},
		{"missing data", func(c *[]scopedCookie, r *[]requestCapture) { (*r)[0].Body = `{"sentinel":"synthetic-body-secret"}` }},
		{"empty cookies", func(c *[]scopedCookie, r *[]requestCapture) { *c = []scopedCookie{} }},
		{"empty requests", func(c *[]scopedCookie, r *[]requestCapture) { *r = []requestCapture{} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := t.TempDir()
			cookies := syntheticCookies()
			requests := []requestCapture{syntheticRequest(flightInitial)}
			tt.change(&cookies, &requests)
			cp, rp, sp := filepath.Join(d, "c.json"), filepath.Join(d, "r.json"), filepath.Join(d, "s.json")
			writeFixtureJSON(t, cp, cookies)
			writeFixtureJSON(t, rp, requests)
			_, err := ImportSession(cp, rp, sp)
			if err == nil {
				t.Fatal("unsafe import accepted")
			}
			if strings.Contains(err.Error(), "synthetic") {
				t.Fatal("import error leaks values")
			}
			if _, err = os.Stat(sp); !os.IsNotExist(err) {
				t.Fatal("failed import wrote output")
			}
		})
	}
}
func TestSimulatedNewClientRejectsMissingPublicOrExpiredSession(t *testing.T) {
	if _, err := NewClient(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing session accepted")
	}
	path, _ := importedFixture(t, flightInitial)
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewClient(path); err == nil {
		t.Fatal("public session accepted")
	}
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	var s privateSession
	_ = decodeJSON(b, &s)
	s.Cookies[0].Expires = "1"
	writeFixtureJSON(t, path, s)
	if _, err := NewClient(path); err == nil {
		t.Fatal("expired session accepted")
	}
	target := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(path, target); err != nil {
		t.Fatal(err)
	}
	if _, err := NewClient(target); err == nil {
		t.Fatal("symlink session accepted")
	}
}
func TestSimulatedImportMalformedArrays(t *testing.T) {
	d := t.TempDir()
	c, r, o := filepath.Join(d, "c"), filepath.Join(d, "r"), filepath.Join(d, "o")
	writeFixtureJSON(t, c, syntheticCookies())
	writeFixtureJSON(t, r, []requestCapture{syntheticRequest(flightInitial)})
	if err := os.WriteFile(c, []byte(`{"name":"synthetic-secret"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportSession(c, r, o); err == nil {
		t.Fatal("object cookie input accepted")
	}
	writeFixtureJSON(t, c, syntheticCookies())
	if err := os.WriteFile(r, []byte(`null`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportSession(c, r, o); err == nil {
		t.Fatal("null requests input accepted")
	}
}

func TestSimulatedCookieExpiryAndValidationSemantics(t *testing.T) {
	for _, expiry := range []json.Number{"-0.5", "-2"} {
		cookie := syntheticCookies()[0]
		cookie.Expires = expiry
		if err := validateCookie(cookie); err == nil {
			t.Fatal("invalid negative expiry accepted")
		}
	}
	cookie := syntheticCookies()[0]
	cookie.Expires = "0"
	if cookieHTTP(cookie).Expires.IsZero() {
		t.Fatal("epoch expiry incorrectly promoted to a session cookie")
	}
	cookie = syntheticCookies()[0]
	cookie.Value = "invalid\x00cookie"
	if err := validateCookie(cookie); err == nil {
		t.Fatal("invalid cookie bytes accepted")
	}
}

func TestSimulatedOperationScopedCookieIsAccepted(t *testing.T) {
	path, _ := importedFixture(t, flightInitial)
	b, _ := os.ReadFile(path)
	var session privateSession
	if err := decodeJSON(b, &session); err != nil {
		t.Fatal(err)
	}
	session.Cookies[0].Path = "/api/v2/flight"
	writeFixtureJSON(t, path, session)
	if _, err := NewClient(path); err != nil {
		t.Fatalf("legitimate operation-scoped cookie was rejected as expired: %v", err)
	}
}

func TestSimulatedBrowserJSONCookieReplayPreservesExactValues(t *testing.T) {
	d := t.TempDir()
	cp, rp, sp := filepath.Join(d, "cookies.json"), filepath.Join(d, "requests.json"), filepath.Join(d, "session.json")
	cookies := syntheticCookies()
	for _, entry := range []struct{ name, value string }{{"g_state", `{"i_l":0,"state":"synthetic-json"}`}, {"tv_user", `"synthetic-guest"`}, {"exp-client-service-affinity-cookie", `"synthetic-affinity"`}} {
		cookie := syntheticCookies()[0]
		cookie.Name = entry.name
		cookie.Value = entry.value
		cookies = append(cookies, cookie)
	}
	writeFixtureJSON(t, cp, cookies)
	writeFixtureJSON(t, rp, []requestCapture{syntheticRequest(flightInitial)})
	if _, err := ImportSession(cp, rp, sp); err != nil {
		t.Fatalf("browser JSON-style cookie import failed: %v", err)
	}
	client, err := NewClient(sp)
	if err != nil {
		t.Fatal(err)
	}
	client.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
		raw := r.Header.Get("Cookie")
		for _, cookie := range cookies {
			if !strings.Contains(raw, cookie.Name+"="+cookie.Value) {
				t.Fatalf("browser cookie %s changed during serialization", cookie.Name)
			}
		}
		return simulatedResponse(r, 200, `{"data":{},"meta":{}}`, http.Header{}), nil
	}))
	if _, err := client.Post(context.Background(), flightInitial, map[string]any{}, Shopper{"SG", "en-SG", "SGD"}); err != nil {
		t.Fatal(err)
	}
}

func TestSimulatedBrowserCookieHeaderInjectionRejected(t *testing.T) {
	for _, unsafe := range []string{"line\rbreak", "line\nbreak", "cookie; injected=1", "tab\tvalue", "null\x00byte", "del\x7fbyte", "nonascii\u0080"} {
		d := t.TempDir()
		cp, rp, sp := filepath.Join(d, "cookies.json"), filepath.Join(d, "requests.json"), filepath.Join(d, "session.json")
		cookies := syntheticCookies()
		cookies[0].Value = unsafe
		writeFixtureJSON(t, cp, cookies)
		writeFixtureJSON(t, rp, []requestCapture{syntheticRequest(flightInitial)})
		if _, err := ImportSession(cp, rp, sp); err == nil {
			t.Fatal("unsafe browser cookie value accepted")
		}
		if _, err := browserCookieHeader([]*http.Cookie{{Name: "guest", Value: unsafe}}); err == nil {
			t.Fatal("unsafe value serialized into Cookie header")
		}
	}
}
