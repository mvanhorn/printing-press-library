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
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
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
	var profile string
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
			"press-auth companion. Make sure you're logged into x.com in Chrome first.\n\n" +
			"If you use multiple Chrome profiles, this command checks each profile for\n" +
			"auth_token + ct0. Use --profile to force a specific Chrome profile.",
		Example: "  x-twitter-pp-cli auth login --chrome\n" +
			"  x-twitter-pp-cli auth login --chrome --profile \"Profile 1\"",
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

			authToken, ct0, userID, source, err := captureXSessionCookies(w, profile)
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
				"user_id":     userID,
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
	cmd.Flags().StringVar(&profile, "profile", "", "Chrome profile name or directory (e.g. \"Default\", \"Profile 1\", \"Work\")")
	return cmd
}

// captureXSessionCookies tries cookie readers in order of how likely an
// installer is to have them: pycookiecheat first (the recommended pip install,
// reads Chrome's cookie DB including httpOnly cookies), then the press-auth
// companion. Returns actionable install + manual guidance when neither works.
func captureXSessionCookies(w io.Writer, profile string) (authToken, ct0, userID, source string, err error) {
	if bin, lookErr := exec.LookPath("pycookiecheat"); lookErr == nil {
		profiles, profileErr := chromeCookieProfiles(profile)
		if profileErr != nil {
			if profile != "" {
				return "", "", "", "", fmt.Errorf("resolving Chrome profile: %w", profileErr)
			}
			fmt.Fprintf(w, "Could not inspect Chrome profiles (%v); trying pycookiecheat default profile.\n", profileErr)
		}
		for _, candidate := range profiles {
			at, c, uid, ok := cookiesFromPycookiecheat(bin, candidate.CookiePath)
			if ok {
				return at, c, uid, "pycookiecheat (" + candidate.DisplayName + ")", nil
			}
		}
		if len(profiles) == 0 && profile == "" {
			at, c, uid, ok := cookiesFromPycookiecheat(bin, "")
			if ok {
				return at, c, uid, "pycookiecheat", nil
			}
		}
		if profile != "" {
			fmt.Fprintf(w, "pycookiecheat is installed but found no x.com session in Chrome profile %q.\n", profile)
		} else {
			fmt.Fprintln(w, "pycookiecheat is installed but found no x.com session in any Chrome profile — are you logged into x.com in Chrome?")
		}
	}
	if bin, lookErr := exec.LookPath("press-auth"); lookErr == nil {
		at, c, uid, ok := cookiesFromPressAuth(bin)
		if ok {
			return at, c, uid, "press-auth", nil
		}
		fmt.Fprintf(w, "press-auth is installed but has no captured x.com session. Run: press-auth login %s\n", xCookieDomain)
	}
	return "", "", "", "", fmt.Errorf("%s", xCookieManualGuidance())
}

// cookiesFromPycookiecheat runs `pycookiecheat https://x.com`, which prints a
// flat {cookie_name: value} JSON object read from Chrome's cookie DB.
func cookiesFromPycookiecheat(bin, cookiePath string) (authToken, ct0, userID string, ok bool) {
	args := []string{"https://" + xCookieDomain}
	if cookiePath != "" {
		args = []string{"-c", cookiePath, "https://" + xCookieDomain}
	}
	out, err := exec.Command(bin, args...).Output()
	if err != nil {
		return "", "", "", false
	}
	var jar map[string]string
	if err := json.Unmarshal(out, &jar); err != nil {
		return "", "", "", false
	}
	authToken = jar["auth_token"]
	ct0 = jar["ct0"]
	userID = xUserIDFromTWID(jar["twid"])
	return authToken, ct0, userID, authToken != "" && ct0 != ""
}

type xChromeCookieProfile struct {
	Dir         string
	DisplayName string
	CookiePath  string
}

func chromeCookieProfiles(profile string) ([]xChromeCookieProfile, error) {
	dataDir, err := chromeUserDataDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return nil, fmt.Errorf("cannot read Chrome data directory: %w", err)
	}

	var profiles []xChromeCookieProfile
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := entry.Name()
		if dir != "Default" && !strings.HasPrefix(dir, "Profile ") {
			continue
		}
		cookiePath := filepath.Join(dataDir, dir, "Cookies")
		if _, err := os.Stat(cookiePath); err != nil {
			continue
		}
		displayName := readChromeProfileDisplayName(filepath.Join(dataDir, dir, "Preferences"))
		if displayName == "" {
			displayName = dir
		}
		profiles = append(profiles, xChromeCookieProfile{
			Dir:         dir,
			DisplayName: displayName,
			CookiePath:  cookiePath,
		})
	}

	sort.Slice(profiles, func(i, j int) bool {
		if profiles[i].Dir == "Default" {
			return true
		}
		if profiles[j].Dir == "Default" {
			return false
		}
		return profiles[i].Dir < profiles[j].Dir
	})

	if profile == "" {
		return profiles, nil
	}
	lower := strings.ToLower(profile)
	for _, candidate := range profiles {
		if strings.ToLower(candidate.Dir) == lower || strings.ToLower(candidate.DisplayName) == lower {
			return []xChromeCookieProfile{candidate}, nil
		}
	}
	return nil, fmt.Errorf("Chrome profile %q not found", profile)
}

func chromeUserDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Google", "Chrome"), nil
	case "linux":
		return filepath.Join(home, ".config", "google-chrome"), nil
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(localAppData, "Google", "Chrome", "User Data"), nil
	default:
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func readChromeProfileDisplayName(prefsPath string) string {
	data, err := os.ReadFile(prefsPath)
	if err != nil {
		return ""
	}
	var prefs struct {
		Profile struct {
			Name string `json:"name"`
		} `json:"profile"`
	}
	if err := json.Unmarshal(data, &prefs); err != nil {
		return ""
	}
	return prefs.Profile.Name
}

// cookiesFromPressAuth runs `press-auth cookies x.com`, which prints a Cookie
// header line ("auth_token=...; ct0=...; ...") for the captured session.
func cookiesFromPressAuth(bin string) (authToken, ct0, userID string, ok bool) {
	out, err := exec.Command(bin, "cookies", xCookieDomain).Output()
	if err != nil {
		return "", "", "", false
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
		case "twid":
			userID = xUserIDFromTWID(strings.TrimSpace(value))
		}
	}
	return authToken, ct0, userID, authToken != "" && ct0 != ""
}

func xUserIDFromTWID(twid string) string {
	twid = strings.TrimSpace(twid)
	if twid == "" {
		return ""
	}
	if decoded, err := url.QueryUnescape(twid); err == nil {
		twid = decoded
	}
	twid = strings.TrimPrefix(twid, "\"")
	twid = strings.TrimSuffix(twid, "\"")
	twid = strings.TrimPrefix(twid, "u%3D")
	twid = strings.TrimPrefix(twid, "u=")
	for _, r := range twid {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return twid
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
		"     -> https://x.com, copy auth_token, ct0, and twid (u=<user id>), then write:\n" +
		"       " + path + "\n" +
		"     {\"auth_token\":\"<auth_token>\",\"ct0\":\"<ct0>\",\"user_id\":\"<twid user id>\",\"web_bearer\":\"" + xWebBearer + "\"}"
}
