// Package bridge is the Go side of the JSON-RPC link to bridge/mt5_bridge.py.
//
// One Bridge corresponds to one running python subprocess. Lifecycle:
//
//	b, err := bridge.New(bridge.Options{})
//	defer b.Close()
//	var acc bridge.AccountInfo
//	if err := b.Call("account_info", nil, &acc); err != nil { ... }
//
// Each mt5-pp-cli invocation typically spawns one bridge, executes a handful of
// calls, and tears it down on exit. Long-running operations (sync, replay,
// watch) keep the same bridge alive for the whole session.
//
// Protocol (line-delimited JSON-RPC over stdio):
//
//	→ {"id":1,"method":"account_info","params":{}}
//	← {"id":1,"result":{...}}
//	← {"id":1,"error":{"code":"NOT_LOGGED_IN","message":"..."}}
package bridge

import (
	"bufio"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

//go:embed mt5_bridge.py
var bridgeScript []byte

// ── sentinel errors ──────────────────────────────────────────────────────────
//
// Callers pattern-match on these to map to the right cli.ExitErr.Code.

var (
	ErrNotLoggedIn    = errors.New("mt5: not logged in")
	ErrTerminalDown   = errors.New("mt5: terminal not running or not reachable")
	ErrBrokerRejected = errors.New("mt5: broker rejected order")
	ErrRateLimited    = errors.New("mt5: rate limited by broker")
	ErrPythonMissing  = errors.New("python: not found on PATH")
	ErrMT5PkgMissing  = errors.New("python: MetaTrader5 package not installed")
	ErrInvalidParams  = errors.New("bridge: invalid params")
	ErrBridgeInternal = errors.New("bridge: internal error")
)

// errFromCode maps a bridge error code string to a Go sentinel.
func errFromCode(code, message string) error {
	switch code {
	case "NOT_LOGGED_IN":
		return fmt.Errorf("%w: %s", ErrNotLoggedIn, message)
	case "TERMINAL_DOWN":
		return fmt.Errorf("%w: %s", ErrTerminalDown, message)
	case "BROKER_REJECTED":
		return fmt.Errorf("%w: %s", ErrBrokerRejected, message)
	case "RATE_LIMITED":
		return fmt.Errorf("%w: %s", ErrRateLimited, message)
	case "PYTHON_MT5_MISSING":
		return fmt.Errorf("%w: %s", ErrMT5PkgMissing, message)
	case "INVALID_PARAMS":
		return fmt.Errorf("%w: %s", ErrInvalidParams, message)
	default:
		return fmt.Errorf("%w (%s): %s", ErrBridgeInternal, code, message)
	}
}

// ── Bridge ───────────────────────────────────────────────────────────────────

type Options struct {
	// PythonPath overrides interpreter resolution. Default: auto-detect.
	PythonPath string
	// ScriptPath overrides where the bridge script is read from. When empty,
	// the embedded script is written to a temp file and used. Setting this is
	// only useful when iterating on the Python script during development.
	ScriptPath string
	// Stderr receives the subprocess's stderr. nil discards it.
	Stderr io.Writer
	// CallTimeout bounds a single Call. 0 = no per-call timeout.
	CallTimeout time.Duration
}

type Bridge struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  *bufio.Reader
	mu      sync.Mutex // serializes writes; the protocol is line-oriented
	closed  atomic.Bool
	nextID  atomic.Int64
	timeout time.Duration

	// broken is set when we detect protocol desync (timeout, ID mismatch).
	// Once broken, every subsequent Call errors fast — the caller must Close
	// the bridge and start a new one rather than risk consuming a late
	// response intended for the timed-out call.
	broken atomic.Bool
}

type request struct {
	ID     int64  `json:"id"`
	Method string `json:"method"`
	Params any    `json:"params,omitempty"`
}

type response struct {
	ID     int64           `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// New spawns a python subprocess and returns a Bridge ready for Call().
func New(opts Options) (*Bridge, error) {
	py := opts.PythonPath
	if py == "" {
		p, err := FindPython()
		if err != nil {
			return nil, err
		}
		py = p
	}
	script := opts.ScriptPath
	if script == "" {
		p, err := materializeScript()
		if err != nil {
			return nil, err
		}
		script = p
	}

	cmd := exec.Command(py, script)
	cmd.Stderr = opts.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("bridge stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("bridge stdout: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("spawn python bridge (%s %s): %w", py, script, err)
	}

	return &Bridge{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  bufio.NewReader(stdout),
		timeout: opts.CallTimeout,
	}, nil
}

// Close shuts the bridge down. Safe to call multiple times.
func (b *Bridge) Close() error {
	if !b.closed.CompareAndSwap(false, true) {
		return nil
	}
	_ = b.stdin.Close()
	done := make(chan error, 1)
	go func() { done <- b.cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-time.After(3 * time.Second):
		_ = b.cmd.Process.Kill()
		<-done
		return errors.New("bridge: timed out, killed subprocess")
	}
}

// Call invokes a named method on the bridge and decodes the result into out.
// out may be nil to discard the result. Pass params=nil for handlers that take
// no parameters; otherwise any JSON-encodable value works.
func (b *Bridge) Call(method string, params any, out any) error {
	if b.closed.Load() {
		return errors.New("bridge: closed")
	}
	if b.broken.Load() {
		return errors.New("bridge: marked broken after prior timeout or protocol desync — close this bridge and open a fresh one")
	}
	id := b.nextID.Add(1)
	req := request{ID: id, Method: method, Params: params}

	buf, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("bridge marshal: %w", err)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if _, err := b.stdin.Write(append(buf, '\n')); err != nil {
		return fmt.Errorf("bridge write: %w", err)
	}

	type readResult struct {
		line []byte
		err  error
	}
	ch := make(chan readResult, 1)
	go func() {
		line, err := b.stdout.ReadBytes('\n')
		ch <- readResult{line, err}
	}()

	var line []byte
	if b.timeout > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), b.timeout)
		defer cancel()
		select {
		case r := <-ch:
			line, err = r.line, r.err
		case <-ctx.Done():
			// CRITICAL: the reader goroutine is still parked on ReadBytes.
			// If we just returned here the next Call would spawn a second
			// reader goroutine, and Python's late response to THIS call
			// could be consumed by that goroutine — attributed to the
			// wrong request id. Mark the bridge broken so every subsequent
			// Call fails fast; caller must Close() and open a fresh bridge.
			b.broken.Store(true)
			return fmt.Errorf("bridge: call %s timed out after %s (bridge marked broken)", method, b.timeout)
		}
	} else {
		r := <-ch
		line, err = r.line, r.err
	}
	if err != nil && len(line) == 0 {
		return fmt.Errorf("bridge read (%s): %w", method, err)
	}

	var resp response
	if err := json.Unmarshal(line, &resp); err != nil {
		return fmt.Errorf("bridge: unparseable response for %s: %w (raw: %s)", method, err, line)
	}
	// Belt-and-braces: also catches the case where a previous response
	// somehow leaked through (e.g., bug in this file's locking).
	if resp.ID != id {
		b.broken.Store(true)
		return fmt.Errorf("bridge: response id %d does not match request id %d for %s (protocol desync; bridge marked broken)", resp.ID, id, method)
	}
	if resp.Error != nil {
		return errFromCode(resp.Error.Code, resp.Error.Message)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(resp.Result, out); err != nil {
		return fmt.Errorf("bridge: decode result for %s: %w (raw: %s)", method, err, resp.Result)
	}
	return nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

// FindPython resolves the Python interpreter we will spawn. On Windows we
// prefer the `py` launcher (handles versioned installs); elsewhere we try
// `python3` then `python`.
func FindPython() (string, error) {
	candidates := []string{"py", "python3", "python"}
	if runtime.GOOS != "windows" {
		candidates = []string{"python3", "python"}
	}
	for _, c := range candidates {
		if p, err := exec.LookPath(c); err == nil {
			return p, nil
		}
	}
	return "", ErrPythonMissing
}

// materializeScript writes the embedded bridge script to a content-addressed
// path so different binaries embedding different scripts coexist without
// aliasing. Stable per-content path means re-runs of the same binary reuse
// the same file (the user can inspect it, attach a debugger, etc.).
//
// The original implementation used a fixed filename and a size-only freshness
// check, which had two failure modes:
//   - Two binaries with different embedded scripts of equal byte length would
//     run each other's script (silent wrong-version).
//   - Concurrent first-run binaries racing on os.WriteFile could leave the
//     file partially written for the loser; the winner's Python would crash
//     mid-import.
//
// Fix: name the file by SHA-256 prefix of the script body, write to a unique
// temp file then rename atomically. Same-content paths skip the write entirely.
func materializeScript() (string, error) {
	dir := filepath.Join(os.TempDir(), "mt5-pp-cli")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	sum := sha256.Sum256(bridgeScript)
	name := fmt.Sprintf("mt5_bridge_%x.py", sum[:8])
	path := filepath.Join(dir, name)

	// Skip the write if a same-content file is already there. Content-
	// addressed filename means matching st.Size() == len() AND matching
	// the body — equal size alone is no longer the gate.
	if st, err := os.Stat(path); err == nil && st.Size() == int64(len(bridgeScript)) {
		return path, nil
	}

	// Write to a unique temp path, then rename. Atomic rename on POSIX
	// AND on Windows (when the destination doesn't exist; if it does,
	// os.Rename on Windows uses MOVEFILE_REPLACE_EXISTING via
	// MoveFileExW — also atomic at the filesystem level).
	tmp, err := os.CreateTemp(dir, name+".tmp-*")
	if err != nil {
		return "", fmt.Errorf("create temp bridge script: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(bridgeScript); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("write temp bridge script: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("close temp bridge script: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		// On Windows two processes racing CreateTemp+Rename can collide; if
		// the destination ended up correct (winner already wrote it), accept.
		if st, sterr := os.Stat(path); sterr == nil && st.Size() == int64(len(bridgeScript)) {
			os.Remove(tmpPath)
			return path, nil
		}
		os.Remove(tmpPath)
		return "", fmt.Errorf("rename bridge script: %w", err)
	}
	return path, nil
}

// SelfTest is a cheap diagnostic used by `mt5-pp-cli doctor`. It reports what we
// can determine without actually spawning the bridge.
func SelfTest() string {
	py, err := FindPython()
	if err != nil {
		return "python NOT found — " + err.Error()
	}
	script, err := materializeScript()
	if err != nil {
		return fmt.Sprintf("python OK (%s) but bridge script: %s", py, err)
	}
	suffix := ""
	if runtime.GOOS != "windows" {
		suffix = " (non-Windows host: live commands disabled, mirror-only commands work)"
	}
	return fmt.Sprintf("python=%s bridge=%s%s", py, script, suffix)
}
