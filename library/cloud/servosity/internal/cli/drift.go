// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/cloud/servosity/internal/store"
)

func newDriftCmd(flags *rootFlags) *cobra.Command {
	var metric string
	var fromStr, toStr string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "drift",
		Short: "Diff two snapshots the CLI itself collected — show what got worse and what recovered",
		Long: `Compare two snapshots from the local store (collected by 'attention' or
'sync stale'/'sync dirty-repos') and emit the symmetric difference:

  - "added"     companies that became attention-worthy / stale
  - "removed"   companies that recovered
  - "unchanged" carried over

The default metric is "attention". --from and --to accept human times like
"yesterday", "6am tomorrow", "2h", or RFC3339; defaults are the two most-recent
snapshots of the chosen metric.`,
		Example: `  # What changed between the two most-recent attention snapshots
  servosity-pp-cli drift --json

  # Compare yesterday's stale snapshot to today's
  servosity-pp-cli drift --metric stale --from yesterday --to now --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				if flags.asJSON {
					_, _ = cmd.OutOrStdout().Write([]byte(`{"meta":{"source":"dry-run"},"added":[],"removed":[]}` + "\n"))
				}
				return nil
			}
			if metric == "" {
				metric = "attention"
			}
			switch metric {
			case "attention", "stale":
			default:
				return usageErr(fmt.Errorf("unsupported --metric %q (try attention | stale)", metric))
			}

			ctx := cmd.Context()
			if dbPath == "" {
				dbPath = defaultDBPath("servosity-pp-cli")
			}
			st, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return configErr(fmt.Errorf("open store: %w", err))
			}
			defer st.Close()
			if err := st.EnsureNovelTables(ctx); err != nil {
				return apiErr(err)
			}

			out, err := computeDrift(ctx, st, metric, fromStr, toStr)
			if err != nil {
				return err
			}
			// If the user explicitly named --from / --to AND the closest
			// snapshot is more than 6 hours off the requested time, emit a
			// stderr warning so an agent reading the JSON doesn't mistake a
			// 5-minute-apart pair for the answer to "yesterday vs now".
			warnIfDriftFallback(cmd, st, ctx, metric, fromStr, toStr)
			payload, _ := json.Marshal(out)
			return printOutputWithFlags(cmd.OutOrStdout(), payload, flags)
		},
	}
	cmd.Flags().StringVar(&metric, "metric", "attention", "Snapshot family to diff: attention | stale")
	cmd.Flags().StringVar(&fromStr, "from", "", "Earlier snapshot timestamp (default: 2nd-most-recent)")
	cmd.Flags().StringVar(&toStr, "to", "", "Later snapshot timestamp (default: most-recent)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite path (default: ~/.local/share/servosity-pp-cli/data.db)")
	return cmd
}

func computeDrift(ctx context.Context, st *store.Store, metric, fromStr, toStr string) (map[string]any, error) {
	now := time.Now()
	var fromRunID, toRunID string

	switch metric {
	case "attention":
		ids, err := st.LatestRunIDs(ctx, "attention_runs", 50)
		if err != nil {
			return nil, apiErr(fmt.Errorf("list attention_runs: %w", err))
		}
		fromRunID, toRunID, err = pickRuns(ids, fromStr, toStr, now)
		if err != nil {
			return nil, err
		}
		fromRows, err := st.AttentionAt(ctx, fromRunID)
		if err != nil {
			return nil, apiErr(err)
		}
		toRows, err := st.AttentionAt(ctx, toRunID)
		if err != nil {
			return nil, apiErr(err)
		}
		return diffAttention(fromRunID, toRunID, fromRows, toRows), nil

	case "stale":
		ids, err := st.LatestRunIDs(ctx, "stale_runs", 50)
		if err != nil {
			return nil, apiErr(err)
		}
		fromRunID, toRunID, err = pickRuns(ids, fromStr, toStr, now)
		if err != nil {
			return nil, err
		}
		fromRows, err := st.StaleAt(ctx, fromRunID, store.StaleFilter{})
		if err != nil {
			return nil, apiErr(err)
		}
		toRows, err := st.StaleAt(ctx, toRunID, store.StaleFilter{})
		if err != nil {
			return nil, apiErr(err)
		}
		return diffStale(fromRunID, toRunID, fromRows, toRows), nil
	}
	return nil, usageErr(fmt.Errorf("unsupported metric"))
}

// pickRuns returns (fromRunID, toRunID). When fromStr/toStr are empty, picks
// the two most-recent run IDs. Otherwise resolves each via parseHumanTime
// and picks the run closest to the requested time.
func pickRuns(ids []string, fromStr, toStr string, now time.Time) (string, string, error) {
	if len(ids) == 0 {
		return "", "", notFoundErr(fmt.Errorf("no snapshots found — run 'attention' or 'sync stale' first"))
	}
	if len(ids) == 1 && fromStr == "" && toStr == "" {
		return "", "", notFoundErr(fmt.Errorf("only one snapshot exists; need at least two to compute drift"))
	}
	if fromStr == "" && toStr == "" {
		// Latest is index 0 (DESC order from LatestRunIDs); previous is index 1.
		return ids[1], ids[0], nil
	}
	pickClosest := func(s string) (string, error) {
		t, err := parseHumanTime(s, now)
		if err != nil {
			return "", usageErr(err)
		}
		return closestRun(ids, t), nil
	}
	from := ids[len(ids)-1]
	to := ids[0]
	if fromStr != "" {
		v, err := pickClosest(fromStr)
		if err != nil {
			return "", "", err
		}
		from = v
	}
	if toStr != "" {
		v, err := pickClosest(toStr)
		if err != nil {
			return "", "", err
		}
		to = v
	}
	return from, to, nil
}

// warnIfDriftFallback emits a stderr warning when an explicit --from / --to
// resolved to a snapshot more than 6 hours from the requested time. Silent
// substitution is the failure the Phase 4.85 review caught.
func warnIfDriftFallback(cmd *cobra.Command, st *store.Store, ctx context.Context, metric, fromStr, toStr string) {
	if fromStr == "" && toStr == "" {
		return // default behavior, not a fallback
	}
	runTable := metric + "_runs"
	switch runTable {
	case "attention_runs", "stale_runs":
	default:
		return
	}
	ids, err := st.LatestRunIDs(ctx, runTable, 50)
	if err != nil || len(ids) == 0 {
		return
	}
	const threshold = 6 * time.Hour
	now := time.Now()
	if fromStr != "" {
		if t, err := parseHumanTime(fromStr, now); err == nil {
			if _, delta := closestRunWithDelta(ids, t); delta > threshold {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: --from %q resolved to a snapshot %v away (closest snapshot is older/newer than 6h from requested time)\n", fromStr, delta.Round(time.Minute))
			}
		}
	}
	if toStr != "" {
		if t, err := parseHumanTime(toStr, now); err == nil {
			if _, delta := closestRunWithDelta(ids, t); delta > threshold {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: --to %q resolved to a snapshot %v away (closest snapshot is older/newer than 6h from requested time)\n", toStr, delta.Round(time.Minute))
			}
		}
	}
}

// closestRun returns the run_id whose embedded timestamp is closest to target.
// run_ids look like "20260511T143000.123Z" (UTC).
func closestRun(ids []string, target time.Time) string {
	best := ids[0]
	bestDiff := time.Duration(1<<62 - 1)
	for _, id := range ids {
		t, err := time.Parse("20060102T150405.000Z", id)
		if err != nil {
			continue
		}
		d := t.Sub(target)
		if d < 0 {
			d = -d
		}
		if d < bestDiff {
			best = id
			bestDiff = d
		}
	}
	return best
}

// closestRunWithDelta returns the closest run AND the absolute time offset from
// the requested target. Callers can warn when delta exceeds a sensible bound
// (e.g., "you asked for yesterday but the closest snapshot is 5 minutes ago").
func closestRunWithDelta(ids []string, target time.Time) (string, time.Duration) {
	best := closestRun(ids, target)
	t, err := time.Parse("20060102T150405.000Z", best)
	if err != nil {
		return best, 0
	}
	d := t.Sub(target)
	if d < 0 {
		d = -d
	}
	return best, d
}

func diffAttention(fromID, toID string, from, to []store.AttentionSnapshotRow) map[string]any {
	keyFn := func(r store.AttentionSnapshotRow) string {
		// Group by company per source so a company that recovered on one source
		// but degraded on another shows in both buckets.
		return r.Source + "|" + r.CompanyID + "|" + r.CompanyName
	}
	fromIdx := map[string]store.AttentionSnapshotRow{}
	for _, r := range from {
		fromIdx[keyFn(r)] = r
	}
	toIdx := map[string]store.AttentionSnapshotRow{}
	for _, r := range to {
		toIdx[keyFn(r)] = r
	}

	type rowOut struct {
		Source      string `json:"source"`
		CompanyID   string `json:"company_id,omitempty"`
		CompanyName string `json:"company,omitempty"`
		Reason      string `json:"reason"`
		Score       int    `json:"score"`
	}
	added := []rowOut{}
	removed := []rowOut{}
	unchangedCount := 0
	for k, r := range toIdx {
		if _, ok := fromIdx[k]; !ok {
			added = append(added, rowOut{r.Source, r.CompanyID, r.CompanyName, r.Reason, r.Score})
		} else {
			unchangedCount++
		}
	}
	for k, r := range fromIdx {
		if _, ok := toIdx[k]; !ok {
			removed = append(removed, rowOut{r.Source, r.CompanyID, r.CompanyName, r.Reason, r.Score})
		}
	}
	sort.Slice(added, func(i, j int) bool { return added[i].Score > added[j].Score })
	sort.Slice(removed, func(i, j int) bool { return removed[i].Score > removed[j].Score })
	return map[string]any{
		"meta": map[string]any{
			"metric":      "attention",
			"from_run_id": fromID,
			"to_run_id":   toID,
			"from_count":  len(from),
			"to_count":    len(to),
			"unchanged":   unchangedCount,
		},
		"added":   added,
		"removed": removed,
	}
}

func diffStale(fromID, toID string, from, to []store.StaleSnapshotRow) map[string]any {
	keyFn := func(r store.StaleSnapshotRow) string {
		return r.Company + "|" + r.BackupSet + "|" + r.BackupAccount
	}
	fromIdx := map[string]store.StaleSnapshotRow{}
	for _, r := range from {
		fromIdx[keyFn(r)] = r
	}
	toIdx := map[string]store.StaleSnapshotRow{}
	for _, r := range to {
		toIdx[keyFn(r)] = r
	}
	type rowOut struct {
		Company   string  `json:"company,omitempty"`
		BackupSet string  `json:"backup_set,omitempty"`
		Engine    string  `json:"engine,omitempty"`
		DaysStale float64 `json:"days_stale"`
	}
	added := []rowOut{}
	removed := []rowOut{}
	unchangedCount := 0
	for k, r := range toIdx {
		if _, ok := fromIdx[k]; !ok {
			added = append(added, rowOut{r.Company, r.BackupSet, r.Engine, r.DaysStale})
		} else {
			unchangedCount++
		}
	}
	for k, r := range fromIdx {
		if _, ok := toIdx[k]; !ok {
			removed = append(removed, rowOut{r.Company, r.BackupSet, r.Engine, r.DaysStale})
		}
	}
	sort.Slice(added, func(i, j int) bool { return added[i].DaysStale > added[j].DaysStale })
	sort.Slice(removed, func(i, j int) bool { return removed[i].DaysStale > removed[j].DaysStale })
	return map[string]any{
		"meta": map[string]any{
			"metric":      "stale",
			"from_run_id": fromID,
			"to_run_id":   toID,
			"from_count":  len(from),
			"to_count":    len(to),
			"unchanged":   unchangedCount,
		},
		"added":   added,
		"removed": removed,
	}
}
