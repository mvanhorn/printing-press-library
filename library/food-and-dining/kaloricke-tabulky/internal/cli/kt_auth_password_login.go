package cli

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/kaloricke-tabulky/internal/config"
)

// newAuthPasswordLoginCmd authenticates by POSTing email + MD5-hashed
// password to /login/create?format=json. The site does the hash itself
// on the client side in the AngularJS app, so this matches the public
// web behavior exactly. Adds a sibling to `auth login --chrome`.
func newAuthPasswordLoginCmd(flags *rootFlags) *cobra.Command {
	var email string
	var fromStdin bool

	cmd := &cobra.Command{
		Use:   "password-login",
		Short: "Authenticate with email + password (MD5-hashed locally)",
		Long: `Authenticate by posting your email and MD5-hashed password to the API.

This matches what the AngularJS web app does in your browser. Your
password is hashed with MD5 before sending; the plaintext never leaves
this process. The cookie session is saved to your config.

The password is never accepted as a command-line argument — that would
leak it into the process list (visible via ps / /proc/<pid>/cmdline).
Supply it interactively (the terminal prompt does not echo) or pipe
from stdin with --password-stdin.

Examples:
  # Interactive prompt (recommended):
  kaloricke-tabulky-pp-cli auth password-login --email <your-email>
  # Read from stdin:
  echo "$PASSWORD" | kaloricke-tabulky-pp-cli auth password-login --email <your-email> --password-stdin

For a no-password flow, use 'auth login --chrome' to import session
cookies directly from a logged-in Chrome profile.`,
		Annotations: map[string]string{
			"mcp:hidden": "true",
		},
		Example: "  kaloricke-tabulky-pp-cli auth password-login --email <your-email>",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if email == "" {
				return fmt.Errorf("--email is required")
			}
			var password string
			if fromStdin {
				b, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading password from stdin: %w", err)
				}
				password = strings.TrimRight(string(b), "\r\n")
			} else {
				if !term.IsTerminal(int(syscall.Stdin)) {
					return fmt.Errorf("no password supplied: pass --password-stdin, or run interactively")
				}
				fmt.Fprint(cmd.OutOrStderr(), "Password: ")
				pw, err := term.ReadPassword(int(syscall.Stdin))
				fmt.Fprintln(cmd.OutOrStderr())
				if err != nil {
					return fmt.Errorf("reading password: %w", err)
				}
				password = strings.TrimRight(string(pw), "\r\n")
			}
			if password == "" {
				return fmt.Errorf("password is empty")
			}

			cookieHeader, err := ktDoPasswordLogin(email, password)
			if err != nil {
				return err
			}

			cfg, err := config.Load("")
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			cfg.AuthHeaderVal = cookieHeader
			// Clear any stale OAuth/bearer fields that might have been
			// left by a prior `auth login --chrome` from an older build
			// of this CLI (commits before ca05f4a4 wrote cookies into
			// AccessToken). With AccessToken non-empty, AuthHeader()'s
			// fallback path would prepend "Bearer " to the cookies on
			// future calls — except AuthHeaderVal wins first; the real
			// risk is the doctor surface showing "browser session" and
			// confusing operators. Clearing is safe: anyone who needs
			// the OAuth path re-runs the appropriate auth command.
			cfg.AccessToken = ""
			cfg.RefreshToken = ""
			cfg.TokenExpiry = time.Time{}
			cfg.AuthSource = "config"
			if err := ktSaveConfig(cfg); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Authenticated as %s. Cookies saved to %s.\n", email, cfg.Path)
			return nil
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "Account email")
	cmd.Flags().BoolVar(&fromStdin, "password-stdin", false, "Read password from stdin")
	return cmd
}

// ktDoPasswordLogin POSTs the MD5 login and returns the
// Cookie-header-shaped string ("name1=value1; name2=value2").
func ktDoPasswordLogin(email, password string) (string, error) {
	pwSum := md5.Sum([]byte(password))
	body := map[string]string{
		"email":    email,
		"password": hex.EncodeToString(pwSum[:]),
	}
	bs, _ := json.Marshal(body)

	req, err := http.NewRequest("POST", "https://www.kaloricketabulky.cz/login/create?format=json", bytes.NewReader(bs))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "kaloricke-tabulky-pp-cli/1.0")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("posting login: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("login HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	var env struct {
		Code    int         `json:"code"`
		Message string      `json:"message"`
		Data    interface{} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &env); err != nil {
		return "", fmt.Errorf("parsing login response: %w (body: %.200s)", err, respBody)
	}
	if env.Code != 0 {
		msg := env.Message
		if msg == "" {
			msg = "login failed (check email/password)"
		}
		return "", fmt.Errorf("login rejected: %s", msg)
	}

	// Collect Set-Cookie headers into a single Cookie-header value.
	parts := make([]string, 0, 2)
	for _, c := range resp.Cookies() {
		if c.Name != "JSESSIONID" && c.Name != "kaloricketabulky_token" {
			continue
		}
		parts = append(parts, c.Name+"="+c.Value)
	}
	cookieHeader := strings.Join(parts, "; ")
	if cookieHeader == "" {
		return "", fmt.Errorf("login succeeded but no session cookies returned (expected JSESSIONID + kaloricketabulky_token)")
	}
	return cookieHeader, nil
}

// ktSaveConfig writes the TOML config to disk. The generator's
// `config.save()` is private, and `SaveTokens` clobbers AuthHeaderVal,
// so cookie-auth CLIs need a sibling write path. This mirrors
// `config.save()`'s behavior 1:1.
func ktSaveConfig(cfg *config.Config) error {
	dir := filepath.Dir(cfg.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	return os.WriteFile(cfg.Path, data, 0o600)
}
