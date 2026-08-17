// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/notebooklm/internal/config"
	"github.com/spf13/cobra"
)

func newAuthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage Google session authentication",
	}
	var chrome bool
	var cookiesFile string
	login := &cobra.Command{
		Use:   "login",
		Short: "Import Google session cookies from Chrome or a file",
		Example: `  notebooklm-pp-cli auth login --chrome
  notebooklm-pp-cli auth login --cookies-file ./cookies.txt`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				if flags.asJSON {
					return printJSON(map[string]any{"authenticated": true, "dry_run": true})
				}
				dryRunMessage("import Google session cookies")
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			var header string
			switch {
			case cookiesFile != "":
				data, err := os.ReadFile(cookiesFile) // #nosec G304 -- user-supplied cookie import path
				if err != nil {
					return err
				}
				header = strings.TrimSpace(string(data))
			case chrome:
				header, err = extractChromeCookies(".google.com")
				if err != nil {
					return err
				}
			default:
				return fmt.Errorf("use --chrome or --cookies-file")
			}
			cfg.AuthHeaderVal = header
			if err := cfg.Save(); err != nil {
				return err
			}
			fmt.Fprintln(os.Stderr, "Saved Google session cookies to", cfg.Path)
			return nil
		},
	}
	login.Flags().BoolVar(&chrome, "chrome", false, "Read cookies from Chrome for .google.com")
	login.Flags().StringVar(&cookiesFile, "cookies-file", "", "Import raw Cookie header or storage state")
	cmd.AddCommand(login)

	status := &cobra.Command{
		Use:     "status",
		Short:   "Show whether Google session cookies are stored locally",
		Example: `  notebooklm-pp-cli auth status --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			authenticated := cfg.AuthHeader() != ""
			if flags.asJSON {
				out := map[string]any{
					"authenticated": authenticated,
					"config_path":   cfg.Path,
				}
				if !authenticated {
					out["remediation"] = "notebooklm-pp-cli auth login --chrome"
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			if authenticated {
				fmt.Println("authenticated: true")
				fmt.Println("config:", cfg.Path)
			} else {
				fmt.Println("authenticated: false")
				fmt.Println("run: notebooklm-pp-cli auth login --chrome")
			}
			return nil
		},
	}
	cmd.AddCommand(status)
	return cmd
}

func extractChromeCookies(domain string) (string, error) {
	backends := []func(string) (string, error){
		extractViaPycookiecheat,
		extractViaBrowserCookies,
	}
	var lastErr error
	for _, fn := range backends {
		header, err := fn(domain)
		if err == nil && header != "" {
			return header, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("no cookies found for %s — log in to Google in Chrome first", domain)
}

func extractViaPycookiecheat(domain string) (string, error) {
	script := `
import json, sys
try:
    from pycookiecheat import chrome_cookies
except ImportError:
    sys.exit(2)
for url in ("https://notebooklm.google.com", "https://google.com"):
    cookies = chrome_cookies(url)
    if cookies:
        print('; '.join(f'{k}={v}' for k,v in cookies.items()))
        sys.exit(0)
sys.exit(1)
`
	out, err := exec.Command("python3", "-c", script).Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 2 {
			return "", fmt.Errorf("pycookiecheat not installed; pip install pycookiecheat or use --cookies-file")
		}
		return "", fmt.Errorf("chrome cookie extraction failed: %w", err)
	}
	header := strings.TrimSpace(string(out))
	if header == "" {
		return "", fmt.Errorf("no cookies found for %s — log in to Google in Chrome first", domain)
	}
	return header, nil
}

func extractViaBrowserCookies(domain string) (string, error) {
	// #nosec G204 -- fixed argv; domain is validated cookie scope, not shell input
	out, err := exec.Command("browser-cookies", "chrome", "--domain", strings.TrimPrefix(domain, ".")).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
