// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newClipsDirectChildrenCountCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "direct-children-count <clip_ids>",
		Short:       "Count direct children of clip(s)",
		Long:        "Count direct children (extends/covers) of clip(s). Pass one or more clip IDs, comma-separated.",
		Example:     "  suno-pp-cli clips direct-children-count 550e8400-e29b-41d4-a716-446655440000,7d869de4-9476-4a4d-a6f2-c0eec968a3e2",
		Annotations: map[string]string{"pp:endpoint": "clips.direct-children-count", "pp:method": "GET", "pp:path": "/api/clips/direct_children_count", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/api/clips/direct_children_count"
			// The endpoint wants the clip id under the query key `clip_id`.
			params := map[string]string{"clip_id": args[0]}
			data, prov, err := resolveRead(cmd.Context(), c, flags, "clips", false, path, params, nil, cmd.ErrOrStderr())
			if err != nil {
				return classifyAPIError(err, flags)
			}
			data = extractResponseData(data)

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var countItems []json.RawMessage
				if json.Unmarshal(data, &countItems) != nil {
					countItems = []json.RawMessage{data}
				}
				printProvenance(cmd, len(countItems), prov)
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				filtered := data
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				wrapped, wrapErr := wrapWithProvenance(filtered, prov)
				if wrapErr != nil {
					return wrapErr
				}
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var items []map[string]any
				if json.Unmarshal(data, &items) == nil && len(items) > 0 {
					if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
						return err
					}
					if len(items) >= 25 {
						fmt.Fprintf(os.Stderr, "\nShowing %d results. To narrow: add --limit, --json --select, or filter flags.\n", len(items))
					}
					return nil
				}
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}

	return cmd
}
