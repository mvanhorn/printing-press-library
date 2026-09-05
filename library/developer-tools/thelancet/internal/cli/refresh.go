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
	var fromYear int
	var toYear int
	var maxPages int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "refresh",
		Short: "Sync Lancet articles from OpenAlex into the local analytics store",
		Long: "Populate the local SQLite store the analytics commands (rank-authors, mesh,\n" +
			"affiliation-growth, drift, curate, visibility-gap) read from. Scopes to a\n" +
			"Lancet journal slug, the flagship 'lancet', or 'all' for the whole family.\n\n" +
			"Use --years for a rolling window ending today, or --from-year/--to-year to\n" +
			"sync an absolute slice of the archive, which is how older years are\n" +
			"backfilled a slice at a time. --years cannot be combined with either\n" +
			"absolute bound.",
		Example: "  thelancet-pp-cli refresh --journal lancet\n" +
			"  thelancet-pp-cli refresh --journal lancet-oncology --years 8\n" +
			"  thelancet-pp-cli refresh --journal lancet --from-year 1990 --to-year 1999\n" +
			"  thelancet-pp-cli refresh --journal all --max-pages 3",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would sync journal %q from OpenAlex into the local store\n", journal)
				return nil
			}
			fromBound, toBound, err := refreshYearBounds(cmd, years, cmd.Flags().Changed("years"), fromYear, toYear)
			if err != nil {
				return err
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
			results, err := lancet.Refresh(ctx, c, s.DB(), journs, fromBound, toBound, maxPages, progress)
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
	cmd.Flags().IntVar(&years, "years", 0, "Only sync works published in the last N years (0 = all available; not combinable with --from-year/--to-year)")
	cmd.Flags().IntVar(&fromYear, "from-year", 0, "Only sync works published in or after this calendar year (0 = no lower bound)")
	cmd.Flags().IntVar(&toYear, "to-year", 0, "Only sync works published in or before this calendar year (0 = no upper bound)")
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

// earliestSyncYear is the oldest calendar year a refresh may be bounded by;
// anything older is a typo rather than a real request.
const earliestSyncYear = 1800

// refreshYearBounds resolves the three year flags into the absolute lower and
// upper publication-year bounds handed to lancet.Refresh. --years is a rolling
// window and cannot be mixed with the absolute --from-year/--to-year bounds.
//
// yearsSet reports whether --years was named on the command line, which is not
// the same as years being non-zero: an explicit "--years 0" must still collide
// with an absolute bound. Testing the value alone would let that combination
// through silently, which is the behaviour this flag pair exists to prevent.
func refreshYearBounds(cmd *cobra.Command, years int, yearsSet bool, fromYear, toYear int) (int, int, error) {
	if yearsSet && (fromYear > 0 || toYear > 0) {
		_ = cmd.Usage()
		return 0, 0, usageErr(fmt.Errorf("--years is a rolling window and cannot be combined with --from-year or --to-year; use --years for the last N years, or --from-year/--to-year for an absolute slice"))
	}
	maxYear := time.Now().Year() + 1
	for _, b := range []struct {
		name  string
		value int
	}{{"--from-year", fromYear}, {"--to-year", toYear}} {
		if b.value == 0 {
			continue
		}
		if b.value < earliestSyncYear || b.value > maxYear {
			_ = cmd.Usage()
			return 0, 0, usageErr(fmt.Errorf("%s must be a calendar year between %d and %d, got %d", b.name, earliestSyncYear, maxYear, b.value))
		}
	}
	if fromYear > 0 && toYear > 0 && fromYear > toYear {
		_ = cmd.Usage()
		return 0, 0, usageErr(fmt.Errorf("--from-year must not be after --to-year: got --from-year %d and --to-year %d", fromYear, toYear))
	}
	if years > 0 {
		return yearLowerBound(years), 0, nil
	}
	return fromYear, toYear, nil
}
