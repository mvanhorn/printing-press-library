// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package mcp

import (
	"context"
	"encoding/json"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/travel/kurumatabi/internal/cliutil/testenv"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestActualExportMirrorAdvertisesAndUsesRequiredSourceResource(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "companion")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	build := exec.Command("go", "build", "-o", bin, "../../cmd/kurumatabi-pp-cli")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("companion build: %v %s", err, output)
	}
	testenv.Isolate(t)
	t.Setenv("KURUMATABI_CLI_PATH", bin)
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><title>Catalog</title><a href="/park/rvpark/1086.html">公園A</a><a href="/park/yypark/213.html">公園B</a></html>`))
	}))
	defer srv.Close()
	t.Setenv("KURUMATABI_BASE_URL", srv.URL)
	s := server.NewMCPServer("test", "0")
	RegisterTools(s)
	registered, ok := s.ListTools()["export"]
	if !ok {
		t.Fatal("export was not mirrored")
	}
	raw, _ := json.Marshal(registered.Tool)
	var tool map[string]any
	_ = json.Unmarshal(raw, &tool)
	schema := tool["inputSchema"].(map[string]any)
	properties := schema["properties"].(map[string]any)
	if _, ok := properties["resource"]; !ok {
		t.Fatal("advertised export cannot specify required resource")
	}
	required, _ := schema["required"].([]any)
	found := false
	for _, key := range required {
		if key == "resource" {
			found = true
		}
	}
	if !found {
		t.Fatal("required resource was not declared")
	}
	called, err := registered.Handler(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: map[string]any{"resource": "source", "format": "json", "limit": float64(1), "no-learn": true, "no-cache": true}}})
	if err != nil || called.IsError || len(called.Content) != 1 {
		t.Fatalf("advertised export failed: %v %+v", err, called)
	}
	var links []struct {
		URL string `json:"url"`
	}
	if err = json.Unmarshal([]byte(called.Content[0].(mcplib.TextContent).Text), &links); err != nil || len(links) != 1 || !strings.HasPrefix(links[0].URL, srv.URL+"/park/") || requests != 1 {
		t.Fatalf("export evidence lost: %+v requests=%d err=%v", links, requests, err)
	}
}
