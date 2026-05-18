// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/spf13/cobra"

// newSyncApiCmd exposes the generator-emitted public-API sync command
// under its own name ("sync-api") so we can claim the top-level "sync"
// for the cache-hydration command. Both surfaces stay available.
func newSyncApiCmd(flags *rootFlags) *cobra.Command {
	cmd := newSyncCmd(flags)
	cmd.Use = "sync-api"
	cmd.Short = "Sync the Granola PUBLIC API (~3 endpoints) into the local store"
	cmd.Long = `Calls Granola's public REST API (using GRANOLA_API_KEY) to sync the
narrow set of resources the public spec exposes — notes list/get and
folders. For everything else (transcripts, panels, recipes, chat,
attendees) use the top-level 'sync' command which reads the desktop
app's cache file.`
	return cmd
}
