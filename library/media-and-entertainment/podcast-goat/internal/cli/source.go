// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.
// PATCH: v0.1 `source compare` — fan out to every matching adapter, print diff.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/dispatch"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/transcript"
)

func newSourceCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "source",
		Short: "Inspect the registered transcript sources",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newSourceCompareCmd(flags))
	cmd.AddCommand(newSourceListCmd(flags))
	return cmd
}

func newSourceListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List adapters in dispatch order",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			adapters := dispatch.Registered()
			type row struct {
				Order int    `json:"order"`
				Name  string `json:"name"`
				Tier  string `json:"tier"`
			}
			var rows []row
			for i, a := range adapters {
				rows = append(rows, row{Order: i + 1, Name: a.Name(), Tier: string(a.Tier())})
			}
			if flags.asJSON {
				out, _ := json.MarshalIndent(rows, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}
			headers := []string{"order", "name", "tier"}
			var data [][]string
			for _, r := range rows {
				data = append(data, []string{fmt.Sprintf("%d", r.Order), r.Name, r.Tier})
			}
			return flags.printTable(cmd, headers, data)
		},
	}
	return cmd
}

func newSourceCompareCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "compare [url]",
		Short:       "Fetch the URL from every matching adapter and compare results",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			url := args[0]
			type row struct {
				Adapter    string                        `json:"adapter"`
				Tier       string                        `json:"tier"`
				Status     string                        `json:"status"`
				Reason     string                        `json:"reason,omitempty"`
				Segments   int                           `json:"segment_count"`
				Tokens     int                           `json:"token_estimate"`
				Speakers   []string                      `json:"speakers,omitempty"`
				CostCredit float64                       `json:"cost_credits,omitempty"`
				Transcript *transcript.Transcript        `json:"-"`
			}
			var rows []row
			for _, a := range dispatch.Registered() {
				r := row{Adapter: a.Name(), Tier: string(a.Tier())}
				if !a.Match(url) {
					r.Status = "skip"
					r.Reason = "URL pattern does not match"
					rows = append(rows, r)
					continue
				}
				tr, err := a.Fetch(cmd.Context(), url)
				if err != nil {
					r.Status = "error"
					r.Reason = err.Error()
					rows = append(rows, r)
					continue
				}
				r.Status = "ok"
				r.Segments = len(tr.Segments)
				r.Tokens = tr.TokenEstimate()
				r.Speakers = tr.Speakers()
				r.CostCredit = tr.CostCredits
				rows = append(rows, r)
			}
			if flags.asJSON {
				out, _ := json.MarshalIndent(rows, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}
			headers := []string{"adapter", "tier", "status", "segments", "tokens", "speakers", "cost"}
			var data [][]string
			for _, r := range rows {
				speakers := fmt.Sprintf("%d", len(r.Speakers))
				cost := ""
				if r.CostCredit > 0 {
					cost = fmt.Sprintf("%.2f", r.CostCredit)
				}
				data = append(data, []string{
					r.Adapter, r.Tier, r.Status,
					fmt.Sprintf("%d", r.Segments), fmt.Sprintf("%d", r.Tokens),
					speakers, cost,
				})
			}
			return flags.printTable(cmd, headers, data)
		},
	}
	return cmd
}
