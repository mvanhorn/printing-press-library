// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/auth"
	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/cliutil"
	"github.com/spf13/cobra"
)

// authLoginCDPFactory is a package-level test seam. Production code falls
// through to the real auth.CDPClient. Tests inject a stub that returns a
// CDPClient pinned at an httptest.NewServer mock so the full RunE path can
// be exercised without a real Chrome.
var authLoginCDPFactory = func(port int) *auth.CDPClient {
	return &auth.CDPClient{Port: port}
}

// authLoginExtractFn is the test seam for the per-tab extraction step.
// Wrapping auth.ExtractFromTab here keeps test wiring lean: a test can
// override this to return canned ExtractedTokens without standing up a
// WebSocket mock for the IIFE round-trip. Production = auth.ExtractFromTab.
var authLoginExtractFn = func(ctx context.Context, c *auth.CDPClient, tab auth.Tab) (*auth.ExtractedTokens, error) {
	return auth.ExtractFromTab(ctx, c, tab)
}

// portReadyPollInterval/-Timeout govern the auto-launch wait loop. Tests
// shrink these via package-level vars so the verify-mode short-circuit
// doesn't have to wait the full 30s budget.
var (
	portReadyPollInterval = 500 * time.Millisecond
	portReadyTimeout      = 30 * time.Second
)

// Test seams for the disk-auth path. Production wires these to the real
// disk-reader + Chrome-cookie functions. Tests override them to inject
// fixture data without touching the user's real Chrome profile.
var (
	authLoginReadLocalStorageFn = func(profileDir string) (map[string]string, error) {
		return auth.ReadSuperhumanLocalStorage(profileDir)
	}
	authLoginReadCookiesFn = func(host string) (map[string]string, error) {
		return auth.DecryptedChromeCookies(host)
	}
	authLoginRefreshFn = func(ctx context.Context, email, googleID string) (*auth.CookieAuthResult, error) {
		return auth.RefreshFromChromeCookies(ctx, email, googleID)
	}
)

func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var (
		useChrome        bool
		useDisk          bool
		autoLaunchChrome bool
		cdpPort          int
	)

	cmd := &cobra.Command{
		Use:     "login",
		Short:   "Sign in by reading Chrome's on-disk session (no debug port required)",
		Example: "  superhuman-pp-cli auth login --disk\n  superhuman-pp-cli auth login --disk --account user@example.com",
		Annotations: map[string]string{
			// Exit code 4 is the conventional "setup required" code (same
			// as authErr — Chrome-not-running is functionally an auth-side
			// setup gap, not a usage error).
			"pp:typed-exit-codes": "0,4",
			// MCP exposure: hidden because the flow requires the user to
			// interact with their own Chrome session, which an agent can't
			// drive remotely.
			"mcp:hidden": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default transport: when neither --chrome nor --disk is
			// supplied, route to the disk path. The CDP path is on life
			// support (Chrome 148+ silently ignores --remote-debugging-port
			// on the default profile) and the disk path is the durable
			// replacement.
			if !useChrome && !useDisk {
				useDisk = true
			}
			if dryRunOK(flags) {
				return nil
			}
			if useDisk {
				return runDiskLogin(cmd, flags, flags.account)
			}
			return runAuthLogin(cmd, flags, runAuthLoginOpts{
				account:          flags.account,
				autoLaunchChrome: autoLaunchChrome,
				cdpPort:          cdpPort,
			})
		},
	}

	cmd.Flags().BoolVar(&useDisk, "disk", false, "Read Chrome's on-disk cookies + localStorage to mint a fresh JWT (default; no debug port required)")
	cmd.Flags().BoolVar(&useChrome, "chrome", false, "(DEPRECATED — use --disk; Chrome 148+ silently ignores --remote-debugging-port on default profile) Attach to Chrome via CDP and extract tokens")
	cmd.Flags().BoolVar(&autoLaunchChrome, "auto-launch-chrome", false, "(deprecated path) Launch a separate Chrome instance for CDP token capture")
	cmd.Flags().IntVar(&cdpPort, "cdp-port", 0, "(deprecated path) Chrome remote-debugging port (default: auto-discover starting at 9222)")
	return cmd
}

// runDiskLogin implements `auth login --disk`: read accounts.superhuman.com
// cookies + Chrome's Superhuman localStorage, enumerate every signed-in
// account, mint a fresh JWT per account via RefreshFromChromeCookies, and
// upsert each into the token store.
//
// Multi-account is the headline ergonomic. The user runs this once and both
// (or N) of their accounts are persisted; `--account <email>` narrows to one.
//
// Verify-friendly: under PRINTING_PRESS_VERIFY=1 we skip the real HTTP refresh
// and write placeholder tokens so doctor/verify probes don't require a real
// keychain prompt + network round-trip.
func runDiskLogin(cmd *cobra.Command, flags *rootFlags, accountFilter string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	cfg, err := flags.loadConfig()
	if err != nil {
		return configErr(err)
	}

	verify := cliutil.IsVerifyEnv()

	// Step 1: resolve Chrome user-data-dir and read Superhuman's localStorage
	// so we can map googleId -> email. Keys look like `user@example.com:id`
	// with values like `"123456789012345678901"` (JSON-quoted).
	dataDir, err := auth.ChromeDataDir()
	if err != nil {
		return authErr(fmt.Errorf("auth login --disk: resolve chrome data dir: %w", err))
	}
	profileDir := filepath.Join(dataDir, "Default")
	kv, err := authLoginReadLocalStorageFn(profileDir)
	if err != nil {
		return authErr(fmt.Errorf("auth login --disk: read chrome localStorage: %w (hint: log in to mail.superhuman.com in Chrome first)", err))
	}
	emailToID := emailToGoogleIDFromLocalStorage(kv)
	if len(emailToID) == 0 {
		return authErr(fmt.Errorf("auth login --disk: no Superhuman accounts found in Chrome localStorage; log in to mail.superhuman.com in Chrome first"))
	}

	// Step 2: read accounts.superhuman.com cookies and enumerate googleIds.
	cookies, err := authLoginReadCookiesFn("accounts.superhuman.com")
	if err != nil {
		return authErr(fmt.Errorf("auth login --disk: read chrome cookies: %w (hint: log in to mail.superhuman.com in Chrome first; if Keychain prompted, click Always Allow and retry)", err))
	}
	googleIDs := auth.ListAccountsOnAccountsHost(cookies)
	if len(googleIDs) == 0 {
		return authErr(fmt.Errorf("auth login --disk: no per-account cookies on accounts.superhuman.com; log in to mail.superhuman.com in Chrome first"))
	}

	// Step 3: refresh each account, upsert into the store. Filter by
	// --account if requested.
	store := auth.NewStoreAt(cfg.TokenStorePath())
	var captured []capturedAccount
	var skippedFiltered int
	for _, gid := range googleIDs {
		email := emailForGoogleID(emailToID, gid)
		if email == "" {
			// googleId cookie with no matching localStorage entry — likely
			// a signed-out-but-cookie-lingering account. Skip silently.
			continue
		}
		if accountFilter != "" && accountFilter != email {
			skippedFiltered++
			continue
		}

		acct, err := captureDiskAccount(ctx, email, gid, verify)
		if err != nil {
			return authErr(fmt.Errorf("auth login --disk: refresh %s: %w", email, err))
		}
		if _, err := store.Upsert(email, acct); err != nil {
			return configErr(fmt.Errorf("auth login --disk: persist %s: %w", email, err))
		}
		captured = append(captured, capturedAccount{
			email:     email,
			expiresMs: acct.SuperhumanToken.Expires,
		})
	}

	if len(captured) == 0 {
		if accountFilter != "" {
			available := sortedEmails(emailToID)
			return authErr(fmt.Errorf("auth login --disk: account %q not found in Chrome cookies; available: %s", accountFilter, strings.Join(available, ", ")))
		}
		return authErr(fmt.Errorf("auth login --disk: no accounts captured; ensure you've logged into mail.superhuman.com in Chrome first"))
	}

	// Step 4: render output. JSON envelope for agents, friendly text for humans.
	w := cmd.OutOrStdout()
	if flags.asJSON {
		rows := make([]map[string]any, 0, len(captured))
		for _, c := range captured {
			rows = append(rows, map[string]any{
				"saved":            true,
				"email":            c.email,
				"id_token_expires": c.expiresMs,
			})
		}
		out := map[string]any{
			"captured":         len(captured),
			"accounts":         rows,
			"token_store_path": cfg.TokenStorePath(),
		}
		return printJSONFiltered(w, out, flags)
	}

	fmt.Fprintf(w, "Captured %d account(s):\n", len(captured))
	for _, c := range captured {
		expiresIn := time.Until(time.UnixMilli(c.expiresMs))
		fmt.Fprintf(w, "  %s (expires in %s)\n", c.email, humanizeDuration(expiresIn))
	}
	return nil
}

// capturedAccount is a per-account row produced by runDiskLogin. Kept
// package-private; tests assert via the store + stdout, not this struct.
type capturedAccount struct {
	email     string
	expiresMs int64
}

// captureDiskAccount runs the cookie-pipeline refresh (or its verify-mode
// short-circuit) and maps the result onto an AccountTokens. Pulled out so
// runDiskLogin's main flow stays a sequence of clearly-named steps.
func captureDiskAccount(ctx context.Context, email, googleID string, verify bool) (auth.AccountTokens, error) {
	if verify {
		// Verify mode: never make real HTTP calls. Write a placeholder that
		// doctor / verify probes can observe but that won't pass real backend
		// auth — the expiry is 1h ahead so classifyAccount labels it valid.
		now := time.Now().UnixMilli()
		return auth.AccountTokens{
			Type:           "google",
			AccessToken:    "verify-mode-access-token",
			UserID:         googleID,
			UserExternalID: "user_verify_mode_" + googleID,
			DeviceID:       "verify-mode-device",
			SuperhumanToken: auth.SuperhumanToken{
				Token:   "verify-mode-id-token",
				Expires: now + int64(time.Hour/time.Millisecond),
			},
			LastUsedAt: now,
		}, nil
	}
	result, err := authLoginRefreshFn(ctx, email, googleID)
	if err != nil {
		return auth.AccountTokens{}, err
	}
	if result == nil || result.IDToken == "" {
		return auth.AccountTokens{}, fmt.Errorf("refresh returned no idToken")
	}
	return auth.AccountTokens{
		// CookieAuthResult is google-only by construction (cookies named
		// by googleId, exchanged at the Google-Firebase accounts service).
		// If we add Microsoft support later, this is the one site to flip.
		Type:           "google",
		AccessToken:    result.AccessToken,
		UserID:         result.GoogleID,
		UserExternalID: result.ExternalID,
		DeviceID:       result.DeviceID,
		SuperhumanToken: auth.SuperhumanToken{
			Token:   result.IDToken,
			Expires: result.IDTokenExpires,
		},
		LastUsedAt: time.Now().UnixMilli(),
	}, nil
}

// emailToGoogleIDFromLocalStorage walks Superhuman localStorage entries and
// pulls out keys of the form `<email>:id` whose values are the user's Google
// ID (JSON-quoted in localStorage). Returns email -> googleId. Unknown keys
// are ignored.
func emailToGoogleIDFromLocalStorage(kv map[string]string) map[string]string {
	out := make(map[string]string, len(kv))
	for k, v := range kv {
		if !strings.HasSuffix(k, ":id") {
			continue
		}
		email := strings.TrimSuffix(k, ":id")
		if !strings.Contains(email, "@") {
			continue
		}
		id := strings.Trim(v, `"`)
		if id == "" {
			continue
		}
		out[email] = id
	}
	return out
}

// emailForGoogleID reverse-looks-up the email for a given googleId. Returns
// "" if not found (caller skips such accounts).
func emailForGoogleID(emailToID map[string]string, googleID string) string {
	for email, id := range emailToID {
		if id == googleID {
			return email
		}
	}
	return ""
}

// sortedEmails returns the keys of emailToID in lexicographic order. Used
// only for error messages so the user gets a deterministic "available
// accounts" list.
func sortedEmails(emailToID map[string]string) []string {
	out := make([]string, 0, len(emailToID))
	for e := range emailToID {
		out = append(out, e)
	}
	sort.Strings(out)
	return out
}

// runAuthLoginOpts bundles the user-facing options so the body below isn't
// burdened with positional plumbing. Kept package-private; tests drive
// RunE via cmd.Execute() instead of poking at this struct.
type runAuthLoginOpts struct {
	account          string
	autoLaunchChrome bool
	cdpPort          int
}

// runAuthLogin is the verifiable RunE body. Pulled out so each early-return
// is one statement and the test path can drive the same code without going
// through cobra.
func runAuthLogin(cmd *cobra.Command, flags *rootFlags, opts runAuthLoginOpts) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	cfg, err := flags.loadConfig()
	if err != nil {
		return configErr(err)
	}

	w := cmd.OutOrStdout()
	stderr := cmd.ErrOrStderr()

	cdpClient := authLoginCDPFactory(opts.cdpPort)
	port, err := cdpClient.DiscoverPort(ctx)
	if err != nil {
		if !errors.Is(err, auth.ErrChromeNotRunning) {
			// Some other transport-level error (proxy in front, etc.).
			// Surface verbatim so the user can act on it.
			return authErr(fmt.Errorf("auth login: %w", err))
		}
		if opts.autoLaunchChrome {
			if launchErr := launchChromeForLogin(ctx, cdpClient, stderr); launchErr != nil {
				return authErr(fmt.Errorf("auth login: %w", launchErr))
			}
			if cliutil.IsVerifyEnv() {
				// Verify mode: launch was a print-only stub. Don't try to
				// re-discover the port; just exit cleanly.
				return nil
			}
			// Re-discover after a successful launch.
			port, err = cdpClient.DiscoverPort(ctx)
			if err != nil {
				return authErr(fmt.Errorf("auth login: chrome launched but cdp port not ready: %w", err))
			}
		} else {
			// The wrapped err already contains the relaunchHint text per
			// U1's contract; surface it directly so the user copy-pastes
			// the recommended command.
			fmt.Fprintln(stderr, err.Error())
			return authErr(fmt.Errorf("auth login: chrome remote-debugging not enabled"))
		}
	}

	// Enumerate Superhuman tabs.
	tabs, err := cdpClient.ListTabs(ctx, port)
	if err != nil {
		return authErr(fmt.Errorf("auth login: %w", err))
	}
	sh := auth.FilterSuperhumanTabs(tabs)
	if len(sh) == 0 {
		return authErr(fmt.Errorf("auth login: no Superhuman tab found in Chrome; open https://mail.superhuman.com first"))
	}

	tab, err := pickTabForLogin(sh, opts.account)
	if err != nil {
		return authErr(fmt.Errorf("auth login: %w", err))
	}

	// Run the extraction IIFE against the chosen tab.
	tokens, err := authLoginExtractFn(ctx, cdpClient, tab)
	if err != nil {
		return authErr(fmt.Errorf("auth login: %w", err))
	}
	if tokens == nil || tokens.Email == "" {
		return authErr(fmt.Errorf("auth login: extract returned no tokens"))
	}

	// Persist. Map ExtractedTokens -> AccountTokens.
	store := auth.NewStoreAt(cfg.TokenStorePath())
	acct := auth.AccountTokens{
		Type:           tokens.Provider,
		AccessToken:    tokens.AccessToken,
		RefreshToken:   tokens.RefreshToken,
		Expires:        tokens.AccessTokenExpires,
		UserID:         tokens.UserID,
		UserPrefix:     tokens.UserPrefix,
		UserExternalID: tokens.UserExternalID,
		DeviceID:       tokens.DeviceID,
		SuperhumanToken: auth.SuperhumanToken{
			Token:   tokens.IDToken,
			Expires: tokens.IDTokenExpires,
		},
		LastUsedAt: time.Now().UnixMilli(),
	}
	if _, err := store.Upsert(tokens.Email, acct); err != nil {
		return configErr(fmt.Errorf("auth login: persist: %w", err))
	}

	// JSON envelope so agent callers get structured output. Matches the
	// auth status JSON shape so downstream tooling can reuse the same
	// account row parser.
	if flags.asJSON {
		out := map[string]any{
			"saved":             true,
			"email":             tokens.Email,
			"provider":          tokens.Provider,
			"id_token_expires":  tokens.IDTokenExpires,
			"has_refresh_token": tokens.RefreshToken != "",
			"token_store_path":  cfg.TokenStorePath(),
		}
		return printJSONFiltered(w, out, flags)
	}

	// Human output.
	expiresIn := time.Until(time.UnixMilli(tokens.IDTokenExpires))
	fmt.Fprintf(w, "Saved %s.\n", tokens.Email)
	fmt.Fprintf(w, "  ID token expires in %s (auto-refresh enabled)\n", humanizeDuration(expiresIn))
	if tokens.RefreshToken != "" {
		fmt.Fprintln(w, "  Refresh token persisted; valid for ~30 days while account stays active in Chrome")
	} else {
		fmt.Fprintln(w, "  WARNING: no refresh token captured (legacy or set-token path). Re-run with a logged-in Chrome session.")
	}
	return nil
}

// pickTabForLogin selects the right Superhuman tab from the discovered set
// using the user's --account pin (when set). Mirrors auth.pickTab's
// semantics but stays at the CLI layer so command-level error wrapping is
// consistent with the rest of auth.go.
func pickTabForLogin(tabs []auth.Tab, email string) (auth.Tab, error) {
	if len(tabs) == 0 {
		return auth.Tab{}, fmt.Errorf("no Superhuman tabs available")
	}
	if email == "" {
		if len(tabs) == 1 {
			return tabs[0], nil
		}
		emails := emailsFromLoginTabs(tabs)
		return auth.Tab{}, fmt.Errorf("multiple Superhuman tabs found (%s); re-run with --account <email>", strings.Join(emails, ", "))
	}
	for _, t := range tabs {
		if tabURLContains(t.URL, email) {
			return t, nil
		}
	}
	emails := emailsFromLoginTabs(tabs)
	return auth.Tab{}, fmt.Errorf("account %q not found among open Superhuman tabs (%s)", email, strings.Join(emails, ", "))
}

// tabURLContains reports whether a tab's URL appears to belong to the given
// account. Substring match catches every routing variant (threads, drafts,
// settings, etc.) — Superhuman tab URLs look like
// https://mail.superhuman.com/<email>/...
func tabURLContains(u, email string) bool {
	return email != "" && strings.Contains(u, email)
}

// emailsFromLoginTabs pulls the <email> segment out of each Superhuman tab
// URL for use in error messages.
func emailsFromLoginTabs(tabs []auth.Tab) []string {
	const prefix = "https://mail.superhuman.com/"
	out := make([]string, 0, len(tabs))
	for _, t := range tabs {
		u := t.URL
		if !strings.HasPrefix(u, prefix) {
			continue
		}
		rest := u[len(prefix):]
		for i := 0; i < len(rest); i++ {
			if rest[i] == '/' || rest[i] == '?' || rest[i] == '#' {
				rest = rest[:i]
				break
			}
		}
		if rest != "" {
			out = append(out, rest)
		}
	}
	if len(out) == 0 {
		// Fall back to raw URLs so the error message is never empty.
		for _, t := range tabs {
			out = append(out, t.URL)
		}
	}
	return out
}

// launchChromeForLogin handles the --auto-launch-chrome path. Honors
// cliutil.IsVerifyEnv() by printing the would-launch line and returning nil
// instead of actually spawning Chrome. Per AGENTS.md side-effect rule the
// launch is opt-in via --auto-launch-chrome; default behavior (no flag) does
// NOT reach this function — caller checks the flag.
func launchChromeForLogin(ctx context.Context, c *auth.CDPClient, stderr io.Writer) error {
	profileDir, err := chromeProfileDir()
	if err != nil {
		return err
	}
	launchCmd := fmt.Sprintf("open -a 'Google Chrome' --args --remote-debugging-port=9222 --user-data-dir=%s", profileDir)
	if cliutil.IsVerifyEnv() {
		fmt.Fprintf(stderr, "would launch: %s\n", launchCmd)
		return nil
	}
	if err := os.MkdirAll(profileDir, 0o700); err != nil {
		return fmt.Errorf("create chrome profile dir: %w", err)
	}
	fmt.Fprintf(stderr, "Launching a separate Chrome instance for token capture (profile: %s)\n", profileDir)
	fmt.Fprintln(stderr, "This is a NEW profile; your main Chrome session is untouched.")
	args := []string{
		"-a", "Google Chrome",
		"--args",
		"--remote-debugging-port=9222",
		"--user-data-dir=" + profileDir,
		"https://mail.superhuman.com/",
	}
	launch := exec.CommandContext(ctx, "open", args...)
	if err := launch.Start(); err != nil {
		return fmt.Errorf("launch chrome: %w", err)
	}
	// Don't Wait — `open` returns immediately after dispatching to LaunchServices.
	// Poll for the port to come up.
	deadline := time.Now().Add(portReadyTimeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if _, perr := c.DiscoverPort(ctx); perr == nil {
			return nil
		}
		time.Sleep(portReadyPollInterval)
	}
	return fmt.Errorf("chrome launched but remote-debugging port did not become ready within %s", portReadyTimeout)
}

// chromeProfileDir resolves the dedicated user-data-dir for the
// auto-launched Chrome instance. Uses $XDG_DATA_HOME when set, else
// ~/.local/share. Kept here so the test surface area stays inside the
// auth_login files.
func chromeProfileDir() (string, error) {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "superhuman-pp-cli", "chrome-profile"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".local", "share", "superhuman-pp-cli", "chrome-profile"), nil
}
