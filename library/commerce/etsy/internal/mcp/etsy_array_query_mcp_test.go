package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cli"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/mcp/cobratree"
)

// TestCobratreeShellOutListingsGetByIdsCommaJoins exercises the cobratree
// companion-CLI shell-out via a test-only mirror registration. The production
// etsy_read/etsy_execute route is covered by the code_orch tests.
func TestCobratreeShellOutListingsGetByIdsCommaJoins(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "etsy-pp-cli")
	build := exec.Command("go", "build", "-o", bin, "./cmd/etsy-pp-cli")
	build.Dir = "../.."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, out)
	}
	resetMCPPathEnv(t)
	for key, value := range map[string]string{
		"ETSY_API_KEY": "synthetic-key:synthetic-secret", "ETSY_SHARED_SECRET": "",
		"ETSY_OAUTH2": "synthetic-token", "ETSY_SHOP_ID": "9000001",
		"ETSY_MCP_READ_ONLY": "", "ETSY_VERIFY": "", "ETSY_DOGFOOD": "", "ETSY_NO_LEARN": "true",
	} {
		t.Setenv(key, value)
	}
	queries := make(chan string, 8)
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/listings/batch" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		queries <- r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"count":0,"results":[]}`))
	}))
	defer fake.Close()
	t.Setenv("ETSY_BASE_URL", fake.URL)
	t.Setenv("ETSY_CLI_PATH", bin)
	cobratree.ResetSiblingCLIPathCache()
	t.Cleanup(cobratree.ResetSiblingCLIPathCache)
	s := server.NewMCPServer("etsy", "test")
	RegisterTools(s)
	// Register a test-only mirror to exercise the cobratree companion-CLI
	// shell-out seam; production etsy_read/etsy_execute does not use this route.
	root := cli.RootCmd()
	command, _, err := root.Find([]string{"listings", "get-by-ids"})
	if err != nil {
		t.Fatal(err)
	}
	delete(command.Annotations, cobratree.EndpointAnnotation)
	cobratree.RegisterAll(s, root, cobratree.CachedSiblingCLIPath)
	tool := s.ListTools()["listings_get_by_ids"]
	if tool == nil {
		t.Fatal("missing listings_get_by_ids")
	}
	if _, ok := tool.Tool.InputSchema.Properties["listing-ids"]; !ok {
		t.Fatalf("missing listing_ids schema: %+v", tool.Tool.InputSchema)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	result, err := tool.Handler(ctx, mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: map[string]any{
		"listing-ids": "1000000001,1000000002,1000000003",
		"data-source": "live", "json": true, "no-cache": true,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("MCP command: %s", mcpTextContent(t, result))
	}
	select {
	case raw := <-queries:
		parsed, err := url.ParseQuery(raw)
		if err != nil {
			t.Fatal(err)
		}
		values := parsed["listing_ids"]
		if len(values) != 1 || values[0] != "1000000001,1000000002,1000000003" || strings.Count(raw, "listing_ids=") != 1 {
			t.Fatalf("ARRAYQ_MCP_VALUE: %s", raw)
		}
	default:
		t.Fatal("ARRAYQ_MCP_NO_REQUEST")
	}
}
