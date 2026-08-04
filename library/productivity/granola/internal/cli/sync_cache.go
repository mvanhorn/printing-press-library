// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola/safestorage"
	"github.com/spf13/cobra"
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

	// PATCH(api-sync-survives-unreadable-cache): Degraded reports that the
	// desktop cache could not be decrypted and only API-derived documents were
	// synced; DecryptErr carries why. Cache-derived surfaces (transcripts,
	// folders, recipes, panels, chat threads) are all zero on such a run, so
	// reporting it as a clean sync would read as "you have no transcripts"
	// rather than "transcripts were not synced".
	Degraded   bool
	DecryptErr error

	// PATCH(api-transcript-hydrate): what the transcript backfill did this run.
	// TranscriptsRemaining is the operator-facing number: a bounded run that
	// leaves work behind must say so, or the user cannot tell "that is all of
	// them" from "run it again".
	TranscriptsFetched   int
	TranscriptSegments   int
	TranscriptsRemaining int
	TranscriptErr        error

	// PATCH(api-catalog-refresh): what the catalog refresh staged this run and
	// why it did not. CatalogErr is non-fatal for the same reason HydrateErr
	// is: recipes, panel templates and folders are three independent
	// endpoints, and one of them rejecting the credential must not cost the
	// user the meetings and transcripts the rest of the run synced.
	CatalogRecipes        int
	CatalogPanelTemplates int
	CatalogFolders        int
	CatalogMemberships    int
	CatalogErr            error

	// PATCH(api-detail-hydrate): transcript timestamps the store layer could
	// not parse. Non-fatal like HydrateErr, but it must reach the operator:
	// an unparseable timestamp is stored as 0 and reads back blank, so without
	// this the only symptom of an upstream format change is transcripts that
	// quietly lose their times.
	UnparsedTimestamps int
	TimestampWarning   string

	// PATCH(transcript-retention-preserves-larger): meetings whose stored
	// transcript this sync left alone because the API path holds a larger
	// copy. Non-fatal like the fields above, and reported for the same
	// reason: a sync that quietly writes fewer segments than last time is
	// indistinguishable from a broken one without a line saying why.
	PreservedTranscripts int
	PreservationWarning  string
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
	// PATCH(api-transcript-hydrate): bounds the per-run transcript backfill.
	// There is no bulk transcript endpoint, so a full first backfill is one
	// rate-limited call per meeting; a default cap keeps `sync` a command
	// rather than an errand, and the summary reports what is left.
	transcriptBudget := granola.DefaultTranscriptBudget
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
			res, err := runCacheSync(cmd.Context(), transcriptBudget)
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
			if res.Degraded {
				// Never let a degraded run read as a clean one.
				summary["degraded"] = true
				summary["degraded_reason"] = "desktop cache unreadable; synced over the API"
			}
			if res.TranscriptsFetched > 0 || res.TranscriptsRemaining > 0 {
				summary["transcripts_fetched"] = res.TranscriptsFetched
				summary["transcripts_remaining"] = res.TranscriptsRemaining
			}
			if res.TranscriptErr != nil {
				summary["transcript_fetch_error"] = res.TranscriptErr.Error()
			}
			if res.CatalogErr != nil {
				summary["catalog_refresh_error"] = res.CatalogErr.Error()
			}
			if res.UnparsedTimestamps > 0 {
				summary["unparsed_timestamps"] = res.UnparsedTimestamps
			}
			if res.PreservedTranscripts > 0 {
				summary["preserved_transcripts"] = res.PreservedTranscripts
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
			if res.TimestampWarning != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", res.TimestampWarning)
			}
			if res.PreservationWarning != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", res.PreservationWarning)
			}
			if res.TranscriptErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: transcript backfill stopped early: %v\n", res.TranscriptErr)
			}
			if res.CatalogErr != nil {
				// Non-fatal: the surfaces that did answer are already synced,
				// and the counts above say which. Naming the failure keeps a
				// stale recipe list from reading as an empty one.
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: catalog refresh incomplete: %v\n", res.CatalogErr)
			}
			if res.TranscriptsRemaining > 0 {
				// The whole point of reporting this: a bounded run that stops
				// with work left must not look like it finished.
				fmt.Fprintf(cmd.ErrOrStderr(),
					"%d transcripts still to fetch; re-run `granola-pp-cli sync` to continue (or --transcript-budget -1 for all)\n",
					res.TranscriptsRemaining)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&transcriptBudget, "transcript-budget", granola.DefaultTranscriptBudget,
		"Max transcripts to backfill this run (0 disables the backfill, -1 fetches all remaining)")
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
func runCacheSync(ctx context.Context, transcriptBudget int) (CacheSyncResult, error) {
	started := time.Now()
	degraded := false
	var decryptErr error
	c, err := openGranolaCache()
	if err != nil {
		// PATCH(encrypted-cache): record the decrypt failure so doctor
		// can report it without itself prompting the Keychain.
		recordSyncDecryptStatus(err)

		// PATCH(api-sync-survives-unreadable-cache): a migrated install can
		// never decrypt again, so failing here strands the one path that still
		// works — /v2/get-documents, which needs no cache content at all.
		// Degrade instead of aborting, and carry the decrypt error so the run
		// is never reported as clean.
		//
		// Only a classified migration degrades. A plain ErrKeyUnavailable on a
		// non-migrated install still has a working remedy (sign into Granola
		// desktop); degrading it would hide that remedy behind a silently
		// thinner sync.
		if !errors.Is(err, safestorage.ErrSchemeMigrated) {
			return CacheSyncResult{Duration: time.Since(started)}, err
		}
		degraded = true
		decryptErr = err
		c = &granola.Cache{Documents: map[string]granola.Document{}}
	}
	// PATCH(encrypted-cache): Granola desktop moved documents
	// out of cache-v6.json into the API around May 2026. Hydrate
	// from /v2/get-documents so SyncFromCache's meeting upsert
	// loop has something to iterate.
	docsFetched, hydrateErr := granola.HydrateDocumentsFromAPI(c, nil)
	// On a degraded run the hydrate is the whole run: there is no cache
	// content to fall back on, so a hydrate failure means nothing was synced.
	if degraded && hydrateErr != nil {
		return CacheSyncResult{Degraded: true, DecryptErr: decryptErr, Duration: time.Since(started)},
			fmt.Errorf("cache unreadable and API hydrate failed: %w", hydrateErr)
	}
	s, err := openGranolaStore(ctx)
	if err != nil {
		return CacheSyncResult{Duration: time.Since(started)}, err
	}
	defer s.Close()

	// PATCH(api-transcript-hydrate): on a degraded run the cache carries no
	// transcripts, so pull them over the API before the upsert. Without this
	// the run syncs titles and attendees and leaves every transcript-dependent
	// command returning empty, which reads as "no transcripts" rather than
	// "not synced". Bounded and resumable: see HydrateTranscriptsFromAPI.
	var tres granola.TranscriptHydrateResult
	var transcriptErr error
	if degraded && transcriptBudget != 0 {
		tres, transcriptErr = granola.HydrateTranscriptsFromAPI(ctx, s.DB(), c, nil, transcriptBudget)
	}

	// PATCH(api-catalog-refresh): the same argument one surface over. Without
	// this the degraded run leaves recipes, panel templates and folders frozen
	// at the last successful decrypt while still answering as though they were
	// current, so a folder or recipe created since then is invisible with no
	// symptom the user can see. Each surface is a single call for the whole
	// set, so unlike transcripts there is nothing to budget or resume.
	var cres granola.CatalogHydrateResult
	var catalogErr error
	if degraded {
		cres, catalogErr = granola.HydrateCatalogFromAPI(c, nil)
	}

	syncOpts := granola.SyncOptions{Degraded: degraded}
	if degraded {
		// Transcripts on this path came from the API, not the cache. Label them
		// accordingly so provenance stays honest and the cross-source retention
		// check compares real counts.
		syncOpts.TranscriptOwner = granola.RowSourceAPI
		// Same for the catalog rows this run creates. Ownership decides which
		// path is allowed to retire a row later, so a folder or recipe the API
		// created must not be filed under the cache path that can no longer
		// read anything.
		syncOpts.CatalogOwner = granola.RowSourceAPI
	}
	sres, err := granola.SyncFromCacheWithOptions(ctx, s.DB(), c, syncOpts)
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
		Degraded:         degraded,
		DecryptErr:       decryptErr,
		Duration:         time.Since(started),

		TranscriptsFetched:   tres.WithSegments,
		TranscriptSegments:   tres.Segments,
		TranscriptsRemaining: tres.Remaining,
		TranscriptErr:        transcriptErr,

		CatalogRecipes:        cres.Recipes,
		CatalogPanelTemplates: cres.PanelTemplates,
		CatalogFolders:        cres.Folders,
		CatalogMemberships:    cres.Memberships,
		CatalogErr:            catalogErr,

		UnparsedTimestamps: sres.UnparsedTimestamps,
		TimestampWarning:   sres.TimestampWarning,

		PreservedTranscripts: sres.PreservedTranscripts,
		PreservationWarning:  sres.PreservationWarning,
	}
	// PATCH(encrypted-cache): record success so doctor can report
	// "ok (last decrypted: <time>)" without itself decrypting.
	state := granola.SyncState{
		LastSyncAt: time.Now().UTC(),
		// PATCH(read-path-store-first-for-recipes-and-chats): stamp the cache
		// sync time only on a genuine decrypt, so store-served reads can date
		// cache-derived rows by when they were actually captured rather than by
		// this run's API-only refresh.
		LastCacheSyncAt: cacheSyncedAt(degraded),
		// PATCH(api-catalog-refresh): stamped only when the catalog refresh
		// actually succeeded, so store-served recipe reads date themselves by
		// when they were refreshed rather than by a cache sync that never ran.
		LastCatalogSyncAt:    catalogSyncedAt(catalogErr, degraded),
		LastDecryptStatus:    granola.DecryptStatusOK,
		LastTokenSource:      tokenSourceLabel(granola.CurrentTokenSource()),
		LastDocumentsFetched: docsFetched,
	}
	// PATCH(api-sync-survives-unreadable-cache): a degraded run did sync
	// documents, but it never decrypted anything. Claiming DecryptStatusOK
	// here would overwrite the failure recordSyncDecryptStatus just stored and
	// make doctor report a healthy encrypted store on a migrated install.
	if degraded {
		state.LastDecryptStatus = granola.DecryptStatusFailed
		state.LastDecryptErrorClass = decryptErrorClass(decryptErr)
		if decryptErr != nil {
			state.LastDecryptErrorMsg = decryptErr.Error()
		}
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
		LastSyncAt:          time.Now().UTC(),
		LastDecryptStatus:   granola.DecryptStatusFailed,
		LastDecryptErrorMsg: err.Error(),
	}
	state.LastDecryptErrorClass = decryptErrorClass(err)
	_ = granola.WriteSyncState(state)
}

// catalogSyncedAt returns now only when this run refreshed the catalog
// surfaces cleanly. A healthy-cache run never calls the API catalog, and a
// failed refresh must not advance the date rows are stamped with.
func catalogSyncedAt(catalogErr error, degraded bool) time.Time {
	if !degraded || catalogErr != nil {
		return time.Time{}
	}
	return time.Now().UTC()
}

// cacheSyncedAt returns now for a genuine cache decrypt and the zero time for a
// degraded run, so a run that never read the cache cannot advance the date the
// cache-derived rows are stamped with.
func cacheSyncedAt(degraded bool) time.Time {
	if degraded {
		return time.Time{}
	}
	return time.Now().UTC()
}

// decryptErrorClass maps a cache-load error onto the stable class string
// doctor reads out of the sync state.
//
// PATCH(api-sync-survives-unreadable-cache): ErrSchemeMigrated must be tested
// first. migratedSchemeError.Is deliberately matches ErrKeyUnavailable too, so
// a plain key_unavailable arm ahead of it swallows every migrated install --
// which is how doctor came to print the unreachable "approve the Keychain
// prompt" remedy for a state where no prompt can ever appear.
func decryptErrorClass(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, safestorage.ErrSchemeMigrated):
		return "scheme_migrated"
	case errors.Is(err, safestorage.ErrKeyUnavailable):
		return "key_unavailable"
	case errors.Is(err, safestorage.ErrDecryptFailed):
		return "decrypt_failed"
	case errors.Is(err, safestorage.ErrUnsupportedPlatform):
		return "unsupported_platform"
	}
	return "other"
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
	case granola.TokenSourcePlaintextSupabaseDesktopFallback:
		return "plaintext_supabase_desktop_fallback"
	case granola.TokenSourceCLISession:
		return "cli_session"
	}
	return "unknown"
}
