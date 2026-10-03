package cobratree

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mvanhorn/printing-press-library/library/travel/kurumatabi/internal/mcp/bound"
)

func TestFailedToolPreservesValidPartialEnvelope(t *testing.T) {
	partial := `{"meta":{"source":"local","requested_records":2,"compared_records":1,"fetch_failures":[{"id":"yypark/213","error":"not cached"}]},"results":[{"field":"name","values":[{"id":"rvpark/1086","value":"公園"}]}]}`
	result := ToolResultErrorFromCLICommand(CLICommandResult{Stdout: partial}, errors.New("CLI exit5: one required record is missing"))
	if !result.IsError || len(result.Content) != 2 || toolResultContentText(result, 0) != partial || !strings.Contains(toolResultContentText(result, 1), "exit5") {
		t.Fatalf("partial evidence was lost: %+v", result)
	}
	if !json.Valid([]byte(toolResultContentText(result, 0))) {
		t.Fatal("partial JSON was mixed with diagnostics")
	}
}

func TestFailedToolPartialPreviewAndDiagnosticBudget(t *testing.T) {
	partial := `{"meta":{"source":"local"},"results":[{"note":"` + strings.Repeat("a", bound.MaxBytes+100) + `"}]}`
	result := ToolResultErrorFromCLICommand(CLICommandResult{Stdout: partial}, errors.New(strings.Repeat("失敗", 10000)))
	first, second := toolResultContentText(result, 0), toolResultContentText(result, 1)
	if !result.IsError || !json.Valid([]byte(first)) || !strings.Contains(first, `"truncated":true`) {
		t.Fatalf("oversize partial not an explicit failed preview: %s", first[:min(len(first), 200)])
	}
	if len(first) > bound.MaxBytes || len(second) > 4000 || len(first)+len(second) > 64000 || !utf8.ValidString(second) {
		t.Fatalf("unbounded or invalid output: %d + %d", len(first), len(second))
	}
	for _, stdout := range []string{"", "not JSON"} {
		legacy := ToolResultErrorFromCLICommand(CLICommandResult{Stdout: stdout}, errors.New("failed"))
		if !legacy.IsError || len(legacy.Content) != 1 || toolResultText(legacy) != "failed" {
			t.Fatal("legacy failure behavior changed")
		}
	}
}
