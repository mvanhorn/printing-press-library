// Copyright 2026 Ade Amos and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

type graphNode struct {
	FieldID   string   `json:"fieldId"`
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	DependsOn []string `json:"dependsOn"`
	UsedBy    []string `json:"usedBy"`
}

type columnGraph struct {
	TableID string      `json:"tableId"`
	Name    string      `json:"tableName"`
	Columns []graphNode `json:"columns"`
}

func newNovelColumnsGraphCmd(flags *rootFlags) *cobra.Command {
	var flagWorkspace string
	cmd := &cobra.Command{
		Use:   "graph <tableId>",
		Short: "Show which columns feed which, resolved from formula field references.",
		Long: "Use this command to see the dependency graph between columns before renaming or deleting one.\n" +
			"Do NOT use it for enrichment run health; use 'errors' or 'watch' instead.",
		Example: "  clay-pp-cli columns graph t_abc123 --workspace 1234567 --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "<tableId>=t_example;--workspace=1;--dry-run",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "columns graph")
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
			byID := indexByID(tbl.Fields)
			nodes := make([]graphNode, 0, len(tbl.Fields))
			usedBy := map[string][]string{}
			for _, f := range tbl.Fields {
				deps := []string{}
				for _, ref := range formulaRefs(f.settings().formulaBody()) {
					if d, ok := byID[ref]; ok {
						deps = append(deps, d.Name)
						usedBy[d.ID] = append(usedBy[d.ID], f.Name)
					}
				}
				nodes = append(nodes, graphNode{FieldID: f.ID, Name: f.Name, Type: f.Type, DependsOn: deps, UsedBy: []string{}})
			}
			for i := range nodes {
				if u, ok := usedBy[nodes[i].FieldID]; ok {
					nodes[i].UsedBy = u
				}
			}
			g := columnGraph{TableID: tbl.ID, Name: tbl.Name, Columns: nodes}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), g, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s (%s)\n", g.Name, g.TableID)
			for _, n := range g.Columns {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-28s %-10s deps=%v usedBy=%v\n", n.Name, n.Type, n.DependsOn, n.UsedBy)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagWorkspace, "workspace", "", "Clay workspace id (or set CLAY_WORKSPACE_ID)")
	return cmd
}
