// Copyright 2026 Paul Taramona and contributors. Licensed under Apache-2.0. See LICENSE.
package main

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	mcptools "github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/mcp"
)

// PATCH(novel1219-mcp-http-upload-cap): Greptile PR #2450 thread 4514749152. The MCP tools accept one multipart file
// of up to 100 MiB as base64 (about 134 MiB of text), so the HTTP transport must let a tools/call that carries a
// maximum supported upload through with the production limits.

const novel1219MaxUploadDecodedBytes = 100 << 20 // internal/mcp mcpMaxUploadBytes

// repeatByteReader yields n copies of b without allocating them.
type repeatByteReader struct {
	b byte
	n int64
}

func (r *repeatByteReader) Read(p []byte) (int, error) {
	if r.n <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > r.n {
		p = p[:r.n]
	}
	for i := range p {
		p[i] = r.b
	}
	r.n -= int64(len(p))
	return len(p), nil
}

func novel1219ToolCallBody(base64Len int64) (io.Reader, int64) {
	prefix := `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"conduyt-crm_execute","arguments":{"endpoint_id":"imports.upload","confirm":true,"params":{"file_filename":"contacts.csv","file":"`
	suffix := `"}}}}`
	return io.MultiReader(strings.NewReader(prefix), &repeatByteReader{b: 'A', n: base64Len}, strings.NewReader(suffix)),
		int64(len(prefix)) + base64Len + int64(len(suffix))
}

func TestNovel1219MCPHTTPMaxSupportedUploadPasses(t *testing.T) {
	var called atomic.Bool
	var got atomic.Int64
	handler := requireBearerAuth("secret", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)
		n, _ := io.Copy(io.Discard, r.Body)
		got.Store(n)
		w.WriteHeader(http.StatusNoContent)
	}))
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	body, size := novel1219ToolCallBody(int64(base64.StdEncoding.EncodedLen(novel1219MaxUploadDecodedBytes)))
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/mcp", body)
	if err != nil {
		t.Fatal(err)
	}
	req.ContentLength = size
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST a maximum supported upload: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusNoContent || !called.Load() {
		t.Fatalf("a tools/call carrying a 100 MiB upload (%d bytes) got status %d, handler called %v", size, resp.StatusCode, called.Load())
	}
	if got.Load() != size {
		t.Fatalf("handler read %d bytes, want the whole %d-byte request", got.Load(), size)
	}
}

func TestNovel1219MCPHTTPToolCallCapCoversMaxUpload(t *testing.T) {
	if mcpHTTPMaxBodyBytes != 4<<20 {
		t.Fatalf("general body cap = %d, want 4 MiB", mcpHTTPMaxBodyBytes)
	}
	need := int64(base64.StdEncoding.EncodedLen(novel1219MaxUploadDecodedBytes)) + 4096
	if mcptools.MaxUploadBase64Bytes != need {
		t.Fatalf("MaxUploadBase64Bytes = %d, want %d (padded base64 of 100 MiB plus 4096 slack)", mcptools.MaxUploadBase64Bytes, need)
	}
	if mcpHTTPMaxToolCallBodyBytes != need+mcpHTTPMaxBodyBytes {
		t.Fatalf("tools/call cap = %d, want the upload payload %d plus a 4 MiB envelope", mcpHTTPMaxToolCallBodyBytes, need)
	}
}

func novel1219Post(t *testing.T, handler http.Handler, body io.Reader, size int64) int {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/mcp", body)
	if err != nil {
		t.Fatal(err)
	}
	req.ContentLength = size
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// The server may refuse and close before the client finishes writing an oversize body.
		return http.StatusRequestEntityTooLarge
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

func TestNovel1219MCPHTTPNonToolCallKeepsTheGeneralBound(t *testing.T) {
	var called atomic.Bool
	handler := requireBearerAuth("secret", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusNoContent)
	}))
	prefix := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"pad":"`
	suffix := `"}}`
	pad := int64(mcpHTTPMaxBodyBytes) - int64(len(prefix)+len(suffix)) + 1
	body := io.MultiReader(strings.NewReader(prefix), &repeatByteReader{b: 'A', n: pad}, strings.NewReader(suffix))
	if got := novel1219Post(t, handler, body, mcpHTTPMaxBodyBytes+1); got != http.StatusRequestEntityTooLarge || called.Load() {
		t.Fatalf("a tools/list one byte over 4 MiB: status %d, handler called %v; want 413 and no call", got, called.Load())
	}
}

func TestNovel1219MCPHTTPBodyLimitEdges(t *testing.T) {
	const small, toolCall = 256, 1024
	toolCallOf := func(total int) string {
		prefix := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"x","arguments":{"file":"`
		suffix := `"}}}`
		return prefix + strings.Repeat("A", total-len(prefix)-len(suffix)) + suffix
	}
	otherOf := func(method string, total int) string {
		prefix := `{"jsonrpc":"2.0","id":1,"method":"` + method + `","params":{"pad":"`
		suffix := `"}}`
		return prefix + strings.Repeat("A", total-len(prefix)-len(suffix)) + suffix
	}
	cases := []struct {
		name       string
		body       string
		want       int
		wantCalled bool
	}{
		{"tools/call at the tools/call cap", toolCallOf(toolCall), http.StatusNoContent, true},
		{"tools/call one byte over the tools/call cap", toolCallOf(toolCall + 1), http.StatusRequestEntityTooLarge, false},
		{"tools/list at the general cap", otherOf("tools/list", small), http.StatusNoContent, true},
		{"tools/list one byte over the general cap", otherOf("tools/list", small+1), http.StatusRequestEntityTooLarge, false},
		{"initialize over the general cap", otherOf("initialize", small+100), http.StatusRequestEntityTooLarge, false},
		{"batch with a tools/call over the general cap", "[" + toolCallOf(small+100) + "]", http.StatusRequestEntityTooLarge, false},
		{"malformed JSON over the general cap", strings.Repeat("A", small+1), http.StatusRequestEntityTooLarge, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var called atomic.Bool
			var got atomic.Int64
			handler := requireBearerAuthWithBodyLimits("secret", nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called.Store(true)
				n, _ := io.Copy(io.Discard, r.Body)
				got.Store(n)
				w.WriteHeader(http.StatusNoContent)
			}), small, toolCall, 5*time.Second)
			// Unknown length (chunked) so the cap is enforced on the bytes read, not a declared header.
			status := novel1219Post(t, handler, strings.NewReader(tc.body), -1)
			if status != tc.want || called.Load() != tc.wantCalled {
				t.Fatalf("%d-byte body: status %d, handler called %v; want %d, %v", len(tc.body), status, called.Load(), tc.want, tc.wantCalled)
			}
			if tc.wantCalled && got.Load() != int64(len(tc.body)) {
				t.Fatalf("handler read %d bytes, want %d", got.Load(), len(tc.body))
			}
		})
	}
}
