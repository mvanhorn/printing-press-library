// PATCH(oauth-login): implements the loopback OAuth 2.0 flow for `gsc auth
// login`. Listens on 127.0.0.1:0 (random port), opens the browser to Google's
// consent screen with PKCE S256, exchanges the resulting code for access +
// refresh tokens, and persists them to config. See
// docs/plans/2026-05-11-feat-oauth-login-flow-plan.md for the design.

package cli

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-search-console/internal/config"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"sync"
)

// Google's installed-app auth endpoint. The /v2 path supports incremental
// auth and PKCE more cleanly than the legacy /o/oauth2/auth that
// google.Endpoint.AuthURL still points at, so we override it explicitly.
const googleAuthURL = "https://accounts.google.com/o/oauth2/v2/auth"

// Google Search Console OAuth scope strings, verbatim. Default is readonly;
// users opt into the write scope explicitly via --scope write because the
// readonly token can't accidentally submit a sitemap or remove a verified
// site if leaked.
const (
	scopeReadonly = "https://www.googleapis.com/auth/webmasters.readonly"
	scopeWrite    = "https://www.googleapis.com/auth/webmasters"
)

// loginTimeout caps how long the loopback listener waits for the user to
// finish in their browser. Five minutes mirrors gcloud's behavior and is long
// enough to handle real-world fumbling without leaving a process hanging
// forever in CI or detached terminals.
const loginTimeout = 5 * time.Minute

func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var (
		scope     string
		noBrowser bool
		port      int
	)
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in via browser (loopback OAuth, persists refresh token for auto-renewal)",
		Long: `Log in to Google Search Console via your default browser.

The CLI spins up a temporary loopback web server on 127.0.0.1, opens the
Google consent screen, and saves the resulting access + refresh tokens to
` + "`~/.config/google-search-console-pp-cli/config.toml`" + ` (mode 0600). From
then on, every command silently refreshes the access token in the background
when it expires.

Prerequisite: run ` + "`gsc auth set-client <client_id> <client_secret>`" + ` once
to register your Google Cloud OAuth client. See README.md for the 5-minute
Google Cloud Console setup walkthrough.`,
		Example: `  # Default (readonly scope)
  google-search-console-pp-cli auth login

  # Request write scope (sitemap submit, site add/delete)
  google-search-console-pp-cli auth login --scope write

  # SSH/headless: print URL, don't try to open a browser
  google-search-console-pp-cli auth login --no-browser`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuthLogin(cmd, flags, scope, noBrowser, port)
		},
	}
	cmd.Flags().StringVar(&scope, "scope", "readonly", "Scope to request: readonly | write")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Don't try to open the browser — print the URL only")
	cmd.Flags().IntVar(&port, "port", 0, "Loopback port (0 = OS-assigned, recommended)")
	return cmd
}

func runAuthLogin(cmd *cobra.Command, flags *rootFlags, scopeArg string, noBrowser bool, port int) error {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return configErr(err)
	}

	if !cfg.HasClient() {
		return authErr(fmt.Errorf(`OAuth client not configured.

Run this once with credentials from your Google Cloud Console OAuth client
(Desktop app type):

  google-search-console-pp-cli auth set-client <client_id> <client_secret>

Walkthrough: https://developers.google.com/identity/protocols/oauth2/native-app#enable-apis`))
	}

	resolvedScope, err := resolveScope(scopeArg)
	if err != nil {
		return err
	}

	// Bind first so we know the actual port before building the auth URL.
	listener, err := startLoopback(port)
	if err != nil {
		return fmt.Errorf("starting loopback listener: %w", err)
	}
	// PATCH(oauth-login): derive redirectURL from the listener's actual bound
	// address. startLoopback() tries 127.0.0.1 first, then falls back to [::1]
	// on IPv6-only hosts; hardcoding 127.0.0.1 in the redirect URL would make
	// Google bounce the user to an unreachable interface in the IPv6 case.
	// RFC 8252 §7.3 explicitly permits http://[::1]:PORT as a loopback redirect
	// URI, and Google accepts it.
	tcpAddr := listener.Addr().(*net.TCPAddr)
	actualPort := tcpAddr.Port
	var redirectURL string
	if tcpAddr.IP.To4() == nil {
		redirectURL = fmt.Sprintf("http://[%s]:%d/callback", tcpAddr.IP.String(), actualPort)
	} else {
		redirectURL = fmt.Sprintf("http://%s:%d/callback", tcpAddr.IP.String(), actualPort)
	}

	oauthCfg := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  googleAuthURL,
			TokenURL: google.Endpoint.TokenURL,
		},
		Scopes:      []string{resolvedScope},
		RedirectURL: redirectURL,
	}

	verifier := oauth2.GenerateVerifier()
	state, err := randomState()
	if err != nil {
		_ = listener.Close()
		return fmt.Errorf("generating state: %w", err)
	}

	authURL := oauthCfg.AuthCodeURL(state,
		oauth2.AccessTypeOffline, // refresh_token in response
		oauth2.SetAuthURLParam("prompt", "consent"),
		oauth2.S256ChallengeOption(verifier),
	)

	w := cmd.OutOrStdout()
	if noBrowser {
		fmt.Fprintln(w, "Open this URL in any browser:")
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "  "+authURL)
		fmt.Fprintln(w, "")
		fmt.Fprintf(w, "Waiting for callback on %s ...\n", redirectURL)
	} else {
		if err := openBrowser(authURL); err != nil {
			// xdg-open / wslview / open all failed — degrade to print-URL.
			fmt.Fprintln(w, "Couldn't auto-open a browser. Open this URL manually:")
			fmt.Fprintln(w, "")
			fmt.Fprintln(w, "  "+authURL)
			fmt.Fprintln(w, "")
			fmt.Fprintf(w, "Waiting for callback on %s ...\n", redirectURL)
		} else {
			fmt.Fprintf(w, "Opened browser. Waiting for you to complete sign-in (listening on %s) ...\n", redirectURL)
		}
	}

	code, callbackErr := waitForCallback(cmd.Context(), listener, state)
	if callbackErr != nil {
		return authErr(callbackErr)
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()
	token, err := oauthCfg.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return authErr(fmt.Errorf("exchanging code for tokens: %w", err))
	}
	if token.RefreshToken == "" {
		// This shouldn't happen with prompt=consent + access_type=offline, but
		// surface it loudly if it does — auto-refresh won't work without one.
		fmt.Fprintln(w, yellow("Warning: Google did not return a refresh token. Auto-refresh will not work."))
		fmt.Fprintln(w, "Try logging out and back in; if the problem persists, revoke this app's access at https://myaccount.google.com/permissions and retry.")
	}

	if err := cfg.SaveTokens(cfg.ClientID, cfg.ClientSecret, token.AccessToken, token.RefreshToken, token.Expiry); err != nil {
		return configErr(fmt.Errorf("saving tokens: %w", err))
	}

	fmt.Fprintln(w, "")
	fmt.Fprintln(w, green("Logged in."))
	fmt.Fprintf(w, "  Tokens saved to: %s\n", cfg.Path)
	if !token.Expiry.IsZero() {
		fmt.Fprintf(w, "  Access token expires: %s (auto-refreshes silently)\n", token.Expiry.Format(time.RFC3339))
	}
	fmt.Fprintf(w, "  Scope: %s\n", resolvedScope)
	return nil
}

// resolveScope maps the user-friendly --scope value to the canonical Google
// scope string. The CLI accepts "readonly" / "write" so users don't have to
// memorize the URL form; the explicit URL form is also accepted for
// power users / script authors.
func resolveScope(scope string) (string, error) {
	switch strings.ToLower(scope) {
	case "readonly", "read", "":
		return scopeReadonly, nil
	case "write", "readwrite", "full":
		return scopeWrite, nil
	case scopeReadonly, scopeWrite:
		return scope, nil
	default:
		return "", fmt.Errorf("invalid --scope %q: use 'readonly' or 'write'", scope)
	}
}

// startLoopback binds a TCP listener on 127.0.0.1 (or [::1] as a fallback).
// Port 0 asks the OS for any free port; RFC 8252 §7.3 requires Google to
// accept any port on the loopback interface, so registering a fixed port in
// Cloud Console isn't needed. We bind 127.0.0.1 literally — `localhost` has
// caused bugs in other CLIs (cli/cli#42765) when the resolver returns IPv6
// first but the OAuth server doesn't tolerate it.
func startLoopback(port int) (net.Listener, error) {
	addrV4 := fmt.Sprintf("127.0.0.1:%d", port)
	if ln, err := net.Listen("tcp4", addrV4); err == nil {
		return ln, nil
	}
	addrV6 := fmt.Sprintf("[::1]:%d", port)
	if ln, err := net.Listen("tcp6", addrV6); err == nil {
		return ln, nil
	}
	return nil, fmt.Errorf("could not bind loopback on port %d (try --port to specify a free port)", port)
}

// waitForCallback handles exactly one VALID callback on the loopback listener.
// Returns the OAuth `code` or a descriptive error.
//
// Single-shot semantics are enforced by sync.Once on the result channel:
// the first VALID callback (passing state compare + carrying either `code`
// or `error`) wins, and any subsequent attempts to post are no-ops. State
// mismatches do NOT shut the listener — they return 404 and keep waiting,
// so the legitimate user's callback can still arrive even after a stray
// forged hit. Security is delivered by sync.Once + the 32-byte random
// state, not by listener teardown.
//
// State comparison is constant-time to avoid any (theoretical) timing oracle.
func waitForCallback(ctx context.Context, listener net.Listener, expectedState string) (string, error) {
	type result struct {
		code string
		err  error
	}
	resultCh := make(chan result, 1)

	// once gate ensures only the first callback fires resultCh, regardless of
	// validity. Subsequent requests get a 404 and don't affect flow state.
	// sync.Once gate ensures resultCh is sent to at most once, regardless of
	// concurrent handler invocations (net/http serves each request on its own
	// goroutine — a plain bool would race under -race).
	var fired sync.Once
	post := func(r result) {
		fired.Do(func() {
			resultCh <- r
		})
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		gotState := q.Get("state")
		if subtle.ConstantTimeCompare([]byte(gotState), []byte(expectedState)) != 1 {
			// Don't tell the caller why — could be CSRF. We do NOT post to
			// resultCh on a mismatch (the legit user's callback may still
			// arrive), but we do log nothing and return 404.
			http.NotFound(w, r)
			return
		}
		if errCode := q.Get("error"); errCode != "" {
			desc := q.Get("error_description")
			renderCallbackPage(w, false, errCode, desc)
			msg := errCode
			if desc != "" {
				msg = errCode + ": " + desc
			}
			post(result{err: fmt.Errorf("login cancelled or failed: %s", msg)})
			return
		}
		code := q.Get("code")
		if code == "" {
			renderCallbackPage(w, false, "missing_code", "no `code` parameter in callback")
			post(result{err: fmt.Errorf("callback missing 'code' parameter")})
			return
		}
		renderCallbackPage(w, true, "", "")
		post(result{code: code})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Anything other than /callback is unexpected. Don't leak state.
		http.NotFound(w, r)
	})

	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			select {
			case resultCh <- result{err: fmt.Errorf("loopback server error: %w", err)}:
			default:
			}
		}
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	// NewTimer + defer Stop so the goroutine that drives time.After doesn't
	// linger for the full loginTimeout when the callback or ctx wakes us early.
	timer := time.NewTimer(loginTimeout)
	defer timer.Stop()
	select {
	case res := <-resultCh:
		return res.code, res.err
	case <-ctx.Done():
		return "", fmt.Errorf("login cancelled: %w", ctx.Err())
	case <-timer.C:
		return "", fmt.Errorf("login timed out after %s — re-run 'auth login'", loginTimeout)
	}
}

// renderCallbackPage writes a minimal HTML response back to the user's
// browser after callback. window.close() is blocked by modern browsers
// unless the window was JS-opened (ours wasn't), so we just tell the user
// to close the tab. Success path is short and friendly; error path echoes
// the OAuth error code/description so the user can debug if they care.
func renderCallbackPage(w http.ResponseWriter, success bool, code, desc string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if success {
		fmt.Fprint(w, `<!doctype html>
<html><head><title>Logged in</title></head>
<body style="font-family:system-ui,sans-serif;max-width:600px;margin:4rem auto;padding:0 1rem">
<h2>Logged in to Google Search Console CLI</h2>
<p>You can close this tab and return to your terminal.</p>
</body></html>`)
		return
	}
	safeCode := template.HTMLEscapeString(code)
	safeDesc := template.HTMLEscapeString(desc)
	fmt.Fprintf(w, `<!doctype html>
<html><head><title>Login failed</title></head>
<body style="font-family:system-ui,sans-serif;max-width:600px;margin:4rem auto;padding:0 1rem">
<h2>Login failed</h2>
<p><strong>%s</strong></p>
<p>%s</p>
<p>Return to your terminal and re-run <code>auth login</code>.</p>
</body></html>`, safeCode, safeDesc)
}

// randomState produces a 32-byte URL-safe base64 string for the OAuth state
// parameter. State is single-use, in-memory only, never persisted.
func randomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// revokeAtGoogle POSTs the refresh token to Google's revocation endpoint.
// Pass the refresh token (not access) because revoking refresh invalidates
// all derived access tokens. Token goes in the body, NEVER the URL.
const googleRevokeURL = "https://oauth2.googleapis.com/revoke"

func revokeAtGoogle(ctx context.Context, token string) error {
	form := url.Values{"token": {token}}.Encode()
	req, err := http.NewRequestWithContext(ctx, "POST", googleRevokeURL, strings.NewReader(form))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// Cap body read so a misbehaving endpoint can't stream gigabytes.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode >= 400 {
		return fmt.Errorf("revoke endpoint returned HTTP %d", resp.StatusCode)
	}
	return nil
}
