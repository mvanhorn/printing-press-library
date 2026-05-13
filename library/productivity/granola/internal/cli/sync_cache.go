// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola/safestorage"
)

// newSyncCacheCmd is registered as the top-level 'sync' replacement.
// Granola's public API only covers ~3 endpoints; the cache file is the
// real source of truth. We hydrate the SQLite store from the cache and
// emit one ndjson summary line so downstream agents and existing sync
// callers see a consistent shape.
func newSyncCacheCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync Granola's local cache file into the SQLite store",
		Long: `Granola's public API exposes only a thin slice of features. Most
useful data — notes, transcripts, panels, folders, recipes, chat
threads — lives in the desktop app's cache file. This command reads
that cache and upserts every row into the local SQLite store so the
'meetings', 'attendee', 'folder', 'stats', and 'memo' commands can
read offline.

The hydration is idempotent: re-running replaces every row.`,
		Annotations: map[string]string{
			"mcp:read-only": "false",
			// touches local SQLite but does not write external state.
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := openGranolaCache()
			if err != nil {
				// PATCH(encrypted-cache): record the decrypt failure so doctor
				// can report it without itself prompting the Keychain.
				recordSyncDecryptStatus(err)
				return err
			}
			// PATCH(encrypted-cache): Granola desktop moved documents
			// out of cache-v6.json into the API around May 2026. Hydrate
			// from /v2/get-documents so SyncFromCache's meeting upsert
			// loop has something to iterate.
			docsFetched, hydrateErr := granola.HydrateDocumentsFromAPI(c, nil)
			s, err := openGranolaStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			res, err := granola.SyncFromCache(cmd.Context(), s.DB(), c)
			if err != nil {
				return err
			}
			summary := map[string]any{
				"event":               "sync_summary",
				"source":              "granola_cache",
				"version":             c.Version,
				"meetings":            res.Meetings,
				"attendees":           res.Attendees,
				"transcript_segments": res.Segments,
				"folders":             res.Folders,
				"folder_memberships":  res.Memberships,
				"panel_templates":     res.Panels,
				"recipes":             res.Recipes,
				"workspaces":          res.Workspaces,
				"chat_threads":        res.ChatThreads,
				"chat_messages":       res.ChatMessages,
				"documents_fetched":   docsFetched,
			}
			if hydrateErr != nil {
				summary["documents_fetch_error"] = hydrateErr.Error()
			}
			b, _ := json.Marshal(summary)
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
			// Surface the hydrate error as a non-fatal warning to stderr
			// so the user sees it but the sync still reports what it
			// successfully synced from the cache (transcripts, folders,
			// recipes, panels, chats).
			if hydrateErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: documents API hydrate failed: %v\n", hydrateErr)
			}
			// PATCH(encrypted-cache): record success so doctor can report
			// "ok (last decrypted: <time>)" without itself decrypting.
			state := granola.SyncState{
				LastSyncAt:           time.Now().UTC(),
				LastDecryptStatus:    granola.DecryptStatusOK,
				LastTokenSource:      tokenSourceLabel(granola.CurrentTokenSource()),
				LastDocumentsFetched: docsFetched,
			}
			if hydrateErr != nil {
				state.LastHydrateErrorMsg = hydrateErr.Error()
			}
			if writeErr := granola.WriteSyncState(state); writeErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to write sync state: %v\n", writeErr)
			}
			return nil
		},
	}
	return cmd
}

// PATCH(encrypted-cache): translate the load-error error chain into a
// sync-state record so doctor can surface "decrypt failed" specifically
// rather than the generic "load failed".
func recordSyncDecryptStatus(err error) {
	state := granola.SyncState{
		LastSyncAt:        time.Now().UTC(),
		LastDecryptStatus: granola.DecryptStatusFailed,
		LastDecryptErrorMsg: err.Error(),
	}
	switch {
	case errors.Is(err, safestorage.ErrKeyUnavailable):
		state.LastDecryptErrorClass = "key_unavailable"
	case errors.Is(err, safestorage.ErrDecryptFailed):
		state.LastDecryptErrorClass = "decrypt_failed"
	case errors.Is(err, safestorage.ErrUnsupportedPlatform):
		state.LastDecryptErrorClass = "unsupported_platform"
	default:
		state.LastDecryptErrorClass = "other"
	}
	_ = granola.WriteSyncState(state)
}

// tokenSourceLabel returns a human-readable + JSON-stable label for the
// TokenSource enum. Used in the sync state record.
func tokenSourceLabel(s granola.TokenSource) string {
	switch s {
	case granola.TokenSourceEnvOverride:
		return "env_override"
	case granola.TokenSourcePlaintextSupabase:
		return "plaintext_supabase"
	case granola.TokenSourceEncryptedSupabase:
		return "encrypted_supabase"
	case granola.TokenSourceStoredAccounts:
		return "stored_accounts"
	}
	return "unknown"
}
