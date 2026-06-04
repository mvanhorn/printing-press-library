// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

// Package dreows wraps a gorilla/websocket connection to the Dreo
// control-plane WS endpoint. Reads are parsed into StateUpdate values
// and surfaced over Updates(); a keepalive goroutine sends the literal
// string "2" every 15 seconds.
package dreows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// StateUpdate is one parsed state frame from the Dreo WS server.
type StateUpdate struct {
	DeviceSn   string          `json:"devicesn"`
	Fields     map[string]any  `json:"fields"`
	ReceivedAt time.Time       `json:"received_at"`
	Raw        json.RawMessage `json:"raw,omitempty"`
}

// Conn is a managed WebSocket connection. Updates are streamed via Updates();
// Send issues a control frame; Close shuts everything down.
type Conn struct {
	ws       *websocket.Conn
	updates  chan StateUpdate
	writeMu  sync.Mutex
	closeOne sync.Once
	closed   chan struct{}
	ctx      context.Context
	cancel   context.CancelFunc

	// errMu protects closeErr. readLoop stashes the underlying error when
	// ReadMessage returns BEFORE Close() was called by the caller —
	// otherwise the channel-close looks identical to a clean shutdown to
	// downstream consumers. Err() exposes it after the updates channel is
	// drained so commands like `watch` and `sensors record` can distinguish
	// "user Ctrl-C / clean stop" (returns nil) from "WS dropped, token
	// expired, server restart, network blip" (returns non-nil).
	errMu    sync.Mutex
	closeErr error
}

// Err returns the error that terminated the connection, if any. A nil
// return means the connection was closed cleanly via Close(); a non-nil
// return means readLoop exited because of an underlying I/O failure
// (server hangup, network drop, token revoked mid-session, etc).
// Callers should read this after the Updates() channel closes.
func (c *Conn) Err() error {
	c.errMu.Lock()
	defer c.errMu.Unlock()
	return c.closeErr
}

func (c *Conn) setErr(err error) {
	c.errMu.Lock()
	defer c.errMu.Unlock()
	if c.closeErr == nil {
		c.closeErr = err
	}
}

// Connect dials wss://<wsHost>/websocket?accessToken=...&timestamp=...
// and returns a managed Conn. The supplied context bounds the dial; the
// returned Conn outlives ctx (callers should Close to terminate).
func Connect(ctx context.Context, wsHost, accessToken string) (*Conn, error) {
	if accessToken == "" {
		return nil, errors.New("dreows: accessToken required")
	}
	ts := time.Now().UnixMilli()
	wsURL := fmt.Sprintf("wss://%s/websocket?accessToken=%s&timestamp=%d", wsHost, accessToken, ts)

	dialer := websocket.Dialer{HandshakeTimeout: 15 * time.Second}
	hdr := http.Header{}
	hdr.Set("User-Agent", "okhttp/4.9.1")

	wsConn, resp, err := dialer.DialContext(ctx, wsURL, hdr)
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		return nil, fmt.Errorf("dreows: dial %s: %w (http status %d)", wsHost, err, status)
	}
	cctx, cancel := context.WithCancel(context.Background())
	c := &Conn{
		ws:      wsConn,
		updates: make(chan StateUpdate, 32),
		closed:  make(chan struct{}),
		ctx:     cctx,
		cancel:  cancel,
	}
	go c.readLoop()
	go c.keepalive()
	return c, nil
}

// Updates returns the read-side channel. Closed when the connection terminates.
func (c *Conn) Updates() <-chan StateUpdate {
	return c.updates
}

// Send issues a Dreo control frame. params is the body for the "params"
// key of the JSON envelope (e.g. {"poweron": true}).
func (c *Conn) Send(deviceSn string, params map[string]any) error {
	if deviceSn == "" {
		return errors.New("dreows.Send: deviceSn required")
	}
	frame := map[string]any{
		"devicesn":  deviceSn,
		"method":    "control",
		"params":    params,
		"timestamp": time.Now().UnixMilli(),
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return c.ws.WriteJSON(frame)
}

// Close terminates the connection. Safe to call multiple times.
func (c *Conn) Close() error {
	var err error
	c.closeOne.Do(func() {
		c.cancel()
		c.writeMu.Lock()
		_ = c.ws.SetWriteDeadline(time.Now().Add(time.Second))
		_ = c.ws.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		err = c.ws.Close()
		c.writeMu.Unlock()
		close(c.closed)
	})
	return err
}

// Done is closed when the connection has fully shut down.
func (c *Conn) Done() <-chan struct{} { return c.closed }

func (c *Conn) keepalive() {
	t := time.NewTicker(15 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-t.C:
			c.writeMu.Lock()
			_ = c.ws.SetWriteDeadline(time.Now().Add(5 * time.Second))
			err := c.ws.WriteMessage(websocket.TextMessage, []byte("2"))
			c.writeMu.Unlock()
			if err != nil {
				select {
				case <-c.ctx.Done():
					// clean shutdown raced the ticker; ignore
				default:
					c.setErr(fmt.Errorf("websocket keepalive failed: %w", err))
					c.cancel() // wake readLoop so it exits promptly
				}
				return
			}
		}
	}
}

func (c *Conn) readLoop() {
	defer func() {
		close(c.updates)
	}()
	// No read deadline - server pushes when it has news.
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}
		_, msg, err := c.ws.ReadMessage()
		if err != nil {
			// Distinguish "caller asked us to close" (clean) from "server
			// hung up / network dropped / TLS reset" (unclean). When ctx
			// is already cancelled, Close() ran and the error is the
			// expected post-close failure; leave closeErr nil. Otherwise,
			// stash the error so Err() can surface it to callers.
			select {
			case <-c.ctx.Done():
				// Clean shutdown — Close() cancelled ctx first.
			default:
				c.setErr(fmt.Errorf("websocket disconnect: %w", err))
			}
			return
		}
		// Server may send a bare "2" pong or other ping frames; ignore non-JSON.
		if len(msg) == 0 || msg[0] != '{' {
			continue
		}
		upd, ok := parseFrame(msg)
		if !ok {
			continue
		}
		select {
		case c.updates <- upd:
		case <-c.ctx.Done():
			return
		}
	}
}

// parseFrame is tolerant of Dreo's state-frame shape variance:
//   - top-level may carry devicesn / deviceSn / sn
//   - state fields may live at top-level, or under "reported", "params",
//     "state", or "data". We flatten every non-protocol key/value pair
//     into Fields.
func parseFrame(raw []byte) (StateUpdate, bool) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return StateUpdate{}, false
	}
	sn, _ := firstString(m, "devicesn", "deviceSn", "sn")
	fields := map[string]any{}
	merge := func(src map[string]any) {
		for k, v := range src {
			if v == nil {
				continue
			}
			if k == "devicesn" || k == "deviceSn" || k == "sn" ||
				k == "method" || k == "timestamp" || k == "code" || k == "msg" {
				continue
			}
			fields[k] = v
		}
	}
	merge(m)
	for _, key := range []string{"reported", "params", "state", "data"} {
		if sub, ok := m[key].(map[string]any); ok {
			merge(sub)
		}
	}
	if sn == "" && len(fields) == 0 {
		return StateUpdate{}, false
	}
	return StateUpdate{
		DeviceSn:   sn,
		Fields:     fields,
		ReceivedAt: time.Now(),
		Raw:        append(json.RawMessage(nil), raw...),
	}, true
}

func firstString(m map[string]any, keys ...string) (string, bool) {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s, true
			}
		}
	}
	return "", false
}
