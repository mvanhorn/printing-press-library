package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/home-assistant/internal/config"
	"nhooyr.io/websocket"
)

func TestWSUnknownCommand(t *testing.T) {
	if !wsUnknownCommand(map[string]any{"error": map[string]any{"code": "unknown_command"}}) {
		t.Fatal("unknown_command must be classified as an optional capability result")
	}
	if !wsUnknownCommand(map[string]any{"error": map[string]any{"message": "Unknown command"}}) {
		t.Fatal("unknown command message must be classified as an optional capability result")
	}
	if wsUnknownCommand(map[string]any{"error": map[string]any{"code": "invalid_format"}}) {
		t.Fatal("input validation failures must remain regular API errors")
	}
}

func TestHomeAssistantWSWatchEventSendsSubscriptionAndReturnsEvent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/websocket" {
			t.Errorf("unexpected websocket path %q", r.URL.Path)
		}
		ws, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept: %v", err)
			return
		}
		defer ws.Close(websocket.StatusNormalClosure, "done")
		if err := writeWSJSON(r.Context(), ws, map[string]any{"type": "auth_required"}); err != nil {
			t.Errorf("challenge: %v", err)
			return
		}
		var auth map[string]any
		if _, raw, err := ws.Read(r.Context()); err != nil || json.Unmarshal(raw, &auth) != nil || auth["access_token"] != "token" {
			t.Errorf("auth frame: %v %s", err, raw)
			return
		}
		if err := writeWSJSON(r.Context(), ws, map[string]any{"type": "auth_ok"}); err != nil {
			t.Errorf("auth ok: %v", err)
			return
		}
		var subscription map[string]any
		if _, raw, err := ws.Read(r.Context()); err != nil || json.Unmarshal(raw, &subscription) != nil || subscription["type"] != "subscribe_events" || subscription["event_type"] != "state_changed" {
			t.Errorf("subscription frame: %v %s", err, raw)
			return
		}
		if err := writeWSJSON(r.Context(), ws, map[string]any{"id": 1, "type": "result", "success": true}); err != nil {
			t.Errorf("ack: %v", err)
			return
		}
		if err := writeWSJSON(r.Context(), ws, map[string]any{"id": 1, "type": "event", "event": map[string]any{"event_type": "state_changed"}}); err != nil {
			t.Errorf("event: %v", err)
		}
	}))
	defer server.Close()
	c := &Client{BaseURL: server.URL, Config: &config.Config{HassToken: "token"}}
	raw, err := c.HomeAssistantWSWatchEvent(context.Background(), "state_changed")
	if err != nil || !strings.Contains(string(raw), "state_changed") {
		t.Fatalf("watch event = %s, %v", raw, err)
	}
}

func TestHomeAssistantWSWatchEventHonorsCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer ws.Close(websocket.StatusNormalClosure, "done")
		_ = writeWSJSON(r.Context(), ws, map[string]any{"type": "auth_required"})
		_, _, _ = ws.Read(r.Context())
		_ = writeWSJSON(r.Context(), ws, map[string]any{"type": "auth_ok"})
		_, _, _ = ws.Read(r.Context())
		_ = writeWSJSON(r.Context(), ws, map[string]any{"id": 1, "type": "result", "success": true})
		<-r.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	c := &Client{BaseURL: server.URL, Config: &config.Config{HassToken: "token"}}
	_, err := c.HomeAssistantWSWatchEvent(ctx, "state_changed")
	if err == nil {
		t.Fatal("event watch must stop when its context is cancelled")
	}
}
