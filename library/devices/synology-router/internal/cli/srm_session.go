// Copyright 2026 eric-jung. Licensed under Apache-2.0. See LICENSE.
// SRM session management — handles login/logout with the Synology Router Manager API.
// Extends the generated session commands with SRM-specific credential persistence.

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/devices/synology-router/internal/client"
	"github.com/mvanhorn/printing-press-library/library/devices/synology-router/internal/config"
)

// newSRMLoginCmd creates the 'login' command that authenticates with SRM and
// saves the session ID to the config file for subsequent requests.
func newSRMLoginCmd(flags *rootFlags) *cobra.Command {
	var account string
	var passwd string
	var save bool

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with the Synology Router and save the session ID",
		Long: `Authenticate with the Synology Router Manager (SRM) API and save
the session ID for subsequent requests.

The session ID is stored in ~/.config/github.com/mvanhorn/printing-press-library/library/devices/synology-router/config.toml
under router_cookie_auth. It can also be set via the SYNOLOGY_ROUTER_COOKIE_AUTH
environment variable.

Note: SRM sessions expire after inactivity. Run 'login' again if you get
authentication errors.`,
		Example: `  # Interactive login (prompts for password)
  github.com/mvanhorn/printing-press-library/library/devices/synology-router login --account admin

  # Non-interactive via stdin (recommended for scripts)
  echo "$PASSWORD" | github.com/mvanhorn/printing-press-library/library/devices/synology-router login --account admin

  # Login without saving (just print the sid)
  echo "$PASSWORD" | github.com/mvanhorn/printing-press-library/library/devices/synology-router login --account admin --no-save`,
		Annotations: map[string]string{
			"pp:endpoint": "session.auth-login",
			"pp:method":   "POST",
			"pp:path":     "/session/login",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if account == "" {
				return usageErr(fmt.Errorf("--account is required"))
			}
			if passwd == "" && !flags.dryRun {
				// Try to read password from stdin if not a tty
				if !isTerminal(os.Stdin) {
					buf := make([]byte, 256)
					n, _ := os.Stdin.Read(buf)
					passwd = strings.TrimSpace(string(buf[:n]))
				}
				if passwd == "" {
					return usageErr(fmt.Errorf("--passwd is required (or pipe password to stdin)"))
				}
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}

			c := client.New(cfg, flags.timeout, flags.rateLimit)
			c.DryRun = flags.dryRun
			// Install SRM transport WITHOUT a sid for login (no-auth route)
			client.NewSRMClient(c, "")

			// Call the SRM login endpoint via the transport
			// The transport will convert this to POST /webapi/auth.cgi
			formFields := url.Values{}
			formFields.Set("account", account)
			formFields.Set("passwd", passwd)
			formFields.Set("format", "sid")
			response, _, err := c.PostFormWithParams("/session/login", nil, formFields)
			if err != nil {
				return authErr(fmt.Errorf("login failed: %w", err))
			}

			if flags.dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), "Dry run: would POST to SRM auth endpoint")
				return nil
			}

			// Parse the SRM response to extract the session ID
			var loginResp struct {
				Success bool `json:"success"`
				Data    struct {
					SID string `json:"sid"`
				} `json:"data"`
				Error struct {
					Code int `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(response, &loginResp); err != nil {
				return fmt.Errorf("parsing login response: %w", err)
			}
			if !loginResp.Success {
				code := loginResp.Error.Code
				switch code {
				case 400:
					return authErr(fmt.Errorf("login failed: no such account or incorrect password (code 400)"))
				case 401:
					return authErr(fmt.Errorf("login failed: account disabled (code 401)"))
				case 402:
					return authErr(fmt.Errorf("login failed: permission denied (code 402)"))
				case 403:
					return authErr(fmt.Errorf("login failed: 2-step verification required (code 403)"))
				default:
					return authErr(fmt.Errorf("login failed with SRM error code %d", code))
				}
			}

			sid := loginResp.Data.SID
			if sid == "" {
				return fmt.Errorf("login succeeded but no session ID returned")
			}

			if flags.asJSON {
				result := map[string]any{
					"success": true,
					"sid":     sid,
					"saved":   save,
				}
				b, _ := json.Marshal(result)
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Login successful (sid: %s...)\n", sid[:min(8, len(sid))])
			}

			if save {
				if err := saveSIDToConfig(flags.configPath, sid); err != nil {
					fmt.Fprintf(os.Stderr, "warning: could not save session to config: %v\n", err)
					fmt.Fprintf(os.Stderr, "Set manually: SYNOLOGY_ROUTER_COOKIE_AUTH=%s\n", sid)
				} else {
					fmt.Fprintf(os.Stderr, "Session ID saved to config.\n")
				}
			} else {
				fmt.Fprintf(os.Stderr, "To persist: github.com/mvanhorn/printing-press-library/library/devices/synology-router auth set-token <sid>\n")
				fmt.Fprintf(os.Stderr, "Or export: SYNOLOGY_ROUTER_COOKIE_AUTH=<sid>\n")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&account, "account", "", "Admin account username (required)")
	cmd.Flags().StringVar(&passwd, "passwd", "", "Admin account password")
	cmd.Flags().BoolVar(&save, "save", true, "Save session ID to config file (default true)")

	return cmd
}

// saveSIDToConfig writes the session ID to the config file's router_cookie_auth field.
func saveSIDToConfig(configPath, sid string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	// Update the session field
	cfg.SynologyRouterCookieAuth = sid

	// Write back to the config file
	cfgPath := cfg.Path
	if cfgPath == "" {
		home, _ := os.UserHomeDir()
		cfgPath = filepath.Join(home, ".config", "github.com/mvanhorn/printing-press-library/library/devices/synology-router", "config.toml")
	}

	// Ensure config directory exists
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	// Read existing config to preserve other settings
	var existing []byte
	existing, _ = os.ReadFile(cfgPath)

	// Update or add the router_cookie_auth line
	newContent := updateTOMLField(string(existing), "router_cookie_auth", sid)

	return os.WriteFile(cfgPath, []byte(newContent), 0o600)
}

// updateTOMLField updates or adds a simple key = "value" line in TOML content.
func updateTOMLField(content, key, value string) string {
	quoted := strconv.Quote(value)
	lines := strings.Split(content, "\n")
	prefix := key + " ="
	updated := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			lines[i] = key + " = " + quoted
			updated = true
			break
		}
	}
	if !updated {
		lines = append(lines, key+" = "+quoted)
	}
	return strings.Join(lines, "\n")
}

// min returns the smaller of a and b.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

