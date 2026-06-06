// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

// auth_chrome.go implements `auth login --chrome`. Function Health blocks
// direct Firebase REST password sign-in at the project level (the legacy
// daveremy/function-health-mcp tool has been broken since May 2026 for
// this exact reason), so the durable auth path is:
//
//   1. Open a real Chrome instance pointed at the user's Default profile,
//      which already has their Firebase session.
//   2. Navigate to my.functionhealth.com and let the SPA hydrate Firebase
//      Auth into IndexedDB.
//   3. Read the idToken + refreshToken from the Firebase storage entry via
//      JavaScript and capture them into a `window` variable.
//   4. Save them via Config.SaveTokens.
//
// The user does need to briefly quit Chrome before this runs because
// browser-use loads the same profile and acquires the profile lock.
// browser-use ships via `uvx` so no separate install is required.
//
// Future refinements: read Chrome's IndexedDB LevelDB directly (no browser
// shell-out) once a pure-Go LevelDB reader for IndexedDB key encoding is
// vetted. For now, browser-use is the most reliable path.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/other/function-health/internal/config"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newAuthLoginChromeFlow(parent context.Context, flags *rootFlags, cfg *config.Config, profile string) error {
	// Root the timeout at the command's context so Ctrl+C cancels the whole
	// Chrome flow instead of leaving it running until its own 90s budget.
	ctx, cancel := context.WithTimeout(parent, 90*time.Second)
	defer cancel()

	if err := requireChromeClosed(); err != nil {
		return err
	}
	if _, err := exec.LookPath("uvx"); err != nil {
		return fmt.Errorf("`uvx` not found on PATH — install uv (https://docs.astral.sh/uv/) so we can drive browser-use without a system install: %w", err)
	}

	// Reset any prior session so the profile loads cleanly.
	_ = runUVXBrowserUse(ctx, []string{"--session", "fh-auth-login", "close"})

	// Open my.functionhealth.com with the user's Chrome profile.
	if err := runUVXBrowserUse(ctx, []string{"--profile", profile, "--session", "fh-auth-login", "open", "https://my.functionhealth.com/"}); err != nil {
		return fmt.Errorf("open my.functionhealth.com via browser-use: %w", err)
	}
	defer func() {
		_ = runUVXBrowserUse(context.Background(), []string{"--session", "fh-auth-login", "close"})
	}()

	// Wait for the SPA to hydrate Firebase into IndexedDB. Firebase JS SDK
	// init can take 5-15s on cold open of a profile.
	if err := sleepOrDone(ctx, 8*time.Second); err != nil {
		return err
	}

	kickJS := `
window.__fh_token = 'pending';
var req = indexedDB.open('firebaseLocalStorageDb');
req.onsuccess = function(ev){
  var db = ev.target.result;
  var stores = Array.from(db.objectStoreNames);
  if(!stores.length){ window.__fh_token = 'NO_STORES:'+db.version; return; }
  var tx = db.transaction(stores[0], 'readonly');
  var os = tx.objectStore(stores[0]);
  var req2 = os.getAll();
  req2.onsuccess = function(){
    var entries = req2.result || [];
    var entry = entries.find(function(e){return e && e.fbase_key && e.fbase_key.indexOf('authUser')>=0;});
    if(!entry){ window.__fh_token = 'NO_AUTH_ENTRY:keys='+entries.map(function(x){return (x&&x.fbase_key)||'?';}).join(','); return; }
    var v = entry.value || {};
    var stm = v.stsTokenManager || {};
    window.__fh_token = JSON.stringify({
      idToken: stm.accessToken || '',
      refreshToken: stm.refreshToken || '',
      expirationTime: stm.expirationTime || 0,
      email: v.email || ((v.providerData && v.providerData[0] && v.providerData[0].email) || ''),
      localId: v.uid || ''
    });
  };
  req2.onerror = function(){ window.__fh_token = 'GETALL_ERROR'; };
};
req.onerror = function(){ window.__fh_token = 'OPEN_ERROR'; };
'kicked'
`
	// Poll for the result with backoff. Firebase init can take 5-15s on cold
	// open of a profile.
	var out string
	for attempt := 0; attempt < 8; attempt++ {
		if err := runUVXBrowserUse(ctx, []string{"--session", "fh-auth-login", "eval", kickJS}); err != nil {
			return fmt.Errorf("install IndexedDB reader: %w", err)
		}
		if err := sleepOrDone(ctx, 2*time.Second); err != nil {
			return err
		}
		o, err := runUVXBrowserUseStdout(ctx, []string{"--session", "fh-auth-login", "eval", "window.__fh_token"})
		if err != nil {
			return fmt.Errorf("read IndexedDB result: %w", err)
		}
		out = strings.TrimSpace(o)
		if strings.HasPrefix(out, "result: ") {
			out = strings.TrimPrefix(out, "result: ")
		}
		// Success: JSON-shaped (starts with `{` or with quoted JSON)
		if strings.HasPrefix(out, `"{`) || strings.HasPrefix(out, "{") {
			break
		}
		// Click any visible element to make sure Firebase finishes init.
		_ = runUVXBrowserUse(ctx, []string{"--session", "fh-auth-login", "scroll", "down"})
		if err := sleepOrDone(ctx, 2*time.Second); err != nil {
			return err
		}
	}
	if strings.HasPrefix(out, "pending") || strings.HasPrefix(out, "NO_STORES") ||
		strings.HasPrefix(out, "NO_AUTH_ENTRY") || strings.HasPrefix(out, "GETALL_ERROR") || strings.HasPrefix(out, "OPEN_ERROR") {
		return fmt.Errorf("Chrome session has no Firebase IndexedDB entry yet (%q). Open https://my.functionhealth.com in your normal Chrome, confirm you can see your dashboard, then quit Chrome and retry", truncate(out, 120))
	}
	out = strings.Trim(out, `"`)
	out = strings.ReplaceAll(out, `\"`, `"`)

	var tok struct {
		IDToken        string `json:"idToken"`
		RefreshToken   string `json:"refreshToken"`
		ExpirationTime any    `json:"expirationTime"`
		Email          string `json:"email"`
		LocalID        string `json:"localId"`
	}
	if err := json.Unmarshal([]byte(out), &tok); err != nil {
		return fmt.Errorf("parse IndexedDB result (%q): %w", truncate(out, 200), err)
	}
	if tok.IDToken == "" {
		return errors.New("Chrome session has no Firebase idToken — please sign in to https://my.functionhealth.com first, then retry")
	}

	expiry := parseExpirationMillis(tok.ExpirationTime)
	if expiry.IsZero() {
		expiry = time.Now().Add(50 * time.Minute)
	}
	cfg.AuthSource = "firebase-chrome"
	if err := cfg.SaveTokens("", "", tok.IDToken, tok.RefreshToken, expiry); err != nil {
		return fmt.Errorf("save tokens: %w", err)
	}
	return nil
}

func parseExpirationMillis(v any) time.Time {
	switch t := v.(type) {
	case float64:
		if t > 0 {
			return time.UnixMilli(int64(t))
		}
	case string:
		// Firebase emits as a number in JS but JSON could carry it as a string.
		var ms int64
		fmt.Sscanf(t, "%d", &ms)
		if ms > 0 {
			return time.UnixMilli(ms)
		}
	}
	return time.Time{}
}

func requireChromeClosed() error {
	out, err := exec.Command("pgrep", "-x", "Google Chrome").Output()
	if err != nil {
		// Non-zero exit means no process found — what we want.
		return nil
	}
	if len(strings.TrimSpace(string(out))) > 0 {
		return errors.New("Chrome is running. Quit Chrome (Cmd+Q, make sure no windows remain), then re-run `function-health-pp-cli auth login --chrome`")
	}
	return nil
}

// sleepOrDone waits for d to elapse, returning early with the context's error
// if it is cancelled first (e.g. the user pressed Ctrl+C). Bare time.Sleep
// would ignore cancellation and keep the flow blocked for the full duration.
func sleepOrDone(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func runUVXBrowserUse(ctx context.Context, args []string) error {
	cmd := exec.CommandContext(ctx, "uvx", append([]string{"browser-use"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("browser-use %v: %w\n%s", args, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func runUVXBrowserUseStdout(ctx context.Context, args []string) (string, error) {
	cmd := exec.CommandContext(ctx, "uvx", append([]string{"browser-use"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("browser-use %v: %w\n%s", args, err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// extendAuthLoginWithChrome rewrites the auth-login command's RunE to a
// version that also accepts --chrome.
func extendAuthLoginWithChrome(cmd *cobra.Command, flags *rootFlags) {
	var useChrome bool
	var chromeProfile string

	prevRun := cmd.RunE
	cmd.Flags().BoolVar(&useChrome, "chrome", false, "Extract the Firebase idToken from your Chrome profile (no password needed; bypasses Function Health's REST-password block)")
	cmd.Flags().StringVar(&chromeProfile, "chrome-profile", "Default", "Chrome profile directory to load. Chrome's first/only profile is 'Default'; additional profiles are 'Profile 1', 'Profile 2', … (the display name 'Person 1' is the *second* profile's directory 'Profile 1')")

	cmd.RunE = func(c *cobra.Command, args []string) error {
		if useChrome {
			if dryRunOK(flags) {
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			if err := newAuthLoginChromeFlow(c.Context(), flags, cfg, chromeProfile); err != nil {
				return authErr(err)
			}
			fmt.Fprintf(c.OutOrStdout(),
				"Saved Firebase idToken from Chrome profile %q.\n  config: %s\n  next: function-health-pp-cli sync\n",
				chromeProfile, cfg.Path)
			return nil
		}
		return prevRun(c, args)
	}
}
