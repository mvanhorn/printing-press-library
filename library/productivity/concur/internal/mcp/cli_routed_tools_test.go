// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// PATCH(Greptile review, "MCP misses the new commands") -- the itemization
// reads and the type edit must be reachable from MCP, and must keep working
// where an in-process typed tool cannot run.

func TestCLIRoutedToolsAreRegisteredAsChildCLITools(t *testing.T) {
	s := server.NewMCPServer("concur", "test")
	RegisterTools(s)
	registered := s.ListTools()

	want := map[string]struct {
		readOnly bool
		required []string
		props    []string
	}{
		"expenses_itemizations": {true, []string{"expense-id", "report-id", "user_id"}, []string{"context-type", "expense-id", "report-id", "user_id"}},
		"reports_exceptions":    {true, []string{"report-id", "user_id"}, []string{"context-type", "report-id", "user_id"}},
		"expenses_update_type":  {false, []string{"expense-id", "report-id", "type", "user_id"}, []string{"context-type", "expense-id", "report-id", "type", "user_id"}},
	}
	for name, w := range want {
		entry, ok := registered[name]
		if !ok {
			t.Errorf("%s is not registered", name)
			continue
		}
		if owner, _ := entry.Tool.Meta.AdditionalFields[mcpTenantGateOwnerKey].(string); owner != "child-cli" {
			t.Errorf("%s: tenant-gate owner = %q, want child-cli (the CLI owns the gate; a parent probe would refuse every call)", name, owner)
		}
		gotReadOnly := entry.Tool.Annotations.ReadOnlyHint != nil && *entry.Tool.Annotations.ReadOnlyHint
		if gotReadOnly != w.readOnly {
			t.Errorf("%s: readOnlyHint = %v, want %v", name, gotReadOnly, w.readOnly)
		}
		required := append([]string{}, entry.Tool.InputSchema.Required...)
		sort.Strings(required)
		if strings.Join(required, ",") != strings.Join(w.required, ",") {
			t.Errorf("%s: required = %v, want %v", name, required, w.required)
		}
		var props []string
		for p := range entry.Tool.InputSchema.Properties {
			props = append(props, p)
		}
		sort.Strings(props)
		if strings.Join(props, ",") != strings.Join(w.props, ",") {
			t.Errorf("%s: properties = %v, want %v", name, props, w.props)
		}
	}
}

func TestCLIRoutedToolBuildsCLIInvocation(t *testing.T) {
	useCLIRecorder(t)

	tests := []struct {
		name string
		tool string
		args map[string]any
		want string
	}{
		{
			name: "itemizations",
			tool: "expenses_itemizations",
			args: map[string]any{"user_id": "u1", "context-type": "TRAVELER", "report-id": "r1", "expense-id": "e1"},
			want: "expenses itemizations --user-id=u1 --context-type=TRAVELER --report-id=r1 --agent -- e1",
		},
		{
			name: "exceptions with the default context-type left out",
			tool: "reports_exceptions",
			args: map[string]any{"user_id": "u1", "report-id": "r1"},
			want: "reports exceptions --user-id=u1 --agent -- r1",
		},
		{
			name: "type edit",
			tool: "expenses_update_type",
			args: map[string]any{"user_id": "u1", "report-id": "r1", "expense-id": "e1", "type": "Hotel Room"},
			want: "expenses update --user-id=u1 --report-id=r1 --type=Hotel Room --agent -- e1",
		},
		{
			name: "surrounding whitespace is trimmed and blank optionals dropped",
			tool: "reports_exceptions",
			args: map[string]any{"user_id": " u1 ", "context-type": "  ", "report-id": "r1\n"},
			want: "reports exceptions --user-id=u1 --agent -- r1",
		},
		{
			// A flag-looking value must stay a value: flags use the = form and
			// the positional follows "--".
			name: "flag-like values cannot become flags",
			tool: "expenses_update_type",
			args: map[string]any{"user_id": "--yes", "report-id": "--stdin", "expense-id": "--dry-run", "type": "--purpose"},
			want: "expenses update --user-id=--yes --report-id=--stdin --type=--purpose --agent -- --dry-run",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := callCLIRoutedTool(t, tt.tool, tt.args)
			if result.IsError {
				t.Fatalf("unexpected tool error: %s", recipeToolText(t, result))
			}
			if got := strings.TrimSpace(recipeToolText(t, result)); got != tt.want {
				t.Fatalf("CLI invoked as %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCLIRoutedToolRejectsBadInputWithoutRunningTheCLI(t *testing.T) {
	cliRan := false
	original := cliRoutedCLIPath
	t.Cleanup(func() { cliRoutedCLIPath = original })
	cliRoutedCLIPath = func() (string, error) {
		cliRan = true
		return "", errors.New("must not be resolved")
	}

	tests := []struct {
		name string
		tool string
		args map[string]any
		want string
	}{
		{"missing required", "expenses_update_type", map[string]any{"user_id": "u1", "report-id": "r1", "expense-id": "e1"}, "missing required parameter(s): type"},
		{"blank required", "reports_exceptions", map[string]any{"user_id": "u1", "report-id": "   "}, "missing required parameter(s): report-id"},
		{"all missing", "expenses_itemizations", map[string]any{}, "missing required parameter(s): user_id, report-id, expense-id"},
		{"unknown parameter", "expenses_update_type", map[string]any{"user_id": "u1", "report-id": "r1", "expense-id": "e1", "type": "Hotel", "purpose": "x"}, `unknown parameter(s) purpose`},
		{"non-string value", "reports_exceptions", map[string]any{"user_id": "u1", "report-id": 12}, `parameter "report-id" must be a string`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := callCLIRoutedTool(t, tt.tool, tt.args)
			if !result.IsError {
				t.Fatalf("expected a tool error, got: %s", recipeToolText(t, result))
			}
			if got := recipeToolText(t, result); !strings.Contains(got, tt.want) {
				t.Fatalf("error = %q, want it to contain %q", got, tt.want)
			}
		})
	}
	if cliRan {
		t.Fatal("the companion CLI was resolved for invalid input")
	}
}

func TestCLIRoutedToolSurfacesCLIFailure(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "failing-cli.sh")
	body := "#!/bin/sh\necho 'expense type \"Train\" is not offered; offered: Airfare, Hotel' >&2\nexit 3\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	original := cliRoutedCLIPath
	t.Cleanup(func() { cliRoutedCLIPath = original })
	cliRoutedCLIPath = func() (string, error) { return script, nil }

	result := callCLIRoutedTool(t, "expenses_update_type", map[string]any{"user_id": "u1", "report-id": "r1", "expense-id": "e1", "type": "Train"})
	if !result.IsError {
		t.Fatalf("a failing CLI must produce a tool error, got: %s", recipeToolText(t, result))
	}
	if got := recipeToolText(t, result); !strings.Contains(got, "offered: Airfare, Hotel") {
		t.Fatalf("the CLI's stderr should reach the agent, got: %s", got)
	}
}

func TestCLIRoutedToolReportsMissingCLI(t *testing.T) {
	original := cliRoutedCLIPath
	t.Cleanup(func() { cliRoutedCLIPath = original })
	cliRoutedCLIPath = func() (string, error) { return "", errors.New("not on PATH") }

	result := callCLIRoutedTool(t, "reports_exceptions", map[string]any{"user_id": "u1", "report-id": "r1"})
	if !result.IsError {
		t.Fatal("expected a tool error when the companion CLI is missing")
	}
	if got := recipeToolText(t, result); !strings.Contains(got, "companion CLI binary not found: not on PATH") {
		t.Fatalf("error = %q", got)
	}
}

// useCLIRecorder points the tools at a stand-in CLI that prints its argv.
func useCLIRecorder(t *testing.T) {
	t.Helper()
	recorder := writeRecipeIntentRecorder(t)
	original := cliRoutedCLIPath
	t.Cleanup(func() { cliRoutedCLIPath = original })
	cliRoutedCLIPath = func() (string, error) { return recorder, nil }
}

// callCLIRoutedTool invokes the named tool's own handler, bypassing the
// server middleware that the tenant-gate tests cover.
func callCLIRoutedTool(t *testing.T, name string, args map[string]any) *mcplib.CallToolResult {
	t.Helper()
	for _, tool := range cliRoutedTools {
		if tool.name != name {
			continue
		}
		result, err := tool.handler()(context.Background(), mcplib.CallToolRequest{
			Params: mcplib.CallToolParams{Name: name, Arguments: args},
		})
		if err != nil {
			t.Fatalf("%s: handler returned a protocol error: %v", name, err)
		}
		return result
	}
	t.Fatalf("no CLI-routed tool named %q", name)
	return nil
}
