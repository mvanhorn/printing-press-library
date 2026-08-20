// Hand-authored tests for the citation-graph builder. No network: a fake
// apiGetter serves canned OpenAlex responses. Not generated.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// fakeGraphClient implements apiGetter with canned responses keyed by the
// OpenAlex filter ("cites:W1", "cited_by:W1", ...). Any /works/<id> path returns
// the seed object.
type fakeGraphClient struct {
	seed     json.RawMessage
	byFilter map[string]json.RawMessage
	calls    int
}

func (f *fakeGraphClient) Get(_ context.Context, path string, params map[string]string) (json.RawMessage, error) {
	f.calls++
	if strings.HasPrefix(path, "/works/") {
		return f.seed, nil
	}
	if r, ok := f.byFilter[params["filter"]]; ok {
		return r, nil
	}
	return json.RawMessage(`{"results":[]}`), nil
}

func seedJSON(id, title string, year, citedBy int) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(
		`{"id":"https://openalex.org/%s","title":%q,"publication_year":%d,"cited_by_count":%d}`,
		id, title, year, citedBy))
}

// worksJSON builds a /works results envelope from short ids (W2, W3, ...).
func worksJSON(ids ...string) json.RawMessage {
	var sb strings.Builder
	sb.WriteString(`{"results":[`)
	for i, id := range ids {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, `{"id":"https://openalex.org/%s","display_name":%q,"publication_year":2021,"cited_by_count":%d}`,
			id, "work "+id, 10+i)
	}
	sb.WriteString(`]}`)
	return json.RawMessage(sb.String())
}

func nodeIDSet(g citationGraph) map[string]bool {
	set := map[string]bool{}
	for _, n := range g.Nodes {
		set[n.ID] = true
	}
	return set
}

func edgeSetOf(g citationGraph) map[string]bool {
	set := map[string]bool{}
	for _, e := range g.Edges {
		set[e.From+"->"+e.To] = true
	}
	return set
}

// assertNoDanglingEdges verifies every edge endpoint is a real node.
func assertNoDanglingEdges(t *testing.T, g citationGraph) {
	t.Helper()
	nodes := nodeIDSet(g)
	for _, e := range g.Edges {
		if !nodes[e.From] || !nodes[e.To] {
			t.Errorf("dangling edge %s->%s (from-node=%v, to-node=%v)", e.From, e.To, nodes[e.From], nodes[e.To])
		}
	}
}

func TestShortID(t *testing.T) {
	tests := []struct{ in, want string }{
		{"https://openalex.org/W123", "W123"},
		{"W123", "W123"},
		{"  https://openalex.org/W9  ", "W9"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := shortID(tt.in); got != tt.want {
			t.Errorf("shortID(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestBuildCitationGraph(t *testing.T) {
	flags := &rootFlags{} // progress disabled (non-TTY in tests)

	t.Run("both directions, depth 1", func(t *testing.T) {
		fc := &fakeGraphClient{
			seed: seedJSON("W1", "Seed", 2020, 100),
			byFilter: map[string]json.RawMessage{
				"cites:W1":    worksJSON("W2", "W3"), // works citing the seed
				"cited_by:W1": worksJSON("W4"),       // works the seed references
			},
		}
		g, err := buildCitationGraph(context.Background(), fc, flags, "W1", 1, 50, "both")
		if err != nil {
			t.Fatalf("buildCitationGraph: %v", err)
		}
		if g.NodeCount != 4 {
			t.Errorf("NodeCount = %d, want 4", g.NodeCount)
		}
		gotEdges := edgeSetOf(g)
		// Citers point TO the seed; the seed points TO its references.
		for _, want := range []string{"W2->W1", "W3->W1", "W1->W4"} {
			if !gotEdges[want] {
				t.Errorf("missing edge %s; got %v", want, gotEdges)
			}
		}
		if g.EdgeCount != 3 {
			t.Errorf("EdgeCount = %d, want 3", g.EdgeCount)
		}
		assertNoDanglingEdges(t, g)
	})

	t.Run("cited-by only -> edges point to seed", func(t *testing.T) {
		fc := &fakeGraphClient{
			seed:     seedJSON("W1", "Seed", 2020, 100),
			byFilter: map[string]json.RawMessage{"cites:W1": worksJSON("W2", "W3")},
		}
		g, err := buildCitationGraph(context.Background(), fc, flags, "W1", 1, 50, "cited-by")
		if err != nil {
			t.Fatalf("buildCitationGraph: %v", err)
		}
		want := map[string]bool{"W2->W1": true, "W3->W1": true}
		if got := edgeSetOf(g); len(got) != 2 || !got["W2->W1"] || !got["W3->W1"] {
			t.Errorf("edges = %v, want %v", got, want)
		}
		assertNoDanglingEdges(t, g)
	})

	t.Run("references only -> edges point from seed", func(t *testing.T) {
		fc := &fakeGraphClient{
			seed:     seedJSON("W1", "Seed", 2020, 100),
			byFilter: map[string]json.RawMessage{"cited_by:W1": worksJSON("W4", "W5")},
		}
		g, err := buildCitationGraph(context.Background(), fc, flags, "W1", 1, 50, "references")
		if err != nil {
			t.Fatalf("buildCitationGraph: %v", err)
		}
		if got := edgeSetOf(g); len(got) != 2 || !got["W1->W4"] || !got["W1->W5"] {
			t.Errorf("edges = %v, want seed->W4, seed->W5", got)
		}
		assertNoDanglingEdges(t, g)
	})

	t.Run("max-nodes cap stops cleanly with no dangling edges", func(t *testing.T) {
		fc := &fakeGraphClient{
			seed:     seedJSON("W1", "Seed", 2020, 100),
			byFilter: map[string]json.RawMessage{"cites:W1": worksJSON("W2", "W3", "W4", "W5")},
		}
		g, err := buildCitationGraph(context.Background(), fc, flags, "W1", 1, 2, "cited-by")
		if err != nil {
			t.Fatalf("buildCitationGraph: %v", err)
		}
		if g.NodeCount != 2 {
			t.Errorf("NodeCount = %d, want 2 (capped)", g.NodeCount)
		}
		// Only one citer fits under the cap; its edge must be present and valid,
		// and no edge may reference a dropped node.
		assertNoDanglingEdges(t, g)
		if g.EdgeCount != 1 {
			t.Errorf("EdgeCount = %d, want 1", g.EdgeCount)
		}
	})

	t.Run("max-nodes 1 makes no neighbor calls", func(t *testing.T) {
		fc := &fakeGraphClient{
			seed:     seedJSON("W1", "Seed", 2020, 100),
			byFilter: map[string]json.RawMessage{"cites:W1": worksJSON("W2")},
		}
		g, err := buildCitationGraph(context.Background(), fc, flags, "W1", 2, 1, "both")
		if err != nil {
			t.Fatalf("buildCitationGraph: %v", err)
		}
		if g.NodeCount != 1 {
			t.Errorf("NodeCount = %d, want 1 (seed only)", g.NodeCount)
		}
		// Exactly one API call: the seed lookup. No neighbor expansion at cap=1.
		if fc.calls != 1 {
			t.Errorf("api calls = %d, want 1 (seed only)", fc.calls)
		}
		if g.Note == "" {
			t.Error("expected a Note when the graph has no neighbors")
		}
	})

	t.Run("depth 2 expands a second hop", func(t *testing.T) {
		fc := &fakeGraphClient{
			seed: seedJSON("W1", "Seed", 2020, 100),
			byFilter: map[string]json.RawMessage{
				"cites:W1": worksJSON("W2"),
				"cites:W2": worksJSON("W5"),
			},
		}
		g, err := buildCitationGraph(context.Background(), fc, flags, "W1", 2, 50, "cited-by")
		if err != nil {
			t.Fatalf("buildCitationGraph: %v", err)
		}
		nodes := nodeIDSet(g)
		for _, id := range []string{"W1", "W2", "W5"} {
			if !nodes[id] {
				t.Errorf("missing node %s; got %v", id, nodes)
			}
		}
		got := edgeSetOf(g)
		if !got["W2->W1"] || !got["W5->W2"] {
			t.Errorf("edges = %v, want W2->W1 and W5->W2", got)
		}
		assertNoDanglingEdges(t, g)
	})
}
