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
	"github.com/mvanhorn/printing-press-library/library/devices/kvmctl/internal/cli"
	"github.com/mvanhorn/printing-press-library/library/devices/kvmctl/internal/semantic"
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
			description := entry.Tool.Description
			if !strings.Contains(description, "Disabled:") || !strings.Contains(description, "target- and operation-bound authorization") {
				t.Fatalf("blocked tool description implies direct access: %q", description)
			}
			if strings.HasPrefix(name, "hid_send-") {
				if !strings.Contains(description, "authorized sequence or workflow") {
					t.Fatalf("missing supported alternative: %q", description)
				}
			} else if !strings.Contains(description, "No authorized MCP alternative exists") {
				t.Fatalf("missing unsupported operation guidance: %q", description)
			}
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

func TestSemanticMCPWritesNeverReachDevice(t *testing.T) {
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
	entry := s.GetTool("semantic_dispatch")
	if entry == nil {
		t.Fatal("semantic_dispatch not registered")
	}
	for _, operation := range semantic.Operations {
		if semanticMCPReadAllowed(operation) {
			continue
		}
		t.Run(operation, func(t *testing.T) {
			result, err := entry.Handler(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Name: "semantic_dispatch", Arguments: map[string]any{"operation": operation, "arguments": map[string]any{"write_enabled": true, "target": api.URL, "approved": true, "approval_token": "unbound-token"}}}})
			if err != nil || result == nil || !result.IsError {
				t.Fatalf("semantic physical write accepted: %#v %v", result, err)
			}
			message := result.Content[0].(mcplib.TextContent).Text
			if !strings.Contains(message, "target- and operation-bound authorization") {
				t.Fatalf("missing authorization guidance: %s", message)
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatalf("unauthorized semantic requests reached device: %d", calls.Load())
	}
}

func TestDirectPhysicalCobraMirrorsAreAbsent(t *testing.T) {
	root := cli.RootCmd()
	for _, names := range physicalCobraMirrorPaths {
		cmd := root
		for _, name := range names {
			var childFound bool
			for _, child := range cmd.Commands() {
				if child.Name() == name {
					cmd = child
					childFound = true
					break
				}
			}
			if !childFound {
				t.Fatalf("expected physical CLI command is missing: %s", strings.Join(names, " "))
			}
		}
	}
	s := server.NewMCPServer("test", "test")
	RegisterTools(s)
	for _, name := range []string{
		"keyboard_key", "keyboard_text", "keyboard_chord", "keyboard_hold", "keyboard_release_all",
		"mouse_move", "mouse_button", "mouse_wheel", "target_switch", "machines_select",
		"act_click_text", "act_press_key", "semantic_click_text", "semantic_press_key",
		"semantic_kvm_send_keys", "semantic_kvm_mouse_click", "semantic_rearm_otg",
	} {
		if s.GetTool(name) != nil {
			t.Errorf("direct physical Cobra mirror is exposed: %s", name)
		}
	}
	for _, operation := range semantic.Operations {
		name := "semantic_" + strings.NewReplacer(".", "_", "-", "_").Replace(operation)
		if s.GetTool(name) != nil {
			t.Errorf("semantic Cobra mirror is exposed: %s", name)
		}
	}
	for _, name := range []string{"sequence_run", "workflow_execute", "machines_list", "observe", "verify"} {
		if s.GetTool(name) == nil {
			t.Errorf("safe or authorized Cobra mirror was removed: %s", name)
		}
	}
}
