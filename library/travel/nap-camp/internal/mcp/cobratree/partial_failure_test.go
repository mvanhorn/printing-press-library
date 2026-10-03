// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cobratree

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mvanhorn/printing-press-library/library/travel/nap-camp/internal/mcp/bound"
)

func TestFailedAdapterPreservesValidPartialEvidence(t *testing.T) {
	partial := `{"results":[{"pair":"11007:20005062"}],"errors":[{"pair":"11008:20005100"}],"complete":false}`
	result := ToolResultFromFailedCLICommand(CLICommandResult{Stdout: partial}, errors.New("one source failed"))
	if !result.IsError {
		t.Fatal("nonzero command became successful MCP result")
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(toolResultText(result)), &envelope); err != nil {
		t.Fatal(err)
	}
	data := envelope["partial_output"].(map[string]any)
	if data["complete"] != false || len(data["results"].([]any)) != 1 || len(data["errors"].([]any)) != 1 || envelope["error"] == nil {
		t.Fatalf("partial facts lost: %#v", envelope)
	}
}

func TestFailedAdapterPreviewAndOverallBudget(t *testing.T) {
	cases := []struct{ name, stdout, diagnostic string }{
		{"opaque", "partial opaque output", "failed"},
		{"capture-overflow-valid-prefix", `{"complete":false}` + strings.Repeat(" ", bound.MaxBytes), "failed"},
		{"invalid-utf8", string([]byte{'{', '"', 'x', '"', ':', '"', 0xff, '"', '}'}), "failed"},
		{"escaped-budget", `{"text":"` + strings.Repeat("<", 18000) + `"}`, strings.Repeat("\x00", 4001)},
		{"large-multibyte", strings.Repeat("なっぷ", 12000), strings.Repeat("界", 3000)},
		{"large-diagnostic", "opaque", strings.Repeat("error", bound.MaxBytes)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := ToolResultFromFailedCLICommand(CLICommandResult{Stdout: tc.stdout}, errors.New(tc.diagnostic))
			text := toolResultText(result)
			if !result.IsError || len(text) > bound.MaxBytes || !utf8.ValidString(text) {
				t.Fatalf("invalid failed result: error=%v bytes=%d", result.IsError, len(text))
			}
			var envelope map[string]any
			if err := json.Unmarshal([]byte(text), &envelope); err != nil {
				t.Fatal(err)
			}
			if envelope["error"] == nil || envelope["partial_stdout_is_preview"] != true || envelope["partial_stdout_encoding"] != "utf-8" {
				t.Fatalf("preview not labeled with error context: %#v", envelope)
			}
			if _, fabricated := envelope["partial_output"]; fabricated {
				t.Fatal("opaque/oversize output fabricated structured facts")
			}
		})
	}
}

func TestActualShellAdapterPreservesFailedChildJSON(t *testing.T) {
	temp := t.TempDir()
	source := filepath.Join(temp, "main.go")
	binary := filepath.Join(temp, "failed-child")
	program := "package main\nimport (\"fmt\";\"os\")\nfunc main(){fmt.Fprint(os.Stdout,`{\"results\":[{\"pair\":\"11007:20005062\"}],\"errors\":[{\"pair\":\"11008:20005100\"}],\"complete\":false}`);fmt.Fprintln(os.Stderr,\"one source failed\");os.Exit(7)}"
	if err := os.WriteFile(source, []byte(program), 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "build", "-o", binary, source)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("building helper: %v %s", err, output)
	}
	handler := shellOutToCLI(func() (string, error) { return binary, nil }, nil, map[string]bool{}, map[string]bool{}, nil, false, nil)
	result, err := handler(context.Background(), mcplib.CallToolRequest{})
	if err != nil || result == nil || !result.IsError {
		t.Fatalf("failed child became successful/protocol failure: %v %#v", err, result)
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(toolResultText(result)), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["exit_code"] != float64(7) || envelope["partial_output"].(map[string]any)["complete"] != false || !strings.Contains(fmt.Sprint(envelope["error"]), "one source failed") {
		t.Fatalf("actual adapter lost stdout/context: %#v", envelope)
	}
}
