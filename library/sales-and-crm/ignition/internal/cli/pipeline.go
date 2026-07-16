// Copyright 2026 corben-tech and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type pipelineView struct {
	ByStatus map[string]int `json:"by_status"`
	Total    int            `json:"total"`
}

func newNovelPipelineCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "pipeline",
		Short:       "Count proposals across DRAFT, AWAITING_ACCEPTANCE, ACCEPTED, and LOST for the whole book of business.",
		Example:     "  ignition-pp-cli pipeline --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch proposals")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			proposals, err := fetchSearchIndex(ctx, c, "PROPOSAL", proposalSearchQuery, "pagedProposals")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			view := buildPipelineView(proposals)
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			printPipelineTable(cmd.OutOrStdout(), view)
			return nil
		},
	}
	return cmd
}

func buildPipelineView(nodes []searchNode) pipelineView {
	view := pipelineView{
		ByStatus: map[string]int{
			"DRAFT":               0,
			"AWAITING_ACCEPTANCE": 0,
			"ACCEPTED":            0,
			"LOST":                0,
			"other":               0,
		},
		Total: len(nodes),
	}
	for _, node := range nodes {
		status := strings.ToUpper(strings.TrimSpace(node.Status))
		switch status {
		case "DRAFT", "AWAITING_ACCEPTANCE", "ACCEPTED", "LOST":
			view.ByStatus[status]++
		default:
			view.ByStatus["other"]++
		}
	}
	return view
}

func printPipelineTable(w io.Writer, view pipelineView) {
	tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "STATUS\tCOUNT")
	for _, status := range []string{"DRAFT", "AWAITING_ACCEPTANCE", "ACCEPTED", "LOST", "other"} {
		fmt.Fprintf(tw, "%s\t%d\n", status, view.ByStatus[status])
	}
	fmt.Fprintf(tw, "TOTAL\t%d\n", view.Total)
	_ = tw.Flush()
}
