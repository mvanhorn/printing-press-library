package salestech

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-salestech/internal/store"
)

// HealthReport summarizes cross-source state — what the API thinks vs what
// the local store has vs how fresh the cursor is per resource. Pre-flight
// for any audit; agent runtime check before composing reports.
type HealthReport struct {
	GeneratedAt    string           `json:"generated_at"`
	StoreOK        bool             `json:"store_ok"`
	StorePath      string           `json:"store_path,omitempty"`
	Resources      []HealthResource `json:"resources"`
	Recommendation string           `json:"recommendation,omitempty"`
}

// HealthResource is one resource type's health snapshot.
type HealthResource struct {
	Resource       string `json:"resource"`
	LocalCount     int    `json:"local_count"`
	APICount       int    `json:"api_count,omitempty"` // 0 when probe skipped
	APICountSource string `json:"api_count_source,omitempty"`
	LastSyncedAt   string `json:"last_synced_at,omitempty"`
	CursorAge      string `json:"cursor_age,omitempty"`
	Drift          int    `json:"drift,omitempty"` // api_count - local_count when both known
	Status         string `json:"status"`          // ok | stale | drift | empty | error
	Detail         string `json:"detail,omitempty"`
}

// APICountFn looks up the API's totalCount for a resource — the CLI's
// command wires this to the generated client's GET /tenant/{tenant}/<r>
// with pageSize=1 + includeTotal=true. Returns (count, sourceLabel, error)
// where sourceLabel describes how the count was obtained ("api:totalCount"
// or "skipped: <why>"). Pass nil to skip API probing (local-only health).
type APICountFn func(ctx context.Context, resource string) (int, string, error)

// Health builds the cross-source health report. When apiCount is nil, the
// API columns are omitted and the report becomes a local-only snapshot.
// staleAfter governs the "stale" status threshold (default 24h).
func Health(ctx context.Context, db *store.Store, apiCount APICountFn, staleAfter time.Duration) (HealthReport, error) {
	if staleAfter <= 0 {
		staleAfter = 24 * time.Hour
	}
	report := HealthReport{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		StoreOK:     true,
		StorePath:   db.Path(),
	}
	resources := []string{ResEstimates, ResEstimateItems, ResStatusChanges, ResFollowUps}
	now := time.Now().UTC()
	for _, r := range resources {
		res := HealthResource{Resource: r}
		// Local count.
		var local int
		err := db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM resources WHERE resource_type = ?`, r).Scan(&local)
		if err != nil {
			res.Status = "error"
			res.Detail = fmt.Sprintf("local count: %s", err)
			report.Resources = append(report.Resources, res)
			continue
		}
		res.LocalCount = local
		// Sync cursor age.
		_, lastSynced, _, err := db.GetSyncState(r)
		if err == nil && !lastSynced.IsZero() {
			res.LastSyncedAt = lastSynced.UTC().Format(time.RFC3339)
			age := now.Sub(lastSynced.UTC())
			res.CursorAge = age.Round(time.Minute).String()
			if age > staleAfter && local > 0 {
				res.Status = "stale"
				res.Detail = fmt.Sprintf("last synced %s ago (> %s threshold)", res.CursorAge, staleAfter)
			}
		}

		// API probe (estimates only — items/status changes/follow-ups don't
		// expose a tenant-wide totalCount in a single call).
		if apiCount != nil && r == ResEstimates {
			cnt, src, err := apiCount(ctx, r)
			if err != nil {
				if res.Status == "" {
					res.Status = "error"
				}
				res.Detail = appendDetail(res.Detail, "api probe: "+err.Error())
			} else {
				res.APICount = cnt
				res.APICountSource = src
				if cnt > 0 && local > 0 {
					res.Drift = cnt - local
					if abs(res.Drift) > maxDrift(cnt, 5) {
						res.Status = "drift"
						res.Detail = appendDetail(res.Detail, fmt.Sprintf("api=%d local=%d drift=%d", cnt, local, res.Drift))
					}
				}
			}
		}
		if res.Status == "" {
			if local == 0 {
				res.Status = "empty"
				res.Detail = appendDetail(res.Detail, "run `sync` to populate")
			} else {
				res.Status = "ok"
			}
		}
		report.Resources = append(report.Resources, res)
	}
	sort.SliceStable(report.Resources, func(i, j int) bool {
		return report.Resources[i].Resource < report.Resources[j].Resource
	})
	// Recommendation summary.
	var issues []string
	for _, r := range report.Resources {
		if r.Status == "empty" || r.Status == "stale" || r.Status == "drift" || r.Status == "error" {
			issues = append(issues, fmt.Sprintf("%s: %s", r.Resource, r.Status))
		}
	}
	if len(issues) > 0 {
		report.Recommendation = "run `sync` to refresh; " + strings.Join(issues, ", ")
	} else {
		report.Recommendation = "local mirror is fresh; audits will reflect current ServiceTitan state"
	}
	return report, nil
}

func appendDetail(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + "; " + b
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// maxDrift returns the larger of (5% of total) and floor (default 5)
// so that small absolute drifts on tiny stores don't pollute the report.
func maxDrift(total, floor int) int {
	v := total / 20 // 5%
	if v < floor {
		return floor
	}
	return v
}

// Status returns store.Status() wrapped with a JSON-friendly shape for
// callers that want the raw counts without the API-side comparison.
func Status(db *store.Store) (map[string]int, error) {
	return db.Status()
}

// ensure json import is used (kept for future serialization callers)
var _ = json.Marshal
