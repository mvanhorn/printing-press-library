// Copyright 2026 cathrynlavery. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored under printing-press patch kit-honest-90.
// Preserved across regen-merge via .printing-press-patches.json.

package store

import (
	"encoding/json"
	"fmt"
)

// SearchBroadcasts narrows the generic FTS5 query to the broadcasts resource
// type. Use this when an agent or workflow needs to find broadcasts by
// subject or content without also matching subscribers, tags, or sequences.
// Returns results in FTS rank order, capped at limit (default 50).
func (s *Store) SearchBroadcasts(query string, limit int) ([]json.RawMessage, error) {
	return s.searchByResourceType("broadcasts", query, limit)
}

// SearchSubscribers narrows the generic FTS5 query to the subscribers
// resource type. Use this when an agent needs to find subscribers by name,
// email fragment, or custom-field content without false matches from
// broadcasts or tags that happen to contain the same words.
func (s *Store) SearchSubscribers(query string, limit int) ([]json.RawMessage, error) {
	return s.searchByResourceType("subscribers", query, limit)
}

func (s *Store) searchByResourceType(resourceType, query string, limit int) ([]json.RawMessage, error) {
	if limit <= 0 {
		limit = 50
	}
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	rows, err := s.db.Query(
		`SELECT r.data FROM resources r
		 JOIN resources_fts f ON r.id = f.id AND r.resource_type = f.resource_type
		 WHERE r.resource_type = ? AND resources_fts MATCH ?
		 ORDER BY rank
		 LIMIT ?`,
		resourceType, query, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []json.RawMessage
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		results = append(results, json.RawMessage(data))
	}
	return results, rows.Err()
}
