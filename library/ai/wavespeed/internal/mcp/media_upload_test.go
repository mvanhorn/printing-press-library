// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

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

// The typed upload tool must never read a caller-named server path.
func TestTypedMediaUploadRefusesWithoutReadingFiles(t *testing.T) {
	s := server.NewMCPServer("wavespeed", "test")
	RegisterTools(s)
	tool, ok := s.ListTools()["media_uploads_upload-media-binary"]
	if !ok {
		t.Fatal("typed upload tool not registered")
	}
	if len(tool.Tool.InputSchema.Properties) != 0 {
		t.Fatalf("typed upload tool advertises inputs: %v", tool.Tool.InputSchema.Properties)
	}
	secret := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(secret, []byte("do-not-send"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Point the client at an unroutable host: any network attempt would
	// surface as a transport error rather than the refusal message.
	t.Setenv("WAVESPEED_BASE_URL", "http://127.0.0.1:9/api/v3")
	req := mcplib.CallToolRequest{}
	req.Params.Name = "media_uploads_upload-media-binary"
	req.Params.Arguments = map[string]any{"file": secret}
	res, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatalf("typed upload succeeded; want a refusal")
	}
	text := ""
	for _, c := range res.Content {
		if tc, ok := c.(mcplib.TextContent); ok {
			text += tc.Text
		}
	}
	if want := "unsupported"; !containsFold(text, want) {
		t.Fatalf("refusal text %q does not say %q", text, want)
	}
}

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}
