package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/config"
)

func etsyRefreshConfig(t *testing.T, serverURL string) *config.Config {
	t.Helper()
	restore, err := cliutil.SetHomeOverride("")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restore)
	home := testenv.Isolate(t, cliutil.ConfigDir, cliutil.DataDir, cliutil.StateDir, cliutil.CacheDir)
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "localapp"))
	t.Setenv("APPDATA", filepath.Join(home, "appdata"))
	for _, name := range []string{"ETSY_OAUTH2", "ETSY_API_KEY", "ETSY_SHARED_SECRET", "ETSY_SHOP_ID", "ETSY_ACCESS_TOKEN", "ETSY_REFRESH_TOKEN", "ETSY_CLIENT_ID", "ETSY_CLIENT_SECRET", "ETSY_BASE_URL", "ETSY_TOKEN_URL", "ETSY_AUTHORIZATION_URL", "ETSY_CA_CERT", "ETSY_TLS_SERVER_NAME", "PRINTING_PRESS_VERIFY"} {
		t.Setenv(name, "")
	}
	return &config.Config{
		BaseURL: serverURL, TokenURL: serverURL + "/token", ClientID: "testkey",
		AccessToken: "old-at", RefreshToken: "old-rt",
		TokenExpiry: time.Now().Add(-time.Minute), Path: filepath.Join(home, "config.toml"),
	}
}

func etsyRefreshGet(t *testing.T, cfg *config.Config, srv *httptest.Server) {
	t.Helper()
	c := New(cfg, 5*time.Second, 0)
	c.HTTPClient = srv.Client()
	c.NoCache = true
	body, err := c.Get(context.Background(), "/api", nil)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(body) != `{"ok":true}` {
		t.Fatalf("API response = %s", body)
	}
}

func etsyReloadTokens(t *testing.T, cfg *config.Config, refresh, access string) *config.Config {
	t.Helper()
	loaded, err := config.Load(cfg.Path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if loaded.RefreshToken != refresh || loaded.AccessToken != access || !loaded.TokenExpiry.After(time.Now()) {
		t.Fatalf("persisted tokens = (%q, %q, %v), want (%q, %q, future expiry)", loaded.RefreshToken, loaded.AccessToken, loaded.TokenExpiry, refresh, access)
	}
	return loaded
}

func TestEtsyRefreshRotationPersistsNewRefreshToken(t *testing.T) {
	var rounds atomic.Int32
	var apiCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/token" {
			round := rounds.Add(1)
			wantRT, nextRT, nextAT := "old-rt", "new-rt", "new-at"
			if round == 2 {
				wantRT, nextRT, nextAT = "new-rt", "newer-rt", "newer-at"
			}
			if err := r.ParseForm(); err != nil {
				t.Error(err)
			}
			_, secretPresent := r.PostForm["client_secret"]
			if round > 2 || r.Method != http.MethodPost || r.PostForm.Get("grant_type") != "refresh_token" || r.PostForm.Get("refresh_token") != wantRT || r.PostForm.Get("client_id") != "testkey" || secretPresent {
				t.Errorf("unexpected refresh round %d: method=%s form=%v", round, r.Method, r.PostForm)
				http.Error(w, "invalid refresh", http.StatusBadRequest)
				return
			}
			fmt.Fprintf(w, `{"access_token":%q,"refresh_token":%q,"expires_in":3600}`, nextAT, nextRT)
			return
		}
		apiCalls.Add(1)
		want := "Bearer new-at"
		if rounds.Load() == 2 {
			want = "Bearer newer-at"
		}
		if got := r.Header.Get("Authorization"); got != want {
			t.Errorf("Authorization = %q, want %q", got, want)
		}
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer srv.Close()
	cfg := etsyRefreshConfig(t, srv.URL)
	etsyRefreshGet(t, cfg, srv)
	if cfg.RefreshToken != "new-rt" {
		t.Fatalf("in-memory refresh token = %q", cfg.RefreshToken)
	}
	loaded := etsyReloadTokens(t, cfg, "new-rt", "new-at")
	// Expire the persisted access token too: a fresh disk token would rightly
	// be adopted by the refresh lock instead of issuing a second grant.
	if err := loaded.WithOAuthRefreshLock(loaded.RefreshToken, func(_ []*cliutil.Credentials, save func() error, _ func(string) error) error {
		loaded.StageOAuthTokens(loaded.ClientID, loaded.ClientSecret, loaded.AccessToken, loaded.RefreshToken, time.Now().Add(-time.Minute))
		return save()
	}); err != nil {
		t.Fatal(err)
	}
	etsyRefreshGet(t, loaded, srv)
	etsyReloadTokens(t, loaded, "newer-rt", "newer-at")
	if rounds.Load() != 2 || apiCalls.Load() != 2 {
		t.Fatalf("refresh/API calls = %d/%d, want 2/2", rounds.Load(), apiCalls.Load())
	}
}

func TestEtsyRefreshWithoutRotationKeepsRefreshToken(t *testing.T) {
	var rounds atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/token" {
			rounds.Add(1)
			fmt.Fprint(w, `{"access_token":"new-at","expires_in":3600}`)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer new-at" {
			t.Errorf("Authorization = %q", got)
		}
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer srv.Close()
	cfg := etsyRefreshConfig(t, srv.URL)
	etsyRefreshGet(t, cfg, srv)
	etsyReloadTokens(t, cfg, "old-rt", "new-at")
	if rounds.Load() != 1 || cfg.RefreshToken != "old-rt" {
		t.Fatalf("refresh calls/token = %d/%q", rounds.Load(), cfg.RefreshToken)
	}
}

func TestEtsyRefreshRefusesCrossOriginRedirect(t *testing.T) {
	var redirected atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirected.Add(1)
		fmt.Fprint(w, `{"access_token":"leaked-at","expires_in":3600}`)
	}))
	defer target.Close()
	var tokenCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/token" {
			t.Error("API reached despite failed refresh")
			http.Error(w, "unexpected API call", http.StatusBadRequest)
			return
		}
		tokenCalls.Add(1)
		http.Redirect(w, r, target.URL+"/token", http.StatusTemporaryRedirect)
	}))
	defer srv.Close()
	cfg := etsyRefreshConfig(t, srv.URL)
	c := New(cfg, 5*time.Second, 0)
	c.HTTPClient = srv.Client()
	_, err := c.Get(context.Background(), "/api", nil)
	if err == nil {
		t.Fatal("cross-origin refresh redirect succeeded")
	}
	for _, secret := range []string{cfg.RefreshToken, cfg.ClientID, "refresh_token", "client_id"} {
		if strings.Contains(err.Error(), secret) {
			t.Errorf("refresh error exposes %q: %v", secret, err)
		}
	}
	if tokenCalls.Load() != 1 || redirected.Load() != 0 {
		t.Fatalf("token/redirect requests = %d/%d, want 1/0", tokenCalls.Load(), redirected.Load())
	}
}
