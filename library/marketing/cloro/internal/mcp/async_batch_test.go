// Copyright 2026 Ricardo Batista and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// The batch endpoint takes a JSON array as its request body. The MCP
// async_batch tool must send that array unchanged.
func TestMCPAsyncBatchSubmitsJSONArrayBody(t *testing.T) {
	resetMCPPathEnv(t)
	var gotPath string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"success":true,"results":[{"index":0,"success":true,"taskId":"t-1"}]}`)
	}))
	defer srv.Close()
	t.Setenv("CLORO_BASE_URL", srv.URL)
	t.Setenv("CLORO_API_KEY", "test-key")

	handler := asyncBatchHandler(t)
	batch := `[{"taskType":"CHATGPT","payload":{"prompt":"What do you know about Acme Corp?","country":"US"}}]`
	result := callAsyncBatch(t, handler, batch)
	if result.IsError {
		t.Fatalf("async_batch with a JSON array returned an error: %s", mcpTextContent(t, result))
	}
	if gotPath != "/v1/async/task/batch" {
		t.Fatalf("request path = %q, want /v1/async/task/batch", gotPath)
	}
	var sent []map[string]any
	if err := json.Unmarshal(gotBody, &sent); err != nil {
		t.Fatalf("request body is not a JSON array: %v\n%s", err, gotBody)
	}
	if len(sent) != 1 || sent[0]["taskType"] != "CHATGPT" {
		t.Fatalf("request body = %s, want the submitted batch", gotBody)
	}
}

func TestMCPAsyncBatchRejectsNonArrayBody(t *testing.T) {
	resetMCPPathEnv(t)
	t.Setenv("CLORO_API_KEY", "test-key")
	handler := asyncBatchHandler(t)
	for _, body := range []string{`{"taskType":"CHATGPT"}`, `"text"`, `not json`} {
		result := callAsyncBatch(t, handler, body)
		if !result.IsError {
			t.Fatalf("async_batch accepted body %s, want an error", body)
		}
		if text := mcpTextContent(t, result); !strings.Contains(text, "JSON array") {
			t.Fatalf("error for body %s = %q, want it to ask for a JSON array", body, text)
		}
	}
}

func asyncBatchHandler(t *testing.T) server.ToolHandlerFunc {
	t.Helper()
	s := server.NewMCPServer("cloro", "test")
	RegisterTools(s)
	tool, ok := s.ListTools()["async_batch"]
	if !ok {
		t.Fatal("async_batch tool is not registered")
	}
	return tool.Handler
}

func callAsyncBatch(t *testing.T, handler server.ToolHandlerFunc, body string) *mcplib.CallToolResult {
	t.Helper()
	req := mcplib.CallToolRequest{}
	req.Params.Name = "async_batch"
	req.Params.Arguments = map[string]any{"body_json": body}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("async_batch handler error = %v", err)
	}
	return result
}
