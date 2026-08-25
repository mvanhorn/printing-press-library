// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0.

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
)

// PATCH(api-detail-hydrate): the generated `sync` machinery (sync.go) writes
// list rows into the GENERIC resources/notes tables via db.UpsertBatch. No
// read command in this CLI queries those tables — the meeting surface
// (meetings, attendees, transcript_segments, folder_memberships) is created
// by granola.EnsureSchema and, until now, only ever populated by
// granola.SyncFromCache.
//
// With the desktop cache permanently unreadable (Granola moved its
// data-encryption key into an entitlement-gated Keychain group), the public
// REST API is the working data path, so it has to reach the same tables. This
// file is the detail stage: page GET /v1/notes for ids, fetch each note's
// full detail with include=transcript, and hand the batch to
// granola.SyncFromAPI, which opens the store the same way runCacheSync does
// (openGranolaStore → EnsureSchema).

// apiHydrateDefaultMaxPages bounds the list stage so a pagination bug upstream
// cannot spin forever. At NotesPageSizeMax (30) per page this covers 6000
// notes, well past any realistic account.
const apiHydrateDefaultMaxPages = 200

// apiHydrateOptions carries the knobs the sync-api command exposes.
type apiHydrateOptions struct {
	// UpdatedAfter maps onto the API's updated_after filter (the incremental
	// cursor for notes). Empty means "everything".
	UpdatedAfter string

	// DBPath honors the sync commands' --db flag. Empty means the default
	// store location.
	DBPath string
	// MaxPages bounds the list stage; 0 uses apiHydrateDefaultMaxPages.
	MaxPages int
	// PageSize is clamped to [1, granola.NotesPageSizeMax] by ListNotesPage.
	PageSize int
	// SkipTranscripts drops include=transcript from the detail calls. The
	// transcript is by far the largest part of a note payload, so this exists
	// for callers that only need metadata.
	SkipTranscripts bool
}

// apiHydrateResult is the summary the sync-api command reports.
type apiHydrateResult struct {
	NotesListed  int
	NotesFetched int

	Skipped     int
	Meetings    int
	Attendees   int
	Segments    int
	Folders     int
	Memberships int
	Summaries   int
	Events      int
	Warnings    []string
	Duration    time.Duration

	// UnparsedTimestamps counts transcript timestamps the store layer could
	// not parse; the matching human-readable line is appended to Warnings.
	UnparsedTimestamps int

	// PATCH(transcript-retention-preserves-larger): PreservedTranscripts
	// counts meetings whose stored transcript this run left alone because the
	// cache path holds a larger copy than upstream retention still serves.
	// The matching human-readable line is appended to Warnings.
	PreservedTranscripts int
}

// domainRows totals the rows this stage wrote into the tables the read
// commands query. Summaries and Events are deliberately excluded: both are
// per-note markers written onto the meetings row (notes_markdown /
// calendar_event_id), not rows of their own, so counting them would inflate
// the auto-refresh provenance line past what actually landed.
func (r apiHydrateResult) domainRows() int {
	return r.Meetings + r.Attendees + r.Segments + r.Folders + r.Memberships
}

// runAPIHydrate performs the two-stage public-API sync and writes the result
// into the Granola domain tables.
//
// Failure policy, in order of how badly each failure generalises:
//
//   - 401 on any request, and 403 on the LIST endpoint, abort the whole run.
//     The credential cannot be used, so every remaining request would fail
//     identically; returning early means no partial store is written
//     (SyncFromAPI never runs).
//   - 404 on a single note's detail is routine — the note was deleted between
//     the list call and the detail call — so that id is skipped with a
//     recorded warning and the rest of the page still hydrates.
//   - 403 on a single note's detail is the same shape of verdict: the note is
//     outside what this token may read (ownership changed, archived, another
//     workspace) while the credential stays valid for every other id in the
//     run. Skipping it keeps the notes already fetched instead of throwing the
//     whole run away over one inaccessible note.
//   - anything else aborts, because a silent partial sync that looks like a
//     complete one is the failure mode this whole effort exists to remove.
func runAPIHydrate(ctx context.Context, flags *rootFlags, opts apiHydrateOptions) (apiHydrateResult, error) {
	started := time.Now()
	res := apiHydrateResult{}

	c, err := flags.newClient()
	if err != nil {
		return res, err
	}
	// The client's on-disk GET cache would happily replay a five-minute-old
	// page here; a sync must see the live state.
	c.NoCache = true

	maxPages := opts.MaxPages
	if maxPages <= 0 {
		maxPages = apiHydrateDefaultMaxPages
	}
	listParams := map[string]string{}
	if opts.UpdatedAfter != "" {
		listParams["updated_after"] = opts.UpdatedAfter
	}

	var notes []granola.APINote
	cursor := ""
	for page := 0; page < maxPages; page++ {
		select {
		case <-ctx.Done():
			res.Duration = time.Since(started)
			return res, ctx.Err()
		default:
		}
		listPage, err := granola.ListNotesPage(c, cursor, opts.PageSize, listParams)
		if err != nil {
			res.Duration = time.Since(started)
			return res, hydrateError(err, flags)
		}
		res.NotesListed += len(listPage.Notes)
		for _, ref := range listPage.Notes {
			if ref.ID == "" {
				continue
			}
			// PATCH(autorefresh-api-hydrates-domain-tables): the detail
			// fetches are the expensive part (one request per note), and the
			// HTTP client carries its own per-request timeout rather than
			// this context. Checking here is what lets the auto-refresh
			// deadline actually bound a page's worth of fetches instead of
			// only firing between pages.
			select {
			case <-ctx.Done():
				res.Duration = time.Since(started)
				return res, ctx.Err()
			default:
			}
			note, err := granola.GetNote(c, ref.ID, !opts.SkipTranscripts)
			if err != nil {
				if reason := skippableNoteError(err); reason != "" {
					res.Skipped++
					res.Warnings = append(res.Warnings,
						fmt.Sprintf("note %s: detail fetch returned %s, skipped", ref.ID, reason))
					continue
				}
				res.Duration = time.Since(started)
				return res, hydrateError(err, flags)
			}
			notes = append(notes, *note)
		}
		if !listPage.HasMore || listPage.Cursor == "" || listPage.Cursor == cursor {
			break
		}
		cursor = listPage.Cursor
	}
	res.NotesFetched = len(notes)

	s, err := openGranolaStoreAt(ctx, opts.DBPath)
	if err != nil {
		res.Duration = time.Since(started)
		return res, err
	}
	defer s.Close()

	sres, err := granola.SyncFromAPI(ctx, s.DB(), notes)
	res.Meetings = sres.Meetings
	res.Attendees = sres.Attendees
	res.Segments = sres.Segments
	res.Folders = sres.Folders
	res.Memberships = sres.Memberships
	res.Summaries = sres.Summaries
	res.Events = sres.Events
	res.UnparsedTimestamps = sres.UnparsedTimestamps
	if sres.TimestampWarning != "" {
		res.Warnings = append(res.Warnings, sres.TimestampWarning)
	}
	res.PreservedTranscripts = sres.PreservedTranscripts
	if sres.PreservationWarning != "" {
		res.Warnings = append(res.Warnings, sres.PreservationWarning)
	}
	res.Duration = time.Since(started)
	if err != nil {
		return res, err
	}
	return res, nil
}

// skippableNoteError reports whether a per-note detail failure is a verdict
// about that one note rather than about the credential, returning the label
// the recorded warning uses. An empty string means "abort the run".
//
// PATCH(api-detail-hydrate): 403 used to land here as a plain
// ErrAPIUnauthorized and abort, discarding every note fetched so far in the
// run. One note the token may not read is not a reason to throw away the rest,
// so it is treated like a 404. 401 is deliberately absent: a rejected
// credential fails every remaining request identically.
func skippableNoteError(err error) string {
	switch {
	case errors.Is(err, granola.ErrNoteNotFound):
		return "404"
	case errors.Is(err, granola.ErrAPIForbidden):
		return "403 (forbidden for this note)"
	}
	return ""
}

// hydrateError routes an auth rejection through the CLI's auth-error
// classification so the exit code and the "check your token" hint match every
// other command, and leaves anything else as a plain API error.
func hydrateError(err error, flags *rootFlags) error {
	if errors.Is(err, granola.ErrAPIUnauthorized) {
		return classifyAPIError(err, flags)
	}
	return apiErr(err)
}

// writeAPIHydrateSummary emits the one ndjson line the sync-api command
// prints for the detail stage, mirroring the shape runCacheSync's wrapper
// uses so downstream agents can parse both with one reader.
func writeAPIHydrateSummary(w io.Writer, res apiHydrateResult) error {
	summary := map[string]any{
		"event":               "sync_summary",
		"source":              "granola_public_api",
		"stage":               "detail_hydrate",
		"notes_listed":        res.NotesListed,
		"notes_fetched":       res.NotesFetched,
		"notes_skipped":       res.Skipped,
		"meetings":            res.Meetings,
		"attendees":           res.Attendees,
		"transcript_segments": res.Segments,
		"folders":             res.Folders,
		"folder_memberships":  res.Memberships,
		"summaries":           res.Summaries,
		"calendar_events":     res.Events,
	}
	if res.UnparsedTimestamps > 0 {
		summary["unparsed_timestamps"] = res.UnparsedTimestamps
	}
	if res.PreservedTranscripts > 0 {
		summary["preserved_transcripts"] = res.PreservedTranscripts
	}
	if len(res.Warnings) > 0 {
		summary["warnings"] = res.Warnings
	}
	return emitNDJSONLine(w, summary)
}
