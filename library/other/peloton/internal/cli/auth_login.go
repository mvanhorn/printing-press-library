// Copyright 2026 twidtwid. Licensed under Apache-2.0. See LICENSE.
//
// PATCH(catalog/peloton): adds `auth login` chromedp + Auth0 SPA localStorage
// harvest. The press's auth_simple.go.tmpl emits set-token/status/logout for
// bearer_token auth, but Peloton's bearer is gated behind Auth0 SPA login —
// users can't paste it without first scraping it from DevTools. This file
// adds a press-spec-friendly browser harvest so the standard `auth login`
// catalog convention works. Mirrors the dominos pattern (auth_login.go
// sibling to press-emitted auth.go). Recorded in
// .printing-press-patches.json.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/peloton/internal/config"
)

// loginURL is the canonical Peloton sign-in page. Auth0 redirects through
// auth.onepeloton.com and back to /home (or /) once the SPA is hydrated; the
// access token lands in localStorage on whichever page the SPA settles on.
const loginURL = "https://members.onepeloton.com/login"

// readTokenExpr scans localStorage for the Auth0 SPA cache key and returns
// the access_token + cached user id as a JSON envelope. Discovers the
// client_id at evaluation time rather than hardcoding so a Peloton client
// rotation doesn't break the harvest. Auth0 stores the OAuth response under:
//
//	@@auth0spajs@@::<client_id>::https://api.onepeloton.com/::openid offline_access
//
// with shape {"body": {"access_token": "...", "expires_in": 3600, ...}, ...}.
const readTokenExpr = `(function(){
  var out = {token:'', user_id:''};
  for (var i = 0; i < localStorage.length; i++) {
    var k = localStorage.key(i);
    if (k && k.indexOf('@@auth0spajs@@::') === 0 && k.indexOf('api.onepeloton.com') !== -1) {
      try {
        var v = JSON.parse(localStorage.getItem(k));
        if (v && v.body && v.body.access_token) {
          out.token = v.body.access_token;
        }
      } catch (e) {}
    }
    // Auth0 also caches a user profile under @@auth0spajs@@::...::@@user@@.
    // Peloton's API expects the bare ID (no "auth0|" prefix that JWT subs carry).
    if (k && k.indexOf('@@auth0spajs@@::') === 0 && k.indexOf('@@user@@') !== -1) {
      try {
        var u = JSON.parse(localStorage.getItem(k));
        if (u && u.decodedToken && u.decodedToken.user) {
          var sub = u.decodedToken.user.sub || '';
          out.user_id = sub.indexOf('|') !== -1 ? sub.split('|').pop() : sub;
        }
      } catch (e) {}
    }
  }
  return JSON.stringify(out);
})()`

// fillLoginExpr is best-effort prefill of the login form when env credentials
// are available. The user can also type in the window themselves; this just
// saves a step. Selectors as of 2026-05.
const fillLoginExpr = `(function(u, p){
  var ue = document.querySelector('input[name="usernameOrEmail"]');
  var pe = document.querySelector('input[name="password"]');
  if (ue) ue.value = u;
  if (pe) pe.value = p;
  if (ue) ue.dispatchEvent(new Event('input', {bubbles: true}));
  if (pe) pe.dispatchEvent(new Event('input', {bubbles: true}));
  return !!(ue && pe);
})('%s', '%s')`

const pollInterval = 1500 * time.Millisecond

type harvestEnvelope struct {
	Token  string `json:"token"`
	UserID string `json:"user_id"`
}

func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var headless bool
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Open Chrome, sign in to Peloton, harvest the Auth0 SPA bearer token",
		Long: `Spawns a chromedp-controlled Chrome window pointed at members.onepeloton.com/login.
If PELOTON_USERNAME and PELOTON_PASSWORD are set in the environment, the form is
pre-filled. You complete the sign-in (handles captchas, 2FA, anything Auth0
throws), then the CLI extracts the bearer token from localStorage and saves
it to ~/.config/peloton-pp-cli/config.toml.

The Chrome profile persists at ~/.config/peloton-pp-cli/chrome/, so subsequent
logins reuse session cookies and finish in seconds. Tokens last about an hour
Peloton-side; re-run when expired.`,
		Example: `  peloton-pp-cli auth login
  peloton-pp-cli auth login --headless
  peloton-pp-cli auth login --timeout 10m`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := harvestTokenViaChrome(cmd.Context(), cmd.ErrOrStderr(), headless, timeout)
			if err != nil {
				return authErr(fmt.Errorf("browser harvest failed: %w", err))
			}
			return saveHarvestedAuth(cmd.OutOrStdout(), flags, env)
		},
	}
	cmd.Flags().BoolVar(&headless, "headless", false, "Run Chrome headless (rare; usually you want to see the sign-in window)")
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "Maximum time to wait for sign-in to complete")
	return cmd
}

// harvestTokenViaChrome spawns Chrome, navigates to the login page, optionally
// pre-fills credentials, then polls localStorage every pollInterval until the
// Auth0 SPA cache key shows up with an access_token. Returns immediately on
// first non-empty harvest; that's the signal the redirect chain settled.
func harvestTokenViaChrome(parent context.Context, w io.Writer, headless bool, timeout time.Duration) (harvestEnvelope, error) {
	var empty harvestEnvelope
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	home, err := os.UserHomeDir()
	if err != nil {
		return empty, fmt.Errorf("locating home dir: %w", err)
	}
	userDataDir := filepath.Join(home, ".config", "peloton-pp-cli", "chrome")
	if err := os.MkdirAll(userDataDir, 0o700); err != nil {
		return empty, fmt.Errorf("creating chrome profile dir: %w", err)
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserDataDir(userDataDir),
		chromedp.Flag("headless", headless),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()

	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()

	fmt.Fprintln(w, "Opening Chrome — sign in to Peloton normally; the token is harvested automatically.")
	if !headless {
		fmt.Fprintln(w, "(Profile persists at ~/.config/peloton-pp-cli/chrome — subsequent logins are faster.)")
	}
	if err := chromedp.Run(browserCtx, chromedp.Navigate(loginURL)); err != nil {
		return empty, fmt.Errorf("navigating to %s: %w", loginURL, err)
	}

	if u, p := os.Getenv("PELOTON_USERNAME"), os.Getenv("PELOTON_PASSWORD"); u != "" && p != "" {
		time.Sleep(1500 * time.Millisecond)
		var ok bool
		_ = chromedp.Run(browserCtx, chromedp.Evaluate(fmt.Sprintf(fillLoginExpr, u, p), &ok))
		if ok {
			fmt.Fprintln(w, "Pre-filled username/password from env. Click 'Log in' to continue.")
		}
	}

	deadline := time.Now().Add(timeout)
	announceEvery := 30 * time.Second
	nextAnnounce := time.Now().Add(announceEvery)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return empty, ctx.Err()
		default:
		}
		var raw string
		if err := chromedp.Run(browserCtx, chromedp.Evaluate(readTokenExpr, &raw)); err == nil && raw != "" {
			var env harvestEnvelope
			if jerr := json.Unmarshal([]byte(raw), &env); jerr == nil && env.Token != "" {
				fmt.Fprintln(w, "Token detected — saving.")
				return env, nil
			}
		}
		if time.Now().After(nextAnnounce) {
			elapsed := time.Since(deadline.Add(-timeout)).Round(time.Second)
			fmt.Fprintf(w, "Still waiting for sign-in (%s elapsed, timeout %s)…\n", elapsed, timeout)
			nextAnnounce = nextAnnounce.Add(announceEvery)
		}
		time.Sleep(pollInterval)
	}
	return empty, fmt.Errorf("timed out after %s waiting for sign-in", timeout)
}

func saveHarvestedAuth(w io.Writer, flags *rootFlags, env harvestEnvelope) error {
	if env.Token == "" {
		return authErr(fmt.Errorf("harvest returned no token; aborting save"))
	}
	if len(env.Token) < 20 {
		return authErr(fmt.Errorf("harvested token suspiciously short (%d chars); aborting", len(env.Token)))
	}
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return configErr(fmt.Errorf("loading config: %w", err))
	}
	// Auth0 SPA tokens are nominally 1 hour Peloton-side. SaveTokens stamps
	// expiry so doctor can warn when stale; we don't have a refresh-token
	// flow (Peloton's Auth0 SDK manages refresh internally and we'd need to
	// re-spawn Chrome anyway), so refreshToken is empty.
	if err := cfg.SaveTokens("", "", env.Token, "", time.Now().Add(time.Hour)); err != nil {
		return configErr(fmt.Errorf("saving config: %w", err))
	}
	fmt.Fprintf(w, "Saved Peloton bearer token to %s.\n", cfg.Path)
	return nil
}
