package mcp

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/n8n/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/n8n/internal/mcp/bound"

	mcplib "github.com/mark3labs/mcp-go/mcp"
)

// An n8n list larger than the MCP budget must hand back a cursor that resumes
// inside the truncated page, not the server's nextCursor (which skips rows).
func TestCodeOrchPagingKeepsTruncatedRows(t *testing.T) {
	rows := make([]string, 0, 400)
	for i := 0; i < 400; i++ {
		rows = append(rows, fmt.Sprintf(`{"id":"w%03d","name":"%s"}`, i, strings.Repeat("x", 200)))
	}
	data := json.RawMessage(`{"data":[` + strings.Join(rows, ",") + `],"nextCursor":"SERVERNEXT"}`)
	text := bound.EndpointPageResponse("GET", data, bound.PageOptions{CursorParam: "cursor", NextCursorPath: "nextCursor"})
	if len(text) > bound.MaxBytes {
		t.Fatalf("result exceeds budget: %d", len(text))
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	next, _ := out["next_cursor"].(string)
	if next == "" {
		next, _ = out["nextCursor"].(string)
	}
	if next == "" || next == "SERVERNEXT" {
		t.Fatalf("truncated page must resume locally, got cursor %q in %v", next, keys(out))
	}
	if up, err := bound.UpstreamCursor(next); err != nil || up == "SERVERNEXT" {
		t.Fatalf("cursor should resume the current page first: upstream=%q err=%v", up, err)
	}
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestCodeOrchUploadsPackageAsMultipart(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("N8N_MCP_UPLOAD_ROOT", "")
	var hits int
	var gotType, gotPart, gotField, gotName string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		gotType = r.Header.Get("Content-Type")
		if err := r.ParseMultipartForm(1 << 20); err == nil {
			gotField = r.FormValue("projectId")
			if f, hdr, err := r.FormFile("package"); err == nil {
				b, _ := io.ReadAll(f)
				gotPart = string(b)
				gotName = hdr.Filename
				_ = f.Close()
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("N8N_BASE_URL", srv.URL)
	t.Setenv("N8N_API_KEY", "test")

	enc := base64.StdEncoding.EncodeToString([]byte("PKGBYTES"))
	var req mcplib.CallToolRequest
	req.Params.Arguments = map[string]any{"endpoint_id": "n8n-packages.import-package", "confirm": true,
		"params": map[string]any{"package": enc, "package_filename": "p.n8np", "projectId": "p1"}}
	res, err := codeOrchInvoke(t.Context(), req, true)
	if err != nil || res == nil || res.IsError {
		t.Fatalf("upload failed: %+v err=%v", res, err)
	}
	if hits != 1 || !strings.HasPrefix(gotType, "multipart/form-data") || gotPart != "PKGBYTES" || gotField != "p1" || gotName != "p.n8np" {
		t.Fatalf("server saw hits=%d type=%q part=%q name=%q projectId=%q", hits, gotType, gotPart, gotName, gotField)
	}

	pkg := filepath.Join(t.TempDir(), "p.n8np")
	if err := os.WriteFile(pkg, []byte("PKGBYTES"), 0o600); err != nil {
		t.Fatal(err)
	}
	req.Params.Arguments = map[string]any{"endpoint_id": "n8n-packages.import-package", "confirm": true,
		"params": map[string]any{"package": pkg, "projectId": "p1"}}
	res, err = codeOrchInvoke(t.Context(), req, true)
	if err != nil {
		t.Fatalf("path upload returned error value: %v", err)
	}
	if res == nil || !res.IsError || hits != 1 {
		t.Fatalf("plain path should be rejected without a server call, hits=%d res=%+v", hits, res)
	}
	msg := fmt.Sprint(res.Content)
	if !strings.Contains(msg, "base64") || !strings.Contains(msg, "N8N_MCP_UPLOAD_ROOT") {
		t.Fatalf("path rejection should name base64 and the upload root, got %s", msg)
	}

	req.Params.Arguments = map[string]any{"endpoint_id": "n8n-packages.import-package", "confirm": true, "params": map[string]any{}}
	res, _ = codeOrchInvoke(t.Context(), req, true)
	if res == nil || !res.IsError || !strings.Contains(fmt.Sprint(res.Content), "params.package") || hits != 1 {
		t.Fatalf("missing package should be a tool error, hits=%d got %+v", hits, res)
	}
}

func TestCodeOrchPagingKeepsRequestCursor(t *testing.T) {
	rows := make([]string, 0, 400)
	for i := 0; i < 400; i++ {
		rows = append(rows, fmt.Sprintf(`{"id":"w%03d","name":"%s"}`, i, strings.Repeat("x", 200)))
	}
	data := json.RawMessage(`{"data":[` + strings.Join(rows, ",") + `],"nextCursor":null}`)
	text := bound.EndpointPageResponse("GET", data, bound.PageOptions{CursorParam: "cursor", NextCursorPath: "nextCursor", RequestCursor: "RAWPAGE2"})
	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatal(err)
	}
	next, _ := out["next_cursor"].(string)
	if up, err := bound.UpstreamCursor(next); err != nil || up != "RAWPAGE2" {
		t.Fatalf("continuation should re-request RAWPAGE2, got upstream=%q err=%v", up, err)
	}
}
