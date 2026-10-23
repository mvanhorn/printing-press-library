package client

import (
	"github.com/mvanhorn/printing-press-library/library/cloud/cloud-run-admin/internal/config"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type accountTransport func(*http.Request) (*http.Response, error)

func (f accountTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAccountRefreshIsolation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, key := range []string{"CLOUD_RUN_ADMIN_CONFIG", "CLOUD_RUN_ADMIN_ACCOUNT", "CLOUD_RUN_ADMIN_OAUTH2C"} {
		t.Setenv(key, "")
	}
	var cacheDirs []string
	for _, name := range []string{"bestself", "littlemight"} {
		cfg, err := config.LoadAccount("", name)
		if err != nil {
			t.Fatal(err)
		}
		if err := cfg.SaveTokens("client", "secret", "expired", "refresh-"+name, time.Now().Add(-time.Hour)); err != nil {
			t.Fatal(err)
		}
		c := New(cfg, time.Second, 0)
		cacheDirs = append(cacheDirs, c.cacheDir)
		c.HTTPClient = &http.Client{Transport: accountTransport(func(r *http.Request) (*http.Response, error) {
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if r.Form.Get("refresh_token") != "refresh-"+name {
				t.Fatal("wrong refresh credential")
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"access_token":"new-` + name + `","expires_in":3600}`)), Header: make(http.Header)}, nil
		})}
		header, err := c.authHeader()
		if err != nil || header != "Bearer new-"+name {
			t.Fatal("refresh failed", err)
		}
		saved, err := config.LoadAccount("", name)
		if err != nil || saved.AccessToken != "new-"+name || saved.RefreshToken != "refresh-"+name {
			t.Fatal("refresh persistence", err)
		}
		c.Config.TokenExpiry = time.Now().Add(-time.Hour)
		c.HTTPClient.Transport = accountTransport(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 400, Body: io.NopCloser(strings.NewReader("secret-error-body"))}, nil
		})
		_, err = c.authHeader()
		if err == nil || strings.Contains(err.Error(), "secret-error-body") {
			t.Fatal("error redaction")
		}
	}
	if cacheDirs[0] == cacheDirs[1] {
		t.Fatal("shared cache")
	}
}

func TestDefaultInvalidationPreservesNamedCaches(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	named := New(&config.Config{Account: "bestself"}, time.Second, 0)
	if err := os.MkdirAll(named.cacheDir, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(named.cacheDir, "cached.json")
	if err := os.WriteFile(marker, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	legacy := New(&config.Config{Account: "default"}, time.Second, 0)
	legacy.invalidateCache()
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("default deleted named account cache", err)
	}
}
