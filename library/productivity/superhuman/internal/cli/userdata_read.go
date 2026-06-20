// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// userdata_read.go is a small helper layer over Superhuman's
// `/v3/userdata.read` endpoint. The endpoint is the read-side counterpart
// to `/v3/userdata.write` — see messages_readstatus.go for the canonical
// usage pattern.
//
// Implementation-time unknown: the response wrapper shape varies across
// /v3/userdata.read callers. For drafts, the bundle returns either the
// raw draftValue object, or a {reads:[{value: draftValue}]} wrapper, or
// a {data: draftValue} wrapper. unmarshalDraftValue tries each shape in
// turn before giving up so a backend tweak does not require a CLI fix.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/client"
)

// draftReadPathFor returns the userdata.read path for a draft. Mirrors the
// shape used by drafts_discard / writeMessage so the validator is happy.
func draftReadPathFor(providerID, draftID string) string {
	return fmt.Sprintf("users/%s/threads/%s/messages/%s/draft", providerID, draftID, draftID)
}

// readDraft fetches the server-side draftValue for the given draft id.
// Returns ErrDraftNotFound when the response decodes successfully but no
// draft body comes back (deleted, never persisted, or wrong provider id).
func readDraft(c *client.Client, providerID, draftID string) (draftValue, int, error) {
	body := map[string]any{
		"reads": []map[string]any{
			{"path": draftReadPathFor(providerID, draftID)},
		},
		"pageToken": nil,
		"pageSize":  nil,
	}
	data, statusCode, err := c.Post("/v3/userdata.read", body)
	if err != nil {
		return draftValue{}, statusCode, err
	}
	dv, ok := unmarshalDraftValue(data)
	if !ok {
		return draftValue{}, statusCode, ErrDraftNotFound
	}
	return dv, statusCode, nil
}

// ErrDraftNotFound is the sentinel returned by readDraft when the response
// did not carry a draft body in any of the known wrapper shapes.
var ErrDraftNotFound = fmt.Errorf("draft not found")

// resolveDraftViaThreadList fetches the draft list via /v3/userdata.getThreads
// and returns the draftValue whose message id, thread id, or containing-thread
// id matches draftID.
//
// PATCH(drafts-get-via-getthreads): the single-id userdata.read path
// (users/<uid>/threads/<id>/messages/<id>/draft, same id twice) does not
// resolve, because real Superhuman drafts have a *distinct* thread id and
// message id (e.g. threads/draft007cf1…/messages/draft00f93…). getThreads —
// which returns the full draftValue per message — is the reliable lookup,
// so drafts get resolves through it instead of guessing the read path.
// threadListEntry is one entry in a /v3/userdata.getThreads response.
type threadListEntry struct {
	ID     string `json:"id"`
	Thread struct {
		Messages map[string]struct {
			Draft draftValue `json:"draft"`
		} `json:"messages"`
	} `json:"thread"`
}

func resolveDraftViaThreadList(c *client.Client, draftID string) (draftValue, int, error) {
	// getThreads caps limit at 100 (>100 returns HTTP 400), so page through
	// the draft list by offset until the draft is found or the list ends.
	// maxPages bounds the walk (100 pages = 10k drafts) so a backend that
	// never returns a short page can't spin forever.
	const pageSize = 100
	const maxPages = 100
	lastStatus := 0
	for page := 0; page < maxPages; page++ {
		body := map[string]any{
			"filter": map[string]any{"type": "draft"},
			"limit":  pageSize,
			"offset": page * pageSize,
		}
		data, statusCode, err := c.Post("/v3/userdata.getThreads", body)
		if err != nil {
			return draftValue{}, statusCode, err
		}
		lastStatus = statusCode
		// getThreads returns {threadList:[...]} either at the top level or
		// under a {data:{...}} wrapper depending on the response path.
		var resp struct {
			ThreadList []threadListEntry `json:"threadList"`
			Data       struct {
				ThreadList []threadListEntry `json:"threadList"`
			} `json:"data"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			return draftValue{}, statusCode, err
		}
		threads := resp.ThreadList
		if len(threads) == 0 {
			threads = resp.Data.ThreadList
		}
		if len(threads) == 0 {
			break
		}
		for _, t := range threads {
			for _, m := range t.Thread.Messages {
				if m.Draft.ID == draftID || m.Draft.ThreadID == draftID || t.ID == draftID {
					return m.Draft, statusCode, nil
				}
			}
		}
		if len(threads) < pageSize {
			break
		}
	}
	return draftValue{}, lastStatus, ErrDraftNotFound
}

// unmarshalDraftValue tries the four known response shapes for
// /v3/userdata.read against a draft path:
//
//  1. {data:{results:[{path, value: draftValue}]}} — the live shape
//     used by runCancelSchedule's extractDraftValueForCancel helper.
//  2. Bare draftValue object — `{"id":"draft00…", …}`.
//  3. {data: draftValue}  — mirrors the threads.get wrapper.
//  4. {reads:[{value: draftValue}]} — mirrors the writes-array shape.
//
// Returns ok=false if none of them match. The function intentionally
// does not require every field to be present, since the validator at the
// write side is stricter than the read side may return.
func unmarshalDraftValue(data json.RawMessage) (draftValue, bool) {
	// Shape 1: {data:{results:[{value: draftValue}]}} — the live shape.
	var resultsWrap struct {
		Data struct {
			Results []struct {
				Value draftValue `json:"value"`
			} `json:"results"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &resultsWrap); err == nil {
		for _, r := range resultsWrap.Data.Results {
			if r.Value.ID != "" {
				return r.Value, true
			}
		}
	}
	// Shape 2: bare object.
	var bare draftValue
	if err := json.Unmarshal(data, &bare); err == nil && bare.ID != "" {
		return bare, true
	}
	// Shape 3: {data: draftValue}.
	var wrapped struct {
		Data draftValue `json:"data"`
	}
	if err := json.Unmarshal(data, &wrapped); err == nil && wrapped.Data.ID != "" {
		return wrapped.Data, true
	}
	// Shape 4: {reads:[{value: draftValue}]}.
	var readsWrap struct {
		Reads []struct {
			Value draftValue `json:"value"`
		} `json:"reads"`
	}
	if err := json.Unmarshal(data, &readsWrap); err == nil && len(readsWrap.Reads) > 0 && readsWrap.Reads[0].Value.ID != "" {
		return readsWrap.Reads[0].Value, true
	}
	return draftValue{}, false
}
