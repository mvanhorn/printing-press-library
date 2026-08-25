// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: quote-snapshot history. Hand-authored.
// pp:data-source local

package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/sea-airport-parking/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/sea-airport-parking/internal/store"

	"github.com/spf13/cobra"
)

func newNovelHistoryCmd(flags *rootFlags) *cobra.Command {
	var entry, exit, since string

	cmd := &cobra.Command{
		Use:   "history",
		Short: "List every recorded quote snapshot for a date range: price, availability, promo, captured time.",
		Long: "List the raw recorded quote snapshots for an exact entry/exit range, newest\n" +
			"first. This is the read surface over the local store; for the summarized\n" +
			"first-vs-latest change use 'drift'. Returns nothing for a range never quoted —\n" +
			"run 'quote' or 'sweep' first to populate it.",
		Example: "  sea-airport-parking-pp-cli history --entry 2026-08-15T11:00 --exit 2026-08-18T11:00 --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return usageErr(err)
			}
			if entry == "" || exit == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--entry and --exit are required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			entryKey, exitKey, err := rangeKeys(entry, exit)
			if err != nil {
				return usageErr(err)
			}
			var sinceT time.Time
			if since != "" {
				d, derr := cliutil.ParseDurationLoose(since)
				if derr != nil {
					return usageErr(fmt.Errorf("invalid --since %q (use e.g. 7d, 24h)", since))
				}
				sinceT = time.Now().Add(-d)
			}

			dbPath := defaultDBPath(dbName)
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local quote history at %s\nrun: sea-airport-parking-pp-cli quote --entry %s --exit %s\n", dbPath, entry, exit)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}

			s, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()
			rows, err := s.ListQuotes(ctx, entryKey, exitKey, sinceT)
			if err != nil {
				return fmt.Errorf("reading history: %w", err)
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, rows)
			}
			if len(rows) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no snapshots recorded for %s → %s\n", entryKey, exitKey)
				return nil
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%d snapshot(s) for %s → %s:\n", len(rows), entryKey, exitKey)
			for _, r := range rows {
				fmt.Fprintf(w, "  %s  %s  $%.2f\n", r.CapturedAt, availabilityLabel(r), r.TotalPrice)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&entry, "entry", "", "Entry date/time of the range, e.g. 2026-08-15T11:00")
	cmd.Flags().StringVar(&exit, "exit", "", "Exit date/time of the range, e.g. 2026-08-18T11:00")
	cmd.Flags().StringVar(&since, "since", "", "Only snapshots captured within this window (e.g. 7d, 24h)")
	return cmd
}

func availabilityLabel(r store.QuoteRecord) string {
	switch {
	case r.SoldOut:
		return "sold-out "
	case r.Available:
		return "available"
	default:
		return "unavail. "
	}
}

// rangeKeys parses entry/exit and formats them to the stored snapshot key
// format (2006-01-02T15:04), matching how the live commands persist a range.
func rangeKeys(entry, exit string) (string, string, error) {
	entryT, err := parseWhen(entry)
	if err != nil {
		return "", "", err
	}
	exitT, err := parseWhen(exit)
	if err != nil {
		return "", "", err
	}
	return entryT.Format("2006-01-02T15:04"), exitT.Format("2006-01-02T15:04"), nil
}
