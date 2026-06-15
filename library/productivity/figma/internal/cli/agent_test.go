// Copyright 2026 Giuliano Giacaglia and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseFigmaRefRawKey(t *testing.T) {
	ref, err := parseFigmaRef("JZyB6K6Z22YyObBdj1r4v1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.FileKey != "JZyB6K6Z22YyObBdj1r4v1" {
		t.Errorf("FileKey = %q, want raw key", ref.FileKey)
	}
	if ref.NodeID != "" {
		t.Errorf("NodeID = %q, want empty", ref.NodeID)
	}
}

func TestParseFigmaRefDesignURL(t *testing.T) {
	ref, err := parseFigmaRef("https://www.figma.com/design/abc123XyZ/My-File?node-id=123-456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.FileKey != "abc123XyZ" {
		t.Errorf("FileKey = %q, want abc123XyZ", ref.FileKey)
	}
	if ref.NodeID != "123:456" {
		t.Errorf("NodeID = %q, want 123:456", ref.NodeID)
	}
}

func TestParseFigmaRefFileURL(t *testing.T) {
	ref, err := parseFigmaRef("https://figma.com/file/abc123XyZ/My-File?node_id=123-456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.FileKey != "abc123XyZ" {
		t.Errorf("FileKey = %q, want abc123XyZ", ref.FileKey)
	}
	if ref.NodeID != "123:456" {
		t.Errorf("NodeID = %q, want 123:456", ref.NodeID)
	}
}

func TestParseFigmaRefProtoURL(t *testing.T) {
	ref, err := parseFigmaRef("https://www.figma.com/proto/abc123XyZ/My-Proto?node-id=9-8")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.FileKey != "abc123XyZ" {
		t.Errorf("FileKey = %q, want abc123XyZ", ref.FileKey)
	}
	if ref.NodeID != "9:8" {
		t.Errorf("NodeID = %q, want 9:8", ref.NodeID)
	}
}

func TestParseFigmaRefRejectsUnknownURL(t *testing.T) {
	cases := []string{
		"https://example.com/foo",
		"https://www.figma.com/",
		"https://www.figma.com/design/",
		"not a real key/with slashes",
	}
	for _, in := range cases {
		if _, err := parseFigmaRef(in); err == nil {
			t.Errorf("parseFigmaRef(%q) = nil error, want error", in)
		}
	}
}

func TestBuildAgentNodeIndexIncludesPathLabels(t *testing.T) {
	doc := map[string]any{
		"id":   "0:0",
		"name": "Document",
		"type": "DOCUMENT",
		"children": []any{
			map[string]any{
				"id":       "0:1",
				"name":     "🕹️ Prototype",
				"type":     "CANVAS",
				"parentId": "0:0",
				"children": []any{
					map[string]any{
						"id":       "1:1",
						"name":     "Prototype",
						"type":     "SECTION",
						"parentId": "0:1",
						"children": []any{
							map[string]any{
								"id":            "1:2",
								"name":          "Signup",
								"type":          "FRAME",
								"parentId":      "1:1",
								"children": []any{
									map[string]any{"id": "1:3", "name": "Email", "type": "TEXT"},
									map[string]any{"id": "1:4", "name": "Submit", "type": "RECTANGLE"},
								},
							},
						},
					},
				},
			},
		},
	}

	all := buildAgentNodeIndex(doc, map[string]bool{
		"CANVAS": true, "SECTION": true, "FRAME": true,
	}, 0)

	var signup *agentNodeSummary
	for i := range all {
		if all[i].Name == "Signup" {
			signup = &all[i]
		}
	}
	if signup == nil {
		t.Fatalf("Signup frame not indexed; got %d nodes: %+v", len(all), all)
	}

	wantLabel := "🕹️ Prototype / Prototype / Signup"
	if signup.Label != wantLabel {
		t.Errorf("Signup label = %q, want %q", signup.Label, wantLabel)
	}
	wantPath := []string{"🕹️ Prototype", "Prototype", "Signup"}
	if len(signup.Path) != len(wantPath) {
		t.Errorf("Signup path = %v, want %v", signup.Path, wantPath)
	} else {
		for i := range wantPath {
			if signup.Path[i] != wantPath[i] {
				t.Errorf("Signup path[%d] = %q, want %q", i, signup.Path[i], wantPath[i])
			}
		}
	}
	if signup.ChildCount != 2 {
		t.Errorf("Signup child_count = %d, want 2", signup.ChildCount)
	}
	if signup.ParentID != "1:1" {
		t.Errorf("Signup parent_id = %q, want 1:1", signup.ParentID)
	}
}

func TestBuildAgentNodeIndexFiltersTypes(t *testing.T) {
	doc := map[string]any{
		"type": "DOCUMENT",
		"children": []any{
			map[string]any{
				"id": "0:1", "name": "Page", "type": "CANVAS",
				"children": []any{
					map[string]any{"id": "1:1", "name": "A", "type": "FRAME"},
					map[string]any{"id": "1:2", "name": "B", "type": "TEXT"},
				},
			},
		},
	}
	got := buildAgentNodeIndex(doc, map[string]bool{"CANVAS": true, "FRAME": true}, 0)
	for _, n := range got {
		if n.Type == "TEXT" {
			t.Errorf("disallowed type TEXT should be filtered out: %+v", n)
		}
	}
	if len(got) != 2 {
		t.Errorf("indexed %d nodes, want 2 (Page + A)", len(got))
	}
}

func TestScoreAgentNodePrefersExactNameOverSubstring(t *testing.T) {
	exact := agentNodeSummary{Name: "Prototype", Label: "🕹️ Prototype / Prototype"}
	substr := agentNodeSummary{Name: "PrototypeFlow", Label: "🕹️ Prototype / PrototypeFlow"}
	if scoreAgentNode("Prototype", exact) <= scoreAgentNode("Prototype", substr) {
		t.Errorf("exact name must outrank substring: exact=%d substr=%d",
			scoreAgentNode("Prototype", exact), scoreAgentNode("Prototype", substr))
	}
	if scoreAgentNode("Prototype", exact) != 100 {
		t.Errorf("exact name score = %d, want 100", scoreAgentNode("Prototype", exact))
	}
	if scoreAgentNode("Prototype", substr) <= 0 {
		t.Errorf("substring match should still score > 0, got %d", scoreAgentNode("Prototype", substr))
	}
	if got := scoreAgentNode("nomatch", exact); got != 0 {
		t.Errorf("no-match score = %d, want 0", got)
	}
}

func TestScoreAgentNodeFindsTokenMatchesInLabel(t *testing.T) {
	node := agentNodeSummary{Name: "Row", Label: "Onboarding / Hero Primary Card"}
	// "hero card" is not a contiguous substring of the label, but both words
	// are label tokens → token tier.
	if got := scoreAgentNode("hero card", node); got <= 0 {
		t.Errorf("token match should score > 0, got %d", got)
	}
	// Token tier must be below substring tier.
	tier := scoreAgentNode("hero card", node)
	sub := scoreAgentNode("hero", node) // "hero" is a contiguous substring of the label
	if tier >= sub {
		t.Errorf("token tier (%d) must be below substring tier (%d)", tier, sub)
	}
}

// ---- Command-level tests with a fake Figma server ----

// figmaFileFixture returns a Figma /v1/files/<key> response with two pages,
// each holding a SECTION named "Prototype" so find-node sees an ambiguous tie.
func figmaFileFixture() string {
	return `{
  "name": "Example Design",
  "document": {
    "id": "0:0", "name": "Document", "type": "DOCUMENT",
    "children": [
      {
        "id": "0:1", "name": "🕹️ Prototype", "type": "CANVAS", "parentId": "0:0",
        "children": [
          {
            "id": "1:1", "name": "Prototype", "type": "SECTION", "parentId": "0:1",
            "children": [
              {"id": "1:2", "name": "Signup", "type": "FRAME", "parentId": "1:1"}
            ]
          }
        ]
      },
      {
        "id": "0:2", "name": "💎 Templates", "type": "CANVAS", "parentId": "0:0",
        "children": [
          {
            "id": "2:3", "name": "Prototype", "type": "SECTION", "parentId": "0:2",
            "children": [
              {"id": "2:4", "name": "Login", "type": "FRAME", "parentId": "2:3"}
            ]
          }
        ]
      }
    ]
  }
}`
}

func newFakeFigmaServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/files/", func(w http.ResponseWriter, r *http.Request) {
		// Distinguish /v1/files/<key> from /v1/files/<key>/nodes
		if strings.HasSuffix(r.URL.Path, "/nodes") {
			// Resolve a single node by id query for the direct-hit path.
			ids := r.URL.Query().Get("ids")
			type nodeEntry struct {
				Document map[string]any `json:"document"`
			}
			out := struct {
				Nodes map[string]nodeEntry `json:"nodes"`
			}{
				Nodes: map[string]nodeEntry{
					ids: {Document: map[string]any{"id": ids, "name": "Prototype", "type": "SECTION"}},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			b, _ := json.Marshal(out)
			_, _ = w.Write(b)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(figmaFileFixture()))
	})
	return httptest.NewServer(mux)
}

// setFigmaTestEnv points the client at a fake server with no real credentials.
func setFigmaTestEnv(t *testing.T, baseURL string) {
	t.Helper()
	t.Setenv("FIGMA_BASE_URL", baseURL)
	// Avoid reading a real config file on the developer's machine.
	t.Setenv("FIGMA_CONFIG", filepath.Join(t.TempDir(), "no-config.toml"))
}

func runAgentRoot(t *testing.T, args []string) (string, error) {
	t.Helper()
	flags := rootFlags{}
	root := newRootCmd(&flags)
	root.SetArgs(args)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	err := root.Execute()
	return out.String(), err
}

func TestAgentOutlineCommand(t *testing.T) {
	srv := newFakeFigmaServer(t)
	defer srv.Close()
	setFigmaTestEnv(t, srv.URL)

	out, err := runAgentRoot(t, []string{"agent", "outline", "testKey", "--depth", "5", "--agent"})
	if err != nil {
		t.Fatalf("agent outline failed: %v\n%s", err, out)
	}

	var res map[string]any
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("decoding outline output: %v\n%s", err, out)
	}
	if res["file_key"] != "testKey" {
		t.Errorf("file_key = %v, want testKey", res["file_key"])
	}
	if res["file_name"] != "Example Design" {
		t.Errorf("file_name = %v, want Example Design", res["file_name"])
	}
	nodes, ok := res["nodes"].([]any)
	if !ok {
		t.Fatalf("nodes is not an array: %T", res["nodes"])
	}
	// Two pages + two sections + two frames = 6 indexed nodes (default types
	// include CANVAS,SECTION,FRAME,...).
	if len(nodes) == 0 {
		t.Fatalf("expected indexed nodes, got 0")
	}
	// Find the Signup frame and assert its label carries full path context.
	var foundSignup bool
	for _, n := range nodes {
		m, _ := n.(map[string]any)
		if m["id"] == "1:2" {
			foundSignup = true
			if m["label"] != "🕹️ Prototype / Prototype / Signup" {
				t.Errorf("Signup label = %v, want full path", m["label"])
			}
		}
	}
	if !foundSignup {
		t.Errorf("expected to find Signup frame (1:2) in outline; nodes=%+v", nodes)
	}
}

func TestAgentFindNodeCommandAmbiguous(t *testing.T) {
	srv := newFakeFigmaServer(t)
	defer srv.Close()
	setFigmaTestEnv(t, srv.URL)

	out, err := runAgentRoot(t, []string{"agent", "find-node", "testKey", "Prototype", "--depth", "5", "--agent"})
	if err != nil {
		t.Fatalf("agent find-node failed: %v\n%s", err, out)
	}

	var res map[string]any
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("decoding find-node output: %v\n%s", err, out)
	}
	if res["query"] != "Prototype" {
		t.Errorf("query = %v, want Prototype", res["query"])
	}
	if res["ambiguous"] != true {
		t.Errorf("ambiguous = %v, want true (two Prototype sections tie at top score)", res["ambiguous"])
	}
	matches, ok := res["matches"].([]any)
	if !ok {
		t.Fatalf("matches is not an array: %T", res["matches"])
	}
	// Both Prototype sections match at score 100.
	if len(matches) < 2 {
		t.Fatalf("expected >=2 matches, got %d", len(matches))
	}
	first, _ := matches[0].(map[string]any)
	second, _ := matches[1].(map[string]any)
	if first["score"].(float64) != 100 || second["score"].(float64) != 100 {
		t.Errorf("top two match scores = %v, %v; want 100, 100", first["score"], second["score"])
	}
	if res["best"] == nil {
		t.Errorf("best must be populated when matches exist")
	}
}

func TestAgentFindNodeCommandNoMatches(t *testing.T) {
	srv := newFakeFigmaServer(t)
	defer srv.Close()
	setFigmaTestEnv(t, srv.URL)

	out, err := runAgentRoot(t, []string{"agent", "find-node", "testKey", "zzznope", "--agent"})
	if err != nil {
		t.Fatalf("agent find-node (no match) failed: %v\n%s", err, out)
	}
	var res map[string]any
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("decoding find-node output: %v\n%s", err, out)
	}
	if res["match_count"].(float64) != 0 {
		t.Errorf("match_count = %v, want 0", res["match_count"])
	}
	if res["ambiguous"] != false {
		t.Errorf("ambiguous = %v, want false on no matches", res["ambiguous"])
	}
	steps, _ := res["next_steps"].([]any)
	if len(steps) == 0 {
		t.Errorf("expected helpful next_steps on no matches")
	}
}

func TestAgentFindNodeCommandDirectHit(t *testing.T) {
	srv := newFakeFigmaServer(t)
	defer srv.Close()
	setFigmaTestEnv(t, srv.URL)

	// URL carries node-id=1-2 (normalized to 1:2) and no query → the
	// direct-hit branch must resolve via /v1/files/<key>/nodes instead of
	// the label-scoring path.
	out, err := runAgentRoot(t, []string{
		"agent", "find-node",
		"https://www.figma.com/design/testKey/X?node-id=1-2",
		"--agent",
	})
	if err != nil {
		t.Fatalf("agent find-node (direct-hit) failed: %v\n%s", err, out)
	}
	var res map[string]any
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("decoding find-node direct-hit output: %v\n%s", err, out)
	}
	if res["node_id"] != "1:2" {
		t.Errorf("node_id = %v, want 1:2 (normalized from URL 1-2)", res["node_id"])
	}
	if res["match_count"].(float64) != 1 {
		t.Errorf("match_count = %v, want 1 for direct hit", res["match_count"])
	}
	if res["ambiguous"] != false {
		t.Errorf("ambiguous = %v, want false for a single direct hit", res["ambiguous"])
	}
	best, ok := res["best"].(map[string]any)
	if !ok {
		t.Fatalf("best is not an object: %T", res["best"])
	}
	if best["id"] != "1:2" {
		t.Errorf("best.id = %v, want 1:2", best["id"])
	}
	if best["score"].(float64) != 100 {
		t.Errorf("best.score = %v, want 100 (direct hit)", best["score"])
	}
	matches, _ := res["matches"].([]any)
	if len(matches) != 1 {
		t.Errorf("matches len = %d, want 1", len(matches))
	}
}

func TestAgentOutlineAcceptsURL(t *testing.T) {
	srv := newFakeFigmaServer(t)
	defer srv.Close()
	setFigmaTestEnv(t, srv.URL)

	out, err := runAgentRoot(t, []string{
		"agent", "outline",
		"https://www.figma.com/design/testKey/Whatever?node-id=1-2",
		"--agent",
	})
	if err != nil {
		t.Fatalf("agent outline (URL) failed: %v\n%s", err, out)
	}
	var res map[string]any
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("decoding outline output: %v\n%s", err, out)
	}
	if res["file_key"] != "testKey" {
		t.Errorf("file_key = %v, want testKey (parsed from URL)", res["file_key"])
	}
}
