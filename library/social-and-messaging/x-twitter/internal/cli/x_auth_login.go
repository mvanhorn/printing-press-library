// Hand-authored cookie-capture for X Articles (Source B / browser session).
//
// X Articles is a browser-only authoring surface (x.com GraphQL) that the v2
// API tokens cannot reach — it needs the same auth_token + ct0 session cookies
// the web app uses. The v2 API itself stays on X_BEARER_TOKEN / X_OAUTH2_USER_TOKEN
// (Source A); this command only captures the cookie session that the Articles
// commands read from ~/.config/x-twitter-pp-cli/cookies.json.
//
// auth_token is httpOnly, so a page-context reader (or a plain DevTools
// document.cookie) can't see it. This command shells out to a cookie reader
// that can: pycookiecheat (pip install; reads Chrome's encrypted cookie DB),
// then press-auth (the optional CDP companion). When neither is present it
// prints actionable manual + install guidance rather than failing opaquely.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/x-twitter/internal/cliutil"
)

// xWebBearer is x.com's public web-app bearer, embedded in the site's JS and
// identical for every visitor (it identifies the web client, not the user).
// X Articles' GraphQL endpoints require it alongside the session cookies.
const xWebBearer = "AAAAAAAAAAAAAAAAAAAAANRILgAAAAAAnNwIzUejRCOuH5E6I8xnZz4puTs%3D1Zv7ttfk8LF81IUq16cHjhLTvJu4FA33AGWWjCpTnA"

const xCookieDomain = "x.com"

func xCookieFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "x-twitter-pp-cli", "cookies.json"), nil
}

func newXAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var chrome bool
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Capture x.com session cookies for X Articles (use --chrome)",
		Long: "Capture your logged-in x.com session cookies for X Articles authoring.\n\n" +
			"X Articles has no v2 API — its editor runs on x.com GraphQL and needs the\n" +
			"same auth_token + ct0 session cookies your browser uses. The v2 API itself\n" +
			"keeps using X_BEARER_TOKEN (reads) and X_OAUTH2_USER_TOKEN (writes); this\n" +
			"only sets up the cookie session the `articles ...` commands read.\n\n" +
			"auth_token is an httpOnly cookie, so this shells out to a cookie reader that\n" +
			"can see it: pycookiecheat (recommended: pip install pycookiecheat), or the\n" +
			"press-auth companion. Make sure you're logged into x.com in Chrome first.",
		Example: "  x-twitter-pp-cli auth login --chrome",
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()
			if !chrome {
				fmt.Fprintln(w, "X Articles need your logged-in x.com session cookies. Run:")
				fmt.Fprintln(w, "  x-twitter-pp-cli auth login --chrome")
				fmt.Fprintln(w, "")
				fmt.Fprintln(w, "(v2 API reads use X_BEARER_TOKEN and writes use X_OAUTH2_USER_TOKEN — see `doctor`.)")
				return nil
			}

			// Side-effect guard: shells out to cookie/browser tools that touch
			// Chrome. Never run that under the verify mock matrix.
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(w, "PRINTING_PRESS_VERIFY=1; skipping cookie capture.")
				return nil
			}

			authToken, ct0, source, err := captureXSessionCookies(w)
			if err != nil {
				return err
			}

			path, err := xCookieFilePath()
			if err != nil {
				return fmt.Errorf("resolving cookie path: %w", err)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				return fmt.Errorf("creating config dir: %w", err)
			}
			doc := map[string]string{
				"auth_token":  authToken,
				"ct0":         ct0,
				"web_bearer":  xWebBearer,
				"captured_at": time.Now().UTC().Format("2006-01-02"),
			}
			blob, err := json.MarshalIndent(doc, "", "  ")
			if err != nil {
				return fmt.Errorf("encoding cookies: %w", err)
			}
			if err := os.WriteFile(path, append(blob, '\n'), 0o600); err != nil {
				return fmt.Errorf("writing %s: %w", path, err)
			}
			fmt.Fprintf(w, "Captured x.com session cookies via %s.\n", source)
			fmt.Fprintf(w, "Saved to %s — the `articles` commands will use it.\n", path)
			fmt.Fprintln(w, "Refresh by re-running this command if X invalidates the session.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&chrome, "chrome", false, "Capture cookies from your logged-in Chrome session")
	cmd.Flags().BoolVar(&chrome, "browser", false, "Alias for --chrome")
	return cmd
}

// captureXSessionCookies tries cookie readers in order of how likely an
// installer is to have them: pycookiecheat first (the recommended pip install,
// reads Chrome's cookie DB including httpOnly cookies), then the press-auth
// companion. Returns actionable install + manual guidance when neither works.
func captureXSessionCookies(w io.Writer) (authToken, ct0, source string, err error) {
	if bin, lookErr := exec.LookPath("pycookiecheat"); lookErr == nil {
		at, c, ok := cookiesFromPycookiecheat(bin)
		if ok {
			return at, c, "pycookiecheat", nil
		}
		fmt.Fprintln(w, "pycookiecheat is installed but found no x.com session — are you logged into x.com in Chrome?")
	}
	if bin, lookErr := exec.LookPath("press-auth"); lookErr == nil {
		at, c, ok := cookiesFromPressAuth(bin)
		if ok {
			return at, c, "press-auth", nil
		}
		fmt.Fprintf(w, "press-auth is installed but has no captured x.com session. Run: press-auth login %s\n", xCookieDomain)
	}
	return "", "", "", fmt.Errorf("%s", xCookieManualGuidance())
}

// cookiesFromPycookiecheat runs `pycookiecheat https://x.com`, which prints a
// flat {cookie_name: value} JSON object read from Chrome's cookie DB.
func cookiesFromPycookiecheat(bin string) (authToken, ct0 string, ok bool) {
	out, err := exec.Command(bin, "https://"+xCookieDomain).Output()
	if err != nil {
		return "", "", false
	}
	var jar map[string]string
	if err := json.Unmarshal(out, &jar); err != nil {
		return "", "", false
	}
	authToken = jar["auth_token"]
	ct0 = jar["ct0"]
	return authToken, ct0, authToken != "" && ct0 != ""
}

// cookiesFromPressAuth runs `press-auth cookies x.com`, which prints a Cookie
// header line ("auth_token=...; ct0=...; ...") for the captured session.
func cookiesFromPressAuth(bin string) (authToken, ct0 string, ok bool) {
	out, err := exec.Command(bin, "cookies", xCookieDomain).Output()
	if err != nil {
		return "", "", false
	}
	for _, pair := range strings.Split(strings.TrimSpace(string(out)), ";") {
		pair = strings.TrimSpace(pair)
		name, value, found := strings.Cut(pair, "=")
		if !found {
			continue
		}
		switch strings.TrimSpace(name) {
		case "auth_token":
			authToken = strings.TrimSpace(value)
		case "ct0":
			ct0 = strings.TrimSpace(value)
		}
	}
	return authToken, ct0, authToken != "" && ct0 != ""
}

func xCookieManualGuidance() string {
	path, _ := xCookieFilePath()
	return "no cookie reader available to capture your httpOnly x.com session.\n\n" +
		"Pick one:\n" +
		"  1. Install pycookiecheat (recommended), then re-run `auth login --chrome`:\n" +
		"       pip install pycookiecheat\n" +
		"  2. Install the press-auth companion, capture once, then re-run:\n" +
		"       go install github.com/mvanhorn/cli-printing-press/v4/cmd/press-auth@latest\n" +
		"       press-auth login " + xCookieDomain + "\n" +
		"  3. Manual: in Chrome (logged into x.com) open DevTools -> Application -> Cookies\n" +
		"     -> https://x.com, copy auth_token and ct0, then write:\n" +
		"       " + path + "\n" +
		"     {\"auth_token\":\"<auth_token>\",\"ct0\":\"<ct0>\",\"web_bearer\":\"" + xWebBearer + "\"}"
}
