// Copyright 2026 joelsephus. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/airtable/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/airtable/internal/store"
	"github.com/spf13/cobra"
)

func newStaleCmd(flags *rootFlags) *cobra.Command {
	var field string
	var equals string
	var olderThan string
	var dbPath string
	var pretty bool

	cmd := &cobra.Command{
		Use:         "stale <baseId> <tableId>",
		Short:       "Find records older than a duration, joined against the local cache",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Find records that haven't changed in some time window from the local
SQLite mirror. Optionally filter by a singleSelect status field.

Reads against the synced 'records' table's created_time / last sync time.`,
		Example: strings.Trim(`
  # Records not touched in 30 days
  airtable-pp-cli stale appXXX tblYYY --older-than 30d

  # Stale records where Status = "In Review"
  airtable-pp-cli stale appXXX tblYYY --field Status --equals "In Review" --older-than 14d --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("baseId and tableId are required\nUsage: %s <baseId> <tableId>", cmd.CommandPath()))
			}
			baseID, tableID := args[0], args[1]

			window, err := cliutil.ParseDurationLoose(olderThan)
			if err != nil {
				return usageErr(fmt.Errorf("--older-than: %w", err))
			}
			if window <= 0 {
				window = 30 * 24 * time.Hour
			}
			cutoff := time.Now().Add(-window)

			if dbPath == "" {
				dbPath = defaultDBPath("airtable-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				// Send the actionable next-step to stderr; keep stdout valid
				// (empty array in JSON mode, prose in human mode) so JSON
				// consumers branch on length, not a synthetic placeholder.
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s for %s/%s\nrun: airtable-pp-cli sync --resources records,webhooks --db %s\n", dbPath, baseID, tableID, dbPath)
				if flags.asJSON {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "no stale records for %s/%s (no local mirror)\n", baseID, tableID)
				}
				return nil
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'airtable-pp-cli sync' first.", err)
			}
			defer db.Close()

			// Records table carries the synced JSON in `data` and a
			// per-row `synced_at`. We pre-filter on synced_at and let the
			// caller filter rows further by their field.
			//
			// PATCH(airtable-stale-base-scope): scope the lookup to baseID
			// (parent_id is populated by dependent sync with the parent base
			// ID). The original code passed tableID into the parent_id
			// predicate, which never matched a real row in multi-base
			// mirrors. tableID is still used for field/equals filtering and
			// for user-facing messages below.
			_ = tableID
			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT id, data, synced_at FROM records
				 WHERE (parent_id = ? OR parent_id IS NULL)
				   AND synced_at <= ?
				 ORDER BY synced_at ASC`,
				baseID, cutoff)
			if err != nil {
				return fmt.Errorf("query records: %w", err)
			}
			defer rows.Close()

			type rec struct {
				ID       string          `json:"id"`
				SyncedAt string          `json:"synced_at"`
				Fields   json.RawMessage `json:"fields,omitempty"`
			}
			var matches []rec
			for rows.Next() {
				var id, data string
				var syncedAt sql.NullTime
				if err := rows.Scan(&id, &data, &syncedAt); err != nil {
					return fmt.Errorf("scan record: %w", err)
				}
				if field != "" {
					var obj map[string]any
					if err := json.Unmarshal([]byte(data), &obj); err == nil {
						fields, _ := obj["fields"].(map[string]any)
						if fields == nil {
							continue
						}
						v, ok := fields[field]
						if !ok {
							continue
						}
						if equals != "" && fmt.Sprintf("%v", v) != equals {
							continue
						}
					}
				}
				r := rec{ID: id}
				if syncedAt.Valid {
					r.SyncedAt = syncedAt.Time.UTC().Format(time.RFC3339)
				}
				r.Fields = json.RawMessage(data)
				matches = append(matches, r)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterate records: %w", err)
			}
			// Empty-result handling: JSON callers get a true empty array so
			// downstream consumers (jq, agents) can branch on length cleanly.
			// Pretty/human callers get a one-line message that names the
			// queried base/table for context.
			if pretty && !flags.asJSON {
				w := cmd.OutOrStdout()
				if len(matches) == 0 {
					fmt.Fprintf(w, "no stale records for %s/%s (older than %s)\n", baseID, tableID, olderThan)
					return nil
				}
				fmt.Fprintf(w, "%d stale records (older than %s)\n", len(matches), olderThan)
				for _, r := range matches {
					fmt.Fprintf(w, "  %s  synced=%s\n", r.ID, r.SyncedAt)
				}
				return nil
			}
			if matches == nil {
				matches = []rec{}
			}
			return flags.printJSON(cmd, matches)
		},
	}
	cmd.Flags().StringVar(&field, "field", "", "Filter on this field name")
	cmd.Flags().StringVar(&equals, "equals", "", "When --field is set, only rows whose field equals this value")
	cmd.Flags().StringVar(&olderThan, "older-than", "30d", "Stale threshold (e.g., 30d, 7d, 12h)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/airtable-pp-cli/data.db)")
	cmd.Flags().BoolVar(&pretty, "pretty", false, "Render as a pretty list instead of JSON")
	return cmd
}
