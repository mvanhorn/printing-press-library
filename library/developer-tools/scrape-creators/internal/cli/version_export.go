// Copyright 2026 Adrian Horning and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored (preserved on reprint): exposes the ledger-owned runtime version
// declared in root.go so non-CLI entrypoints (the MCP server) advertise the same
// version the release automation stamps, without declaring a second version var
// that the library's release-ledger guard would reject.

package cli

// Version reports the runtime version stamped into this build. The backing
// var lives in root.go and is owned by the post-merge release workflow and
// the GoReleaser -X flag; nothing else may declare or assign it.
func Version() string {
	return version
}
