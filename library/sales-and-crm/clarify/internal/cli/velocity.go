// Copyright 2026 Isaac Marks and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: velocity — per-stage dwell time and conversion counts.
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

type velocityStageView struct {
	Stage          string  `json:"stage"`
	OpenDeals      int     `json:"open_deals"`
	AvgDaysInStage float64 `json:"avg_days_in_stage,omitempty"`
	TransitionsOut int     `json:"transitions_out"`
}

type velocityTransitionView struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Count int    `json:"count"`
}

type velocityView struct {
	Stages          []velocityStageView      `json:"stages"`
	Transitions     []velocityTransitionView `json:"transitions"`
	ObservationsNew int                      `json:"observations_recorded_this_run"`
	HistoryRows     int                      `json:"history_rows"`
	Note            string                   `json:"note,omitempty"`
}

func newNovelVelocityCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "velocity",
		Short: "Per-stage dwell time and stage-to-stage conversion counts for your deal pipeline.",
		Long: `Computes pipeline velocity from a stage-history side table this CLI
maintains locally: every velocity, stale, or brief run records each deal's
current stage, and stage changes between runs become dwell-time and conversion
data. History accrues over time — the first run reports the current stage
distribution; later runs add transition analytics no Clarify surface keeps.

Reads the local mirror: run 'clarify-pp-cli sync --resources resources --path-context object=deal' first.`,
		Example:     `  clarify-pp-cli velocity --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute per-stage dwell time and conversions from the local stage history")
				return nil
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
			if err := db.EnsureClarifySideTables(ctx); err != nil {
				return err
			}

			deals, err := loadClarifyObjects(ctx, db, "deal")
			if err != nil {
				return err
			}
			observations := make([]store.DealStageObservation, 0, len(deals))
			openPerStage := map[string]int{}
			for _, d := range deals {
				stage := attrString(d.Attrs, clarifyStageKeys...)
				if stage == "" {
					continue
				}
				observations = append(observations, store.DealStageObservation{DealID: d.ID, Stage: stage})
				if !clarifyStageClosed(stage) {
					openPerStage[stage]++
				}
			}
			recorded, err := db.RecordStageObservations(ctx, observations, time.Now())
			if err != nil {
				return err
			}
			history, badHistoryRows, err := db.StageHistory(ctx)
			if err != nil {
				return err
			}
			if badHistoryRows > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d stage-history rows had unparseable timestamps and were excluded\n", badHistoryRows)
			}

			// Dwell time and transitions from consecutive history rows per deal.
			dwellTotal := map[string]float64{}
			dwellCount := map[string]int{}
			transitionsOut := map[string]int{}
			transitionCounts := map[[2]string]int{}
			for i := 1; i < len(history); i++ {
				prev, cur := history[i-1], history[i]
				if prev.DealID != cur.DealID || prev.ObservedAt.IsZero() || cur.ObservedAt.IsZero() {
					continue
				}
				days := cur.ObservedAt.Sub(prev.ObservedAt).Hours() / 24
				dwellTotal[prev.Stage] += days
				dwellCount[prev.Stage]++
				transitionsOut[prev.Stage]++
				transitionCounts[[2]string{prev.Stage, cur.Stage}]++
			}

			stageSet := map[string]bool{}
			for s := range openPerStage {
				stageSet[s] = true
			}
			for s := range dwellCount {
				stageSet[s] = true
			}
			view := velocityView{Stages: []velocityStageView{}, Transitions: []velocityTransitionView{}, ObservationsNew: recorded, HistoryRows: len(history)}
			for s := range stageSet {
				entry := velocityStageView{Stage: s, OpenDeals: openPerStage[s], TransitionsOut: transitionsOut[s]}
				if dwellCount[s] > 0 {
					entry.AvgDaysInStage = dwellTotal[s] / float64(dwellCount[s])
				}
				view.Stages = append(view.Stages, entry)
			}
			sort.Slice(view.Stages, func(i, j int) bool { return view.Stages[i].Stage < view.Stages[j].Stage })
			for pair, count := range transitionCounts {
				view.Transitions = append(view.Transitions, velocityTransitionView{From: pair[0], To: pair[1], Count: count})
			}
			sort.Slice(view.Transitions, func(i, j int) bool { return view.Transitions[i].Count > view.Transitions[j].Count })

			if len(deals) == 0 {
				view.Note = "no deals in the local mirror; run: clarify-pp-cli sync --resources resources --path-context object=deal"
			} else if len(view.Transitions) == 0 {
				view.Note = "no stage transitions observed yet: dwell and conversion analytics accrue as you re-run sync + velocity over time"
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Pipeline velocity (%d history rows, %d new observations this run):\n\n", view.HistoryRows, view.ObservationsNew)
			for _, s := range view.Stages {
				fmt.Fprintf(out, "  %-25s  %3d open", s.Stage, s.OpenDeals)
				if s.AvgDaysInStage > 0 {
					fmt.Fprintf(out, "  avg %.1f days in stage", s.AvgDaysInStage)
				}
				if s.TransitionsOut > 0 {
					fmt.Fprintf(out, "  %d moved on", s.TransitionsOut)
				}
				fmt.Fprintln(out)
			}
			if len(view.Transitions) > 0 {
				fmt.Fprintln(out, "\nObserved transitions:")
				for _, t := range view.Transitions {
					fmt.Fprintf(out, "  %s -> %s  (%d)\n", t.From, t.To, t.Count)
				}
			}
			if view.Note != "" {
				fmt.Fprintf(out, "\n%s\n", view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (defaults to the standard local mirror)")
	return cmd
}
