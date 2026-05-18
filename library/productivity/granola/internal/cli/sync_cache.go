// Copyright 2026 dstevens. Licensed under Apache-2.0.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola/safestorage"
)

// CacheSyncResult captures everything a cache sync produces, in a form that
// either the user-facing `sync` command or the auto-refresh hook can consume.
// HydrateErr is non-fatal: hydration of /v2/get-documents may fail while the
// cache decrypt itself succeeded, so callers surface it as a warning rather
// than aborting.
type CacheSyncResult struct {
	Version          int
	Meetings         int
	Attendees        int
	Segments         int
	Folders          int
	Memberships      int
	Panels           int
	Recipes          int
	Workspaces       int
	ChatThreads      int
	ChatMessages     int
	DocumentsFetched int
	HydrateErr       error
	StateWriteErr    error
	Duration         time.Duration
}

// TotalRows is the headline count used by the auto-refresh provenance line.
func (r CacheSyncResult) TotalRows() int {
	return r.Meetings + r.Attendees + r.Segments + r.Folders + r.Memberships +
		r.Panels + r.Recipes + r.Workspaces + r.ChatThreads + r.ChatMessages
}

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
			res, err := runCacheSync(cmd.Context())
			if err != nil {
				return err
			}
			summary := map[string]any{
				"event":               "sync_summary",
				"source":              "granola_cache",
				"version":             res.Version,
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
				"documents_fetched":   res.DocumentsFetched,
			}
			if res.HydrateErr != nil {
				summary["documents_fetch_error"] = res.HydrateErr.Error()
			}
			b, _ := json.Marshal(summary)
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
			// Surface the hydrate error as a non-fatal warning to stderr
			// so the user sees it but the sync still reports what it
			// successfully synced from the cache (transcripts, folders,
			// recipes, panels, chats).
			if res.HydrateErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: documents API hydrate failed: %v\n", res.HydrateErr)
			}
			if res.StateWriteErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to write sync state: %v\n", res.StateWriteErr)
			}
			return nil
		},
	}
	return cmd
}

// PATCH(auto-refresh): Factored out of newSyncCacheCmd.RunE so the auto-refresh
// hook in PersistentPreRunE can drive the same cache→SQLite hydration without
// going through Cobra's command dispatch. The user-visible sync command becomes
// a thin wrapper that adds JSON output formatting; auto-refresh consumes the
// returned struct and emits a one-line provenance summary instead.
//
// runCacheSync decrypts the encrypted desktop cache, hydrates documents from
// /v2/get-documents, upserts every row into the local SQLite store, and writes
// the SyncState record doctor reads. It is best-effort with respect to document
// hydration (returned in result.HydrateErr) but returns an error when the cache
// itself cannot be opened — the caller decides whether that is fatal.
func runCacheSync(ctx context.Context) (CacheSyncResult, error) {
	started := time.Now()
	c, err := openGranolaCache()
	if err != nil {
		// PATCH(encrypted-cache): record the decrypt failure so doctor
		// can report it without itself prompting the Keychain.
		recordSyncDecryptStatus(err)
		return CacheSyncResult{Duration: time.Since(started)}, err
	}
	// PATCH(encrypted-cache): Granola desktop moved documents
	// out of cache-v6.json into the API around May 2026. Hydrate
	// from /v2/get-documents so SyncFromCache's meeting upsert
	// loop has something to iterate.
	docsFetched, hydrateErr := granola.HydrateDocumentsFromAPI(c, nil)
	s, err := openGranolaStore(ctx)
	if err != nil {
		return CacheSyncResult{Duration: time.Since(started)}, err
	}
	defer s.Close()
	sres, err := granola.SyncFromCache(ctx, s.DB(), c)
	if err != nil {
		return CacheSyncResult{Duration: time.Since(started)}, err
	}
	res := CacheSyncResult{
		Version:          c.Version,
		Meetings:         sres.Meetings,
		Attendees:        sres.Attendees,
		Segments:         sres.Segments,
		Folders:          sres.Folders,
		Memberships:      sres.Memberships,
		Panels:           sres.Panels,
		Recipes:          sres.Recipes,
		Workspaces:       sres.Workspaces,
		ChatThreads:      sres.ChatThreads,
		ChatMessages:     sres.ChatMessages,
		DocumentsFetched: docsFetched,
		HydrateErr:       hydrateErr,
		Duration:         time.Since(started),
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
		// Surface state-write failure on the result so the wrapper can
		// route it to stderr the same way the original RunE did. Kept
		// separate from HydrateErr because the manual sync command
		// prints each with its own stderr label.
		res.StateWriteErr = writeErr
	}
	return res, nil
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
