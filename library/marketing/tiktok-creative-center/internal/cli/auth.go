// Copyright 2026 Jon and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-implemented auth capture for the TikTok Creative Center CLI.
//
// TikTok Creative Center uses composed auth: a login session cookie plus an
// X-CSRFToken header. There is no official API key. `auth login --chrome`
// launches a Chrome profile, harvests cookies + the CSRF token from a live API
// request, and writes them into config.toml's [headers] section. The generated
// HTTP client applies Config.Headers to every request, so no client edits are
// needed. `auth login --manual` is the always-works fallback.

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/mvanhorn/printing-press-library/library/marketing/tiktok-creative-center/internal/config"
	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

const creativeCenterURL = "https://ads.tiktok.com/creative/creativeCenter/"

// csrfHeaderKey is the request header the Creative Center sends on API calls.
const csrfHeaderKey = "X-CSRFToken"

// csrfCookieCandidates are cookie names that may carry the CSRF token value,
// used as a fallback when no live request is observed.
var csrfCookieCandidates = []string{"tt_csrf_token", "passport_csrf_token", "csrf_session_id", "s_v_web_id"}

func newAuthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Capture and inspect your TikTok Creative Center session.",
		Long: "Creative Center uses a login session cookie plus an X-CSRFToken header (no official API key). " +
			"'auth login --chrome' captures both from a logged-in Chrome profile; 'auth login --manual' " +
			"accepts pasted values; 'auth status' reports what is configured.",
	}

	var flagChrome, flagManual bool
	var flagProfile, flagCookie, flagCSRF, flagUA string

	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Capture your Creative Center session into config.toml.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			chrome := flagChrome
			if !chrome && !flagManual {
				chrome = true // default to --chrome when neither is set
			}
			if chrome {
				return chromeLogin(cmd, flagProfile, flags)
			}
			return manualLogin(flagCookie, flagCSRF, flagUA, flags)
		},
	}
	loginCmd.Flags().BoolVar(&flagChrome, "chrome", false, "Capture from a logged-in Chrome profile (default)")
	loginCmd.Flags().BoolVar(&flagManual, "manual", false, "Paste Cookie/CSRF/UA values instead of launching Chrome")
	loginCmd.Flags().StringVar(&flagProfile, "profile", "", "Chrome user-data-dir (default: bundled profile; env TTCC_CHROME_PROFILE)")
	loginCmd.Flags().StringVar(&flagCookie, "cookie", "", "With --manual: the raw Cookie header value")
	loginCmd.Flags().StringVar(&flagCSRF, "csrf", "", "With --manual: the X-CSRFToken value")
	loginCmd.Flags().StringVar(&flagUA, "ua", "", "With --manual: the User-Agent string")

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Report whether a session is configured (values masked).",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			return authStatus(cmd, flags)
		},
	}

	cmd.AddCommand(loginCmd)
	cmd.AddCommand(statusCmd)
	return cmd
}

// chromeLogin launches Chrome, harvests cookies + CSRF, writes config.
func chromeLogin(cmd *cobra.Command, profileFlag string, flags *rootFlags) error {
	profilePath, err := resolveChromeProfile(profileFlag)
	if err != nil {
		return err
	}
	if _, err := os.Stat(profilePath); err != nil {
		fmt.Fprintf(os.Stderr, "Chrome profile not found at %s.\n", profilePath)
		fmt.Fprintf(os.Stderr, "Log into %s in Chrome first, or use 'auth login --manual'.\n", creativeCenterURL)
		return fmt.Errorf("chrome profile missing: %w", err)
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 90*time.Second)
	defer cancel()

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false),
		chromedp.UserDataDir(profilePath),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()
	taskCtx, cancelTask := chromedp.NewContext(allocCtx)
	defer cancelTask()

	var (
		mu        sync.Mutex
		csrfToken string
	)

	chromedp.ListenTarget(taskCtx, func(ev any) {
		req, ok := ev.(*network.EventRequestWillBeSent)
		if !ok || req.Request == nil {
			return
		}
		if !strings.Contains(req.Request.URL, "CreativeOne/") &&
			!strings.Contains(req.Request.URL, "cc_portal_api/") {
			return
		}
		headers := req.Request.Headers
		if v := headerLookup(headers, csrfHeaderKey); v != "" {
			mu.Lock()
			if csrfToken == "" {
				csrfToken = v
			}
			mu.Unlock()
		}
	})

	var cookies []*network.Cookie
	var ua string
	if err := chromedp.Run(taskCtx,
		network.Enable(),
		chromedp.Navigate(creativeCenterURL),
		chromedp.Sleep(5*time.Second),
		chromedp.ActionFunc(func(ctx context.Context) error {
			cs, err := network.GetCookies().Do(ctx)
			if err != nil {
				return err
			}
			cookies = cs
			var gotUA string
			if err := chromedp.Evaluate("navigator.userAgent", &gotUA).Do(ctx); err == nil {
				ua = gotUA
			}
			return nil
		}),
	); err != nil {
		fmt.Fprintf(os.Stderr, "Chrome capture failed: %v\n", err)
		fmt.Fprintf(os.Stderr, "Falling back hint: use 'auth login --manual --cookie ... --csrf ...'.\n")
		return fmt.Errorf("chrome capture failed: %w", err)
	}

	// Give late-firing XHR a chance if we did not catch the CSRF header.
	if csrfToken == "" {
		chromedp.Run(taskCtx, chromedp.Sleep(3*time.Second))
	}
	mu.Lock()
	csrf := csrfToken
	mu.Unlock()

	if csrf == "" {
		csrf = csrfFromCookies(cookies)
	}

	cookieHeader := buildCookieHeader(cookies)
	if cookieHeader == "" {
		return fmt.Errorf("no TikTok session cookies captured; log into %s in Chrome first", creativeCenterURL)
	}
	if csrf == "" {
		return fmt.Errorf("cookies captured but X-CSRFToken not found; retry, or use 'auth login --manual'")
	}
	if ua == "" {
		ua = defaultUserAgent
	}

	if err := writeAuthConfig(flags, cookieHeader, csrf, ua); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "captured %d cookies for tiktok.com\n", countTikTokCookies(cookies))
	fmt.Fprintf(os.Stderr, "CSRF token: %s\n", maskSecret(csrf))
	fmt.Fprintf(os.Stderr, "User-Agent: %s\n", ua)
	fmt.Fprintf(os.Stderr, "session written to config; run 'auth status' to verify.\n")
	return nil
}

// manualLogin writes auth values supplied via flags.
func manualLogin(cookie, csrf, ua string, flags *rootFlags) error {
	if cookie == "" || csrf == "" {
		return fmt.Errorf("--manual requires --cookie and --csrf (and optionally --ua); "+
			"copy them from your browser devtools Network request headers to %s", creativeCenterURL)
	}
	if ua == "" {
		ua = defaultUserAgent
	}
	if err := writeAuthConfig(flags, cookie, csrf, ua); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "CSRF token: %s\n", maskSecret(csrf))
	fmt.Fprintf(os.Stderr, "session written to config; run 'auth status' to verify.\n")
	return nil
}

// authStatus reports configured auth state with values masked.
func authStatus(cmd *cobra.Command, flags *rootFlags) error {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return configErr(err)
	}
	hasCookie := cfg.Headers["Cookie"] != ""
	hasCSRF := cfg.Headers[csrfHeaderKey] != "" || cfg.Headers[strings.ToLower(csrfHeaderKey)] != ""
	status := "configured"
	if !hasCookie || !hasCSRF {
		status = "missing"
	}
	return flags.printJSON(cmd, map[string]any{
		"status":      status,
		"cookie":      presentMasked(cfg.Headers["Cookie"]),
		"x_csrftoken": presentMasked(headerLookup(cfg.Headers, csrfHeaderKey)),
		"user_agent":  cfg.Headers["User-Agent"],
		"config_path": cfg.Path,
		"hint":        "run 'auth login --chrome' to capture or refresh",
	})
}

// writeAuthConfig loads the existing config, sets the auth headers, and writes
// it back as TOML, preserving base_url + any other [headers] entries.
func writeAuthConfig(flags *rootFlags, cookie, csrf, ua string) error {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return configErr(err)
	}
	headers := map[string]string{}
	for k, v := range cfg.Headers {
		headers[k] = v
	}
	headers["Cookie"] = cookie
	headers[csrfHeaderKey] = csrf
	headers["User-Agent"] = ua

	type configFile struct {
		BaseURL    string            `toml:"base_url,omitempty"`
		AuthHeader string            `toml:"auth_header,omitempty"`
		Headers    map[string]string `toml:"headers"`
	}
	out := configFile{
		BaseURL:    cfg.BaseURL,
		AuthHeader: cfg.AuthHeaderVal,
		Headers:    headers,
	}
	data, err := toml.Marshal(out)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Path), 0o700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	if err := os.WriteFile(cfg.Path, data, 0o600); err != nil {
		return fmt.Errorf("writing config %s: %w", cfg.Path, err)
	}
	return nil
}

// resolveChromeProfile picks the Chrome user-data-dir from flag, env, or default.
func resolveChromeProfile(profileFlag string) (string, error) {
	if profileFlag != "" {
		return profileFlag, nil
	}
	if p := os.Getenv("TTCC_CHROME_PROFILE"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "tiktok-creative-center-pp-cli", "chrome-profile"), nil
}

// buildCookieHeader builds a "name=value; name=value" string from tiktok cookies.
func buildCookieHeader(cookies []*network.Cookie) string {
	var parts []string
	for _, c := range cookies {
		if c == nil {
			continue
		}
		if !strings.Contains(c.Domain, "tiktok.com") {
			continue
		}
		if c.Name == "" {
			continue
		}
		parts = append(parts, c.Name+"="+c.Value)
	}
	return strings.Join(parts, "; ")
}

// countTikTokCookies counts captured cookies for tiktok.com.
func countTikTokCookies(cookies []*network.Cookie) int {
	n := 0
	for _, c := range cookies {
		if c != nil && strings.Contains(c.Domain, "tiktok.com") {
			n++
		}
	}
	return n
}

// csrfFromCookies extracts a CSRF token from a candidate cookie as a fallback.
func csrfFromCookies(cookies []*network.Cookie) string {
	for _, c := range cookies {
		if c == nil {
			continue
		}
		for _, name := range csrfCookieCandidates {
			if strings.EqualFold(c.Name, name) && c.Value != "" {
				return c.Value
			}
		}
	}
	return ""
}

// headerLookup finds a header value case-insensitively in a string map.
func headerLookup(headers any, key string) string {
	key = strings.ToLower(key)
	switch m := headers.(type) {
	case map[string]string:
		for k, v := range m {
			if strings.ToLower(k) == key {
				return v
			}
		}
	case network.Headers:
		for k, v := range m {
			if strings.ToLower(k) == key {
				return toStr(v)
			}
		}
	}
	return ""
}

// presentMasked returns "present (****)" for a non-empty value, else "absent".
func presentMasked(v string) string {
	if v == "" {
		return "absent"
	}
	return "present (" + maskSecret(v) + ")"
}

// maskSecret returns a masked view of a secret for safe display.
func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 4 {
		return "****"
	}
	return "****" + s[len(s)-4:]
}

// defaultUserAgent is a recent desktop Chrome UA used when capture omits one.
const defaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"
