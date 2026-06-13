// Chrome cookie import for SEMrush UI-mode commands. Reads SEMrush session
// cookies from the user's Chrome profile (via kooky, which handles macOS
// Keychain decryption automatically) and persists them to
// ~/.config/semrush-pp-cli/cookies.json for replay by UI-mode commands.
//
// On macOS, the first read triggers a one-time Keychain access prompt.
// The persisted file is mode 0600 (owner-only).
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/browserutils/kooky"
	"github.com/browserutils/kooky/browser/chrome"
	"github.com/spf13/cobra"
)

// SemrushCookie is the persisted shape — only the fields we need to replay.
type SemrushCookie struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Domain   string    `json:"domain"`
	Path     string    `json:"path"`
	Expires  time.Time `json:"expires,omitempty"`
	Secure   bool      `json:"secure"`
	HTTPOnly bool      `json:"http_only"`
}

// CookieJar is the persisted file shape.
type CookieJar struct {
	CapturedAt time.Time       `json:"captured_at"`
	Source     string          `json:"source"`
	Cookies    []SemrushCookie `json:"cookies"`
}

func newAuthLoginCmd(_ *rootFlags) *cobra.Command {
	var fromChrome bool
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Import SEMrush session cookies from your browser (use --chrome)",
		Long: "Import your logged-in SEMrush session cookies for UI-mode commands " +
			"like 'keyword-magic' that hit endpoints requiring a browser session " +
			"(not just the API key). On macOS, the first run prompts for Keychain " +
			"access to read Chrome's encrypted cookie store.",
		Example: strings.Trim(`
  semrush-pp-cli auth login --chrome
  semrush-pp-cli auth status
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !fromChrome {
				return fmt.Errorf("specify a source: --chrome (only supported source today)")
			}
			n, path, err := importSemrushCookiesFromChrome(cmd.Context())
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Imported %d SEMrush cookies from Chrome → %s\n", n, path)
			fmt.Fprintln(cmd.OutOrStdout(), "UI-mode commands (e.g. 'keyword-magic') can now use these cookies.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&fromChrome, "chrome", false, "Import SEMrush cookies from your Google Chrome profile")
	return cmd
}

// importSemrushCookiesFromChrome reads valid SEMrush cookies from a Chrome
// cookie store and writes them to the cookie jar file. Tries known Chrome
// macOS paths in order — the layout has changed across versions (some have
// the file at Default/Cookies, newer versions at Default/Network/Cookies).
func importSemrushCookiesFromChrome(ctx context.Context) (int, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return 0, "", err
	}
	chromeRoot := filepath.Join(home, "Library", "Application Support", "Google", "Chrome")
	// Order: prefer Default profile (most-used), then numbered profiles.
	// For each profile, try the modern "Network/Cookies" layout first, then
	// the older "Cookies" path.
	profiles := []string{"Default", "Profile 1", "Profile 2", "Profile 3"}
	layouts := []string{filepath.Join("Network", "Cookies"), "Cookies"}

	// Session cookies SEMrush needs (HTTPOnly, set by server) — used to pick
	// the right profile when the user has multiple Chrome profiles.
	sessionMarkers := map[string]bool{
		"PHPSESSID":      true,
		"SSO-JWT":        true,
		"GCLB":           true,
		"_sm_bot":        true,
		"_sm_bot_verify": true,
	}

	// Scan ALL profiles + layouts, accumulate cookies, then pick the profile
	// with the most session-shaped cookies. Falls back to the profile with
	// the most cookies overall if no markers are found anywhere.
	type profileHit struct {
		path     string
		cookies  []*kooky.Cookie
		markers  int
	}
	var hits []profileHit
	var lastErr error
	for _, prof := range profiles {
		for _, layout := range layouts {
			path := filepath.Join(chromeRoot, prof, layout)
			if _, statErr := os.Stat(path); statErr != nil {
				continue
			}
			cs, readErr := chrome.ReadCookies(ctx, path,
				kooky.Valid,
				kooky.DomainHasSuffix("semrush.com"),
			)
			if readErr != nil {
				lastErr = fmt.Errorf("reading %s: %w", path, readErr)
				continue
			}
			if len(cs) == 0 {
				continue
			}
			markers := 0
			for _, c := range cs {
				if sessionMarkers[c.Name] {
					markers++
				}
			}
			hits = append(hits, profileHit{path: path, cookies: cs, markers: markers})
		}
	}
	if len(hits) == 0 {
		if lastErr != nil {
			return 0, "", fmt.Errorf("no semrush.com cookies found in any Chrome profile — are you logged into SEMrush? Last error: %w (tip: on macOS, allow Keychain access for the binary when the popup appears)", lastErr)
		}
		return 0, "", fmt.Errorf("no Chrome cookie file found at any expected path under %s", chromeRoot)
	}
	// Pick profile with most session markers; tiebreak by total cookie count
	best := hits[0]
	for _, h := range hits[1:] {
		if h.markers > best.markers || (h.markers == best.markers && len(h.cookies) > len(best.cookies)) {
			best = h
		}
	}
	if best.markers == 0 {
		fmt.Fprintf(os.Stderr, "warning: no session-shape cookies (PHPSESSID, SSO-JWT) found in any Chrome profile — your session may be expired or you may need to log in.\n")
	}
	cookies := best.cookies
	_ = best.path

	jar := CookieJar{
		CapturedAt: time.Now().UTC(),
		Source:     "chrome",
		Cookies:    make([]SemrushCookie, 0, len(cookies)),
	}
	for _, c := range cookies {
		jar.Cookies = append(jar.Cookies, SemrushCookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  c.Expires,
			Secure:   c.Secure,
			HTTPOnly: c.HttpOnly,
		})
	}

	path, err := semrushCookieJarPath()
	if err != nil {
		return 0, "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return 0, "", err
	}
	data, err := json.MarshalIndent(jar, "", "  ")
	if err != nil {
		return 0, "", err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return 0, "", err
	}
	return len(jar.Cookies), path, nil
}

// loadSemrushCookieJar reads the persisted cookie jar, or returns an error
// suggesting the user run 'auth login --chrome'.
func loadSemrushCookieJar() (*CookieJar, error) {
	path, err := semrushCookieJarPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no SEMrush cookies imported yet — run 'semrush-pp-cli auth login --chrome' first")
		}
		return nil, err
	}
	var jar CookieJar
	if err := json.Unmarshal(data, &jar); err != nil {
		return nil, fmt.Errorf("parsing cookie jar: %w", err)
	}
	return &jar, nil
}

// applyCookiesToRequest sets the Cookie header from the jar for any cookie
// that matches the request URL's host.
func (j *CookieJar) applyCookiesToRequest(req *http.Request) {
	host := req.URL.Host
	now := time.Now()
	var parts []string
	for _, c := range j.Cookies {
		// Domain match (kooky stores cookies with leading dot for ETLD+1)
		d := strings.TrimPrefix(c.Domain, ".")
		if !(host == d || strings.HasSuffix(host, "."+d)) {
			continue
		}
		// Expiry
		if !c.Expires.IsZero() && c.Expires.Before(now) {
			continue
		}
		// Semicolons in a cookie value would split the Cookie header field.
		// Skip any cookie whose value contains the delimiter to avoid
		// silently mangling the header (rare but possible for base64/JWT
		// session tokens stored by Chrome).
		if strings.ContainsAny(c.Value, ";") {
			continue
		}
		parts = append(parts, c.Name+"="+c.Value)
	}
	if len(parts) > 0 {
		req.Header.Set("Cookie", strings.Join(parts, "; "))
	}
}

func semrushCookieJarPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "semrush-pp-cli", "cookies.json"), nil
}
