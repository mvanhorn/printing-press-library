// Copyright 2026 Arun Gopalakrishnan and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/ti/internal/ticompliance"
)

func newNovelPartRatingsCmd(flags *rootFlags) *cobra.Command {
	var flagCookies string

	cmd := &cobra.Command{
		Use:   "ratings <opn>",
		Short: "Rich Environmental Ratings for a part: RoHS cell with exemption wording, REACH cell, SVHC sentence (login-gated).",
		Long: strings.TrimSpace(`
Injects the deposited myTI cookie snapshot into a fresh Browserbase session,
resolves the part's Material Content report page, and scrapes the
Environmental Ratings table. Emits TI's raw cell text verbatim (e.g.
"Exempt-7(a)", "Affected") plus the SVHC disclosure sentence when present.

The CLI never logs in; when cookies are stale it exits with a distinct
"login required" error so the orchestrator can trigger a fresh HIL login.`),
		Example: strings.Trim(`
  ti-pp-cli part ratings TUSB320RWBR --cookies ./ti-cookies.json --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would inject cookies and scrape the Environmental Ratings table")
				return nil
			}
			if len(args) < 1 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an orderable part number is required"))
			}
			path := resolveCookiesPath(flagCookies)
			if path == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--cookies <file> (or TI_COOKIES_FILE) is required"))
			}
			cookies, err := ticompliance.LoadCookies(path)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			b, err := ticompliance.NewBrowser(false)
			if err != nil {
				return err
			}
			defer b.Close(ctx)
			if err := b.InjectCookies(ctx, cookies); err != nil {
				return err
			}
			res, err := b.ScrapeRatings(ctx, strings.ToUpper(strings.TrimSpace(args[0])))
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), res, flags)
		},
	}
	cmd.Flags().StringVar(&flagCookies, "cookies", "", "path to the deposited myTI cookie snapshot JSON (fallback: TI_COOKIES_FILE)")
	return cmd
}
