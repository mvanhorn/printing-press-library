package client

import (
	"errors"
	"fmt"
	"strings"
)

// ErrWriteNotConfirmed is returned when the CLI is about to send a
// non-GET request without --yes. The CLI maps it to exit 2 (usage).
var ErrWriteNotConfirmed = errors.New("write not confirmed")

// WriteNotConfirmedError carries the refused request line.
type WriteNotConfirmedError struct {
	Method string
	Path   string
}

func (e *WriteNotConfirmedError) Error() string {
	return fmt.Sprintf("refusing to send %s %s without --yes (preview it first with --dry-run)", e.Method, e.Path)
}

func (e *WriteNotConfirmedError) Is(target error) bool { return target == ErrWriteNotConfirmed }

// SetWriteGate makes the client refuse non-GET requests unless confirmed
// (--yes). Clients that never call it (the MCP binary has its own confirm
// flow) are unaffected. --dry-run never reaches the wire, so it is exempt.
func (c *Client) SetWriteGate(confirmed bool) {
	if c == nil {
		return
	}
	c.writeGateOn = true
	c.writeConfirmed = confirmed
}

// writeGateCheck returns a refusal for an unconfirmed non-GET request.
// POST /scopes (tokenScopes) is read-only and exempt. OAuth token endpoints
// use a separate http client and never pass through here.
func (c *Client) writeGateCheck(method, path string) error {
	if c == nil || !c.writeGateOn || c.writeConfirmed || c.DryRun {
		return nil
	}
	if !isMutatingVerb(method) {
		return nil
	}
	if method == "POST" && strings.TrimRight(strings.SplitN(path, "?", 2)[0], "/") == "/scopes" {
		return nil
	}
	return &WriteNotConfirmedError{Method: method, Path: path}
}
