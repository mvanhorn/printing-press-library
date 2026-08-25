// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: library what's-new — newest literature across the whole
// library, ordered by date, filterable by category and age.

package cli

import (
	"fmt"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/extron/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/extron/internal/extron"
	"github.com/spf13/cobra"
)

// pp:data-source local

func newNovelLiteratureRecentCmd(flags *rootFlags) *cobra.Command {
	var dbPath, flagDays, flagCategory string
	var limit int

	cmd := &cobra.Command{
		Use:     "recent",
		Short:   "Newest Extron literature across the whole library, ordered by date, filterable by category and age.",
		Long:    "List the newest documents across the whole synced literature catalog, ordered by date. Use this command to track what Extron released recently. Do NOT use it for the drift of your own downloads; use 'literature updates' instead.",
		Example: "  extron-pp-cli literature recent --days 30 --category manual --json --select title,date,category,url",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "list recent literature")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			dbPath = resolveCatalogDB(flags, dbPath)
			db, ok := openCatalogReadOnly(cmd, dbPath)
			if !ok {
				return printJSONFiltered(cmd.OutOrStdout(), make([]extron.Doc, 0), flags)
			}
			defer db.Close()
			hintIfUnsynced(cmd, db, catalogResource)
			hintIfCatalogIncomplete(cmd, db)

			docs, err := loadCatalogDocs(ctx, db)
			if err != nil {
				return err
			}
			sortDocsByDateDesc(docs)
			rows := make([]extron.Doc, 0, len(docs))
			for _, d := range docs {
				if !matchCategory(d, flagCategory) {
					continue
				}
				if flagDays != "" {
					cutoff, err := parseDaysAgo(flagDays)
					if err != nil {
						return err
					}
					t := parseCatalogDate(d.Date)
					if t.IsZero() || t.Before(cutoff) {
						continue
					}
				}
				rows = append(rows, d)
				if limit > 0 && len(rows) >= limit {
					break
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No recent literature matches. Run 'extron-pp-cli catalog sync' first if the catalog is empty.")
				return nil
			}
			return printAutoTable(cmd.OutOrStdout(), docsToTable(rows))
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	cmd.Flags().StringVar(&flagDays, "days", "", "only docs published within this window (e.g. 30d, 1w, 7)")
	cmd.Flags().StringVar(&flagCategory, "category", "", "filter by category (brochure, manual, declaration, design guide, product guide, revit bim)")
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum rows to return (0 = no limit)")
	return cmd
}

// parseDaysAgo accepts "30d"/"1w"/"2h" durations or a bare day count.
func parseDaysAgo(s string) (time.Time, error) {
	now := time.Now()
	if d, err := cliutil.ParseDurationLoose(s); err == nil {
		return now.Add(-d), nil
	}
	if n, err := strconv.Atoi(s); err == nil {
		return now.AddDate(0, 0, -n), nil
	}
	return time.Time{}, fmt.Errorf("invalid --days value %q (use e.g. 30d, 1w, or 7)", s)
}
