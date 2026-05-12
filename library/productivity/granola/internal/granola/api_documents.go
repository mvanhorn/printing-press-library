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
// API. It pages until the API returns fewer than DefaultDocumentsPageSize
// rows or has_more is false. Returns the count of documents merged.
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
	for page := 0; page < maxPages; page++ {
		offset := page * DefaultDocumentsPageSize
		docs, err := client.GetDocuments(DefaultDocumentsPageSize, offset, false)
		if err != nil {
			if errors.Is(err, ErrRefreshRefused) {
				return added, fmt.Errorf("hydrate documents: access token expired and refresh blocked for encrypted source - open Granola desktop briefly to refresh, then retry: %w", err)
			}
			return added, fmt.Errorf("hydrate documents page %d: %w", page, err)
		}
		if len(docs) == 0 {
			break
		}
		for _, d := range docs {
			if d.ID == "" {
				continue
			}
			cache.Documents[d.ID] = d
			added++
		}
		if len(docs) < DefaultDocumentsPageSize {
			break
		}
	}
	return added, nil
}
