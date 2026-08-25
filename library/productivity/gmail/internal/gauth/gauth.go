// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Package gauth implements multi-account installed-app OAuth for Gmail with
// named per-account profiles. Ported from the proven google-calendar-pp-cli
// gauth layer with one deliberate scope-model change:
//
// EVERY profile requests exactly ONE scope: gmail.modify. The profiles.yaml
// `role:` field is kept for forward compatibility but every role maps to that
// single scope, because in this binary the scope ladder buys nothing:
//   - modify ⊃ read, so a separate readonly grant adds no live capability
//     while doubling the consent surface;
//   - send, insert, draft, and settings endpoints do not exist in this
//     binary (pruned from the spec before generation), so send authority
//     cannot be exercised no matter what the token allows;
//   - permanent delete (single or batched) requires the full
//     https://mail.google.com/ scope, which is never requested — trash is
//     the destruction ceiling by scope physics.
//
// Config dir layout (default ~/.config/gmail-pp-cli/gauth, override with
// GMAIL_CONFIG_DIR or --auth-dir):
//
//	client.json     user-supplied installed-app OAuth client (never committed)
//	profiles.yaml   name -> {email, role}; role kept for forward-compat
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

	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/cliutil"
)

// ScopeModify is the single OAuth scope every profile holds. See the package
// doc for why no role ever maps to anything narrower or broader.
const ScopeModify = "https://www.googleapis.com/auth/gmail.modify"

// Profile is one named Google account. Role is recorded and surfaced but does
// not change the requested scope (see ScopesFor).
type Profile struct {
	Name  string `yaml:"name"`
	Email string `yaml:"email"`
	Role  string `yaml:"role"`
}

type profilesFile struct {
	Accounts []Profile `yaml:"accounts"`
}

// ConfigDir resolves the auth/config home.
func ConfigDir(override string) string {
	if override != "" {
		return override
	}
	if v := os.Getenv("GMAIL_CONFIG_DIR"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "gmail-pp-cli", "gauth")
}

// ScopesFor maps a role to the OAuth scopes its token holds. Every role maps
// to the single gmail.modify scope: modify ⊃ read, send/settings endpoints
// don't exist in this binary, and permanent delete is scope-impossible
// (it needs https://mail.google.com/, never requested). The role argument is
// accepted — not validated against a closed set — so profiles.yaml written
// for a future role-differentiated version keeps working today.
func ScopesFor(role string) ([]string, error) {
	_ = role
	return []string{ScopeModify}, nil
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

// DefaultAccount returns the only configured profile when exactly one exists.
// With several profiles there is no safe implicit choice for a mailbox tool,
// so the caller must pass --account; the error lists what is available.
func DefaultAccount(dir string) (Profile, error) {
	ps, err := LoadProfiles(dir)
	if err != nil {
		return Profile{}, err
	}
	if len(ps) == 1 {
		return ps[0], nil
	}
	names := make([]string, 0, len(ps))
	for _, p := range ps {
		names = append(names, p.Name)
	}
	return Profile{}, fmt.Errorf("multiple profiles configured — pass --account <name> (have: %s)", strings.Join(names, ", "))
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
		return nil, fmt.Errorf("profile %q: %w (re-run: accounts auth --account %s)", p.name, err, p.name)
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
		return nil, fmt.Errorf("no token for profile %q — run: accounts auth --account %s", prof.Name, prof.Name)
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
// the token. Before saving, it verifies the consented Google account matches
// the profile's declared email — picking the wrong account in the browser
// must be a caught error, not a silent misconfiguration. On mismatch the
// token is discarded without ever touching disk.
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
	out(fmt.Sprintf("Authorizing profile %q (%s, scope gmail.modify).", prof.Name, prof.Email))
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

	// Verify the consented account is the declared one BEFORE the token ever
	// reaches disk. A wrong-account token is discarded here, not persisted
	// then removed.
	got, err := consentedEmail(ctx, tok.AccessToken)
	if err != nil {
		// Verification itself failed (network, transient API error). The
		// grant is real; keep it but tell the operator to confirm.
		if serr := saveToken(dir, prof.Name, tok); serr != nil {
			return serr
		}
		out(fmt.Sprintf("warning: could not verify consented account (%v) — run doctor to confirm", err))
		return nil
	}
	if !strings.EqualFold(got, prof.Email) {
		return fmt.Errorf("consented account %s does NOT match profile %q (%s) — token discarded; re-run and pick the right account", got, prof.Name, prof.Email)
	}
	if err := saveToken(dir, prof.Name, tok); err != nil {
		return err
	}
	out(fmt.Sprintf("Verified: consented account matches (%s). Token saved.", got))
	return nil
}

// consentedEmail fetches the Gmail profile's emailAddress using the fresh
// token; works under gmail.modify (and any broader) scope. This is a one-shot
// post-consent verification call; it still paces through a
// cliutil.AdaptiveLimiter and applies typed 429 handling: one retry honoring
// Retry-After (capped), then a typed rate-limit error.
func consentedEmail(ctx context.Context, accessToken string) (string, error) {
	lim := cliutil.NewAdaptiveLimiterAuto(2.0)
	for attempt := 0; ; attempt++ {
		lim.Wait()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet,
			"https://gmail.googleapis.com/gmail/v1/users/me/profile", nil)
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
			return "", errors.New("users/me/profile: HTTP 429 rate limited (retried once) — wait a moment and re-run accounts auth")
		}
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("users/me/profile: HTTP %d", resp.StatusCode)
		}
		lim.OnSuccess()
		var body struct {
			EmailAddress string `json:"emailAddress"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			return "", err
		}
		return body.EmailAddress, nil
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
