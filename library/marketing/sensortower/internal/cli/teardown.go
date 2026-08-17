// Copyright 2026 waveriderai and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Hand-authored body.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

// pp:data-source auto
//
// teardown spends exactly ONE live request on the iOS app hub (which carries
// the full versions[] history and the current category_rankings), then reads —
// never fetches — the local rank_snapshot mirror to align releases against rank
// moves. With no local history the timeline still ships; the alignment columns
// are simply absent and a note says why. Alignment is never fabricated.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/sensortower/internal/store"
	"github.com/spf13/cobra"
)

type teardownRelease struct {
	Date    string `json:"date"`
	Version string `json:"version"`
	// DaysSincePrevious is null for the oldest known release: there is no
	// earlier version to measure against.
	DaysSincePrevious *int `json:"days_since_previous"`
	// The rank_* fields appear only when the local snapshot mirror actually
	// covers both sides of the release window.
	RankBefore *int `json:"rank_before,omitempty"`
	RankAfter  *int `json:"rank_after,omitempty"`
	RankDelta  *int `json:"rank_delta,omitempty"`
}

type teardownCadence struct {
	MedianDaysBetween *float64 `json:"median_days_between"`
	Releases90d       int      `json:"releases_90d"`
	TotalVersions     int      `json:"total_versions"`
}

type teardownResult struct {
	AppID            json.RawMessage   `json:"app_id"`
	Name             string            `json:"name"`
	CurrentVersion   string            `json:"current_version"`
	Country          string            `json:"country"`
	Cadence          teardownCadence   `json:"cadence"`
	CategoryRankings json.RawMessage   `json:"category_rankings"`
	Releases         []teardownRelease `json:"releases"`
	Note             string            `json:"note,omitempty"`
}

// iosAppHub is the subset of the 51-key hub object this command reads.
type iosAppHub struct {
	AppID            json.RawMessage `json:"app_id"`
	Name             string          `json:"name"`
	CurrentVersion   string          `json:"current_version"`
	CategoryRankings json.RawMessage `json:"category_rankings"`
	Versions         []struct {
		Date  json.Number `json:"date"`
		Value string      `json:"value"`
	} `json:"versions"`
}

func newNovelTeardownCmd(flags *rootFlags) *cobra.Command {
	var flagCountry string
	var flagWindow int
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "teardown <app-id>",
		Short: "Align an app's release timeline against its rank history to see which shipped version moved the needle.",
		Long: "Reconstruct an iOS app's shipping cadence from its full version history and line each\n" +
			"release up against the rank change in the days that follow it.\n\n" +
			"Rank alignment is only reported where this CLI's own local rank_snapshot mirror covers\n" +
			"both sides of a release window. Snapshots are written by `movers`, so on a cold store\n" +
			"the release timeline and cadence stats ship without alignment and the note says so\n" +
			"rather than inventing rank movement.",
		Example:     "  sensortower-pp-cli teardown 460177396 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch the iOS app hub (1 request) and align its release timeline against local rank snapshots")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<app-id> is required (a numeric iOS App Store id such as 460177396)"))
			}
			appID := args[0]
			if flagWindow <= 0 {
				return usageErr(fmt.Errorf("--window must be a positive number of days"))
			}
			if flagLimit <= 0 {
				return usageErr(fmt.Errorf("--limit must be a positive number of releases"))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// The one and only request this command is allowed to make.
			data, err := c.Get(ctx, replacePathParam("/api/ios/apps/{app_id}", "app_id", appID), map[string]string{"country": flagCountry})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var hub iosAppHub
			if err := json.Unmarshal(data, &hub); err != nil {
				return apiErr(fmt.Errorf("decoding the iOS app hub response: %w", err))
			}

			// Version dates are epoch milliseconds and the API returns them
			// newest-first; sort explicitly rather than trusting the order.
			type release struct {
				at      time.Time
				version string
			}
			all := make([]release, 0, len(hub.Versions))
			for _, v := range hub.Versions {
				if at, ok := epochMSTime(v.Date); ok {
					all = append(all, release{at: at, version: v.Value})
				}
			}
			sort.Slice(all, func(i, j int) bool { return all[i].at.Before(all[j].at) })

			result := teardownResult{
				AppID:            hub.AppID,
				Name:             hub.Name,
				CurrentVersion:   hub.CurrentVersion,
				Country:          flagCountry,
				CategoryRankings: hub.CategoryRankings,
				Cadence:          teardownCadence{TotalVersions: len(all)},
				Releases:         []teardownRelease{},
			}
			if result.CategoryRankings == nil {
				result.CategoryRankings = json.RawMessage(`null`)
			}
			if len(all) == 0 {
				result.Note = "the API returned no version history for this app id, so there is no release timeline to report. Verify the id with 'sensortower-pp-cli apps ios --app-ids <id>'."
				return emitNovelResult(cmd, flags, result, "releases")
			}

			// Cadence over the whole history, not just the printed window.
			gaps := make([]float64, 0, len(all))
			for i := 1; i < len(all); i++ {
				gaps = append(gaps, all[i].at.Sub(all[i-1].at).Hours()/24)
			}
			result.Cadence.MedianDaysBetween = medianOf(gaps)
			cutoff := time.Now().UTC().AddDate(0, 0, -90)
			for _, r := range all {
				if r.at.After(cutoff) {
					result.Cadence.Releases90d++
				}
			}

			ranks, rankCategory, rankErr := readAppRankHistory(ctx, appID, flagCountry)
			if rankErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not read the local rank_snapshot history: %v\n", rankErr)
			}

			// Most recent --limit releases, newest first.
			start := len(all) - flagLimit
			if start < 0 {
				start = 0
			}
			aligned := 0
			for i := len(all) - 1; i >= start; i-- {
				rel := teardownRelease{
					Date:    all[i].at.Format("2006-01-02"),
					Version: all[i].version,
				}
				if i > 0 {
					rel.DaysSincePrevious = intPtr(int(all[i].at.Sub(all[i-1].at).Hours() / 24))
				}
				if before, after, ok := alignRankWindow(ranks, all[i].at, flagWindow); ok {
					rel.RankBefore = intPtr(before)
					rel.RankAfter = intPtr(after)
					rel.RankDelta = intPtr(before - after)
					aligned++
				}
				result.Releases = append(result.Releases, rel)
			}

			switch {
			case len(ranks) == 0:
				result.Note = "no local free-chart rank_snapshot history for this app, so no release is aligned against a rank move. Run 'sensortower-pp-cli movers <category>' (or 'watch digest' after it) on the app's category over several days to build the history this alignment needs. Only the free chart is aligned, so a history built solely with --chart grossing or --chart paid will not surface here."
			case aligned == 0:
				result.Note = fmt.Sprintf("local rank history exists (%d snapshots in category %s) but none of it brackets a release in the printed window, so no rank alignment is reported. Alignment needs a snapshot before a release and another within %d days after it.", len(ranks), rankCategory, flagWindow)
			default:
				result.Note = fmt.Sprintf("%d of %d printed releases are aligned against local free-chart snapshot history in category %s; rank_delta is rank_before - rank_after, so a positive value means the app climbed. Ranks from other categories are never mixed in, because the two charts are separate ladders. Correlation, not causation.", aligned, len(result.Releases), rankCategory)
			}

			return emitNovelResult(cmd, flags, result, "releases")
		},
	}
	cmd.Flags().StringVar(&flagCountry, "country", "US", "Two-letter country code for the rank context (e.g. US, GB, JP)")
	cmd.Flags().IntVar(&flagWindow, "window", 7, "Days after each release to look for a rank move")
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "How many of the most recent releases to print")
	return cmd
}

// rankPoint is one dated rank observation from the local mirror.
type rankPoint struct {
	at   time.Time
	rank int
}

// readAppRankHistory loads the local free-chart snapshots for one app, oldest
// first, together with the category they belong to. A cold store yields an empty
// slice and no error: absent history is not a failure, it just means nothing can
// be aligned.
//
// The history is scoped to ONE category — the one the newest snapshot belongs
// to. An app can be snapshotted on several category charts at once (`movers
// 6016` and `movers 36` both write it), and a rank on one chart is not
// comparable to a rank on another. Mixing them would let alignRankWindow read
// "before" off one ladder and "after" off another and print the difference as a
// post-release rank move the app never made.
func readAppRankHistory(ctx context.Context, appIDKey, country string) ([]rankPoint, string, error) {
	dbPath := defaultDBPath("sensortower-pp-cli")
	if !novelFileExists(dbPath) {
		return nil, "", nil
	}
	db, err := store.OpenReadOnlyContext(ctx, dbPath)
	if err != nil {
		return nil, "", err
	}
	defer db.Close()

	// captured_at breaks the tie when two categories share the newest date, so
	// the chosen category is deterministic rather than SQLite's row order.
	rows, err := db.DB().QueryContext(ctx,
		`SELECT json_extract(data, '$.date'),
		        json_extract(data, '$.rank'),
		        json_extract(data, '$.category')
		   FROM resources
		  WHERE resource_type = ?
		    AND json_extract(data, '$.app_id_key') = ?
		    AND json_extract(data, '$.country') = ?
		    AND json_extract(data, '$.chart') = 'free'
		    AND json_extract(data, '$.category') = (
		          SELECT json_extract(latest.data, '$.category')
		            FROM resources AS latest
		           WHERE latest.resource_type = ?
		             AND json_extract(latest.data, '$.app_id_key') = ?
		             AND json_extract(latest.data, '$.country') = ?
		             AND json_extract(latest.data, '$.chart') = 'free'
		           ORDER BY json_extract(latest.data, '$.date') DESC,
		                    json_extract(latest.data, '$.captured_at') DESC
		           LIMIT 1
		        )`,
		rankSnapshotResource, appIDKey, country,
		rankSnapshotResource, appIDKey, country,
	)
	if err != nil {
		if syncHintMissingTable(err) {
			return nil, "", nil
		}
		return nil, "", err
	}
	// Drain fully before touching the connection again.
	var points []rankPoint
	var category string
	for rows.Next() {
		var date sql.NullString
		var rank sql.NullInt64
		var cat sql.NullString
		if err := rows.Scan(&date, &rank, &cat); err != nil {
			_ = rows.Close()
			return nil, "", err
		}
		if !date.Valid || !rank.Valid {
			continue
		}
		at, parseErr := time.Parse("2006-01-02", date.String)
		if parseErr != nil {
			continue
		}
		if cat.Valid && category == "" {
			category = cat.String
		}
		points = append(points, rankPoint{at: at.UTC(), rank: int(rank.Int64)})
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, "", err
	}
	_ = rows.Close()

	sort.Slice(points, func(i, j int) bool { return points[i].at.Before(points[j].at) })
	return points, category, nil
}

// alignRankWindow finds the last rank observed at or before a release and the
// first one within window days after it. Reports ok=false unless both exist:
// a half-covered window supports no delta.
func alignRankWindow(points []rankPoint, releasedAt time.Time, window int) (before int, after int, ok bool) {
	day := releasedAt.Truncate(24 * time.Hour)
	deadline := day.AddDate(0, 0, window)
	foundBefore, foundAfter := false, false
	for _, p := range points {
		if !p.at.After(day) {
			before, foundBefore = p.rank, true // points are sorted, so this keeps the latest
			continue
		}
		if !foundAfter && !p.at.After(deadline) {
			after, foundAfter = p.rank, true
		}
	}
	return before, after, foundBefore && foundAfter
}

// medianOf returns the median, or nil for an empty sample rather than 0 — an
// unknown cadence must not read as "ships every zero days".
func medianOf(values []float64) *float64 {
	if len(values) == 0 {
		return nil
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	var m float64
	if len(sorted)%2 == 1 {
		m = sorted[mid]
	} else {
		m = (sorted[mid-1] + sorted[mid]) / 2
	}
	m = float64(int64(m*10+0.5)) / 10
	return &m
}
