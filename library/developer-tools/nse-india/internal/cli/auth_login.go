// Copyright 2026 mayank-lavania. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/nse-india/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/nse-india/internal/config"
	// config used via flags.configPath; cliutil for IsVerifyEnv

	"github.com/spf13/cobra"
)

func newAuthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage NSE India session authentication",
		Annotations: map[string]string{
			"mcp:hidden": "true",
		},
	}
	cmd.AddCommand(newAuthLoginCmd(flags))
	cmd.AddCommand(newAuthStatusCmd(flags))
	cmd.AddCommand(newAuthLogoutCmd(flags))
	return cmd
}

func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var flagChrome bool
	var flagCookie string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Store NSE browser session cookie for cookie-gated endpoints (options chain, index constituents)",
		Long: `Store a browser session cookie so nse-india-pp-cli can reach NSE India endpoints
that require a real browser session (options chain, equity-stockIndices, etc.).

Two modes:
  --chrome    Extract cookies automatically from Chrome via press-auth (if installed)
              or pycookiecheat (fallback). Prints the captured cookie value.
  --cookie    Paste a cookie string directly (e.g. copied from browser DevTools).
              Format: "bm_sv=...; nsit=...; nseappid=..."

The cookie is stored in ~/.config/nse-india-pp-cli/config.json and injected into
every subsequent request automatically.`,
		Example: "  nse-india-pp-cli auth login --chrome\n" +
			"  nse-india-pp-cli auth login --cookie \"bm_sv=ABC...; nsit=XYZ...\"",
		Annotations: map[string]string{
			"mcp:hidden": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Side-effect gate: short-circuit in verify mode
			if cliutil.IsVerifyEnv() {
				if flags.asJSON {
					return printOutput(cmd.OutOrStdout(), json.RawMessage(`{"verify_noop":true,"success":false}`), true)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "verify mode: auth login skipped")
				return nil
			}

			if !flagChrome && flagCookie == "" {
				return fmt.Errorf("specify --chrome to extract from browser, or --cookie <value> to paste a cookie string")
			}

			var cookieStr string

			if flagCookie != "" {
				cookieStr = strings.TrimSpace(flagCookie)
			} else {
				// --chrome: try press-auth first, then pycookiecheat
				var err error
				cookieStr, err = extractChromeCookie()
				if err != nil {
					return fmt.Errorf("cookie extraction failed: %w\n\nFallback: copy cookies from Chrome DevTools (Network tab → nseindia.com request → Cookie header) and use:\n  nse-india-pp-cli auth login --cookie \"<paste here>\"", err)
				}
			}

			if cookieStr == "" {
				return fmt.Errorf("extracted cookie is empty — try --cookie <value> instead")
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			if err := cfg.SaveBrowserCookie(cookieStr); err != nil {
				return fmt.Errorf("saving cookie: %w", err)
			}

			// Print status, not the raw cookie value
			preview := cookieStr
			if len(preview) > 40 {
				preview = preview[:40] + "..."
			}

			if flags.asJSON {
				result := map[string]any{
					"success":        true,
					"cookie_preview": preview,
					"config_path":    cfg.Path,
				}
				b, _ := json.Marshal(result)
				return printOutput(cmd.OutOrStdout(), json.RawMessage(b), true)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Cookie saved: %s\nConfig: %s\n\nVerify with: nse-india-pp-cli auth status\n", preview, cfg.Path)
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagChrome, "chrome", false, "Extract session cookie from Chrome automatically (requires press-auth or pycookiecheat)")
	cmd.Flags().StringVar(&flagCookie, "cookie", "", "Paste cookie string directly (e.g. \"bm_sv=...; nsit=...\")")
	return cmd
}

func newAuthStatusCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show whether a browser session cookie is configured",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			if cfg.BrowserCookie == "" {
				if flags.asJSON {
					return printOutput(cmd.OutOrStdout(), json.RawMessage(`{"authenticated":false,"message":"No browser cookie configured. Run: nse-india-pp-cli auth login --chrome"}`), true)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Not authenticated. Run: nse-india-pp-cli auth login --chrome")
				return nil
			}
			preview := cfg.BrowserCookie
			if len(preview) > 40 {
				preview = preview[:40] + "..."
			}
			if flags.asJSON {
				result := map[string]any{
					"authenticated":  true,
					"cookie_preview": preview,
					"config_path":    cfg.Path,
				}
				b, _ := json.Marshal(result)
				return printOutput(cmd.OutOrStdout(), json.RawMessage(b), true)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Authenticated (cookie: %s)\nConfig: %s\n", preview, cfg.Path)
			return nil
		},
	}
}

func newAuthLogoutCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove the stored browser session cookie",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				if flags.asJSON {
					return printOutput(cmd.OutOrStdout(), json.RawMessage(`{"verify_noop":true,"success":false}`), true)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "verify mode: auth logout skipped")
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			if err := cfg.ClearBrowserCookie(); err != nil {
				return fmt.Errorf("clearing cookie: %w", err)
			}
			if flags.asJSON {
				return printOutput(cmd.OutOrStdout(), json.RawMessage(`{"success":true}`), true)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Browser cookie cleared.")
			return nil
		},
	}
}

// extractChromeCookie tries press-auth first, then pycookiecheat as fallback.
func extractChromeCookie() (string, error) {
	// Try press-auth companion
	if path, err := exec.LookPath("press-auth"); err == nil {
		out, err := exec.Command(path, "extract", "--domain", "nseindia.com", "--format", "header").Output() // #nosec G204 -- path resolved via exec.LookPath; arguments are hardcoded constants
		if err == nil && len(out) > 0 {
			return strings.TrimSpace(string(out)), nil
		}
	}

	// Fallback: pycookiecheat (Python)
	script := `
import sys
try:
    from pycookiecheat import chrome_cookies
    cookies = chrome_cookies("https://www.nseindia.com/")
    parts = [f"{k}={v}" for k, v in cookies.items()]
    print("; ".join(parts))
except ImportError:
    print("ERROR: pycookiecheat not installed. Run: pip install pycookiecheat", file=sys.stderr)
    sys.exit(1)
except Exception as e:
    print(f"ERROR: {e}", file=sys.stderr)
    sys.exit(1)
`
	python := "python3"
	if _, err := exec.LookPath(python); err != nil {
		python = "python"
	}
	if _, err := exec.LookPath(python); err != nil {
		return "", fmt.Errorf("press-auth not found and Python not available for pycookiecheat fallback")
	}

	cmd := exec.Command(python, "-c", script)
	out, err := cmd.Output()
	if err != nil {
		stderr := ""
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = strings.TrimSpace(string(exitErr.Stderr))
		}
		if strings.Contains(stderr, "pycookiecheat not installed") {
			return "", fmt.Errorf("press-auth not found and pycookiecheat not installed\n\nInstall press-auth: https://github.com/mvanhorn/press-auth\nOr install pycookiecheat: pip install pycookiecheat\nOr paste cookie manually: nse-india-pp-cli auth login --cookie \"<value>\"")
		}
		return "", fmt.Errorf("%s", stderr)
	}

	result := strings.TrimSpace(string(out))
	if strings.HasPrefix(result, "ERROR:") {
		return "", fmt.Errorf("%s", result)
	}

	// Ensure Chrome is not already open with a lock (macOS)
	if strings.Contains(result, "database is locked") {
		fmt.Fprintf(os.Stderr, "hint: close Chrome and retry, or use --cookie to paste manually\n")
	}

	return result, nil
}
