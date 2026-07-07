package mcp

import (
	"reflect"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// Read tools must advertise readOnlyHint; sync mutates local state (writes the
// snapshot DB + FTS index) and must not. A false readOnlyHint on a mutating
// tool is a real bug, so guard the count and the per-tool annotation.
func TestToolReadOnlyHints(t *testing.T) {
	ts := tools()
	if len(ts) != 22 {
		t.Fatalf("expected 22 tools, got %d", len(ts))
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
	if got := accumulate["type"]; got != "boolean" {
		t.Fatalf("sync accumulate type = %#v, want boolean", got)
	}
	profile, ok := syncTool.InputSchema.Properties["profile"].(map[string]any)
	if !ok {
		t.Fatalf("sync profile schema missing or wrong shape: %#v", syncTool.InputSchema.Properties["profile"])
	}
	if got := profile["type"]; got != "string" {
		t.Fatalf("sync profile type = %#v, want string", got)
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
		{name: "sync", req: toolRequest(map[string]any{"profile": "Default", "accumulate": false}), want: []string{"sync", "--profile", "Default"}},
		{name: "sync", req: toolRequest(map[string]any{"profile": "Default", "accumulate": true}), want: []string{"sync", "--profile", "Default", "--accumulate"}},
		// Contract pin: a non-boolean accumulate (e.g. a client sending the string
		// "true") is treated as false by reqBool — no --accumulate, no error. The MCP
		// schema declares accumulate as boolean, so a conforming client never hits this;
		// this case documents the silent-false coercion so any future change is conscious.
		{name: "sync", req: toolRequest(map[string]any{"profile": "Default", "accumulate": "true"}), want: []string{"sync", "--profile", "Default"}},
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
