// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Home Assistant's configuration registries are exposed through its documented
// WebSocket command API, so compound CLI commands share this temporary session.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"nhooyr.io/websocket"
)

// HomeAssistantWSCall authenticates a temporary Home Assistant WebSocket
// session and sends one documented command. It returns the command result,
// never a locally synthesized registry response.
func (c *Client) HomeAssistantWSCall(ctx context.Context, command map[string]any) (json.RawMessage, error) {
	if c == nil || c.Config == nil {
		return nil, fmt.Errorf("Home Assistant WebSocket requires configured server and token")
	}
	auth := strings.TrimSpace(c.Config.AuthHeader())
	if !strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return nil, fmt.Errorf("Home Assistant WebSocket requires bearer-token authentication")
	}
	token := strings.TrimSpace(auth[len("Bearer "):])
	if token == "" {
		return nil, fmt.Errorf("Home Assistant WebSocket requires bearer-token authentication")
	}
	base, err := url.Parse(c.RequestBaseURL())
	if err != nil {
		return nil, fmt.Errorf("parse Home Assistant server URL: %w", err)
	}
	switch base.Scheme {
	case "http":
		base.Scheme = "ws"
	case "https":
		base.Scheme = "wss"
	default:
		return nil, fmt.Errorf("unsupported Home Assistant URL scheme %q", base.Scheme)
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/api/websocket"
	ws, _, err := websocket.Dial(ctx, base.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("connect Home Assistant WebSocket: %w", err)
	}
	defer ws.Close(websocket.StatusNormalClosure, "done")
	_, raw, err := ws.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("read Home Assistant auth challenge: %w", err)
	}
	var challenge map[string]any
	if err := json.Unmarshal(raw, &challenge); err != nil {
		return nil, fmt.Errorf("decode Home Assistant auth challenge: %w", err)
	}
	if challenge["type"] != "auth_required" {
		return nil, fmt.Errorf("unexpected Home Assistant WebSocket challenge %q", challenge["type"])
	}
	if err := writeWSJSON(ctx, ws, map[string]any{"type": "auth", "access_token": token}); err != nil {
		return nil, err
	}
	_, raw, err = ws.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("read Home Assistant auth result: %w", err)
	}
	var authResult map[string]any
	if err := json.Unmarshal(raw, &authResult); err != nil {
		return nil, fmt.Errorf("decode Home Assistant auth result: %w", err)
	}
	if authResult["type"] != "auth_ok" {
		return nil, fmt.Errorf("Home Assistant WebSocket authentication failed: %s", string(raw))
	}
	if command["id"] == nil {
		command["id"] = 1
	}
	wantID := command["id"]
	if err := writeWSJSON(ctx, ws, command); err != nil {
		return nil, err
	}
	for {
		_, raw, err = ws.Read(ctx)
		if err != nil {
			return nil, fmt.Errorf("read Home Assistant command result: %w", err)
		}
		var response map[string]any
		if err := json.Unmarshal(raw, &response); err != nil {
			return nil, fmt.Errorf("decode Home Assistant command result: %w", err)
		}
		if fmt.Sprint(response["id"]) != fmt.Sprint(wantID) {
			continue
		}
		if success, _ := response["success"].(bool); !success {
			// An unknown command is only an optional-capability result after the
			// authenticated server has actually rejected this exact command. This
			// distinction keeps callers from treating locally guessed routes as
			// supported features.
			if wsUnknownCommand(response) {
				return nil, &CapabilityError{Surface: fmt.Sprint(command["type"]), Detail: string(raw)}
			}
			return nil, fmt.Errorf("Home Assistant WebSocket command failed: %s", string(raw))
		}
		return json.RawMessage(raw), nil
	}
}

// HomeAssistantWSWatchEvent keeps an authenticated subscribe_events request
// open until the server delivers one matching event or the caller cancels its
// context. Unlike HomeAssistantWSCall, it does not close after the subscription
// acknowledgement, so `event watch` is a real stream rather than a probe.
func (c *Client) HomeAssistantWSWatchEvent(ctx context.Context, eventType string) (json.RawMessage, error) {
	if c == nil || c.Config == nil {
		return nil, fmt.Errorf("Home Assistant WebSocket requires configured server and token")
	}
	auth := strings.TrimSpace(c.Config.AuthHeader())
	if !strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return nil, fmt.Errorf("Home Assistant WebSocket requires bearer-token authentication")
	}
	token := strings.TrimSpace(auth[len("Bearer "):])
	if token == "" {
		return nil, fmt.Errorf("Home Assistant WebSocket requires bearer-token authentication")
	}
	base, err := url.Parse(c.RequestBaseURL())
	if err != nil {
		return nil, fmt.Errorf("parse Home Assistant server URL: %w", err)
	}
	switch base.Scheme {
	case "http":
		base.Scheme = "ws"
	case "https":
		base.Scheme = "wss"
	default:
		return nil, fmt.Errorf("unsupported Home Assistant URL scheme %q", base.Scheme)
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/api/websocket"
	ws, _, err := websocket.Dial(ctx, base.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("connect Home Assistant WebSocket: %w", err)
	}
	defer ws.Close(websocket.StatusNormalClosure, "event watch done")
	_, raw, err := ws.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("read Home Assistant auth challenge: %w", err)
	}
	var challenge map[string]any
	if err := json.Unmarshal(raw, &challenge); err != nil || challenge["type"] != "auth_required" {
		return nil, fmt.Errorf("unexpected Home Assistant WebSocket auth challenge")
	}
	if err := writeWSJSON(ctx, ws, map[string]any{"type": "auth", "access_token": token}); err != nil {
		return nil, err
	}
	_, raw, err = ws.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("read Home Assistant auth result: %w", err)
	}
	var authResult map[string]any
	if err := json.Unmarshal(raw, &authResult); err != nil || authResult["type"] != "auth_ok" {
		return nil, fmt.Errorf("Home Assistant WebSocket authentication failed")
	}
	const subscriptionID = 1
	if err := writeWSJSON(ctx, ws, map[string]any{"id": subscriptionID, "type": "subscribe_events", "event_type": eventType}); err != nil {
		return nil, err
	}
	acknowledged := false
	for {
		_, raw, err = ws.Read(ctx)
		if err != nil {
			return nil, fmt.Errorf("read Home Assistant event stream: %w", err)
		}
		var response map[string]any
		if err := json.Unmarshal(raw, &response); err != nil {
			return nil, fmt.Errorf("decode Home Assistant event stream: %w", err)
		}
		if fmt.Sprint(response["id"]) != fmt.Sprint(subscriptionID) {
			continue
		}
		if response["type"] == "event" {
			return json.RawMessage(raw), nil
		}
		if success, exists := response["success"].(bool); exists {
			if !success {
				return nil, fmt.Errorf("Home Assistant event subscription failed: %s", string(raw))
			}
			acknowledged = true
			continue
		}
		if !acknowledged {
			return nil, fmt.Errorf("unexpected Home Assistant event subscription response: %s", string(raw))
		}
	}
}

func wsUnknownCommand(response map[string]any) bool {
	errValue, ok := response["error"].(map[string]any)
	if !ok {
		return false
	}
	code := strings.ToLower(fmt.Sprint(errValue["code"]))
	message := strings.ToLower(fmt.Sprint(errValue["message"]))
	return code == "unknown_command" || code == "unsupported_command" ||
		strings.Contains(message, "unknown command") || strings.Contains(message, "unsupported command")
}

func writeWSJSON(ctx context.Context, ws *websocket.Conn, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err := ws.Write(ctx, websocket.MessageText, raw); err != nil {
		return fmt.Errorf("write Home Assistant WebSocket command: %w", err)
	}
	return nil
}
