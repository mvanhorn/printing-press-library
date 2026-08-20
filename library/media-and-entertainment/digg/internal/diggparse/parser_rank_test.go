package diggparse

import (
	"encoding/json"
	"strings"
	"testing"
)

// crossSectionConflictFixture mirrors the shape that triggered the reported
// bug: the same clusterId appears in the RSC stream twice with different
// rank-bearing field values. In production this happens when a cluster lives
// in the main /ai feed AND a featured / weekly / pinned / sevenDaysStories
// section. With last-wins merge precedence (the pre-fix behavior), the
// secondary section's rank=1 stamps the cluster's currentRank, displacing
// the live leaderboard rank. After the first-non-zero-wins fix, the earlier
// stream occurrence's rank wins.
//
// Captured from a real digg.com/ai shape on 2026-05-24. Cluster IDs and
// titles are realistic; structure has been trimmed to the fields that
// exercise the merge path.
const crossSectionConflictFixture = `prefix ` +
	`{"clusterId":"74d77886-d16f-4acf-a886-5503426b105a","clusterUrlId":"0zzxfag9",` +
	`"title":"Trump administration reinstates O-1 visa policy","tldr":"Up to half of OpenAI researchers on O-1 visas could be impacted.",` +
	`"currentRank":5,"peakRank":3,"previousRank":7,"delta":2}` +
	` middle content ` +
	`{"clusterId":"74d77886-d16f-4acf-a886-5503426b105a","clusterUrlId":"0zzxfag9",` +
	`"title":"Trump administration reinstates O-1 visa policy",` +
	`"currentRank":1,"peakRank":1,"previousRank":1,"delta":99}` +
	` suffix`

func TestMergeClustersFirstWinsForRankFields(t *testing.T) {
	clusters, err := ExtractClusters(crossSectionConflictFixture)
	if err != nil {
		t.Fatal(err)
	}
	if len(clusters) != 1 {
		t.Fatalf("got %d clusters, want 1 (deduped by clusterId)", len(clusters))
	}
	c := clusters[0]

	// First occurrence's non-zero rank values must win over the later
	// occurrence's secondary-section ranks. This is the bug fix: without
	// it, the later rank=1 (from a featured/pinned/historical block)
	// would clobber the live leaderboard rank=5.
	if c.CurrentRank != 5 {
		t.Errorf("CurrentRank = %d, want 5 (first non-zero must win)", c.CurrentRank)
	}
	if c.PeakRank != 3 {
		t.Errorf("PeakRank = %d, want 3 (first non-zero must win)", c.PeakRank)
	}
	if c.PreviousRank != 7 {
		// First occurrence has previousRank=7, second has previousRank=1.
		// First-non-zero-wins → 7.
		t.Errorf("PreviousRank = %d, want 7 (first non-zero must win)", c.PreviousRank)
	}
	if c.Delta != 2 {
		t.Errorf("Delta = %d, want 2 (first non-zero must win)", c.Delta)
	}

	// Non-rank fields keep their existing later-wins-when-nonzero semantics.
	// Title is identical across occurrences here; just sanity-check that
	// extraction populated it.
	if c.Title == "" {
		t.Errorf("Title was dropped: %+v", c)
	}
}

func TestMergeClustersStillGapFillsZeroAccumulator(t *testing.T) {
	// First occurrence has no rank-bearing fields; later occurrence has
	// rank=3. First-wins on rank gates on accumulator==0, so the later
	// non-zero rank legitimately fills the gap. Regression guard against
	// over-tightening the precedence into strict-first-wins.
	decoded := `{"clusterId":"abc","clusterUrlId":"x","title":"Story"}` +
		` other ` +
		`{"clusterId":"abc","currentRank":3,"peakRank":2}`
	clusters, err := ExtractClusters(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(clusters) != 1 {
		t.Fatalf("got %d clusters, want 1", len(clusters))
	}
	c := clusters[0]
	if c.CurrentRank != 3 {
		t.Errorf("CurrentRank = %d, want 3 (zero accumulator must accept later non-zero)", c.CurrentRank)
	}
	if c.PeakRank != 2 {
		t.Errorf("PeakRank = %d, want 2 (zero accumulator must accept later non-zero)", c.PeakRank)
	}
}

func TestMergeClustersDistinctClustersPreserveOwnRanks(t *testing.T) {
	// Two different clusters, each appearing once with rank=1 and rank=2
	// respectively. First-wins gate must not collapse distinct clusters
	// into a single rank value.
	decoded := `{"clusterId":"alpha","currentRank":1,"title":"A"}` +
		` and ` +
		`{"clusterId":"beta","currentRank":2,"title":"B"}`
	clusters, err := ExtractClusters(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(clusters) != 2 {
		t.Fatalf("got %d clusters, want 2", len(clusters))
	}
	byID := map[string]Cluster{}
	for _, c := range clusters {
		byID[c.ClusterID] = c
	}
	if byID["alpha"].CurrentRank != 1 {
		t.Errorf("alpha rank = %d, want 1", byID["alpha"].CurrentRank)
	}
	if byID["beta"].CurrentRank != 2 {
		t.Errorf("beta rank = %d, want 2", byID["beta"].CurrentRank)
	}
}

func TestExtractClustersNormalizesPeakRankSentinel(t *testing.T) {
	// peakRank=9999 is Digg's internal sentinel for newly-detected clusters
	// with no tracked all-time peak. The extraction layer must normalize
	// to 0 so the json:"peakRank,omitempty" tag drops the field from output.
	decoded := `{"clusterId":"sentinel-case","clusterUrlId":"x","title":"Newly detected",` +
		`"currentRank":4,"peakRank":9999}`
	clusters, err := ExtractClusters(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(clusters) != 1 {
		t.Fatalf("got %d clusters, want 1", len(clusters))
	}
	c := clusters[0]
	if c.PeakRank != 0 {
		t.Errorf("PeakRank = %d, want 0 (9999 sentinel must normalize)", c.PeakRank)
	}

	// JSON marshalling must omit peakRank entirely because of omitempty.
	out, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "peakRank") {
		t.Errorf("JSON output still contains peakRank field: %s", out)
	}
}

func TestExtractClustersSentinelDoesNotBlockLaterPeakRank(t *testing.T) {
	// First occurrence carries the 9999 sentinel (newly-detected, no
	// tracked peak). Later occurrence has a legitimate peakRank=42 from
	// another section. The pre-merge sentinel normalization must zero out
	// the sentinel so the first-non-zero-wins merge guard accepts the
	// later real value. Without parse-time normalization, the sentinel
	// would block the real peak and the output would silently drop to 0.
	decoded := `{"clusterId":"sentinel-then-real","clusterUrlId":"x","title":"Story",` +
		`"currentRank":5,"peakRank":9999}` +
		` middle ` +
		`{"clusterId":"sentinel-then-real","peakRank":42}`
	clusters, err := ExtractClusters(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(clusters) != 1 {
		t.Fatalf("got %d clusters, want 1", len(clusters))
	}
	if clusters[0].PeakRank != 42 {
		t.Errorf("PeakRank = %d, want 42 (later legitimate value must not be blocked by 9999 sentinel)", clusters[0].PeakRank)
	}
}

func TestExtractClustersPreservesLegitimatePeakRank(t *testing.T) {
	// Any peakRank other than 9999 must pass through unchanged.
	decoded := `{"clusterId":"legit-peak","clusterUrlId":"x","title":"Tracked",` +
		`"currentRank":7,"peakRank":42}`
	clusters, err := ExtractClusters(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(clusters) != 1 {
		t.Fatalf("got %d clusters, want 1", len(clusters))
	}
	if clusters[0].PeakRank != 42 {
		t.Errorf("PeakRank = %d, want 42 (legitimate value must be preserved)", clusters[0].PeakRank)
	}
}

func TestExtractClustersOmitsAbsentPeakRank(t *testing.T) {
	// A cluster with no peakRank field at all must remain zero and serialize-absent.
	decoded := `{"clusterId":"no-peak","clusterUrlId":"x","title":"No peak data","currentRank":2}`
	clusters, err := ExtractClusters(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(clusters) != 1 {
		t.Fatalf("got %d clusters, want 1", len(clusters))
	}
	c := clusters[0]
	if c.PeakRank != 0 {
		t.Errorf("PeakRank = %d, want 0", c.PeakRank)
	}
	out, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "peakRank") {
		t.Errorf("JSON output should omit peakRank for absent field: %s", out)
	}
}
