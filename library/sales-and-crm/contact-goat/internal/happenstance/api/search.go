// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Status enum values for search and research async jobs. Mirror the upstream
// OpenAPI shape verbatim so callers can compare against the constants rather
// than scattering literal strings.
const (
	StatusRunning          = "RUNNING"
	StatusCompleted        = "COMPLETED"
	StatusFailed           = "FAILED"
	StatusFailedAmbiguous  = "FAILED_AMBIGUOUS"
)

// DefaultPollTimeout matches the cookie surface's poll ceiling. Both surfaces
// converge on 180s after real-session evidence that Happenstance routinely
// takes 2-5 minutes to finish a search. See internal/client/people_search.go.
const DefaultPollTimeout = 180 * time.Second

// DefaultPollInterval matches the cookie surface's polling cadence. The web
// UI polls about every 1-2s; we pick the lower bound to surface results as
// soon as the server marks COMPLETED.
const DefaultPollInterval = 1 * time.Second

// SearchOptions configures POST /v1/search. Mirrors SearchPeopleOptions in
// internal/client/people_search.go but tracks the public-API field names and
// surface (group_ids, include_friends_connections, include_my_connections).
//
// A nil *SearchOptions on Search() means "defaults": no group filter, both
// connection scopes off (the public API defaults to a global search if both
// flags are false; callers who want to scope to their network must opt in).
type SearchOptions struct {
	// GroupIDs filters the search to members of the named Happenstance
	// groups. Look these up via GET /v1/groups (Client.Groups).
	GroupIDs []string

	// IncludeFriendsConnections widens the search to include the connections
	// of your Happenstance friends (2nd-degree on the friend graph).
	IncludeFriendsConnections bool

	// IncludeMyConnections includes your own LinkedIn-synced connections
	// (1st-degree). Defaults off; turn on when you want a strict
	// in-network search.
	IncludeMyConnections bool
}

// PollSearchOptions controls PollSearch's loop behavior. A nil pointer means
// "defaults": 180s timeout, 1s interval.
type PollSearchOptions struct {
	// Timeout bounds the total wall-clock spent polling before giving up.
	// Zero falls back to DefaultPollTimeout.
	Timeout time.Duration

	// Interval is the delay between successive GetSearch calls. Zero falls
	// back to DefaultPollInterval.
	Interval time.Duration

	// PageID, when non-empty, is forwarded to GetSearch on every poll so
	// FindMore-paginated results stream into the caller. Empty for the
	// initial parent search.
	PageID string
}

// createSearchRequest is the POST /v1/search body. Field names match the
// public-API OpenAPI spec verbatim.
type createSearchRequest struct {
	Text                      string   `json:"text"`
	GroupIDs                  []string `json:"group_ids,omitempty"`
	IncludeFriendsConnections bool     `json:"include_friends_connections,omitempty"`
	IncludeMyConnections      bool     `json:"include_my_connections,omitempty"`
}

// Search calls POST /v1/search. The returned envelope contains the asynchronous
// search id; callers must poll via PollSearch (or GetSearch in a custom loop)
// until Status is COMPLETED. Costs 2 credits.
func (c *Client) Search(ctx context.Context, text string, opts *SearchOptions) (SearchEnvelope, error) {
	if strings.TrimSpace(text) == "" {
		return SearchEnvelope{}, errors.New("happenstance api: search text is empty")
	}
	body := createSearchRequest{Text: text}
	if opts != nil {
		body.GroupIDs = opts.GroupIDs
		body.IncludeFriendsConnections = opts.IncludeFriendsConnections
		body.IncludeMyConnections = opts.IncludeMyConnections
	}

	raw, err := c.do(ctx, http.MethodPost, "/search", body)
	if err != nil {
		return SearchEnvelope{}, err
	}
	var env SearchEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return SearchEnvelope{}, fmt.Errorf("happenstance api: decoding /search response: %w", err)
	}
	return env, nil
}

// GetSearch calls GET /v1/search/{id}. Free probe — no credits spent.
// pageID is optional; when non-empty it is forwarded as ?page_id= so the
// server returns the next page of a FindMore-paginated search.
func (c *Client) GetSearch(ctx context.Context, id, pageID string) (SearchEnvelope, error) {
	if strings.TrimSpace(id) == "" {
		return SearchEnvelope{}, errors.New("happenstance api: GetSearch requires a non-empty search id")
	}
	path := "/search/" + url.PathEscape(id)
	if pageID != "" {
		path += "?page_id=" + url.QueryEscape(pageID)
	}
	raw, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return SearchEnvelope{}, err
	}
	var env SearchEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return SearchEnvelope{}, fmt.Errorf("happenstance api: decoding /search/%s response: %w", id, err)
	}
	return env, nil
}

// FindMore calls POST /v1/search/{id}/find-more on a parent search. Returns
// the new page id; the CLI surface in unit 6 stitches that back into a
// re-poll on the parent search via GetSearch(ctx, parentID, pageID).
//
// Costs 2 credits. The upstream API rejects find-more on non-parent searches
// (i.e. searches that were themselves spawned by a previous find-more) with
// 422; that error is surfaced verbatim so the caller can react.
func (c *Client) FindMore(ctx context.Context, parentSearchID string) (FindMoreEnvelope, error) {
	if strings.TrimSpace(parentSearchID) == "" {
		return FindMoreEnvelope{}, errors.New("happenstance api: FindMore requires a non-empty parent search id")
	}
	path := "/search/" + url.PathEscape(parentSearchID) + "/find-more"
	raw, err := c.do(ctx, http.MethodPost, path, struct{}{})
	if err != nil {
		// Map the generic 422 message into something callers can grep on.
		// The upstream payload is the source of truth ("parent search only"
		// language is what the docs use); we surface the literal string so
		// the caller's error-classification heuristics can match.
		if strings.Contains(err.Error(), "422 unprocessable") {
			return FindMoreEnvelope{}, fmt.Errorf("%w (find-more is callable on a parent search only)", err)
		}
		return FindMoreEnvelope{}, err
	}
	var env FindMoreEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return FindMoreEnvelope{}, fmt.Errorf("happenstance api: decoding /search/%s/find-more response: %w", parentSearchID, err)
	}
	return env, nil
}

// PollSearch repeatedly calls GetSearch until Status is COMPLETED, FAILED, or
// FAILED_AMBIGUOUS, or until the configured timeout fires. Returns the final
// envelope on terminal status; on timeout returns the last polled envelope
// (Status will still be RUNNING) without an error so callers can render
// partial progress. Honors ctx.Done(): if the context is cancelled mid-loop
// the function returns ctx.Err() immediately.
func (c *Client) PollSearch(ctx context.Context, id string, opts *PollSearchOptions) (SearchEnvelope, error) {
	if strings.TrimSpace(id) == "" {
		return SearchEnvelope{}, errors.New("happenstance api: PollSearch requires a non-empty search id")
	}
	timeout := DefaultPollTimeout
	interval := DefaultPollInterval
	pageID := ""
	if opts != nil {
		if opts.Timeout > 0 {
			timeout = opts.Timeout
		}
		if opts.Interval > 0 {
			interval = opts.Interval
		}
		pageID = opts.PageID
	}

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var last SearchEnvelope
	for {
		env, err := c.GetSearch(ctx, id, pageID)
		if err != nil {
			return SearchEnvelope{}, err
		}
		last = env
		if isTerminalStatus(env.Status) {
			return env, nil
		}
		if time.Now().After(deadline) {
			// Return the last polled envelope without an error. The caller
			// inspects env.Status; RUNNING signals "timed out, server still
			// working" the same way the cookie surface's Completed=false does.
			return last, nil
		}
		select {
		case <-ctx.Done():
			return SearchEnvelope{}, ctx.Err()
		case <-ticker.C:
		}
	}
}

// isTerminalStatus reports whether the async-job status indicates the server
// is done with this id (success or failure). RUNNING is the only non-terminal
// status both the search and research endpoints use.
func isTerminalStatus(status string) bool {
	switch status {
	case StatusCompleted, StatusFailed, StatusFailedAmbiguous:
		return true
	default:
		return false
	}
}

// FormatGroupMention formats a group display name as it should appear inside
// a search request body's text field. The OpenAPI shape documents groups as
// @-mentions but does not pin down the exact quoting rule for names with
// spaces; the simplest unambiguous form is `@"Display Name"` and that is
// what this helper emits. Real-server behavior may force a tweak (e.g. to
// drop the quotes when the name has no spaces, or to expect underscores
// instead) — when that happens, change this single function and every
// call site stays correct.
func FormatGroupMention(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if strings.ContainsAny(name, " \t\"") {
		// Escape any embedded double-quote so the mention round-trips.
		escaped := strings.ReplaceAll(name, `"`, `\"`)
		return "@\"" + escaped + "\""
	}
	return "@" + name
}
