// Hand-authored Lancet analytics sync command. Not generated.

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/thelancet/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/thelancet/internal/lancet"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/thelancet/internal/store"
)

func newLancetRefreshCmd(flags *rootFlags) *cobra.Command {
	var journal string
	var years int
	var maxPages int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "refresh",
		Short: "Sync Lancet articles from OpenAlex into the local analytics store",
		Long: "Populate the local SQLite store the analytics commands (rank-authors, mesh,\n" +
			"affiliation-growth, drift, curate, visibility-gap) read from. Scopes to a\n" +
			"Lancet journal slug, the flagship 'lancet', or 'all' for the whole family.",
		Example: "  thelancet-pp-cli refresh --journal lancet\n" +
			"  thelancet-pp-cli refresh --journal lancet-oncology --years 8\n" +
			"  thelancet-pp-cli refresh --journal all --max-pages 3",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would sync journal %q from OpenAlex into the local store\n", journal)
				return nil
			}
			journs, ok := lancet.Lookup(journal)
			if !ok {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("unknown journal %q; see 'thelancet-pp-cli refresh --help' for slugs (or use 'all')", journal))
			}
			if maxPages < 1 {
				maxPages = 1
			}
			// Curtail work under live-dogfood to fit the per-command timeout.
			if cliutil.IsDogfoodEnv() {
				maxPages = 1
				if len(journs) > 1 {
					journs = journs[:1]
				}
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("thelancet-pp-cli")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			s, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer s.Close()

			var progress = cmd.ErrOrStderr()
			if flags.asJSON || flags.agent {
				progress = nil
			}
			results, err := lancet.Refresh(ctx, c, s.DB(), journs, yearLowerBound(years), maxPages, progress)
			if err != nil {
				return fmt.Errorf("refresh: %w", err)
			}
			total := 0
			for _, r := range results {
				total += r.Stored
			}
			view := map[string]any{"journals": results, "total_stored": total, "db": dbPath}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%-36s %6d works\n", r.Journal, r.Stored)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d works stored in %s\n", total, dbPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&journal, "journal", "lancet", "Journal slug, 'lancet' (flagship), or 'all' for the whole Lancet family")
	cmd.Flags().IntVar(&years, "years", 0, "Only sync works published in the last N years (0 = all available)")
	cmd.Flags().IntVar(&maxPages, "max-pages", 5, "Maximum 200-item pages to fetch per journal (bounds cost)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default ~/.local/share/thelancet-pp-cli/data.db)")
	return cmd
}

// yearLowerBound converts a "last N years" window into an absolute lower-bound
// year, using the current calendar year as the reference. 0 means no bound.
func yearLowerBound(years int) int {
	if years <= 0 {
		return 0
	}
	currentYear := time.Now().Year()
	return currentYear - years + 1
}