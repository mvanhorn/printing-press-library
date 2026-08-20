// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelTasksLineageCmd(flags *rootFlags) *cobra.Command {
	var flagRunID string

	cmd := &cobra.Command{
		Use:   "lineage [run_id]",
		Short: "Print the offline previous_interaction_id follow-up chain for a run.",
		Example: strings.Trim(`
  parallel-pp-cli tasks lineage trun_demo --json --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}

			runID := strings.TrimSpace(flagRunID)
			if runID == "" && len(args) > 0 {
				runID = strings.TrimSpace(args[0])
			}
			if runID == "" && !hasChangedLocalFlags(cmd) {
				return cmd.Help()
			}
			if runID == "" {
				return usageErr(fmt.Errorf("run_id is required as a positional argument or --run-id"))
			}

			db, err := openStoreForRead(cmd.Context(), "parallel-pp-cli")
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			if db == nil {
				out := map[string]any{
					"run_id": runID,
					"chain":  []any{},
					"note":   "no local store",
				}
				return flags.printJSON(cmd, out)
			}
			defer db.Close()

			hintIfUnsynced(cmd, db, "tasks")
			hintIfStale(cmd, db, "tasks", flags.maxAge)

			chain, err := db.WalkTaskLineage(runID)
			if err != nil {
				return fmt.Errorf("tasks lineage: %w", err)
			}
			if len(chain) == 0 {
				return notFoundErr(fmt.Errorf("no local lineage for run_id %q; sync tasks or provide a known run id", runID))
			}

			return flags.printJSON(cmd, map[string]any{
				"run_id": runID,
				"chain":  chain,
			})
		},
	}
	cmd.Flags().StringVar(&flagRunID, "run-id", "", "Task run id to trace")
	return cmd
}
