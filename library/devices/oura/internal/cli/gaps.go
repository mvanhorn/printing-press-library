// Copyright 2026 slinsmaier and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/devices/oura/internal/store"
	"github.com/spf13/cobra"
)

type gapDay struct {
	Day     string   `json:"day"`
	Missing []string `json:"missing_resources"`
}

type gapsView struct {
	Since     string   `json:"since"`
	Until     string   `json:"until"`
	Resources []string `json:"resources_checked"`
	Gaps      []gapDay `json:"gaps"`
	Note      string   `json:"note,omitempty"`
}

var gapCheckTables = map[string]string{
	"sleep":     "daily_sleep",
	"readiness": "daily_readiness",
	"activity":  "daily_activity",
}

func newNovelGapsCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "gaps",
		Short: "List every day in a date range where expected sync data is missing from the local store — identifies ring non-wear and sync failures",
		Long: `Walks every day in the range and checks whether daily sleep, readiness, and
activity rows exist in the local store. A day missing one or more of these
usually means the ring wasn't worn or a sync failed — the API itself
cannot report what it doesn't have, so this only works against local data.`,
		Example:     `  oura-pp-cli gaps --since 30d --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		// pp:data-source local
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list days missing synced sleep/readiness/activity data in the range")
				return nil
			}
			start, err := resolveSinceDay(flagSince, 30)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			end := today()

			if dbPath == "" {
				dbPath = defaultDBPath("oura-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprint(cmd.ErrOrStderr(), missingMirrorMessage(dbPath))
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			present := make(map[string]map[string]bool)
			resourceNames := []string{"sleep", "readiness", "activity"}
			for _, name := range resourceNames {
				days, err := daysWithData(db, gapCheckTables[name], start, end)
				if err != nil {
					return fmt.Errorf("querying %s presence: %w", name, err)
				}
				present[name] = days
			}

			view := gapsView{Since: start, Until: end, Resources: resourceNames}
			anyData := false
			for d := start; d <= end; d = addDays(d, 1) {
				var missing []string
				for _, name := range resourceNames {
					if present[name][d] {
						anyData = true
					} else {
						missing = append(missing, name)
					}
				}
				if len(missing) > 0 {
					view.Gaps = append(view.Gaps, gapDay{Day: d, Missing: missing})
				}
			}
			if !anyData {
				view.Note = "no synced data at all in this range — run 'oura-pp-cli sync' first"
			}

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			out := cmd.OutOrStdout()
			if len(view.Gaps) == 0 {
				fmt.Fprintf(out, "no gaps found between %s and %s\n", start, end)
			} else {
				fmt.Fprintln(out, "day\tmissing")
				for _, g := range view.Gaps {
					fmt.Fprintf(out, "%s\t%v\n", g.Day, g.Missing)
				}
			}
			if view.Note != "" {
				fmt.Fprintln(out, "note:", view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "Start of the window: a duration like 30d or an absolute YYYY-MM-DD date (default 30d)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func daysWithData(db *store.Store, table, start, end string) (map[string]bool, error) {
	query := fmt.Sprintf(`SELECT DISTINCT day FROM %s WHERE day >= ? AND day <= ? AND day IS NOT NULL`, table)
	rows, err := db.DB().Query(query, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]bool)
	for rows.Next() {
		var day sql.NullString
		if err := rows.Scan(&day); err != nil {
			continue
		}
		if day.Valid {
			result[day.String] = true
		}
	}
	return result, rows.Err()
}
