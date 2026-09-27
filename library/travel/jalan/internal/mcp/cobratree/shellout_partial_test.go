package cobratree

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mvanhorn/printing-press-library/library/travel/jalan/internal/mcp/bound"
)

func TestShellOutStayPartialResultWithRealCLI(t *testing.T) {
	binary := buildStayPartialCompanion(t)
	cases := []struct {
		name, scenario                 string
		path                           []string
		wantData, wantError, oversized bool
	}{
		{name: "stay partial", scenario: "partial", path: []string{"stay", "compare"}, wantData: true, wantError: true},
		{name: "ordinary success", scenario: "success", path: []string{"stay", "offers"}, wantData: true},
		{name: "other exit code discards data", scenario: "failure", path: []string{"stay", "compare"}, wantError: true},
		{name: "non stay exit8 discards data", scenario: "partial", path: []string{"pages", "list"}, wantError: true},
		{name: "empty exit8 stdout", scenario: "empty", path: []string{"stay", "compare"}, wantError: true},
		{name: "oversized partial remains bounded and explicit", scenario: "oversized", path: []string{"stay", "compare"}, wantData: true, wantError: true, oversized: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler := shellOutToCLI(func() (string, error) { return binary, nil }, tc.path, map[string]bool{}, map[string]bool{"fixture-scenario": true}, nil, true, nil)
			result, err := handler(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: map[string]any{"fixture-scenario": tc.scenario}}})
			if err != nil {
				t.Fatalf("unexpected transport error: %v", err)
			}
			if result.IsError != tc.wantError {
				t.Fatalf("IsError=%v want%v", result.IsError, tc.wantError)
			}
			for index := range result.Content {
				if size := len(toolResultContentText(result, index)); size > bound.MaxBytes {
					t.Fatalf("content block%d exceeds bound: %d", index, size)
				}
			}
			if !tc.wantData {
				if len(result.Content) != 1 {
					t.Fatalf("failure exposed extra stdout block: %+v", result.Content)
				}
				diagnostic := toolResultText(result)
				if strings.Contains(diagnostic, "successful-result") || strings.Contains(diagnostic, `"results"`) {
					t.Fatalf("ordinary failure exposed stdout: %s", diagnostic)
				}
				return
			}
			if tc.wantError {
				if len(result.Content) != 2 {
					t.Fatalf("partial response lost stdout or diagnostic: blocks=%d", len(result.Content))
				}
				diagnostic := toolResultContentText(result, 1)
				if !strings.Contains(diagnostic, "exit status 8") || !strings.Contains(diagnostic, "partial") {
					t.Fatalf("partial exit or diagnostic hidden: %s", diagnostic)
				}
			}
			var payload map[string]any
			if err := json.Unmarshal([]byte(toolResultText(result)), &payload); err != nil {
				t.Fatalf("data content is not JSON: %v", err)
			}
			if tc.oversized {
				if payload["truncated"] != true || payload["_pp_truncated"] != true || payload["resumable"] != false || payload["max_bytes"] != float64(bound.MaxBytes) || payload["original_bytes"].(float64) <= bound.MaxBytes {
					t.Fatalf("oversized partial lacked explicit truncation: %+v", payload)
				}
				if _, ok := payload["preview"].(string); !ok {
					t.Fatal("bounded partial preview was lost")
				}
				if !strings.Contains(toolResultContentText(result, 1), `"truncated":true`) {
					t.Fatal("oversized diagnostic was not explicitly bounded")
				}
				return
			}
			meta, ok := payload["meta"].(map[string]any)
			if !ok {
				t.Fatal("meta lost")
			}
			results, ok := payload["results"].([]any)
			if !ok || len(results) != 1 {
				t.Fatalf("successful subset lost: %v", payload)
			}
			if results[0].(map[string]any)["id"] != "successful-result" {
				t.Fatalf("wrong successful subset: %v", results)
			}
			failures, ok := payload["fetch_failures"].([]any)
			if !ok {
				t.Fatal("fetch_failures lost")
			}
			if tc.wantError {
				if meta["status"] != "partial" || len(failures) != 1 || failures[0].(map[string]any)["code"] != "access_failure" {
					t.Fatalf("partial coverage or failure lost: %v", payload)
				}
			} else {
				if meta["status"] != "ok" || len(failures) != 0 {
					t.Fatalf("ordinary success changed: %v", payload)
				}
				if len(result.Content) != 2 || toolResultContentText(result, 1) != "warning: fixture success hint" {
					t.Fatal("ordinary success hints changed")
				}
			}
		})
	}
}

func buildStayPartialCompanion(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	source := filepath.Join(directory, "companion.go")
	program := `package main
import("encoding/json";"os";"strings")
func main(){
 scenario:="partial"
 for _,arg:=range os.Args[1:] {if strings.HasPrefix(arg,"--fixture-scenario="){scenario=strings.TrimPrefix(arg,"--fixture-scenario=")}}
 failures:=[]any{map[string]any{"code":"access_failure","check_in":"2026-11-11"}}
 status:="partial";exitCode:=8
 if scenario=="success" {status="ok";failures=[]any{};exitCode=0}
 if scenario=="failure" {exitCode=7}
 name:="bounded successful observation"
 if scenario=="oversized" {name=strings.Repeat("x",80000)}
 if scenario=="empty" {_,_=os.Stdout.WriteString(" \n")} else {
  _=json.NewEncoder(os.Stdout).Encode(map[string]any{"meta":map[string]any{"status":status,"source":"fixture","observed_at":"2026-09-27T15:00:00Z"},"results":[]any{map[string]any{"id":"successful-result","name":name}},"pagination":map[string]any{"has_more":false},"fetch_failures":failures})
 }
 if exitCode==0 {_,_=os.Stderr.WriteString("warning: fixture success hint\n")} else {
  message:="partial fixture diagnostic"
  if scenario=="oversized" {message+=strings.Repeat("y",80000)}
  _=json.NewEncoder(os.Stderr).Encode(map[string]any{"error":map[string]any{"code":"partial","message":message},"exit_code":exitCode})
 }
 os.Exit(exitCode)
}
`
	if err := os.WriteFile(source, []byte(program), 0600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(directory, "companion")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	if output, err := exec.Command("go", "build", "-o", binary, source).CombinedOutput(); err != nil {
		t.Fatalf("build real companion: %v\n%s", err, output)
	}
	return binary
}
