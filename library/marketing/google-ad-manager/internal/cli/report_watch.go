// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-ad-manager/internal/cliutil"
	"github.com/spf13/cobra"
)

// reportRow is a flattened view of one fetched report row: a stable key built
// from the row's dimension values and a single numeric metric value pulled from
// the primary metric column selected by --metric. Diffing happens on this
// reduced shape so the logic is testable without the nested GAM row JSON.
type reportRow struct {
	Key   string  `json:"key"`
	Value float64 `json:"value"`
}

// reportChange is one row's movement between the cached run and the current run.
type reportChange struct {
	Key  string  `json:"key"`
	Prev float64 `json:"prev"`
	Curr float64 `json:"curr"`
	// Delta is curr-prev. Pct is the percent change vs prev (0 when prev==0 and
	// curr==0; 100 when a row appears from a zero/absent baseline with a nonzero
	// current value). New rows have prev 0; dropped rows have curr 0.
	Delta float64 `json:"delta"`
	Pct   float64 `json:"pct"`
}

// metricValueAt reads the metric at the given 0-based index from a fetched GAM
// report row's first (primary) metric value group. GAM rows are positional:
// primaryValues[i] corresponds to metrics[i] in the report definition. A value
// may arrive as doubleValue or as intValue (an int64 encoded as a string), so
// both are handled. Returns (value, true) when present and parseable.
func metricValueAt(row json.RawMessage, idx int) (float64, bool) {
	var parsed struct {
		MetricValueGroups []struct {
			PrimaryValues []struct {
				DoubleValue *float64 `json:"doubleValue"`
				IntValue    *string  `json:"intValue"`
			} `json:"primaryValues"`
		} `json:"metricValueGroups"`
	}
	if err := json.Unmarshal(row, &parsed); err != nil {
		return 0, false
	}
	if len(parsed.MetricValueGroups) == 0 {
		return 0, false
	}
	pv := parsed.MetricValueGroups[0].PrimaryValues
	if idx < 0 || idx >= len(pv) {
		return 0, false
	}
	v := pv[idx]
	switch {
	case v.DoubleValue != nil:
		return *v.DoubleValue, true
	case v.IntValue != nil:
		if f, err := strconv.ParseFloat(*v.IntValue, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

// rowKey builds a stable join key from a row's dimension values, in order. The
// dimension order is fixed by the report definition, so the same logical row
// produces the same key across runs.
func rowKey(row json.RawMessage) string {
	var parsed struct {
		DimensionValues []struct {
			StringValue *string `json:"stringValue"`
			IntValue    *string `json:"intValue"`
			BoolValue   *bool   `json:"boolValue"`
		} `json:"dimensionValues"`
	}
	if err := json.Unmarshal(row, &parsed); err != nil {
		return ""
	}
	parts := make([]string, 0, len(parsed.DimensionValues))
	for _, d := range parsed.DimensionValues {
		switch {
		case d.StringValue != nil:
			parts = append(parts, *d.StringValue)
		case d.IntValue != nil:
			parts = append(parts, *d.IntValue)
		case d.BoolValue != nil:
			parts = append(parts, strconv.FormatBool(*d.BoolValue))
		default:
			parts = append(parts, "")
		}
	}
	return strings.Join(parts, "␟")
}

// parseReportRows flattens raw GAM rows into keyed metric rows for the metric
// column at metricIndex. Rows whose metric value can't be parsed are skipped
// (they contribute no diff) rather than becoming a phantom zero.
func parseReportRows(rows []json.RawMessage, metricIndex int) []reportRow {
	out := make([]reportRow, 0, len(rows))
	for _, r := range rows {
		v, ok := metricValueAt(r, metricIndex)
		if !ok {
			continue
		}
		out = append(out, reportRow{Key: rowKey(r), Value: v})
	}
	return out
}

// diffReportRows compares two keyed metric snapshots and returns one change per
// key present in either snapshot, plus the subset whose absolute percent change
// meets or exceeds threshold ("flagged"). Pure and deterministic: callers test
// it directly. Keys present only in curr are treated as new (prev 0); keys only
// in prev are treated as dropped (curr 0).
func diffReportRows(prev, curr []reportRow, threshold float64) (changes, flagged []reportChange) {
	prevByKey := make(map[string]float64, len(prev))
	for _, r := range prev {
		prevByKey[r.Key] = r.Value
	}
	currByKey := make(map[string]float64, len(curr))
	for _, r := range curr {
		currByKey[r.Key] = r.Value
	}
	// Stable order: current rows first (in their order), then prev-only keys.
	seen := make(map[string]bool, len(curr))
	emit := func(key string, p, c float64) {
		ch := reportChange{Key: key, Prev: p, Curr: c, Delta: c - p}
		ch.Pct = pctChange(p, c)
		changes = append(changes, ch)
		if math.Abs(ch.Pct) >= threshold && threshold > 0 {
			flagged = append(flagged, ch)
		}
	}
	for _, r := range curr {
		if seen[r.Key] {
			continue
		}
		seen[r.Key] = true
		emit(r.Key, prevByKey[r.Key], r.Value)
	}
	for _, r := range prev {
		if seen[r.Key] {
			continue
		}
		seen[r.Key] = true
		emit(r.Key, r.Value, currByKey[r.Key]) // currByKey[r.Key] is 0 (dropped)
	}
	return changes, flagged
}

// pctChange returns the percent change from prev to curr. When prev is 0 it
// returns 0 if curr is also 0, else 100 (a row materializing from nothing).
func pctChange(prev, curr float64) float64 {
	if prev == 0 {
		if curr == 0 {
			return 0
		}
		return 100
	}
	return (curr - prev) / math.Abs(prev) * 100
}

// reportWatchCachePath returns the per-report cache file path under the user
// cache dir, e.g. <cache>/google-ad-manager-pp-cli/report-watch-<id>.json.
func reportWatchCachePath(reportID string) (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	// filepath.Base on the leaf strips any residual path separators so the
	// report id can only ever name a file inside the cache directory.
	leaf := filepath.Base("report-watch-" + reportID + ".json")
	return filepath.Join(dir, "google-ad-manager-pp-cli", leaf), nil
}

// pp:data-source live -- always fetches the report live from the GAM API; the
// local cache file holds only the prior snapshot used to compute the diff.
func newNovelReportWatchCmd(flags *rootFlags) *cobra.Command {
	var flagMetric string
	var flagThreshold float64
	var flagNetwork string
	var flagReportTimeout time.Duration

	cmd := &cobra.Command{
		Use:   "watch <report-id>",
		Short: "Re-run a saved report and diff it against the last cached run to surface what moved.",
		Long: `Re-run a saved report by ID, then diff the result against the previous run cached
on disk, surfacing which rows moved on a chosen metric column.

Rows are matched across runs by their dimension-value key. --metric selects which
primary metric column to diff: pass a 0-based column index (default 0 = the
report's first metric). GAM's fetchRows response is positional and carries no
metric-name headers, so a metric *name* cannot be resolved here — use the column
index. --threshold is an absolute percent-change cutoff: rows whose |pct| meets
or exceeds it are also reported under "flagged".

Output: {report_id, metric_index, changes:[{key,prev,curr,delta,pct}], flagged:[...]}.
The first run for a report has no baseline and emits all rows as the baseline
(with a note); the fresh result is cached afterward at
<user-cache>/google-ad-manager-pp-cli/report-watch-<id>.json.`,
		Example:     "  google-ad-manager-pp-cli report watch 1234567 --metric 0 --threshold 10",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rerun report <report-id> and diff against last cached run")
				return nil
			}
			// Bound by the report-timeout (async report runs take minutes), NOT
			// the root --timeout, whose 60s default would cut off polling.
			ctx, cancel := context.WithTimeout(cmd.Context(), flagReportTimeout+30*time.Second)
			defer cancel()

			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				return usageErr(fmt.Errorf("report id required: watch <report-id>"))
			}
			reportID := strings.TrimSpace(args[0])
			// GAM report IDs are numeric resource identifiers. Validating the
			// shape here keeps the value from being interpolated into the cache
			// file path as a path-traversal or file-inclusion vector.
			if _, perr := strconv.ParseInt(reportID, 10, 64); perr != nil {
				return usageErr(fmt.Errorf("report id must be numeric (got %q)", reportID))
			}

			// --metric is an optional 0-based column index; blank => 0.
			metricIndex := 0
			if m := strings.TrimSpace(flagMetric); m != "" {
				n, err := strconv.Atoi(m)
				if err != nil || n < 0 {
					return usageErr(fmt.Errorf("--metric must be a 0-based metric column index (got %q)", flagMetric))
				}
				metricIndex = n
			}

			code, err := resolveNetworkCode(flagNetwork)
			if err != nil {
				return err
			}

			limit := 1000
			if cliutil.IsDogfoodEnv() {
				limit = 25
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			reportName := networkParent(code) + "/reports/" + reportID

			rawRows, _, err := runReportAndFetch(ctx, c, reportName, limit, flagReportTimeout)
			if err != nil {
				return err
			}
			curr := parseReportRows(rawRows, metricIndex)

			cachePath, err := reportWatchCachePath(reportID)
			if err != nil {
				return apiErr(fmt.Errorf("resolving cache path: %w", err))
			}

			prev, hadCache := loadReportWatchCache(cachePath)

			// Persist the fresh snapshot for the next run regardless of outcome.
			if werr := saveReportWatchCache(cachePath, curr); werr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not write watch cache %s: %v\n", cachePath, werr)
			}

			if !hadCache {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"report_id":    reportID,
					"metric_index": metricIndex,
					"note":         "no prior cached run; emitting current rows as baseline",
					"baseline":     curr,
					"row_count":    len(curr),
				}, flags)
			}

			changes, flagged := diffReportRows(prev, curr, flagThreshold)
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"report_id":    reportID,
				"metric_index": metricIndex,
				"threshold":    flagThreshold,
				"changes":      changes,
				"flagged":      flagged,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&flagMetric, "metric", "", "0-based primary metric column index to diff (default 0 = first metric).")
	cmd.Flags().Float64Var(&flagThreshold, "threshold", 0, "Absolute percent-change cutoff; rows with |pct| >= this are flagged.")
	cmd.Flags().StringVar(&flagNetwork, "network", "", "GAM network code (else $GOOGLE_AD_MANAGER_NETWORK_CODE).")
	cmd.Flags().DurationVar(&flagReportTimeout, "report-timeout", 300*time.Second, "Max time to poll the async report run before giving up. Governs the whole run/fetch and is NOT bounded by --timeout; large reports may need a higher value.")
	return cmd
}

// loadReportWatchCache reads a cached snapshot. The second return is false when
// no usable cache exists (absent or unparseable), signalling a baseline run.
func loadReportWatchCache(path string) ([]reportRow, bool) {
	// #nosec G304 -- path is always reportWatchCachePath's output: a filepath.Base'd
	// leaf built from a numeric-validated report id, under the user cache dir. It
	// cannot be steered outside that directory by caller input.
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var rows []reportRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, false
	}
	return rows, true
}

// saveReportWatchCache writes the snapshot, creating the cache directory.
// The cache can hold revenue and delivery figures, so it is kept user-private
// (0700 dir, 0600 file) rather than world-readable.
func saveReportWatchCache(path string, rows []reportRow) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(rows)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
