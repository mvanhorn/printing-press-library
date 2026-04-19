// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package linkedin

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

// DefaultCallTimeout bounds any single tool call. The upstream scraper is
// Selenium-backed and sometimes slow, but if we haven't heard back in 30s
// something is wedged.
const DefaultCallTimeout = 30 * time.Second

// Client is a single long-lived MCP stdio session. Create once per CLI run
// with NewClient, call Initialize, then issue one or more CallTool requests,
// then Close. The zero value is not usable.
type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	enc *json.Encoder
	dec *json.Decoder

	writeMu sync.Mutex // serialize writes to stdin
	nextID  int64

	// Pending request tracking so concurrent callers can wait on the reader.
	pendMu  sync.Mutex
	pending map[int64]chan *JSONRPCResponse

	stderrBuf stderrRing

	done chan struct{}

	closed atomic.Bool
	initOK atomic.Bool
}

// Options configures a new client.
type Options struct {
	// Command is the executable to run. If empty, defaults to "uvx".
	Command string
	// Args are the subprocess arguments. If Command is "uvx" (the default)
	// and Args is nil, defaults to ["linkedin-scraper-mcp@latest"].
	Args []string
	// Env overrides the subprocess environment. If nil, inherits os.Environ().
	Env []string
	// ClientInfo is surfaced in the initialize request. Defaults to
	// {Name: "contact-goat-pp-cli", Version: "0.1.0"}.
	ClientInfo Implementation
	// StderrTee, if non-nil, receives a copy of everything the subprocess
	// writes to stderr. Useful for --verbose flags.
	StderrTee io.Writer
}

// NewClient spawns the MCP subprocess and wires up the JSON-RPC plumbing.
// It does NOT call Initialize for you -- the caller should do that so it can
// surface an actionable error if the subprocess aborts during handshake.
func NewClient(ctx context.Context, opts Options) (*Client, error) {
	if opts.Command == "" {
		opts.Command = "uvx"
	}
	if opts.Command == "uvx" && opts.Args == nil {
		opts.Args = []string{"linkedin-scraper-mcp@latest"}
	}
	if opts.ClientInfo.Name == "" {
		opts.ClientInfo.Name = "contact-goat-pp-cli"
	}
	if opts.ClientInfo.Version == "" {
		opts.ClientInfo.Version = "0.1.0"
	}

	cmd := exec.CommandContext(ctx, opts.Command, opts.Args...)
	if opts.Env != nil {
		cmd.Env = opts.Env
	} else {
		cmd.Env = os.Environ()
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting %s: %w", opts.Command, err)
	}

	c := &Client{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  stdout,
		stderr:  stderr,
		enc:     json.NewEncoder(stdin),
		dec:     json.NewDecoder(stdout),
		pending: make(map[int64]chan *JSONRPCResponse),
		done:    make(chan struct{}),
	}
	c.stderrBuf.cap = 64 * 1024 // keep last 64KB of stderr for error messages

	go c.readLoop()
	go c.pumpStderr(opts.StderrTee)

	return c, nil
}

// Initialize performs the MCP handshake. Must be called before CallTool.
func (c *Client) Initialize(ctx context.Context, clientInfo Implementation) (*InitializeResult, error) {
	if clientInfo.Name == "" {
		clientInfo.Name = "contact-goat-pp-cli"
	}
	req := InitializeRequest{
		ProtocolVersion: ProtocolVersion,
		Capabilities:    ClientCapabilities{},
		ClientInfo:      clientInfo,
	}
	var result InitializeResult
	if err := c.call(ctx, "initialize", req, &result); err != nil {
		return nil, err
	}
	// Per MCP, the client must send `notifications/initialized` after the
	// server responds to `initialize`.
	if err := c.notify("notifications/initialized", nil); err != nil {
		return nil, fmt.Errorf("sending initialized notification: %w", err)
	}
	c.initOK.Store(true)
	return &result, nil
}

// ListTools calls `tools/list`.
func (c *Client) ListTools(ctx context.Context) (*ListToolsResult, error) {
	if !c.initOK.Load() {
		return nil, errors.New("client not initialized: call Initialize first")
	}
	var result ListToolsResult
	if err := c.call(ctx, "tools/list", struct{}{}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CallTool invokes `tools/call` with a per-call timeout of DefaultCallTimeout
// (unless ctx already has a tighter deadline). The raw result is returned as
// MCP content blocks; callers that want the text payload can use
// result.Content[0].Text or TextPayload.
func (c *Client) CallTool(ctx context.Context, name string, args map[string]any) (*CallToolResult, error) {
	if !c.initOK.Load() {
		return nil, errors.New("client not initialized: call Initialize first")
	}
	if args == nil {
		args = map[string]any{}
	}
	deadline := time.Now().Add(DefaultCallTimeout)
	if d, ok := ctx.Deadline(); !ok || d.After(deadline) {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, deadline)
		defer cancel()
	}
	var result CallToolResult
	if err := c.call(ctx, "tools/call", CallToolRequest{Name: name, Arguments: args}, &result); err != nil {
		return nil, err
	}
	if result.IsError {
		return &result, fmt.Errorf("tool %q returned error: %s", name, TextPayload(&result))
	}
	return &result, nil
}

// TextPayload concatenates every text content block into a single string.
func TextPayload(r *CallToolResult) string {
	if r == nil {
		return ""
	}
	var out string
	for _, b := range r.Content {
		if b.Type == "text" {
			out += b.Text
		}
	}
	return out
}

// Close terminates the subprocess. Safe to call multiple times.
func (c *Client) Close() error {
	if c.closed.Swap(true) {
		return nil
	}
	// Politely close stdin; the server should exit on EOF.
	_ = c.stdin.Close()

	// Wait up to 2s for a clean exit, then kill.
	waitErr := make(chan error, 1)
	go func() { waitErr <- c.cmd.Wait() }()
	select {
	case err := <-waitErr:
		close(c.done)
		// Wake any pending callers.
		c.drainPending(err)
		return nil
	case <-time.After(2 * time.Second):
		_ = c.cmd.Process.Kill()
		<-waitErr
		close(c.done)
		c.drainPending(errors.New("subprocess killed during close"))
		return nil
	}
}

// StderrTail returns a copy of the most recent stderr output (up to the
// ring-buffer capacity). Useful for surfacing crash causes.
func (c *Client) StderrTail() string {
	return c.stderrBuf.String()
}

// ---------------------------------------------------------------------------
// internals
// ---------------------------------------------------------------------------

func (c *Client) call(ctx context.Context, method string, params any, out any) error {
	id := atomic.AddInt64(&c.nextID, 1)
	ch := make(chan *JSONRPCResponse, 1)

	c.pendMu.Lock()
	c.pending[id] = ch
	c.pendMu.Unlock()
	defer func() {
		c.pendMu.Lock()
		delete(c.pending, id)
		c.pendMu.Unlock()
	}()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
	if err := c.write(req); err != nil {
		return fmt.Errorf("write %s: %w", method, err)
	}

	select {
	case <-ctx.Done():
		return fmt.Errorf("%s timed out: %w (stderr tail: %s)", method, ctx.Err(), c.StderrTail())
	case <-c.done:
		return fmt.Errorf("%s: subprocess exited (stderr tail: %s)", method, c.StderrTail())
	case resp := <-ch:
		if resp.Error != nil {
			return fmt.Errorf("%s: jsonrpc error %d: %s", method, resp.Error.Code, resp.Error.Message)
		}
		if out == nil {
			return nil
		}
		if len(resp.Result) == 0 {
			return nil
		}
		if err := json.Unmarshal(resp.Result, out); err != nil {
			return fmt.Errorf("%s: decode result: %w", method, err)
		}
		return nil
	}
}

func (c *Client) notify(method string, params any) error {
	// Notifications have no id.
	req := JSONRPCRequest{JSONRPC: "2.0", Method: method, Params: params}
	return c.write(req)
}

func (c *Client) write(v any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.enc.Encode(v)
}

func (c *Client) readLoop() {
	for {
		var resp JSONRPCResponse
		if err := c.dec.Decode(&resp); err != nil {
			c.drainPending(fmt.Errorf("reader: %w (stderr tail: %s)", err, c.StderrTail()))
			return
		}
		// ID can be float64 (json default) or string. Notifications from the
		// server (no id) are dropped; we don't speak them upstream yet.
		id, ok := extractID(resp.ID)
		if !ok {
			continue
		}
		c.pendMu.Lock()
		ch, found := c.pending[id]
		c.pendMu.Unlock()
		if found {
			ch <- &resp
		}
	}
}

func extractID(v any) (int64, bool) {
	switch x := v.(type) {
	case float64:
		return int64(x), true
	case int64:
		return x, true
	case int:
		return int64(x), true
	case json.Number:
		i, err := x.Int64()
		if err != nil {
			return 0, false
		}
		return i, true
	default:
		return 0, false
	}
}

func (c *Client) drainPending(err error) {
	c.pendMu.Lock()
	defer c.pendMu.Unlock()
	for id, ch := range c.pending {
		select {
		case ch <- &JSONRPCResponse{Error: &JSONRPCError{Code: -32000, Message: err.Error()}}:
		default:
		}
		delete(c.pending, id)
	}
}

func (c *Client) pumpStderr(tee io.Writer) {
	scanner := bufio.NewScanner(c.stderr)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		c.stderrBuf.Write([]byte(line + "\n"))
		if tee != nil {
			fmt.Fprintln(tee, line)
		}
	}
}

// stderrRing is a thread-safe bounded ring buffer for recent stderr output.
type stderrRing struct {
	mu  sync.Mutex
	buf []byte
	cap int
}

func (r *stderrRing) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cap <= 0 {
		r.cap = 64 * 1024
	}
	r.buf = append(r.buf, p...)
	if len(r.buf) > r.cap {
		r.buf = r.buf[len(r.buf)-r.cap:]
	}
	return len(p), nil
}

func (r *stderrRing) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return string(r.buf)
}
