// Copyright 2026 Giuliano Giacaglia and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
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
								"id":       "1:2",
								"name":     "Signup",
								"type":     "FRAME",
								"parentId": "1:1",
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

func TestSanitizeFilename(t *testing.T) {
	cases := []struct{ in, want string }{
		{"🕹️ Prototype / Prototype / Signup", "prototype-prototype-signup"},
		{"  ...///  ", "node"},
		{"A B__C.png", "a-b__c.png"},
	}
	for _, tc := range cases {
		if got := sanitizeFilename(tc.in); got != tc.want {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	long := strings.Repeat("a", 80)
	if got := sanitizeFilename(long); len(got) != 60 {
		t.Errorf("long sanitized len = %d, want 60", len(got))
	}
}

func TestIsLikelyScreenNode(t *testing.T) {
	accept := agentNodeSummary{Name: "Cash transfer Intro", Type: "FRAME"}
	if !isLikelyScreenNode(accept) {
		t.Fatalf("expected %q to be screen-like", accept.Name)
	}
	cases := []agentNodeSummary{
		{Name: "1", Type: "FRAME"},
		{Name: "Group 13", Type: "FRAME"},
		{Name: "Status Bar", Type: "FRAME"},
		{Name: "Home Indicator", Type: "FRAME"},
		{Name: "Screen", Type: "GROUP"},
		{Name: "Screen", Type: "VECTOR"},
	}
	for _, tc := range cases {
		if isLikelyScreenNode(tc) {
			t.Errorf("isLikelyScreenNode(%+v) = true, want false", tc)
		}
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
          },
          {
            "id": "1:10", "name": "Onboarding", "type": "SECTION", "parentId": "0:1",
            "children": [
              {"id": "1:11", "name": "Welcome", "type": "FRAME", "parentId": "1:10"},
              {"id": "1:12", "name": "Permissions", "type": "FRAME", "parentId": "1:10"},
              {"id": "1:13", "name": "Complete", "type": "FRAME", "parentId": "1:10"}
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

// newFakeFigmaMissingNodeServer returns a fake Figma server whose
// /v1/files/<key>/nodes endpoint returns the given raw JSON body. This lets
// tests simulate both Figma invalid-id shapes: {} (id not in the map) and
// {"nodes":{"<id>":null}} (id present but node deleted/hidden).
func newFakeFigmaMissingNodeServer(t *testing.T, nodesBody string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/files/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/nodes") {
			_, _ = w.Write([]byte(nodesBody))
			return
		}
		_, _ = w.Write([]byte(figmaFileFixture()))
	})
	return httptest.NewServer(mux)
}

func newFakeFigmaShotServer(t *testing.T) *httptest.Server {
	return newFakeFigmaShotServerWithTimeout(t, false)
}

func newFakeFigmaShotTimeoutServer(t *testing.T) *httptest.Server {
	return newFakeFigmaShotServerWithTimeout(t, true)
}

func newFakeFigmaShotServerWithTimeout(t *testing.T, timeoutMultiID bool) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var base string
	mux.HandleFunc("/v1/files/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/nodes") {
			ids := r.URL.Query().Get("ids")
			fmt.Fprintf(w, `{"nodes":{%q:{"document":{"id":%q,"name":"Signup","type":"FRAME"}}}}`, ids, ids)
			return
		}
		_, _ = w.Write([]byte(figmaFileFixture()))
	})
	mux.HandleFunc("/v1/images/", func(w http.ResponseWriter, r *http.Request) {
		ids := strings.Split(r.URL.Query().Get("ids"), ",")
		w.Header().Set("Content-Type", "application/json")
		if timeoutMultiID && len(ids) > 1 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"status":400,"err":"Render timeout, try requesting fewer or smaller images"}`))
			return
		}
		_, _ = w.Write([]byte(`{"err":null,"images":{`))
		for i, id := range ids {
			if i > 0 {
				_, _ = w.Write([]byte(","))
			}
			fmt.Fprintf(w, `%q:%q`, id, base+"/render/"+id+".png")
		}
		_, _ = w.Write([]byte(`}}`))
	})
	mux.HandleFunc("/render/", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "2:4") {
			http.Error(w, "blocked", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\nFAKEPNGBYTES"))
	})
	srv := httptest.NewServer(mux)
	base = srv.URL
	return srv
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

func newFakeFigmaProjectsServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/teams/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"Design (READY)","projects":[{"id":"P1","name":"App"},{"id":"P2","name":"Marketing"}]}`))
	})
	mux.HandleFunc("/v1/projects/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "P1") {
			_, _ = w.Write([]byte(`{"name":"App","files":[{"key":"AAA","name":"PaiN (1.7) — V2","last_modified":"2026-06-16T10:00:00Z"},{"key":"BBB","name":"PaiN (1.7) — V2","last_modified":"2026-06-15T10:00:00Z"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"name":"Marketing","files":[{"key":"CCC","name":"Brand Site","last_modified":"2026-06-10T10:00:00Z"}]}`))
	})
	return httptest.NewServer(mux)
}

func TestParseProjectTeamRef(t *testing.T) {
	cases := []struct{ in, kind, id string }{
		{"https://www.figma.com/files/project/P123/My-Project", "project", "P123"},
		{"https://www.figma.com/files/team/T123/Team", "team", "T123"},
		{"https://www.figma.com/files/org/team/T999/Team", "team", "T999"},
	}
	for _, tc := range cases {
		kind, id, err := parseProjectTeamRef(tc.in)
		if err != nil {
			t.Fatalf("parseProjectTeamRef(%q): %v", tc.in, err)
		}
		if kind != tc.kind || id != tc.id {
			t.Errorf("parseProjectTeamRef(%q) = (%q,%q), want (%q,%q)", tc.in, kind, id, tc.kind, tc.id)
		}
	}
	if _, _, err := parseProjectTeamRef("https://www.figma.com/design/abc/File"); err == nil {
		t.Fatalf("expected junk URL error")
	}
}

func TestSlugifyAliasCollisions(t *testing.T) {
	entries := buildKnownFilesEntries([]figmaFileMeta{{Key: "AAA", Name: "PaiN (1.7) — V2"}, {Key: "BBB", Name: "PaiN (1.7) — V2"}}, "App")
	if entries["pain-1-7-v2"].FileKey != "AAA" {
		t.Fatalf("first collision key = %+v, want AAA", entries["pain-1-7-v2"])
	}
	if entries["pain-1-7-v2-2"].FileKey != "BBB" {
		t.Fatalf("second collision key = %+v, want BBB", entries["pain-1-7-v2-2"])
	}
}

func TestIndexFilesProject(t *testing.T) {
	srv := newFakeFigmaProjectsServer(t)
	defer srv.Close()
	setFigmaTestEnv(t, srv.URL)
	out, err := runAgentRoot(t, []string{"agent", "index-files", "--project", "P1", "--agent", "--no-cache"})
	if err != nil {
		t.Fatalf("agent index-files project failed: %v\n%s", err, out)
	}
	var res struct {
		Files map[string]knownFile `json:"files"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("decoding output: %v\n%s", err, out)
	}
	if len(res.Files) != 2 {
		t.Fatalf("files len = %d, want 2: %+v", len(res.Files), res.Files)
	}
	if got := res.Files["pain-1-7-v2"]; got.FileKey != "AAA" || !strings.Contains(got.URL, "AAA") {
		t.Fatalf("first file = %+v, want AAA URL", got)
	}
	if got := res.Files["pain-1-7-v2"].Aliases; len(got) < 2 || got[1] != strings.ToLower("PaiN (1.7) — V2") {
		t.Fatalf("aliases = %#v, want lowercased display alias", got)
	}
}

func TestIndexFilesTeamWalksProjects(t *testing.T) {
	srv := newFakeFigmaProjectsServer(t)
	defer srv.Close()
	setFigmaTestEnv(t, srv.URL)
	out, err := runAgentRoot(t, []string{"agent", "index-files", "--team", "T1", "--agent", "--no-cache"})
	if err != nil {
		t.Fatalf("agent index-files team failed: %v\n%s", err, out)
	}
	var res struct {
		Files map[string]knownFile `json:"files"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("decoding output: %v\n%s", err, out)
	}
	if len(res.Files) != 3 {
		t.Fatalf("files len = %d, want 3: %+v", len(res.Files), res.Files)
	}
	if res.Files["brand-site"].Project != "Marketing" || res.Files["pain-1-7-v2"].Project != "App" {
		t.Fatalf("project names missing: %+v", res.Files)
	}
}

func TestIndexFilesMergeIntoAdditive(t *testing.T) {
	srv := newFakeFigmaProjectsServer(t)
	defer srv.Close()
	setFigmaTestEnv(t, srv.URL)
	path := filepath.Join(t.TempDir(), "known-files.json")
	initial := `{"_comment":"keep","files":{"pain-1-7-v2":{"file_key":"OLD","name":"Old Pain","url":"https://example.com","aliases":["pain"],"notes":"hand note"}},"owner":"agent"}`
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runAgentRoot(t, []string{"agent", "index-files", "--project", "P1", "--merge-into", path, "--agent", "--no-cache"})
	if err != nil {
		t.Fatalf("merge failed: %v\n%s", err, out)
	}
	var summary map[string]any
	if err := json.Unmarshal([]byte(out), &summary); err != nil {
		t.Fatalf("decoding summary: %v", err)
	}
	if summary["added"].(float64) != 1 || summary["skipped"].(float64) != 1 || summary["updated"].(float64) != 0 {
		t.Fatalf("summary = %+v, want added=1 skipped=1 updated=0", summary)
	}
	var doc struct {
		Files map[string]map[string]any `json:"files"`
	}
	b, _ := os.ReadFile(path)
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("decoding merged file: %v\n%s", err, b)
	}
	if doc.Files["pain-1-7-v2"]["file_key"] != "OLD" || doc.Files["pain-1-7-v2"]["notes"] != "hand note" {
		t.Fatalf("existing entry not preserved: %+v", doc.Files["pain-1-7-v2"])
	}
	out, err = runAgentRoot(t, []string{"agent", "index-files", "--project", "P1", "--merge-into", path, "--force", "--agent", "--no-cache"})
	if err != nil {
		t.Fatalf("force merge failed: %v\n%s", err, out)
	}
	b, _ = os.ReadFile(path)
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("decoding force merged file: %v", err)
	}
	if doc.Files["pain-1-7-v2"]["file_key"] != "AAA" || doc.Files["pain-1-7-v2"]["notes"] != "hand note" {
		t.Fatalf("force entry did not update while preserving notes: %+v", doc.Files["pain-1-7-v2"])
	}
}

func TestAgentOutlineCommand(t *testing.T) {
	srv := newFakeFigmaServer(t)
	defer srv.Close()
	setFigmaTestEnv(t, srv.URL)

	out, err := runAgentRoot(t, []string{"agent", "outline", "testKey", "--depth", "5", "--agent", "--no-cache"})
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

	out, err := runAgentRoot(t, []string{"agent", "find-node", "testKey", "Prototype", "--depth", "5", "--agent", "--no-cache"})
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

	out, err := runAgentRoot(t, []string{"agent", "find-node", "testKey", "zzznope", "--agent", "--no-cache"})
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
		"--agent", "--no-cache",
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

func TestAgentFindNodeCommandDirectHitMissing(t *testing.T) {
	cases := []struct {
		name      string
		nodesBody string // raw /v1/files/<key>/nodes response
	}{
		{
			// Figma shape when the node-id is not in the file at all.
			name:      "empty nodes map",
			nodesBody: `{"nodes":{}}`,
		},
		{
			// Figma shape when the id is present but the node was
			// deleted/hidden — value is a JSON null.
			name:      "null node entry",
			nodesBody: `{"nodes":{"99:99":null}}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newFakeFigmaMissingNodeServer(t, tc.nodesBody)
			defer srv.Close()
			setFigmaTestEnv(t, srv.URL)

			_, err := runAgentRoot(t, []string{
				"agent", "find-node",
				"https://www.figma.com/design/testKey/X?node-id=99-99",
				"--agent",
			})
			if err == nil {
				t.Fatalf("expected an error for missing node, got nil (fake successful match)")
			}
			if !strings.Contains(err.Error(), "not found") {
				t.Errorf("error = %q, want it to mention 'not found'", err.Error())
			}
		})
	}
}

func TestAgentOutlineAcceptsURL(t *testing.T) {
	srv := newFakeFigmaServer(t)
	defer srv.Close()
	setFigmaTestEnv(t, srv.URL)

	out, err := runAgentRoot(t, []string{
		"agent", "outline",
		"https://www.figma.com/design/testKey/Whatever?node-id=1-2",
		"--agent", "--no-cache",
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

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

func TestDownloadToFileRejectsOversize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.CopyN(w, zeroReader{}, (40<<20)+1)
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "oversize.png")
	if _, err := downloadToFile(srv.Client(), srv.URL, dest); err == nil || !strings.Contains(err.Error(), "exceeded 40 MiB cap") {
		t.Fatalf("downloadToFile oversize error = %v, want cap error", err)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("oversize destination should not exist, stat err = %v", err)
	}
}

func TestAgentShotDownloadsFile(t *testing.T) {
	srv := newFakeFigmaShotServer(t)
	defer srv.Close()
	setFigmaTestEnv(t, srv.URL)
	outDir := t.TempDir()

	out, err := runAgentRoot(t, []string{"agent", "shot", "testKey", "Signup", "--max", "1", "--out-dir", outDir, "--agent", "--no-cache"})
	if err != nil {
		t.Fatalf("agent shot failed: %v\n%s", err, out)
	}
	var res map[string]any
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("decoding shot output: %v\n%s", err, out)
	}
	if res["count"].(float64) != 1 {
		t.Fatalf("count = %v, want 1", res["count"])
	}
	images := res["images"].([]any)
	img := images[0].(map[string]any)
	if img["url"] == "" {
		t.Errorf("url missing: %+v", img)
	}
	path, _ := img["path"].(string)
	if path == "" {
		t.Fatalf("path missing: %+v", img)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat downloaded file: %v", err)
	}
	if st.Size() == 0 {
		t.Fatalf("downloaded file is empty")
	}
}

func TestAgentShotNoDownloadReturnsURLOnly(t *testing.T) {
	srv := newFakeFigmaShotServer(t)
	defer srv.Close()
	setFigmaTestEnv(t, srv.URL)
	outDir := t.TempDir()

	out, err := runAgentRoot(t, []string{"agent", "shot", "testKey", "Signup", "--max", "1", "--out-dir", outDir, "--no-download", "--agent", "--no-cache"})
	if err != nil {
		t.Fatalf("agent shot --no-download failed: %v\n%s", err, out)
	}
	var res map[string]any
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("decoding shot output: %v\n%s", err, out)
	}
	img := res["images"].([]any)[0].(map[string]any)
	if img["url"] == "" {
		t.Errorf("url missing: %+v", img)
	}
	if _, ok := img["path"]; ok {
		t.Errorf("path present with --no-download: %+v", img)
	}
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no files, got %d", len(entries))
	}
}

func TestAgentShotAmbiguousMaxOneDoesNotRender(t *testing.T) {
	srv := newFakeFigmaShotServer(t)
	defer srv.Close()
	setFigmaTestEnv(t, srv.URL)

	out, err := runAgentRoot(t, []string{"agent", "shot", "testKey", "Prototype", "--max", "1", "--agent", "--no-cache"})
	if err != nil {
		t.Fatalf("agent shot ambiguous failed: %v\n%s", err, out)
	}
	var res map[string]any
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("decoding shot output: %v\n%s", err, out)
	}
	if res["ambiguous"] != true {
		t.Fatalf("ambiguous = %v, want true", res["ambiguous"])
	}
	if got := len(res["images"].([]any)); got != 0 {
		t.Fatalf("images len = %d, want 0", got)
	}
	if got := len(res["matches"].([]any)); got < 2 {
		t.Fatalf("matches len = %d, want >=2", got)
	}
}

func TestAgentShotDownloadFailureKeepsURL(t *testing.T) {
	srv := newFakeFigmaShotServer(t)
	defer srv.Close()
	setFigmaTestEnv(t, srv.URL)

	out, err := runAgentRoot(t, []string{"agent", "shot", "testKey", "Login", "--max", "1", "--out-dir", t.TempDir(), "--agent", "--no-cache"})
	if err != nil {
		t.Fatalf("agent shot download failure should still exit 0: %v\n%s", err, out)
	}
	var res map[string]any
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("decoding shot output: %v\n%s", err, out)
	}
	img := res["images"].([]any)[0].(map[string]any)
	if img["url"] == "" {
		t.Fatalf("url missing after download failure: %+v", img)
	}
	if _, ok := img["path"]; ok {
		t.Fatalf("path present after download failure: %+v", img)
	}
	if errText, _ := img["download_error"].(string); !strings.Contains(errText, "503") {
		t.Fatalf("download_error = %q, want HTTP 503", errText)
	}
}

func TestRenderImagesSplitsOnTimeout(t *testing.T) {
	srv := newFakeFigmaShotTimeoutServer(t)
	defer srv.Close()
	setFigmaTestEnv(t, srv.URL)
	flags := rootFlags{}
	c, err := flags.newClient()
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	got, err := renderImages(c, "testKey", "png", 2, []string{"1:11", "1:12", "1:13"}, 3)
	if err != nil {
		t.Fatalf("renderImages returned error: %v", err)
	}
	for _, id := range []string{"1:11", "1:12", "1:13"} {
		if got[id] == nil || *got[id] == "" {
			t.Fatalf("image %s = %v, want non-empty URL; all=%+v", id, got[id], got)
		}
	}
}

func TestAgentShotSucceedsDespiteRenderTimeout(t *testing.T) {
	srv := newFakeFigmaShotTimeoutServer(t)
	defer srv.Close()
	setFigmaTestEnv(t, srv.URL)

	out, err := runAgentRoot(t, []string{"agent", "shot", "testKey", "Onboarding", "--children", "--max", "3", "--batch", "3", "--out-dir", t.TempDir(), "--agent", "--no-cache"})
	if err != nil {
		t.Fatalf("agent shot should split render timeout and exit 0: %v\n%s", err, out)
	}
	var res map[string]any
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("decoding shot output: %v\n%s", err, out)
	}
	if res["count"].(float64) != 3 {
		t.Fatalf("count = %v, want 3", res["count"])
	}
	for _, raw := range res["images"].([]any) {
		img := raw.(map[string]any)
		if img["path"] == "" && img["url"] == "" {
			t.Fatalf("image missing path/url: %+v", img)
		}
	}
}

func TestAgentShotChildrenRendersSectionScreens(t *testing.T) {
	srv := newFakeFigmaShotServer(t)
	defer srv.Close()
	setFigmaTestEnv(t, srv.URL)

	out, err := runAgentRoot(t, []string{"agent", "shot", "testKey", "Onboarding", "--children", "--max", "5", "--no-download", "--agent", "--no-cache"})
	if err != nil {
		t.Fatalf("agent shot --children failed: %v\n%s", err, out)
	}
	var res map[string]any
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("decoding shot output: %v\n%s", err, out)
	}
	images := res["images"].([]any)
	if len(images) != 3 {
		t.Fatalf("images len = %d, want 3", len(images))
	}
	for _, raw := range images {
		img := raw.(map[string]any)
		label, _ := img["label"].(string)
		if !strings.HasPrefix(label, "🕹️ Prototype / Onboarding / ") {
			t.Fatalf("label = %q, want Onboarding child", label)
		}
		if img["id"] == "1:10" || img["type"] == "SECTION" {
			t.Fatalf("rendered section instead of child: %+v", img)
		}
	}
}

func TestAgentShotNoMatch(t *testing.T) {
	srv := newFakeFigmaShotServer(t)
	defer srv.Close()
	setFigmaTestEnv(t, srv.URL)

	out, err := runAgentRoot(t, []string{"agent", "shot", "testKey", "zzznope", "--agent", "--no-cache"})
	if err != nil {
		t.Fatalf("agent shot no-match failed: %v\n%s", err, out)
	}
	var res map[string]any
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("decoding shot output: %v\n%s", err, out)
	}
	if res["count"].(float64) != 0 {
		t.Fatalf("count = %v, want 0", res["count"])
	}
	if got := len(res["images"].([]any)); got != 0 {
		t.Fatalf("images len = %d, want 0", got)
	}
	steps := res["next_steps"].([]any)
	if len(steps) == 0 || !strings.Contains(steps[0].(string), "agent outline") {
		t.Fatalf("next_steps = %+v, want agent outline hint", steps)
	}
}
