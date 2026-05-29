// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: find every meeting whose given property was EVER set to a
// given value within a date range, even if it has since changed.

package cli

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/store"
	"github.com/spf13/cobra"
)

func newNovelMeetingsEverHadCmd(flags *rootFlags) *cobra.Command {
	var property string
	var value string
	var fromArg string
	var toArg string
	var dbPath string
	var filterFlags []string
	var filterDebug bool

	cmd := &cobra.Command{
		Use:         "ever-had",
		Short:       "Find every meeting whose property was EVER set to a given value within a date range",
		Long:        `Walk the local hubspot_property_history table for rows matching (object_type='meetings', property, value) inside the [from, to] window, then join the current meeting row from hubspot_meetings_crm so the report shows both the historical hit AND the meeting's current state.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  hubspot-pp-cli meetings ever-had --property hs_meeting_outcome --value SCHEDULED --from 2026-05-01 --to 2026-05-31
  hubspot-pp-cli meetings ever-had --property hs_meeting_outcome --value Scheduled --from 2026-04-01 --to 2026-04-30 --filter 'hubspot_owner_id=12345678'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate inside RunE so verify/dry-run can still walk through.
			if property == "" || value == "" {
				if dryRunOK(flags) {
					return nil
				}
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			expr, err := cliutil.ParseFilters(filterFlags)
			if err != nil {
				return err
			}
			if filterDebug {
				fmt.Fprint(cmd.ErrOrStderr(), expr.DebugString())
			}
			fromTS, toTS, err := parseDateWindow(fromArg, toArg)
			if err != nil {
				return err
			}
			if dbPath == "" {
				dbPath = defaultDBPath("hubspot-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			rows, err := queryMeetingsEverHad(cmd, db, property, value, fromTS, toTS, expr)
			if err != nil {
				return err
			}

			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"property": property,
					"value":    value,
					"from":     fromTS.Format(time.RFC3339),
					"to":       toTS.Format(time.RFC3339),
					"results":  rows,
				})
			}
			headers := []string{"meeting_id", "title", "owner_id", "first_hit_at", "current_outcome"}
			out := make([][]string, 0, len(rows))
			for _, r := range rows {
				out = append(out, []string{r.MeetingID, r.Title, r.OwnerID, r.FirstHitAt, r.CurrentOutcome})
			}
			return flags.printTable(cmd, headers, out)
		},
	}
	cmd.Flags().StringVar(&property, "property", "", "Property name to query (e.g. hs_meeting_outcome) — required")
	cmd.Flags().StringVar(&value, "value", "", "Value the property must have EVER held — required")
	cmd.Flags().StringVar(&fromArg, "from", "", "Lower-bound timestamp (YYYY-MM-DD or RFC3339). Default: epoch")
	cmd.Flags().StringVar(&toArg, "to", "", "Upper-bound timestamp (YYYY-MM-DD or RFC3339). Default: now")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringSliceVar(&filterFlags, "filter", nil, filterDescription)
	cmd.Flags().BoolVar(&filterDebug, "filter-debug", false, "Print parsed --filter expression to stderr before applying it")
	return cmd
}

type meetingsEverHadRow struct {
	MeetingID      string `json:"meeting_id"`
	Title          string `json:"title"`
	OwnerID        string `json:"owner_id"`
	FirstHitAt     string `json:"first_hit_at"`
	CurrentOutcome string `json:"current_outcome"`
}

func queryMeetingsEverHad(cmd *cobra.Command, db *store.Store, property, value string, fromTS, toTS time.Time, expr cliutil.FilterExpr) ([]meetingsEverHadRow, error) {
	// Pull each matching object_id with its earliest hit timestamp inside the window.
	q := `SELECT object_id, MIN(timestamp) AS first_hit_at
FROM hubspot_property_history
WHERE object_type = 'meetings'
  AND property = ?
  AND value = ?
  AND timestamp BETWEEN ? AND ?
GROUP BY object_id
ORDER BY first_hit_at DESC`
	rows, err := db.DB().QueryContext(cmd.Context(), q, property, value, fromTS.UTC(), toTS.UTC())
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()
	filterFields := expr.FieldsReferenced()
	out := []meetingsEverHadRow{}
	for rows.Next() {
		var id, firstHit string
		if err := rows.Scan(&id, &firstHit); err != nil {
			return nil, err
		}
		row := meetingsEverHadRow{MeetingID: id, FirstHitAt: firstHit}
		// Best-effort: look up the meeting's current state so the report shows
		// "was X, now is Y". If the meeting was deleted from the local mirror
		// (or never synced), we still emit the row — we just omit the join.
		var title, owner, outcome sql.NullString
		var raw sql.NullString
		_ = db.DB().QueryRowContext(cmd.Context(), `
SELECT COALESCE(json_extract(data, '$.properties.hs_meeting_title'), ''),
       COALESCE(json_extract(data, '$.properties.hubspot_owner_id'), ''),
       COALESCE(json_extract(data, '$.properties.hs_meeting_outcome'), ''),
       data
FROM hubspot_meetings_crm
WHERE id = ?`, id).Scan(&title, &owner, &outcome, &raw)
		row.Title = nullStr(title)
		row.OwnerID = nullStr(owner)
		row.CurrentOutcome = nullStr(outcome)
		if !expr.IsEmpty() {
			// Filter against the meeting's current properties. A meeting
			// missing from the local mirror (raw empty) is treated as not
			// matching any HAS/EQ/CONTAINS clause, but still matches NOT_HAS
			// for absent fields — extractPropertiesRow returns an empty map
			// in that case, which is exactly the desired semantics.
			propRow := extractPropertiesRow(raw.String, filterFields)
			if !expr.Match(propRow) {
				continue
			}
		}
		out = append(out, row)
	}
	return out, nil
}

// parseDateWindow accepts YYYY-MM-DD or RFC3339; missing values default to
// epoch / now respectively.
func parseDateWindow(fromArg, toArg string) (time.Time, time.Time, error) {
	var from, to time.Time
	var err error
	if fromArg == "" {
		from = time.Unix(0, 0)
	} else {
		from, err = parseDateOrTimestamp(fromArg)
		if err != nil {
			return from, to, fmt.Errorf("--from: %w", err)
		}
	}
	if toArg == "" {
		to = time.Now()
	} else {
		to, err = parseDateOrTimestamp(toArg)
		if err != nil {
			return from, to, fmt.Errorf("--to: %w", err)
		}
	}
	if to.Before(from) {
		return from, to, fmt.Errorf("--to (%s) is before --from (%s)", to.Format(time.RFC3339), from.Format(time.RFC3339))
	}
	return from, to, nil
}

// parseDateOrTimestamp accepts YYYY-MM-DD (interpreted in UTC) or RFC3339.
func parseDateOrTimestamp(s string) (time.Time, error) {
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("unrecognized timestamp %q (expected YYYY-MM-DD or RFC3339)", s)
}
