// Copyright 2026 Beau Sterling and contributors. Licensed under Apache-2.0. See LICENSE.

package milanote

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// packetSeparator splits Engine.IO packets inside one polling payload.
const packetSeparator = "\x1e"

// ackPolls bounds how many long-poll GETs wait for one ack. The server holds
// an idle GET for up to its ping interval (25s).
const ackPolls = 3

// Conn is one socket.io connection over HTTP long-polling. The web app sends
// every change as an "action" event and waits for the ack [{"status":200}];
// the same server answers the polling transport, so plain requests suffice.
type Conn struct {
	c       *Client
	user    *User
	ids     *IDGen
	url     string
	ack     int
	device  string
	session string
	closed  bool
}

// Open performs the Engine.IO handshake and the socket.io namespace connect.
func (c *Client) Open(ctx context.Context) (*Conn, error) {
	user, err := c.Me(ctx)
	if err != nil {
		return nil, err
	}
	stamp := Base62(time.Now().Unix())
	s := &Conn{
		c:       c,
		user:    user,
		ids:     c.ids,
		url:     fmt.Sprintf("%s/socket.io/?userId=%s&EIO=4&transport=polling", c.BaseURL, user.ID),
		device:  "mdid-" + stamp + RandomBase62(4),
		session: "msid-" + stamp + RandomBase62(4),
	}
	packets, err := s.get(ctx)
	if err != nil {
		return nil, err
	}
	if len(packets) == 0 || !strings.HasPrefix(packets[0], "0") {
		return nil, errors.New("milanote did not open a sync connection")
	}
	var open struct {
		SID string `json:"sid"`
	}
	if err := json.Unmarshal([]byte(packets[0][1:]), &open); err != nil || open.SID == "" {
		return nil, errors.New("milanote sync handshake had no session id")
	}
	s.url += "&sid=" + open.SID
	if err := s.post(ctx, "40"); err != nil {
		return nil, err
	}
	for i := 0; i < ackPolls; i++ {
		packets, err := s.get(ctx)
		if err != nil {
			return nil, err
		}
		for _, p := range packets {
			if strings.HasPrefix(p, "40") {
				return s, nil
			}
			if strings.HasPrefix(p, "44") {
				return nil, fmt.Errorf("milanote refused the sync connection: %s", strings.TrimPrefix(p, "44"))
			}
		}
	}
	return nil, errors.New("milanote did not accept the sync connection")
}

func (s *Conn) get(ctx context.Context) ([]string, error) {
	req, err := s.c.newRequest(ctx, http.MethodGet, s.url, nil, "")
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "*/*")
	resp, err := s.c.Poll.Do(req)
	if err != nil {
		return nil, fmt.Errorf("milanote sync poll: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, ErrUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("milanote sync poll: HTTP %d %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return strings.Split(string(body), packetSeparator), nil
}

func (s *Conn) post(ctx context.Context, packet string) error {
	req, err := s.c.newRequest(ctx, http.MethodPost, s.url, bytes.NewReader([]byte(packet)), "text/plain;charset=UTF-8")
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "*/*")
	resp, err := s.c.Poll.Do(req)
	if err != nil {
		return fmt.Errorf("milanote sync send: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return ErrUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("milanote sync send: HTTP %d %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// stamp adds the envelope fields the web app puts on every synced action.
func (s *Conn) stamp(action map[string]any, channels []string) map[string]any {
	out := make(map[string]any, len(action)+7)
	for k, v := range action {
		out[k] = v
	}
	out["sync"] = true
	out["user"] = map[string]any{"_id": s.user.ID, "clientId": s.ids.ClientID(), "clientTick": s.ids.Tick()}
	out["tokens"] = []any{}
	out["deviceId"] = s.device
	out["sessionId"] = s.session
	out["channels"] = channels
	return out
}

// Send emits one action and waits for Milanote's ack.
func (s *Conn) Send(ctx context.Context, action map[string]any, channels []string) error {
	if s.closed {
		return errors.New("milanote sync connection is closed")
	}
	id := s.ack
	s.ack++
	frame, err := json.Marshal([]any{"action", s.stamp(action, channels)})
	if err != nil {
		return fmt.Errorf("encoding action: %w", err)
	}
	if err := s.post(ctx, fmt.Sprintf("42%d%s", id, frame)); err != nil {
		return err
	}
	prefix := fmt.Sprintf("43%d", id)
	for i := 0; i < ackPolls; i++ {
		packets, err := s.get(ctx)
		if err != nil {
			return err
		}
		for _, p := range packets {
			switch {
			case p == "2":
				if err := s.post(ctx, "3"); err != nil {
					return err
				}
			case strings.HasPrefix(p, prefix):
				return ackError(strings.TrimPrefix(p, prefix), action)
			case strings.HasPrefix(p, "41"), p == "1":
				return errors.New("milanote closed the sync connection")
			}
		}
	}
	return fmt.Errorf("milanote did not confirm %v in time", action["type"])
}

func ackError(payload string, action map[string]any) error {
	var answer []map[string]any
	if err := json.Unmarshal([]byte(payload), &answer); err != nil || len(answer) == 0 {
		return fmt.Errorf("milanote sent an unreadable ack for %v", action["type"])
	}
	status, _ := answer[0]["status"].(float64)
	if int(status) == http.StatusOK {
		return nil
	}
	msg, _ := answer[0]["message"].(string)
	if msg == "" {
		if e, ok := answer[0]["error"].(string); ok {
			msg = e
		}
	}
	return fmt.Errorf("milanote rejected %v (status %d) %s", action["type"], int(status), msg)
}

// Close ends the connection; errors are ignored because the server times
// idle sessions out on its own.
func (s *Conn) Close(ctx context.Context) {
	if s == nil || s.closed {
		return
	}
	s.closed = true
	_ = s.post(ctx, "1")
}

// User is the signed-in account this connection writes as.
func (s *Conn) User() *User { return s.user }

// NewID mints a fresh element id.
func (s *Conn) NewID() string { return s.ids.Next() }
