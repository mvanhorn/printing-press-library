package cobratree

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tablecheck/internal/mcp/bound"
)

// This child acts as a companion CLI and exits before the Go test runner can
// append its own output. The parent supplies a complete valid inventory JSON.
func TestOversizeCompanionCLIHelper(t *testing.T) {
	if os.Getenv("TABLECHECK_MCP_OUTPUT_TEST_CHILD") != "1" {
		return
	}
	payload, err := os.ReadFile(os.Args[len(os.Args)-1])
	if err != nil {
		os.Exit(7)
	}
	if _, err := os.Stdout.Write(payload); err != nil {
		os.Exit(7)
	}
	os.Exit(0)
}

func inventoryJSONAtSize(t *testing.T, size int, multibyte bool) string {
	t.Helper()
	prefix := `{"checks":[{"venue_id":"venue-123","name_ja":"`
	suffix := `","status":"available"}],"meta":{"requests":0}}`
	remaining := size - len(prefix) - len(suffix)
	if remaining < 0 {
		t.Fatal("requested inventory size is too small")
	}
	filler := strings.Repeat("x", remaining)
	if multibyte {
		filler = strings.Repeat("鮨", remaining/len("鮨")) + strings.Repeat("x", remaining%len("鮨"))
	}
	payload := prefix + filler + suffix
	if len(payload) != size || !json.Valid([]byte(payload)) || !utf8.ValidString(payload) {
		t.Fatalf("invalid fixture: bytes=%d, requested=%d", len(payload), size)
	}
	return payload
}

func companionWithInventory(t *testing.T, payload string) (string, []string) {
	t.Helper()
	t.Setenv("TABLECHECK_MCP_OUTPUT_TEST_CHILD", "1")
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "inventory.json")
	if err := os.WriteFile(file, []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
	return bin, []string{"-test.run=^TestOversizeCompanionCLIHelper$", "--", file}
}

func assertOversizeRecovery(t *testing.T, result *mcplib.CallToolResult) {
	t.Helper()
	if result == nil || !result.IsError {
		t.Fatalf("oversized inventory returned successful MCP result: %v", result)
	}
	if len(result.Content) != 1 {
		t.Fatalf("oversized inventory returned additional partial content: %d blocks", len(result.Content))
	}
	text := toolResultText(result)
	for _, required := range []string{strconv.Itoa(bound.MaxBytes), "venue", "date", "--limit", "--select", "CLI"} {
		if !strings.Contains(text, required) {
			t.Fatalf("recovery instructions omit %q: %q", required, text)
		}
	}
	for _, forbidden := range []string{`"checks"`, `"preview"`, `"status":"available"`, "venue-123"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("oversize error leaked inventory/preview fragment %q", forbidden)
		}
	}
	if len(text) > bound.MaxBytes || !utf8.ValidString(text) {
		t.Fatal("oversize error itself violates the MCP byte/UTF-8 bounds")
	}
}

func TestShelloutOversizedInventoryIsErrorWithoutPartialChecks(t *testing.T) {
	payload := inventoryJSONAtSize(t, bound.MaxBytes+5000, true)
	bin, args := companionWithInventory(t, payload)
	commandResult, err := RunCLICommand(context.Background(), bin, args)
	if err == nil {
		t.Fatal("valid JSON inventory over 60KB returned success")
	}
	if commandResult.Stdout != "" {
		t.Fatal("failed oversized command retained partial stdout inventory")
	}
	handler := shellOutToCLI(func() (string, error) { return bin, nil }, args, nil, nil, nil, true, nil)
	result, transportErr := handler(context.Background(), mcplib.CallToolRequest{})
	if transportErr != nil {
		t.Fatalf("expected MCP tool error, got transport error: %v", transportErr)
	}
	assertOversizeRecovery(t, result)
}

func TestMCPStdoutByteBoundariesPreserveCompleteJSON(t *testing.T) {
	for _, multibyte := range []bool{false, true} {
		for _, size := range []int{bound.MaxBytes - 1, bound.MaxBytes, bound.MaxBytes + 1} {
			name := strconv.Itoa(size)
			if multibyte {
				name += "-multibyte"
			}
			t.Run(name, func(t *testing.T) {
				payload := inventoryJSONAtSize(t, size, multibyte)
				bin, args := companionWithInventory(t, payload)
				commandResult, err := RunCLICommand(context.Background(), bin, args)
				if size > bound.MaxBytes {
					if err == nil || commandResult.Stdout != "" {
						t.Fatal("oversize companion result was accepted or returned a partial inventory")
					}
					assertOversizeRecovery(t, ToolResultFromCLICommand(CLICommandResult{Stdout: payload, StderrHints: []string{"warning: ignored partial hint"}}))
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if commandResult.Stdout != payload {
					t.Fatal("near-boundary companion JSON was not preserved byte for byte")
				}
				result := ToolResultFromCLICommand(commandResult)
				if result.IsError || toolResultText(result) != payload {
					t.Fatal("near-boundary MCP JSON was changed or rejected")
				}
			})
		}
	}
}
