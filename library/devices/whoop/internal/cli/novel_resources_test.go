package cli

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/whoop/internal/store"
)

// Regression test for the analytics readers pointing at store resource
// names that sync never writes. Sync persists records under the resource
// names returned by defaultSyncResources() ("cycle", "recovery",
// "activity" for sleeps, "activity-workout"), but the local-analytics
// readers (trend, digest, correlate, sleep-debt, strain-budget, classify)
// shipped reading "cycle_recovery" and "sleep" — names with zero rows on
// every synced mirror, which silently produced recovery 0 / observations 0.
//
// This test seeds a store under the EXACT names sync writes and asserts
// loadMetricSeries finds data for every metric it supports.
func TestLoadMetricSeriesUsesSyncResourceNames(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	seed := map[string]json.RawMessage{
		"cycle":    json.RawMessage(`{"id":1,"start":"2026-06-04T08:00:00Z","score":{"strain":12.3}}`),
		"recovery": json.RawMessage(`{"created_at":"2026-06-04T08:00:00Z","score":{"recovery_score":54}}`), // no "start": real recovery records only carry created_at
		"activity": json.RawMessage(`{"id":"s1","start":"2026-06-04T08:00:00Z","score":{"sleep_performance_percentage":66}}`),
	}
	for res, data := range seed {
		// resources.id is the table's sole PRIMARY KEY (not composite with
		// resource_type), so seed IDs must be unique across resources.
		if err := db.Upsert(res, "seed-"+res, data); err != nil {
			t.Fatalf("seed %s: %v", res, err)
		}
	}

	for _, metric := range []string{"strain", "recovery", "sleep"} {
		pts, err := loadMetricSeries(db, metric, time.Time{})
		if err != nil {
			t.Fatalf("loadMetricSeries(%q): %v", metric, err)
		}
		if len(pts) == 0 {
			t.Errorf("metric %q: no data found — reader resource name diverges from the names sync writes", metric)
		}
	}
}

// The direct db.List readers (classify → workouts, sleep-debt → sleeps,
// strain-budget → recoveries) must point at resources sync actually
// populates. syncResourcePath is the authoritative resource-name registry;
// an unknown name there means the reader can never see data.
func TestDirectReaderResourceNamesAreSyncResources(t *testing.T) {
	for _, res := range []string{"activity-workout", "activity", "recovery", "cycle"} {
		if _, err := syncResourcePath(res); err != nil {
			t.Errorf("analytics reader resource %q is not a sync resource: %v", res, err)
		}
	}
}
