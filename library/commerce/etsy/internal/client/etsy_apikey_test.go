package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/config"
)

func TestEtsyAPIKeyHeader(t *testing.T) {
	// etsy-ext: refused placeholders must never reach the HTTP server or error text.
	for _, tc := range []struct {
		name, key, secret, want    string
		refused, hint, hidden, msg string
	}{
		{name: "compose", key: "k", secret: "realsecret", want: "k:realsecret"},
		{name: "already composed", key: "k:s", secret: "s", want: "k:s"},
		{name: "key only", key: "k", want: "k"},
		{name: "empty key", secret: "s"},
		{name: "trim whitespace", key: " k ", secret: " s ", want: "k:s"},
		{name: "env placeholder", key: "k", secret: "${ETSY_SHARED_SECRET}", refused: "yes", hint: "ETSY_SHARED_SECRET", hidden: "${ETSY_SHARED_SECRET}", msg: "is an unresolved template"},
		{name: "inline placeholder", key: "k:${SECRET}", refused: "yes", hint: "ETSY_SHARED_SECRET", hidden: "${SECRET}", msg: "is an unresolved template"},
		{name: "legacy secret placeholder", key: "k:<your-token>", refused: "yes", hint: "ETSY_SHARED_SECRET", hidden: "<your-token>", msg: "looks like placeholder text"},
		{name: "literal env secret", key: "k", secret: "<your-shared-secret>", refused: "yes", hint: "ETSY_SHARED_SECRET", hidden: "<your-shared-secret>", msg: "looks like placeholder text"},
		{name: "key placeholder", key: "${KEY}:s", refused: "yes", hint: "ETSY_API_KEY", hidden: "${KEY}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			restore, err := cliutil.SetHomeOverride("")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(restore)
			home := testenv.Isolate(t, cliutil.ConfigDir, cliutil.DataDir, cliutil.StateDir, cliutil.CacheDir)
			t.Setenv("ETSY_SHARED_SECRET", tc.secret)
			t.Setenv("LOCALAPPDATA", filepath.Join(home, "localapp"))
			t.Setenv("APPDATA", filepath.Join(home, "appdata"))
			var mu sync.Mutex
			var got string
			var present bool
			var calls int
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				got = r.Header.Get("x-api-key")
				_, present = r.Header[http.CanonicalHeaderKey("x-api-key")]
				calls++
				mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"ok":true}`))
			}))
			defer srv.Close()
			cfg := &config.Config{BaseURL: srv.URL, EtsyApiKey: tc.key}
			c := New(cfg, 5*time.Second, 0)
			c.HTTPClient = srv.Client()
			_, err = c.Get(context.Background(), "/etsy-key-probe", nil)
			if tc.refused != "" {
				if !errors.Is(err, ErrPlaceholderCredential) {
					t.Fatal("expected placeholder credential error")
				}
				// etsy-ext: shared-secret guidance retains sentinel identity with safe wording.
				if tc.hint == "ETSY_SHARED_SECRET" {
					etsyOtherMessage := "looks like placeholder text"
					if tc.msg == etsyOtherMessage {
						etsyOtherMessage = "unresolved template"
					}
					if !strings.Contains(err.Error(), "ETSY_SHARED_SECRET "+tc.msg) || strings.Contains(err.Error(), etsyOtherMessage) || strings.Contains(err.Error(), "placeholder credential") {
						t.Fatal("incorrect shared-secret guidance")
					}
				} else if !strings.Contains(err.Error(), "placeholder credential") {
					t.Fatal("API key guidance changed")
				}
				// The generic setup hint itself uses <your-shared-secret>; inspect the diagnostic separately.
				etsyDiagnostic := strings.SplitN(err.Error(), "; set it with:", 2)[0]
				if strings.Contains(etsyDiagnostic, tc.hidden) || !strings.Contains(err.Error(), tc.hint) {
					t.Fatal("error leaked a credential or omitted the setup variable")
				}
				mu.Lock()
				defer mu.Unlock()
				if calls != 0 {
					t.Fatal("refused credential reached the server")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			mu.Lock()
			defer mu.Unlock()
			if calls != 1 || got != tc.want || present != (tc.want != "") {
				t.Fatalf("calls=%d header=%q present=%v; want one call, %q, present=%v", calls, got, present, tc.want, tc.want != "")
			}
			if cfg.EtsyApiKey != tc.key {
				t.Fatal("request changed the persisted API key")
			}
		})
	}
}

func TestEtsyAPIKeyHelpers(t *testing.T) {
	for _, secret := range []string{"", " s "} {
		t.Run("secret="+secret, func(t *testing.T) {
			t.Setenv("ETSY_SHARED_SECRET", secret)
			for _, key := range []string{"", "   "} {
				cfg := &config.Config{EtsyApiKey: key}
				if got := cfg.ComposedAPIKey(); got != "" {
					t.Fatalf("empty key composed to %q", got)
				}
				if cfg.APIKeyHasSharedSecret() {
					t.Fatal("empty key reported a shared secret")
				}
				if cfg.SharedSecretConfigured() != (secret != "") {
					t.Fatal("incorrect shared secret configuration status")
				}
			}
			cfg := &config.Config{EtsyApiKey: "k:s"}
			if !cfg.APIKeyHasSharedSecret() || cfg.ComposedAPIKey() != "k:s" {
				t.Fatal("precomposed key was not preserved")
			}
		})
	}
}
