package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/care/internal/client"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/storage"
	"github.com/chromedp/chromedp"
	"github.com/spf13/cobra"
)

// care.com auth uses a persistent Chrome profile (login once, valid for weeks)
// instead of press-auth, because care.com's passwordless magic-link login
// breaks controlled-window completion detection and its single-active-session
// rotation makes ephemeral captures go stale. See care_cookie.go for how the
// client consumes the cached cookie.

func careProfileDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".care-pp-cli", "chrome-profile")
}

func careChromeExecPath() string {
	for _, p := range []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func careAllocOpts(headful bool) []chromedp.ExecAllocatorOption {
	opts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	opts = append(opts,
		chromedp.UserDataDir(careProfileDir()),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
	)
	if headful {
		opts = append(opts, chromedp.Flag("headless", false))
	}
	if p := careChromeExecPath(); p != "" {
		opts = append(opts, chromedp.ExecPath(p))
	}
	return opts
}

func careCookieHeader(cookies []*network.Cookie) string {
	var parts []string
	for _, c := range cookies {
		if strings.Contains(c.Domain, "care.com") {
			parts = append(parts, c.Name+"="+c.Value)
		}
	}
	return strings.Join(parts, "; ")
}

// careExtractCookies opens the persistent profile and returns the care.com
// cookie header. When waitForLogin is true it opens headful at the login page
// and blocks until the browser reaches an authenticated member area.
func careExtractCookies(parent context.Context, headful, waitForLogin bool) (string, error) {
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(parent, careAllocOpts(headful)...)
	defer cancelAlloc()
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	startURL := "https://www.care.com/app/mhp"
	if waitForLogin {
		startURL = "https://www.care.com/login"
	}
	// Navigation may be challenged (Akamai) or slow; cookies still load from the
	// persistent profile regardless, so a nav error is non-fatal.
	_ = chromedp.Run(ctx, chromedp.Navigate(startURL))

	if waitForLogin {
		deadline := time.Now().Add(5 * time.Minute)
		for {
			var loc string
			if err := chromedp.Run(ctx, chromedp.Location(&loc)); err != nil {
				return "", err
			}
			if strings.Contains(loc, "/app/") && !strings.Contains(loc, "/login") {
				break
			}
			if time.Now().After(deadline) {
				return "", fmt.Errorf("timed out waiting for login (5m)")
			}
			time.Sleep(2 * time.Second)
		}
	}

	var cookies []*network.Cookie
	if err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		c, err := storage.GetCookies().Do(ctx)
		cookies = c
		return err
	})); err != nil {
		return "", err
	}
	return careCookieHeader(cookies), nil
}

func careCacheCookies(header string) error {
	p := client.CareCookieCachePath()
	if p == "" {
		return fmt.Errorf("cannot resolve cookie cache path")
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(header), 0o600)
}

func newCareAuthRefreshCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "refresh",
		Short: "Refresh the care.com session cookie from the local Chrome profile (no re-login)",
		Long:  "Reads the session cookies from care-pp-cli's persistent Chrome profile (headless) and caches them for the CLI. The profile stays logged in for weeks, so this needs no interaction. Run it if commands start returning auth errors.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 90*time.Second)
			defer cancel()
			header, err := careExtractCookies(ctx, false, false)
			if err != nil {
				return fmt.Errorf("extracting cookies: %w", err)
			}
			if !strings.Contains(header, "csc-session") {
				return fmt.Errorf("no valid care.com session in profile; run: care-pp-cli auth profile-login")
			}
			if err := careCacheCookies(header); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Refreshed care.com session (%d bytes cached).\n", len(header))
			return nil
		},
	}
}

func newCareAuthProfileLoginCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "profile-login",
		Short: "Log in to care.com once in a persistent Chrome window (stays valid for weeks)",
		Long:  "Opens a Chrome window using care-pp-cli's dedicated profile (separate from your daily Chrome). Log in with the passwordless email option (NOT Google/Apple - those block automation). The CLI captures and caches your session; it persists for weeks. Use 'auth refresh' afterward to update the cookie without re-logging-in.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 6*time.Minute)
			defer cancel()
			w := cmd.OutOrStdout()
			fmt.Fprintln(w, "Opening care.com login in a controlled Chrome window...")
			fmt.Fprintln(w, "  - Use the passwordless EMAIL option (not Google/Apple).")
			fmt.Fprintln(w, "  - The window captures automatically once you reach your member home.")
			header, err := careExtractCookies(ctx, true, true)
			if err != nil {
				return fmt.Errorf("login: %w", err)
			}
			if err := careCacheCookies(header); err != nil {
				return err
			}
			fmt.Fprintf(w, "Login captured (%d bytes). You're set - the session persists for weeks.\n", len(header))
			return nil
		},
	}
}
