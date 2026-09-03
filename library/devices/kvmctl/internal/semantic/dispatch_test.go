package semantic

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/devices/kvmctl/internal/client"
	"github.com/mvanhorn/printing-press-library/library/devices/kvmctl/internal/config"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDispatchStatusReturnsStableEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":{"ok":true}}`))
	}))
	defer srv.Close()
	c := client.New(&config.Config{BaseURL: srv.URL}, 0, 0)
	got, err := Dispatch(context.Background(), c, "status", nil)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if json.Unmarshal(got, &out) != nil || out["operation"] != "status" || out["ok"] != true {
		t.Fatalf("%s", got)
	}
}

func TestDispatchRejectsUnknownAndWriteWithoutGate(t *testing.T) {
	c := client.New(&config.Config{BaseURL: "http://127.0.0.1"}, 0, 0)
	if _, err := Dispatch(context.Background(), c, "nope", nil); err == nil {
		t.Fatal("unknown operation accepted")
	}
	if _, err := Dispatch(context.Background(), c, "keyboard", map[string]any{"key": "A"}); err == nil {
		t.Fatal("write without gate accepted")
	}
}

func TestClickTextRequiresFreshObservationWritesAndReturnsPostObservation(t *testing.T) {
	t.Setenv("KVMCTL_OCR_PROTOCOL", "json")
	t.Setenv("KVMCTL_OCR_COMMAND", writeFakeOCR(t, `{"width":100,"height":50,"words":[{"text":"Proceed","confidence":95,"x":10,"y":10,"width":40,"height":10}]}`))
	var snapshots int
	var events []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/streamer/snapshot":
			snapshots++
			_, _ = w.Write([]byte("snapshot"))
		case "/api/hid/events/send_mouse_move", "/api/hid/events/send_mouse_button":
			events = append(events, r.URL.Path+"?"+r.URL.RawQuery)
			_, _ = w.Write([]byte(`{"result":{}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	c := client.New(&config.Config{BaseURL: srv.URL}, 0, 0)

	observed := dispatchObject(t, c, "observe", nil)
	id := observed["evidence"].(map[string]any)["observation"].(map[string]any)["observation_id"].(string)
	if _, err := Dispatch(context.Background(), c, "click-text", map[string]any{"observation_id": id, "text": "Proceed"}); err == nil {
		t.Fatal("click-text accepted without write_enabled")
	}
	out := dispatchObject(t, c, "click-text", map[string]any{"write_enabled": true, "observation_id": id, "text": "  proceed  "})
	if out["ok"] != true || snapshots != 3 || len(events) != 3 {
		t.Fatalf("output=%#v snapshots=%d events=%v", out, snapshots, events)
	}
	if _, ok := out["evidence"].(map[string]any)["post_observation"]; !ok {
		t.Fatalf("missing post observation: %#v", out)
	}
}

func TestClickTextRefusesChangedScreenBeforeHID(t *testing.T) {
	t.Setenv("KVMCTL_OCR_PROTOCOL", "json")
	t.Setenv("KVMCTL_OCR_COMMAND", writeFakeOCR(t, `{"width":100,"height":50,"words":[{"text":"Proceed","confidence":95,"x":10,"y":10,"width":40,"height":10}]}`))
	var snapshots, hidEvents int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/streamer/snapshot":
			snapshots++
			_, _ = w.Write([]byte(fmt.Sprintf("snapshot-%d", snapshots)))
		case "/api/hid/events/send_mouse_move", "/api/hid/events/send_mouse_button":
			hidEvents++
			_, _ = w.Write([]byte(`{"result":{}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	c := client.New(&config.Config{BaseURL: srv.URL}, 0, 0)
	observed := dispatchObject(t, c, "observe", nil)
	id := observed["evidence"].(map[string]any)["observation"].(map[string]any)["observation_id"].(string)
	if _, err := Dispatch(context.Background(), c, "click-text", map[string]any{"write_enabled": true, "observation_id": id, "text": "Proceed"}); err == nil {
		t.Fatal("changed screen did not refuse click")
	}
	if hidEvents != 0 {
		t.Fatalf("changed screen sent %d HID events", hidEvents)
	}
}

func TestPressKeyAndVerifyTextFailClosedOnAmbiguity(t *testing.T) {
	t.Setenv("KVMCTL_OCR_PROTOCOL", "json")
	t.Setenv("KVMCTL_OCR_COMMAND", writeFakeOCR(t, `{"width":100,"height":50,"words":[{"text":"Ready","confidence":95,"x":10,"y":10,"width":20,"height":10},{"text":"Ready","confidence":95,"x":50,"y":10,"width":20,"height":10}]}`))
	var keyEvents []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/streamer/snapshot":
			_, _ = w.Write([]byte("snapshot"))
		case "/api/hid/events/send_key":
			keyEvents = append(keyEvents, r.URL.RawQuery)
			_, _ = w.Write([]byte(`{"result":{}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	c := client.New(&config.Config{BaseURL: srv.URL}, 0, 0)
	observed := dispatchObject(t, c, "observe", nil)
	id := observed["evidence"].(map[string]any)["observation"].(map[string]any)["observation_id"].(string)
	if _, err := Dispatch(context.Background(), c, "press-key", map[string]any{"write_enabled": true, "observation_id": "wrong", "key": "Enter"}); err == nil {
		t.Fatal("press-key accepted a stale observation")
	}
	out := dispatchObject(t, c, "press-key", map[string]any{"write_enabled": true, "observation_id": id, "key": "Enter"})
	if out["ok"] != true || len(keyEvents) != 2 {
		t.Fatalf("press-key output=%#v events=%v", out, keyEvents)
	}
	verified := dispatchObject(t, c, "verify-text", map[string]any{"text": "ready"})
	if verified["ok"] != false || verified["evidence"].(map[string]any)["outcome"] != "ambiguous" {
		t.Fatalf("verify-text did not report ambiguity: %#v", verified)
	}
}

func TestObserveReturnsUnavailableEnvelopeWithoutSnapshotOrOCR(t *testing.T) {
	t.Setenv("KVMCTL_OCR_COMMAND", "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "down", http.StatusServiceUnavailable) }))
	defer srv.Close()
	c := client.New(&config.Config{BaseURL: srv.URL}, 0, 0)
	out := dispatchObject(t, c, "observe", nil)
	if out["ok"] != false || out["state"] != "unavailable" || out["evidence"].(map[string]any)["unavailable"] != true {
		t.Fatalf("expected unavailable envelope, got %#v", out)
	}
}

func TestObserveReturnsUnavailableEnvelopeWithoutConfiguredOCR(t *testing.T) {
	t.Setenv("KVMCTL_OCR_COMMAND", "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("live-snapshot")) }))
	defer srv.Close()
	c := client.New(&config.Config{BaseURL: srv.URL}, 0, 0)
	out := dispatchObject(t, c, "observe", nil)
	if out["ok"] != false || out["state"] != "unavailable" || out["evidence"].(map[string]any)["unavailable"] != true {
		t.Fatalf("expected OCR-unavailable envelope, got %#v", out)
	}
	if !strings.Contains(out["error"].(map[string]any)["code"].(string), "ocr unavailable") {
		t.Fatalf("missing OCR unavailable error: %#v", out)
	}
}

func TestObserveCapturesFreshSnapshotAndConfiguredOCREvidence(t *testing.T) {
	t.Setenv("KVMCTL_OCR_PROTOCOL", "json")
	command := writeFakeOCR(t, `{"width":100,"height":50,"words":[{"text":"Continue","confidence":95,"x":10,"y":10,"width":40,"height":10}]}`)
	t.Setenv("KVMCTL_OCR_COMMAND", command)

	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/streamer/snapshot" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		requests++
		_, _ = w.Write([]byte("live-snapshot"))
	}))
	defer srv.Close()

	c := client.New(&config.Config{BaseURL: srv.URL}, 0, 0)
	out := dispatchObject(t, c, "observe", nil)
	if requests != 1 || out["ok"] != true || out["operation"] != "observe" {
		t.Fatalf("observe output=%#v requests=%d", out, requests)
	}
	evidence := out["evidence"].(map[string]any)
	observation := evidence["observation"].(map[string]any)
	if observation["observation_id"] == "" || observation["image_sha256"] == "" {
		t.Fatalf("missing canonical observation evidence: %#v", observation)
	}
}

func dispatchObject(t *testing.T, c *client.Client, operation string, args map[string]any) map[string]any {
	t.Helper()
	raw, err := Dispatch(context.Background(), c, operation, args)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func writeFakeOCR(t *testing.T, response string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-ocr")
	program := "#!/bin/sh\ncat >/dev/null\nprintf '%s\\n' '" + response + "'\n"
	if err := os.WriteFile(path, []byte(program), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}
