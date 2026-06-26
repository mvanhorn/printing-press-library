// Copyright 2026 Chris Hatton and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: workbook stale. Hand-filled scaffold.

// pp:data-source local
package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// staleWorkbook is a candidate workbook with its raw updated_at timestamp.
type staleWorkbook struct {
	Name      string
	Path      string
	OwnerID   string
	UpdatedAt string
}

// staleWorkbookRow is an emitted stale-workbook record.
type staleWorkbookRow struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	OwnerEmail string `json:"ownerEmail"`
	UpdatedAt  string `json:"updatedAt"`
	DaysSince  int    `json:"daysSince"`
}

func newNovelWorkbookStaleCmd(flags *rootFlags) *cobra.Command {
	var flagDays int

	cmd := &cobra.Command{
		Use:   "stale",
		Short: "List workbooks untouched for more than N days, joined to their owner and folder path.",
		Example: strings.Trim(`
  sigma-computing-pp-cli workbook stale --days 90
  sigma-computing-pp-cli workbook stale --days 30 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagDays < 0 {
				return fmt.Errorf("invalid --days %d: must be a non-negative integer", flagDays)
			}

			db, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer db.Close()

			// Offline command: warn (on stderr) when the local store has
			// never been synced, so an empty result reads as "run sync
			// first" rather than "no stale workbooks".
			hintIfUnsynced(cmd, db, "workbooks")

			candidates, err := loadWorkbooksForStale(db.DB())
			if err != nil {
				return fmt.Errorf("loading workbooks: %w", err)
			}

			rows := filterStaleWorkbooks(candidates, flagDays, time.Now())
			// Resolve owner emails.
			for i := range rows {
				if email := memberEmailByID(db.DB(), rows[i].OwnerEmail); email != "" {
					rows[i].OwnerEmail = email
				}
			}

			if wantJSON(flags, cmd) {
				if rows == nil {
					rows = []staleWorkbookRow{}
				}
				return flags.printJSON(cmd, rows)
			}
			headers := []string{"NAME", "PATH", "OWNER EMAIL", "UPDATED AT", "DAYS SINCE"}
			out := make([][]string, 0, len(rows))
			for _, r := range rows {
				out = append(out, []string{r.Name, r.Path, r.OwnerEmail, r.UpdatedAt, strconv.Itoa(r.DaysSince)})
			}
			return flags.printTable(cmd, headers, out)
		},
	}
	cmd.Flags().IntVar(&flagDays, "days", 90, "Workbooks untouched for more than this many days are reported")
	return cmd
}

// loadWorkbooksForStale reads the workbook fields needed for staleness analysis.
func loadWorkbooksForStale(db *sql.DB) ([]staleWorkbook, error) {
	rows, err := db.Query(
		`SELECT COALESCE(name,''), COALESCE(path,''), COALESCE(owner_id,''), COALESCE(updated_at,'') FROM workbooks`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []staleWorkbook
	for rows.Next() {
		var wb staleWorkbook
		if err := rows.Scan(&wb.Name, &wb.Path, &wb.OwnerID, &wb.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, wb)
	}
	return out, rows.Err()
}

// parseWorkbookTime parses the store's updated_at timestamp across the formats
// SQLite/Sigma may emit. Returns ok=false when unparseable or empty.
func parseWorkbookTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// filterStaleWorkbooks returns workbooks whose updated_at is older than `days`
// before `now`, sorted by daysSince descending. Workbooks with unparseable
// timestamps are skipped. Pure function for testability.
func filterStaleWorkbooks(workbooks []staleWorkbook, days int, now time.Time) []staleWorkbookRow {
	cutoff := now.AddDate(0, 0, -days)
	var out []staleWorkbookRow
	for _, wb := range workbooks {
		t, ok := parseWorkbookTime(wb.UpdatedAt)
		if !ok {
			continue
		}
		if !t.Before(cutoff) {
			continue
		}
		daysSince := int(now.Sub(t).Hours() / 24)
		out = append(out, staleWorkbookRow{
			Name:       wb.Name,
			Path:       wb.Path,
			OwnerEmail: wb.OwnerID, // resolved to email by caller
			UpdatedAt:  wb.UpdatedAt,
			DaysSince:  daysSince,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].DaysSince > out[j].DaysSince
	})
	return out
}
