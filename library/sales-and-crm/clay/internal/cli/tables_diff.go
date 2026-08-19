// Copyright 2026 Ade Amos and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

type columnDelta struct {
	Column string `json:"column"`
	Change string `json:"change"`
	Left   string `json:"left,omitempty"`
	Right  string `json:"right,omitempty"`
}

type tableDiff struct {
	LeftTable  string        `json:"leftTableId"`
	RightTable string        `json:"rightTableId"`
	Deltas     []columnDelta `json:"deltas"`
	Identical  bool          `json:"identical"`
}

func newNovelTablesDiffCmd(flags *rootFlags) *cobra.Command {
	var flagWorkspace string
	cmd := &cobra.Command{
		Use:   "diff <leftTableId> <rightTableId>",
		Short: "Structurally compare two tables' column graphs, including formulas and enrichment bindings.",
		Long: "Use this command to see what drifted between a template table and a copy.\n" +
			"Do NOT use it to compare row data; it compares schema only.",
		Example: "  clay-pp-cli tables diff t_abc123 t_def456 --workspace 1234567 --agent",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "<left>=t_a;<right>=t_b;--workspace=1;--dry-run",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("two table ids are required"))
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "tables diff")
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
			left, err := fetchTable(ctx, c, ws, args[0])
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			right, err := fetchTable(ctx, c, ws, args[1])
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			lIdx, rIdx := indexByName(left.Fields), indexByName(right.Fields)
			lByID, rByID := indexByID(left.Fields), indexByID(right.Fields)
			d := tableDiff{LeftTable: left.ID, RightTable: right.ID, Deltas: make([]columnDelta, 0)}

			for _, f := range left.Fields {
				if isSystemField(f.ID) {
					continue
				}
				other, ok := rIdx[lowerTrim(f.Name)]
				if !ok {
					d.Deltas = append(d.Deltas, columnDelta{Column: f.Name, Change: "only-in-left", Left: f.Type})
					continue
				}
				if f.Type != other.Type {
					d.Deltas = append(d.Deltas, columnDelta{Column: f.Name, Change: "type-changed", Left: f.Type, Right: other.Type})
				}
				lf := resolveRefsToNames(f.settings().formulaBody(), lByID)
				rf := resolveRefsToNames(other.settings().formulaBody(), rByID)
				if lf != rf {
					d.Deltas = append(d.Deltas, columnDelta{Column: f.Name, Change: "formula-changed", Left: lf, Right: rf})
				}
				if la, ra := f.settings().ActionKey, other.settings().ActionKey; la != ra {
					d.Deltas = append(d.Deltas, columnDelta{Column: f.Name, Change: "action-changed", Left: la, Right: ra})
				}
			}
			for _, f := range right.Fields {
				if isSystemField(f.ID) {
					continue
				}
				if _, ok := lIdx[lowerTrim(f.Name)]; !ok {
					d.Deltas = append(d.Deltas, columnDelta{Column: f.Name, Change: "only-in-right", Right: f.Type})
				}
			}
			d.Identical = len(d.Deltas) == 0

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFiltered(cmd.OutOrStdout(), d, flags); err != nil {
					return err
				}
			} else if d.Identical {
				fmt.Fprintln(cmd.OutOrStdout(), "schemas are identical")
			} else {
				for _, x := range d.Deltas {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-16s %-28s %s | %s\n", x.Change, x.Column, x.Left, x.Right)
				}
			}
			if !d.Identical {
				return notFoundErr(fmt.Errorf("%d schema difference(s)", len(d.Deltas)))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagWorkspace, "workspace", "", "Clay workspace id (or set CLAY_WORKSPACE_ID)")
	return cmd
}
