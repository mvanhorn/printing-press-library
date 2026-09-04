// Copyright 2026 Paul Gradeff and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: new-since diff against a local store.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/monitoring/utah-pmn/internal/store"
	"github.com/spf13/cobra"
)

// pp:data-source auto
func newNovelSinceCmd(flags *rootFlags) *cobra.Command {
	var flagLocation string
	var flagDays int
	var flagLimit int
	var flagDB string
	var flagPeek bool
	var flagLandUse bool

	cmd := &cobra.Command{
		Use:   "since",
		Short: "Show only notices first seen since the last run (local diff)",
		Long: "Show only notices first seen since the last run. The first run records a baseline " +
			"(everything is new); later runs report just what changed — ideal on a schedule. " +
			"With --location, scans one ZIP/city; without it, sweeps all Millard County towns. " +
			"Use --peek to see new notices without recording them, --landuse to keep only approval items.",
		Example:     "  utah-pmn-pp-cli since --location Fillmore --agent",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would report notices first seen since the last run")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			start, end := dateWindow(flagDays)
			var notices []pmnNotice
			if flagLocation != "" {
				notices, err = fetchNotices(ctx, c, flagLocation, start, end, flagLimit)
				sortNoticesByDate(notices)
			} else {
				notices, err = sweepLocations(ctx, c, millardCityNames(), start, end, flagLimit)
			}
			if err != nil {
				return classifyAPIError(err, flags)
			}

			dbPath := flagDB
			if dbPath == "" {
				dbPath = defaultDBPath("utah-pmn-pp-cli")
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if err := ensurePMNTables(ctx, db.DB()); err != nil {
				return fmt.Errorf("preparing tables: %w", err)
			}

			now := time.Now().Format(time.RFC3339)
			fresh := make([]pmnNotice, 0)
			for _, n := range notices {
				if flagLandUse && !bodyLooksLandUse(n) {
					if m, _ := agendaHasLandUse(n); !m {
						continue
					}
				}
				seen, err := noticeSeen(ctx, db.DB(), n.NoticeID)
				if err != nil {
					return fmt.Errorf("checking seen: %w", err)
				}
				if seen {
					continue
				}
				fresh = append(fresh, n)
				if !flagPeek {
					if err := recordNotice(ctx, db.DB(), n, now); err != nil {
						return fmt.Errorf("recording notice: %w", err)
					}
				}
			}
			b, err := json.Marshal(fresh)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
		},
	}
	cmd.Flags().StringVar(&flagLocation, "location", "", "ZIP or city to scan (default: all Millard County towns)")
	cmd.Flags().IntVar(&flagDays, "days", 90, "Days ahead to scan from today")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Max notices per location before diffing")
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path (default: standard cache location)")
	cmd.Flags().BoolVar(&flagPeek, "peek", false, "Show new notices without recording them")
	cmd.Flags().BoolVar(&flagLandUse, "landuse", false, "Keep only land-use approval notices")
	return cmd
}
