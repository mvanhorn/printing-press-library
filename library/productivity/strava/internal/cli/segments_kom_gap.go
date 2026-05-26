// Copyright 2026 azaaron. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/strava/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/strava/internal/store"
)

type komGapRow struct {
	SegmentID   string  `json:"segment_id"`
	Name        string  `json:"name"`
	Distance    float64 `json:"distance_km"`
	MyBestSec   int     `json:"my_best_seconds"`
	MyBest      string  `json:"my_best"`
	KOMSec      int     `json:"kom_seconds"`
	KOM         string  `json:"kom"`
	GapSec      int     `json:"gap_seconds"`
	GapPct      float64 `json:"gap_pct"`
}

func newSegmentsKomGapCmd(flags *rootFlags) *cobra.Command {
	var topN int
	var dbPath string

	cmd := &cobra.Command{
		Use:         "kom-gap",
		Short:       "See your gap to the KOM on each starred segment",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `For each starred segment, fetches your best effort from local SQLite and
the current leaderboard leader time from the Strava API, then ranks by
the gap you're closest to closing.

Requires synced data: run 'strava-pp-cli sync' first.
Requires read scope for leaderboard access.`,
		Example: strings.Trim(`
  strava-pp-cli segments kom-gap
  strava-pp-cli segments kom-gap --top 10 --agent
  strava-pp-cli segments kom-gap --json --select name,my_best,kom,gap_pct`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				sample := []komGapRow{
					{SegmentID: "229781", Name: "Hawk Hill", Distance: 3.2,
						MyBestSec: 780, MyBest: "13:00", KOMSec: 720, KOM: "12:00",
						GapSec: 60, GapPct: 8.3},
				}
				return printJSONFiltered(cmd.OutOrStdout(), sample, flags)
			}

			if dbPath == "" {
				dbPath = defaultDBPath("strava-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w\nRun 'strava-pp-cli sync' first", err)
			}
			defer db.Close()

			// Get starred segments from local store
			query := `SELECT id, data FROM resources WHERE resource_type IN ('segments', 'starred')`
			rows, err := db.DB().QueryContext(cmd.Context(), query)
			if err != nil {
				return fmt.Errorf("querying starred segments: %w", err)
			}
			defer rows.Close()

			type segInfo struct {
				id       string
				name     string
				distance float64
			}
			var segments []segInfo
			for rows.Next() {
				var id, data sql.NullString
				if err := rows.Scan(&id, &data); err != nil || !data.Valid {
					continue
				}
				var seg map[string]any
				if err := json.Unmarshal([]byte(data.String), &seg); err != nil {
					continue
				}
				// Handle two schemas stored by sync:
				// 'segments' rows: flat {id, name, distance, ...}
				// 'starred' rows:  {id, segment: {id, name, distance, ...}}
				name, _ := seg["name"].(string)
				dist := jsonFloat(seg, "distance")
				if name == "" || dist == 0 {
					if nested, ok := seg["segment"].(map[string]any); ok {
						if name == "" {
							name, _ = nested["name"].(string)
						}
						if dist == 0 {
							dist = jsonFloat(nested, "distance")
						}
					}
				}
				segments = append(segments, segInfo{id: id.String, name: name, distance: dist / 1000})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading rows: %w", err)
			}

			if len(segments) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), []komGapRow{}, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			var result []komGapRow
			for _, seg := range segments {
				if cliutil.IsDogfoodEnv() && len(result) >= 2 {
					break
				}

				// Get athlete's best effort for this segment from local store.
				// The store writes resource_type='segment-efforts' (hyphen, matching
				// the API interface name), not 'segment_efforts' (underscore).
				effortQuery := `SELECT data FROM resources WHERE resource_type = 'segment-efforts' AND json_extract(data, '$.segment.id') = ? ORDER BY json_extract(data, '$.elapsed_time') ASC LIMIT 1`
				effortRow := db.DB().QueryRowContext(cmd.Context(), effortQuery, seg.id)
				var effortData sql.NullString
				myBestSec := 0
				if err := effortRow.Scan(&effortData); err == nil && effortData.Valid {
					var effort map[string]any
					if json.Unmarshal([]byte(effortData.String), &effort) == nil {
						myBestSec = int(jsonFloat(effort, "elapsed_time"))
					}
				}

				// Fetch live leaderboard top-1
				lbData, err := c.Get(cmd.Context(),
					"/segments/"+seg.id+"/leaderboard",
					map[string]string{"per_page": "1"})
				if err != nil {
					continue
				}
				var lb map[string]any
				if err := json.Unmarshal(lbData, &lb); err != nil {
					continue
				}
				entries, _ := lb["entries"].([]any)
				if len(entries) == 0 {
					continue
				}
				top, _ := entries[0].(map[string]any)
				komSec := int(jsonFloat(top, "elapsed_time"))
				if komSec == 0 || myBestSec == 0 {
					continue
				}

				gapSec := myBestSec - komSec
				if gapSec < 0 {
					gapSec = 0 // user holds or beats KOM
				}
				gapPct := math.Round(float64(gapSec)/float64(komSec)*1000) / 10

				result = append(result, komGapRow{
					SegmentID: seg.id,
					Name:      seg.name,
					Distance:  math.Round(seg.distance*10) / 10,
					MyBestSec: myBestSec,
					MyBest:    formatDuration(myBestSec),
					KOMSec:    komSec,
					KOM:       formatDuration(komSec),
					GapSec:    gapSec,
					GapPct:    gapPct,
				})

				// Throttle leaderboard calls — one per starred segment can exhaust
				// Strava's 200 req/15 min quota quickly without a delay.
				time.Sleep(500 * time.Millisecond)
			}

			// Sort by gap ascending (smallest gap first = most closeable)
			sort.Slice(result, func(i, j int) bool {
				return result[i].GapPct < result[j].GapPct
			})

			if topN > 0 && len(result) > topN {
				result = result[:topN]
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().IntVar(&topN, "top", 0, "Show only top N results by closest gap (0 = all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
