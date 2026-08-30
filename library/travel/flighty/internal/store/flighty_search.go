// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored Flighty domain search: FTS-backed airport lookup over the
// generic resources FTS index. Used by identifier resolution so single-airport
// lookups hit the FTS index instead of loading the whole catalog.
package store

import (
	"context"
	"encoding/json"
	"fmt"
)

// SearchAirports returns up to limit airport rows matching the free-text
// query (IATA, name, city, or slug) from the local mirror, best match first.
// The returned rows are the stored catalog JSON blobs.
func (s *Store) SearchAirports(ctx context.Context, query string, limit int) ([]json.RawMessage, error) {
	if limit <= 0 {
		limit = 5
	}
	matchQuery := FTSMatchQuery(query)
	if matchQuery == "" {
		return []json.RawMessage{}, nil
	}
	rows, err := s.DB().QueryContext(ctx, `
		SELECT r.data FROM resources r
		JOIN resources_fts f ON r.id = f.id AND r.resource_type = f.resource_type
		WHERE resources_fts MATCH ?
		AND r.resource_type = 'airports'
		ORDER BY f.rank
		LIMIT ?`, matchQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("searching airports: %w", err)
	}
	defer rows.Close()
	results := make([]json.RawMessage, 0, limit)
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("scanning airport search row: %w", err)
		}
		results = append(results, json.RawMessage(data))
	}
	return results, rows.Err()
}
