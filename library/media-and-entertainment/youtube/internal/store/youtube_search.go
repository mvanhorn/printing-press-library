// Copyright 2026 Justin and contributors. Licensed under Apache-2.0. See LICENSE.

// Domain-named search entry point for the YouTube store. Callers use
// SearchYoutube so the data pipeline reads as domain code; it delegates to the
// generated FTS search. Hand-authored file — preserved across regeneration.

package store

import "encoding/json"

// SearchYoutube runs the full-text search over locally cached YouTube
// resources (videos, channels, playlists, search results, transcripts).
func (s *Store) SearchYoutube(query string, limit int, resourceTypes ...string) ([]json.RawMessage, error) {
	return s.Search(query, limit, resourceTypes...)
}
