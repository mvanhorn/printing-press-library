// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Package gauth implements multi-account installed-app OAuth for Google Calendar
// with per-profile SCOPE ROLES: a profile declared readonly requests
// calendar.readonly ONLY, so write authority never exists on that token —
// least-privilege enforced by the OAuth grant itself, not by CLI politeness.
//
// Config dir layout (default ~/.config/google-calendar-pp-cli/gauth, override with
// GCAL_CONFIG_DIR or --config-dir):
//
//	client.json     user-supplied installed-app OAuth client (never committed)
//	profiles.yaml   name -> {email, role}; role: readonly | writable
//	tokens/<name>.json  oauth2 token per profile, 0600
package gauth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"gopkg.in/yaml.v3"

	"github.com/mvanhorn/printing-press-library/library/productivity/google-calendar/internal/cliutil"
)

const (
	RoleReadonly = "readonly"
	RoleWritable = "writable"

	scopeReadonly = "https://www.googleapis.com/auth/calendar.readonly"
	scopeFull     = "https://www.googleapis.com/auth/calendar"
)

// Profile is one named Google account with a declared role.
type Profile struct {
	Name    string `yaml:"name"`
	Email   string `yaml:"email"`
	Role    string `yaml:"role"`
	Default bool   `yaml:"default_write,omitempty"` // default target for creates
}

type profilesFile struct {
	Accounts []Profile `yaml:"accounts"`
}

// ConfigDir resolves the auth/config home.
func ConfigDir(override string) string {
	if override != "" {
		return override
	}
	if v := os.Getenv("GCAL_CONFIG_DIR"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "google-calendar-pp-cli", "gauth")
}

// ScopesFor maps a role to the OAuth scopes its token is allowed to hold.
func ScopesFor(role string) ([]string, error) {
	switch role {
	case RoleReadonly:
		return []string{scopeReadonly}, nil
	case RoleWritable:
		return []string{scopeFull}, nil
	default:
		return nil, fmt.Errorf("unknown role %q (want %s|%s)", role, RoleReadonly, RoleWritable)
	}
}

// LoadProfiles reads profiles.yaml. Missing file is an explicit, actionable error.
func LoadProfiles(dir string) ([]Profile, error) {
	p := filepath.Join(dir, "profiles.yaml")
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("profiles not found at %s — create it with accounts: [{name, email, role}] entries: %w", p, err)
	}
	var f profilesFile
	if err := yaml.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", p, err)
	}
	if len(f.Accounts) == 0 {
		return nil, fmt.Errorf("%s contains no accounts", p)
	}
	seen := map[string]bool{}
	for i := range f.Accounts {
		a := &f.Accounts[i]
		a.Role = strings.ToLower(strings.TrimSpace(a.Role))
		if a.Name == "" || a.Email == "" {
			return nil, fmt.Errorf("%s: account %d missing name or email", p, i)
		}
		if seen[a.Name] {
			return nil, fmt.Errorf("%s: duplicate profile name %q", p, a.Name)
		}
		seen[a.Name] = true
		if _, err := ScopesFor(a.Role); err != nil {
			return nil, fmt.Errorf("%s: account %q: %w", p, a.Name, err)
		}
	}
	return f.Accounts, nil
}

// Get returns the named profile.
func Get(dir, name string) (Profile, error) {
	ps, err := LoadProfiles(dir)
	if err != nil {
		return Profile{}, err
	}
	for _, p := range ps {
		if p.Name == name {
			return p, nil
		}
	}
	names := make([]string, 0, len(ps))
	for _, p := range ps {
		names = append(names, p.Name)
	}
	return Profile{}, fmt.Errorf("no profile %q (have: %s)", name, strings.Join(names, ", "))
}

// DefaultWrite returns the profile marked default_write.
func DefaultWrite(dir string) (Profile, error) {
	ps, err := LoadProfiles(dir)
	if err != nil {
		return Profile{}, err
	}
	for _, p := range ps {
		if p.Default {
			if p.Role != RoleWritable {
				return Profile{}, fmt.Errorf("default_write profile %q is not writable", p.Name)
			}
			return p, nil
		}
	}
	return Profile{}, errors.New("no profile marked default_write in profiles.yaml")
}

func oauthConfig(dir string, scopes []string) (*oauth2.Config, error) {
	b, err := os.ReadFile(filepath.Join(dir, "client.json"))
	if err != nil {
		return nil, fmt.Errorf("client.json not found in %s (download the Desktop-app OAuth client from Google Cloud console): %w", dir, err)
	}
	cfg, err := google.ConfigFromJSON(b, scopes...)
	if err != nil {
		return nil, fmt.Errorf("parsing client.json: %w", err)
	}
	return cfg, nil
}

func tokenPath(dir, name string) string {
	return filepath.Join(dir, "tokens", name+".json")
}

func loadToken(dir, name string) (*oauth2.Token, error) {
	b, err := os.ReadFile(tokenPath(dir, name))
	if err != nil {
		return nil, err
	}
	var t oauth2.Token
	if err := json.Unmarshal(b, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

func saveToken(dir, name string, t *oauth2.Token) error {
	if err := os.MkdirAll(filepath.Join(dir, "tokens"), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	tmp := tokenPath(dir, name) + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, tokenPath(dir, name))
}

// persistingSource wraps a TokenSource and writes refreshed tokens back to disk.
type persistingSource struct {
	dir, name string
	src       oauth2.TokenSource
	last      string
}

func (p *persistingSource) Token() (*oauth2.Token, error) {
	t, err := p.src.Token()
	if err != nil {
		return nil, fmt.Errorf("profile %q: %w (re-run: auth --account %s)", p.name, err, p.name)
	}
	if t.AccessToken != p.last {
		p.last = t.AccessToken
		if err := saveToken(p.dir, p.name, t); err != nil {
			return nil, fmt.Errorf("persisting refreshed token for %q: %w", p.name, err)
		}
	}
	return t, nil
}

// Source returns an auto-refreshing, auto-persisting token source for a profile.
func Source(ctx context.Context, dir string, prof Profile) (oauth2.TokenSource, error) {
	scopes, err := ScopesFor(prof.Role)
	if err != nil {
		return nil, err
	}
	cfg, err := oauthConfig(dir, scopes)
	if err != nil {
		return nil, err
	}
	tok, err := loadToken(dir, prof.Name)
	if err != nil {
		return nil, fmt.Errorf("no token for profile %q — run: auth --account %s", prof.Name, prof.Name)
	}
	return &persistingSource{dir: dir, name: prof.Name, src: cfg.TokenSource(ctx, tok)}, nil
}

// AccessToken resolves a live bearer token for the named profile.
func AccessToken(ctx context.Context, dir, name string) (string, error) {
	prof, err := Get(dir, name)
	if err != nil {
		return "", err
	}
	src, err := Source(ctx, dir, prof)
	if err != nil {
		return "", err
	}
	t, err := src.Token()
	if err != nil {
		return "", err
	}
	return t.AccessToken, nil
}

// Authenticate runs the installed-app loopback flow for one profile and saves
// the token. It then verifies the consented Google account matches the
// profile's declared email — picking the wrong account in the browser must be
// a caught error, not a silent misconfiguration.
func Authenticate(ctx context.Context, dir string, prof Profile, out func(string)) error {
	scopes, err := ScopesFor(prof.Role)
	if err != nil {
		return err
	}
	cfg, err := oauthConfig(dir, scopes)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("loopback listener: %w", err)
	}
	defer ln.Close()
	cfg.RedirectURL = fmt.Sprintf("http://%s", ln.Addr().String())

	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return err
	}
	state := hex.EncodeToString(stateBytes)

	authURL := cfg.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent select_account"),
		oauth2.SetAuthURLParam("login_hint", prof.Email),
	)
	out(fmt.Sprintf("Authorizing profile %q (%s, role %s).", prof.Name, prof.Email, prof.Role))
	out("Opening browser — pick the MATCHING Google account. If no browser opens, visit:")
	out("  " + authURL)
	openBrowser(authURL)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("state") != state {
			errCh <- errors.New("oauth state mismatch")
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}
		if e := q.Get("error"); e != "" {
			errCh <- fmt.Errorf("authorization denied: %s", e)
			fmt.Fprintln(w, "Authorization denied. You can close this tab.")
			return
		}
		codeCh <- q.Get("code")
		fmt.Fprintln(w, "Authorized. You can close this tab and return to the terminal.")
	})}
	go srv.Serve(ln) //nolint:errcheck
	defer srv.Close()

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		return err
	case <-time.After(5 * time.Minute):
		return errors.New("timed out waiting for browser authorization (5m)")
	case <-ctx.Done():
		return ctx.Err()
	}

	tok, err := cfg.Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf("code exchange: %w", err)
	}
	if err := saveToken(dir, prof.Name, tok); err != nil {
		return err
	}

	// Verify the consented account is the declared one.
	got, err := consentedEmail(ctx, tok.AccessToken)
	if err != nil {
		out(fmt.Sprintf("warning: could not verify consented account (%v) — run doctor to confirm", err))
		return nil
	}
	if !strings.EqualFold(got, prof.Email) {
		_ = os.Remove(tokenPath(dir, prof.Name))
		return fmt.Errorf("consented account %s does NOT match profile %q (%s) — token discarded; re-run and pick the right account", got, prof.Name, prof.Email)
	}
	out(fmt.Sprintf("Verified: consented account matches (%s). Token saved.", got))
	return nil
}

// consentedEmail fetches the primary calendar id (== account email) using the
// fresh token; works under calendar.readonly and calendar scopes alike.
// This is a one-shot post-consent verification call; it still paces through a
// cliutil.AdaptiveLimiter and applies typed 429 handling: one retry honoring
// Retry-After (capped), then a typed rate-limit error.
func consentedEmail(ctx context.Context, accessToken string) (string, error) {
	lim := cliutil.NewAdaptiveLimiterAuto(2.0)
	for attempt := 0; ; attempt++ {
		lim.Wait()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet,
			"https://www.googleapis.com/calendar/v3/users/me/calendarList/primary", nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}
		if resp.StatusCode == http.StatusTooManyRequests && attempt == 0 {
			lim.OnRateLimit()
			wait := retryAfter(resp.Header.Get("Retry-After"), 2*time.Second, 5*time.Second)
			resp.Body.Close()
			select {
			case <-time.After(wait):
				continue
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			return "", errors.New("calendarList/primary: HTTP 429 rate limited (retried once) — wait a moment and re-run login")
		}
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("calendarList/primary: HTTP %d", resp.StatusCode)
		}
		lim.OnSuccess()
		var body struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			return "", err
		}
		return body.ID, nil
	}
}

// retryAfter parses a Retry-After header given in whole seconds, falling back
// to def when absent or unparseable and capping the wait at max.
func retryAfter(v string, def, max time.Duration) time.Duration {
	if v == "" {
		return def
	}
	secs, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || secs < 0 {
		return def
	}
	d := time.Duration(secs) * time.Second
	if d > max {
		return max
	}
	return d
}

// TokenStatus reports, per profile, whether a token exists and its expiry.
type TokenStatus struct {
	Profile Profile   `json:"profile"`
	HasTok  bool      `json:"has_token"`
	Expiry  time.Time `json:"expiry,omitempty"`
}

// Statuses lists token state for every profile.
func Statuses(dir string) ([]TokenStatus, error) {
	ps, err := LoadProfiles(dir)
	if err != nil {
		return nil, err
	}
	out := make([]TokenStatus, 0, len(ps))
	for _, p := range ps {
		st := TokenStatus{Profile: p}
		if t, err := loadToken(dir, p.Name); err == nil {
			st.HasTok = true
			st.Expiry = t.Expiry
		}
		out = append(out, st)
	}
	return out, nil
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return
	}
	_ = cmd.Start()
}
