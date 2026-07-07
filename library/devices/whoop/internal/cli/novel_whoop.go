// Whoop transcendence commands. Hand-authored, not generated.
// All read from the local SQLite store populated by `sync`.

package cli

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/whoop/internal/store"

	"github.com/spf13/cobra"
)

// scoreOf walks a JSON cycle/recovery/sleep/workout row and returns the
// score field requested. Returns NaN if the path is absent so callers can
// skip incomplete rows without polluting averages.
func scoreOf(raw json.RawMessage, path ...string) float64 {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return math.NaN()
	}
	var cur any = obj
	for _, k := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return math.NaN()
		}
		cur = m[k]
	}
	if f, ok := cur.(float64); ok {
		return f
	}
	return math.NaN()
}

// startTimeOf parses the "start" ISO8601 field. Zero time on miss.
func startTimeOf(raw json.RawMessage) time.Time {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return time.Time{}
	}
	s, _ := obj["start"].(string)
	if s == "" {
		// PATCH(analytics-readers-use-sync-store-names): recovery records carry no "start" field — fall back to created_at so time-windowed analytics can see them.
		s, _ = obj["created_at"].(string)
	}
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func openWhoopStore(ctx context.Context) (*store.Store, error) {
	db, err := store.OpenWithContext(ctx, defaultDBPath("whoop-pp-cli"))
	if err != nil {
		return nil, fmt.Errorf("opening local store: %w\nRun 'whoop-pp-cli sync' first.", err)
	}
	return db, nil
}

func emit(cmd *cobra.Command, flags *rootFlags, v any) error {
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// ---------------- trend ----------------

func newTrendCmd(flags *rootFlags) *cobra.Command {
	var weeks int
	var metric string
	cmd := &cobra.Command{
		Use:   "trend",
		Short: "Multi-week rollup of strain, recovery, or sleep",
		Long: `Aggregates locally synced cycles, recoveries, and sleeps into weekly
buckets and reports min/avg/max for the chosen metric.

Run 'sync' first to populate the local store.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  whoop-pp-cli trend --weeks 12 --metric strain
  whoop-pp-cli trend --weeks 4 --metric recovery --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openWhoopStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			cutoff := time.Now().AddDate(0, 0, -7*weeks)
			rows, err := loadMetricSeries(db, metric, cutoff)
			if err != nil {
				return err
			}
			buckets := bucketByISOWeek(rows)
			out := make([]map[string]any, 0, len(buckets))
			keys := make([]string, 0, len(buckets))
			for k := range buckets {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				vs := buckets[k]
				lo, hi, sum := math.Inf(1), math.Inf(-1), 0.0
				for _, v := range vs {
					if v < lo {
						lo = v
					}
					if v > hi {
						hi = v
					}
					sum += v
				}
				avg := sum / float64(len(vs))
				out = append(out, map[string]any{
					"week": k, "n": len(vs),
					"min": round1(lo), "avg": round1(avg), "max": round1(hi),
				})
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "No data. Run 'whoop-pp-cli sync' first.")
				return nil
			}
			return emit(cmd, flags, map[string]any{"metric": metric, "weeks": weeks, "buckets": out})
		},
	}
	cmd.Flags().IntVar(&weeks, "weeks", 12, "Number of weeks to roll up")
	cmd.Flags().StringVar(&metric, "metric", "strain", "Metric: strain, recovery, sleep")
	return cmd
}

func loadMetricSeries(db *store.Store, metric string, since time.Time) ([]struct {
	t time.Time
	v float64
}, error) {
	var resource string
	var path []string
	switch metric {
	case "strain":
		resource, path = "cycle", []string{"score", "strain"}
	case "recovery":
		// PATCH(analytics-readers-use-sync-store-names): stored under "recovery" (GET /v2/recovery); "cycle_recovery" is the per-cycle endpoint sync never writes.
		resource, path = "recovery", []string{"score", "recovery_score"}
	case "sleep":
		// PATCH(analytics-readers-use-sync-store-names): sleep records sync under "activity" (GET /v2/activity/sleep); there is no "sleep" sync resource.
		resource, path = "activity", []string{"score", "sleep_performance_percentage"}
	default:
		return nil, fmt.Errorf("unknown metric %q (use strain, recovery, sleep)", metric)
	}
	items, err := db.List(resource, 0)
	if err != nil {
		return nil, err
	}
	var out []struct {
		t time.Time
		v float64
	}
	for _, raw := range items {
		v := scoreOf(raw, path...)
		if math.IsNaN(v) {
			continue
		}
		t := startTimeOf(raw)
		if t.IsZero() || t.Before(since) {
			continue
		}
		out = append(out, struct {
			t time.Time
			v float64
		}{t, v})
	}
	return out, nil
}

func bucketByISOWeek(rows []struct {
	t time.Time
	v float64
}) map[string][]float64 {
	out := map[string][]float64{}
	for _, r := range rows {
		y, w := r.t.ISOWeek()
		k := fmt.Sprintf("%d-W%02d", y, w)
		out[k] = append(out[k], r.v)
	}
	return out
}

func round1(f float64) float64 {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return 0
	}
	return math.Round(f*10) / 10
}

// ---------------- digest ----------------

func newDigestCmd(flags *rootFlags) *cobra.Command {
	var since string
	var redact bool
	var format string
	cmd := &cobra.Command{
		Use:         "digest",
		Short:       "Coach-mode shareable digest (Markdown or JSON)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     `  whoop-pp-cli digest --since 7d --redact-pii`,
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := parseDuration(since)
			if err != nil {
				return err
			}
			cutoff := time.Now().Add(-d)
			db, err := openWhoopStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			cycles, _ := loadMetricSeries(db, "strain", cutoff)
			recs, _ := loadMetricSeries(db, "recovery", cutoff)
			sleeps, _ := loadMetricSeries(db, "sleep", cutoff)
			summary := map[string]any{
				"window":   since,
				"strain":   summarize(cycles),
				"recovery": summarize(recs),
				"sleep":    summarize(sleeps),
			}
			if !redact {
				if profile, _ := db.List("user", 1); len(profile) > 0 {
					summary["profile"] = json.RawMessage(profile[0])
				}
			}
			if format == "markdown" {
				out := cmd.OutOrStdout()
				fmt.Fprintf(out, "# Whoop digest (last %s)\n\n", since)
				for _, m := range []string{"strain", "recovery", "sleep"} {
					s := summary[m].(map[string]any)
					fmt.Fprintf(out, "**%s** — n=%d, avg=%.1f, min=%.1f, max=%.1f\n\n",
						strings.Title(m), s["n"], s["avg"], s["min"], s["max"])
				}
				return nil
			}
			return emit(cmd, flags, summary)
		},
	}
	cmd.Flags().StringVar(&since, "since", "7d", "Window: 24h, 7d, 30d")
	cmd.Flags().BoolVar(&redact, "redact-pii", false, "Omit profile fields")
	cmd.Flags().StringVar(&format, "format", "json", "Output: json or markdown")
	return cmd
}

func summarize(rows []struct {
	t time.Time
	v float64
}) map[string]any {
	if len(rows) == 0 {
		return map[string]any{"n": 0, "avg": 0.0, "min": 0.0, "max": 0.0}
	}
	lo, hi, sum := math.Inf(1), math.Inf(-1), 0.0
	for _, r := range rows {
		if r.v < lo {
			lo = r.v
		}
		if r.v > hi {
			hi = r.v
		}
		sum += r.v
	}
	return map[string]any{"n": len(rows), "avg": round1(sum / float64(len(rows))), "min": round1(lo), "max": round1(hi)}
}

func parseDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		var n int
		_, err := fmt.Sscanf(s, "%dd", &n)
		if err != nil {
			return 0, err
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

// ---------------- classify ----------------

// Workouts in Whoop carry a sport_id; misclassification is common because
// auto-detection uses HR alone. We re-score by comparing the workout's
// HR profile (avg, max, ratio) to the user's other workouts of each sport
// and flag rows where the nearest neighbor isn't the assigned sport.
func newClassifyCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "classify",
		Short:       "Flag potentially mislabeled workouts via HR-shape heuristics",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openWhoopStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			// PATCH(analytics-readers-use-sync-store-names): workouts live under "activity-workout"; "activity" holds sleeps.
			items, err := db.List("activity-workout", 0)
			if err != nil {
				return err
			}
			type wo struct {
				id      string
				sportID float64
				avgHR   float64
				maxHR   float64
				strain  float64
			}
			var wos []wo
			for _, raw := range items {
				var obj map[string]any
				if json.Unmarshal(raw, &obj) != nil {
					continue
				}
				id, _ := obj["id"].(string)
				sid, _ := obj["sport_id"].(float64)
				avg := scoreOf(raw, "score", "average_heart_rate")
				mx := scoreOf(raw, "score", "max_heart_rate")
				st := scoreOf(raw, "score", "strain")
				if math.IsNaN(avg) || math.IsNaN(mx) {
					continue
				}
				wos = append(wos, wo{id, sid, avg, mx, st})
			}
			// Per-sport mean HR profile
			means := map[float64][2]float64{}
			counts := map[float64]int{}
			for _, w := range wos {
				m := means[w.sportID]
				means[w.sportID] = [2]float64{m[0] + w.avgHR, m[1] + w.maxHR}
				counts[w.sportID]++
			}
			for s, m := range means {
				means[s] = [2]float64{m[0] / float64(counts[s]), m[1] / float64(counts[s])}
			}
			var flagged []map[string]any
			for _, w := range wos {
				assigned := means[w.sportID]
				assignedDist := math.Hypot(w.avgHR-assigned[0], w.maxHR-assigned[1])
				bestSport := w.sportID
				bestDist := assignedDist
				for s, m := range means {
					d := math.Hypot(w.avgHR-m[0], w.maxHR-m[1])
					if d < bestDist {
						bestDist = d
						bestSport = s
					}
				}
				if bestSport != w.sportID && counts[bestSport] >= 3 {
					flagged = append(flagged, map[string]any{
						"workout_id": w.id,
						"assigned":   w.sportID,
						"suggested":  bestSport,
						"avg_hr":     w.avgHR,
						"distance":   round1(bestDist),
						"strain":     w.strain,
					})
				}
			}
			if len(flagged) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "No mislabeled workouts detected.")
				return emit(cmd, flags, map[string]any{"flagged": []any{}})
			}
			return emit(cmd, flags, map[string]any{"flagged": flagged, "n": len(flagged)})
		},
	}
	return cmd
}

// ---------------- correlate ----------------

func newCorrelateCmd(flags *rootFlags) *cobra.Command {
	var lag int
	cmd := &cobra.Command{
		Use:         "correlate <a> <b>",
		Short:       "Pearson correlation between two daily metrics",
		Long:        "Metrics: strain, recovery, sleep. Optional --lag in days.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     `  whoop-pp-cli correlate recovery strain --lag 1`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if len(args) != 2 {
				return fmt.Errorf("need exactly two metric names")
			}
			db, err := openWhoopStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			cutoff := time.Now().AddDate(-1, 0, 0)
			a, err := loadMetricSeries(db, args[0], cutoff)
			if err != nil {
				return err
			}
			b, err := loadMetricSeries(db, args[1], cutoff)
			if err != nil {
				return err
			}
			amap := map[string]float64{}
			for _, r := range a {
				amap[r.t.Format("2006-01-02")] = r.v
			}
			var xs, ys []float64
			for _, r := range b {
				key := r.t.AddDate(0, 0, -lag).Format("2006-01-02")
				if v, ok := amap[key]; ok {
					xs = append(xs, v)
					ys = append(ys, r.v)
				}
			}
			if len(xs) < 3 {
				fmt.Fprintf(cmd.ErrOrStderr(), "Insufficient data (need 3 paired observations, got %d). Run sync first.\n", len(xs))
				return emit(cmd, flags, map[string]any{
					"a": args[0], "b": args[1], "lag_days": lag,
					"n": len(xs), "r": 0.0, "interpretation": "insufficient data",
				})
			}
			r := pearson(xs, ys)
			return emit(cmd, flags, map[string]any{
				"a": args[0], "b": args[1], "lag_days": lag,
				"n": len(xs), "r": round1(r*100) / 100,
				"interpretation": pearsonInterp(r),
			})
		},
	}
	cmd.Flags().IntVar(&lag, "lag", 0, "Days to lag B behind A (e.g. -lag 1: today's recovery vs yesterday's strain)")
	return cmd
}

func pearson(xs, ys []float64) float64 {
	var sx, sy float64
	for i := range xs {
		sx += xs[i]
		sy += ys[i]
	}
	mx, my := sx/float64(len(xs)), sy/float64(len(ys))
	var num, dx2, dy2 float64
	for i := range xs {
		dx, dy := xs[i]-mx, ys[i]-my
		num += dx * dy
		dx2 += dx * dx
		dy2 += dy * dy
	}
	d := math.Sqrt(dx2 * dy2)
	if d == 0 {
		return 0
	}
	return num / d
}

func pearsonInterp(r float64) string {
	a := math.Abs(r)
	switch {
	case a < 0.1:
		return "no relationship"
	case a < 0.3:
		return "weak"
	case a < 0.5:
		return "moderate"
	case a < 0.7:
		return "strong"
	default:
		return "very strong"
	}
}

// ---------------- sleep-debt ----------------

func newSleepDebtCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "sleep-debt",
		Short:       "Rolling 14-day sleep debt and tomorrow's recovery prediction",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openWhoopStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			// PATCH(analytics-readers-use-sync-store-names): sleeps sync under the "activity" resource.
			items, err := db.List("activity", 0)
			if err != nil {
				return err
			}
			cutoff := time.Now().AddDate(0, 0, -14)
			var debt float64
			var n int
			for _, raw := range items {
				if startTimeOf(raw).Before(cutoff) {
					continue
				}
				perf := scoreOf(raw, "score", "sleep_performance_percentage")
				need := scoreOf(raw, "score", "stage_summary", "total_in_bed_time_milli")
				if math.IsNaN(perf) {
					continue
				}
				_ = need
				debt += (100 - perf)
				n++
			}
			recs, _ := loadMetricSeries(db, "recovery", cutoff)
			recAvg := 0.0
			if len(recs) > 0 {
				for _, r := range recs {
					recAvg += r.v
				}
				recAvg /= float64(len(recs))
			}
			// Naive prediction: baseline recovery minus debt penalty.
			predicted := recAvg - (debt/float64(max(n, 1)))*0.3
			if predicted < 0 {
				predicted = 0
			}
			band := "green"
			if predicted < 67 {
				band = "yellow"
			}
			if predicted < 34 {
				band = "red"
			}
			return emit(cmd, flags, map[string]any{
				"window_days":        14,
				"sleep_debt_pct_pts": round1(debt),
				"observations":       n,
				"baseline_recovery":  round1(recAvg),
				"predicted_recovery": round1(predicted),
				"predicted_band":     band,
			})
		},
	}
	return cmd
}

// ---------------- strain-budget ----------------

func newStrainBudgetCmd(flags *rootFlags) *cobra.Command {
	var weeklyTarget float64
	cmd := &cobra.Command{
		Use:         "strain-budget",
		Short:       "Recommend today's strain ceiling given weekly target and current recovery",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     `  whoop-pp-cli strain-budget --weekly-target 70`,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openWhoopStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			weekStart := time.Now().Truncate(24*time.Hour).AddDate(0, 0, -int(time.Now().Weekday()))
			cycles, err := loadMetricSeries(db, "strain", weekStart)
			if err != nil {
				return err
			}
			used := 0.0
			for _, c := range cycles {
				used += c.v
			}
			daysLeft := 7 - int(time.Now().Weekday())
			if daysLeft < 1 {
				daysLeft = 1
			}
			remaining := weeklyTarget - used
			if remaining < 0 {
				remaining = 0
			}
			perDay := remaining / float64(daysLeft)
			recs, _ := loadMetricSeries(db, "recovery", time.Now().AddDate(0, 0, -1))
			latestRec := 0.0
			if len(recs) > 0 {
				sort.Slice(recs, func(i, j int) bool { return recs[i].t.After(recs[j].t) })
				latestRec = recs[0].v
			}
			// Scale by recovery: green allows full, yellow x0.7, red x0.4.
			scale := 1.0
			band := "green"
			switch {
			case latestRec < 34:
				scale = 0.4
				band = "red"
			case latestRec < 67:
				scale = 0.7
				band = "yellow"
			}
			ceiling := round1(perDay * scale)
			return emit(cmd, flags, map[string]any{
				"weekly_target":   weeklyTarget,
				"used_so_far":     round1(used),
				"days_left":       daysLeft,
				"todays_ceiling":  ceiling,
				"latest_recovery": round1(latestRec),
				"recovery_band":   band,
				"explain":         fmt.Sprintf("Remaining strain %.1f over %d days = %.1f/day, scaled by %s recovery (x%.1f)", remaining, daysLeft, perDay, band, scale),
			})
		},
	}
	cmd.Flags().Float64Var(&weeklyTarget, "weekly-target", 70, "Target weekly strain")
	return cmd
}

// ---------------- diff ----------------

func newDiffCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "diff <weekA> <weekB>",
		Short:       "Compare two ISO weeks (e.g. 2026-W18 2026-W19) — strain, recovery, sleep deltas",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if len(args) != 2 {
				return fmt.Errorf("need two ISO week labels (e.g. 2026-W18 2026-W19)")
			}
			db, err := openWhoopStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			out := map[string]any{"a": args[0], "b": args[1]}
			for _, m := range []string{"strain", "recovery", "sleep"} {
				rows, _ := loadMetricSeries(db, m, time.Now().AddDate(-1, 0, 0))
				bs := bucketByISOWeek(rows)
				avgA, avgB := mean(bs[args[0]]), mean(bs[args[1]])
				out[m] = map[string]any{
					"a_avg": round1(avgA), "b_avg": round1(avgB),
					"delta": round1(avgB - avgA),
				}
			}
			return emit(cmd, flags, out)
		},
	}
	return cmd
}

func mean(vs []float64) float64 {
	if len(vs) == 0 {
		return 0
	}
	s := 0.0
	for _, v := range vs {
		s += v
	}
	return s / float64(len(vs))
}

// ---------------- watch ----------------

func newWatchCmd(flags *rootFlags) *cobra.Command {
	var port int
	var secret string
	var hook string
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Run a local HTTP server that receives Whoop webhooks and runs a shell hook",
		Long: `Starts an HTTP server on --port that validates HMAC-SHA256-signed Whoop
webhook deliveries, prints them as JSONL to stdout, and optionally invokes
a shell command (--hook) per delivery. Configure the listener URL in the
Whoop developer dashboard pointing at this host:port/webhook.

Side effects gated: requires --listen to actually start the server.`,
		Annotations: map[string]string{},
		Example: `  whoop-pp-cli watch --listen --port 9876 --secret $WHOOP_WEBHOOK_SECRET \
    --hook 'osascript -e "display notification \"new whoop event\""'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			listen, _ := cmd.Flags().GetBool("listen")
			if isVerifyEnv() || !listen {
				return emit(cmd, flags, map[string]any{
					"would_listen": fmt.Sprintf(":%d/webhook", port),
					"hook":         hook,
					"hint":         "Pass --listen to actually start the server.",
				})
			}
			mux := http.NewServeMux()
			mux.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				if secret != "" {
					sig := r.Header.Get("X-WHOOP-Signature")
					mac := hmac.New(sha256.New, []byte(secret))
					mac.Write(body)
					expected := hex.EncodeToString(mac.Sum(nil))
					if !hmac.Equal([]byte(sig), []byte(expected)) {
						http.Error(w, "bad signature", http.StatusUnauthorized)
						return
					}
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(body))
				if hook != "" {
					_ = exec.Command("sh", "-c", hook).Run()
				}
				w.WriteHeader(http.StatusOK)
			})
			addr := fmt.Sprintf(":%d", port)
			fmt.Fprintf(cmd.ErrOrStderr(), "Listening on %s/webhook\n", addr)
			return http.ListenAndServe(addr, mux)
		},
	}
	cmd.Flags().IntVar(&port, "port", 9876, "Listen port")
	cmd.Flags().StringVar(&secret, "secret", os.Getenv("WHOOP_WEBHOOK_SECRET"), "HMAC secret (defaults to $WHOOP_WEBHOOK_SECRET)")
	cmd.Flags().StringVar(&hook, "hook", "", "Shell command to run per delivery")
	cmd.Flags().Bool("listen", false, "Actually start the server (default: print plan only)")
	return cmd
}

func isVerifyEnv() bool {
	return os.Getenv("PRINTING_PRESS_VERIFY") == "1"
}

// ---------------- journal ----------------

// Journal answers ride on the recovery payload as a `survey_responses`
// array. We cross-reference present-vs-absent for a given question and
// report the mean recovery delta.
func newJournalCmd(flags *rootFlags) *cobra.Command {
	var question string
	cmd := &cobra.Command{
		Use:         "journal",
		Short:       "Cross-reference Whoop Journal answers against recovery deltas",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     `  whoop-pp-cli journal --question "alcohol"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if question == "" {
				return cmd.Help()
			}
			db, err := openWhoopStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			// PATCH(analytics-readers-use-sync-store-names): recoveries sync under "recovery", not "cycle_recovery".
			items, err := db.List("recovery", 0)
			if err != nil {
				return err
			}
			var withYes, withNo []float64
			for _, raw := range items {
				score := scoreOf(raw, "score", "recovery_score")
				if math.IsNaN(score) {
					continue
				}
				var obj map[string]any
				if json.Unmarshal(raw, &obj) != nil {
					continue
				}
				resp, _ := obj["survey_responses"].([]any)
				answered := false
				yes := false
				for _, r := range resp {
					m, _ := r.(map[string]any)
					q, _ := m["question"].(string)
					if strings.Contains(strings.ToLower(q), strings.ToLower(question)) {
						answered = true
						if v, ok := m["answered_yes"].(bool); ok {
							yes = v
						}
					}
				}
				if !answered {
					continue
				}
				if yes {
					withYes = append(withYes, score)
				} else {
					withNo = append(withNo, score)
				}
			}
			if len(withYes)+len(withNo) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "No journal entries found for that question.")
				return emit(cmd, flags, map[string]any{"matches": 0})
			}
			return emit(cmd, flags, map[string]any{
				"question":         question,
				"yes_n":            len(withYes),
				"no_n":             len(withNo),
				"yes_avg_recovery": round1(mean(withYes)),
				"no_avg_recovery":  round1(mean(withNo)),
				"delta":            round1(mean(withYes) - mean(withNo)),
			})
		},
	}
	cmd.Flags().StringVar(&question, "question", "", "Substring of the journal question (e.g. 'alcohol')")
	return cmd
}

// silence unused
var _ = sql.ErrNoRows
