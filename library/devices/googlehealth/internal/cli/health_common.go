// Copyright 2026 ryanc00per and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/googlehealth/internal/health"
	"github.com/mvanhorn/printing-press-library/library/devices/googlehealth/internal/store"
)

// loadHealthPoints reads the synced Google Health data-point rows from the
// local store and extracts the recognizable points. It is the shared input
// for the trends, streaks, and correlate analytics commands.
//
// The query is scoped to the known data-point resource types (the same set
// sync writes) so non-point rows such as profile, settings, and identity are
// never loaded. ExtractPoints would discard those rows anyway, so the filter
// is behavior-preserving — it just avoids the wasted I/O of pulling every row
// into memory on a large store. The resources table has an index on
// resource_type, so the IN filter is cheap.
func loadHealthPoints(db *store.Store) ([]health.Point, error) {
	dataTypes := googleHealthDataTypes()
	placeholders := make([]string, len(dataTypes))
	args := make([]any, len(dataTypes))
	for i, t := range dataTypes {
		placeholders[i] = "?"
		args[i] = t
	}
	query := fmt.Sprintf(
		"SELECT data FROM resources WHERE resource_type IN (%s)",
		strings.Join(placeholders, ","),
	)
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying local data: %w", err)
	}
	defer rows.Close()

	var raws [][]byte
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			continue
		}
		raws = append(raws, data)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading local data: %w", err)
	}
	return health.ExtractPoints(raws), nil
}
