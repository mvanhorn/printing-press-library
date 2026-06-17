// Copyright 2026 Brandon Nye and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature (do not re-stub on regen).

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelLabelsCoverageCmd(flags *rootFlags) *cobra.Command {
	var flagRepo string
	var flagRefresh bool
	var flagMaxPages int

	cmd := &cobra.Command{
		Use:   "coverage",
		Short: "Report each label's open and closed issue counts, and flag unused labels and unlabeled issues.",
		Long: `Report per-label open/closed counts across synced issues, plus labels defined on
the repo but applied to nothing (prune candidates) and issues with no label —
triage hygiene no single GitHub API call returns.

Pass --repo owner/repo the first time to populate; later runs reuse it offline.`,
		Example:     "  github-pp-cli labels coverage --repo cli/cli",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			st, err := openNovelStoreRW(cmd)
			if err != nil {
				return err
			}
			defer st.Close()
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if err := nvEnsurePopulated(ctx, cmd, flags, st, flagRepo, flagRefresh, flagMaxPages, "issues", "labels"); err != nil {
				return err
			}

			issues, err := st.List("issues", 0)
			if err != nil {
				return fmt.Errorf("listing synced issues: %w", err)
			}
			repoLabels, err := st.List("labels", 0)
			if err != nil {
				return fmt.Errorf("listing synced labels: %w", err)
			}
			report := nvLabelCoverage(nvRaws(issues), nvRaws(repoLabels))

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				if len(report.Labels) == 0 {
					fmt.Fprintln(cmd.ErrOrStderr(), "no synced issues/labels (try --repo owner/repo to populate)")
				}
				tr := make([][]string, 0, len(report.Labels))
				for _, l := range report.Labels {
					tr = append(tr, []string{l.Label, strconv.Itoa(l.Open), strconv.Itoa(l.Closed), strconv.Itoa(l.Total)})
				}
				if err := flags.printTable(cmd, []string{"LABEL", "OPEN", "CLOSED", "TOTAL"}, tr); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "\nunlabeled: %d open, %d closed\n", report.UnlabeledOpen, report.UnlabeledClosed)
				if len(report.UnusedLabels) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "unused labels (%d): %s\n", len(report.UnusedLabels), strings.Join(report.UnusedLabels, ", "))
				}
				return nil
			}
			return flags.printJSON(cmd, report)
		},
	}
	cmd.Flags().StringVar(&flagRepo, "repo", "", "owner/repo to populate the local mirror from (inferred from prior syncs when omitted)")
	cmd.Flags().BoolVar(&flagRefresh, "refresh", false, "Re-fetch issues/labels even if the local mirror already has them")
	cmd.Flags().IntVar(&flagMaxPages, "max-pages", 3, "Maximum pages (100/page) to fetch when populating")
	return cmd
}
