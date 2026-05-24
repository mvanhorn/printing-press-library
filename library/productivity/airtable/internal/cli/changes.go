// Copyright 2026 joelsephus. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/airtable/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/airtable/internal/store"
	"github.com/spf13/cobra"
)

func newChangesCmd(flags *rootFlags) *cobra.Command {
	var since string
	var groupBy string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "changes",
		Short:       "Report what changed across synced webhook payloads in a recent window",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Joins the synced webhook payloads with the current records snapshot to
report what fields changed across a base in the last N hours/days, grouped
by table, field, or actor.

Requires that 'airtable-pp-cli sync' has populated the local mirror with
webhook payload data.`,
		Example: strings.Trim(`
  # Show all changes in the last 7 days, grouped by table
  airtable-pp-cli changes --since 7d --group-by table

  # Changes in the last 24 hours grouped by field
  airtable-pp-cli changes --since 24h --group-by field

  # Last 2 weeks grouped by actor (--json for piping)
  airtable-pp-cli changes --since 2w --group-by actor --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				// help is fine for the no-args interactive case
			}
			if dryRunOK(flags) {
				return nil
			}
			switch groupBy {
			case "", "table", "field", "actor":
			default:
				return usageErr(fmt.Errorf("--group-by must be one of: table, field, actor (got %q)", groupBy))
			}
			window, err := cliutil.ParseDurationLoose(since)
			if err != nil {
				return usageErr(fmt.Errorf("--since: %w", err))
			}
			if window <= 0 {
				window = 7 * 24 * time.Hour
			}

			if dbPath == "" {
				dbPath = defaultDBPath("airtable-pp-cli")
			}

			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: airtable-pp-cli sync --resources records,webhooks --db %s\n", dbPath, dbPath)
				if flags.asJSON {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}

			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'airtable-pp-cli sync' first.", err)
			}
			defer db.Close()

			cutoff := time.Now().Add(-window)

			// Read synced webhook payload rows from the resources table.
			// The schema doesn't ship a dedicated webhook_payloads table; the
			// generic resources table is the canonical landing zone for
			// payload JSON synced under resource_type='webhooks'.
			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT id, data, updated_at FROM resources
				 WHERE resource_type = 'webhooks'
				   AND updated_at >= ?`,
				cutoff)
			if err != nil {
				return fmt.Errorf("read webhook payloads: %w", err)
			}
			defer rows.Close()

			var all []change
			for rows.Next() {
				var id, data string
				var when sql.NullTime
				if err := rows.Scan(&id, &data, &when); err != nil {
					return fmt.Errorf("scan webhook row: %w", err)
				}
				whenStr := ""
				if when.Valid {
					whenStr = when.Time.UTC().Format(time.RFC3339)
				}
				extractChangesFromPayload([]byte(data), id, whenStr, &all)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterate webhook payloads: %w", err)
			}

			if groupBy == "" {
				return flags.printJSON(cmd, all)
			}

			counts := map[string]int{}
			for _, c := range all {
				var key string
				switch groupBy {
				case "table":
					key = c.Table
				case "field":
					key = c.Field
				case "actor":
					key = c.Actor
				}
				if key == "" {
					key = "(unknown)"
				}
				counts[key]++
			}

			type kv struct {
				Key   string `json:"key"`
				Count int    `json:"count"`
			}
			out := make([]kv, 0, len(counts))
			for k, v := range counts {
				out = append(out, kv{Key: k, Count: v})
			}
			sort.Slice(out, func(i, j int) bool {
				if out[i].Count != out[j].Count {
					return out[i].Count > out[j].Count
				}
				return out[i].Key < out[j].Key
			})
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&since, "since", "7d", "Time window (7d, 24h, 2w, 1h30m, etc.)")
	cmd.Flags().StringVar(&groupBy, "group-by", "", "Group results by: table, field, actor")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/airtable-pp-cli/data.db)")
	return cmd
}

// extractChangesFromPayload best-effort-walks an Airtable webhook payload
// JSON and emits one change record per (table, field, actor) tuple
// observed. The shape varies by webhook spec; missing fields collapse to
// "" so the caller can attribute counts under "(unknown)".
func extractChangesFromPayload(data []byte, id, when string, out *[]change) {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return
	}
	actor := ""
	if a, ok := obj["actionMetadata"].(map[string]any); ok {
		if src, ok := a["source"].(string); ok {
			actor = src
		}
	}
	if actor == "" {
		if u, ok := obj["user"].(map[string]any); ok {
			if id, ok := u["id"].(string); ok {
				actor = id
			}
		}
	}
	if changed, ok := obj["changedTablesById"].(map[string]any); ok {
		for tableID, raw := range changed {
			tbl, _ := raw.(map[string]any)
			emitFieldEvents(tableID, tbl, actor, when, id, out)
		}
		return
	}
	// Fallback: emit a single bare event so the row counts as a change.
	*out = append(*out, change{Actor: actor, When: when, Source: id})
}

type change struct {
	Table  string `json:"table,omitempty"`
	Field  string `json:"field,omitempty"`
	Actor  string `json:"actor,omitempty"`
	When   string `json:"when,omitempty"`
	Source string `json:"source,omitempty"`
}

func emitFieldEvents(tableID string, tbl map[string]any, actor, when, source string, out *[]change) {
	if tbl == nil {
		*out = append(*out, change{Table: tableID, Actor: actor, When: when, Source: source})
		return
	}
	if recs, ok := tbl["changedRecordsById"].(map[string]any); ok {
		for _, rraw := range recs {
			rec, _ := rraw.(map[string]any)
			if cur, ok := rec["current"].(map[string]any); ok {
				if cellVals, ok := cur["cellValuesByFieldId"].(map[string]any); ok {
					for fieldID := range cellVals {
						*out = append(*out, change{Table: tableID, Field: fieldID, Actor: actor, When: when, Source: source})
					}
					continue
				}
			}
			*out = append(*out, change{Table: tableID, Actor: actor, When: when, Source: source})
		}
		return
	}
	*out = append(*out, change{Table: tableID, Actor: actor, When: when, Source: source})
}
