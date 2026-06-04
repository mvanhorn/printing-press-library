// Copyright 2026 Giuliano Giacaglia and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/cloud/render/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/cloud/render/internal/store"

	"github.com/spf13/cobra"
)

// pp:novel-static-reference
//
// Plan capacity reference (CPU cores, memory MiB) for Render service plans.
// Sourced from https://render.com/pricing (October 2025). These values
// approximate plan ceilings for p95 utilization calculation; verify in the
// dashboard before acting on absolute numbers.
var renderPlanCapacity = map[string]struct {
	CPU    float64 // cores
	Memory float64 // MiB
}{
	"free":      {CPU: 0.1, Memory: 256},
	"starter":   {CPU: 0.5, Memory: 512},
	"standard":  {CPU: 1, Memory: 2048},
	"pro":       {CPU: 2, Memory: 4096},
	"pro-plus":  {CPU: 4, Memory: 8192},
	"pro-max":   {CPU: 4, Memory: 16384},
	"pro-ultra": {CPU: 8, Memory: 32768},
}

// rightsizeRow is the per-service result.
type rightsizeRow struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Plan           string  `json:"plan,omitempty"`
	P95CPUPercent  float64 `json:"p95_cpu_percent"`
	P95MemPercent  float64 `json:"p95_mem_percent"`
	Recommendation string  `json:"recommendation"`
}

type rightsizeReport struct {
	Since   string           `json:"since"`
	High    float64          `json:"high"`
	Low     float64          `json:"low"`
	Rows    []rightsizeRow   `json:"rows"`
	Skipped []map[string]any `json:"skipped"`
}

func newRightsizeCmd(flags *rootFlags) *cobra.Command {
	var (
		since  string
		high   float64
		low    float64
		limit  int
		dbPath string
	)
	cmd := &cobra.Command{
		Use:   "rightsize",
		Short: "Compute p95 CPU and memory utilization for each cached service against plan capacity; recommend up/down sizing.",
		Example: strings.Trim(`
  render-pp-cli rightsize
  render-pp-cli rightsize --since 14d --high 85 --low 15 --limit 25
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run": true, "command": "rightsize"}`)
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("render-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nlocal cache empty — run 'render-pp-cli sync' first", err)
			}
			defer db.Close()

			services, err := loadResourceItems(db, "services")
			if err != nil {
				return err
			}
			if len(services) == 0 {
				return fmt.Errorf("no services in cache — run 'render-pp-cli sync' first")
			}
			if limit > 0 && len(services) > limit {
				services = services[:limit]
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			limiter := cliutil.NewAdaptiveLimiter(flags.rateLimit)
			start, err := parseTimelineWindow(since, time.Now().UTC())
			if err != nil {
				return fmt.Errorf("--since: %w", err)
			}
			startStr := start.Format(time.RFC3339)
			endStr := time.Now().UTC().Format(time.RFC3339)

			rep := rightsizeReport{Since: since, High: high, Low: low, Rows: []rightsizeRow{}, Skipped: []map[string]any{}}
			for _, s := range services {
				plan := lookupServicePlan(db, s.ID)
				cap, hasCap := renderPlanCapacity[plan]

				limiter.Wait()
				cpuP95, cpuErr := fetchP95(c, "/metrics/cpu", s.ID, startStr, endStr)
				if cpuErr != nil {
					rep.Skipped = append(rep.Skipped, map[string]any{"id": s.ID, "name": s.Name, "reason": "cpu fetch: " + cpuErr.Error()})
					continue
				}

				limiter.Wait()
				memP95, memErr := fetchP95(c, "/metrics/memory", s.ID, startStr, endStr)
				if memErr != nil {
					rep.Skipped = append(rep.Skipped, map[string]any{"id": s.ID, "name": s.Name, "reason": "memory fetch: " + memErr.Error()})
					continue
				}

				cpuPct := 0.0
				memPct := 0.0
				if hasCap {
					if cap.CPU > 0 {
						cpuPct = (cpuP95 / cap.CPU) * 100
					}
					if cap.Memory > 0 {
						memPct = (memP95 / cap.Memory) * 100
					}
				}
				rec := classifyRightsize(cpuPct, memPct, high, low)
				rep.Rows = append(rep.Rows, rightsizeRow{
					ID:             s.ID,
					Name:           s.Name,
					Plan:           plan,
					P95CPUPercent:  roundTo(cpuPct, 1),
					P95MemPercent:  roundTo(memPct, 1),
					Recommendation: rec,
				})
			}

			sort.Slice(rep.Rows, func(i, j int) bool { return rep.Rows[i].P95CPUPercent > rep.Rows[j].P95CPUPercent })

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), rep, flags)
			}
			if len(rep.Rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No services produced metrics.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-18s %-30s %-10s %-8s %-8s %s\n", "ID", "NAME", "PLAN", "CPU%", "MEM%", "RECOMMENDATION")
			for _, r := range rep.Rows {
				name := r.Name
				if len(name) > 28 {
					name = name[:25] + "..."
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-18s %-30s %-10s %-8.1f %-8.1f %s\n", r.ID, name, r.Plan, r.P95CPUPercent, r.P95MemPercent, r.Recommendation)
			}
			if len(rep.Skipped) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\nSkipped %d services (errors fetching metrics).\n", len(rep.Skipped))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "7d", "Window for metrics: duration (7d, 14d) or RFC3339 timestamp")
	cmd.Flags().Float64Var(&high, "high", 80, "Percent threshold above which the plan is considered saturated")
	cmd.Flags().Float64Var(&low, "low", 20, "Percent threshold below which the plan is considered overprovisioned")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum services to evaluate (rate-limited round-trip per service)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/render-pp-cli/data.db)")
	return cmd
}

// classifyRightsize returns a one-line recommendation given CPU/memory p95
// percentages and the configured thresholds.
func classifyRightsize(cpuPct, memPct, high, low float64) string {
	if cpuPct >= high || memPct >= high {
		return "upsize: plan saturated (cpu or mem p95 above high threshold)"
	}
	if cpuPct <= low && memPct <= low && cpuPct > 0 && memPct > 0 {
		return "downsize: plan overprovisioned (both metrics below low threshold)"
	}
	return "ok"
}

// fetchP95 queries the metrics endpoint and computes the p95 of the
// returned numeric samples. Render returns either a flat array or a values
// array nested under "values"; we accept either.
func fetchP95(c interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
}, path, resource, start, end string) (float64, error) {
	params := map[string]string{
		"resource":  resource,
		"startTime": start,
		"endTime":   end,
	}
	data, err := c.Get(path, params)
	if err != nil {
		return 0, err
	}
	values := extractMetricValues(data)
	if len(values) == 0 {
		return 0, nil
	}
	return percentile(values, 0.95), nil
}

// extractMetricValues returns the numeric samples from a Render metrics
// payload. Walks both array and object shapes, falling back through the
// common nested paths so the diff is robust to Render's heterogeneous
// metric responses.
func extractMetricValues(data json.RawMessage) []float64 {
	out := []float64{}
	walk(data, func(v any) {
		switch t := v.(type) {
		case float64:
			out = append(out, t)
		case map[string]any:
			if val, ok := t["value"].(float64); ok {
				out = append(out, val)
			}
		}
	})
	return out
}

// walk recursively visits every JSON value invoking fn on each. Used to
// flatten nested Render metrics payloads before percentile computation.
func walk(data json.RawMessage, fn func(any)) {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return
	}
	walkValue(v, fn)
}

func walkValue(v any, fn func(any)) {
	fn(v)
	switch t := v.(type) {
	case []any:
		for _, e := range t {
			walkValue(e, fn)
		}
	case map[string]any:
		for _, e := range t {
			walkValue(e, fn)
		}
	}
}

func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sort.Float64s(values)
	idx := int(float64(len(values)-1) * p)
	return values[idx]
}

func roundTo(f float64, places int) float64 {
	pow := 1.0
	for i := 0; i < places; i++ {
		pow *= 10
	}
	return float64(int(f*pow+0.5)) / pow
}

// lookupServicePlan returns the plan field for a service id, or "" when the
// service isn't cached.
func lookupServicePlan(db *store.Store, id string) string {
	var raw []byte
	err := db.DB().QueryRow(`SELECT data FROM resources WHERE resource_type = 'services' AND id = ?`, id).Scan(&raw)
	if err != nil {
		return ""
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	return strFromAny(obj["plan"])
}
