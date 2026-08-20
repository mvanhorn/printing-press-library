// Copyright 2026 Isaac Marks and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: stale — open deals with no activity in N days, grouped by stage.
// pp:data-source local
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/clarify/internal/store"

	"github.com/spf13/cobra"
)

type staleDealView struct {
	ID       string  `json:"id"`
	Name     string  `json:"name,omitempty"`
	Stage    string  `json:"stage,omitempty"`
	Amount   float64 `json:"amount,omitempty"`
	DaysIdle int     `json:"days_idle"`
}

type staleView struct {
	Days         int             `json:"days"`
	StaleDeals   []staleDealView `json:"stale_deals"`
	ByStage      map[string]int  `json:"by_stage"`
	ScannedDeals int             `json:"scanned_deals"`
	Note         string          `json:"note,omitempty"`
}

func newNovelStaleCmd(flags *rootFlags) *cobra.Command {
	var flagDays int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "stale",
		Short: "Open deals with no activity in N days, grouped by pipeline stage.",
		Long: `Use this command to find deals with no recent activity.
Do NOT use it for a full daily overview; use 'brief' instead.

Reads the local mirror: run 'clarify-pp-cli sync --resources resources --path-context object=deal' first.`,
		Example:     `  clarify-pp-cli stale --days 14 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would report open deals idle longer than the --days threshold from the local mirror")
				return nil
			}
			if flagDays <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--days must be a positive number of days"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, ok, err := clarifyMirrorGuard(cmd, flags, ctx, dbPath)
			if err != nil || !ok {
				return err
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "resources") {
				hintIfStale(cmd, db, "resources", flags.maxAge)
			}

			deals, err := loadClarifyObjects(ctx, db, "deal")
			if err != nil {
				return err
			}
			cutoff := time.Now().AddDate(0, 0, -flagDays)
			view := staleView{Days: flagDays, StaleDeals: []staleDealView{}, ByStage: map[string]int{}, ScannedDeals: len(deals)}
			skippedNoTimestamp := 0
			// Opportunistically record stage observations so velocity's
			// history accrues from every analytics run.
			observations := make([]store.DealStageObservation, 0, len(deals))
			for _, d := range deals {
				stage := attrString(d.Attrs, clarifyStageKeys...)
				if stage != "" {
					observations = append(observations, store.DealStageObservation{DealID: d.ID, Stage: stage})
				}
				if clarifyStageClosed(stage) {
					continue
				}
				updated, hasTS := objUpdatedAt(d)
				if !hasTS {
					skippedNoTimestamp++
					continue
				}
				if updated.After(cutoff) {
					continue
				}
				entry := staleDealView{
					ID:       d.ID,
					Name:     attrString(d.Attrs, clarifyNameKeys...),
					Stage:    stage,
					DaysIdle: int(time.Since(updated).Hours() / 24),
				}
				if amount, ok := attrNumber(d.Attrs, clarifyAmountKeys...); ok {
					entry.Amount = amount
				}
				view.StaleDeals = append(view.StaleDeals, entry)
				stageKey := stage
				if stageKey == "" {
					stageKey = "(no stage)"
				}
				view.ByStage[stageKey]++
			}
			sort.Slice(view.StaleDeals, func(i, j int) bool {
				return view.StaleDeals[i].DaysIdle > view.StaleDeals[j].DaysIdle
			})
			if err := db.EnsureClarifySideTables(ctx); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: stage-history side table unavailable: %v\n", err)
			} else if _, obsErr := db.RecordStageObservations(ctx, observations, time.Now()); obsErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: stage observations not recorded: %v\n", obsErr)
			}
			if len(deals) == 0 {
				view.Note = "no deals in the local mirror; run: clarify-pp-cli sync --resources resources --path-context object=deal"
			} else if skippedNoTimestamp > 0 {
				view.Note = fmt.Sprintf("%d deals had no parseable update timestamp and were skipped", skippedNoTimestamp)
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(view.StaleDeals) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No open deals idle longer than %d days (%d scanned).\n", flagDays, view.ScannedDeals)
				if view.Note != "" {
					fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				}
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d open deals idle longer than %d days:\n\n", len(view.StaleDeals), flagDays)
			for _, d := range view.StaleDeals {
				name := d.Name
				if name == "" {
					name = d.ID
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %-40s  %-20s  %3dd idle\n", name, d.Stage, d.DaysIdle)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&flagDays, "days", 14, "Idle threshold in days: deals with no update for this long count as stale")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (defaults to the standard local mirror)")
	return cmd
}
