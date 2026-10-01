package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
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
