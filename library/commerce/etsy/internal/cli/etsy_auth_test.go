package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/config"
)

func TestEtsyDefaultScopes(t *testing.T) {
	want := []string{"listings_r", "shops_r", "transactions_r"}
	got := etsyDefaultScopes()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scopes = %v, want %v", got, want)
	}
	got[0] = "listings_w"
	if !reflect.DeepEqual(etsyDefaultScopes(), want) {
		t.Fatal("default scopes share mutable storage")
	}
}

func TestEtsyRequestedScopesDefaultDrift(t *testing.T) {
	t.Setenv("ETSY_SCOPES", "")
	if err := os.Unsetenv("ETSY_SCOPES"); err != nil {
		t.Fatal(err)
	}
	if got := etsyRequestedScopes(false, nil); !reflect.DeepEqual(got, etsyDefaultScopes()) {
		t.Fatalf("requested defaults = %v, want %v", got, etsyDefaultScopes())
	}
}

func TestEtsyKeystring(t *testing.T) {
	for _, tc := range []struct{ input, want string }{{"k:s", "k"}, {"k", "k"}, {"", ""}, {" k :s:other", "k"}} {
		if got := etsyKeystring(tc.input); got != tc.want {
			t.Errorf("keystring(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
	if got := etsyDefaultClientID(nil); got != "" {
		t.Fatalf("nil config ID = %q", got)
	}
	if got := etsyDefaultClientID(&config.Config{EtsyApiKey: "key:secret"}); got != "key" {
		t.Fatalf("config ID = %q", got)
	}
}

func TestBuildEtsyAuthorizeURL(t *testing.T) {
	for _, base := range []string{"", "https://example.invalid/connect"} {
		for _, redirect := range []string{"", "http://127.0.0.1:8085/callback"} {
			u, err := url.Parse(buildEtsyAuthorizeURL(base, "client", redirect, "state", "challenge", etsyDefaultScopes()))
			if err != nil {
				t.Fatal(err)
			}
			wantBase := base
			if wantBase == "" {
				wantBase = etsyAuthorizeURL
			}
			if strings.Split(u.String(), "?")[0] != wantBase {
				t.Fatalf("base = %s", u)
			}
			want := url.Values{"client_id": {"client"}, "response_type": {"code"}, "state": {"state"}, "code_challenge": {"challenge"}, "code_challenge_method": {"S256"}, "scope": {"listings_r shops_r transactions_r"}}
			if redirect != "" {
				want.Set("redirect_uri", redirect)
			}
			if !reflect.DeepEqual(u.Query(), want) {
				t.Fatalf("params = %v, want %v", u.Query(), want)
			}
		}
	}
	u, err := url.Parse(buildEtsyAuthorizeURL("", "id", "", "state", "", make([]string, 0)))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"scope", "code_challenge", "code_challenge_method", "redirect_uri"} {
		if u.Query().Has(key) {
			t.Errorf("unexpected %s", key)
		}
	}
}

func TestEtsyLoginDryRun(t *testing.T) {
	for _, tc := range []struct {
		name, envID, envScopes string
		args                   []string
		wantID, wantScopes     string
	}{
		{name: "all scopes", args: []string{"--all-scopes"}, wantID: "testkey", wantScopes: "listings_r shops_r transactions_r email_r listings_w listings_d shops_w transactions_w address_r address_w profile_r profile_w"},
		{name: "explicit scopes override all", envScopes: "email_r", args: []string{"--all-scopes", "--scopes", "listings_r"}, wantID: "testkey", wantScopes: "listings_r"},
		{name: "environment scopes override all", envScopes: "email_r,listings_r", args: []string{"--all-scopes"}, wantID: "testkey", wantScopes: "email_r listings_r"},
		{name: "keystring", wantID: "testkey", wantScopes: "listings_r shops_r transactions_r"},
		{name: "environment ID", envID: "envid", wantID: "envid", wantScopes: "listings_r shops_r transactions_r"},
		{name: "flag ID", envID: "envid", args: []string{"--client-id", "flagid"}, wantID: "flagid", wantScopes: "listings_r shops_r transactions_r"},
		{name: "flag scopes", envScopes: "email_r", args: []string{"--scopes", "listings_w,shops_r"}, wantID: "testkey", wantScopes: "listings_w shops_r"},
		{name: "environment scopes", envScopes: "email_r listings_r", wantID: "testkey", wantScopes: "email_r listings_r"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			restore, err := cliutil.SetHomeOverride("")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(restore)
			home := testenv.Isolate(t, cliutil.ConfigDir, cliutil.DataDir, cliutil.StateDir, cliutil.CacheDir)
			t.Setenv("LOCALAPPDATA", filepath.Join(home, "localapp"))
			t.Setenv("APPDATA", filepath.Join(home, "appdata"))
			for _, key := range []string{"PRINTING_PRESS_VERIFY", "DOGFOOD", "PRINTING_PRESS_DOGFOOD", "ETSY_CLIENT_SECRET", "ETSY_SHARED_SECRET", "ETSY_CA_CERT", "ETSY_AUTHORIZATION_URL", "ETSY_OAUTH2", "ETSY_REDIRECT_URI"} {
				t.Setenv(key, "")
			}
			t.Setenv("ETSY_API_KEY", "testkey:testsecret")
			t.Setenv("ETSY_CLIENT_ID", tc.envID)
			t.Setenv("ETSY_SCOPES", tc.envScopes)
			cmd := newAuthLoginCmd(&rootFlags{dryRun: true, configPath: filepath.Join(home, "config.toml")})
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetIn(strings.NewReader(""))
			cmd.SetArgs(tc.args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			output := out.String()
			if strings.Contains(output, "testsecret") || strings.Contains(output, "client_secret") {
				t.Fatal("output contains client secret")
			}
			if !strings.HasPrefix(output, "would launch: ") {
				t.Fatalf("unexpected output: %s", output)
			}
			u, err := url.Parse(strings.TrimSpace(strings.TrimPrefix(output, "would launch: ")))
			if err != nil {
				t.Fatal(err)
			}
			q := u.Query()
			for key, want := range map[string]string{"client_id": tc.wantID, "scope": tc.wantScopes, "code_challenge_method": "S256", "response_type": "code", "redirect_uri": "http://127.0.0.1:8085/callback"} {
				if got := q.Get(key); got != want {
					t.Errorf("%s = %q, want %q", key, got, want)
				}
			}
			if len(q.Get("state")) < 16 {
				t.Error("state too short")
			}
			if q.Get("code_challenge") == "" {
				t.Error("missing PKCE challenge")
			}
			if tc.envScopes == "" && len(tc.args) == 0 {
				for _, scope := range strings.Fields(q.Get("scope")) {
					if strings.HasSuffix(scope, "_w") || strings.HasSuffix(scope, "_d") || strings.HasPrefix(scope, "address_") {
						t.Errorf("write scope in defaults: %s", scope)
					}
				}
			}
		})
	}
}

func etsyTwoStepOptions(t *testing.T) etsyLoginOptions {
	t.Helper()
	restore, err := cliutil.SetHomeOverride("")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restore)
	home := testenv.Isolate(t, cliutil.ConfigDir, cliutil.DataDir, cliutil.StateDir, cliutil.CacheDir)
	for _, key := range []string{"PRINTING_PRESS_VERIFY", "DOGFOOD", "PRINTING_PRESS_DOGFOOD", "ETSY_CLIENT_ID", "ETSY_CLIENT_SECRET", "ETSY_API_KEY", "ETSY_SHARED_SECRET", "ETSY_OAUTH2", "ETSY_REDIRECT_URI", "ETSY_SCOPES", "ETSY_TOKEN_URL", "ETSY_AUTHORIZATION_URL", "ETSY_CA_CERT"} {
		t.Setenv(key, "")
	}
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "localapp"))
	t.Setenv("APPDATA", filepath.Join(home, "appdata"))
	cfg, err := config.Load(filepath.Join(home, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	return etsyLoginOptions{PendingPath: filepath.Join(home, "pending.json"), RedirectURI: "https://example.invalid/etsy/callback", ClientID: "testkey", Scopes: etsyDefaultScopes(), Config: cfg, Now: time.Now}
}

func etsyReadPending(t *testing.T, path string) etsyPendingLogin {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var p etsyPendingLogin
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestEtsyTwoStepStart(t *testing.T) {
	o := etsyTwoStepOptions(t)
	result, err := etsyStartLogin(o)
	if err != nil {
		t.Fatal(err)
	}
	p := etsyReadPending(t, o.PendingPath)
	u, err := url.Parse(result.AuthorizeURL)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(p.CodeVerifier))
	for key, want := range map[string]string{"client_id": "testkey", "redirect_uri": o.RedirectURI, "state": p.State, "scope": strings.Join(etsyDefaultScopes(), " "), "code_challenge_method": "S256", "code_challenge": base64.RawURLEncoding.EncodeToString(sum[:])} {
		if u.Query().Get(key) != want {
			t.Errorf("incorrect %s", key)
		}
	}
	info, err := os.Stat(o.PendingPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 || p.ExpiresAt.Sub(p.CreatedAt) != 10*time.Minute || len(p.State) < 32 {
		t.Fatal("invalid pending security properties")
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), p.CodeVerifier) {
		t.Fatal("verifier exposed")
	}
	for _, raw := range []string{"http://example.com/cb", "https://example.invalid/cb#fragment", "https:///cb"} {
		o.RedirectURI = raw
		if _, err := etsyStartLogin(o); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
}

func TestEtsyTwoStepCompletion(t *testing.T) {
	for _, scenario := range []string{"success", "mismatch", "expired", "denied", "failure", "missing", "missing code", "redirect mismatch"} {
		t.Run(scenario, func(t *testing.T) {
			o := etsyTwoStepOptions(t)
			if _, err := etsyStartLogin(o); err != nil {
				t.Fatal(err)
			}
			p := etsyReadPending(t, o.PendingPath)
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if err := r.ParseForm(); err != nil {
					t.Error(err)
				}
				if r.Method != "POST" {
					t.Error("expected POST")
				}
				for key, want := range map[string]string{"grant_type": "authorization_code", "code": "abc123", "redirect_uri": p.RedirectURI, "client_id": p.ClientID, "code_verifier": p.CodeVerifier} {
					if r.PostForm.Get(key) != want {
						t.Errorf("incorrect token form %s", key)
					}
				}
				if r.PostForm.Has("client_secret") {
					t.Error("secret sent")
				}
				if scenario == "failure" {
					w.WriteHeader(400)
					fmt.Fprintf(w, `{"error":%q}`, "abc123 "+p.CodeVerifier)
					return
				}
				fmt.Fprint(w, `{"access_token":"at-1","refresh_token":"rt-1","expires_in":3600,"token_type":"Bearer"}`)
			}))
			defer server.Close()
			o.TokenURL, o.HTTPClient = server.URL, server.Client()
			callback := o.RedirectURI + "?code=abc123&state=" + p.State
			switch scenario {
			case "mismatch":
				callback = o.RedirectURI + "?code=abc123&state=wrong"
			case "expired":
				o.Now = func() time.Time { return p.ExpiresAt.Add(time.Second) }
			case "denied":
				callback = o.RedirectURI + "?error=access_denied&error_description=declined"
			case "missing":
				if err := os.Remove(o.PendingPath); err != nil {
					t.Fatal(err)
				}
			case "missing code":
				callback = o.RedirectURI + "?state=" + p.State
			case "redirect mismatch":
				o.RedirectURI = "https://other.invalid/cb"
			}
			result, err := etsyCompleteLogin(context.Background(), o, callback)
			if scenario == "success" {
				if err != nil {
					t.Fatal(err)
				}
				if !result.Authenticated || !result.ExpiresAt.After(time.Now()) {
					t.Fatal("invalid success result")
				}
				cfg, err := config.Load(o.Config.Path)
				if err != nil {
					t.Fatal(err)
				}
				if cfg.AccessToken != "at-1" || cfg.RefreshToken != "rt-1" || !cfg.TokenExpiry.After(time.Now()) || cfg.ClientSecret != "" {
					t.Fatal("tokens not saved")
				}
			} else {
				want := map[string]string{"mismatch": "state mismatch", "expired": "expired", "denied": "access_denied", "failure": "authorization codes are single use", "missing": "no pending login", "missing code": "missing code", "redirect mismatch": "differs"}[scenario]
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("error = %v, want %s", err, want)
				}
				if strings.Contains(err.Error(), p.CodeVerifier) || strings.Contains(err.Error(), "abc123") {
					t.Fatal("secret in error")
				}
			}
			wantCalls := int32(0)
			if scenario == "success" || scenario == "failure" {
				wantCalls = 1
			}
			if calls.Load() != wantCalls {
				t.Fatal("unexpected token exchange")
			}
			_, statErr := os.Stat(o.PendingPath)
			keep := scenario == "mismatch" || scenario == "missing code" || scenario == "redirect mismatch"
			if keep && statErr != nil {
				t.Fatal("pending lost")
			}
			if !keep && !os.IsNotExist(statErr) {
				t.Fatal("pending not removed")
			}
		})
	}
}

func TestEtsyTwoStepCommand(t *testing.T) {
	for _, scenario := range []string{"json", "json-all", "human", "both", "dry", "waived"} {
		t.Run(scenario, func(t *testing.T) {
			o := etsyTwoStepOptions(t)
			flags := &rootFlags{configPath: o.Config.Path, asJSON: strings.HasPrefix(scenario, "json"), dryRun: scenario == "dry"}
			if scenario == "waived" {
				t.Setenv("PRINTING_PRESS_VERIFY", "1")
			}
			cmd := newAuthLoginCmd(flags)
			var out, diagnostics bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&diagnostics)
			args := []string{"--redirect-uri", o.RedirectURI}
			if scenario == "json-all" {
				args = append(args, "--all-scopes")
			}
			if scenario != "waived" {
				args = append(args, "--client-id", "testkey")
			}
			if scenario == "both" {
				args = append(args, "--callback-url", "https://example.invalid/?code=abc")
			}
			cmd.SetArgs(args)
			err := cmd.Execute()
			if scenario == "both" {
				if err == nil || !strings.Contains(err.Error(), "cannot be used together") {
					t.Fatalf("error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			dir, err := cliutil.StateDir()
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "etsy-oauth-pending.json")
			if scenario == "dry" || scenario == "waived" {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatal("dry run persisted pending")
				}
				return
			}
			p := etsyReadPending(t, path)
			if strings.Contains(out.String(), p.CodeVerifier) {
				t.Fatal("verifier printed")
			}
			wantScopes := []string{"listings_r", "shops_r", "transactions_r"}
			if scenario == "json-all" {
				wantScopes = append(wantScopes, "email_r", "listings_w", "listings_d", "shops_w", "transactions_w", "address_r", "address_w", "profile_r", "profile_w")
			}
			if !reflect.DeepEqual(p.Scopes, wantScopes) {
				t.Fatalf("pending scopes = %v, want %v", p.Scopes, wantScopes)
			}
			if strings.HasPrefix(scenario, "json") {
				var result etsyLoginStart
				if err := json.Unmarshal(out.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				u, err := url.Parse(result.AuthorizeURL)
				if err != nil {
					t.Fatal(err)
				}
				if got := u.Query().Get("scope"); got != strings.Join(wantScopes, " ") {
					t.Fatalf("authorize scopes = %q, want %v", got, wantScopes)
				}
				var obj map[string]any
				if err := json.Unmarshal(out.Bytes(), &obj); err != nil {
					t.Fatal(err)
				}
				for _, key := range []string{"authorize_url", "expires_at", "next"} {
					if obj[key] == nil {
						t.Errorf("missing %s", key)
					}
				}
				if _, ok := obj["scopes"].([]any); !ok {
					t.Fatal("scopes not array")
				}
				if diagnostics.Len() != 0 {
					t.Fatal("JSON prose")
				}
			} else if !strings.HasPrefix(out.String(), "https://") || !strings.Contains(diagnostics.String(), "--callback-url") {
				t.Fatal("missing human instructions")
			}
		})
	}
}
