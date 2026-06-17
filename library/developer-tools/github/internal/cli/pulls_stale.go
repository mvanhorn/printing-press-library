// Copyright 2026 Brandon Nye and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature (do not re-stub on regen).

package cli

import (
	"fmt"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/github/internal/cliutil"

	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelPullsStaleCmd(flags *rootFlags) *cobra.Command {
	var flagOlderThan, flagRepo string
	var flagRefresh bool
	var flagMaxPages int

	cmd := &cobra.Command{
		Use:   "stale",
		Short: "List open PRs with no activity in N days, sorted by staleness with last-activity time.",
		Long: `List open pull requests whose last activity is older than a window, sorted most
stale first, with the last-activity timestamp and staleness in days (from the
local mirror's updated_at) — decide which PRs to nudge or close.

Pass --repo owner/repo the first time to populate; later runs reuse it offline.
For who-owes-reviews, use 'github-pp-cli pulls review-load'.`,
		Example:     "  github-pp-cli pulls stale --repo cli/cli --older-than 14d",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			older, err := cliutil.ParseDurationLoose(flagOlderThan)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --older-than %q: %w", flagOlderThan, err))
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
			stale := nvStalePRs(nvRaws(prs), older, time.Now())

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				if len(stale) == 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "no open PRs older than %s (try --repo owner/repo to populate)\n", flagOlderThan)
				}
				tr := make([][]string, 0, len(stale))
				for _, p := range stale {
					tr = append(tr, []string{"#" + strconv.FormatInt(p.Number, 10), strconv.Itoa(p.StaleDays) + "d", p.LastActivity, p.Title})
				}
				return flags.printTable(cmd, []string{"PR", "STALE", "LAST_ACTIVITY", "TITLE"}, tr)
			}
			return flags.printJSON(cmd, stale)
		},
	}
	cmd.Flags().StringVar(&flagOlderThan, "older-than", "14d", "Staleness threshold (e.g. 14d, 1w, 720h)")
	cmd.Flags().StringVar(&flagRepo, "repo", "", "owner/repo to populate the local mirror from (inferred from prior syncs when omitted)")
	cmd.Flags().BoolVar(&flagRefresh, "refresh", false, "Re-fetch pulls even if the local mirror already has them")
	cmd.Flags().IntVar(&flagMaxPages, "max-pages", 3, "Maximum pages (100/page) to fetch when populating")
	return cmd
}
