package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/mcp/cobratree"
)

func TestEtsyMCPWriteSeam(t *testing.T) {
	// Build before isolating HOME so the compiler keeps its normal dependency cache.
	bin := filepath.Join(t.TempDir(), "etsy-pp-cli")
	build := exec.Command("go", "build", "-o", bin, "./cmd/etsy-pp-cli")
	build.Dir = "../.."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, out)
	}
	resetMCPPathEnv(t)
	for _, key := range []string{"ETSY_API_KEY", "ETSY_SHARED_SECRET", "ETSY_SHOP_ID", "ETSY_OAUTH2", "ETSY_CONFIG", "ETSY_MCP_READ_ONLY", "ETSY_VERIFY", "ETSY_DOGFOOD"} {
		t.Setenv(key, "")
	}
	var reads, writes atomic.Int64
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			writes.Add(1)
		} else {
			reads.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"listing_id":1000000001,"title":"Synthetic listing","description":"Synthetic","price":{"amount":1200,"divisor":100,"currency_code":"USD"},"quantity":1,"who_made":"i_did","when_made":"2020_2026","taxonomy_id":1}`))
	}))
	defer fake.Close()
	t.Setenv("ETSY_BASE_URL", fake.URL)
	// Synthetic credentials ensure a forwarding regression reaches the HTTP seam.
	t.Setenv("ETSY_API_KEY", "synthetic-key:synthetic-secret")
	t.Setenv("ETSY_OAUTH2", "synthetic-token")
	t.Setenv("ETSY_SHOP_ID", "9000001")
	t.Setenv("ETSY_CLI_PATH", bin)
	cobratree.ResetSiblingCLIPathCache()
	t.Cleanup(cobratree.ResetSiblingCLIPathCache)
	t.Setenv(cliutil.MCPReadOnlyEnvVar, "")
	s := server.NewMCPServer("etsy", "test")
	RegisterTools(s)
	for _, tc := range []struct {
		name string
		args map[string]any
	}{
		{"listings_bulk_edit", map[string]any{"field": "title", "set": []any{"Synthetic new title"}, "listings": "1000000001", "apply": true, "yes": true}},
		{"listings_copy", map[string]any{"listing_id": "1000000001", "yes": true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tool := s.ListTools()[tc.name]
			if tool == nil {
				t.Fatal("missing tool")
			}
			if _, ok := tool.Tool.InputSchema.Properties["yes"]; ok {
				t.Fatal("MCP exposes yes")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			result, err := tool.Handler(ctx, mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: tc.args}})
			if err != nil {
				t.Fatal(err)
			}
			text := mcpTextContent(t, result)
			if writes.Load() != 0 {
				t.Fatalf("MCP wrote %d requests", writes.Load())
			}
			if !result.IsError && !strings.Contains(text, "preview") && !strings.Contains(text, "dry_run") {
				t.Fatalf("expected preview/refusal: %s", text)
			}
			// Explicit yes must be rejected before any execution. This catches a
			// mutation that forwards yes even if the write gate later refuses it.
			if !result.IsError || !strings.Contains(text, "yes") {
				t.Fatalf("yes was not refused: %s", text)
			}
		})
	}
	if reads.Load() != 0 {
		t.Fatalf("invalid yes should be refused before HTTP: %d reads", reads.Load())
	}
	// Positive control: the same registered copy handler can execute a real
	// companion CLI and return a read-only preview against the synthetic server.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	result, err := s.ListTools()["listings_copy"].Handler(ctx, mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: map[string]any{"listing_id": "1000000001", "dry-run": true}}})
	if err != nil {
		t.Fatal(err)
	}
	text := mcpTextContent(t, result)
	if result.IsError || !strings.Contains(text, "preview") || !strings.Contains(text, "Synthetic listing") {
		t.Fatalf("copy preview control failed: %s", text)
	}
	if reads.Load() != 1 || writes.Load() != 0 {
		t.Fatalf("preview requests: reads=%d writes=%d", reads.Load(), writes.Load())
	}
}
