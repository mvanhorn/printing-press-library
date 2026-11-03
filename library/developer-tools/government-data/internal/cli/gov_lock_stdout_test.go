package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/govdata"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/mcp/cobratree"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/store"
	"github.com/spf13/cobra"
)

func TestIntakeLockDefaultsToStdoutAndIsUsableByMCP(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "catalog.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := govdata.Snapshot{ID: "fixture", CapturedAt: "2026-11-03T00:00:00Z", Datasets: []govdata.Dataset{{Source: "california", ID: "ca1", Title: "Water", Resources: []govdata.Resource{{ID: "r1", URL: "https://agency.example.org/data.csv", Format: "CSV"}}}}}
	if err := db.SaveCatalogSnapshot(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	cmd := newGovLocalCmd(&rootFlags{asJSON: true}, "lock")
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--snapshot", "fixture", "--resource", "california:ca1:r1", "--db", dbPath})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal(output.Bytes(), &manifest); err != nil || len(manifest.Results) != 1 {
		t.Fatalf("default stdout manifest: %s; %v", output.String(), err)
	}
	if manifest.Results[0]["url"] != "https://agency.example.org/data.csv" {
		t.Fatal("manifest selected wrong resource")
	}
	for _, mode := range []string{"", "1"} {
		t.Setenv(cliutil.MCPReadOnlyEnvVar, mode)
		root := &cobra.Command{Use: "government-data-pp-cli"}
		parent := &cobra.Command{Use: "intake"}
		parent.AddCommand(newGovLocalCmd(&rootFlags{}, "lock"))
		root.AddCommand(parent)
		s := server.NewMCPServer("lock-policy-test", "0.0.0")
		cobratree.RegisterAll(s, root, func() (string, error) { return "unneeded-test-binary", nil })
		tool := s.ListTools()[cobratree.ToolNameForCommand(s, root, "intake lock")]
		if tool == nil || tool.Tool.Annotations.ReadOnlyHint == nil || !*tool.Tool.Annotations.ReadOnlyHint {
			t.Fatal("stdout manifest missing as read-only MCP tool")
		}
		if _, exposed := tool.Tool.InputSchema.Properties["output"]; exposed {
			t.Fatal("MCP exposed filesystem destination")
		}
		for _, name := range []string{"snapshot", "resource"} {
			if _, exists := tool.Tool.InputSchema.Properties[name]; !exists {
				t.Fatalf("MCP cannot supply required %s", name)
			}
		}
	}
}
