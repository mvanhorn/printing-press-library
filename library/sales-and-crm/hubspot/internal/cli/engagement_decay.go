// Copyright 2026 sdhilip200. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/store"
	"github.com/spf13/cobra"
)

func newNovelEngagementDecayCmd(flags *rootFlags) *cobra.Command {
	var window string
	var minPriorOpens int
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "engagement-decay",
		Short: "Find previously-active contacts who have gone dark, ranked by prior engagement intensity.",
		Long: strings.Trim(`
Surface contacts that used to open or click your emails (at least
--min-prior-opens) but whose last activity has fallen outside the --window
lookback. Ranks by prior open count then by how long the contact has been
silent, so the most engaged-then-quiet contacts come first. Contacts with
no recorded last activity at all qualify when their open count meets the
threshold — total silence is the dominant signal.

Reads exclusively from the local crm table populated by 'sync'. When
hs_email_open isn't in the synced properties, the output is empty.
`, "\n"),
		Example: strings.Trim(`
  # Contacts with >=3 prior opens who have gone silent over the last 30 days
  hubspot-pp-cli engagement-decay --window 30d --min-prior-opens 3 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			win, err := cliutil.ParseDurationLoose(window)
			if err != nil {
				return usageErr(fmt.Errorf("--window %q: %w", window, err))
			}
			if win <= 0 {
				return usageErr(fmt.Errorf("--window must be positive"))
			}
			if minPriorOpens < 0 {
				return usageErr(fmt.Errorf("--min-prior-opens must be >= 0"))
			}
			if limit <= 0 {
				return usageErr(fmt.Errorf("--limit must be > 0"))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("hubspot-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			now := time.Now().UTC()
			items, scanned, parseErrors, err := computeDecay(cmd.Context(), db, now, win, int64(minPriorOpens))
			if err != nil {
				return err
			}
			matches := len(items)
			items = limitInt(items, limit)
			result := map[string]any{
				"window":          window,
				"cutoff":          now.Add(-win).Format(time.RFC3339),
				"min_prior_opens": minPriorOpens,
				"items":           items,
				"scanned":         scanned,
				"matches":         matches,
			}
			if parseErrors > 0 {
				result["parse_errors"] = parseErrors
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&window, "window", "30d", "Lookback window for last-activity cutoff (e.g. 30d, 4w, 720h)")
	cmd.Flags().IntVar(&minPriorOpens, "min-prior-opens", 1, "Minimum hs_email_open count for a contact to qualify as previously-active")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of items to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override default SQLite database path")
	return cmd
}
