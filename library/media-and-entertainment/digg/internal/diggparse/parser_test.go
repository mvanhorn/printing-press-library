package diggparse

import (
	"strings"
	"testing"
)

func TestJSUnescape(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{`hello`, "hello"},
		{`a\"b`, `a"b`},
		{`a\\b`, `a\b`},
		{`a\nb`, "a\nb"},
		{`aAb`, "aAb"},
		{`a\/b`, "a/b"},
	}
	for _, tc := range cases {
		if got := jsUnescape(tc.in); got != tc.want {
			t.Errorf("jsUnescape(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestDecodeRSCReturnsEmptyOnNoMatch(t *testing.T) {
	if DecodeRSC([]byte(`<html><body>hi</body></html>`)) != "" {
		t.Fatal("expected empty decoded for HTML with no RSC pushes")
	}
}

func TestDecodeRSCSingle(t *testing.T) {
	html := []byte(`<script>self.__next_f.push([1,"hello\nworld"])</script>`)
	got := DecodeRSC(html)
	want := "hello\nworld"
	if got != want {
		t.Errorf("DecodeRSC = %q, want %q", got, want)
	}
}

func TestDecodeRSCMultipleConcat(t *testing.T) {
	html := []byte(`<script>self.__next_f.push([1,"part-1 "])</script><script>self.__next_f.push([1,"part-2"])</script>`)
	got := DecodeRSC(html)
	if got != "part-1 part-2" {
		t.Errorf("DecodeRSC = %q, want %q", got, "part-1 part-2")
	}
}

func TestMatchBalancedObject(t *testing.T) {
	s := `{"a":1,"b":{"c":2}}`
	if got := matchBalancedObject(s, 0); got != len(s) {
		t.Errorf("balanced end = %d, want %d", got, len(s))
	}

	s2 := `{"a":"}{","b":2}` // string contains braces
	if got := matchBalancedObject(s2, 0); got != len(s2) {
		t.Errorf("balanced end = %d, want %d", got, len(s2))
	}
}

func TestExtractClustersSimple(t *testing.T) {
	decoded := `prefix...{"clusterId":"abc","clusterUrlId":"x123","label":"hello","currentRank":3}...suffix`
	clusters, err := ExtractClusters(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(clusters) != 1 {
		t.Fatalf("got %d clusters, want 1", len(clusters))
	}
	c := clusters[0]
	if c.ClusterID != "abc" || c.Label != "hello" || c.CurrentRank != 3 {
		t.Errorf("unexpected cluster: %+v", c)
	}
}

func TestExtractClustersDedup(t *testing.T) {
	// Same clusterId twice; second occurrence has more fields.
	decoded := `{"clusterId":"abc","label":"first"}` + ` other ` +
		`{"clusterId":"abc","label":"second","tldr":"a tldr","currentRank":1}`
	clusters, err := ExtractClusters(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(clusters) != 1 {
		t.Fatalf("got %d clusters, want 1", len(clusters))
	}
	c := clusters[0]
	// Merged: rank from second wins (it's non-zero in the second only).
	if c.CurrentRank != 1 {
		t.Errorf("merged rank = %d, want 1", c.CurrentRank)
	}
	if c.TLDR != "a tldr" {
		t.Errorf("merged tldr = %q, want %q", c.TLDR, "a tldr")
	}
	if !strings.Contains(c.Label, "first") && !strings.Contains(c.Label, "second") {
		t.Errorf("merged label dropped: %q", c.Label)
	}
}

func TestExtractEvents(t *testing.T) {
	decoded := `{"id":"e1","type":"cluster_detected","clusterId":"c1","label":"L","at":"2026-05-09T00:00:00Z","createdAt":"2026-05-09T00:00:01Z","dedupeKey":"x:1"}`
	events, err := ExtractEvents(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Type != "cluster_detected" || events[0].ClusterID != "c1" {
		t.Errorf("unexpected events: %+v", events)
	}
}

func TestParseTrendingStatus(t *testing.T) {
	body := []byte(`{"isFetching":false,"storiesToday":42,"clustersToday":3,"events":[{"id":"a","type":"batch_started","at":"2026-05-09T00:00:00Z","dedupeKey":"k"}]}`)
	ts, err := ParseTrendingStatus(body)
	if err != nil {
		t.Fatal(err)
	}
	if ts.StoriesToday != 42 || ts.ClustersToday != 3 || len(ts.Events) != 1 {
		t.Errorf("unexpected trending: %+v", ts)
	}
}
