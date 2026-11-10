// PATCH(redirect-and-redaction-policy): MCP path parameters are escaped like the CLI's replacePathParam; see .printing-press-patches.json.

package mcp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
)

func TestMCPPathSegmentEscapesSlashAndQuestionMark(t *testing.T) {
	if got := mcpPathSegment("a/b?c=d e"); got != "a%2Fb%3Fc=d%20e" {
		t.Fatalf("got %q", got)
	}
}

// A code containing "/" and "?" must stay ONE path segment on the wire, and
// the redaction must still recognise and hide it.
func TestCodeOrchExecuteEscapesPathParamsAndMasksThem(t *testing.T) {
	var mu sync.Mutex
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotPath, gotQuery = r.URL.EscapedPath(), r.URL.RawQuery
		mu.Unlock()
		w.WriteHeader(404)
		_, _ = io.WriteString(w, `{"error":"unknown code `+r.URL.Path+`"}`)
	}))
	defer srv.Close()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL+"/api/v1")
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	req := mcplib.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"endpoint_id": "public.post-screen-share-code",
		"params":      map[string]any{"code": "SECRET/part?x=1-ABC123456"},
	}
	res, err := codeOrchTestInvoke(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if gotPath != "/api/v1/public/screen-share/SECRET%2Fpart%3Fx=1-ABC123456" || gotQuery != "" {
		t.Fatalf("wire path=%q query=%q, want the code as a single escaped segment", gotPath, gotQuery)
	}
	text := ""
	for _, c := range res.Content {
		if tc, ok := c.(mcplib.TextContent); ok {
			text += tc.Text
		}
	}
	if strings.Contains(text, "ABC123456") {
		t.Fatalf("MCP result leaked the code: %s", text)
	}
}
