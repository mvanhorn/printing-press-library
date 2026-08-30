// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0.

// pp:data-source live
// Supported strategies: auto, local, live, or computed.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/is-agentic/internal/agentic"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newNovelEvidenceCmd(flags))
	})
}

type evidenceView struct {
	FetchedAt string          `json:"fetched_at"`
	Report    json.RawMessage `json:"report"`
}

func newNovelEvidenceCmd(flags *rootFlags) *cobra.Command {
	var target string
	cmd := &cobra.Command{
		Use:         "evidence",
		Short:       "Package a report and its provenance into a portable evidence artifact.",
		Example:     "  is-agentic-pp-cli evidence --target https://is-agentic.com --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !hasChangedLocalFlags(cmd) && !flags.dryRun {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "fetch and package report evidence")
			}
			if target == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--target is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			report, err := agentic.New().Fetch(ctx, target)
			if err != nil {
				return fmt.Errorf("fetching evidence target: %w", err)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) && !flags.asJSON && !flags.agent && flags.selectFields == "" && !flags.compact && !flags.csv && !flags.plain && isatty(cmd.OutOrStdout()) {
				return renderReport(cmd.OutOrStdout(), report)
			}
			return printJSONFiltered(cmd.OutOrStdout(), evidenceView{FetchedAt: report.FetchedAt.UTC().Format("2006-01-02T15:04:05Z07:00"), Report: json.RawMessage(report.Raw)}, flags)
		},
	}
	cmd.Flags().StringVar(&target, "target", "", "public URL to package as evidence")
	return cmd
}
