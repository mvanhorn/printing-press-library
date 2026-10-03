package mcp

import (
	"context"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/travel/hostelworld/internal/platform"
	"path/filepath"
	"testing"
)

func TestPlanningToolsDeclareOptionalLocalSaveAndLiveSource(t *testing.T) {
	s := server.NewMCPServer("test", "test")
	RegisterTools(s)
	for _, name := range []string{"hostels_inspect", "hostels_offers", "hostels_compare", "hostels_dates", "hostels_search"} {
		entry := s.GetTool(name)
		if entry == nil {
			t.Fatalf("missing %s", name)
		}
		a := entry.Tool.Annotations
		if a.ReadOnlyHint == nil || *a.ReadOnlyHint || a.DestructiveHint == nil || *a.DestructiveHint || a.OpenWorldHint == nil || !*a.OpenWorldHint {
			t.Fatalf("%s: misleading annotations %#v", name, a)
		}
		if _, ok := entry.Tool.InputSchema.Properties["save"]; !ok {
			t.Fatalf("%s save is not exposed", name)
		}
	}
	entry := s.GetTool("hostels_compare")
	for _, key := range []string{"id", "id2", "id3", "id4", "id5"} {
		if _, ok := entry.Tool.InputSchema.Properties[key]; !ok {
			t.Fatalf("missing distinct compare slot %s", key)
		}
	}
}

func TestMCPSearchAndSQLUseVerifiedPlanningProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile", "data.db")
	ctx := platform.ContextWithSession(context.Background(), &platform.Session{GateOutcome: platform.GateVerified, Paths: platform.Paths{DataFile: path}})
	if got, err := mcpDBPath(ctx); err != nil || got != path {
		t.Fatalf("MCP cache path=%q: %v", got, err)
	}
	bad := platform.ContextWithSession(context.Background(), &platform.Session{Paths: platform.Paths{DataFile: path}})
	if _, err := mcpDBPath(bad); err == nil {
		t.Fatal("MCP unverified profile fell back to shared database")
	}
}
