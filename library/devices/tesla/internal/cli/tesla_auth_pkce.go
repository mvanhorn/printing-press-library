// tesla auth login PKCE void-callback flow.
//
// Opens the user's default browser at Tesla's real OAuth authorization URL with
// redirect_uri=https://auth.tesla.com/void/callback, asks the user to paste
// the redirected URL back, parses ?code= and ?state= from it, validates state,
// and exchanges the code for refresh+access tokens via PKCE.
//
// No webview, no callback HTTP server, no custom URL scheme handler. Tesla's
// auth.tesla.com owns the login UX (password, MFA, captcha).
//
// Hand-coded; lives outside the generator's emit set so it survives regens.
package cli

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/config"
)

const (
	teslaAuthURL      = "https://auth.tesla.com/oauth2/v3/authorize"
	teslaVoidCallback = "https://auth.tesla.com/void/callback"
)

// pkceState carries the per-flow secrets generated client-side: verifier
// (private), challenge (public, sent in the auth URL), state (CSRF).
type pkceState struct {
	Verifier  string
	Challenge string
	State     string
}

func newPKCEState() (*pkceState, error) {
	vb := make([]byte, 32)
	if _, err := rand.Read(vb); err != nil {
		return nil, fmt.Errorf("pkce verifier: %w", err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(vb)

	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])

	sb := make([]byte, 16)
	if _, err := rand.Read(sb); err != nil {
		return nil, fmt.Errorf("pkce state: %w", err)
	}
	state := hex.EncodeToString(sb)

	return &pkceState{Verifier: verifier, Challenge: challenge, State: state}, nil
}

func buildTeslaAuthURL(p *pkceState) string {
	q := url.Values{}
	q.Set("client_id", teslaClientID)
	q.Set("redirect_uri", teslaVoidCallback)
	q.Set("response_type", "code")
	q.Set("scope", teslaTokenScopes)
	q.Set("code_challenge", p.Challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("state", p.State)
	return teslaAuthURL + "?" + q.Encode()
}

// parseCallbackURL extracts code+state from the void-callback URL the user
// pastes. Tolerates surrounding whitespace, quotes, and the case where the
// user pastes the auth URL itself (before completing login).
func parseCallbackURL(raw string) (code, state string, err error) {
	raw = strings.TrimSpace(raw)
	raw = strings.Trim(raw, "\"' ")
	if raw == "" {
		return "", "", fmt.Errorf("empty URL pasted")
	}
	u, perr := url.Parse(raw)
	if perr != nil {
		return "", "", fmt.Errorf("not a URL: %w", perr)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", "", fmt.Errorf("URL missing scheme/host (paste the full https://... URL from your browser's address bar)")
	}
	qq := u.Query()
	if e := qq.Get("error"); e != "" {
		if desc := qq.Get("error_description"); desc != "" {
			return "", "", fmt.Errorf("login error from Tesla: %s: %s", e, desc)
		}
		if e == "login_cancelled" {
			return "", "", fmt.Errorf("login cancelled in browser")
		}
		return "", "", fmt.Errorf("login error from Tesla: %s", e)
	}
	code = qq.Get("code")
	state = qq.Get("state")
	if code == "" {
		return "", "", fmt.Errorf("URL is missing ?code= (you may have pasted the URL before completing login; finish login in the browser, then copy the URL of the 404 page you land on)")
	}
	return code, state, nil
}

// exchangeAuthCode swaps an authorization code for tokens via PKCE.
func exchangeAuthCode(code, verifier string) (access, refresh string, expiresIn int, err error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", teslaClientID)
	form.Set("code", code)
	form.Set("code_verifier", verifier)
	form.Set("redirect_uri", teslaVoidCallback)
	req, rerr := http.NewRequest("POST", teslaTokenURL, strings.NewReader(form.Encode()))
	if rerr != nil {
		return "", "", 0, rerr
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "tesla-pp-cli/1.0")
	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, herr := httpClient.Do(req)
	if herr != nil {
		return "", "", 0, fmt.Errorf("token exchange: %w", herr)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", "", 0, fmt.Errorf("token exchange http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if jerr := json.Unmarshal(body, &out); jerr != nil {
		return "", "", 0, fmt.Errorf("parse token response: %w", jerr)
	}
	if out.AccessToken == "" {
		return "", "", 0, fmt.Errorf("token response missing access_token")
	}
	return out.AccessToken, out.RefreshToken, out.ExpiresIn, nil
}

// openBrowser launches the OS default browser at the given URL. Best-effort:
// a non-nil error means the agent could not invoke the OS opener and the
// caller should print the URL for manual copy.
func openBrowser(rawURL string) error {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		c = exec.Command("open", rawURL)
	case "linux":
		c = exec.Command("xdg-open", rawURL)
	case "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	return c.Start()
}

// runTeslaPKCEFlow executes the void-callback PKCE flow end-to-end. Stdin is
// used to read the pasted callback URL; stderr is used for human prompts;
// stdout receives the final JSON result envelope on success. Honors verify
// and dry-run short-circuits before touching the browser or stdin.
//
// U3 wires this in as the default branch of `tesla auth login`. The function
// lives here so the helpers + flow logic stay co-located.
func runTeslaPKCEFlow(cmd *cobra.Command, flags *rootFlags) error {
	if cliutil.IsVerifyEnv() {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"verify_noop": true, "status": "logged_in", "method": "pkce_void_callback"}, flags)
	}
	if dryRunOK(flags) {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "method": "pkce_void_callback"}, flags)
	}
	if flags != nil && flags.noInput {
		return fmt.Errorf("auth login PKCE flow requires interactive stdin; pass --paste or --refresh-token for non-interactive use")
	}

	cfg, err := config.Load(flagsConfigPath(flags))
	if err != nil {
		return configErr(err)
	}
	p, err := newPKCEState()
	if err != nil {
		return err
	}
	authURL := buildTeslaAuthURL(p)

	w := cmd.OutOrStderr()
	fmt.Fprintln(w, "Opening Tesla login in your browser...")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  If it doesn't open automatically, copy this URL:")
	fmt.Fprintln(w, "  "+authURL)
	fmt.Fprintln(w, "")
	if oerr := openBrowser(authURL); oerr != nil {
		fmt.Fprintf(w, "  (couldn't auto-open browser: %v -- paste the URL above manually)\n", oerr)
	}
	fmt.Fprintln(w, "After logging in, Tesla redirects you to a 404 page on auth.tesla.com.")
	fmt.Fprintln(w, "That's expected. Copy the FULL URL from your browser's address bar and paste it here:")
	fmt.Fprintln(w, "")

	pasted, perr := readSingleLine(cmd.InOrStdin())
	if perr != nil {
		return fmt.Errorf("read pasted URL: %w", perr)
	}
	code, gotState, perr := parseCallbackURL(pasted)
	if perr != nil {
		return perr
	}
	if gotState != p.State {
		return fmt.Errorf("CSRF state mismatch (expected %q, got %q); auth flow may have been intercepted -- start over", p.State, gotState)
	}

	access, refresh, expiresIn, eerr := exchangeAuthCode(code, p.Verifier)
	if eerr != nil {
		return eerr
	}
	expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second).UTC()
	if serr := saveTeslaTokens(cfg, refresh, access, expiresAt); serr != nil {
		return serr
	}
	return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
		"status":       "logged_in",
		"method":       "pkce_void_callback",
		"expires_at":   expiresAt.Format(time.RFC3339),
		"expires_in":   expiresIn,
		"storage_path": cfg.Path,
		"hint":         "Bearer auto-refreshes when it expires. Run 'tesla auth status' to verify.",
	}, flags)
}

// readSingleLine reads from r until the first newline or EOF; trims surrounding
// whitespace. Single-line read matches what users actually do (a single paste +
// Enter); avoids waiting for Ctrl-D the way io.ReadAll would.
func readSingleLine(r io.Reader) (string, error) {
	buf := make([]byte, 0, 1024)
	one := make([]byte, 1)
	for {
		n, err := r.Read(one)
		if n > 0 {
			if one[0] == '\n' {
				break
			}
			buf = append(buf, one[0])
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
		if len(buf) > 8192 {
			return "", fmt.Errorf("pasted URL too long (>8KB); try again")
		}
	}
	return strings.TrimSpace(string(buf)), nil
}

// flagsConfigPath returns the configPath rootFlags carries. Wrapped to keep the
// nil-rootFlags edge handled in one place; production paths always pass real flags.
func flagsConfigPath(f *rootFlags) string {
	if f == nil {
		return ""
	}
	return f.configPath
}
