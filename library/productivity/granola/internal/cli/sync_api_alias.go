// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

// newSyncApiCmd exposes the generator-emitted public-API sync command
// under its own name ("sync-api") so we can claim the top-level "sync"
// for the cache-hydration command. Both surfaces stay available.
//
// PATCH(api-detail-hydrate): the generated command only ever wrote list rows
// into the generic resources/notes tables, which no read command queries. It
// now chains a detail stage that fetches each note's full payload and
// hydrates the Granola domain tables (meetings, attendees,
// transcript_segments, folders, folder_memberships) — the surface the
// meeting, attendee, folder, talktime, and memo commands actually read.
//
// The generated RunE is kept and run first so every flag it declares
// (--resources, --full, --since, --concurrency, --max-pages, --param, …)
// keeps its documented behavior; the detail stage runs afterwards and prints
// its own sync_summary line.
func newSyncApiCmd(flags *rootFlags) *cobra.Command {
	cmd := newSyncCmd(flags)
	cmd.Use = "sync-api"
	cmd.Short = "Sync the Granola public REST API into the local store"
	cmd.Long = `Calls Granola's public REST API (using GRANOLA_API_KEY) and hydrates the
local SQLite store.

Two stages run in order:

  1. List stage — pages the resources the public spec exposes (notes,
     folders) and records them, honoring --resources, --since, --full,
     --max-pages, and the other flags below.

  2. Detail stage — fetches each note individually (GET /v1/notes/{id},
     include=transcript) and writes meetings, attendees, calendar events,
     summaries, folder and space membership, and transcript segments into
     the tables the read commands consume. The list endpoint returns only
     id/title/owner/timestamps, so this stage is what makes 'meetings',
     'attendee', 'folder', 'talktime', 'transcript', and 'memo' work from
     API-sourced data.

API-sourced rows and desktop-cache rows coexist: each sync path only clears
the rows it owns, so running this command and the top-level 'sync' command
against the same store is safe in either order.`
	// The generated Example block advertises resources from the generator's
	// boilerplate ("channels,messages") and spells the command "sync", which
	// on this CLI is the cache-hydration command. Replace it with the real
	// surface.
	cmd.Example = `  # Full public-API sync: list every note, then hydrate its detail
  granola-pp-cli sync-api

  # Incremental: only notes updated in the last 7 days
  granola-pp-cli sync-api --since 7d

  # Notes only, skipping the folders list stage
  granola-pp-cli sync-api --resources notes`

	generated := cmd.RunE
	cmd.RunE = func(c *cobra.Command, args []string) error {
		if generated != nil {
			if err := generated(c, args); err != nil {
				return err
			}
		}
		if dryRunOK(flags) {
			return nil
		}
		res, err := runAPIHydrate(c.Context(), flags, apiHydrateOptions{
			UpdatedAfter: syncApiSinceValue(c),
			DBPath:       syncApiDBPath(c),
		})
		if err != nil {
			return err
		}
		if err := writeAPIHydrateSummary(c.OutOrStdout(), res); err != nil {
			return err
		}
		for _, w := range res.Warnings {
			c.PrintErrf("warning: %s\n", w)
		}
		return nil
	}
	return cmd
}

// syncApiSinceValue reads --since off the generated command and resolves it
// the same way the list stage does — parseSinceDuration then RFC3339 — so
// both stages narrow to the same window. Returns "" when the flag is absent,
// unset, or unparseable; an unparseable value has already failed the
// generated RunE, so this never silently swallows a user error.
func syncApiSinceValue(cmd *cobra.Command) string {
	f := cmd.Flags().Lookup("since")
	if f == nil || f.Value.String() == "" {
		return ""
	}
	ts, err := parseSinceDuration(f.Value.String())
	if err != nil {
		return ""
	}
	// PATCH(api-list-stage-matches-live-contract): must match the list
	// stage's UTC Z form; a numeric offset 400s the whole detail stage.
	return granolaAPITimestamp(ts)
}

// syncApiDBPath reads the shared --db flag so the detail stage writes the
// domain tables to the same database the list stage used.
func syncApiDBPath(cmd *cobra.Command) string {
	f := cmd.Flags().Lookup("db")
	if f == nil {
		return ""
	}
	return f.Value.String()
}
