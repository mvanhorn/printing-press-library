// Copyright 2026 dan-bronson. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/harvest/internal/store"
)

type reconcileChange struct {
	ID        string                 `json:"id"`
	Kind      string                 `json:"kind"` // "added" | "removed" | "drift"
	Fields    map[string]interface{} `json:"fields,omitempty"`
	SpentDate string                 `json:"spent_date,omitempty"`
}

type reconcileReport struct {
	From      string            `json:"from"`
	To        string            `json:"to"`
	LocalRows int               `json:"local_rows"`
	APIRows   int               `json:"api_rows"`
	Added     int               `json:"added"`
	Removed   int               `json:"removed"`
	Drift     int               `json:"drift"`
	Changes   []reconcileChange `json:"changes"`
}

func newReconcileCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		from   string
		to     string
		fields string
	)

	cmd := &cobra.Command{
		Use:   "reconcile",
		Short: "Diff local SQLite vs the Harvest API for a date range",
		Long: `Re-fetches every time entry in [from, to] from the API and diffs against the
local snapshot. Surfaces:
  - added:    entry exists upstream but not locally (you forgot to sync)
  - removed:  entry exists locally but the API deleted it
  - drift:    same ID, different field values (someone edited after last sync)

Exit code 2 if any drift/added/removed is detected. Cron-friendly. Useful for
nightly integrity checks on stale dashboards.`,
		Example: `  # Last week's reconcile
  harvest-pp-cli reconcile --from 2026-05-08 --to 2026-05-15 --json

  # Watch a single day
  harvest-pp-cli reconcile --from 2026-05-14 --to 2026-05-14 --fields hours,notes,billable --json`,
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, &reconcileReport{From: from, To: to, Changes: []reconcileChange{}})
			}
			if from == "" || to == "" {
				return fmt.Errorf("--from and --to are required (YYYY-MM-DD)")
			}
			if _, err := time.Parse("2006-01-02", from); err != nil {
				return fmt.Errorf("parse --from: %w", err)
			}
			if _, err := time.Parse("2006-01-02", to); err != nil {
				return fmt.Errorf("parse --to: %w", err)
			}
			if dbPath == "" {
				dbPath = defaultDBPath("harvest-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			report, err := runReconcile(cmd, flags, db, from, to, fields)
			if err != nil {
				return err
			}
			if err := flags.printJSON(cmd, report); err != nil {
				return err
			}
			if report.Added+report.Removed+report.Drift > 0 {
				return &typedExit{code: 2, msg: fmt.Sprintf("%d added, %d removed, %d drift", report.Added, report.Removed, report.Drift)}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&from, "from", "", "Earliest spent_date (YYYY-MM-DD, required)")
	cmd.Flags().StringVar(&to, "to", "", "Latest spent_date (YYYY-MM-DD, required)")
	cmd.Flags().StringVar(&fields, "fields", "hours,notes,billable,billable_rate,is_locked,project.id,task.id", "Comma-separated dotted JSON paths to compare")
	return cmd
}

func runReconcile(cmd *cobra.Command, flags *rootFlags, db *store.Store, from, to, fields string) (*reconcileReport, error) {
	// Local rows.
	localRows, err := db.DB().QueryContext(cmd.Context(), `SELECT data FROM time_entries
		WHERE json_extract(data, '$.spent_date') >= ?
		  AND json_extract(data, '$.spent_date') <= ?`, from, to)
	if err != nil {
		return nil, fmt.Errorf("query local: %w", err)
	}
	defer localRows.Close()
	local := map[string]map[string]any{}
	for localRows.Next() {
		var raw []byte
		if err := localRows.Scan(&raw); err != nil {
			return nil, err
		}
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			continue
		}
		id := extractReconcileID(obj)
		if id != "" {
			local[id] = obj
		}
	}

	// API rows.
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	api := map[string]map[string]any{}
	page := 1
	for {
		params := map[string]string{
			"from":     from,
			"to":       to,
			"page":     strconv.Itoa(page),
			"per_page": "2000",
		}
		raw, err := c.Get("/time_entries", params)
		if err != nil {
			return nil, fmt.Errorf("fetch live page %d: %w", page, err)
		}
		var resp struct {
			TimeEntries []map[string]any `json:"time_entries"`
			NextPage    *int             `json:"next_page"`
		}
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, fmt.Errorf("parse live response: %w", err)
		}
		for _, e := range resp.TimeEntries {
			id := extractReconcileID(e)
			if id != "" {
				api[id] = e
			}
		}
		if resp.NextPage == nil {
			break
		}
		page = *resp.NextPage
	}

	report := &reconcileReport{From: from, To: to, LocalRows: len(local), APIRows: len(api)}
	keys := map[string]bool{}
	for k := range local {
		keys[k] = true
	}
	for k := range api {
		keys[k] = true
	}
	fieldList := splitFields(fields)
	var idsSorted []string
	for k := range keys {
		idsSorted = append(idsSorted, k)
	}
	sort.Strings(idsSorted)
	for _, id := range idsSorted {
		l, hasL := local[id]
		r, hasR := api[id]
		switch {
		case !hasL && hasR:
			report.Added++
			report.Changes = append(report.Changes, reconcileChange{ID: id, Kind: "added", SpentDate: stringFieldFromObj(r, "spent_date")})
		case hasL && !hasR:
			report.Removed++
			report.Changes = append(report.Changes, reconcileChange{ID: id, Kind: "removed", SpentDate: stringFieldFromObj(l, "spent_date")})
		case hasL && hasR:
			diffs := compareReconcileFields(l, r, fieldList)
			if len(diffs) > 0 {
				report.Drift++
				report.Changes = append(report.Changes, reconcileChange{ID: id, Kind: "drift", Fields: diffs, SpentDate: stringFieldFromObj(r, "spent_date")})
			}
		}
	}
	return report, nil
}

func extractReconcileID(obj map[string]any) string {
	switch v := obj["id"].(type) {
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case string:
		return v
	}
	return ""
}

func stringFieldFromObj(obj map[string]any, key string) string {
	if v, ok := obj[key].(string); ok {
		return v
	}
	return ""
}

func splitFields(csv string) []string {
	out := []string{}
	cur := ""
	for _, r := range csv {
		switch r {
		case ',':
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
		case ' ':
		default:
			cur += string(r)
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func compareReconcileFields(a, b map[string]any, fields []string) map[string]interface{} {
	out := map[string]interface{}{}
	for _, f := range fields {
		va := getDotted(a, f)
		vb := getDotted(b, f)
		if !reflect.DeepEqual(va, vb) {
			out[f] = map[string]interface{}{"local": va, "api": vb}
		}
	}
	return out
}

func getDotted(obj map[string]any, path string) any {
	if obj == nil {
		return nil
	}
	parts := []string{}
	cur := ""
	for _, r := range path {
		if r == '.' {
			parts = append(parts, cur)
			cur = ""
		} else {
			cur += string(r)
		}
	}
	if cur != "" {
		parts = append(parts, cur)
	}
	var v any = obj
	for _, p := range parts {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[p]
	}
	return v
}
