package cliutil

import (
	"fmt"
	"time"
)

// Hand-written guards shared by the unknown-caller CLI commands and the MCP code-orchestration executor (board #35, parity rounds 2 and 3). Not generated: a reprint must keep them.

// ErrCallDerivedFields is returned when a create-contact request that carries callId also carries source or assignedTo. The server
// derives the lead's source ("Inbound Call") and its owner (the rep who took the call), and keeps an explicit assignedTo, so a
// client value could silently assign the lead to the wrong rep.
var ErrCallDerivedFields = fmt.Errorf("--call-id cannot be combined with source or assignedTo (--source, --assigned-to, or those keys in --stdin JSON): for a lead created from a call the server derives both (source 'Inbound Call', owner = the rep who took the call). Omit them")

// CheckCallDerivedFields refuses the combination after flags and stdin JSON are merged into the body.
func CheckCallDerivedFields(body map[string]any) error {
	if _, hasCall := body["callId"]; !hasCall {
		return nil
	}
	_, hasSource := body["source"]
	_, hasOwner := body["assignedTo"]
	if hasSource || hasOwner {
		return ErrCallDerivedFields
	}
	return nil
}

// CheckRFC3339Instant requires an RFC 3339 timestamp with Z or an explicit offset and a real calendar date. An offset-less value
// would be read in the runtime's or server's timezone and shift the counter window.
func CheckRFC3339Instant(flag, value string) error {
	if _, err := time.Parse(time.RFC3339Nano, value); err != nil {
		return fmt.Errorf("--%s must be an RFC 3339 instant with Z or an explicit offset, such as 2026-10-15T07:00:00Z or 2026-10-15T00:00:00-07:00 (got %q): %v", flag, value, err)
	}
	return nil
}
