// Copyright 2026 waveriderai and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: analyst/firm accuracy scorecard. Ranks ratings-analysts rows by
// Benzinga's ratings_accuracy metric, and (with --ticker/--today) left-joins
// recent rating changes to tag each issuer's hit rate.
//
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/benzinga/internal/store"

	"github.com/spf13/cobra"
)

type analystScore struct {
	Rank        int     `json:"rank"`
	Analyst     string  `json:"analyst"`
	Firm        string  `json:"firm"`
	Metric      string  `json:"metric"`
	Value       float64 `json:"value"`
	SuccessRate float64 `json:"success_rate"`
	AvgReturn   float64 `json:"avg_return"`
}

type taggedRating struct {
	Ticker        string  `json:"ticker"`
	Date          string  `json:"date"`
	Analyst       string  `json:"analyst"`
	Firm          string  `json:"firm"`
	Action        string  `json:"action"`
	RatingCurrent string  `json:"rating_current"`
	PriceTarget   string  `json:"price_target"`
	AccuracyValue float64 `json:"accuracy_value"`
	AccuracyKnown bool    `json:"accuracy_known"`
}

func newNovelAnalystAccuracyCmd(flags *rootFlags) *cobra.Command {
	var (
		flagTicker string
		flagToday  bool
		flagMetric string
		flagLimit  int
		dbPath     string
	)

	cmd := &cobra.Command{
		Use:   "analyst-accuracy",
		Short: "Rank rating-issuing firms and analysts by Benzinga's historical accuracy",
		Long: strings.Trim(`
Ranks the synced analyst roster by Benzinga's ratings_accuracy metric (default
1y_smart_score; choose another with --metric). With --ticker or --today, left-
joins recent rating changes and tags each with the issuing analyst's accuracy so
you can weight a fresh upgrade/downgrade by its source's track record.

Reads the local SQLite mirror. Run 'sync' first:
  benzinga-pp-cli sync --resources calendar-ratings-analysts,calendar-ratings --since 7d
`, "\n"),
		Example:     "  benzinga-pp-cli analyst-accuracy --ticker AAPL --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			metric := strings.TrimSpace(flagMetric)
			if metric == "" {
				metric = "1y_smart_score"
			}

			db, ok, err := novelStore(cmd, flags, dbPath,
				"benzinga-pp-cli sync --resources calendar-ratings-analysts,calendar-ratings --since 7d")
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			defer db.Close()
			maybeEmitSyncHints(cmd, db, "calendar-ratings-analysts", flags.maxAge)

			analystRows, err := novelRows(cmd.Context(), db, "calendar-ratings-analysts")
			if err != nil {
				return fmt.Errorf("querying analysts: %w", err)
			}

			// Index analysts by id and by lower-cased full name for the join.
			byID := map[string]map[string]any{}
			byName := map[string]map[string]any{}
			for _, m := range analystRows {
				if id := novelStr(m, "id"); id != "" {
					byID[id] = m
				}
				if name := strings.ToLower(novelStr(m, "name_full")); name != "" {
					byName[name] = m
				}
			}

			if flagTicker != "" || flagToday {
				return runAnalystAccuracyJoin(cmd, flags, db, metric, normTicker(flagTicker), flagLimit, byID, byName)
			}

			// Default: rank analysts by the chosen accuracy metric.
			scores := make([]analystScore, 0, len(analystRows))
			for _, m := range analystRows {
				acc, hasAcc := m["ratings_accuracy"].(map[string]any)
				if !hasAcc {
					continue
				}
				val, ok := novelFloat(acc, metric)
				if !ok {
					continue
				}
				sr, _ := novelFloat(acc, "1y_success_rate")
				ar, _ := novelFloat(acc, "1y_average_return")
				scores = append(scores, analystScore{
					Analyst:     novelStr(m, "name_full"),
					Firm:        novelStr(m, "firm_name"),
					Metric:      metric,
					Value:       val,
					SuccessRate: sr,
					AvgReturn:   ar,
				})
			}
			sort.SliceStable(scores, func(i, j int) bool { return scores[i].Value > scores[j].Value })
			if flagLimit > 0 && len(scores) > flagLimit {
				scores = scores[:flagLimit]
			}
			for i := range scores {
				scores[i].Rank = i + 1
			}

			return novelEmit(cmd, flags, scores, func() {
				out := cmd.OutOrStdout()
				if len(scores) == 0 {
					fmt.Fprintf(out, "No analyst accuracy data for metric %q. Run sync, or try --metric 1y_success_rate.\n", metric)
					return
				}
				fmt.Fprintf(out, "RANK\tANALYST\tFIRM\t%s\tSUCCESS%%\tAVG_RET\n", strings.ToUpper(metric))
				for _, s := range scores {
					fmt.Fprintf(out, "%d\t%s\t%s\t%.2f\t%.2f\t%.2f\n", s.Rank, s.Analyst, s.Firm, s.Value, s.SuccessRate, s.AvgReturn)
				}
			})
		},
	}

	cmd.Flags().StringVar(&flagTicker, "ticker", "", "Tag recent rating changes for this ticker with issuer accuracy")
	cmd.Flags().BoolVar(&flagToday, "today", false, "Tag all recent rating changes with issuer accuracy")
	cmd.Flags().StringVar(&flagMetric, "metric", "1y_smart_score", "ratings_accuracy field to rank by (e.g. 1y_smart_score, 1y_success_rate, 1y_average_return)")
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Maximum rows to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

func runAnalystAccuracyJoin(cmd *cobra.Command, flags *rootFlags, db *store.Store, metric, ticker string, limit int, byID, byName map[string]map[string]any) error {
	ratingRows, err := novelRows(cmd.Context(), db, "calendar-ratings")
	if err != nil {
		return fmt.Errorf("querying ratings: %w", err)
	}
	tagged := make([]taggedRating, 0)
	for _, m := range ratingRows {
		t := normTicker(novelStr(m, "ticker"))
		if ticker != "" && t != ticker {
			continue
		}
		tr := taggedRating{
			Ticker:        t,
			Date:          novelStr(m, "date"),
			Analyst:       novelStr(m, "analyst_name"),
			Firm:          novelStr(m, "analyst"),
			Action:        strings.TrimSpace(novelStr(m, "action_company") + " " + novelStr(m, "action_pt")),
			RatingCurrent: novelStr(m, "rating_current"),
			PriceTarget:   novelStr(m, "pt_current"),
		}
		var arow map[string]any
		if id := novelStr(m, "analyst_id"); id != "" {
			arow = byID[id]
		}
		if arow == nil {
			arow = byName[strings.ToLower(novelStr(m, "analyst_name"))]
		}
		if arow != nil {
			if acc, ok := arow["ratings_accuracy"].(map[string]any); ok {
				if v, ok := novelFloat(acc, metric); ok {
					tr.AccuracyValue = v
					tr.AccuracyKnown = true
				}
			}
		}
		tagged = append(tagged, tr)
	}
	// Highest-accuracy issuers first; unknown accuracy sinks to the bottom.
	sort.SliceStable(tagged, func(i, j int) bool {
		if tagged[i].AccuracyKnown != tagged[j].AccuracyKnown {
			return tagged[i].AccuracyKnown
		}
		return tagged[i].AccuracyValue > tagged[j].AccuracyValue
	})
	if limit > 0 && len(tagged) > limit {
		tagged = tagged[:limit]
	}

	return novelEmit(cmd, flags, tagged, func() {
		out := cmd.OutOrStdout()
		if len(tagged) == 0 {
			fmt.Fprintln(out, "No rating changes found. Run sync --resources calendar-ratings,calendar-ratings-analysts.")
			return
		}
		fmt.Fprintf(out, "TICKER\tDATE\tANALYST\tFIRM\tACTION\tRATING\tPT\t%s\n", strings.ToUpper(metric))
		for _, tr := range tagged {
			acc := "n/a"
			if tr.AccuracyKnown {
				acc = fmt.Sprintf("%.2f", tr.AccuracyValue)
			}
			fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				tr.Ticker, tr.Date, tr.Analyst, tr.Firm, tr.Action, tr.RatingCurrent, tr.PriceTarget, acc)
		}
	})
}
