package mcp

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// Read tools must advertise readOnlyHint; sync mutates local state (writes the
// snapshot DB + FTS index) and must not. A false readOnlyHint on a mutating
// tool is a real bug, so guard the count and the per-tool annotation.
func TestToolReadOnlyHints(t *testing.T) {
	ts := tools()
	if len(ts) != 23 {
		t.Fatalf("expected 23 tools, got %d", len(ts))
	}
	writeTools := map[string]bool{"sync": true, "archive_enable": true, "archive_disable": true}
	for _, spec := range ts {
		hint := spec.tool.Annotations.ReadOnlyHint
		if hint == nil {
			t.Fatalf("tool %q has no read-only annotation", spec.tool.Name)
		}
		wantReadOnly := !writeTools[spec.tool.Name]
		if *hint != wantReadOnly {
			t.Fatalf("tool %q readOnlyHint = %v, want %v", spec.tool.Name, *hint, wantReadOnly)
		}
	}
}

func TestToolListIncludesArchiveAndSyncAccumulate(t *testing.T) {
	wantNames := []string{
		"search",
		"list",
		"domains",
		"searches",
		"downloads",
		"visited",
		"report",
		"heatmap",
		"journeys",
		"timeline",
		"rabbitholes",
		"dwell",
		"graph",
		"profile",
		"devices",
		"icloud-tabs",
		"topic",
		"sql",
		"sync",
		"archive_status",
		"archive_enable",
		"archive_disable",
		"doctor",
	}

	ts := tools()
	gotNames := make([]string, 0, len(ts))
	for _, spec := range ts {
		gotNames = append(gotNames, spec.tool.Name)
	}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("tool names = %#v, want %#v", gotNames, wantNames)
	}

	byName := toolSpecsByName(ts)
	for _, forbidden := range []string{"archive_clobber", "archive_reset", "archive_vacuum"} {
		if _, ok := byName[forbidden]; ok {
			t.Fatalf("tool %q must not be exposed over MCP", forbidden)
		}
	}

	syncTool := byName["sync"].tool
	accumulate, ok := syncTool.InputSchema.Properties["accumulate"].(map[string]any)
	if !ok {
		t.Fatalf("sync accumulate schema missing or wrong shape: %#v", syncTool.InputSchema.Properties["accumulate"])
	}
	if got := accumulate["type"]; got != "string" {
		t.Fatalf("sync accumulate type = %#v, want string", got)
	}
}

func TestToolDescriptionsSteerCachedReadsBeforeSync(t *testing.T) {
	byName := toolSpecsByName(tools())
	for _, name := range []string{"search", "list", "domains", "visited", "report", "sql"} {
		desc := byName[name].tool.Description
		if !strings.Contains(desc, "cached store") || !strings.Contains(desc, "no sync required") {
			t.Fatalf("%s description = %q, want cached-store/no-sync steering", name, desc)
		}
	}

	syncDesc := byName["sync"].tool.Description
	if strings.Contains(strings.ToLower(syncDesc), "run this first") {
		t.Fatalf("sync description still encourages reflexive sync: %q", syncDesc)
	}
	for _, want := range []string{"Refresh", "Only needed", "read tools query the cached store without sync"} {
		if !strings.Contains(syncDesc, want) {
			t.Fatalf("sync description missing %q: %q", want, syncDesc)
		}
	}

	doctorDesc := byName["doctor"].tool.Description
	if !strings.Contains(doctorDesc, "cached_store is queryable") || !strings.Contains(doctorDesc, "without sync") {
		t.Fatalf("doctor description = %q, want cached_store offline-read guidance", doctorDesc)
	}
}

// TestWriteToolsDoNotCarryCachedReadSteering asserts that the write/mutating
// tools (sync, archive_enable, archive_disable) do NOT carry the cached-read
// steering phrases that belong only on read tools.  A false-positive match
// would mislead agents into thinking these mutation tools are cache-servable.
func TestWriteToolsDoNotCarryCachedReadSteering(t *testing.T) {
	byName := toolSpecsByName(tools())

	// None of the write tools must advertise "no sync required" — that phrase
	// is the agent-steering signal that a tool reads from the cached store.
	for _, name := range []string{"sync", "archive_enable", "archive_disable"} {
		desc := byName[name].tool.Description
		if strings.Contains(desc, "no sync required") {
			t.Fatalf("%s description must NOT contain \"no sync required\" (write tool should not carry cached-read steering): %q", name, desc)
		}
	}

	// archive_enable and archive_disable are about seeding/stopping the archive,
	// not reading from it.  Their descriptions must not reference "cached store"
	// as a queryable source.
	for _, name := range []string{"archive_enable", "archive_disable"} {
		desc := byName[name].tool.Description
		if strings.Contains(desc, "cached store") {
			t.Fatalf("%s description must NOT contain \"cached store\" (not a cached-read tool): %q", name, desc)
		}
	}
}

func TestArchiveAndSyncCommandArgs(t *testing.T) {
	ts := toolSpecsByName(tools())
	tests := []struct {
		name string
		req  mcp.CallToolRequest
		want []string
	}{
		{name: "archive_status", want: []string{"archive", "status"}},
		{name: "archive_enable", want: []string{"archive", "enable"}},
		{name: "archive_disable", want: []string{"archive", "disable"}},
		{name: "sync", req: toolRequest(map[string]any{"profile": "Default"}), want: []string{"sync", "--profile", "Default"}},
		{name: "sync", req: toolRequest(map[string]any{"profile": "Default", "accumulate": "false"}), want: []string{"sync", "--profile", "Default"}},
		{name: "sync", req: toolRequest(map[string]any{"profile": "Default", "accumulate": "true"}), want: []string{"sync", "--profile", "Default", "--accumulate"}},
	}

	for _, tt := range tests {
		spec, ok := ts[tt.name]
		if !ok {
			t.Fatalf("missing tool %q", tt.name)
		}
		got := spec.cmdArgs(tt.req)
		if !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("%s args = %#v, want %#v", tt.name, got, tt.want)
		}
	}
}

func toolSpecsByName(ts []toolSpec) map[string]toolSpec {
	byName := make(map[string]toolSpec, len(ts))
	for _, spec := range ts {
		byName[spec.tool.Name] = spec
	}
	return byName
}

func toolRequest(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: args},
	}
}

func TestRunSelfReturnsStdoutOnlyWhenStderrHasHint(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "fake-cli.sh")
	content := "#!/usr/bin/env bash\nprintf '[]\\n'\nprintf 'no activity hint\\n' >&2\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}

	prev := osExecutable
	osExecutable = func() (string, error) { return script, nil }
	t.Cleanup(func() { osExecutable = prev })

	out, err := runSelf("list", "--json", "--since", "2099-01-01")
	if err != nil {
		t.Fatalf("runSelf err: %v", err)
	}
	if strings.TrimSpace(out) != "[]" {
		t.Fatalf("stdout polluted: %q", out)
	}
}
