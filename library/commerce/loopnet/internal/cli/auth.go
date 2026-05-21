// auth command — manages the Akamai clearance cookies the Surf transport
// replays. LoopNet's data pages sit behind Akamai Bot Manager, which serves
// a JS sensor page to plain HTTP clients. A real browser session mints
// cookies that, replayed over HTTP, return real content for a few hours.
//
// `auth set`     stores a Cookie header you paste from your browser.
// `auth refresh` drives the browser-use tool to mint fresh cookies for you.
// `auth status`  shows cookie age and live-tests whether they still work.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/loopnet/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/loopnet/internal/loopnet"
)

// mintURL is the LoopNet page auth refresh opens to mint clearance cookies.
const mintURL = "https://www.loopnet.com/search/office/los-angeles-ca/for-sale/"

func newAuthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage the LoopNet clearance cookies the CLI replays",
		Long: `LoopNet's data pages are protected by Akamai Bot Manager. The CLI fetches
them over plain HTTP by replaying clearance cookies from a real browser
session. Those cookies last a few hours; refresh them when fetches start
failing with a bot-challenge error.`,
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newAuthSetCmd(flags))
	cmd.AddCommand(newAuthRefreshCmd(flags))
	cmd.AddCommand(newAuthStatusCmd(flags))
	return cmd
}

// --- auth set ---------------------------------------------------------------

func newAuthSetCmd(flags *rootFlags) *cobra.Command {
	var cookies, cookiesFile string

	cmd := &cobra.Command{
		Use:   "set",
		Short: "Store a clearance-cookie header pasted from your browser",
		Long: `Store the LoopNet Cookie header from your browser.

In Chrome: open DevTools (F12) -> Network tab -> reload loopnet.com -> click
the document request -> copy the full 'Cookie:' request header value. Then:

  loopnet-pp-cli auth set --cookies "bm_sv=...; ak_bmsc=...; ..."`,
		Example: `  loopnet-pp-cli auth set --cookies "bm_sv=abc; ak_bmsc=def"
  loopnet-pp-cli auth set --cookies-file ./loopnet-cookies.txt`,
		RunE: func(cmd *cobra.Command, args []string) error {
			value := strings.TrimSpace(cookies)
			if value == "" && cookiesFile != "" {
				data, err := os.ReadFile(cookiesFile)
				if err != nil {
					return usageErr(fmt.Errorf("reading --cookies-file: %w", err))
				}
				value = strings.TrimSpace(string(data))
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would store clearance cookies")
				return nil
			}
			if value == "" {
				return usageErr(fmt.Errorf("provide --cookies \"<header>\" or --cookies-file <path>"))
			}
			value = strings.TrimPrefix(value, "Cookie:")
			value = strings.TrimSpace(value)
			if !strings.Contains(value, "=") {
				return usageErr(fmt.Errorf("that does not look like a Cookie header (no name=value pairs)"))
			}
			if err := lnSaveCookies(value); err != nil {
				return configErr(fmt.Errorf("saving cookies: %w", err))
			}
			n := len(strings.Split(value, ";"))
			return flags.printJSON(cmd, map[string]any{
				"status": "saved", "cookies": n, "path": lnCookiePath(),
			})
		},
	}
	cmd.Flags().StringVar(&cookies, "cookies", "", "Cookie header value to store")
	cmd.Flags().StringVar(&cookiesFile, "cookies-file", "", "Read the Cookie header from this file")
	return cmd
}

// --- auth refresh -----------------------------------------------------------

func newAuthRefreshCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "refresh",
		Short: "Mint fresh clearance cookies by driving the browser-use tool",
		Long: `Refresh opens LoopNet in a real browser via the browser-use CLI, lets
Akamai validate the session, captures the resulting cookies, and stores
them. A browser window opens briefly. Requires the browser-use CLI on PATH
(install: https://github.com/browser-use/browser-use).`,
		Example: `  loopnet-pp-cli auth refresh`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would mint cookies via: browser-use --headed open", mintURL)
				return nil
			}
			// auth refresh opens a real browser window — never do that
			// inside the verifier or the live-dogfood matrix.
			if cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "auth refresh skipped (test environment — would open a browser)")
				return nil
			}
			if _, err := exec.LookPath("browser-use"); err != nil {
				return configErr(fmt.Errorf(
					"browser-use CLI not found on PATH.\n" +
						"Install it (https://github.com/browser-use/browser-use), or use\n" +
						"'loopnet-pp-cli auth set --cookies \"...\"' to paste cookies manually."))
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 120*time.Second)
			defer cancel()
			env := append(os.Environ(), "PYTHONUTF8=1", "PYTHONIOENCODING=utf-8")

			fmt.Fprintln(cmd.ErrOrStderr(), "opening LoopNet in a browser to mint clearance cookies...")
			openCmd := exec.CommandContext(ctx, "browser-use", "--headed", "open", mintURL)
			openCmd.Env = env
			if out, err := openCmd.CombinedOutput(); err != nil {
				return apiErr(fmt.Errorf("browser-use open failed: %w\n%s", err, lnTruncate(string(out), 400)))
			}
			time.Sleep(4 * time.Second) // let Akamai's sensor settle

			getCmd := exec.CommandContext(ctx, "browser-use", "cookies", "get")
			getCmd.Env = env
			out, err := getCmd.Output()
			if err != nil {
				_ = exec.Command("browser-use", "close").Run()
				return apiErr(fmt.Errorf("browser-use cookies get failed: %w", err))
			}
			_ = exec.CommandContext(ctx, "browser-use", "close").Run()

			cookie := parseBrowserUseCookies(string(out))
			if cookie == "" {
				return apiErr(fmt.Errorf("no cookies captured from browser-use — try 'auth set' to paste them manually"))
			}
			if err := lnSaveCookies(cookie); err != nil {
				return configErr(fmt.Errorf("saving cookies: %w", err))
			}

			// Self-test: confirm the fresh cookies actually clear Akamai.
			working := false
			if _, ferr := lnFetchSearch(flags, "office", "los-angeles-ca", "for-sale", 1, loopnet.SearchFilters{}); ferr == nil {
				working = true
			}
			return flags.printJSON(cmd, map[string]any{
				"status":   "refreshed",
				"cookies":  len(strings.Split(cookie, ";")),
				"verified": working,
				"note":     refreshNote(working),
			})
		},
	}
	return cmd
}

func refreshNote(working bool) string {
	if working {
		return "Clearance cookies minted and verified — live fetches should work now."
	}
	return "Cookies were saved but a test fetch still hit a challenge. Try 'auth refresh' again, or paste cookies with 'auth set'."
}

// parseBrowserUseCookies turns browser-use's `cookies get` output (a
// Python-repr list of cookie dicts) into a "name=value; name=value" header,
// keeping only loopnet.com cookies.
func parseBrowserUseCookies(raw string) string {
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start < 0 || end <= start {
		return ""
	}
	body := raw[start : end+1]
	// Best effort: convert the Python repr to JSON and decode.
	jsonish := body
	for _, r := range []struct{ from, to string }{
		{"'", "\""}, {": True", ": true"}, {": False", ": false"}, {": None", ": null"},
	} {
		jsonish = strings.ReplaceAll(jsonish, r.from, r.to)
	}
	var pairs []string
	var entries []map[string]any
	if json.Unmarshal([]byte(jsonish), &entries) == nil {
		for _, e := range entries {
			name, _ := e["name"].(string)
			value, _ := e["value"].(string)
			domain, _ := e["domain"].(string)
			if name != "" && strings.Contains(domain, "loopnet") {
				pairs = append(pairs, name+"="+value)
			}
		}
	}
	if len(pairs) == 0 {
		// Fallback: pull each cookie dict from the repr and apply the same
		// loopnet-domain filter the JSON path uses, so a shared browser
		// profile's third-party session cookies are never forwarded.
		objRE := regexp.MustCompile(`\{[^{}]*\}`)
		nameRE := regexp.MustCompile(`'name':\s*'([^']*)'`)
		valueRE := regexp.MustCompile(`'value':\s*'([^']*)'`)
		domainRE := regexp.MustCompile(`'domain':\s*'([^']*)'`)
		for _, obj := range objRE.FindAllString(body, -1) {
			name := firstSubmatch(nameRE, obj)
			domain := firstSubmatch(domainRE, obj)
			if name == "" || !strings.Contains(domain, "loopnet") {
				continue
			}
			pairs = append(pairs, name+"="+firstSubmatch(valueRE, obj))
		}
	}
	return strings.Join(pairs, "; ")
}

// firstSubmatch returns the first capture group of re in s, or "" if no match.
func firstSubmatch(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}

func lnTruncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// --- auth status ------------------------------------------------------------

func newAuthStatusCmd(flags *rootFlags) *cobra.Command {
	var noProbe bool

	cmd := &cobra.Command{
		Use:         "status",
		Short:       "Show clearance-cookie age and whether live fetches work",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     `  loopnet-pp-cli auth status`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			cookie, savedAt := lnLoadCookies()
			result := map[string]any{
				"cookies_set": cookie != "",
				"path":        lnCookiePath(),
			}
			if cookie != "" {
				result["cookie_count"] = len(strings.Split(cookie, ";"))
				if !savedAt.IsZero() {
					result["saved_at"] = savedAt.Format(time.RFC3339)
					result["age_hours"] = round2(time.Since(savedAt).Hours())
				}
			}
			if cookie == "" {
				result["note"] = "No clearance cookies stored. Run 'loopnet-pp-cli auth refresh' or 'auth set'."
				return flags.printJSON(cmd, result)
			}
			if !noProbe && !cliutil.IsVerifyEnv() {
				working := false
				if _, ferr := lnFetchSearch(flags, "office", "los-angeles-ca", "for-sale", 1, loopnet.SearchFilters{}); ferr == nil {
					working = true
				}
				result["live_fetch_ok"] = working
				if !working {
					result["note"] = "Cookies are stored but a test fetch hit a challenge — they have likely expired. Run 'auth refresh'."
				}
			}
			return flags.printJSON(cmd, result)
		},
	}
	cmd.Flags().BoolVar(&noProbe, "no-probe", false, "Skip the live test fetch")
	return cmd
}
