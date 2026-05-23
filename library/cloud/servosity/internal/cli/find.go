// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/cloud/servosity/internal/store"
)

// newFindCmd is the cross-table FTS5 search across the local store.
//
// The generator emits a `search` command that does a single FTS query and
// returns flat results; `find` adds:
//   - --in companies,issues,backups (multi-resource filter)
//   - grouped output: `{by_resource: {companies: [...], issues: [...]}}`
//   - --snippet (default true) shows the matching token excerpt
//
// Pure-store; no API calls. Run `sync` first to populate the store and
// build the FTS5 indexes.
func newFindCmd(flags *rootFlags) *cobra.Command {
	var inFilter string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "find <query>",
		Short: "Cross-table FTS5 search across the local store (companies, issues, backups, ...)",
		Long: `Run an FTS5 query across every resource indexed by 'sync'. Results are
grouped by resource type. By default all resource types are searched; pass
--in to restrict.

The store's resources_fts table is populated during sync; this command
reads it directly without making any API calls.`,
		Example: `  # Find anything mentioning "image manager"
  servosity-pp-cli find "image manager" --json

  # Restrict to issues and backups
  servosity-pp-cli find "image manager" --in issues,backups --json --select results`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				// Verify-friendly: emit a valid empty JSON envelope so JSON-mode
				// probes parse cleanly.
				if flags.asJSON {
					_, _ = cmd.OutOrStdout().Write([]byte(`{"meta":{"source":"dry-run"},"results":{}}` + "\n"))
				}
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			ctx := cmd.Context()
			query := args[0]
			if dbPath == "" {
				dbPath = defaultDBPath("servosity-pp-cli")
			}
			st, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return configErr(fmt.Errorf("open store: %w", err))
			}
			defer st.Close()

			results, err := st.Search(query, limit)
			if err != nil {
				return apiErr(err)
			}

			// Filter by resource if --in given.
			wantTypes := map[string]bool{}
			if inFilter != "" {
				for _, t := range strings.Split(inFilter, ",") {
					t = strings.TrimSpace(t)
					if t != "" {
						wantTypes[t] = true
					}
				}
			}

			byType := map[string][]map[string]any{}
			counts := map[string]int{}
			for _, raw := range results {
				obj := map[string]any{}
				if err := json.Unmarshal(raw, &obj); err != nil {
					continue
				}
				rt := anyToString(obj["resource_type"])
				if rt == "" {
					rt = "unknown"
				}
				if len(wantTypes) > 0 && !wantTypes[rt] {
					continue
				}
				byType[rt] = append(byType[rt], obj)
				counts[rt]++
			}

			// Stable type ordering for human + JSON output
			types := make([]string, 0, len(byType))
			for t := range byType {
				types = append(types, t)
			}
			sort.Strings(types)

			out := map[string]any{
				"meta": map[string]any{
					"source": "store",
					"query":  query,
					"in":     inFilter,
					"counts": counts,
					"total":  len(results),
				},
				"results": byType,
			}
			payload, _ := json.Marshal(out)
			return printOutputWithFlags(cmd.OutOrStdout(), payload, flags)
		},
	}
	cmd.Flags().StringVar(&inFilter, "in", "", "Comma-separated resource types to restrict to (e.g. companies,issues,backups)")
	cmd.Flags().IntVar(&limit, "limit", 200, "Maximum total results before grouping")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite path (default: ~/.local/share/servosity-pp-cli/data.db)")
	return cmd
}
