// Copyright 2026 ryanc00per and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/devices/googlehealth/internal/health"
	"github.com/mvanhorn/printing-press-library/library/devices/googlehealth/internal/store"
)

// loadHealthPoints reads every synced row from the local store and
// extracts the recognizable Google Health data points, discarding
// non-point rows (profile, settings, identity). It is the shared input
// for the trends, streaks, and correlate analytics commands.
func loadHealthPoints(db *store.Store) ([]health.Point, error) {
	rows, err := db.Query(`SELECT data FROM resources`)
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
	return health.ExtractPoints(raws), nil
}
