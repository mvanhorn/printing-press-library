// Copyright 2026 drdriftwood. Licensed under Apache-2.0. See LICENSE.

// Package obsidian provides runtime detection helpers for the Obsidian
// desktop app. The Local REST API plugin (when installed and enabled)
// listens on 127.0.0.1:27124, which is a reliable signal that Obsidian
// is running. V1 of obsidian-pp-cli uses this only to vary the staleness
// warning shown by mirror-backed commands; the official `obsidian` CLI
// itself signals "no active vault" with its own non-zero exit when the
// app is closed.
package obsidian

import (
	"net"
	"time"
)

// IsRunning returns true when something is listening on the Local REST
// API plugin's loopback port. False is the safe default: the worst that
// happens is we suggest "run sync" without checking that obsidian can
// reach the vault — and `sync` itself surfaces the underlying error.
func IsRunning() bool {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:27124", 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
