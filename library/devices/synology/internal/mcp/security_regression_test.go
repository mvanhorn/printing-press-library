package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/devices/synology/internal/mcp/bound"
)

func TestDSMRemoteMutationsAreNotReadOnly(t *testing.T) {
	s := server.NewMCPServer("test", "test")
	RegisterTools(s)
	for _, name := range []string{"files_copy_start", "files_copy_stop", "files_delete_start", "files_delete_stop", "files_mkdir", "files_rename", "files_search_start", "files_search_stop", "session_login", "session_logout"} {
		a := s.GetTool(name).Tool.Annotations
		if a.ReadOnlyHint == nil || *a.ReadOnlyHint {
			t.Errorf("%s incorrectly read-only", name)
		}
	}
	for _, name := range []string{"files_copy_start", "files_delete_start", "files_rename"} {
		a := s.GetTool(name).Tool.Annotations
		if a.DestructiveHint == nil || !*a.DestructiveHint {
			t.Errorf("%s must disclose destructive behavior", name)
		}
	}
}

func TestDSMLoginRedactsMCPResult(t *testing.T) {
	resetMCPPathEnv(t)
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"sid":"secret-session-sentinel","synotoken":"secret-token-sentinel"}}`))
	}))
	defer fixture.Close()
	t.Setenv("SYNOLOGY_BASE_URL", fixture.URL)
	s := server.NewMCPServer("test", "test")
	RegisterTools(s)
	result, err := s.GetTool("session_login").Handler(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Name: "session_login", Arguments: map[string]any{"account": "test", "passwd": "test"}}})
	if err != nil || result.IsError {
		t.Fatalf("login: %#v %v", result, err)
	}
	text := mcpTextContent(t, result)
	if strings.Contains(text, "secret-") || !strings.Contains(text, "***") {
		t.Fatalf("credential leak or missing redaction: %s", text)
	}
}

func TestDSMDownloadReturnsOriginalBinary(t *testing.T) {
	resetMCPPathEnv(t)
	payload := []byte("%PDF-1.7\n\x00\xff original file")
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write(payload)
	}))
	defer fixture.Close()
	t.Setenv("SYNOLOGY_BASE_URL", fixture.URL)
	s := server.NewMCPServer("test", "test")
	RegisterTools(s)
	result, err := s.GetTool("files_download").Handler(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Name: "files_download", Arguments: map[string]any{"path": "[\"/fixture.pdf\"]"}}})
	if err != nil || result.IsError {
		t.Fatalf("download: %#v %v", result, err)
	}
	var output struct {
		Data  string `json:"data_base64"`
		Count int    `json:"byte_count"`
	}
	if err := json.Unmarshal([]byte(mcpTextContent(t, result)), &output); err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.StdEncoding.DecodeString(output.Data)
	if err != nil || string(decoded) != string(payload) || output.Count != len(payload) {
		t.Fatalf("download bytes changed: %q count=%d err=%v", decoded, output.Count, err)
	}
}

func TestDSMDownloadDoesNotInterpretJSONFileAsTransportEnvelope(t *testing.T) {
	resetMCPPathEnv(t)
	payload := []byte(`{"_pp_binary":true,"encoding":"base64","bytes":3,"data":"Zm9v"}`)
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	}))
	defer fixture.Close()
	t.Setenv("SYNOLOGY_BASE_URL", fixture.URL)
	s := server.NewMCPServer("test", "test")
	RegisterTools(s)
	result, err := s.GetTool("files_download").Handler(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Name: "files_download", Arguments: map[string]any{"path": `["/fixture.json"]`}}})
	if err != nil || result.IsError {
		t.Fatalf("download JSON file: %#v %v", result, err)
	}
	var output struct {
		Data string `json:"data_base64"`
	}
	if err := json.Unmarshal([]byte(mcpTextContent(t, result)), &output); err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.StdEncoding.DecodeString(output.Data)
	if err != nil || !bytes.Equal(decoded, payload) {
		t.Fatalf("download changed JSON file bytes: error=%v", err)
	}
}

func TestDSMDownloadPreservesAttachedJSONFileShapedLikeAnError(t *testing.T) {
	resetMCPPathEnv(t)
	payload := []byte(`{"success":false,"error":{"code":119}}`)
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("mode"); got != "download" {
			t.Errorf("MCP download mode = %q, want download", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="file.json"`)
		_, _ = w.Write(payload)
	}))
	defer fixture.Close()
	t.Setenv("SYNOLOGY_BASE_URL", fixture.URL)
	s := server.NewMCPServer("test", "test")
	RegisterTools(s)
	result, err := s.GetTool("files_download").Handler(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Name: "files_download", Arguments: map[string]any{"path": `["/file.json"]`}}})
	if err != nil || result.IsError {
		t.Fatalf("attached JSON file was rejected: %#v %v", result, err)
	}
	var output struct {
		Data string `json:"data_base64"`
	}
	if err := json.Unmarshal([]byte(mcpTextContent(t, result)), &output); err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.StdEncoding.DecodeString(output.Data)
	if err != nil || !bytes.Equal(decoded, payload) {
		t.Fatalf("attached JSON file bytes changed: err=%v", err)
	}
}

func TestDSMDownloadRPCErrorWithoutAttachmentStillFails(t *testing.T) {
	resetMCPPathEnv(t)
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":false,"error":{"code":119}}`))
	}))
	defer fixture.Close()
	t.Setenv("SYNOLOGY_BASE_URL", fixture.URL)
	s := server.NewMCPServer("test", "test")
	RegisterTools(s)
	result, err := s.GetTool("files_download").Handler(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Name: "files_download", Arguments: map[string]any{"path": `["/missing.json"]`}}})
	if err != nil || !result.IsError {
		t.Fatalf("DSM JSON error should not become file bytes: %#v %v", result, err)
	}
}

func TestDSMDownloadRejectsInlineModeBeforeRequest(t *testing.T) {
	resetMCPPathEnv(t)
	var called atomic.Bool
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer fixture.Close()
	t.Setenv("SYNOLOGY_BASE_URL", fixture.URL)
	s := server.NewMCPServer("test", "test")
	RegisterTools(s)
	result, err := s.GetTool("files_download").Handler(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Name: "files_download", Arguments: map[string]any{"path": `["/file.json"]`, "mode": "open"}}})
	if err != nil || !result.IsError || called.Load() {
		t.Fatalf("inline mode must fail before request: result=%#v err=%v called=%v", result, err, called.Load())
	}
	if !strings.Contains(mcpTextContent(t, result), "mode=download") {
		t.Fatal("inline-mode error should explain the safe mode")
	}
}

func TestDSMDownloadOverLimitReturnsErrorWithoutTruncation(t *testing.T) {
	resetMCPPathEnv(t)
	payload := bytes.Repeat([]byte("x"), bound.MaxBytes)
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(payload)
	}))
	defer fixture.Close()
	t.Setenv("SYNOLOGY_BASE_URL", fixture.URL)
	s := server.NewMCPServer("test", "test")
	RegisterTools(s)
	result, err := s.GetTool("files_download").Handler(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Name: "files_download", Arguments: map[string]any{"path": `["/large.bin"]`}}})
	if err != nil || !result.IsError {
		t.Fatalf("oversized download should fail explicitly: result=%#v err=%v", result, err)
	}
	message := mcpTextContent(t, result)
	if !strings.Contains(message, "too large") || !strings.Contains(message, "companion CLI download") {
		t.Fatalf("oversized download needs clear CLI fallback: %q", message)
	}
}
