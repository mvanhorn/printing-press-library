package cli

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/redfin/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/redfin/internal/config"
	"github.com/mvanhorn/printing-press-library/library/other/redfin/internal/redfin"
	"github.com/spf13/cobra"
)

// PATCH: redfin blocks listing detail endpoints at the edge unless the request
// carries cookies a real browser earned (issue #698), so the CLI gains a
// cookie import path. No other command needs auth.
func newAuthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage browser session cookies for listing detail endpoints",
		Long: `Search and market commands need no auth. Listing detail (listing, comps)
can return HTTP 403 without a browser session. Browse redfin.com in a
browser, export the cookies to a file, and import them here.`,
	}
	cmd.AddCommand(newAuthImportFileCmd(flags))
	cmd.AddCommand(newAuthStatusCmd(flags))
	cmd.AddCommand(newAuthLogoutCmd(flags))
	return cmd
}

func newAuthImportFileCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "import-file <path>",
		Short: "Import redfin.com cookies from a JSON export or cookies.txt file",
		Long: `Read a browser cookie export and save its unexpired redfin.com cookies to
the config file (mode 0600). Accepts a JSON array of cookies, an object
with a cookies array, or a Netscape cookies.txt file. Cookies for other
sites are ignored. Cookie values are never printed.

Browser challenge cookies expire, so re-import if listing returns 403 again.`,
		Example:     "  redfin-pp-cli auth import-file ~/Downloads/redfin-cookies.json",
		Annotations: map[string]string{"mcp:hidden": "true"},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			data, err := os.ReadFile(args[0])
			if err != nil {
				return usageErr(fmt.Errorf("reading cookie file: %w", err))
			}
			cookies, err := redfin.ParseCookieExport(data, time.Now())
			if err != nil {
				return usageErr(err)
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			if err := cfg.SaveCookies(redfin.CookieHeader(cookies)); err != nil {
				return configErr(err)
			}
			challenge := redfin.HasChallengeCookies(cookies)
			if !challenge {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning: no _abck or bm_sz cookie found; browse a redfin.com listing page first, then export again")
			}
			return flags.printJSON(cmd, map[string]any{
				"imported":          len(cookies),
				"challenge_cookies": challenge,
				"config":            cfg.Path,
			})
		},
	}
}

func newAuthStatusCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "status",
		Short:       "Show whether session cookies are stored",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			count := 0
			if cfg.CookieHeader != "" {
				count = len(strings.Split(cfg.CookieHeader, "; "))
			}
			return flags.printJSON(cmd, map[string]any{"cookies_stored": count, "config": cfg.Path})
		},
	}
}

func newAuthLogoutCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "logout",
		Short:       "Remove stored session cookies",
		Annotations: map[string]string{"mcp:hidden": "true"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			if err := cfg.SaveCookies(""); err != nil {
				return configErr(err)
			}
			return flags.printJSON(cmd, map[string]any{"cookies_stored": 0, "config": cfg.Path})
		},
	}
}

// listingBlocked reports whether err is a 403 from the Redfin API.
func listingBlocked(err error) bool {
	var apiError *client.APIError
	return errors.As(err, &apiError) && apiError.StatusCode == http.StatusForbidden
}

// listingBlockedHint adds the fix to a 403 from a listing detail endpoint.
func listingBlockedHint(err error) error {
	if listingBlocked(err) {
		return fmt.Errorf("%w\nhint: Redfin blocks this endpoint without a browser session. Browse redfin.com, export cookies, then run: redfin-pp-cli auth import-file <path>", err)
	}
	return err
}
