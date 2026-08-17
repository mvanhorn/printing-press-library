// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// TestCommandSelectPaths_PinnedCommandsPresent asserts the four
// commands U7 wires through to agent-context have a populated cheat
// sheet. New roots added to tools/select-paths-gen/main.go should pick
// up an assertion here so a future generator regression on those
// commands surfaces in the test suite, not at agent-runtime.
func TestCommandSelectPaths_PinnedCommandsPresent(t *testing.T) {
	t.Parallel()
	for _, cmd := range []string{"topic", "compare", "recall", "markets get-by-slug"} {
		paths, ok := commandSelectPaths[cmd]
		if !ok {
			t.Errorf("commandSelectPaths missing entry for %q", cmd)
			continue
		}
		if len(paths) == 0 {
			t.Errorf("commandSelectPaths[%q] is empty; the generator should have populated it", cmd)
		}
	}
}

// TestCommandSelectPaths_MarketsGetBySlugFields covers the canonical
// Polymarket fields the U7 plan calls out explicitly: an agent that
// just learned the slug should be able to --select these without
// guessing. If the upstream Market struct drops one of these (or
// renames a JSON tag), this test fails and the next regeneration must
// resolve the drift.
func TestCommandSelectPaths_MarketsGetBySlugFields(t *testing.T) {
	t.Parallel()
	paths := commandSelectPaths["markets get-by-slug"]
	required := []string{
		"question", "slug", "outcomes", "outcomePrices",
		"lastTradePrice", "bestBid", "bestAsk",
		"volume", "liquidity", "endDate",
	}
	set := stringSet(paths)
	for _, r := range required {
		if !set[r] {
			t.Errorf("markets get-by-slug select_paths missing %q (got: %s)", r, strings.Join(paths, ", "))
		}
	}
}

// TestCommandSelectPaths_TopicHasNestedHits asserts the slice-of-struct
// expansion: topicResult.Hits is []topicHit so paths must include
// hits.id, hits.title, etc. Without bare-name slice handling agents
// would have to guess hits[0].id and silently get nothing.
func TestCommandSelectPaths_TopicHasNestedHits(t *testing.T) {
	t.Parallel()
	paths := commandSelectPaths["topic"]
	set := stringSet(paths)
	for _, r := range []string{
		"hits", "hits.id", "hits.kind", "hits.source", "hits.title", "hits.yesProbability",
	} {
		if !set[r] {
			t.Errorf("topic select_paths missing %q (got: %s)", r, strings.Join(paths, ", "))
		}
	}
}

// TestCommandSelectPaths_CompareHasNestedVenue covers the
// pointer-to-struct expansion (comparePair.PM is *compareVenue).
// Without pointer following, an agent passing pairs.polymarket.id
// would silently get nothing.
func TestCommandSelectPaths_CompareHasNestedVenue(t *testing.T) {
	t.Parallel()
	paths := commandSelectPaths["compare"]
	set := stringSet(paths)
	for _, r := range []string{
		"pairs", "pairs.polymarket", "pairs.polymarket.id", "pairs.match",
		"pairs.kalshi.yesProbability",
	} {
		if !set[r] {
			t.Errorf("compare select_paths missing %q (got: %s)", r, strings.Join(paths, ", "))
		}
	}
}

// TestCommandSelectPaths_NestedMetaEnvelope covers the
// {meta, results} polymorphic envelope. topic/compare/recall all
// expose meta.* fields and the cheatsheet should list them so an
// agent can --select meta.price_source without guessing.
func TestCommandSelectPaths_NestedMetaEnvelope(t *testing.T) {
	t.Parallel()
	cases := map[string][]string{
		"topic":   {"meta.price_source", "meta.index_synced_at", "meta.learnings_applied", "meta.teach_hint"},
		"compare": {"meta.price_source", "meta.index_synced_at"},
	}
	for cmd, want := range cases {
		set := stringSet(commandSelectPaths[cmd])
		for _, k := range want {
			if !set[k] {
				t.Errorf("%s select_paths missing %q", cmd, k)
			}
		}
	}
}

// TestCommandSelectPaths_SlicePathsUseBareNames is the structural
// guard for the rule documented in tools/select-paths-gen/main.go:
// slice fields use the element-type field name, not [0] / [*]
// notation. Catches a future "let's just dump array indices" mistake.
func TestCommandSelectPaths_SlicePathsUseBareNames(t *testing.T) {
	t.Parallel()
	for cmd, paths := range commandSelectPaths {
		for _, p := range paths {
			if strings.ContainsAny(p, "[]") {
				t.Errorf("%s select_paths %q must not contain array indices; use bare names", cmd, p)
			}
		}
	}
}

// TestCommandSelectPaths_SortedAndUnique guards the generator output
// shape. Disorder would surface as gnarly diffs every run; duplicates
// would inflate the cheatsheet without helping agents.
func TestCommandSelectPaths_SortedAndUnique(t *testing.T) {
	t.Parallel()
	for cmd, paths := range commandSelectPaths {
		if !sort.StringsAreSorted(paths) {
			t.Errorf("%s select_paths is not sorted (got: %s)", cmd, strings.Join(paths, ", "))
		}
		seen := map[string]bool{}
		for _, p := range paths {
			if seen[p] {
				t.Errorf("%s select_paths has duplicate %q", cmd, p)
			}
			seen[p] = true
		}
	}
}

// TestCommandSelectPaths_NoUnexportedJSONDash guards the
// readJSONTag / unexported-skip rules in the generator. `rankScore`
// on topicHit is unexported lowercase but the camel-case lookup
// shouldn't match the lowercase ident either; verify the surface only
// holds tagged or exported fields by checking we don't see any
// well-known "should-be-skipped" leaks.
func TestCommandSelectPaths_NoUnexportedJSONDash(t *testing.T) {
	t.Parallel()
	// rankScore is an unexported topicHit field used as a Go-side
	// ranking scratch space; it must not surface as a select path.
	for _, p := range commandSelectPaths["topic"] {
		if strings.HasSuffix(p, ".rankScore") || p == "rankScore" {
			t.Errorf("topic select_paths leaked unexported field %q", p)
		}
	}
}

// TestCommandSelectPaths_AgentContextRoundTrip asserts the cheatsheet
// reaches the agent-context envelope under commands.<name>.select_paths
// using the full dotted command path for subcommands. This is the
// contract agents actually consume.
func TestCommandSelectPaths_AgentContextRoundTrip(t *testing.T) {
	t.Parallel()
	root := RootCmd()
	ctx := buildAgentContext(root)
	// Walk commands looking for the markets > get-by-slug subcommand
	// and assert its SelectPaths matches the cheatsheet entry.
	var found *agentContextCommand
	for i := range ctx.Commands {
		if ctx.Commands[i].Name != "markets" {
			continue
		}
		for j := range ctx.Commands[i].Subcommands {
			if ctx.Commands[i].Subcommands[j].Name == "get-by-slug" {
				found = &ctx.Commands[i].Subcommands[j]
				break
			}
		}
	}
	if found == nil {
		t.Fatal("agent-context did not surface markets > get-by-slug")
	}
	if len(found.SelectPaths) == 0 {
		t.Fatalf("markets get-by-slug select_paths empty in agent-context envelope")
	}
	// Spot-check: same set as the cheatsheet.
	want := commandSelectPaths["markets get-by-slug"]
	if len(found.SelectPaths) != len(want) {
		t.Errorf("agent-context select_paths len = %d, want %d", len(found.SelectPaths), len(want))
	}
}

// TestCommandSelectPaths_AgentContextTopLevel asserts the same
// round-trip for a top-level command (no parent prefix). topic is
// pinned in roots; its select_paths should make it into the
// commands[].SelectPaths field of the envelope.
func TestCommandSelectPaths_AgentContextTopLevel(t *testing.T) {
	t.Parallel()
	root := RootCmd()
	ctx := buildAgentContext(root)
	for i := range ctx.Commands {
		if ctx.Commands[i].Name == "topic" {
			if len(ctx.Commands[i].SelectPaths) == 0 {
				t.Fatal("topic.select_paths empty in agent-context envelope")
			}
			return
		}
	}
	t.Fatal("agent-context did not surface topic")
}

// TestCommandSelectPaths_AgentContextJSONMarshal asserts the field
// shape an agent actually parses. Anything that breaks the JSON
// envelope (a hyphen-keyed command, a non-string entry) shows up
// here. Uses real json encoding rather than struct introspection so
// the test fails the same way an agent's parser would.
func TestCommandSelectPaths_AgentContextJSONMarshal(t *testing.T) {
	t.Parallel()
	root := RootCmd()
	ctx := buildAgentContext(root)
	data, err := json.Marshal(ctx)
	if err != nil {
		t.Fatalf("marshal agent-context: %v", err)
	}
	var probe struct {
		SchemaVersion string `json:"schema_version"`
		Commands      []struct {
			Name        string   `json:"name"`
			SelectPaths []string `json:"select_paths,omitempty"`
			Subcommands []struct {
				Name        string   `json:"name"`
				SelectPaths []string `json:"select_paths,omitempty"`
			} `json:"subcommands,omitempty"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		t.Fatalf("unmarshal agent-context: %v", err)
	}
	if probe.SchemaVersion == "" {
		t.Error("schema_version missing from agent-context envelope")
	}
	var topicPaths int
	for _, c := range probe.Commands {
		if c.Name == "topic" {
			topicPaths = len(c.SelectPaths)
		}
	}
	if topicPaths == 0 {
		t.Error("topic.select_paths missing from JSON-encoded agent-context")
	}
}

// TestSelectPathsGenerator_NoDriftFromHEAD runs the generator and
// diffs its output against the committed select_paths.go. A non-empty
// diff means the committed file is stale; the local lint (and CI, if
// wired) fires from this assertion. This is the canonical "did you
// forget to re-run go generate" check.
//
// The test silently skips when:
//   - the go toolchain is unavailable in PATH (e.g. minimal CI runner)
//   - the tools/select-paths-gen directory has moved (refactor in
//     flight)
//
// Anything else (build failure, parse failure, diff) fails the test so
// the next agent debugging select-paths drift gets a clear signal.
//
// The test writes the regenerated file to a temp path via the
// SELECT_PATHS_GEN_OUT env-var override on the generator itself, so it
// never touches the worktree's committed select_paths.go.
func TestSelectPathsGenerator_NoDriftFromHEAD(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no `go` in PATH; skipping generator drift check")
	}
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Skip("cannot resolve test source location")
	}
	cliRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	toolDir := filepath.Join(cliRoot, "tools", "select-paths-gen")
	if _, err := os.Stat(toolDir); err != nil {
		t.Skipf("tools/select-paths-gen missing at %s; skipping drift check", toolDir)
	}
	committedPath := filepath.Join(cliRoot, "internal", "cli", "select_paths.go")
	committed, err := os.ReadFile(committedPath)
	if err != nil {
		t.Fatalf("read committed select_paths.go: %v", err)
	}

	// Generate to a scratch file in t.TempDir() so the assertion
	// never mutates the worktree's committed copy. The generator
	// reads source from the CLI root (cmd.Dir) and writes to the
	// env-overridden path.
	tmpOut := filepath.Join(t.TempDir(), "select_paths.go")
	cmd := exec.Command("go", "run", "./tools/select-paths-gen")
	cmd.Dir = cliRoot
	cmd.Env = append(os.Environ(), "SELECT_PATHS_GEN_OUT="+tmpOut)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("run generator: %v; stderr: %s", err, stderr.String())
	}
	regenerated, err := os.ReadFile(tmpOut)
	if err != nil {
		t.Fatalf("read regenerated select_paths.go: %v", err)
	}
	if !bytes.Equal(committed, regenerated) {
		t.Errorf("select_paths.go is stale; run `go generate ./internal/cli/...` from the CLI root to refresh.\n\n"+
			"Diff hint: %d bytes committed vs %d regenerated.\nRegenerated at: %s",
			len(committed), len(regenerated), tmpOut)
	}
}

func stringSet(in []string) map[string]bool {
	out := make(map[string]bool, len(in))
	for _, s := range in {
		out[s] = true
	}
	return out
}
