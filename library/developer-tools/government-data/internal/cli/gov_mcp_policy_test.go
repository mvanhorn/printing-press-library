package cli

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/mcp/cobratree"
	"github.com/spf13/cobra"
)

// Exercise the actual mirror, rather than checking command annotations alone:
// registration changes schemas and hints when the server is read-only.
func TestGovCatalogMCPPersistencePolicy(t *testing.T) {
	for _, readOnly := range []bool{false, true} {
		name := "normal"
		if readOnly {
			name = "read-only"
		}
		t.Run(name, func(t *testing.T) {
			env := ""
			if readOnly {
				env = "1"
			}
			t.Setenv(cliutil.MCPReadOnlyEnvVar, env)
			root := &cobra.Command{Use: "government-data-pp-cli"}
			flags := &rootFlags{}
			root.AddCommand(newGovCatalog(flags), newGovSync(flags))
			s := server.NewMCPServer("government-data-policy-test", "0.0.0")
			missingBinary := filepath.Join(t.TempDir(), "must-not-spawn")
			cobratree.RegisterAll(s, root, func() (string, error) { return missingBinary, nil })
			tools := s.ListTools()
			for _, path := range [][]string{{"catalog", "find"}, {"catalog", "export"}, {"sync"}} {
				toolName := cobratree.ToolNameForCommand(s, root, strings.Join(path, " "))
				tool := tools[toolName]
				if tool == nil {
					t.Fatalf("missing mirrored tool for %v", path)
				}
				isSync := path[0] == "sync"
				wantReadOnly := readOnly && !isSync
				gotReadOnly := tool.Tool.Annotations.ReadOnlyHint != nil && *tool.Tool.Annotations.ReadOnlyHint
				if gotReadOnly != wantReadOnly {
					t.Errorf("%v readOnlyHint=%v, want %v", path, gotReadOnly, wantReadOnly)
				}
				if !wantReadOnly && (tool.Tool.Annotations.OpenWorldHint == nil || !*tool.Tool.Annotations.OpenWorldHint) {
					t.Errorf("%v persistence must advertise open-world behavior", path)
				}
				if !wantReadOnly && tool.Tool.Annotations.DestructiveHint != nil && !*tool.Tool.Annotations.DestructiveHint {
					t.Errorf("%v must not advertise additive-only persistence", path)
				}
				props := tool.Tool.InputSchema.Properties
				_, hasSave := props["save"]
				if hasSave != (!readOnly || isSync) {
					t.Errorf("%v save parameter exposed=%v", path, hasSave)
				}
				if _, exists := props["db"]; exists {
					t.Errorf("%v exposes an arbitrary database write destination", path)
				}
				blocked := []string{"db"}
				if readOnly && !isSync {
					blocked = append(blocked, "save")
				}
				for _, flag := range blocked {
					result, err := tool.Handler(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: map[string]any{flag: "unapproved-destination"}}})
					if err != nil || result == nil || !result.IsError {
						t.Fatalf("%v accepted blocked %s: result=%v error=%v", path, flag, result, err)
					}
					var text string
					for _, content := range result.Content {
						if c, ok := content.(mcplib.TextContent); ok {
							text += c.Text
						}
					}
					if !strings.Contains(text, flag) || strings.Contains(text, "companion CLI binary not found") {
						t.Errorf("%v %s was not refused before shell-out: %s", path, flag, text)
					}
				}
			}
		})
	}
}
