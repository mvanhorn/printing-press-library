// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: rank ads by click-through decay so tired creative is obvious.
// pp:data-source local
// Supported strategies: auto, local, live, or computed. This command is local-only.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/openai-ads/internal/store"
	"github.com/spf13/cobra"
)

// fatigueRow is one ranked ad.
type fatigueRow struct {
	AdID        string  `json:"ad_id"`
	Name        string  `json:"name"`
	Status      string  `json:"status"`
	CtrCurrent  float64 `json:"ctr_current"`
	CtrEarlier  float64 `json:"ctr_earlier"`
	Decay       float64 `json:"decay"` // percentage-point decline in CTR
	Impressions int64   `json:"impressions"`
	Buckets     int     `json:"buckets"`
}

// fatigueOutput is the internal carrier for the ranked rows plus an empty-state
// explanation. Only Results reaches stdout — every novel command emits a bare
// array there so agents get one shape across the surface — and Note goes to stderr.
type fatigueOutput struct {
	Results []fatigueRow `json:"results"`
	Note    string       `json:"note,omitempty"`
}

// fatigueDecay measures the falloff between the first half and the second half
// of an ad's CTR history (in percentage points). Positive = decaying.
func fatigueDecay(ctrs []float64) float64 {
	if len(ctrs) < 2 {
		return 0
	}
	mid := len(ctrs) / 2
	first := avgFloat(ctrs[:mid])
	second := avgFloat(ctrs[mid:])
	return first - second
}

func avgFloat(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	var s float64
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}

func newNovelFatigueCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int
	var flagDays int

	cmd := &cobra.Command{
		Use:   "fatigue",
		Short: "Rank ads by click-through decay so tired creative is obvious before spend is wasted.",
		Long: `Measure each ad's click-through rate across its ad insight snapshots and
rank the worst-decaying ads first. Empty insight data returns an empty array on
stdout and explains why on stderr.`,
		Example: strings.Trim(`
  openai-ads-pp-cli fatigue --agent
  openai-ads-pp-cli fatigue --limit 20 --days 14
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "fatigue")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := openStoreForRead(ctx, "openai-ads-pp-cli")
			if err != nil {
				return err
			}
			if db == nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "hint: run 'openai-ads-pp-cli sync' first to populate the local database.")
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), make([]fatigueRow, 0), flags)
				}
				return nil
			}
			defer db.Close()

			limit := flagLimit
			if limit <= 0 {
				limit = 10
			}
			days := flagDays
			if days <= 0 {
				days = 7
			}
			out, err := loadFatigue(db, days)
			if err != nil {
				return err
			}
			if out.Note == "" && len(out.Results) == 0 {
				out.Note = "no ad insight snapshots exist yet; run 'openai-ads-pp-cli sync' to capture them."
			}
			if limit < len(out.Results) {
				out.Results = out.Results[:limit]
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if out.Note != "" {
					fmt.Fprintln(cmd.ErrOrStderr(), "note: "+out.Note)
				}
				return printJSONFiltered(cmd.OutOrStdout(), out.Results, flags)
			}
			if len(out.Results) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", out.Note)
				return nil
			}
			maps, merr := rowsToMaps(out.Results)
			if merr != nil {
				return merr
			}
			return printAutoTable(cmd.OutOrStdout(), maps)
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Maximum number of ads to rank.")
	cmd.Flags().IntVar(&flagDays, "days", 7, "How many days of insight history to consider.")
	return cmd
}

// loadFatigue gathers per-ad CTR histories and ranks by decay.
func loadFatigue(db *store.Store, days int) (fatigueOutput, error) {
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour).UTC()
	out := fatigueOutput{Results: make([]fatigueRow, 0)}
	rows, err := db.Query(`SELECT ai.ads_id, a.name, a.status, ai.data, ai.synced_at
		FROM ads_insights ai
		JOIN ads a ON a.id = ai.ads_id`)
	if err != nil {
		return out, fmt.Errorf("querying ad insights: %w", err)
	}
	defer rows.Close()

	hist := map[string]*fatAd{}
	order := []string{}
	for rows.Next() {
		var (
			adID, name, status sql.NullString
			data, synced       sql.NullString
		)
		if err := rows.Scan(&adID, &name, &status, &data, &synced); err != nil {
			return out, fmt.Errorf("scanning insight row: %w", err)
		}
		if t, ok := parseSyncTS(synced.String); ok && t.Before(cutoff) {
			continue
		}
		key := adID.String
		if _, ok := hist[key]; !ok {
			hist[key] = &fatAd{adID: key, name: name.String, status: status.String}
			order = append(order, key)
		}
		imp, clicks, ctr := insightMetrics(data.String)
		hist[key].buckets++
		hist[key].imps += imp
		if ctr > 0 {
			hist[key].ctrs = append(hist[key].ctrs, ctr)
		}
		_ = clicks
	}
	if err := rows.Err(); err != nil {
		return out, err
	}

	sort.SliceStable(order, func(i, j int) bool {
		di := fatigueDecay(hist[order[i]].ctrs)
		dj := fatigueDecay(hist[order[j]].ctrs)
		return di > dj
	})
	for _, key := range order {
		a := hist[key]
		ctrs := a.ctrs
		var current, earlier float64
		if len(ctrs) > 0 {
			current = ctrs[len(ctrs)-1]
			earlier = fatigueEarlierAvg(ctrs)
		}
		out.Results = append(out.Results, fatigueRow{
			AdID:        a.adID,
			Name:        a.name,
			Status:      a.status,
			CtrCurrent:  round4(current * 100),
			CtrEarlier:  round4(earlier * 100),
			Decay:       round4(fatigueDecay(ctrs) * 100),
			Impressions: a.imps,
			Buckets:     a.buckets,
		})
	}
	return out, nil
}

// fatigueEarlierAvg is the average CTR over the first half of the history.
func fatigueEarlierAvg(ctrs []float64) float64 {
	if len(ctrs) < 2 {
		return 0
	}
	return avgFloat(ctrs[:len(ctrs)/2])
}

type fatAd struct {
	adID    string
	name    string
	status  string
	ctrs    []float64
	imps    int64
	buckets int
}

// insightMetrics extracts impressions and CTR from an insight blob.
func insightMetrics(data string) (int64, int64, float64) {
	if strings.TrimSpace(data) == "" {
		return 0, 0, 0
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(data), &obj); err != nil {
		return 0, 0, 0
	}
	var imps, clicks int64
	if v, ok := obj["impressions"].(float64); ok {
		imps = int64(v)
	}
	if v, ok := obj["clicks"].(float64); ok {
		clicks = int64(v)
	}
	var ctr float64
	if v, ok := obj["ctr"].(float64); ok {
		ctr = v
	}
	if ctr <= 0 && imps > 0 {
		ctr = float64(clicks) / float64(imps)
	}
	return imps, clicks, ctr
}

func parseSyncTS(s string) (time.Time, bool) {
	if strings.TrimSpace(s) == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func round4(v float64) float64 {
	return float64(int64(v*10000+0.5)) / 10000
}
