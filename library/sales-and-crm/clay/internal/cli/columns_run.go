// Copyright 2026 Ade Amos and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: columns run.
//
// Clay runs ACTION columns (enrichments, HTTP API). Formula columns have no run
// endpoint of their own: they are derived, and recompute when an upstream
// dependency runs. So to refresh a formula, run the action column it reads from.
//
// Verified against the app's own traffic: the UI sends no forceRun field, and a
// plain run of the dependency recomputes downstream formulas within seconds.
// Tables also default to AUTO_RUN_MODE "keep_existing", which is why editing a
// formula alone leaves existing cells stale and a status poll still reports
// "settled" — nothing was ever queued.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type runResult struct {
	TableID  string   `json:"tableId"`
	FieldIDs []string `json:"fieldIds"`
	ViewID   string   `json:"viewId,omitempty"`
	Records  []string `json:"recordIds,omitempty"`
	NumTop   int      `json:"numRecords,omitempty"`
	RunMode  string   `json:"runMode,omitempty"`
	Started  bool     `json:"started"`
}

func newNovelColumnsRunCmd(flags *rootFlags) *cobra.Command {
	var flagWorkspace, flagView, flagRecords string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "run <tableId> <fieldId>...",
		Short: "Trigger a run for one or more columns, recomputing their cells.",
		Long: "Runs action columns (enrichments, HTTP API calls).\n\n" +
			"Formula columns have no run of their own — they are derived and recompute when an\n" +
			"upstream dependency runs. To refresh a formula, run the action column it reads from,\n" +
			"then read the formula back with 'tables rows --no-cache'.\n\n" +
			"This SPENDS CREDITS on enrichment columns. Use --dry-run first.",
		Example: "  clay-pp-cli columns run t_example123 f_example456 --limit 10\n" +
			"  clay-pp-cli columns run t_example123 f_example456 --records r_example789",
		Annotations: map[string]string{
			"mcp:read-only": "false",
			"pp:happy-args": "<tableId>=t_example;<fieldId>=f_example;--workspace=1;--dry-run",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<tableId> and at least one <fieldId> are required"))
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "columns run")
			}
			ws, err := resolveWorkspace(flagWorkspace)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			tableID, fieldIDs := args[0], args[1:]

			res := runResult{TableID: tableID, FieldIDs: fieldIDs}
			runRecords := map[string]any{}

			if strings.TrimSpace(flagRecords) != "" {
				// Explicit record ids: run exactly these cells.
				ids := []string{}
				for _, r := range strings.Split(flagRecords, ",") {
					if r = strings.TrimSpace(r); r != "" {
						ids = append(ids, r)
					}
				}
				if len(ids) == 0 {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--records was set but contained no record ids"))
				}
				res.Records = ids
				runRecords["recordIds"] = ids
			} else {
				// View-scoped: run the top N records of a view.
				view := flagView
				if view == "" {
					tbl, tErr := fetchTable(ctx, c, ws, tableID)
					if tErr != nil {
						return tErr
					}
					view = tbl.FirstViewID
				}
				if view == "" {
					return usageErr(fmt.Errorf("no view id available; pass --view or --records"))
				}
				res.ViewID, res.NumTop = view, flagLimit
				runRecords["viewIdTopRecords"] = map[string]any{
					"viewId": view, "numRecords": flagLimit,
				}
			}

			body := map[string]any{
				"fieldIds":   fieldIDs,
				"runRecords": runRecords,
				"callerName": "clay-pp-cli",
			}
			raw, status, err := c.Patch(ctx, fmt.Sprintf("/workspaces/%s/tables/%s/run", ws, tableID), body)
			if err != nil {
				return fmt.Errorf("triggering run: %w", err)
			}
			if status >= 300 {
				return fmt.Errorf("triggering run: HTTP %d", status)
			}
			var out struct {
				RunMode string `json:"runMode"`
			}
			_ = json.Unmarshal(raw, &out)
			res.RunMode, res.Started = out.RunMode, true

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "started %s run for %d column(s) on %s\n",
				res.RunMode, len(fieldIDs), tableID)
			fmt.Fprintf(cmd.OutOrStdout(), "poll with: clay-pp-cli watch %s\n", tableID)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagWorkspace, "workspace", "", "Clay workspace id (or set CLAY_WORKSPACE_ID)")
	cmd.Flags().StringVar(&flagView, "view", "", "View id (gv_...); defaults to the table's first view")
	cmd.Flags().StringVar(&flagRecords, "records", "", "Comma-separated record ids (r_...) to run instead of the view's top rows")
	cmd.Flags().IntVar(&flagLimit, "limit", 25, "How many of the view's top records to run")
	return cmd
}
