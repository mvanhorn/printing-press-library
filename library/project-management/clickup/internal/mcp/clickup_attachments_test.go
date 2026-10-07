// Hand-authored tests for clickup_attachments.go (PATCH multipart-attachment-upload).

package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestAttachmentToolsRegistered(t *testing.T) {
	s := server.NewMCPServer("test", "0")
	RegisterTools(s)
	tools := s.ListTools()

	// Novel commands are exposed as cobratree shell-out tools.
	for _, name := range []string{"task_attach", "task_attachments"} {
		if tools[name] == nil {
			t.Errorf("missing shell-out tool %q", name)
		}
	}
	if tl := tools["task_attachments"]; tl != nil {
		if ro := tl.Tool.Annotations.ReadOnlyHint; ro == nil || !*ro {
			t.Errorf("task_attachments should be read-only")
		}
	}

	// Typed upload tools must take a local file_path, not a JSON attachment body.
	for _, name := range []string{"task_attachment_create-task", "workspaces_attachments_post-entity"} {
		tl := tools[name]
		if tl == nil {
			t.Errorf("missing typed tool %q", name)
			continue
		}
		if _, ok := tl.Tool.InputSchema.Properties["file_path"]; !ok {
			t.Errorf("%s: no file_path param", name)
		}
		if _, ok := tl.Tool.InputSchema.Properties["attachment"]; ok {
			t.Errorf("%s: still exposes the JSON attachment body param", name)
		}
		required := strings.Join(tl.Tool.InputSchema.Required, ",")
		if !strings.Contains(required, "file_path") {
			t.Errorf("%s: file_path not required (required=%s)", name, required)
		}
	}
}

func TestMultipartUploadHandlerRejectsMissingFile(t *testing.T) {
	h := makeMultipartUploadHandler(multipartUploadSpec{PathTemplate: "/v2/task/{task_id}/attachment", PathParams: []string{"task_id"}})
	call := func(args map[string]any) *mcplib.CallToolResult {
		req := mcplib.CallToolRequest{}
		req.Params.Arguments = args
		res, err := h(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	if res := call(map[string]any{"task_id": "x"}); !res.IsError {
		t.Error("expected error without file_path")
	}
	if res := call(map[string]any{"task_id": "x", "file_path": filepath.Join(t.TempDir(), "missing.pdf")}); !res.IsError {
		t.Error("expected error for missing file")
	}
	p := filepath.Join(t.TempDir(), "a.txt")
	os.WriteFile(p, []byte("a"), 0o644)
	if res := call(map[string]any{"file_path": p}); !res.IsError {
		t.Error("expected error without task_id")
	}
}

func TestEscapePathSegment(t *testing.T) {
	for in, want := range map[string]string{"abc123xyz": "abc123xyz", "PROJ-123": "PROJ-123", "a/b": "a%2Fb", "a b": "a%20b", "..": "%2E%2E"} {
		if got := escapePathSegment(in); got != want {
			t.Errorf("escapePathSegment(%q) = %q, want %q", in, got, want)
		}
	}
}
