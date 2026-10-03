// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source local
package cli

import (
	"fmt"
	hw "github.com/mvanhorn/printing-press-library/library/travel/hostelworld/internal/hostelworld"
	"github.com/spf13/cobra"
)

func newNovelHostelsSavedCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{Use: "saved", Short: "Read saved planning evidence offline; dated prices are marked stale", Example: "  " + "hostelworld-pp-cli hostels saved --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": ""}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 0 || limit < 1 || limit > 50 {
			return usageErr(fmt.Errorf("saved takes no positional arguments; --limit must be 1–50"))
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		db, guard, err := OpenPlanningReadOnly(ctx, "")
		if err != nil {
			return err
		}
		defer guard.Close()
		if db == nil {
			if err := guard.Check(); err != nil {
				return err
			}
			return flags.printJSON(cmd, map[string]any{"results": []any{}, "freshness": "stale", "notice": "save live evidence with hostels inspect/offers --save"})
		}
		defer db.Close()
		rows, err := db.DB().QueryContext(ctx, "SELECT data FROM resources WHERE resource_type='planning_snapshot' ORDER BY rowid DESC LIMIT ?", limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		out := []any{}
		for rows.Next() {
			var data string
			if err := rows.Scan(&data); err != nil {
				return err
			}
			v, err := hw.Decode([]byte(data))
			if err != nil {
				return fmt.Errorf("saved planning snapshot is invalid")
			}
			v["freshness"] = "stale"
			out = append(out, v)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		if err := guard.Check(); err != nil {
			return err
		}
		return flags.printJSON(cmd, map[string]any{"results": out, "freshness": "stale", "notice": "saved dated prices are observations; re-run a live command to refresh", "retention_limit": 200})
	}}
	decoratePlanning(cmd, flags)
	cmd.Annotations["pp:data-source"] = "local"
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum saved snapshots to return, 1–50")
	return cmd
}
