package config

import (
	"encoding/base64"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/wework/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/wework/internal/cliutil/testenv"
)

func jwtWithPayload(p string) string {
	return "aaa." + base64.RawURLEncoding.EncodeToString([]byte(p)) + ".bbb"
}

func mockDoer(status int, body string) RefreshDoer {
	return func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}, nil
	}
}

func TestIsWeworkIssuer(t *testing.T) {
	good := []string{"https://idp.wework.com/", "https://idp.wework.com", "https://wework.com/", "https://EU.Auth.WeWork.com/"}
	for _, s := range good {
		if !isWeworkIssuer(s) {
			t.Errorf("isWeworkIssuer(%q) = false, want true", s)
		}
	}
	bad := []string{
		"https://evil.example.com/",        // attacker host
		"https://idp.wework.com.evil.com/", // suffix-spoof
		"https://notwework.com/",           // substring, not subdomain
		"http://idp.wework.com/",           // not https
		"https://wework.com.attacker.net/", // parent-spoof
		"",                                 // empty
		"://bad",                           // unparseable
	}
	for _, s := range bad {
		if isWeworkIssuer(s) {
			t.Errorf("isWeworkIssuer(%q) = true, want false", s)
		}
	}
}

// A token whose issuer is not a WeWork host must NOT trigger a refresh (the
// refresh token must never leave for a foreign endpoint).
func TestRefreshRefusesForeignIssuer(t *testing.T) {
	// Expired token (exp in the past) with a hostile issuer + azp.
	tok := jwtWithPayload(`{"exp":1000000000,"iss":"https://evil.example.com/","azp":"someclient"}`)
	c := &Config{
		Path:         filepath.Join(t.TempDir(), "config.toml"),
		WeworkToken:  tok,
		RefreshToken: "secret-refresh",
	}
	called := false
	doer := func(*http.Request) (*http.Response, error) { called = true; return nil, nil }
	refreshed, err := c.RefreshWeworkTokenIfNeeded(doer)
	if refreshed {
		t.Fatal("refreshed against a foreign issuer, want refusal")
	}
	if err == nil {
		t.Fatal("expected an error refusing the foreign issuer")
	}
	if called {
		t.Fatal("refresh HTTP call was made to a foreign issuer — refresh token could leak")
	}
}

func TestJwtIssAzp(t *testing.T) {
	tok := jwtWithPayload(`{"iss":"https://idp.wework.com/","azp":"CLIENT123","exp":1}`)
	iss, azp := jwtIssAzp(tok)
	if iss != "https://idp.wework.com/" || azp != "CLIENT123" {
		t.Fatalf("got iss=%q azp=%q", iss, azp)
	}
	if i, a := jwtIssAzp("not-a-jwt"); i != "" || a != "" {
		t.Fatalf("expected empty for bad token, got %q/%q", i, a)
	}
}

func TestDoAuth0Refresh(t *testing.T) {
	access, refresh, expIn, err := doAuth0Refresh(
		"https://idp.wework.com/oauth/token", "client", "rt0",
		mockDoer(200, `{"access_token":"newAT","refresh_token":"newRT","expires_in":43200}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if access != "newAT" || refresh != "newRT" || expIn != 43200 {
		t.Fatalf("got access=%q refresh=%q expiresIn=%d", access, refresh, expIn)
	}

	if _, _, _, err := doAuth0Refresh("https://x/oauth/token", "c", "rt",
		mockDoer(403, `{"error":"invalid_grant","error_description":"expired"}`)); err == nil {
		t.Fatal("expected error on rejected refresh")
	}
}

func TestRefreshWeworkTokenIfNeededSkips(t *testing.T) {
	// No refresh token -> no refresh, no error, no disk write.
	c := &Config{WeworkToken: jwtWithPayload(`{"iss":"https://idp/","azp":"c","exp":1}`)}
	if did, err := c.RefreshWeworkTokenIfNeeded(mockDoer(200, `{}`)); did || err != nil {
		t.Fatalf("no refresh token: did=%v err=%v", did, err)
	}

	// Token still fresh -> no refresh.
	future := time.Now().Add(2 * time.Hour).Unix()
	fresh := jwtWithPayload(`{"iss":"https://idp/","azp":"c","exp":` + itoa(future) + `}`)
	c2 := &Config{WeworkToken: fresh, RefreshToken: "rt"}
	if did, err := c2.RefreshWeworkTokenIfNeeded(func(*http.Request) (*http.Response, error) {
		t.Fatal("should not have called the network for a fresh token")
		return nil, nil
	}); did || err != nil {
		t.Fatalf("fresh token: did=%v err=%v", did, err)
	}
}

func TestRefreshWeworkTokenNowForcesFreshToken(t *testing.T) {
	home := testenv.Isolate(t, cliutil.ConfigDir, cliutil.DataDir)
	future := time.Now().Add(2 * time.Hour).Unix()
	tok := jwtWithPayload(`{"iss":"https://idp.wework.com/","azp":"public-client","exp":` + itoa(future) + `}`)
	c := &Config{
		Path:         filepath.Join(home, "config.toml"),
		WeworkToken:  tok,
		RefreshToken: "rt-old",
		envOverrides: map[string]bool{},
	}
	called := false
	doer := func(*http.Request) (*http.Response, error) {
		called = true
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(
				`{"access_token":"new-access","refresh_token":"rt-new","expires_in":43200}`)),
		}, nil
	}
	refreshed, err := c.RefreshWeworkTokenNow(doer)
	if err != nil {
		t.Fatalf("force refresh failed: %v", err)
	}
	if !called || !refreshed {
		t.Fatalf("force refresh did not call Auth0: called=%v refreshed=%v", called, refreshed)
	}
	if c.WeworkToken != "new-access" || c.RefreshToken != "rt-new" {
		t.Fatalf("rotated credentials not retained: token=%q refresh=%q", c.WeworkToken, c.RefreshToken)
	}
}

func TestPersistedRotationBeatsStaleEnvironmentBootstrap(t *testing.T) {
	testenv.Isolate(t, cliutil.ConfigDir, cliutil.DataDir)
	oldAccess := jwtWithPayload(`{"iss":"https://idp.wework.com/","azp":"public-client","exp":1000000000}`)
	newAccess := jwtWithPayload(`{"iss":"https://idp.wework.com/","azp":"public-client","exp":4102444800}`)
	t.Setenv("WEWORK_TOKEN", oldAccess)
	t.Setenv("WEWORK_REFRESH_TOKEN", "rt-old")

	bootstrap, err := Load("")
	if err != nil {
		t.Fatalf("load environment bootstrap: %v", err)
	}
	bootstrap.ApplyWeworkAuthBootstrap()
	if err := bootstrap.SaveWeworkSession(newAccess, "rt-new", JWTExpiry(newAccess)); err != nil {
		t.Fatalf("persist rotation: %v", err)
	}

	reloaded, err := Load("")
	if err != nil {
		t.Fatalf("reload rotated session: %v", err)
	}
	reloaded.ApplyWeworkAuthBootstrap()
	if reloaded.WeworkToken != newAccess || reloaded.RefreshToken != "rt-new" {
		t.Fatalf("stale environment replayed over rotated session: token=%q refresh=%q", reloaded.WeworkToken, reloaded.RefreshToken)
	}
}

func TestConcurrentRefreshUsesOneRotatingTokenExchange(t *testing.T) {
	testenv.Isolate(t, cliutil.ConfigDir, cliutil.DataDir)
	expired := jwtWithPayload(`{"iss":"https://idp.wework.com/","azp":"public-client","exp":1}`)
	fresh := jwtWithPayload(`{"iss":"https://idp.wework.com/","azp":"public-client","exp":4102444800}`)
	bootstrap, err := Load("")
	if err != nil {
		t.Fatalf("load bootstrap config: %v", err)
	}
	if err := bootstrap.SaveWeworkAuth(expired, "rt-old", "account-1", "3"); err != nil {
		t.Fatalf("seed renewable session: %v", err)
	}
	first, err := Load("")
	if err != nil {
		t.Fatalf("load first config: %v", err)
	}
	second, err := Load("")
	if err != nil {
		t.Fatalf("load second config: %v", err)
	}

	var mu sync.Mutex
	calls := 0
	doer := func(*http.Request) (*http.Response, error) {
		mu.Lock()
		calls++
		mu.Unlock()
		time.Sleep(50 * time.Millisecond)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(
				`{"access_token":"` + fresh + `","refresh_token":"rt-new","expires_in":43200}`)),
		}, nil
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	for _, cfg := range []*Config{first, second} {
		go func(c *Config) {
			<-start
			_, refreshErr := c.RefreshWeworkTokenIfNeeded(doer)
			results <- refreshErr
		}(cfg)
	}
	close(start)
	for range 2 {
		if err := <-results; err != nil {
			t.Fatalf("concurrent refresh failed: %v", err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if calls != 1 {
		t.Fatalf("rotating refresh token exchanged %d times, want exactly once", calls)
	}
}
