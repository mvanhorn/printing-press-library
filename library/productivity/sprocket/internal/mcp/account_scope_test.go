package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mvanhorn/printing-press-library/library/productivity/sprocket/internal/store"
)

func TestMCPToolsReadOnlyCurrentAccountScope(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SPROCKET_TOKEN", "")
	t.Setenv("SPROCKET_BASE_URL", "")
	t.Setenv("SPROCKET_CLUB", "")
	configA := filepath.Join(home, "account-a.toml")
	configB := filepath.Join(home, "account-b.toml")
	if err := os.WriteFile(configA, []byte("token = \"synthetic-account-a\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configB, []byte("token = \"synthetic-account-b\"\n"), 0600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SPROCKET_CONFIG", configA)
	pathA, err := dbPath()
	if err != nil {
		t.Fatal(err)
	}
	seedMCPAccountResource(t, pathA, "account-a-only")
	clientA, err := newMCPClient()
	if err != nil {
		t.Fatal(err)
	}
	if clientA.Config.Path != configA {
		t.Fatal("MCP client used a different account configuration")
	}

	t.Setenv("SPROCKET_CONFIG", configB)
	pathB, err := dbPath()
	if err != nil {
		t.Fatal(err)
	}
	if pathA == pathB {
		t.Fatal("two account configurations selected the same database")
	}
	seedMCPAccountResource(t, pathB, "account-b-only")
	seedMCPAccountResource(t, filepath.Join(home, ".local", "share", "sprocket-pp-cli", "data.db"), "legacy-shared-only")

	for _, tc := range []struct {
		name       string
		configPath string
		want       string
		other      string
	}{
		{name: "account A", configPath: configA, want: "account-a-only", other: "account-b-only"},
		{name: "account B", configPath: configB, want: "account-b-only", other: "account-a-only"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SPROCKET_CONFIG", tc.configPath)
			for _, tool := range []struct {
				name string
				call func(context.Context, mcplib.CallToolRequest) (*mcplib.CallToolResult, error)
				args map[string]any
			}{
				{name: "search", call: handleSearch, args: map[string]any{"query": tc.want}},
				{name: "sql", call: handleSQL, args: map[string]any{"query": "SELECT id FROM resources ORDER BY id"}},
			} {
				result, err := tool.call(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: tool.args}})
				if err != nil {
					t.Fatalf("%s: %v", tool.name, err)
				}
				if result.IsError || len(result.Content) != 1 {
					t.Fatalf("%s result = %+v", tool.name, result)
				}
				content, ok := mcplib.AsTextContent(result.Content[0])
				if !ok || !json.Valid([]byte(content.Text)) {
					t.Fatalf("%s output is not JSON text", tool.name)
				}
				if !strings.Contains(content.Text, tc.want) || strings.Contains(content.Text, tc.other) || strings.Contains(content.Text, "legacy-shared-only") {
					t.Fatalf("%s returned another account's data: %s", tool.name, content.Text)
				}
			}
		})
	}
}

func seedMCPAccountResource(t *testing.T, path, id string) {
	t.Helper()
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	data, err := json.Marshal(map[string]string{"id": id, "name": id})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Upsert("teams", id, data); err != nil {
		t.Fatal(err)
	}
}
