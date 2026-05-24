// Copyright 2026 joelsephus. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/airtable/internal/store"
	"github.com/spf13/cobra"
)

func newTraverseCmd(flags *rootFlags) *cobra.Command {
	var depth int
	var dbPath string
	var pretty bool

	cmd := &cobra.Command{
		Use:         "traverse <baseId> <recordId>",
		Short:       "Walk linked-record relationships from a starting record over the local mirror",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Recursive walk over synced records following linked-record array fields.
Offline; reads the local SQLite mirror only.`,
		Example: strings.Trim(`
  # Walk linked records 2 hops deep
  airtable-pp-cli traverse appXXX recYYY --depth 2

  # Tree rendering
  airtable-pp-cli traverse appXXX recYYY --depth 3 --pretty

  # ndjson edges for piping
  airtable-pp-cli traverse appXXX recYYY --depth 2 --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("baseId and recordId are required\nUsage: %s <baseId> <recordId>", cmd.CommandPath()))
			}
			baseID, recordID := args[0], args[1]

			if depth <= 0 {
				depth = 2
			}

			if dbPath == "" {
				dbPath = defaultDBPath("airtable-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				// Actionable next-step on stderr; stdout stays clean (empty
				// array in JSON mode, just the root recordId in pretty mode)
				// so agents walking edges don't see a phantom self-loop.
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s for %s\nrun: airtable-pp-cli sync --resources records,webhooks --db %s\n", dbPath, recordID, dbPath)
				if flags.asJSON {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), recordID)
				}
				return nil
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'airtable-pp-cli sync' first.", err)
			}
			defer db.Close()

			type edge struct {
				From  string `json:"from"`
				To    string `json:"to"`
				Field string `json:"field,omitempty"`
				Depth int    `json:"depth"`
			}
			var edges []edge
			seen := map[string]bool{recordID: true}
			frontier := []string{recordID}
			for d := 1; d <= depth; d++ {
				next := []string{}
				for _, id := range frontier {
					var data string
					// PATCH(airtable-traverse-base-scope): scope the lookup to
					// the user-supplied baseID via parent_id so multi-base
					// mirrors don't return edges from unrelated bases that
					// happen to share a record-ID prefix.
					err := db.DB().QueryRowContext(cmd.Context(),
						`SELECT data FROM records WHERE id = ? AND parent_id = ?`, id, baseID).Scan(&data)
					if err != nil {
						continue
					}
					var obj map[string]any
					if err := json.Unmarshal([]byte(data), &obj); err != nil {
						continue
					}
					fields, _ := obj["fields"].(map[string]any)
					for fieldName, val := range fields {
						arr, ok := val.([]any)
						if !ok {
							continue
						}
						for _, item := range arr {
							s, ok := item.(string)
							if !ok {
								continue
							}
							// Heuristic: linked-record IDs in Airtable start with "rec"
							if !strings.HasPrefix(s, "rec") {
								continue
							}
							edges = append(edges, edge{From: id, To: s, Field: fieldName, Depth: d})
							if !seen[s] {
								seen[s] = true
								next = append(next, s)
							}
						}
					}
				}
				frontier = next
				if len(frontier) == 0 {
					break
				}
			}

			// Empty-result handling: JSON callers get a real empty array so
			// downstream agents/jq users can branch on length cleanly.
			// Pretty/human callers get the root recordId plus a "no linked
			// records" note, never a synthetic self-loop edge.
			if pretty && !flags.asJSON {
				w := cmd.OutOrStdout()
				fmt.Fprintf(w, "%s\n", recordID)
				if len(edges) == 0 {
					fmt.Fprintln(w, "  (no linked records)")
					return nil
				}
				for _, e := range edges {
					prefix := strings.Repeat("  ", e.Depth)
					fmt.Fprintf(w, "%s└─ %s (%s)\n", prefix, e.To, e.Field)
				}
				return nil
			}
			if edges == nil {
				edges = []edge{}
			}
			return flags.printJSON(cmd, edges)
		},
	}
	cmd.Flags().IntVar(&depth, "depth", 2, "Maximum traversal depth")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/airtable-pp-cli/data.db)")
	cmd.Flags().BoolVar(&pretty, "pretty", false, "Render as a tree")
	return cmd
}
