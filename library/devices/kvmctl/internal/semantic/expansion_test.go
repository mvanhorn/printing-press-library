package semantic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/devices/kvmctl/internal/client"
	"github.com/mvanhorn/printing-press-library/library/devices/kvmctl/internal/config"
)

func mustClient(t *testing.T, url string) *client.Client {
	t.Helper()
	return client.New(&config.Config{BaseURL: url}, 0, 0)
}

func TestDispatchCoversAllToolSpecOps(t *testing.T) {
	expected := []string{
		"capabilities", "snapshot", "ocr", "verify",
		"host.identity.inspect", "host.graphics.inspect", "service.render_access.inspect",
		"host.reboot", "select", "hid_reset", "rearm_otg",
		"kvm_send_text", "kvm_send_keys", "kvm_hold_key", "kvm_release_all",
		"kvm_mouse_move", "kvm_mouse_move_pct", "kvm_mouse_click", "kvm_mouse_scroll",
		"kvm_status", "kvm_screenshot_to_file", "kvm_ocr_screenshot", "kvm_ocr_click",
		"exec_command",
		"kvm_sequence_plan", "kvm_sequence_authorize", "kvm_sequence_execute",
		"kvm_workflow_authorize", "kvm_workflow_list", "kvm_workflow_inspect", "kvm_workflow_execute",
	}
	for _, name := range expected {
		found := false
		for _, op := range Operations {
			if op == name {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("operation %q not registered", name)
		}
	}
	if len(Operations) < len(expected) {
		t.Fatalf("Operations has %d, want >= %d", len(Operations), len(expected))
	}
}

func TestDispatchEvidenceEnvelopeHasRequiredFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "snapshot") {
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write([]byte{0xFF, 0xD8, 0xFF})
			return
		}
		_, _ = w.Write([]byte(`{"result":{"ok":true,"platform":{"model":"GL-iNet"},"system":{"kvmd_version":"4.82"}}}`))
	}))
	defer srv.Close()
	c := mustClient(t, srv.URL)
	for _, name := range []string{"capabilities", "snapshot", "kvm_status"} {
		got, err := Dispatch(context.Background(), c, name, nil)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var out map[string]any
		if err := json.Unmarshal(got, &out); err != nil {
			t.Fatalf("%s unmarshal: %v", name, err)
		}
		for _, field := range []string{"operation", "transport", "read_only", "ok", "evidence", "state"} {
			if _, ok := out[field]; !ok {
				t.Fatalf("%s missing field %s in %s", name, field, string(got))
			}
		}
	}
}

func TestDispatchWriteGating(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"ok":true}}`))
	}))
	defer srv.Close()
	c := mustClient(t, srv.URL)
	writeGated := []string{"hid_reset", "kvm_send_keys", "kvm_mouse_move", "select", "host.reboot", "exec_command", "kvm_sequence_authorize", "kvm_workflow_execute"}
	for _, name := range writeGated {
		if _, err := Dispatch(context.Background(), c, name, map[string]any{}); err == nil {
			t.Fatalf("write-gated %q accepted without write_enabled", name)
		}
		if _, err := Dispatch(context.Background(), c, name, map[string]any{"write_enabled": false}); err == nil {
			t.Fatalf("write-gated %q accepted with write_enabled=false", name)
		}
	}
}

func TestDispatchRedactsSecrets(t *testing.T) {
	c := mustClient(t, "http://127.0.0.1")
	got, err := Dispatch(context.Background(), c, "kvm_send_keys", map[string]any{"write_enabled": true, "combo": "Ctrl+A", "approval_token": "should-not-leak", "token": "also-secret"})
	if err != nil {
		t.Fatalf("redaction probe failed: %v", err)
	}
	var out map[string]any
	_ = json.Unmarshal(got, &out)
	ev, _ := out["evidence"].(map[string]any)
	if _, ok := ev["approval_token"]; ok {
		t.Fatal("secret leaked into evidence")
	}
	if _, ok := ev["token"]; ok {
		t.Fatal("secret leaked into evidence")
	}
}

func TestDispatchBoundsValidation(t *testing.T) {
	c := mustClient(t, "http://127.0.0.1")
	cases := []struct {
		op   string
		args map[string]any
	}{
		{"kvm_hold_key", map[string]any{"write_enabled": true, "key": "Shift", "duration_ms": 99999}},
		{"kvm_mouse_move", map[string]any{"write_enabled": true, "x": 99999, "y": 0}},
		{"kvm_mouse_move_pct", map[string]any{"write_enabled": true, "x_pct": 200, "y_pct": 0}},
		{"kvm_mouse_click", map[string]any{"write_enabled": true, "count": 99}},
		{"kvm_mouse_scroll", map[string]any{"write_enabled": true, "dx": 999}},
	}
	for _, tc := range cases {
		if _, err := Dispatch(context.Background(), c, tc.op, tc.args); err == nil {
			t.Fatalf("bounds not enforced for %s %#v", tc.op, tc.args)
		}
	}
}

func TestDispatchSnapshotPreviewWidthBounded(t *testing.T) {
	c := mustClient(t, "http://127.0.0.1")
	if _, err := Dispatch(context.Background(), c, "snapshot", map[string]any{"preview_max_width": -1}); err == nil {
		t.Fatal("negative preview_max_width accepted")
	}
}

func TestDispatchSelectRequiresMachine(t *testing.T) {
	c := mustClient(t, "http://127.0.0.1")
	if _, err := Dispatch(context.Background(), c, "select", map[string]any{"write_enabled": true}); err == nil {
		t.Fatal("select without machine accepted")
	}
}

func TestDispatchHostProbesReturnEnvelopeWithoutLiveHardware(t *testing.T) {
	c := mustClient(t, "http://127.0.0.1")
	for _, name := range []string{"host.identity.inspect", "host.graphics.inspect", "service.render_access.inspect"} {
		got, err := Dispatch(context.Background(), c, name, nil)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var out map[string]any
		if err := json.Unmarshal(got, &out); err != nil || out["operation"] != name {
			t.Fatalf("%s envelope: %s err=%v", name, string(got), err)
		}
		if out["transport"] != "host" {
			t.Fatalf("%s transport=%v want host", name, out["transport"])
		}
	}
}

func TestDispatchExecCommandRequiresSSHTransport(t *testing.T) {
	c := mustClient(t, "http://127.0.0.1")
	if _, err := Dispatch(context.Background(), c, "exec_command", map[string]any{"write_enabled": true, "command": "uptime", "transport": "kvm"}); err == nil {
		t.Fatal("exec_command with wrong transport accepted")
	}
	if _, err := Dispatch(context.Background(), c, "exec_command", map[string]any{"write_enabled": true, "command": "uptime; rm -rf /", "transport": "ssh"}); err == nil {
		t.Fatal("exec_command with shell metachars accepted")
	}
}

func TestDispatchWorkflowEnvelope(t *testing.T) {
	c := mustClient(t, "http://127.0.0.1")
	if _, err := Dispatch(context.Background(), c, "kvm_workflow_list", nil); err != nil {
		t.Fatalf("workflow_list: %v", err)
	}
	if _, err := Dispatch(context.Background(), c, "kvm_workflow_inspect", map[string]any{"name": ""}); err == nil {
		t.Fatal("workflow_inspect with empty name accepted")
	}
}

func TestDispatchSequencePlanRequiresTarget(t *testing.T) {
	c := mustClient(t, "http://127.0.0.1")
	if _, err := Dispatch(context.Background(), c, "kvm_sequence_plan", map[string]any{"actions": []any{}}); err == nil {
		t.Fatal("sequence_plan without target accepted")
	}
}
