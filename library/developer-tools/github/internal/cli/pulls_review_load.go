// Copyright 2026 Brandon Nye and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature (do not re-stub on regen).

package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelPullsReviewLoadCmd(flags *rootFlags) *cobra.Command {
	var flagState, flagRepo string
	var flagRefresh bool
	var flagMaxPages int

	cmd := &cobra.Command{
		Use:   "review-load",
		Short: "Aggregate open PRs by requested reviewer to show who has the largest unactioned review queue.",
		Long: `Aggregate synced pull requests by requested reviewer and rank reviewers by how
many PRs are waiting on them — the team's review bottleneck, an aggregation the
GitHub API and 'gh' never compute across PRs.

Pass --repo owner/repo the first time to populate; later runs reuse it offline.
For the author-side stale-PR list, use 'github-pp-cli pulls stale'.`,
		Example:     "  github-pp-cli pulls review-load --repo cli/cli --state open",
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
			if err := nvEnsurePopulated(ctx, cmd, flags, st, flagRepo, flagRefresh, flagMaxPages, "pulls"); err != nil {
				return err
			}

			prs, err := st.List("pulls", 0)
			if err != nil {
				return fmt.Errorf("listing synced pulls: %w", err)
			}
			load := nvReviewerLoad(nvRaws(prs), flagState)

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				if len(load) == 0 {
					fmt.Fprintln(cmd.ErrOrStderr(), "no requested reviewers on synced PRs (try --repo owner/repo to populate)")
				}
				tr := make([][]string, 0, len(load))
				for _, r := range load {
					tr = append(tr, []string{r.Reviewer, strconv.Itoa(r.OpenPRs)})
				}
				return flags.printTable(cmd, []string{"REVIEWER", "PRS_AWAITING"}, tr)
			}
			return flags.printJSON(cmd, load)
		},
	}
	cmd.Flags().StringVar(&flagState, "state", "open", "PR state to count: open, closed, or all")
	cmd.Flags().StringVar(&flagRepo, "repo", "", "owner/repo to populate the local mirror from (inferred from prior syncs when omitted)")
	cmd.Flags().BoolVar(&flagRefresh, "refresh", false, "Re-fetch pulls even if the local mirror already has them")
	cmd.Flags().IntVar(&flagMaxPages, "max-pages", 3, "Maximum pages (100/page) to fetch when populating")
	return cmd
}
