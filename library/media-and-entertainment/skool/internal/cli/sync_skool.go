// Copyright 2026 Zain Haseeb and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel feature; not generated.
//
// PATCH(amend-2026-08-12: hydrate posts and members into the local mirror)
// The generated flat-list sync can only walk resources whose spec declares a
// plain `list` endpoint returning a top-level array or a {data:[...]}-shaped
// envelope. Skool has neither: posts and members are rendered inside the
// community page's Next.js `pageProps` envelope. Before this file the only
// syncable resource was `notifications`, which left the SQLite mirror, FTS
// search, and cross-community SQL with nothing to operate on.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/skool/internal/store"
)

// skoolCommunityResources are the resources hydrated from the community page
// envelope rather than through the generic flat-list sync loop.
var skoolCommunityResources = map[string]bool{
	"posts":   true,
	"members": true,
}

func isSkoolCommunityResource(resource string) bool {
	return skoolCommunityResources[resource]
}

// syncSkoolCommunityResource paginates the community page and writes the
// unwrapped records to the local store. `community` is the slug resolved by
// the sync command (--community, SKOOL_COMMUNITY, or template_vars.community).
func syncSkoolCommunityResource(c interface {
	Get(string, map[string]string) (json.RawMessage, error)
}, db *store.Store, resource, community string, maxPages int) syncResult {
	started := time.Now()

	if !humanFriendly {
		fmt.Fprintf(os.Stdout, `{"event":"sync_start","resource":"%s"}`+"\n", resource)
	}

	if community == "" {
		// The aggregation loop in sync.go is the single sync_error emission
		// point; emitting here too would double-report every failure.
		err := fmt.Errorf("syncing %s needs a community: pass --community <slug>, set SKOOL_COMMUNITY, or set template_vars.community in the config", resource)
		return syncResult{Resource: resource, Err: err, Duration: time.Since(started)}
	}

	path := "/_next/data/{buildId}/{community}.json"
	path = replacePathParam(path, "community", community)

	if maxPages <= 0 || maxPages > 100 {
		maxPages = 100
	}

	seen := map[string]struct{}{}
	var collected []json.RawMessage
	var lastKeys []string

	for page := 1; page <= maxPages; page++ {
		params := map[string]string{"g": community}
		if resource == "members" {
			params["t"] = "members"
		}
		if page > 1 {
			params["p"] = strconv.Itoa(page)
		}

		data, err := c.Get(path, params)
		if err != nil {
			if w, ok := isSyncAccessWarning(err); ok {
				if !humanFriendly {
					fmt.Fprintf(os.Stdout, `{"event":"sync_warning","resource":"%s","status":%d,"reason":"%s"}`+"\n", resource, w.Status, w.Reason)
				}
				return syncResult{Resource: resource, Count: len(collected), Warn: fmt.Errorf("skipped %s: %s", resource, w.Reason), Duration: time.Since(started)}
			}
			return syncResult{Resource: resource, Count: len(collected), Err: fmt.Errorf("fetching %s page %d: %w", resource, page, err), Duration: time.Since(started)}
		}

		items, keys := extractSkoolPageRecords(data, resource)
		lastKeys = keys
		added := 0
		for _, item := range items {
			id := recordIdentity(item)
			if id == "" {
				continue
			}
			if _, dup := seen[id]; dup {
				continue
			}
			seen[id] = struct{}{}
			collected = append(collected, item)
			added++
		}
		if added == 0 {
			break
		}
	}

	if len(collected) == 0 {
		// No records and no transport error: the envelope key we expect is
		// absent. Name the keys that were present (names only, never values)
		// so the shape drift is diagnosable without a debugger.
		warn := fmt.Errorf("no %s records found in the community page envelope (pageProps keys seen: %v)", resource, lastKeys)
		if !humanFriendly {
			fmt.Fprintf(os.Stdout, `{"event":"sync_warning","resource":"%s","reason":"envelope_shape_unrecognized","message":%q}`+"\n", resource, warn.Error())
		} else {
			fmt.Fprintf(os.Stderr, "  %s: warning: %v\n", resource, warn)
		}
		return syncResult{Resource: resource, Warn: warn, Duration: time.Since(started)}
	}

	stored, _, err := db.UpsertBatch(resource, collected)
	if err != nil {
		return syncResult{Resource: resource, Err: err, Duration: time.Since(started)}
	}
	_ = db.SaveSyncState(resource, "", stored)

	if !humanFriendly {
		fmt.Fprintf(os.Stdout, `{"event":"sync_complete","resource":"%s","total":%d}`+"\n", resource, stored)
	}
	return syncResult{Resource: resource, Count: stored, Duration: time.Since(started)}
}

// extractSkoolPageRecords unwraps the records for `resource` out of a Next.js
// `pageProps` envelope. Posts have a known, stable shape
// (pageProps.postTrees[].post). Members do not: the members tab has shipped
// under more than one envelope key, so the member path scans pageProps for the
// first array of user-shaped objects instead of hard-coding a key that a Skool
// deploy can rename. The second return value is the list of pageProps keys
// seen, used only to describe shape drift in a warning.
func extractSkoolPageRecords(data json.RawMessage, resource string) ([]json.RawMessage, []string) {
	var env struct {
		PageProps map[string]json.RawMessage `json:"pageProps"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, nil
	}
	keys := make([]string, 0, len(env.PageProps))
	for k := range env.PageProps {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if resource == "posts" {
		raw, ok := env.PageProps["postTrees"]
		if !ok {
			return nil, keys
		}
		var trees []struct {
			Post json.RawMessage `json:"post"`
		}
		if err := json.Unmarshal(raw, &trees); err != nil {
			return nil, keys
		}
		out := make([]json.RawMessage, 0, len(trees))
		for _, t := range trees {
			if len(t.Post) > 0 {
				out = append(out, t.Post)
			}
		}
		return out, keys
	}

	// members: first array of user-shaped objects wins.
	for _, k := range keys {
		if out := userShapedArray(env.PageProps[k]); len(out) > 0 {
			return out, keys
		}
	}
	// One level deeper — the members tab has nested its list under a wrapper
	// object (e.g. {"usersData":{"users":[...]}}) in some Skool builds.
	for _, k := range keys {
		var nested map[string]json.RawMessage
		if json.Unmarshal(env.PageProps[k], &nested) != nil {
			continue
		}
		nestedKeys := make([]string, 0, len(nested))
		for nk := range nested {
			nestedKeys = append(nestedKeys, nk)
		}
		sort.Strings(nestedKeys)
		for _, nk := range nestedKeys {
			if out := userShapedArray(nested[nk]); len(out) > 0 {
				return out, keys
			}
		}
	}
	return nil, keys
}

// userShapedArray returns the elements of raw when raw is a JSON array whose
// first element looks like a Skool user record, either directly or wrapped as
// {"user": {...}}. Wrapped entries are unwrapped to the inner user object so
// the stored row shape matches what `search` and `sql` expect.
func userShapedArray(raw json.RawMessage) []json.RawMessage {
	var arr []json.RawMessage
	if json.Unmarshal(raw, &arr) != nil || len(arr) == 0 {
		return nil
	}
	unwrapped := make([]json.RawMessage, 0, len(arr))
	for _, el := range arr {
		var obj map[string]json.RawMessage
		if json.Unmarshal(el, &obj) != nil {
			return nil
		}
		if inner, ok := obj["user"]; ok && looksLikeUser(inner) {
			unwrapped = append(unwrapped, inner)
			continue
		}
		if !looksLikeUser(el) {
			return nil
		}
		unwrapped = append(unwrapped, el)
	}
	return unwrapped
}

func looksLikeUser(raw json.RawMessage) bool {
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return false
	}
	if _, ok := obj["id"]; !ok {
		return false
	}
	for _, k := range []string{"firstName", "lastName", "name"} {
		if _, ok := obj[k]; ok {
			return true
		}
	}
	return false
}

// recordIdentity returns the record's id for in-run dedupe. It deliberately
// mirrors nothing more than the id field — UpsertBatch owns primary-key
// resolution for the write itself.
func recordIdentity(raw json.RawMessage) string {
	var obj struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(raw, &obj) != nil {
		return ""
	}
	return obj.ID
}
