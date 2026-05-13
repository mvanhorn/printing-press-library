// Copyright 2026 Granola Printing Press contributors. Licensed under Apache-2.0.

package granola

import (
	"errors"
	"fmt"
)

// PATCH(encrypted-cache): Granola desktop stopped storing meeting documents
// in cache-v6.json around the same time the .enc encryption rolled out.
// The cache now holds transcripts/folders/recipes/panels/chats only;
// documents are fetched lazily from https://api.granola.ai/v2/get-documents.
//
// HydrateDocumentsFromAPI pages through that endpoint and stuffs the
// results into cache.Documents so the existing SyncFromCache path stays
// unchanged. SyncFromCache iterates cache.Documents to upsert meetings;
// after hydration that map carries everything sync used to read from
// cache.state.documents in the old shape.

// DefaultDocumentsPageSize matches Granola desktop's own page size for
// /v2/get-documents. Larger pages mean fewer round-trips but a higher
// chance of a 429 from the API limiter; 100 is the documented ceiling.
const DefaultDocumentsPageSize = 100

// HydrateDocumentsFromAPI populates cache.Documents from the internal
// API. It pages until the API signals end-of-stream - either via
// has_more=false in the response envelope, or by returning fewer than
// DefaultDocumentsPageSize rows. Returns the count of documents merged.
//
// Returns nil error on a fresh-install / no-documents case (the API may
// return an empty array). Returns the underlying error wrapped on
// network / auth / parse failures - callers MUST surface that error
// rather than silently leaving cache.Documents empty, because the
// existing meeting commands depend on it.
//
// Errors map onto well-known WorkOS / safestorage sentinels when
// applicable: ErrRefreshRefused fires if the access token expired and
// the source is the encrypted store (D6); callers should surface a
// "wake Granola desktop" hint rather than silently continuing.
func HydrateDocumentsFromAPI(cache *Cache, client *InternalClient) (int, error) {
	if cache == nil {
		return 0, fmt.Errorf("nil cache")
	}
	if client == nil {
		var err error
		client, err = NewInternalClient()
		if err != nil {
			return 0, fmt.Errorf("hydrate documents: %w", err)
		}
	}
	if cache.Documents == nil {
		cache.Documents = map[string]Document{}
	}

	const maxPages = 200 // hard cap to avoid runaway loops if API misbehaves
	added := 0
	page := 0
	for ; page < maxPages; page++ {
		offset := page * DefaultDocumentsPageSize
		env, err := client.GetDocumentsPage(DefaultDocumentsPageSize, offset, false)
		if err != nil {
			if errors.Is(err, ErrRefreshRefused) {
				return added, fmt.Errorf("hydrate documents: access token expired and refresh blocked for encrypted source - open Granola desktop briefly to refresh, then retry: %w", err)
			}
			return added, fmt.Errorf("hydrate documents page %d: %w", page, err)
		}
		if len(env.Docs) == 0 {
			break
		}
		for _, d := range env.Docs {
			if d.ID == "" {
				continue
			}
			cache.Documents[d.ID] = d
			added++
		}
		// Prefer the API's own has_more signal when present (wrapped envelope).
		// Fall back to short-page detection for the legacy bare-array shape,
		// which has no pagination metadata and may falsely report HasMore=false.
		if env.HasMore {
			continue
		}
		if len(env.Docs) < DefaultDocumentsPageSize {
			break
		}
		// Page was exactly full and has_more is not set. With the wrapped
		// envelope this means "no more pages"; with bare-array it's
		// ambiguous. Bias toward trusting the explicit signal - the
		// modern envelope is wrapped, and the cost of under-fetching on
		// the rare bare-array exactly-full-page edge case is much smaller
		// than the cost of always making one extra round-trip.
		break
	}
	// If we exited via the for-condition rather than a break, the API
	// kept signaling more documents past maxPages. Surface this so the
	// caller sees the truncation rather than silently under-reporting.
	if page == maxPages {
		return added, fmt.Errorf("hydrate documents: hit page cap (%d pages * %d per page = %d documents); some documents may be missing - re-run sync or raise maxPages", maxPages, DefaultDocumentsPageSize, added)
	}
	return added, nil
}
