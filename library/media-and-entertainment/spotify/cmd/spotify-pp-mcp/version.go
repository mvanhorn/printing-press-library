// Copyright 2026 Rob Zehner and contributors. Licensed under Apache-2.0. See LICENSE.

package main

// version is the printed MCP server's version, overridable at build time via
// ldflags. Declared outside main.go so the library's release-ledger guard can
// verify runtime-version stamps without this PR touching a stamped line.
var version = "0.0.0-dev"
