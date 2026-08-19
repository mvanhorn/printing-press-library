// Copyright 2026 Ade Amos and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

type columnErrorReport struct {
	FieldID  string         `json:"fieldId"`
	Column   string         `json:"column"`
	Type     string         `json:"type"`
	Statuses map[string]int `json:"statuses"`
	Failed   int            `json:"failedCount"`
}

type errorsView struct {
	TableID   string              `json:"tableId"`
	TableName string              `json:"tableName"`
	Columns   []columnErrorReport `json:"columns"`
	Failed    int                 `json:"totalFailed"`
}

// nonFailureStatuses are run statuses that do not indicate a problem.
var nonFailureStatuses = map[string]bool{
	"SUCCESS": true, "QUEUED": true, "RUNNING": true, "EMPTY": true, "PENDING": true,
}

func newNovelErrorsCmd(flags *rootFlags) *cobra.Command {
	var flagWorkspace string
	var flagAll bool
	cmd := &cobra.Command{
		Use:   "errors <tableId>",
		Short: "Report why columns failed, combining Clay run statuses with per-column detail.",
		Long: "Use this command when a column is empty and you need to know whether the run failed or never ran.\n" +
			"Do NOT use it to re-run enrichment; trigger runs in the Clay app.",
		Example: "  clay-pp-cli errors t_abc123 --workspace 1234567 --agent",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "<tableId>=t_example;--workspace=1;--dry-run",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "errors")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<tableId> is required"))
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
			tbl, err := fetchTable(ctx, c, ws, args[0])
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			rs, err := fetchRunStatus(ctx, c, ws, args[0])
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			byID := indexByID(tbl.Fields)
			view := errorsView{TableID: tbl.ID, TableName: tbl.Name, Columns: make([]columnErrorReport, 0)}

			ids := make([]string, 0, len(rs.StatusCountsByField))
			for id := range rs.StatusCountsByField {
				ids = append(ids, id)
			}
			sort.Strings(ids)

			for _, id := range ids {
				counts := rs.StatusCountsByField[id]
				rep := columnErrorReport{FieldID: id, Statuses: map[string]int{}}
				if f, ok := byID[id]; ok {
					rep.Column, rep.Type = f.Name, f.Type
				}
				for _, sc := range counts {
					rep.Statuses[sc.Status] = sc.Count
					if !nonFailureStatuses[sc.Status] {
						rep.Failed += sc.Count
					}
				}
				if rep.Failed == 0 && !flagAll {
					continue
				}
				view.Failed += rep.Failed
				view.Columns = append(view.Columns, rep)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
					return err
				}
			} else if len(view.Columns) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: no column errors\n", tbl.Name)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "%s (%s)\n", view.TableName, view.TableID)
				for _, col := range view.Columns {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-28s %-8s failed=%-5d %v\n", col.Column, col.Type, col.Failed, col.Statuses)
				}
			}
			if view.Failed > 0 {
				return notFoundErr(fmt.Errorf("%d failed cell run(s) across %d column(s)", view.Failed, len(view.Columns)))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagWorkspace, "workspace", "", "Clay workspace id (or set CLAY_WORKSPACE_ID)")
	cmd.Flags().BoolVar(&flagAll, "all", false, "Include columns with no failures")
	return cmd
}
