// Copyright 2026 sdhilip200. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/store"
	"github.com/spf13/cobra"
)

func newNovelDailyDigestCmd(flags *rootFlags) *cobra.Command {
	var since string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "daily-digest",
		Short: "One-command morning briefing — new high-score contacts, biggest score drops, lifecycle promotions, freshly stale VIPs.",
		Long: strings.Trim(`
Snapshots the current score + engagement state for every contact, then
composes four sections that share a single --since window:

  score_movers       Top 5 score changes (gainers and droppers) of >= 20%
  stage_promotions   Top 5 lifecycle transitions seen since the last snapshot
  fresh_stale_vips   Top 5 stale-but-valuable contacts (score >= 70)
  decay_movers       Top 5 engagement-decay candidates

First-run note: with no prior snapshots in the window the score_movers and
stage_promotions sections are empty and the response carries an
explanatory 'note'. Re-running after a sync cycle populates history.
`, "\n"),
		Example: strings.Trim(`
  # Daily morning briefing for the last 24 hours
  hubspot-pp-cli daily-digest --since 1d --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			d, err := cliutil.ParseDurationLoose(since)
			if err != nil {
				return usageErr(fmt.Errorf("--since %q: %w", since, err))
			}
			if d <= 0 {
				return usageErr(fmt.Errorf("--since must be positive"))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("hubspot-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			now := time.Now().UTC()
			scoreTaken, err := snapshotScores(cmd.Context(), db, now)
			if err != nil {
				return fmt.Errorf("snapshotting scores: %w", err)
			}
			engTaken, err := snapshotEngagement(cmd.Context(), db, now)
			if err != nil {
				return fmt.Errorf("snapshotting engagement: %w", err)
			}

			// score_movers: pull up to 5 gainers and 5 droppers, then keep
			// the top 5 by absolute drift overall.
			movers, _, moversParseErrors, err := computeScoreDrift(cmd.Context(), db, now, d, 20, "any")
			if err != nil {
				return err
			}
			topMovers := limitInt(movers, 5)

			promotions, promotionsParseErrors, err := computeStagePromotions(cmd.Context(), db, now, d)
			if err != nil {
				return err
			}
			topPromotions := limitInt(promotions, 5)

			silentDays := int(d.Hours() / 24)
			if silentDays < 1 {
				silentDays = 1
			}
			staleVIPs, _, staleParseErrors, err := computeStaleVIPs(cmd.Context(), db, now, 70, silentDays)
			if err != nil {
				return err
			}
			topStaleVIPs := limitInt(staleVIPs, 5)

			decay, _, decayParseErrors, err := computeDecay(cmd.Context(), db, now, d, 1)
			if err != nil {
				return err
			}
			topDecay := limitInt(decay, 5)

			// Each compute helper iterates the contacts table independently;
			// taking the maximum (rather than the sum) approximates the count
			// of distinct rows that failed to parse in any single pass without
			// double-counting the same bad row across four scans.
			parseErrors := moversParseErrors
			for _, n := range []int{promotionsParseErrors, staleParseErrors, decayParseErrors} {
				if n > parseErrors {
					parseErrors = n
				}
			}

			result := map[string]any{
				"since":            since,
				"score_movers":     topMovers,
				"stage_promotions": topPromotions,
				"fresh_stale_vips": topStaleVIPs,
				"decay_movers":     topDecay,
				"snapshots_taken":  scoreTaken + engTaken,
			}
			if parseErrors > 0 {
				result["parse_errors"] = parseErrors
			}
			if len(topMovers) == 0 && len(topPromotions) == 0 {
				hasPrior, perr := scoreSnapshotHasPriorInWindow(cmd.Context(), db.DB(), now, d)
				if perr == nil && !hasPrior {
					result["note"] = "no prior score snapshots within window; score_movers and stage_promotions will populate after another digest run"
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "1d", "Lookback window for the digest (e.g. 1d, 12h, 1w)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override default SQLite database path")
	return cmd
}
