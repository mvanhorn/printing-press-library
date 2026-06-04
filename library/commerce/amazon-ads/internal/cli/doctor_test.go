package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorValidatesRefreshTokenCredentials(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	clearAmazonAdsEnvForCLITest(t)

	var tokenCalls int
	var profileCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			tokenCalls++
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm: %v", err)
			}
			if got := r.Form.Get("refresh_token"); got != "refresh-from-file" {
				t.Fatalf("refresh_token = %q, want refresh-from-file", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"fresh-access","expires_in":3600}`))
		case "/v2/profiles":
			profileCalls++
			if got := r.Header.Get("Authorization"); got != "Bearer fresh-access" {
				t.Fatalf("Authorization = %q, want Bearer fresh-access", got)
			}
			if got := r.Header.Get("Amazon-Advertising-API-ClientId"); got != "client-from-file" {
				t.Fatalf("client ID header = %q, want client-from-file", got)
			}
			if got := r.Header.Get("Amazon-Advertising-API-Scope"); got != "profile-from-file" {
				t.Fatalf("scope header = %q, want profile-from-file", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"profileId":"profile-from-file"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	configDir := filepath.Join(home, ".config", "amazon-ads-pp-cli")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	configPath := filepath.Join(configDir, "config.toml")
	dotEnv := strings.Join([]string{
		"AMAZON_ADS_CLIENT_ID=client-from-file",
		"AMAZON_ADS_CLIENT_SECRET=secret-from-file",
		"AMAZON_ADS_REFRESH_TOKEN=refresh-from-file",
		"AMAZON_ADS_PROFILE_ID=profile-from-file",
		"AMAZON_ADS_BASE_URL=" + srv.URL,
		"AMAZON_ADS_TOKEN_URL=" + srv.URL + "/token",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(configDir, ".env"), []byte(dotEnv), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	root := RootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"--config", configPath, "doctor", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("doctor returned error: %v\noutput:\n%s", err, out.String())
	}

	var report map[string]any
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("parse doctor output: %v\n%s", err, out.String())
	}
	if got := report["auth"]; got != "configured (oauth2 refresh)" {
		t.Fatalf("auth = %v, want configured", got)
	}
	if got := report["credentials"]; got != "valid" {
		t.Fatalf("credentials = %v, want valid; report=%v", got, report)
	}
	if tokenCalls != 1 {
		t.Fatalf("tokenCalls = %d, want 1", tokenCalls)
	}
	if profileCalls < 1 {
		t.Fatalf("profileCalls = %d, want at least 1", profileCalls)
	}
}

func TestDoctorUnauthenticatedHintUsesLoginFlow(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	clearAmazonAdsEnvForCLITest(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"missing auth"}`))
	}))
	defer srv.Close()

	configDir := filepath.Join(home, ".config", "amazon-ads-pp-cli")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	configPath := filepath.Join(configDir, "config.toml")
	dotEnv := strings.Join([]string{
		"AMAZON_ADS_CLIENT_ID=client-from-file",
		"AMAZON_ADS_CLIENT_SECRET=secret-from-file",
		"AMAZON_ADS_REFRESH_TOKEN=",
		"AMAZON_ADS_PROFILE_ID=profile-from-file",
		"AMAZON_ADS_BASE_URL=" + srv.URL,
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(configDir, ".env"), []byte(dotEnv), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	root := RootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"--config", configPath, "doctor", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("doctor returned error: %v\noutput:\n%s", err, out.String())
	}

	var report map[string]any
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("parse doctor output: %v\n%s", err, out.String())
	}
	hint, _ := report["auth_hint"].(string)
	if !strings.Contains(hint, "amazon-ads-pp-cli auth login --port 8085") {
		t.Fatalf("auth_hint = %q, want explicit auth login command", hint)
	}
	if !strings.Contains(hint, defaultOAuthCallbackURL) {
		t.Fatalf("auth_hint = %q, want callback URL", hint)
	}
}

func clearAmazonAdsEnvForCLITest(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"AMAZON_ADS_CLIENT_ID",
		"AMAZON_ADS_CLIENT_SECRET",
		"AMAZON_ADS_REFRESH_TOKEN",
		"AMAZON_ADS_API_CLIENT_ID",
		"AMAZON_ADS_PROFILE_ID",
		"AMAZON_ADS_BASE_URL",
		"AMAZON_ADS_AUTHORIZATION_URL",
		"AMAZON_ADS_TOKEN_URL",
		"AMAZON_ADS_CONFIG",
	} {
		t.Setenv(key, "")
	}
}
