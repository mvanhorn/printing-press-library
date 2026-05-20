// Copyright 2026 bk20260126-code. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type snapshotDetail struct {
	Name         string           `json:"name"`
	ElementCount int              `json:"elementCount"`
	CreatedAt    string           `json:"createdAt"`
	Elements     []map[string]any `json:"elements,omitempty"`
}

type diffResult struct {
	Snapshot1 string              `json:"snapshot1"`
	Snapshot2 string              `json:"snapshot2"`
	Added     []map[string]any    `json:"added"`
	Removed   []map[string]any    `json:"removed"`
	Changed   []elementChangePair `json:"changed"`
	Unchanged int                 `json:"unchanged"`
	Summary   string              `json:"summary"`
}

type elementChangePair struct {
	ID     string         `json:"id"`
	Before map[string]any `json:"before"`
	After  map[string]any `json:"after"`
	Fields []string       `json:"changedFields"`
}

func newDiffCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff <snapshot1> <snapshot2>",
		Short: "Compare two canvas snapshots and show element-level changes.",
		Long: `Compare two named canvas snapshots and report which elements were added, removed, or modified.

Requires the canvas server to be running. Both snapshots must have been saved with 'snapshots create'.`,
		Example: strings.Trim(`
  excalidraw-mcp-pp-cli diff v1 v2
  excalidraw-mcp-pp-cli diff before-refactor after-refactor --json
  excalidraw-mcp-pp-cli diff pre-ai post-ai --json --select added,removed,summary`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			snap1Name := args[0]
			snap2Name := args[1]

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Fetch both snapshots
			data1, err := c.Get(fmt.Sprintf("/api/snapshots/%s", snap1Name), nil)
			if err != nil {
				return fmt.Errorf("fetching snapshot %q: %w", snap1Name, err)
			}
			data2, err := c.Get(fmt.Sprintf("/api/snapshots/%s", snap2Name), nil)
			if err != nil {
				return fmt.Errorf("fetching snapshot %q: %w", snap2Name, err)
			}

			var snap1, snap2 snapshotDetail
			if err := json.Unmarshal(data1, &snap1); err != nil {
				// try unwrapping
				var wrapper struct {
					Snapshot snapshotDetail `json:"snapshot"`
				}
				if err2 := json.Unmarshal(data1, &wrapper); err2 == nil {
					snap1 = wrapper.Snapshot
				}
			}
			if err := json.Unmarshal(data2, &snap2); err != nil {
				var wrapper struct {
					Snapshot snapshotDetail `json:"snapshot"`
				}
				if err2 := json.Unmarshal(data2, &wrapper); err2 == nil {
					snap2 = wrapper.Snapshot
				}
			}

			// Build ID-keyed maps for comparison
			map1 := make(map[string]map[string]any)
			map2 := make(map[string]map[string]any)
			for _, el := range snap1.Elements {
				if id, ok := el["id"].(string); ok {
					map1[id] = el
				}
			}
			for _, el := range snap2.Elements {
				if id, ok := el["id"].(string); ok {
					map2[id] = el
				}
			}

			var result diffResult
			result.Snapshot1 = snap1Name
			result.Snapshot2 = snap2Name
			result.Added = []map[string]any{}
			result.Removed = []map[string]any{}
			result.Changed = []elementChangePair{}

			// Find removed (in snap1, not in snap2)
			for id, el := range map1 {
				if _, exists := map2[id]; !exists {
					result.Removed = append(result.Removed, el)
				}
			}

			// Find added and changed
			for id, el2 := range map2 {
				el1, existed := map1[id]
				if !existed {
					result.Added = append(result.Added, el2)
					continue
				}
				// Check for changes
				var changedFields []string
				for key, v2 := range el2 {
					v1, ok := el1[key]
					if !ok {
						changedFields = append(changedFields, key)
						continue
					}
					b1, _ := json.Marshal(v1)
					b2, _ := json.Marshal(v2)
					if string(b1) != string(b2) {
						changedFields = append(changedFields, key)
					}
				}
				if len(changedFields) > 0 {
					result.Changed = append(result.Changed, elementChangePair{
						ID:     id,
						Before: el1,
						After:  el2,
						Fields: changedFields,
					})
				} else {
					result.Unchanged++
				}
			}

			result.Summary = fmt.Sprintf("+%d added, -%d removed, ~%d changed, %d unchanged",
				len(result.Added), len(result.Removed), len(result.Changed), result.Unchanged)

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				raw := json.RawMessage(jsonMarshalAny(result))
				if flags.selectFields != "" {
					raw = filterFields(raw, flags.selectFields)
				}
				return printOutput(cmd.OutOrStdout(), raw, true)
			}

			// Human output
			fmt.Fprintf(cmd.OutOrStdout(), "Diff: %s → %s\n", snap1Name, snap2Name)
			fmt.Fprintln(cmd.OutOrStdout(), result.Summary)
			if len(result.Added) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "\nAdded:")
				for _, el := range result.Added {
					fmt.Fprintf(cmd.OutOrStdout(), "  + %s (%s)\n", el["id"], el["type"])
				}
			}
			if len(result.Removed) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "\nRemoved:")
				for _, el := range result.Removed {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s (%s)\n", el["id"], el["type"])
				}
			}
			if len(result.Changed) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "\nChanged:")
				for _, pair := range result.Changed {
					fmt.Fprintf(cmd.OutOrStdout(), "  ~ %s [%s]\n", pair.ID, strings.Join(pair.Fields, ", "))
				}
			}
			return nil
		},
	}
	return cmd
}

func jsonMarshalAny(v any) []byte {
	b, _ := json.MarshalIndent(v, "", "  ")
	return b
}
