package edeka

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"
)

// rewriteTransport sends requests for the shop host to a local test server,
// so cookies keep the real edeka.shop host the session code expects.
type rewriteTransport struct{ target *url.URL }

func (t rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	r.URL.Scheme = t.target.Scheme
	r.URL.Host = t.target.Host
	return http.DefaultTransport.RoundTrip(r)
}

// TestRenewedRememberCookieKeepsNewExpiry: when the shop renews the
// remember-me cookie with a later expiry, Persist must write the new value
// and the new expiry to the session file, not only the value.
func TestRenewedRememberCookieKeepsNewExpiry(t *testing.T) {
	const year = 365 * 24 * 3600
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: RememberCookie, Value: "renewed", Path: "/", MaxAge: year})
		io.WriteString(w, `<a class="btn button-service" aria-label="Mein Konto"></a>`)
	}))
	defer srv.Close()
	target, _ := url.Parse(srv.URL)

	host := "test-market.edeka.shop"
	path := filepath.Join(t.TempDir(), "session.json")
	oldExp := time.Now().Add(time.Hour)
	s := &Session{Market: "test-market", Cookies: []Cookie{{Name: RememberCookie, Value: "placeholder", Domain: host, Path: "/", Expires: float64(oldExp.Unix())}}}
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	sh := NewTestShop(&http.Client{Transport: rewriteTransport{target}}, s, "https://"+host)
	sh.Path = path
	if ok, err := sh.LoggedIn(context.Background()); err != nil || !ok {
		t.Fatalf("LoggedIn = %v, %v", ok, err)
	}
	if err := sh.Persist(); err != nil {
		t.Fatal(err)
	}
	back, err := LoadSession(path)
	if err != nil {
		t.Fatal(err)
	}
	if !back.Has(RememberCookie, host) {
		t.Fatal("remember-me cookie lost")
	}
	exp := back.RememberExpiry(host)
	if want := time.Now().Add(year*time.Second - time.Hour); exp.Before(want) {
		t.Fatalf("renewed expiry not persisted: got %s, want after %s", exp, want)
	}
	for _, c := range back.Cookies {
		if c.Name == RememberCookie && c.Value != "renewed" {
			t.Fatalf("renewed value not persisted: %q", c.Value)
		}
	}
}

// TestUpdateMarketKeepsRotatedCookies: selecting a market re-reads the session
// under the lock, so a cookie another run rotated in between is kept.
func TestUpdateMarketKeepsRotatedCookies(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	s := &Session{Market: "a", Stoken: "tok", Cookies: []Cookie{{Name: "sid", Value: "v1", Domain: "a.edeka.shop", Path: "/"}}}
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	other, _ := LoadSession(path)
	other.Cookies[0].Value = "v2"
	if err := other.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := UpdateMarket(path, "b")
	if err != nil {
		t.Fatal(err)
	}
	back, _ := LoadSession(path)
	if got.Market != "b" || back.Market != "b" || back.Stoken != "" || back.Cookies[0].Value != "v2" {
		t.Fatalf("after UpdateMarket: market=%q stoken=%q sid=%q", back.Market, back.Stoken, back.Cookies[0].Value)
	}
	if _, err := UpdateMarket(filepath.Join(t.TempDir(), "missing.json"), "b"); err != ErrNoSession {
		t.Fatalf("missing session: want ErrNoSession, got %v", err)
	}
}

// TestPersistKeepsMarketSelectedElsewhere: a long-running command must not
// write back the market it loaded at start over a newer selection.
func TestPersistKeepsMarketSelectedElsewhere(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	s := &Session{Market: "a", Cookies: []Cookie{{Name: "sid", Value: "v1", Domain: "a.edeka.shop", Path: "/"}}}
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	sh, err := OpenWith(s, path, "a.edeka.shop")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := UpdateMarket(path, "b"); err != nil {
		t.Fatal(err)
	}
	if err := sh.Persist(); err != nil {
		t.Fatal(err)
	}
	back, _ := LoadSession(path)
	if back.Market != "b" {
		t.Fatalf("Persist overwrote the selected market: %q", back.Market)
	}
}
