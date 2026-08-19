// Copyright 2026 Ade Amos and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

type wbNode struct {
	NodeID     string   `json:"nodeId"`
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	FieldCount int      `json:"totalFieldCount,omitempty"`
	Credit     *float64 `json:"creditEstimate,omitempty"`
}

type wbEdge struct {
	Source string `json:"sourceNodeId"`
	Target string `json:"targetNodeId"`
	Type   string `json:"type"`
}

type workbookGraph struct {
	WorkbookID  string   `json:"workbookId"`
	Nodes       []wbNode `json:"nodes"`
	Edges       []wbEdge `json:"edges"`
	TotalCredit float64  `json:"totalCreditEstimate"`
}

func newNovelWorkbooksGraphCmd(flags *rootFlags) *cobra.Command {
	var flagWorkspace string
	cmd := &cobra.Command{
		Use:   "graph <workbookId>",
		Short: "Show how every table and source in a workbook feeds the others, with credit estimates.",
		Long: "Use this command to understand an unfamiliar workbook before changing anything in it.\n" +
			"Do NOT use it for column-level dependencies; use 'columns graph'.",
		Example: "  clay-pp-cli workbooks graph wb_abc123 --workspace 1234567 --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "<workbookId>=wb_example;--workspace=1;--dry-run",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "workbooks graph")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<workbookId> is required"))
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
			raw, err := c.Get(ctx, fmt.Sprintf("/workspaces/%s/workbooks/%s/overview", ws, args[0]), nil)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), fmt.Errorf("fetching workbook overview: %w", err), flags)
			}
			var payload struct {
				Nodes []wbNode `json:"nodes"`
				Edges []wbEdge `json:"edges"`
			}
			if err := json.Unmarshal(raw, &payload); err != nil {
				return fmt.Errorf("parsing workbook overview: %w", err)
			}
			g := workbookGraph{WorkbookID: args[0], Nodes: payload.Nodes, Edges: payload.Edges}
			if g.Nodes == nil {
				g.Nodes = []wbNode{}
			}
			if g.Edges == nil {
				g.Edges = []wbEdge{}
			}
			for _, n := range g.Nodes {
				if n.Credit != nil {
					g.TotalCredit += *n.Credit
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), g, flags)
			}
			if len(g.Nodes) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "workbook has no nodes")
				return nil
			}
			for _, n := range g.Nodes {
				credit := "-"
				if n.Credit != nil {
					credit = fmt.Sprintf("%.2f", *n.Credit)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %-10s %-40s fields=%-4d credit=%s\n", n.Type, n.Name, n.FieldCount, credit)
			}
			for _, e := range g.Edges {
				fmt.Fprintf(cmd.OutOrStdout(), "  edge %s -> %s (%s)\n", e.Source, e.Target, e.Type)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  total credit estimate: %.2f\n", g.TotalCredit)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagWorkspace, "workspace", "", "Clay workspace id (or set CLAY_WORKSPACE_ID)")
	return cmd
}
