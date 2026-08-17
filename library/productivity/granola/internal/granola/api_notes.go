// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0.

package granola

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/client"
)

// PATCH(api-detail-hydrate): Granola desktop moved its data-encryption key
// into an entitlement-gated Keychain group, so the encrypted local cache is
// permanently unreadable by this CLI. The PUBLIC REST API at
// https://public-api.granola.ai is now the working data path, and it carries
// far more than the generated spec assumed: per-note detail includes
// attendees, calendar event, folder/space membership, summaries, and (with
// include=transcript) the full transcript.
//
// This file models that surface. It is deliberately NOT api_documents.go:
// that file drives the INTERNAL API (/v2/get-documents via InternalClient
// with the WorkOS token), which is the exact path the key migration killed.
//
// The list endpoint returns THIN rows — id/object/title/owner/timestamps
// only — so a useful sync is two-stage: page the list, then fetch each note's
// detail. Callers hand the resulting []APINote to SyncFromAPI.

// NotesPageSizeMax is the maximum page_size GET /v1/notes accepts. Verified
// live: values above 30 are rejected with a validation error, and the
// documented minimum is 1 (default 10). Paging at the ceiling minimises the
// number of list round-trips before the (much larger) detail stage.
const NotesPageSizeMax = 30

// Sentinel errors so callers can distinguish "skip this note" from "stop the
// whole sync". A 404 on one note id in a page is routine (the note was
// deleted between the list call and the detail call); a 401/403 means the
// credential is bad and every subsequent request would fail the same way.
var (
	// ErrNoteNotFound reports a 404 from GET /v1/notes/{id}.
	ErrNoteNotFound = errors.New("note not found")
	// ErrAPIUnauthorized reports a 401/403 from the public API.
	ErrAPIUnauthorized = errors.New("public API rejected the credential")
	// ErrAPIForbidden narrows ErrAPIUnauthorized to the 403 half.
	//
	// PATCH(api-detail-hydrate): 403 answers a different question depending on
	// what was requested. On the LIST endpoint it means the token cannot list
	// notes at all, so aborting is right. On a single-resource GET it means
	// "forbidden for THIS note" — ownership changed, the note was archived, or
	// it sits outside the token's scope — while the credential remains valid
	// for everything else in the run. A 403 error therefore matches BOTH this
	// sentinel and ErrAPIUnauthorized: callers that can skip one item test for
	// ErrAPIForbidden first, and every caller that only cares that the
	// credential was rejected (exit code, auth hint) keeps working unchanged.
	// 401 never matches this sentinel; it is fatal everywhere.
	ErrAPIForbidden = errors.New("public API forbade access to this resource")
)

// APIGetter is the slice of *client.Client this file needs. Declaring it as
// an interface keeps the granola package testable without a live key and
// lets callers pass the shared client built by rootFlags.newClient().
type APIGetter interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
}

// APIPerson is a person as the public API renders them. Attendees carry both
// name and email; calendar invitees carry email only, which is why attendee
// reconciliation keys on email (see SyncFromAPI).
type APIPerson struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

// APINoteRef is one THIN row from GET /v1/notes. Everything else about a
// note — attendees, transcript, summaries, membership — requires the detail
// call.
type APINoteRef struct {
	ID        string     `json:"id"`
	Object    string     `json:"object,omitempty"`
	Title     string     `json:"title,omitempty"`
	Owner     *APIPerson `json:"owner,omitempty"`
	CreatedAt string     `json:"created_at,omitempty"`
	UpdatedAt string     `json:"updated_at,omitempty"`
}

// APINotesPage is the GET /v1/notes envelope.
type APINotesPage struct {
	Notes   []APINoteRef `json:"notes"`
	HasMore bool         `json:"hasMore"`
	Cursor  string       `json:"cursor"`
}

// APICalendarEvent is the calendar_event block on a note detail.
//
// Organiser is kept as raw JSON because the live payload has been observed
// carrying it both as an object and as a bare email string; OrganiserEmail
// normalises both rather than hard-failing the whole note on a shape we did
// not anticipate.
type APICalendarEvent struct {
	EventTitle         string          `json:"event_title,omitempty"`
	Invitees           []APIPerson     `json:"invitees,omitempty"`
	Organiser          json.RawMessage `json:"organiser,omitempty"`
	CalendarEventID    string          `json:"calendar_event_id,omitempty"`
	ScheduledStartTime string          `json:"scheduled_start_time,omitempty"`
	ScheduledEndTime   string          `json:"scheduled_end_time,omitempty"`
}

// OrganiserEmail extracts the organiser's email from either an object with an
// "email" key or a bare JSON string. Returns "" when neither shape matches.
func (e *APICalendarEvent) OrganiserEmail() string {
	if e == nil || len(e.Organiser) == 0 {
		return ""
	}
	var obj APIPerson
	if err := json.Unmarshal(e.Organiser, &obj); err == nil && obj.Email != "" {
		return obj.Email
	}
	var s string
	if err := json.Unmarshal(e.Organiser, &s); err == nil && strings.Contains(s, "@") {
		return s
	}
	return ""
}

// OrganiserName extracts the organiser's display name when the payload
// carries the object shape. Returns "" for the bare-string shape.
func (e *APICalendarEvent) OrganiserName() string {
	if e == nil || len(e.Organiser) == 0 {
		return ""
	}
	var obj APIPerson
	if err := json.Unmarshal(e.Organiser, &obj); err == nil {
		return obj.Name
	}
	return ""
}

// APIMembership is one entry in folder_membership or space_membership. The
// key spelling varies between the two collections, so every observed spelling
// is declared and Ident/Label pick the first non-empty one — a membership
// entry we cannot identify is skipped rather than written under an empty id.
type APIMembership struct {
	ID       string `json:"id,omitempty"`
	FolderID string `json:"folder_id,omitempty"`
	SpaceID  string `json:"space_id,omitempty"`
	Name     string `json:"name,omitempty"`
	Title    string `json:"title,omitempty"`
}

// Ident returns the membership container's id.
func (m APIMembership) Ident() string {
	return firstNonEmpty(m.ID, m.FolderID, m.SpaceID)
}

// Label returns the membership container's human-readable name.
func (m APIMembership) Label() string {
	return firstNonEmpty(m.Title, m.Name)
}

// APISpeaker is the speaker block on a transcript segment.
//
// Source is the API's enum: "microphone" or "speaker" (singular). That
// vocabulary does NOT match the store's — see NormalizeSpeakerSource.
// Name/DiarizationLabel are optional: the API can resolve speaker identity
// the desktop cache never carried.
type APISpeaker struct {
	Source            string `json:"source,omitempty"`
	Name              string `json:"name,omitempty"`
	DiarizationLabel  string `json:"diarization_label,omitempty"`
	Label             string `json:"label,omitempty"`
	SpeakerIdentifier string `json:"speaker_identifier,omitempty"`
}

// APITranscriptSegment is one entry in the transcript array returned by
// GET /v1/notes/{id}?include=transcript.
type APITranscriptSegment struct {
	Text      string      `json:"text,omitempty"`
	StartTime string      `json:"start_time,omitempty"`
	EndTime   string      `json:"end_time,omitempty"`
	Speaker   *APISpeaker `json:"speaker,omitempty"`
}

// APINote is the full note returned by GET /v1/notes/{id}. Transcript is nil
// unless the request passed include=transcript; a nil transcript is a normal
// outcome, not a failure.
type APINote struct {
	ID               string                 `json:"id"`
	Object           string                 `json:"object,omitempty"`
	Title            string                 `json:"title,omitempty"`
	WebURL           string                 `json:"web_url,omitempty"`
	Owner            *APIPerson             `json:"owner,omitempty"`
	CreatedAt        string                 `json:"created_at,omitempty"`
	UpdatedAt        string                 `json:"updated_at,omitempty"`
	CalendarEvent    *APICalendarEvent      `json:"calendar_event,omitempty"`
	Attendees        []APIPerson            `json:"attendees,omitempty"`
	FolderMembership []APIMembership        `json:"folder_membership,omitempty"`
	SpaceMembership  []APIMembership        `json:"space_membership,omitempty"`
	Transcript       []APITranscriptSegment `json:"transcript,omitempty"`
	SummaryText      string                 `json:"summary_text,omitempty"`
	SummaryMarkdown  string                 `json:"summary_markdown,omitempty"`
}

// NormalizeSpeakerSource translates the public API's speaker.source enum onto
// the vocabulary the store (and therefore talktime, transcript, export)
// already speaks.
//
// This is load-bearing, not cosmetic. The API emits "speaker" (singular) for
// the far-end audio; talktime matches `case "system", "speakers":`. Writing
// "speaker" through unchanged makes talktime silently report zero seconds for
// the other party while every other command still looks healthy — a silent
// wrong answer, which is worse than an error.
func NormalizeSpeakerSource(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return ""
	case "microphone", "mic", "me":
		return "microphone"
	case "speaker", "speakers", "system", "them":
		return "system"
	default:
		return strings.ToLower(strings.TrimSpace(s))
	}
}

// ResolvedName returns the speaker's human name when the API resolved one.
func (s *APISpeaker) ResolvedName() string {
	if s == nil {
		return ""
	}
	return s.Name
}

// ResolvedLabel returns the diarization label (e.g. "SPEAKER_01") when the
// API carried one.
func (s *APISpeaker) ResolvedLabel() string {
	if s == nil {
		return ""
	}
	return firstNonEmpty(s.DiarizationLabel, s.Label, s.SpeakerIdentifier)
}

// ListNotesPage fetches one page of GET /v1/notes. Pass an empty cursor for
// the first page; the returned page's Cursor feeds the next call while
// HasMore is true. pageSize is clamped to [1, NotesPageSizeMax] — the API
// rejects anything outside that range outright.
//
// extraParams carries optional server-side filters (created_before,
// created_after, updated_after, folder_id). Empty values are dropped.
func ListNotesPage(c APIGetter, cursor string, pageSize int, extraParams map[string]string) (APINotesPage, error) {
	var page APINotesPage
	if c == nil {
		return page, fmt.Errorf("nil api client")
	}
	if pageSize <= 0 || pageSize > NotesPageSizeMax {
		pageSize = NotesPageSizeMax
	}
	params := map[string]string{"page_size": fmt.Sprintf("%d", pageSize)}
	for k, v := range extraParams {
		if v != "" {
			params[k] = v
		}
	}
	if cursor != "" {
		params["cursor"] = cursor
	}
	raw, err := c.Get("/v1/notes", params)
	if err != nil {
		return page, classifyPublicAPIError(err, "list notes")
	}
	if err := json.Unmarshal(raw, &page); err != nil {
		return page, fmt.Errorf("list notes: decoding response: %w", err)
	}
	return page, nil
}

// GetNote fetches the full detail for one note. withTranscript adds
// include=transcript, the only value the endpoint accepts; without it the
// transcript field comes back null.
//
// A 404 is returned as ErrNoteNotFound so the caller can skip that id and
// keep going; 401/403 come back as ErrAPIUnauthorized so the caller can abort
// before writing a partial store. A 403 additionally matches ErrAPIForbidden,
// which on this single-note endpoint is a verdict about the note rather than
// the credential — see that sentinel's comment.
func GetNote(c APIGetter, id string, withTranscript bool) (*APINote, error) {
	if c == nil {
		return nil, fmt.Errorf("nil api client")
	}
	if id == "" {
		return nil, fmt.Errorf("empty note id")
	}
	params := map[string]string{}
	if withTranscript {
		params["include"] = "transcript"
	}
	raw, err := c.Get("/v1/notes/"+id, params)
	if err != nil {
		return nil, classifyPublicAPIError(err, "get note "+id)
	}
	var note APINote
	if err := json.Unmarshal(raw, &note); err != nil {
		return nil, fmt.Errorf("get note %s: decoding response: %w", id, err)
	}
	if note.ID == "" {
		note.ID = id
	}
	return &note, nil
}

// classifyPublicAPIError maps client transport errors onto the sentinels
// above. Falls back to substring matching when the error is not a
// *client.APIError so that wrapped/retried failures still classify.
func classifyPublicAPIError(err error, what string) error {
	if err == nil {
		return nil
	}
	status := 0
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		status = apiErr.StatusCode
	} else {
		switch {
		case strings.Contains(err.Error(), "HTTP 404"):
			status = 404
		case strings.Contains(err.Error(), "HTTP 401"):
			status = 401
		case strings.Contains(err.Error(), "HTTP 403"):
			status = 403
		}
	}
	// Multiple %w verbs so the result matches BOTH the sentinels (errors.Is)
	// and the underlying *client.APIError (errors.As).
	switch status {
	case 404:
		return fmt.Errorf("%s: %w: %w", what, ErrNoteNotFound, err)
	case 401:
		return fmt.Errorf("%s: %w: %w", what, ErrAPIUnauthorized, err)
	case 403:
		// Carries the narrower sentinel too, so a caller working through a
		// list of ids can skip the forbidden one instead of discarding the
		// whole run. See ErrAPIForbidden.
		return fmt.Errorf("%s: %w: %w: %w", what, ErrAPIUnauthorized, ErrAPIForbidden, err)
	}
	return fmt.Errorf("%s: %w", what, err)
}
