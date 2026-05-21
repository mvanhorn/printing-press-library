// Copyright 2026 ioncom. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/framer/internal/store"
	"github.com/spf13/cobra"
)

type snapshotDiffEntry struct {
	Change       string `json:"change"`
	ResourceType string `json:"resource_type"`
	ID           string `json:"id"`
}

func newDiffCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "diff <snapshot_a> <snapshot_b>",
		Short: "Diff two snapshots to show what changed",
		Long: `Compares two snapshots and shows added, removed, and modified items.
Snapshot references can be numeric IDs or relative references like
'latest', 'latest~1', 'latest~2', etc.`,
		Example: strings.Trim(`
  # Compare two snapshots by ID
  framer-pp-cli diff 1 2

  # Compare the last two snapshots
  framer-pp-cli diff latest~1 latest

  # JSON output
  framer-pp-cli diff 1 2 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			if dbPath == "" {
				dbPath = defaultDBPath("framer-pp-cli")
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'framer-pp-cli sync' and 'framer-pp-cli snapshot' first.", err)
			}
			defer db.Close()

			rawDB := db.DB()

			snapshotA, err := resolveSnapshotRef(rawDB, args[0])
			if err != nil {
				return fmt.Errorf("resolving snapshot_a %q: %w", args[0], err)
			}
			snapshotB, err := resolveSnapshotRef(rawDB, args[1])
			if err != nil {
				return fmt.Errorf("resolving snapshot_b %q: %w", args[1], err)
			}

			dataA, err := loadSnapshotData(rawDB, snapshotA)
			if err != nil {
				return fmt.Errorf("loading snapshot %d: %w", snapshotA, err)
			}
			dataB, err := loadSnapshotData(rawDB, snapshotB)
			if err != nil {
				return fmt.Errorf("loading snapshot %d: %w", snapshotB, err)
			}

			// Index by composite key: resource_type + "/" + id
			indexA := indexByID(dataA)
			indexB := indexByID(dataB)

			var changes []snapshotDiffEntry

			// Added: in B but not A
			for key, item := range indexB {
				if _, ok := indexA[key]; !ok {
					changes = append(changes, snapshotDiffEntry{
						Change:       "added",
						ResourceType: itemResourceType(item),
						ID:           itemID(item),
					})
				}
			}

			// Removed: in A but not B
			for key, item := range indexA {
				if _, ok := indexB[key]; !ok {
					changes = append(changes, snapshotDiffEntry{
						Change:       "removed",
						ResourceType: itemResourceType(item),
						ID:           itemID(item),
					})
				}
			}

			// Modified: in both but data differs
			for key, itemA := range indexA {
				if itemB, ok := indexB[key]; ok {
					dataFieldA, _ := json.Marshal(itemA["data"])
					dataFieldB, _ := json.Marshal(itemB["data"])
					if string(dataFieldA) != string(dataFieldB) {
						changes = append(changes, snapshotDiffEntry{
							Change:       "modified",
							ResourceType: itemResourceType(itemA),
							ID:           itemID(itemA),
						})
					}
				}
			}

			if flags.asJSON {
				output := map[string]interface{}{
					"snapshot_a": snapshotA,
					"snapshot_b": snapshotB,
					"added":      countDiffChanges(changes, "added"),
					"removed":    countDiffChanges(changes, "removed"),
					"modified":   countDiffChanges(changes, "modified"),
					"changes":    changes,
				}
				return flags.printJSON(cmd, output)
			}

			if len(changes) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No differences between snapshot #%d and #%d\n", snapshotA, snapshotB)
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Diff: snapshot #%d -> #%d (%d changes)\n\n",
				snapshotA, snapshotB, len(changes))

			headers := []string{"CHANGE", "TYPE", "ID"}
			rows := make([][]string, 0, len(changes))
			for _, c := range changes {
				rows = append(rows, []string{c.Change, c.ResourceType, c.ID})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/framer-pp-cli/data.db)")

	return cmd
}

// resolveSnapshotRef converts a snapshot reference (numeric ID, "latest",
// "latest~N") into a numeric snapshot ID.
func resolveSnapshotRef(db *sql.DB, ref string) (int64, error) {
	ref = strings.TrimSpace(ref)

	// Numeric ID
	if id, err := strconv.ParseInt(ref, 10, 64); err == nil {
		var exists int
		if err := db.QueryRow(`SELECT COUNT(*) FROM snapshots WHERE id = ?`, id).Scan(&exists); err != nil {
			return 0, fmt.Errorf("checking snapshot: %w", err)
		}
		if exists == 0 {
			return 0, fmt.Errorf("snapshot #%d not found", id)
		}
		return id, nil
	}

	// latest or latest~N
	if strings.HasPrefix(ref, "latest") {
		offset := 0
		if strings.Contains(ref, "~") {
			parts := strings.SplitN(ref, "~", 2)
			n, err := strconv.Atoi(parts[1])
			if err != nil {
				return 0, fmt.Errorf("invalid offset in %q: %w", ref, err)
			}
			offset = n
		}
		var id int64
		err := db.QueryRow(
			`SELECT id FROM snapshots ORDER BY id DESC LIMIT 1 OFFSET ?`, offset,
		).Scan(&id)
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("no snapshot at offset %d from latest", offset)
		}
		if err != nil {
			return 0, fmt.Errorf("resolving %q: %w", ref, err)
		}
		return id, nil
	}

	return 0, fmt.Errorf("unrecognized snapshot reference %q (use a numeric ID, 'latest', or 'latest~N')", ref)
}

func loadSnapshotData(db *sql.DB, id int64) ([]map[string]interface{}, error) {
	var dataStr string
	err := db.QueryRow(`SELECT data FROM snapshots WHERE id = ?`, id).Scan(&dataStr)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("snapshot #%d not found", id)
	}
	if err != nil {
		return nil, err
	}

	var items []map[string]interface{}
	if err := json.Unmarshal([]byte(dataStr), &items); err != nil {
		return nil, fmt.Errorf("deserializing snapshot data: %w", err)
	}
	return items, nil
}

func indexByID(items []map[string]interface{}) map[string]map[string]interface{} {
	idx := make(map[string]map[string]interface{}, len(items))
	for _, item := range items {
		rt := itemResourceType(item)
		id := itemID(item)
		key := rt + "/" + id
		idx[key] = item
	}
	return idx
}

func itemResourceType(item map[string]interface{}) string {
	if v, ok := item["resource_type"]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func itemID(item map[string]interface{}) string {
	if v, ok := item["id"]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func countDiffChanges(changes []snapshotDiffEntry, changeType string) int {
	count := 0
	for _, c := range changes {
		if c.Change == changeType {
			count++
		}
	}
	return count
}
