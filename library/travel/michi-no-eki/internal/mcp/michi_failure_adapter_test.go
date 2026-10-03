package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/travel/michi-no-eki/internal/mcp/bound"
	"github.com/mvanhorn/printing-press-library/library/travel/michi-no-eki/internal/mcp/cobratree"
)

func TestMichiFailureAdapterPreservesStructuredAccounting(t *testing.T) {
	for _, tc := range []struct {
		name, payload string
		code          int
		failed        bool
	}{
		{"partial", `{"meta":{"source":"live"},"results":{"requested_ids":["1","2"],"requested_count":2,"successful_count":1,"stations":[{"id":"1","name":"synthetic station"}],"fetch_failures":[{"id":"2","error":"source unavailable"}]}}`, 0, false},
		{"all-failed", `{"meta":{"source":"live"},"results":{"requested_ids":["1","2"],"requested_count":2,"successful_count":0,"stations":[],"fetch_failures":[{"id":"1","error":"source unavailable"},{"id":"2","error":"source unavailable"}]}}`, 5, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bin, count := writeMichiFailureCompanion(t, tc.payload, tc.code)
			s := server.NewMCPServer("test", "test")
			tool := mcplib.NewTool("compare", mcplib.WithString("ids"), mcplib.WithBoolean("agent"), mcplib.WithReadOnlyHintAnnotation(true))
			tool.Meta = &mcplib.Meta{AdditionalFields: map[string]any{"pp:cli-command": "compare", "pp:tenant-gate": "child-cli"}}
			s.AddTool(tool, func(context.Context, mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
				t.Fatal("old handler used")
				return nil, nil
			})
			registerMichiFailureAdapters(s, func() (string, error) { return bin, nil })
			registered := s.GetTool("compare")
			if registered.Tool.Meta.AdditionalFields["pp:tenant-gate"] != "child-cli" || registered.Tool.Annotations.ReadOnlyHint == nil || !*registered.Tool.Annotations.ReadOnlyHint {
				t.Fatal("tool identity or annotations lost")
			}
			result, err := registered.Handler(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: map[string]any{"ids": "1,2", "agent": true}}})
			if err != nil || result == nil || result.IsError != tc.failed {
				t.Fatalf("result=%+v error=%v", result, err)
			}
			first, ok := result.Content[0].(mcplib.TextContent)
			if !ok || !json.Valid([]byte(first.Text)) || strings.TrimSpace(first.Text) != tc.payload {
				t.Fatalf("structured evidence changed: %+v", result.Content)
			}
			if tc.failed && (len(result.Content) < 2 || !strings.Contains(result.Content[1].(mcplib.TextContent).Text, "exit status 5")) {
				t.Fatal("failed tool lost explicit exit diagnostic")
			}
			data, err := os.ReadFile(count)
			if err != nil || strings.Count(string(data), "x") != 1 {
				t.Fatalf("companion rerun: %q, %v", data, err)
			}
		})
	}
}

func TestMichiMirrorArgsKeepsSchemaBoundary(t *testing.T) {
	properties := map[string]any{"id": nil, "args": nil, "agent": nil, "select": nil, "max-scan-pages": nil}
	argv, err := michiMirrorArgs("station-notices", properties, map[string]any{"id": "123", "agent": true, "select": "--home=/tmp/injection", "max-scan-pages": float64(2)})
	if err != nil || strings.Join(argv, " ") != "station-notices --agent --max-scan-pages=2 --select=--home=/tmp/injection 123" {
		t.Fatalf("argv=%q error=%v", argv, err)
	}
	for _, args := range []map[string]any{{"token": "secret"}, {"home": "/tmp/other"}, {"args": "123 --token=secret"}, {"id": "--home=/tmp/other"}, {"agent=true": true}} {
		if _, err := michiMirrorArgs("station-notices", properties, args); err == nil {
			t.Fatalf("unsafe or unknown args accepted: %v", args)
		}
	}
}

func TestMichiFailedCompanionResultBoundsAndFallback(t *testing.T) {
	for _, stdout := range []string{"", "not JSON", `{"truncated":` + strings.Repeat("x", bound.MaxBytes)} {
		result := michiFailedCompanionResult(cobratree.CLICommandResult{Stdout: stdout}, fmt.Errorf("source failure %s", strings.Repeat("x", bound.MaxBytes+1)))
		if !result.IsError || len(result.Content) != 1 || len(result.Content[0].(mcplib.TextContent).Text) > bound.MaxBytes {
			t.Fatal("fallback not bounded error")
		}
	}
	payload := `{"data":"` + strings.Repeat("x", bound.MaxBytes-100) + `"}`
	result := michiFailedCompanionResult(cobratree.CLICommandResult{Stdout: payload}, fmt.Errorf("error %s", strings.Repeat("失敗", bound.MaxBytes)))
	total := 0
	for _, item := range result.Content {
		total += len(item.(mcplib.TextContent).Text)
	}
	if !result.IsError || total > bound.MaxBytes || result.Content[0].(mcplib.TextContent).Text != payload {
		t.Fatal("bounded factual output lost")
	}
}

func writeMichiFailureCompanion(t *testing.T, payload string, code int) (string, string) {
	t.Helper()
	dir := t.TempDir()
	count := filepath.Join(dir, "calls.txt")
	path := filepath.Join(dir, "helper.sh")
	body := "#!/bin/sh\nprintf '%s\\n' '" + payload + "'\nprintf 'warning: source fetch failures\\n' >&2\nprintf x >> '" + count + "'\nexit " + fmt.Sprint(code) + "\n"
	if runtime.GOOS == "windows" {
		path = filepath.Join(dir, "helper.bat")
		body = "@echo off\r\necho " + payload + "\r\necho warning: source fetch failures 1>&2\r\n<nul set /p \"=x\" >> \"" + count + "\"\r\nexit /b " + fmt.Sprint(code) + "\r\n"
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path, count
}

func TestMichiFailedCompanionResultBoundsInvalidUTF8(t *testing.T) {
	payload := `{"data":"` + strings.Repeat("x", bound.MaxBytes-20) + `"}`
	result := michiFailedCompanionResult(cobratree.CLICommandResult{Stdout: payload}, fmt.Errorf("%s", strings.Repeat(string([]byte{0xff}), 8)))
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded mcplib.CallToolResult
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, item := range wire.Content {
		total += len(item.Text)
	}
	if !result.IsError || total > bound.MaxBytes || wire.Content[0].Text != payload {
		t.Fatalf("decoded text grew beyond bound: %d bytes", total)
	}
	invalid := "{\"data\":\"" + string([]byte{0xff}) + "\"}"
	if !json.Valid([]byte(invalid)) {
		t.Fatal("fixture should expose JSON's permissive UTF-8 acceptance")
	}
	fallback := michiFailedCompanionResult(cobratree.CLICommandResult{Stdout: invalid}, fmt.Errorf("failed"))
	if !fallback.IsError || len(fallback.Content) != 1 || fallback.Content[0].(mcplib.TextContent).Text == invalid {
		t.Fatal("invalid UTF-8 stdout was preserved as complete evidence")
	}
}
