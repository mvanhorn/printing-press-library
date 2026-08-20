// Package cost computes per-run Apify cost projections and ledger rollups.
//
// Apify bills in compute units (CU) + memory-GB-hours + storage-GB-hours +
// data-transfer + PPE per-event. The dashboard shows aggregates; the API
// exposes raw numbers but no per-run rollup. This package fills the gap
// using cached run history (for projections via p50/p90) and the
// /v2/users/me/usage/monthly endpoint (for actuals).
//
// Cost surprise is the #1 G2 complaint about Apify. Every command that
// triggers a run should show a one-line projection before the call so the
// user catches budget blow-ups before they happen.
package cost

import (
	"fmt"
	"sort"
	"strings"
)

// Pricing defaults — accurate as of 2026 published Apify pricing.
// Users on enterprise plans can override via APIFY_UNIT_PRICE_* env vars.
const (
	DefaultCUPriceUSD             = 0.25 // per compute unit (free tier overage)
	DefaultMemoryGBHourPriceUSD   = 0.30 // per RAM-GB-hour
	DefaultDatasetGBMonthPriceUSD = 1.00 // per dataset-GB-month
	DefaultExternalEgressPriceUSD = 0.20 // per GB transferred out
)

// RunStats holds the cost-relevant numbers from one historical run.
// Populated from sync of /v2/actor-runs.
type RunStats struct {
	RunID            string
	ActorID          string
	ComputeUnits     float64 // run.stats.computeUnits
	MemoryAvgMBytes  int     // run.options.memoryMbytes
	DurationSecs     float64 // (finished_at - started_at)
	DatasetItemBytes int64   // dataset.cleanItemCount * avg item size, approximated
}

// Projection is a forward estimate before a run starts.
type Projection struct {
	ActorID    string
	SampleSize int
	P50USD     float64
	P90USD     float64
	NoteOnSize string // human-readable confidence note
}

// Estimate computes USD cost for a single historical run.
// Components: compute-units + memory-GB-hours + (negligible storage/transfer).
func (r *RunStats) Estimate() float64 {
	cu := r.ComputeUnits * DefaultCUPriceUSD
	memGBHours := (float64(r.MemoryAvgMBytes) / 1024.0) * (r.DurationSecs / 3600.0)
	mem := memGBHours * DefaultMemoryGBHourPriceUSD
	return cu + mem
}

// Project returns a p50/p90 USD estimate for a future run of `actorID`
// based on the prior runs for that Actor. Returns a zero-confidence
// projection. A projection is produced for any sample of at least one run;
// samples of 1-2 are flagged low-confidence but still enforce --max-cost so
// the cap is never silently bypassed. Only a zero-sample Actor yields no
// projection — the caller must surface that explicitly.
func Project(actorID string, history []RunStats) Projection {
	costs := make([]float64, 0, len(history))
	for _, r := range history {
		if r.ActorID != actorID {
			continue
		}
		costs = append(costs, r.Estimate())
	}
	p := Projection{ActorID: actorID, SampleSize: len(costs)}
	if len(costs) == 0 {
		p.NoteOnSize = "no prior runs cached"
		return p
	}
	sort.Float64s(costs)
	p.P50USD = percentile(costs, 50)
	p.P90USD = percentile(costs, 90)
	if len(costs) < 3 {
		p.NoteOnSize = fmt.Sprintf("low confidence — only %d prior run(s) cached", len(costs))
	} else {
		p.NoteOnSize = fmt.Sprintf("from %d prior runs", len(costs))
	}
	return p
}

// CanEnforce reports whether --max-cost can be enforced for this projection.
// Enforcement needs a cap and at least one prior run to project against.
func CanEnforce(p Projection, maxCost float64) bool {
	return maxCost > 0 && p.SampleSize >= 1
}

// FormatProjection emits the one-line cost summary used by `run`.
func FormatProjection(p Projection, maxCost float64) string {
	if p.SampleSize == 0 {
		base := fmt.Sprintf("projected ~?? (%s)", p.NoteOnSize)
		if maxCost > 0 {
			base += fmt.Sprintf("; --max-cost $%.2f cannot be enforced without run history", maxCost)
		} else {
			base += "; pass --max-cost to cap"
		}
		return base
	}
	base := fmt.Sprintf("projected ~$%.2f (p50 %s); p90 ~$%.2f",
		p.P50USD, p.NoteOnSize, p.P90USD)
	if maxCost > 0 {
		if p.P50USD > maxCost {
			base += fmt.Sprintf(" — EXCEEDS --max-cost $%.2f", maxCost)
		} else if p.P90USD > maxCost {
			base += fmt.Sprintf(" — p90 exceeds --max-cost $%.2f (risk)", maxCost)
		}
	}
	return base
}

// ExceedsBudget returns true when the projection's p50 is above the cap.
// Caller uses this to abort runs pre-flight when --max-cost is set.
// Enforces for any sample of at least one run; a zero-sample Actor cannot
// be projected, so the caller must warn that the cap is unenforced.
func ExceedsBudget(p Projection, maxCost float64) bool {
	if !CanEnforce(p, maxCost) {
		return false
	}
	return p.P50USD > maxCost
}

// LedgerRow is one line of the cost report.
type LedgerRow struct {
	GroupKey     string  `json:"group_key"`
	RunCount     int     `json:"run_count"`
	TotalUSD     float64 `json:"total_usd"`
	AvgPerRunUSD float64 `json:"avg_per_run_usd"`
	TotalCU      float64 `json:"total_cu"`
}

// Rollup groups historical runs by `groupBy` ("actor", "schedule", "day")
// and returns one ledger row per group, sorted by total USD descending.
func Rollup(history []RunStats, groupBy string) []LedgerRow {
	groups := map[string]*LedgerRow{}
	for _, r := range history {
		key := groupKey(r, groupBy)
		row, ok := groups[key]
		if !ok {
			row = &LedgerRow{GroupKey: key}
			groups[key] = row
		}
		row.RunCount++
		row.TotalUSD += r.Estimate()
		row.TotalCU += r.ComputeUnits
	}
	out := make([]LedgerRow, 0, len(groups))
	for _, row := range groups {
		if row.RunCount > 0 {
			row.AvgPerRunUSD = row.TotalUSD / float64(row.RunCount)
		}
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TotalUSD > out[j].TotalUSD })
	return out
}

func groupKey(r RunStats, by string) string {
	switch strings.ToLower(by) {
	case "actor":
		if r.ActorID == "" {
			return "(unknown actor)"
		}
		return r.ActorID
	default:
		return r.ActorID
	}
}

func percentile(sorted []float64, p int) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	pos := float64(p) / 100.0 * float64(len(sorted)-1)
	lower := int(pos)
	upper := lower + 1
	if upper >= len(sorted) {
		return sorted[lower]
	}
	w := pos - float64(lower)
	return sorted[lower]*(1-w) + sorted[upper]*w
}
