// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/ai/notebooklm/internal/store"
	"github.com/spf13/cobra"
)

func newStatsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Summarize local cache row counts and sync freshness using SQL aggregation",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		Example: `  notebooklm-pp-cli sync --json
  notebooklm-pp-cli stats --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSON(map[string]any{"notebooks": 0})
			}
			st, err := store.Open(dbPath)
			if err != nil {
				return configErr(err)
			}
			defer st.Close()
			rows, err := st.ReadOnlyQuery(cmd.Context(),
				`SELECT COUNT(*) AS notebook_count FROM notebooks`, 10)
			if err != nil {
				return apiErr(err)
			}
			syncRows, err := st.ReadOnlyQuery(cmd.Context(),
				`SELECT resource_type, COALESCE(total_count,0) AS total_count, COUNT(*) AS groups FROM sync_state GROUP BY resource_type`, 50)
			if err != nil {
				return apiErr(err)
			}
			payload := map[string]any{
				"notebooks": rows,
				"sync":      syncRows,
			}
			if flags.asJSON {
				return printJSON(payload)
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "metric\tvalue\n")
			fmt.Fprintf(w, "sync_groups\t%d\n", len(syncRows))
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite cache path")
	return cmd
}
