// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/mvanhorn/printing-press-library/library/ai/cartesia/internal/store"
	"github.com/spf13/cobra"
)

// changelogEntry is one row in the diff between the two most recent voice snapshots.
type changelogEntry struct {
	VoiceID string         `json:"voice_id"`
	Status  string         `json:"status"` // added, removed, changed
	Changes map[string]any `json:"changes,omitempty"`
}

// newVoicesChangelogCmd registers `voices changelog [--since <window>]`.
// Compares the most recent voice snapshot timestamp against the prior
// snapshot, emitting added, removed, and changed entries per voice.
func newVoicesChangelogCmd(flags *rootFlags) *cobra.Command {
	var since string
	var dbPath string
	cmd := &cobra.Command{
		Use:         "changelog",
		Short:       "Diff the two most recent voice snapshots",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     `  cartesia-pp-cli voices changelog --since 7d --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("cartesia-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			// Pick the two most recent distinct snapshot timestamps in the window.
			tsQ := `SELECT DISTINCT snapshot_at FROM voices_snapshot`
			argv := []any{}
			if since != "" {
				t, err := parseSince(since)
				if err != nil {
					return fmt.Errorf("--since: %w", err)
				}
				tsQ += " WHERE snapshot_at >= ?"
				argv = append(argv, t.UTC().Format("2006-01-02 15:04:05"))
			}
			tsQ += " ORDER BY snapshot_at DESC LIMIT 2"
			rows, err := db.DB().QueryContext(cmd.Context(), tsQ, argv...)
			if err != nil {
				return fmt.Errorf("query timestamps: %w", err)
			}
			var stamps []string
			for rows.Next() {
				var s string
				if err := rows.Scan(&s); err != nil {
					rows.Close()
					return fmt.Errorf("scan ts: %w", err)
				}
				stamps = append(stamps, s)
			}
			// PATCH(greptile #560): surface mid-iteration errors instead of
			// truncating to <2 stamps and returning the misleading "fewer
			// than two snapshots available" note. Same fix shape as
			// agents_since.go and loadSnapshot below.
			if err := rows.Err(); err != nil {
				rows.Close()
				return fmt.Errorf("iterate snapshot timestamps: %w", err)
			}
			rows.Close()

			result := map[string]any{
				"entries": []changelogEntry{},
			}
			if len(stamps) < 2 {
				result["note"] = "fewer than two snapshots available"
				return flags.printJSON(cmd, result)
			}
			current, err := loadSnapshot(db, stamps[0])
			if err != nil {
				return err
			}
			prior, err := loadSnapshot(db, stamps[1])
			if err != nil {
				return err
			}

			entries := []changelogEntry{}
			for id, cur := range current {
				prev, ok := prior[id]
				if !ok {
					entries = append(entries, changelogEntry{VoiceID: id, Status: "added"})
					continue
				}
				if !reflect.DeepEqual(cur, prev) {
					entries = append(entries, changelogEntry{
						VoiceID: id,
						Status:  "changed",
						Changes: map[string]any{"from": prev, "to": cur},
					})
				}
			}
			for id := range prior {
				if _, ok := current[id]; !ok {
					entries = append(entries, changelogEntry{VoiceID: id, Status: "removed"})
				}
			}
			result["entries"] = entries
			result["current_snapshot_at"] = stamps[0]
			result["prior_snapshot_at"] = stamps[1]
			return flags.printJSON(cmd, result)
		},
	}

	cmd.Flags().StringVar(&since, "since", "", "Only consider snapshots newer than this window")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func loadSnapshot(db *store.Store, ts string) (map[string]map[string]any, error) {
	rows, err := db.DB().Query(`SELECT voice_id, data FROM voices_snapshot WHERE snapshot_at = ?`, ts)
	if err != nil {
		return nil, fmt.Errorf("query snapshot: %w", err)
	}
	defer rows.Close()
	out := map[string]map[string]any{}
	for rows.Next() {
		var id, data string
		if err := rows.Scan(&id, &data); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(data), &obj); err != nil {
			obj = map[string]any{"_raw": data}
		}
		out[id] = obj
	}
	return out, rows.Err()
}
