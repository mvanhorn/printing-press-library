package cli

import (
	"database/sql"
	"fmt"
	"io"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/drudgereport/internal/drudge"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/drudgereport/internal/store"
	"github.com/spf13/cobra"
)

// newDigestCmd returns the local Drudge history digest command.
func newDigestCmd(flags *rootFlags) *cobra.Command {
	var week bool
	var day bool
	cmd := &cobra.Command{
		Use:         "digest",
		Short:       "One-pager: splash count, longest-tenured splash, top domains, biggest red surges.",
		Example:     "  drudgereport-pp-cli digest --week --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if week && day {
				return usageErr(fmt.Errorf("--week and --day are mutually exclusive"))
			}

			ctx := cmd.Context()
			s, err := store.OpenWithContext(ctx, defaultDBPath("drudgereport-pp-cli"))
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer s.Close()
			if err := store.EnsureDrudgeSchema(ctx, s.DB()); err != nil {
				return fmt.Errorf("ensure drudge schema: %w", err)
			}

			window := 7 * 24 * time.Hour
			windowName := "week"
			if day {
				window = 24 * time.Hour
				windowName = "day"
			}
			result, err := queryDigest(cmd, s.DB(), windowName, window)
			if err != nil {
				return err
			}
			if _, ok := result["error"]; ok {
				return emitDrudgeLocal(cmd.OutOrStdout(), flags, result, func(w io.Writer) error {
					fmt.Fprintln(w, result["message"])
					return nil
				})
			}
			return emitDrudgeLocal(cmd.OutOrStdout(), flags, result, func(w io.Writer) error {
				fmt.Fprintf(w, "window: %v  splash count: %v\n\n", result["window"], result["splash_count"])
				if longest, ok := result["longest_tenured_splash"].(map[string]any); ok && longest != nil {
					fmt.Fprintf(w, "longest splash: %v (%vs)\n%v\n\n", longest["title"], longest["splash_tenure_seconds"], longest["url"])
				}
				fmt.Fprintln(w, "top domains")
				topDomains, _ := result["top_domains"].([]map[string]any)
				for _, row := range topDomains {
					fmt.Fprintf(w, "  %v  %v\n", row["count"], row["outbound_domain"])
				}
				fmt.Fprintln(w, "\nred surges")
				surges, _ := result["biggest_red_surges"].([]map[string]any)
				for _, row := range surges {
					fmt.Fprintf(w, "  %v red snapshots  %v\n", row["red_snapshot_count"], row["title"])
				}
				return nil
			})
		},
	}
	cmd.Flags().BoolVar(&week, "week", false, "Use a 7 day window")
	cmd.Flags().BoolVar(&day, "day", false, "Use a 1 day window")
	return cmd
}

func queryDigest(cmd *cobra.Command, db *sql.DB, windowName string, window time.Duration) (map[string]any, error) {
	var total sql.NullInt64
	if err := db.QueryRowContext(cmd.Context(), `SELECT COUNT(*) FROM drudge_story`).Scan(&total); err != nil {
		return nil, fmt.Errorf("query local data count: %w", err)
	}
	if !total.Valid || total.Int64 == 0 {
		return map[string]any{
			"error":   "no_data",
			"message": "Run drudgereport-pp-cli sync or splash to populate snapshot history.",
		}, nil
	}

	end := time.Now().UTC()
	start := end.Add(-window)
	startRaw := start.Format(time.RFC3339Nano)
	endRaw := end.Format(time.RFC3339Nano)

	var splashCount sql.NullInt64
	if err := db.QueryRowContext(cmd.Context(),
		`SELECT COUNT(DISTINCT story_id) FROM drudge_story WHERE slot = ? AND captured_at >= ? AND captured_at < ?`,
		string(drudge.SlotSplash), startRaw, endRaw,
	).Scan(&splashCount); err != nil {
		return nil, fmt.Errorf("query splash count: %w", err)
	}

	longest, err := queryDigestLongestSplash(cmd, db, startRaw, endRaw)
	if err != nil {
		return nil, err
	}
	topDomains, err := queryDigestTopDomains(cmd, db, startRaw, endRaw)
	if err != nil {
		return nil, err
	}
	redSurges, err := queryDigestRedSurges(cmd, db, startRaw, endRaw)
	if err != nil {
		return nil, err
	}
	count := int64(0)
	if splashCount.Valid {
		count = splashCount.Int64
	}
	return map[string]any{
		"window":                  windowName,
		"splash_count":            count,
		"longest_tenured_splash":  longest,
		"top_domains":             topDomains,
		"biggest_red_surges":      redSurges,
		"window_start":            start.Format(time.RFC3339),
		"window_end":              end.Format(time.RFC3339),
		"window_duration_seconds": int64(window.Seconds()),
	}, nil
}

func queryDigestLongestSplash(cmd *cobra.Command, db *sql.DB, startRaw, endRaw string) (map[string]any, error) {
	var storyID, title, url string
	var firstSeen, lastSeen sql.NullString
	// PATCH(greptile-2026-05-21:digest-longest-splash-contiguous): the digest's
	// "longest-tenured splash" headline must reflect the longest CONTIGUOUS
	// splash run in the window, not the gross MIN/MAX of splash appearances.
	// A story that was splash Jan 1-5, demoted Jan 6, re-promoted Jan 8 has
	// a real splash run of ~2 days, not 10+. Reuse the gap-and-island pattern
	// from tenure.go: tag each splash row with the most recent non-splash
	// captured_at (or epoch), GROUP BY (story_id, marker) to get per-run
	// MIN/MAX, then rank by run length.
	err := db.QueryRowContext(cmd.Context(),
		`WITH splash_rows AS (
			SELECT story_id, captured_at
			FROM drudge_story
			WHERE slot = ?
			  AND captured_at >= ?
			  AND captured_at < ?
		),
		run_marked AS (
			SELECT
				sr.story_id,
				sr.captured_at,
				COALESCE((
					SELECT MAX(captured_at) FROM drudge_story
					WHERE story_id = sr.story_id
					  AND slot != ?
					  AND captured_at < sr.captured_at
				), '1970-01-01T00:00:00Z') AS run_marker
			FROM splash_rows sr
		),
		runs AS (
			SELECT
				story_id,
				MIN(captured_at) AS first_seen_at,
				MAX(captured_at) AS last_seen_at,
				(strftime('%s', MAX(captured_at)) - strftime('%s', MIN(captured_at))) AS run_length_s
			FROM run_marked
			GROUP BY story_id, run_marker
		)
		SELECT r.story_id,
		       COALESCE(s.title, '') AS title,
		       COALESCE(s.url, '') AS url,
		       r.first_seen_at,
		       r.last_seen_at
		FROM runs r
		LEFT JOIN drudge_story s
			ON s.story_id = r.story_id
		   AND s.captured_at = (
				SELECT MAX(captured_at) FROM drudge_story
				WHERE story_id = r.story_id
		   )
		ORDER BY r.run_length_s DESC, r.first_seen_at ASC
		LIMIT 1`,
		string(drudge.SlotSplash), startRaw, endRaw, string(drudge.SlotSplash),
	).Scan(&storyID, &title, &url, &firstSeen, &lastSeen)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query longest splash: %w", err)
	}
	tenureSeconds := int64(0)
	if firstSeen.Valid && lastSeen.Valid {
		var err error
		tenureSeconds, err = secondsBetween(firstSeen.String, lastSeen.String)
		if err != nil {
			return nil, err
		}
	}
	return map[string]any{
		"story_id":              storyID,
		"title":                 title,
		"url":                   url,
		"first_seen_at":         nullStringText(firstSeen),
		"last_seen_at":          nullStringText(lastSeen),
		"splash_tenure_seconds": tenureSeconds,
	}, nil
}

func queryDigestTopDomains(cmd *cobra.Command, db *sql.DB, startRaw, endRaw string) ([]map[string]any, error) {
	rows, err := db.QueryContext(cmd.Context(),
		`SELECT outbound_domain, COUNT(*)
		 FROM drudge_story
		 WHERE captured_at >= ? AND captured_at < ?
		   AND outbound_domain != ''
		 GROUP BY outbound_domain
		 ORDER BY COUNT(*) DESC
		 LIMIT 5`,
		startRaw, endRaw,
	)
	if err != nil {
		return nil, fmt.Errorf("query top domains: %w", err)
	}
	defer rows.Close()

	results := make([]map[string]any, 0)
	for rows.Next() {
		var domain string
		var value int64
		if err := rows.Scan(&domain, &value); err != nil {
			return nil, fmt.Errorf("scan top domain: %w", err)
		}
		results = append(results, map[string]any{"outbound_domain": domain, "count": value})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate top domains: %w", err)
	}
	return results, nil
}

func queryDigestRedSurges(cmd *cobra.Command, db *sql.DB, startRaw, endRaw string) ([]map[string]any, error) {
	rows, err := db.QueryContext(cmd.Context(),
		`WITH red_runs AS (
			SELECT story_id, MIN(captured_at) AS first_red_at, MAX(captured_at) AS last_red_at, COUNT(*) AS red_snapshot_count
			FROM drudge_story
			WHERE is_red = 1 AND captured_at >= ? AND captured_at < ?
			GROUP BY story_id
		)
		SELECT r.story_id, s.title, s.url, s.outbound_domain, r.first_red_at, r.last_red_at, r.red_snapshot_count
		FROM red_runs r
		JOIN drudge_story s ON s.rowid = (
			SELECT rowid FROM drudge_story
			WHERE story_id = r.story_id
			ORDER BY captured_at DESC
			LIMIT 1
		)
		ORDER BY r.red_snapshot_count DESC, (strftime('%s', r.last_red_at) - strftime('%s', r.first_red_at)) DESC
		LIMIT 5`,
		startRaw, endRaw,
	)
	if err != nil {
		return nil, fmt.Errorf("query red surges: %w", err)
	}
	defer rows.Close()

	results := make([]map[string]any, 0)
	for rows.Next() {
		var storyID, title, url, domain string
		var firstRed, lastRed sql.NullString
		var redSnapshots sql.NullInt64
		if err := rows.Scan(&storyID, &title, &url, &domain, &firstRed, &lastRed, &redSnapshots); err != nil {
			return nil, fmt.Errorf("scan red surge: %w", err)
		}
		redTenureSeconds := int64(0)
		if firstRed.Valid && lastRed.Valid {
			var err error
			redTenureSeconds, err = secondsBetween(firstRed.String, lastRed.String)
			if err != nil {
				return nil, err
			}
		}
		// PATCH(greptile-2026-05-21:digest-total-tenure): total_tenure_seconds
		// must be the full lifetime of the story on Drudge (first to last
		// captured_at, regardless of slot), not the red-window duration.
		// Previously this duplicated redTenureSeconds, making the two
		// fields meaningless to differentiate.
		var totalFirstSeen, totalLastSeen sql.NullString
		if err := db.QueryRowContext(cmd.Context(),
			`SELECT MIN(captured_at), MAX(captured_at) FROM drudge_story WHERE story_id = ?`,
			storyID,
		).Scan(&totalFirstSeen, &totalLastSeen); err != nil {
			return nil, fmt.Errorf("query story total tenure for %s: %w", storyID, err)
		}
		totalTenureSeconds := int64(0)
		if totalFirstSeen.Valid && totalLastSeen.Valid {
			var err error
			totalTenureSeconds, err = secondsBetween(totalFirstSeen.String, totalLastSeen.String)
			if err != nil {
				return nil, err
			}
		}
		count := int64(0)
		if redSnapshots.Valid {
			count = redSnapshots.Int64
		}
		results = append(results, map[string]any{
			"story_id":             storyID,
			"title":                title,
			"url":                  url,
			"outbound_domain":      domain,
			"first_red_at":         nullStringText(firstRed),
			"last_red_at":          nullStringText(lastRed),
			"red_snapshot_count":   count,
			"red_tenure_seconds":   redTenureSeconds,
			"total_tenure_seconds": totalTenureSeconds,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate red surges: %w", err)
	}
	return results, nil
}
