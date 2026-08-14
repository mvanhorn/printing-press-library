// Copyright 2026 srijits and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

func newDuplicatesApplyCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use: "apply <plan-id>", Short: "Apply a reviewed duplicate plan", Example: "  raindrop-pp-cli duplicates apply 1 --dry-run --agent", Args: cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:destructive": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			planID, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return usageErr(fmt.Errorf("invalid plan id %q", args[0]))
			}
			db, _, err := openNovelStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			var payload, status string
			if err := db.DB().QueryRowContext(cmd.Context(), `SELECT payload,status FROM cleanup_plans WHERE id=? AND kind='duplicates'`, planID).Scan(&payload, &status); err != nil {
				return fmt.Errorf("loading plan: %w", err)
			}
			if status != "planned" {
				return fmt.Errorf("plan %d is %s", planID, status)
			}
			var groups []struct {
				Keep      string   `json:"keep"`
				Remove    []string `json:"remove"`
				MergeTags []string `json:"merge_tags"`
			}
			if err := json.Unmarshal([]byte(payload), &groups); err != nil {
				return err
			}
			if flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"plan_id": planID, "dry_run": true, "groups": groups}, flags)
			}
			if !flags.yes {
				return usageErr(fmt.Errorf("duplicate apply is destructive; rerun with --yes after reviewing 'duplicates plan' output"))
			}
			client, err := flags.newClient()
			if err != nil {
				return err
			}
			deleted := 0
			for _, group := range groups {
				if _, _, err := client.PutWithParams(cmd.Context(), "/raindrop/"+group.Keep, nil, map[string]any{"tags": group.MergeTags}); err != nil {
					return classifyAPIError(err, flags)
				}
				for _, id := range group.Remove {
					if _, _, err := client.DeleteWithParams(cmd.Context(), "/raindrop/"+id, nil); err != nil {
						return classifyDeleteError(err, flags)
					}
					deleted++
				}
			}
			_, err = db.DB().ExecContext(cmd.Context(), `UPDATE cleanup_plans SET status='applied',applied_at=? WHERE id=?`, time.Now().UTC().Format(time.RFC3339), planID)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"plan_id": planID, "status": "applied", "groups": len(groups), "deleted": deleted}, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path")
	return cmd
}
