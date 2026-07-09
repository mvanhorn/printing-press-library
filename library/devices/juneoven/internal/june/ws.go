package june

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// Oven -> companion message codes.
const (
	codeAck       = 10020
	codeState     = 10018
	codeTelemetry = 10013
	codeCancelled = 10017
	codeCamera    = 10011
)

// CommandResult is the outcome of a signed command: the oven's ack status
// ("success" or "not-allowed"), or ok=false when no ack arrived in time.
type CommandResult struct {
	Status string `json:"status"`
	Acked  bool   `json:"acked"`
}

// dial opens the companion WebSocket with the bearer token and deflate disabled
// (the oven requires plain frames).
func dial(ctx context.Context, token string) (*websocket.Conn, error) {
	d := websocket.Dialer{
		HandshakeTimeout:  15 * time.Second,
		EnableCompression: false, // permessage-deflate breaks the oven
	}
	h := http.Header{}
	h.Set("Authorization", "Bearer "+token)
	h.Set("User-Agent", UserAgent)
	conn, _, err := d.DialContext(ctx, WSURL, h)
	if err != nil {
		return nil, fmt.Errorf("dialing oven websocket: %w", err)
	}
	return conn, nil
}

// SendCommand signs and sends one command, then waits up to listen for the
// matching ack. It sends an 11011 keepalive first (presence), like the app.
func SendCommand(ctx context.Context, id *Identity, code int, data json.RawMessage, listen time.Duration) (CommandResult, error) {
	s, err := id.Signer()
	if err != nil {
		return CommandResult{}, err
	}
	conn, err := dial(ctx, id.AccessToken)
	if err != nil {
		return CommandResult{}, err
	}
	defer conn.Close()

	now := NowMillis()
	keepalive, _, err := s.Frame(CodeKeepalive, json.RawMessage("{}"), id.DeviceName, id.DeviceID, id.OvenID, now)
	if err != nil {
		return CommandResult{}, err
	}
	if err := conn.WriteMessage(websocket.TextMessage, []byte(keepalive)); err != nil {
		return CommandResult{}, fmt.Errorf("sending keepalive: %w", err)
	}

	frame, order, err := s.Frame(code, data, id.DeviceName, id.DeviceID, id.OvenID, NowMillis())
	if err != nil {
		return CommandResult{}, err
	}
	if err := conn.WriteMessage(websocket.TextMessage, []byte(frame)); err != nil {
		return CommandResult{}, fmt.Errorf("sending command: %w", err)
	}

	deadline := time.Now().Add(listen)
	_ = conn.SetReadDeadline(deadline)
	for time.Now().Before(deadline) {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break // timeout or close
		}
		var env struct {
			MessageCode int `json:"message_code"`
			Data        struct {
				RequestOrder int64  `json:"request_order"`
				Status       string `json:"status"`
			} `json:"data"`
		}
		if json.Unmarshal(msg, &env) != nil {
			continue
		}
		if env.MessageCode == codeAck && env.Data.RequestOrder == order {
			return CommandResult{Status: env.Data.Status, Acked: true}, nil
		}
	}
	return CommandResult{Acked: false}, nil
}

// TelemetryEvent is one decoded line from the live cook stream.
type TelemetryEvent struct {
	Type      string `json:"type"`                 // "telemetry" | "state" | "cancelled" | "camera"
	State     string `json:"state,omitempty"`      // for state events
	CurrentF  *int   `json:"current_f,omitempty"`  // for telemetry
	Progress  *int   `json:"progress,omitempty"`   // for telemetry
	CameraURL string `json:"camera_url,omitempty"` // for camera frames
}

// Watch opens the socket, sends periodic keepalives, and calls emit for each
// decoded telemetry/state event until the cook ends (10017, or a 10018 idle
// transition) or ctx is cancelled.
func Watch(ctx context.Context, id *Identity, emit func(TelemetryEvent)) error {
	s, err := id.Signer()
	if err != nil {
		return err
	}
	conn, err := dial(ctx, id.AccessToken)
	if err != nil {
		return err
	}
	defer conn.Close()

	sendKeepalive := func() error {
		f, _, err := s.Frame(CodeKeepalive, json.RawMessage("{}"), id.DeviceName, id.DeviceID, id.OvenID, NowMillis())
		if err != nil {
			return err
		}
		return conn.WriteMessage(websocket.TextMessage, []byte(f))
	}
	if err := sendKeepalive(); err != nil {
		return err
	}

	ticker := time.NewTicker(7 * time.Second)
	defer ticker.Stop()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = sendKeepalive()
			}
		}
	}()

	for {
		if ctx.Err() != nil {
			return nil
		}
		_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		_, msg, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("reading telemetry: %w", err)
		}
		ev, done, ok := decodeTelemetry(msg)
		if ok {
			emit(ev)
		}
		if done {
			return nil
		}
	}
}

func decodeTelemetry(msg []byte) (ev TelemetryEvent, done, ok bool) {
	var env struct {
		MessageCode int             `json:"message_code"`
		Data        json.RawMessage `json:"data"`
	}
	if json.Unmarshal(msg, &env) != nil {
		return ev, false, false
	}
	switch env.MessageCode {
	case codeTelemetry:
		var d struct {
			SensorData struct {
				Cavity int `json:"cavity"`
			} `json:"sensor_data"`
			CookStateData struct {
				Progress int `json:"progress"`
			} `json:"cook_state_data"`
		}
		if json.Unmarshal(env.Data, &d) != nil {
			return ev, false, false
		}
		f := MilliCToFahrenheit(d.SensorData.Cavity)
		p := d.CookStateData.Progress
		return TelemetryEvent{Type: "telemetry", CurrentF: &f, Progress: &p}, false, true
	case codeState:
		var d struct {
			State string `json:"state"`
		}
		if json.Unmarshal(env.Data, &d) != nil {
			return ev, false, false
		}
		// A natural transition to idle ends the watch.
		return TelemetryEvent{Type: "state", State: d.State}, d.State == "idle", true
	case codeCancelled:
		return TelemetryEvent{Type: "cancelled"}, true, true
	case codeCamera:
		var d struct {
			SignedURL string `json:"signed_url"`
		}
		if json.Unmarshal(env.Data, &d) != nil {
			return ev, false, false
		}
		return TelemetryEvent{Type: "camera", CameraURL: d.SignedURL}, false, true
	}
	return ev, false, false
}
