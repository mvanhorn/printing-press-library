//go:build !unix

// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.

package client

// lockSessionCache is a no-op where flock is unavailable (including the
// windows goreleaser targets). The session file is a derived cache; a lost
// concurrent write only forces the next lookup.
func lockSessionCache(string) func() { return func() {} }
