// Copyright 2026 rob-coco. Licensed under Apache-2.0. See LICENSE.

// T4 — Top-tracks rotation drift.
//
//	top drift --range short|medium|long [--since <date>]
//
// Two-row diff over `top_tracks_snapshot` filtered by `time_range`. Emits
// risen/fallen/stable cohorts. Spotify's top-tracks endpoint returns
// "current" only — without local snapshotting there is no drift answer.
// This is the product thesis's headline transcendence move.

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/spotify/internal/cliutil"
)

func newTopDriftCmd(flags *rootFlags) *cobra.Command {
	var rangeArg string
	var sinceArg string
	cmd := &cobra.Command{
		Use:   "drift --range short|medium|long [--since <date>]",
		Short: "Compare top-tracks snapshots to see who rose, fell, or stayed",
		Long: `Compares two snapshots in top_tracks_snapshot keyed on time_range. The
current snapshot is the most recent capture; the prior snapshot is the most
recent capture BEFORE the --since date (default: 28 days ago).

Requires at least two prior top-tracks captures (call 'top tracks --range
<range>' or 'sync' twice over time). Drift is the cohort change between
those two captures.`,
		Example:     "  spotify-pp-cli top drift --range medium --since 2026-04-01",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			switch rangeArg {
			case "short", "medium", "long":
			default:
				return usageErr(fmt.Errorf("--range must be short, medium, or long"))
			}
			timeRange := rangeArg + "_term"

			var since time.Time
			if sinceArg != "" {
				t, err := time.Parse("2006-01-02", sinceArg)
				if err != nil {
					return usageErr(fmt.Errorf("--since must be YYYY-MM-DD: %w", err))
				}
				since = t
			} else {
				since = time.Now().AddDate(0, 0, -28)
			}

			db, err := openTranscendenceStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "range": rangeArg}, flags)
			}

			result, err := computeTopDrift(db.DB(), timeRange, since)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&rangeArg, "range", "medium", "Time range: short, medium, or long")
	cmd.Flags().StringVar(&sinceArg, "since", "", "Compare against the most recent snapshot before this date (YYYY-MM-DD). Default: 28 days ago.")
	return cmd
}

// topDriftResult is T4's output shape.
type topDriftResult struct {
	Range      string           `json:"range"`
	CurrentAt  string           `json:"current_at"`
	PriorAt    string           `json:"prior_at"`
	Risen      []map[string]any `json:"risen"`
	Fallen     []map[string]any `json:"fallen"`
	Added      []map[string]any `json:"added"`
	Dropped    []map[string]any `json:"dropped"`
	Stable     []map[string]any `json:"stable"`
	IsBaseline bool             `json:"is_baseline,omitempty"`
}

func computeTopDrift(db storeQueryer, timeRange string, before time.Time) (*topDriftResult, error) {
	// Find the most recent snapshot.
	var currentAt string
	if err := db.QueryRow(`SELECT MAX(captured_at) FROM top_tracks_snapshot WHERE time_range = ?`, timeRange).Scan(&currentAt); err != nil {
		// Empty table is fine — return baseline.
	}
	if currentAt == "" {
		return &topDriftResult{Range: timeRange, IsBaseline: true, Risen: []map[string]any{}, Fallen: []map[string]any{}, Added: []map[string]any{}, Dropped: []map[string]any{}, Stable: []map[string]any{}}, nil
	}

	// Find the most recent snapshot strictly before `before`.
	var priorAt string
	row := db.QueryRow(`SELECT MAX(captured_at) FROM top_tracks_snapshot
		WHERE time_range = ? AND captured_at < ? AND captured_at != ?`,
		timeRange, before.UTC().Format(time.RFC3339), currentAt)
	_ = row.Scan(&priorAt)

	if priorAt == "" {
		// Fall back: any prior snapshot at all.
		row := db.QueryRow(`SELECT MAX(captured_at) FROM top_tracks_snapshot
			WHERE time_range = ? AND captured_at < ?`, timeRange, currentAt)
		_ = row.Scan(&priorAt)
	}

	result := &topDriftResult{
		Range:     timeRange,
		CurrentAt: currentAt,
		PriorAt:   priorAt,
		Risen:     []map[string]any{},
		Fallen:    []map[string]any{},
		Added:     []map[string]any{},
		Dropped:   []map[string]any{},
		Stable:    []map[string]any{},
	}
	if priorAt == "" {
		result.IsBaseline = true
		return result, nil
	}

	curr, err := topSnapshotPositions(db, "top_tracks_snapshot", "track_id", timeRange, currentAt)
	if err != nil {
		return nil, err
	}
	prior, err := topSnapshotPositions(db, "top_tracks_snapshot", "track_id", timeRange, priorAt)
	if err != nil {
		return nil, err
	}

	for id, info := range curr {
		if pinfo, ok := prior[id]; ok {
			switch {
			case pinfo.position > info.position:
				result.Risen = append(result.Risen, map[string]any{"id": id, "name": info.name, "from": pinfo.position, "to": info.position})
			case pinfo.position < info.position:
				result.Fallen = append(result.Fallen, map[string]any{"id": id, "name": info.name, "from": pinfo.position, "to": info.position})
			default:
				result.Stable = append(result.Stable, map[string]any{"id": id, "name": info.name, "position": info.position})
			}
		} else {
			result.Added = append(result.Added, map[string]any{"id": id, "name": info.name, "position": info.position})
		}
	}
	for id, info := range prior {
		if _, ok := curr[id]; !ok {
			result.Dropped = append(result.Dropped, map[string]any{"id": id, "name": info.name, "prior_position": info.position})
		}
	}
	return result, nil
}

func topSnapshotPositions(db storeQueryer, table, idCol, timeRange, capturedAt string) (map[string]snapTrackInfo, error) {
	q := fmt.Sprintf(`SELECT %s, position, COALESCE(track_name, '') FROM %s WHERE time_range = ? AND captured_at = ?`, idCol, table)
	// top_artists_snapshot uses artist_name; switch column when needed.
	if table == "top_artists_snapshot" {
		q = fmt.Sprintf(`SELECT %s, position, COALESCE(artist_name, '') FROM %s WHERE time_range = ? AND captured_at = ?`, idCol, table)
	}
	rows, err := db.Query(q, timeRange, capturedAt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]snapTrackInfo{}
	for rows.Next() {
		var id, name string
		var position int
		if err := rows.Scan(&id, &position, &name); err != nil {
			return nil, err
		}
		out[id] = snapTrackInfo{position: position, name: name}
	}
	return out, rows.Err()
}
