// Package client implements a minimal Socket.IO v4 / engine.io v4
// long-polling client for Uptime Kuma v2. Kuma's management API (monitors,
// heartbeats, notifications, maintenance, edits) is exposed exclusively over
// Socket.IO; there are no authenticated REST endpoints.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Config carries connection settings. Credentials come from the environment
// (UPTIME_KUMA_URL / UPTIME_KUMA_USERNAME / UPTIME_KUMA_PASSWORD) or flags;
// they must never appear in argv defaults or logs.
type Config struct {
	BaseURL  string
	Username string
	Password string

	HTTPClient *http.Client
}

// AuthError signals rejected credentials (distinct exit code from network errors).
type AuthError struct{ Msg string }

func (e *AuthError) Error() string { return "auth failed: " + e.Msg }

// ProtocolError signals a malformed or unexpected Socket.IO exchange.
type ProtocolError struct{ Msg string }

func (e *ProtocolError) Error() string { return "protocol error: " + e.Msg }

type session struct {
	base   *url.URL
	sid    string
	http   *http.Client
	mu     sync.Mutex
	nextID int
}

// Client is a Uptime Kuma v2 Socket.IO client.
type Client struct {
	cfg    Config
	sess   *session
	authd  bool
	jwt    string
	authMu sync.Mutex
	connMu sync.Mutex

	stashMu sync.Mutex
	stash   [][]byte // event records ("42...") seen while waiting for other packets
	acks    map[int][]byte
	stream  *eventStream // active heartbeat collection; backpressure prevents dropped events

	readerMu      sync.Mutex
	readerRunning bool
	readerCancel  context.CancelFunc
	nsConnected   chan struct{}
	signalNSOnce  sync.Once
	ackCh         map[int]chan []byte
}

const maxStashedEvents = 256

type eventStream struct {
	name        string
	queue       [][]byte
	queuedBytes int
	space       chan struct{}
	done        chan struct{}
}

// appendStashedEventLocked retains a bounded tail of server-pushed events.
// Callers must hold stashMu.
func (c *Client) appendStashedEventLocked(payload []byte) {
	payload = append([]byte(nil), payload...)
	if len(c.stash) >= maxStashedEvents {
		copy(c.stash, c.stash[len(c.stash)-maxStashedEvents+1:])
		c.stash = c.stash[:maxStashedEvents-1]
	}
	c.stash = append(c.stash, payload)
}

// noteAck remembers an ACK record seen before its emit was even made (the
// server can bundle several responses into one poll payload).
func (c *Client) noteAcks(recs []string) {
	c.stashMu.Lock()
	defer c.stashMu.Unlock()
	if c.acks == nil {
		c.acks = map[int][]byte{}
	}
	for _, r := range recs {
		if len(r) > 3 && r[0] == '4' && r[1] == '3' {
			id := 0
			ok := true
			for _, ch := range r[2:] {
				if ch < '0' || ch > '9' {
					break
				}
				id = id*10 + int(ch-'0')
			}
			if !ok || id == 0 {
				continue
			}
			rest := ""
			for i, ch := range r[2:] {
				if ch < '0' || ch > '9' {
					rest = r[2+i:]
					break
				}
			}
			c.acks[id] = []byte(rest)
		}
	}
}

// stashEvents buffers Socket.IO EVENT records (full engine.io records
// beginning with "42"); the arguments JSON starts at index 2.
func (c *Client) stashEvents(recs []string) {
	c.stashMu.Lock()
	defer c.stashMu.Unlock()
	for _, r := range recs {
		if len(r) > 2 && r[0] == '4' && r[1] == '2' {
			c.appendStashedEventLocked([]byte(r[2:]))
		}
	}
}

// beginEventStream registers before the request is emitted, so a whole poll
// response can arrive without filling and truncating the ordinary stash.
func (c *Client) beginEventStream(name string) (*eventStream, error) {
	c.stashMu.Lock()
	defer c.stashMu.Unlock()
	if c.stream != nil {
		return nil, &ProtocolError{Msg: "another heartbeat collection is active"}
	}
	stream := &eventStream{name: name, queue: make([][]byte, 0, maxStashedEvents), space: make(chan struct{}, 1), done: make(chan struct{})}
	c.stream = stream
	return stream, nil
}

func (c *Client) endEventStream(stream *eventStream) {
	c.stashMu.Lock()
	defer c.stashMu.Unlock()
	if c.stream == stream {
		c.stream = nil
		close(stream.done)
		for _, p := range stream.queue {
			c.appendStashedEventLocked(p)
		}
		stream.queue = nil
		stream.queuedBytes = 0
	}
}

// drainEventStream frees queue space while preserving the batch for the
// collector. A waiting reader resumes after the collector takes this batch.
func (c *Client) drainEventStream(stream *eventStream) [][]byte {
	c.stashMu.Lock()
	defer c.stashMu.Unlock()
	batch := stream.queue
	stream.queue = nil
	stream.queuedBytes = 0
	select {
	case stream.space <- struct{}{}:
	default:
	}
	return batch
}

// finishEventStream closes the active stream only when its queue is empty.
// Sends and teardown use the same mutex, so no event enters an abandoned queue.
func (c *Client) finishEventStream(stream *eventStream) ([][]byte, bool) {
	c.stashMu.Lock()
	defer c.stashMu.Unlock()
	if len(stream.queue) > 0 {
		batch := stream.queue
		stream.queue = nil
		stream.queuedBytes = 0
		select {
		case stream.space <- struct{}{}:
		default:
		}
		return batch, false
	}
	if c.stream == stream {
		c.stream = nil
		close(stream.done)
	}
	return nil, true
}

func payloadForEvent(p []byte, name string) json.RawMessage {
	var pair []json.RawMessage
	if json.Unmarshal(p, &pair) != nil || len(pair) < 2 {
		return nil
	}
	var got string
	if json.Unmarshal(pair[0], &got) != nil || got != name {
		return nil
	}
	if len(pair) >= 3 {
		return json.RawMessage(p)
	}
	return pair[1]
}

// routeEvent appends to a bounded active queue. A full queue pauses the reader
// until the collector drains it, preserving bursts without an unbounded stash.
func (c *Client) routeEvent(ctx context.Context, p []byte) {
	for {
		c.stashMu.Lock()
		stream := c.stream
		if stream == nil || payloadForEvent(p, stream.name) == nil {
			c.appendStashedEventLocked(p)
			c.stashMu.Unlock()
			return
		}
		if len(stream.queue) < maxStashedEvents && len(p) <= maxCollectedHeartbeatBytes-stream.queuedBytes {
			stream.queue = append(stream.queue, append([]byte(nil), p...))
			stream.queuedBytes += len(p)
			c.stashMu.Unlock()
			return
		}
		c.stashMu.Unlock()
		select {
		case <-stream.space:
		case <-stream.done:
		case <-ctx.Done():
			return
		}
	}
}

func (c *Client) takeStashedEvent(name string) json.RawMessage {
	c.stashMu.Lock()
	defer c.stashMu.Unlock()
	for i, p := range c.stash {
		var pair []json.RawMessage
		if json.Unmarshal(p, &pair) == nil && len(pair) >= 2 {
			var n string
			if json.Unmarshal(pair[0], &n) == nil && n == name {
				c.stash = append(c.stash[:i], c.stash[i+1:]...)
				if len(pair) >= 3 {
					return p
				}
				return pair[1]
			}
		}
	}
	return nil
}

// DrainStashedEvents returns all currently buffered payloads for an event.
func (c *Client) DrainStashedEvents(name string) []json.RawMessage {
	c.stashMu.Lock()
	defer c.stashMu.Unlock()
	out := make([]json.RawMessage, 0)
	kept := c.stash[:0]
	for _, p := range c.stash {
		var pair []json.RawMessage
		var n string
		if json.Unmarshal(p, &pair) == nil && len(pair) >= 2 && json.Unmarshal(pair[0], &n) == nil && n == name {
			if len(pair) >= 3 {
				out = append(out, p)
			} else {
				out = append(out, pair[1])
			}
		} else {
			kept = append(kept, p)
		}
	}
	c.stash = kept
	return out
}

// New builds a client without connecting.
func New(cfg Config) *Client {
	hc := cfg.HTTPClient
	if hc == nil {
		// Kuma long polls may remain open for pingInterval+pingTimeout.
		hc = &http.Client{Timeout: 90 * time.Second}
	}
	cfg.HTTPClient = withSafeRedirects(hc)
	return &Client{
		cfg:         cfg,
		nsConnected: make(chan struct{}),
		ackCh:       map[int]chan []byte{},
	}
}

// withSafeRedirects preserves the caller's transport, timeout, and redirect
// policy while refusing to resend credential-bearing Socket.IO POST bodies
// to another host or port, or across a TLS downgrade.
func withSafeRedirects(client *http.Client) *http.Client {
	safe := *client
	previous := safe.CheckRedirect
	safe.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > 0 {
			from := via[0].URL
			if !strings.EqualFold(from.Hostname(), req.URL.Hostname()) || effectivePort(from) != effectivePort(req.URL) {
				return fmt.Errorf("refusing Socket.IO redirect to a different host or port")
			}
			if from.Scheme == "https" && req.URL.Scheme != "https" {
				return fmt.Errorf("refusing HTTPS-to-HTTP Socket.IO redirect")
			}
		}
		if previous != nil {
			return previous(req, via)
		}
		if len(via) >= 10 {
			return fmt.Errorf("stopped after 10 redirects")
		}
		return nil
	}
	return &safe
}

func effectivePort(u *url.URL) string {
	if port := u.Port(); port != "" {
		return port
	}
	switch strings.ToLower(u.Scheme) {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}

func isLoopbackHost(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// signalAck wakes anything waiting on ack id (payload is read from c.acks).
func (c *Client) signalAck(id int) {
	c.stashMu.Lock()
	ch := c.ackCh[id]
	c.stashMu.Unlock()
	if ch != nil {
		select {
		case ch <- []byte{}:
		default:
		}
	}
}

func socketIOParse(base string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("base URL must be http(s)")
	}
	if u.Scheme != "https" && !isLoopbackHost(u.Hostname()) {
		return nil, fmt.Errorf("refusing to send Uptime Kuma credentials over non-loopback %s transport; use HTTPS", u.Scheme)
	}
	q := u.Query()
	q.Set("EIO", "4")
	q.Set("transport", "polling")
	u.RawQuery = q.Encode()
	u.Path = strings.TrimSuffix(u.Path, "/") + "/socket.io/"
	return u, nil
}

const maxResponseBodyBytes = 10 << 20

func readResponseBody(body io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, maxResponseBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxResponseBodyBytes {
		return nil, fmt.Errorf("response body exceeds %d-byte limit", maxResponseBodyBytes)
	}
	return data, nil
}

func (c *Client) do(ctx context.Context, u *url.URL, method string, body io.Reader) ([]byte, error) {
	hc := c.cfg.HTTPClient
	if hc == nil {
		// Kuma's default Engine.IO ping interval is 25s and polling requests
		// may remain open for pingInterval+pingTimeout.
		hc = &http.Client{Timeout: 90 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := readResponseBody(resp.Body)
	if err != nil {
		return nil, err
	}
	if os.Getenv("KUMA_PP_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[kuma-pp-debug] %s %s -> %d (%d bytes)\n", method, redactURL(u), resp.StatusCode, len(data))
	}
	if resp.StatusCode >= 400 {
		return nil, &ProtocolError{Msg: fmt.Sprintf("HTTP %d: %.200s", resp.StatusCode, data)}
	}
	return data, nil
}

// parseEngineIO splits an engine.io payload into its length-prefixed records.
// Records look like "<len>:<packet>"; packet[0] is the packet type and the
// rest is the packet body.
func parseEngineIO(payload []byte) []string {
	var out []string
	for len(payload) > 0 {
		colon := bytes.IndexByte(payload, ':')
		if colon < 0 {
			// Socket.IO polling responses may use the Engine.IO record
			// separator instead of length prefixes (notably Kuma's 40
			// namespace ACK bundled with info/loginRequired events).
			for _, record := range bytes.Split(payload, []byte{0x1e}) {
				if len(record) > 0 {
					out = append(out, string(record))
				}
			}
			break
		}
		validLength := colon > 0
		for _, digit := range payload[:colon] {
			if digit < '0' || digit > '9' {
				validLength = false
				break
			}
		}
		if !validLength {
			for _, record := range bytes.Split(payload, []byte{0x1e}) {
				if len(record) > 0 {
					out = append(out, string(record))
				}
			}
			break
		}
		n, err := strconv.ParseInt(string(payload[:colon]), 10, 64)
		if err != nil || n < 0 || n > int64(len(payload)-colon-1) {
			// A numeric prefix claims framing but is invalid. Reject the
			// remaining payload instead of repeatedly rescanning it or
			// accepting an attacker-selected suffix as a new record.
			break
		}
		end := int(colon) + 1 + int(n)
		out = append(out, string(payload[colon+1:end]))
		payload = payload[end:]
	}
	return out
}

// connect performs the engine.io handshake and the Socket.IO namespace connect.
func (c *Client) connect(ctx context.Context) error {
	c.readerMu.Lock()
	if c.readerCancel != nil {
		c.readerCancel()
		c.readerCancel = nil
	}
	c.readerMu.Unlock()
	c.nsConnected = make(chan struct{})
	c.signalNSOnce = sync.Once{}
	base, err := socketIOParse(c.cfg.BaseURL)
	if err != nil {
		return err
	}

	// engine.io open
	openURL := *base
	data, err := c.do(ctx, &openURL, http.MethodGet, nil)
	if err != nil {
		return fmt.Errorf("handshake failed: %w", err)
	}
	recs := parseEngineIO(data)
	if len(recs) == 0 && len(data) > 1 && data[0] == '0' {
		// Some servers emit the open packet unprefixed when nothing follows.
		recs = append(recs, string(data))
	}
	var opened bool
	for _, rec := range recs {
		if len(rec) > 0 && rec[0] == '0' { // engine.io OPEN
			var hs struct {
				SID string `json:"sid"`
			}
			if err := json.Unmarshal([]byte(rec[1:]), &hs); err != nil || hs.SID == "" {
				return &ProtocolError{Msg: "handshake missing sid"}
			}
			c.sess = &session{
				base: base,
				sid:  hs.SID,
				http: c.cfg.HTTPClient,
			}
			if c.sess.http == nil {
				c.sess.http = &http.Client{Timeout: 90 * time.Second}
			}
			opened = true
		}
	}
	if !opened {
		return &ProtocolError{Msg: fmt.Sprintf("no engine.io open packet in handshake (%d records)", len(recs))}
	}

	// Engine.io v4: the client must keep exactly ONE polling GET open at all
	// times. Start the reader loop, then send the Socket.IO namespace-connect
	// packet ("40") and wait for the server's acknowledgement.
	c.startReader(ctx)
	if _, err := c.do(ctx, c.emitURL(), http.MethodPost, strings.NewReader("40")); err != nil {
		return fmt.Errorf("namespace connect emit failed: %w", err)
	}
	select {
	case <-c.nsConnected:
		return nil
	case <-time.After(20 * time.Second):
		return &ProtocolError{Msg: "timed out waiting for Socket.IO namespace connect"}
	}
}

// startReader launches the single long-poll GET loop that receives all
// server->client packets (acks, pushes, pings). It answers engine.io PINGs
// with PONG and records every packet it sees.
func (c *Client) startReader(ctx context.Context) {
	readerCtx, cancel := context.WithCancel(ctx)
	c.readerMu.Lock()
	if c.readerRunning {
		cancel()
		c.readerMu.Unlock()
		return
	}
	c.readerRunning = true
	c.readerCancel = cancel
	c.readerMu.Unlock()
	go func() {
		defer func() {
			c.readerMu.Lock()
			c.readerRunning = false
			c.readerCancel = nil
			c.readerMu.Unlock()
		}()
		for {
			select {
			case <-readerCtx.Done():
				return
			default:
			}
			data, err := c.do(readerCtx, c.pollURL(), http.MethodGet, nil)
			if err != nil {
				if readerCtx.Err() != nil {
					return
				}
				timer := time.NewTimer(500 * time.Millisecond)
				select {
				case <-timer.C:
				case <-readerCtx.Done():
					timer.Stop()
					return
				}
				continue
			}
			body := strings.TrimSpace(string(data))
			switch body {
			case "":
				continue // empty poll: server had nothing (shouldn't happen with hold-open)
			case "2":
				_, _ = c.do(readerCtx, c.emitURL(), http.MethodPost, strings.NewReader("3")) // PING -> PONG
				continue
			case "40":
				c.signalNSOnce.Do(func() { close(c.nsConnected) })
				continue
			}
			recs := parseEngineIO(data)
			for _, rec := range recs {
				switch {
				case rec == "2":
					_, _ = c.do(readerCtx, c.emitURL(), http.MethodPost, strings.NewReader("3"))
				case rec == "40" || strings.HasPrefix(rec, "40{"):
					c.signalNSOnce.Do(func() { close(c.nsConnected) })
				case strings.HasPrefix(rec, "43"):
					id := 0
					rest := rec[2:]
					i := 0
					for i < len(rest) && rest[i] >= '0' && rest[i] <= '9' {
						id = id*10 + int(rest[i]-'0')
						i++
					}
					if id > 0 {
						c.stashMu.Lock()
						if c.acks == nil {
							c.acks = map[int][]byte{}
						}
						c.acks[id] = []byte(rest[i:])
						c.stashMu.Unlock()
						c.signalAck(id)
					}
				default:
					if len(rec) > 1 && rec[0] == '4' && rec[1] == '2' {
						c.routeEvent(readerCtx, []byte(rec[2:]))
					}
				}
			}
		}
	}()
}

func redactURL(u *url.URL) string {
	copy := *u
	if copy.User != nil {
		copy.User = url.User("REDACTED")
	}
	copy.RawQuery = ""
	copy.ForceQuery = false
	copy.Fragment = ""
	return copy.String()
}

func (c *Client) pollURL() *url.URL {
	u := *c.sess.base
	q := u.Query()
	q.Set("sid", c.sess.sid)
	q.Set("t", strconv.FormatInt(time.Now().UnixNano(), 36)+strconv.Itoa(int(time.Now().UnixNano()%1000)))
	u.RawQuery = q.Encode()
	return &u
}

func (c *Client) emitURL() *url.URL {
	u := *c.sess.base
	q := u.Query()
	q.Set("sid", c.sess.sid)
	u.RawQuery = q.Encode()
	return &u
}

func hasRecord(data []byte, t byte) bool {
	for _, b := range data {
		if b == ':' {
			continue
		}
		if b == t {
			return true
		}
	}
	return false
}

func hasNSConnect(recs []string) bool {
	for _, r := range recs {
		if r == "40" {
			return true
		}
	}
	return false
}

// login authenticates with username/password and stores the returned JWT.
func (c *Client) login(ctx context.Context) error {
	c.authMu.Lock()
	defer c.authMu.Unlock()

	c.sess.mu.Lock()
	c.sess.nextID++
	id := c.sess.nextID
	c.sess.mu.Unlock()

	frame := fmt.Sprintf(`42%d["login",{"username":%s,"password":%s,"token":""}]`, id,
		jsonString(c.cfg.Username), jsonString(c.cfg.Password))
	if _, err := c.do(ctx, c.emitURL(), http.MethodPost, strings.NewReader(frame)); err != nil {
		return fmt.Errorf("login emit failed: %w", err)
	}

	ack, err := c.pollForAck(ctx, id, 90*time.Second)
	if err != nil {
		return err
	}
	resp, perr := decodeAckArgs(ack)
	if perr != nil {
		return perr
	}
	if !resp.OK {
		msg := resp.Message
		if msg == "" {
			msg = "rejected"
		}
		return &AuthError{Msg: msg}
	}
	c.jwt = resp.Token
	c.authd = true
	return nil
}

// EnsureConnected connects and logs in if not already authenticated.
func (c *Client) EnsureConnected(ctx context.Context) error {
	c.connMu.Lock()
	defer c.connMu.Unlock()
	c.authMu.Lock()
	authd := c.authd
	c.authMu.Unlock()
	if authd {
		return nil
	}
	if err := c.connect(ctx); err != nil {
		return err
	}
	return c.login(ctx)
}

// decodeAckArgs parses a Socket.IO ACK argument array like
// [{"ok":true,"token":"..."}] and returns its first element.
func decodeAckArgs(ack []byte) (*ackArgs, error) {
	var arr []*ackArgs
	if err := json.Unmarshal(ack, &arr); err == nil {
		if len(arr) == 0 || arr[0] == nil {
			return nil, &ProtocolError{Msg: fmt.Sprintf("empty ack %s", ack)}
		}
		return arr[0], nil
	}
	var one ackArgs
	if err := json.Unmarshal(ack, &one); err != nil {
		return nil, &ProtocolError{Msg: fmt.Sprintf("unparseable ack %s", ack)}
	}
	return &one, nil
}

type ackArgs struct {
	OK      bool   `json:"ok"`
	Token   string `json:"token"`
	Message string `json:"message"`
}

// emitAck emits event/data and returns the raw ack JSON argument.
func (c *Client) emitAck(ctx context.Context, event string, data any) ([]byte, error) {
	c.sess.mu.Lock()
	c.sess.nextID++
	id := c.sess.nextID
	c.sess.mu.Unlock()

	var arg string
	if data != nil {
		b, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		arg = string(b)
	}
	frame := "42" + strconv.Itoa(id) + "[" + jsonString(event)
	if arg != "" {
		frame += "," + arg
	}
	frame += "]"
	if _, err := c.do(ctx, c.emitURL(), http.MethodPost, strings.NewReader(frame)); err != nil {
		return nil, fmt.Errorf("emit %s failed: %w", event, err)
	}
	return c.pollForAck(ctx, id, 90*time.Second)
}

func (c *Client) emitNoAck(ctx context.Context, event string, data any) error {
	arg := ""
	if data != nil {
		b, err := json.Marshal(data)
		if err != nil {
			return err
		}
		arg = "," + string(b)
	}
	frame := "42[" + jsonString(event) + arg + "]"
	_, err := c.do(ctx, c.emitURL(), http.MethodPost, strings.NewReader(frame))
	return err
}

// pollForAck polls until the ack for id arrives. Server-initiated pushes seen
// along the way are buffered by callWithPushFallback consumers via the queue.
func (c *Client) pollForAck(ctx context.Context, wantID int, wait time.Duration) ([]byte, error) {
	// startReader is the sole owner of long-poll GETs. Register the waiter
	// before checking the mailbox so an ACK cannot arrive between the check and
	// registration.
	c.stashMu.Lock()
	ch := make(chan []byte, 1)
	c.ackCh[wantID] = ch
	if ack, ok := c.acks[wantID]; ok {
		delete(c.acks, wantID)
		delete(c.ackCh, wantID)
		c.stashMu.Unlock()
		return ack, nil
	}
	c.stashMu.Unlock()
	defer func() {
		c.stashMu.Lock()
		delete(c.ackCh, wantID)
		c.stashMu.Unlock()
	}()

	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case ack := <-ch:
		c.stashMu.Lock()
		ack = append([]byte(nil), c.acks[wantID]...)
		delete(c.acks, wantID)
		c.stashMu.Unlock()
		if len(ack) > 0 {
			return ack, nil
		}
		return nil, &ProtocolError{Msg: fmt.Sprintf("empty ack %d", wantID)}
	case <-timer.C:
		return nil, &ProtocolError{Msg: fmt.Sprintf("timed out waiting for ack %d", wantID)}
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Call emits an event and requires ok:true in the ack.
func (c *Client) Call(ctx context.Context, event string, data any) error {
	ack, err := c.emitAck(ctx, event, data)
	if err != nil {
		return err
	}
	return checkOK(ack)
}

// CallRaw emits an event and returns the raw ack JSON argument.
func (c *Client) CallRaw(ctx context.Context, event string, data any) ([]byte, error) {
	return c.emitAck(ctx, event, data)
}

// CallWithPushFallback handles the read pattern where the ack is bare
// {"ok":true} and the real payload rides a paired push event (Kuma v2 style:
// getMonitorList -> monitorList). It waits up to wait for the push after the
// bare ack, then fails.
func (c *Client) CallWithPushFallback(ctx context.Context, event string, data any, pushEvent string, pushWait time.Duration) (json.RawMessage, error) {
	if event == "getHeartbeats" {
		stream, err := c.beginEventStream(pushEvent)
		if err != nil {
			return nil, err
		}
		defer c.endEventStream(stream)
		if err := c.emitNoAck(ctx, event, data); err != nil {
			return nil, fmt.Errorf("emit %s failed: %w", event, err)
		}
		return c.collectStashedEvents(ctx, pushEvent, pushWait, stream)
	}
	ack, err := c.emitAck(ctx, event, data)
	if err != nil {
		return nil, err
	}

	// Acks are argument arrays like [{"ok":true,...}] (Kuma sometimes sends a
	// bare object). Keep the raw fields so we can tell a bare ok from an
	// ack that already carries the payload.
	var arr []map[string]json.RawMessage
	var obj map[string]json.RawMessage
	if json.Unmarshal(ack, &arr) == nil && len(arr) > 0 && arr[0] != nil {
		obj = arr[0]
	} else if json.Unmarshal(ack, &obj) != nil || obj == nil {
		return nil, &ProtocolError{Msg: fmt.Sprintf("unparseable ack %s", ack)}
	}

	okRaw, hasOK := obj["ok"]
	if hasOK {
		var ok bool
		if json.Unmarshal(okRaw, &ok) == nil && !ok {
			msg := ""
			if m, has := obj["message"]; has {
				json.Unmarshal(m, &msg)
			}
			if msg == "" {
				msg = "server returned ok:false"
			}
			return nil, fmt.Errorf("%s", msg)
		}
	}

	// Ack carries the payload alongside ok -> hand it straight back.
	if len(obj) > 1 || !hasOK {
		return json.RawMessage(ack), nil
	}

	// Bare {"ok":true}: wait for the paired push event.
	if raw := c.takeStashedEvent(pushEvent); raw != nil {
		return raw, nil
	}
	deadline := time.NewTimer(pushWait)
	defer deadline.Stop()
	for {
		if raw := c.takeStashedEvent(pushEvent); raw != nil {
			return raw, nil
		}
		select {
		case <-deadline.C:
			return nil, &ProtocolError{Msg: fmt.Sprintf("ack was bare ok:true and no %q push arrived within %s", pushEvent, pushWait)}
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
}

// streamIdleGap is how long the collector waits for a further push after the
// last one arrived. Kuma emits one heartbeatList record per monitor, so the
// stream is a burst rather than a single frame: returning on the first record
// truncates the result, while waiting for the full deadline makes every read
// pay the worst case. Returning once the burst goes quiet does neither.
const streamIdleGap = 500 * time.Millisecond

// A sustained stream fails explicitly instead of growing a result without
// limit or returning a partial heartbeat report as if it were complete.
const maxCollectedHeartbeatBytes = 20 << 20

func (c *Client) collectStashedEvents(ctx context.Context, name string, wait time.Duration, stream *eventStream) (json.RawMessage, error) {
	deadline := time.Now().Add(wait)
	var payloads []json.RawMessage
	var lastEvent time.Time
	collectedBytes := 0
	appendPayload := func(raw json.RawMessage) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if time.Now().After(deadline) {
			return &ProtocolError{Msg: fmt.Sprintf("timed out collecting %q heartbeat stream", name)}
		}
		if len(raw) > maxCollectedHeartbeatBytes-collectedBytes {
			return &ProtocolError{Msg: fmt.Sprintf("%q heartbeat stream exceeds %d bytes", name, maxCollectedHeartbeatBytes)}
		}
		payloads = append(payloads, raw)
		collectedBytes += len(raw)
		lastEvent = time.Now()
		return nil
	}
	appendBatch := func(batch [][]byte) error {
		for _, p := range batch {
			if err := appendPayload(payloadForEvent(p, name)); err != nil {
				return err
			}
		}
		return nil
	}
	for {
		for {
			raw := c.takeStashedEvent(name)
			if raw == nil {
				break
			}
			if err := appendPayload(raw); err != nil {
				return nil, err
			}
		}
		if err := appendBatch(c.drainEventStream(stream)); err != nil {
			return nil, err
		}
		settled := len(payloads) > 0 && time.Since(lastEvent) >= streamIdleGap
		if settled {
			late, finished := c.finishEventStream(stream)
			if err := appendBatch(late); err != nil {
				return nil, err
			}
			if finished {
				return joinStreamPayloads(name, payloads)
			}
			continue
		}
		if time.Now().After(deadline) {
			if len(payloads) == 0 {
				return joinStreamPayloads(name, payloads)
			}
			return nil, &ProtocolError{Msg: fmt.Sprintf("timed out collecting %q heartbeat stream", name)}
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// joinStreamPayloads renders a collected burst. A single record is returned
// as-is so existing single-payload decoding is unchanged; multiple records are
// wrapped in an array for the caller to merge.
func joinStreamPayloads(name string, payloads []json.RawMessage) (json.RawMessage, error) {
	switch len(payloads) {
	case 0:
		return nil, &ProtocolError{Msg: fmt.Sprintf("timed out waiting for %q", name)}
	case 1:
		return payloads[0], nil
	default:
		return json.Marshal(payloads)
	}
}

// pollOnce performs one poll and returns all "42[...]" event records found.
func (c *Client) pollOnce(ctx context.Context) ([][]byte, error) {
	data, err := c.do(ctx, c.pollURL(), http.MethodGet, nil)
	if err != nil {
		return nil, err
	}
	var out [][]byte
	for _, rec := range parseEngineIO(data) {
		if len(rec) > 2 && rec[0] == '4' && rec[1] == '2' {
			out = append(out, []byte(rec[2:]))
		}
	}
	return out, nil
}

func checkOK(ack []byte) error {
	args, err := decodeAckArgs(ack)
	if err != nil {
		return err
	}
	if !args.OK {
		msg := args.Message
		if msg == "" {
			msg = "server returned ok:false"
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// SessionID exposes the negotiated engine.io session id (tests/diagnostics).
func (c *Client) SessionID() string {
	if c.sess == nil {
		return ""
	}
	return c.sess.sid
}

// Authenticated reports whether login has completed.
func (c *Client) Authenticated() bool {
	c.authMu.Lock()
	defer c.authMu.Unlock()
	return c.authd
}

// convenience alias used by tests
func (c *Client) callWithPushFallback(ctx context.Context, event string, data any, pushEvent string, pushWait time.Duration) (json.RawMessage, error) {
	return c.CallWithPushFallback(ctx, event, data, pushEvent, pushWait)
}
