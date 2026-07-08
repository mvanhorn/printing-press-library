// Copyright 2026 David Barbier and contributors. Licensed under Apache-2.0. See LICENSE.
// Restored by hand after a regen merge dropped the generated intents.go while
// tools.go kept its RegisterIntents call (generator merge bug, retro'd).
// The spec declares no MCP intents, so registration is a no-op.

package mcp

import "github.com/mark3labs/mcp-go/server"

// RegisterIntents registers spec-declared MCP intents. This CLI's spec
// declares none.
func RegisterIntents(_ *server.MCPServer) {}
