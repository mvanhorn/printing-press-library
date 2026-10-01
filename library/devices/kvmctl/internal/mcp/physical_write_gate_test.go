package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestRawPhysicalWritesNeverReachDevice(t *testing.T) {
	var calls atomic.Int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer api.Close()
	t.Setenv("KVMCTL_HOME", t.TempDir())
	t.Setenv("KVMCTL_BASE_URL", api.URL)
	t.Setenv("KVMCTL_WRITE_ENABLED", "1")
	s := server.NewMCPServer("test", "test")
	RegisterTools(s)
	for _, name := range []string{"hid_reset", "hid_send-key", "hid_send-mouse-button", "hid_send-mouse-move", "hid_send-mouse-wheel", "hid_send-shortcut", "streamer_set-params", "system_set-otg-functions"} {
		t.Run(name, func(t *testing.T) {
			entry := s.GetTool(name)
			if entry.Tool.Annotations.DestructiveHint == nil || !*entry.Tool.Annotations.DestructiveHint {
				t.Fatal("missing physical mutation hint")
			}
			result, err := entry.Handler(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Name: name, Arguments: map[string]any{"key": "Enter", "state": true, "button": "left", "x": 0, "y": 0, "delta": 1, "keys": "Ctrl+Alt+Delete", "desired_fps": 30, "start_cdrom": true, "start_flash": true, "target": api.URL, "token": "unbound-token", "write_enabled": true}}})
			if err != nil || result == nil || !result.IsError {
				t.Fatalf("physical write accepted: %#v %v", result, err)
			}
			text := result.Content[0].(mcplib.TextContent).Text
			if !strings.Contains(text, "target- and operation-bound authorization") {
				t.Fatalf("missing actionable authorization error: %s", text)
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatalf("unauthorized requests reached device: %d", calls.Load())
	}
}

func TestDeviceReadsRemainAvailable(t *testing.T) {
	for _, path := range []string{"/api/hid", "/api/streamer/snapshot", "/api/info"} {
		if physicalWriteBlocked("GET", path) {
			t.Fatalf("read blocked: %s", path)
		}
	}
}
