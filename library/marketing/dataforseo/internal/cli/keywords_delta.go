// Copyright 2026 mazzsterr. Licensed under Apache-2.0. See LICENSE.
// Hand-written transcendence command. Not generated.

package cli

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/dataforseo/internal/store"
	"github.com/spf13/cobra"
)

type keywordDeltaRow struct {
	Keyword         string  `json:"keyword"`
	Source          string  `json:"source"`
	LocationCode    string  `json:"location_code,omitempty"`
	LocationName    string  `json:"location_name,omitempty"`
	LanguageCode    string  `json:"language_code,omitempty"`
	LanguageName    string  `json:"language_name,omitempty"`
	SearchPartners  string  `json:"search_partners,omitempty"`
	CurrentVolume   float64 `json:"current_volume"`
	PreviousVolume  float64 `json:"previous_volume"`
	Delta           float64 `json:"delta"`
	CurrentSyncedAt string  `json:"current_synced_at"`
}

func newKeywordsDeltaCmd(flags *rootFlags) *cobra.Command {
	var sinceStr string
	var single string

	cmd := &cobra.Command{
		Use:   "delta",
		Short: "Diff current vs last-known search volume per keyword from the local store",
		Long: `Joins the latest stored search volume against the previous stored value per keyword,
emitting movers sorted by absolute delta. Reads from the local SQLite store
populated by 'sync' or by previous calls to keywords-data endpoints.

Empty when the store has fewer than 2 snapshots per keyword (sync first).`,
		Example: strings.Trim(`
  dataforseo-pp-cli keywords delta --since 7d
  dataforseo-pp-cli keywords delta --keyword "tree service daytona" --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			since, err := parseDeltaSince(sinceStr)
			if err != nil {
				return usageErr(err)
			}

			ctx := context.Background()
			s, err := store.OpenWithContext(ctx, defaultDBPath("dataforseo-pp-cli"))
			if err != nil {
				return err
			}
			defer s.Close()

			rows, err := computeKeywordDeltas(s, since, single)
			if err != nil {
				return err
			}

			if len(rows) == 0 {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), []keywordDeltaRow{}, flags)
				}
				fmt.Fprintln(cmd.ErrOrStderr(), "No prior data — sync first with: dataforseo-pp-cli sync")
				return nil
			}

			sort.Slice(rows, func(i, j int) bool {
				return math.Abs(rows[i].Delta) > math.Abs(rows[j].Delta)
			})

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			headers := []string{"KEYWORD", "SOURCE", "LOCATION", "LANGUAGE", "PARTNERS", "CURRENT", "PREVIOUS", "DELTA", "SYNCED_AT"}
			out := make([][]string, 0, len(rows))
			for _, r := range rows {
				out = append(out, []string{
					r.Keyword,
					r.Source,
					firstNonEmpty(r.LocationCode, r.LocationName),
					firstNonEmpty(r.LanguageCode, r.LanguageName),
					r.SearchPartners,
					strconv.FormatFloat(r.CurrentVolume, 'f', -1, 64),
					strconv.FormatFloat(r.PreviousVolume, 'f', -1, 64),
					strconv.FormatFloat(r.Delta, 'f', -1, 64),
					r.CurrentSyncedAt,
				})
			}
			return flags.printTable(cmd, headers, out)
		},
	}
	cmd.Flags().StringVar(&sinceStr, "since", "7d", "Only include keywords with a current snapshot newer than this (e.g. 24h, 7d)")
	cmd.Flags().StringVar(&single, "keyword", "", "Restrict to a single keyword")
	return cmd
}

func parseDeltaSince(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 7 * 24 * time.Hour, nil
	}
	// Allow "7d" / "30d" shorthand which time.ParseDuration rejects.
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil || n < 0 {
			return 0, fmt.Errorf("invalid --since %q", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

// computeKeywordDeltas scans observations whose JSON payload looks like a
// Google Ads search volume row (carries a "keyword" + "search_volume" field)
// and pairs the latest two snapshots per keyword. The ranking and filters run
// in SQLite so large stores do not have to materialize full histories in Go.
func computeKeywordDeltas(s *store.Store, since time.Duration, only string) ([]keywordDeltaRow, error) {
	db := s.DB()
	cutoff := time.Now().Add(-since)
	// PATCH: Rank observations in SQLite and materialize only the two snapshots needed per keyword.
	q := `WITH ranked AS (
	        SELECT trim(CAST(json_extract(data, '$.keyword') AS TEXT)) AS kw,
	               CAST(json_extract(data, '$.search_volume') AS REAL) AS sv,
	               source,
	               COALESCE(CAST(json_extract(data, '$.location_code') AS TEXT), '') AS location_code,
	               COALESCE(CAST(json_extract(data, '$.location_name') AS TEXT), '') AS location_name,
	               COALESCE(CAST(json_extract(data, '$.language_code') AS TEXT), '') AS language_code,
	               COALESCE(CAST(json_extract(data, '$.language_name') AS TEXT), '') AS language_name,
	               COALESCE(CAST(json_extract(data, '$.search_partners') AS TEXT), '') AS search_partners,
	               observed_at,
	               row_number() OVER (
	                 PARTITION BY source,
	                              lower(trim(CAST(json_extract(data, '$.keyword') AS TEXT))),
	                              COALESCE(CAST(json_extract(data, '$.location_code') AS TEXT), ''),
	                              COALESCE(CAST(json_extract(data, '$.location_name') AS TEXT), ''),
	                              COALESCE(CAST(json_extract(data, '$.language_code') AS TEXT), ''),
	                              COALESCE(CAST(json_extract(data, '$.language_name') AS TEXT), ''),
	                              COALESCE(CAST(json_extract(data, '$.search_partners') AS TEXT), '')
	                 ORDER BY sequence DESC
	               ) AS snapshot_rank
	        FROM resource_history
	        WHERE resource_type = 'keywords-data'
	          AND source LIKE '/v3/keywords_data/google_ads/search_volume/%'
	          AND json_extract(data, '$.keyword') IS NOT NULL
	          AND json_extract(data, '$.search_volume') IS NOT NULL
	          AND (? = '' OR lower(trim(CAST(json_extract(data, '$.keyword') AS TEXT))) = lower(?))
	      )
	      SELECT kw, sv, source, location_code, location_name, language_code, language_name, search_partners, observed_at, snapshot_rank
	      FROM ranked AS snapshot
	      WHERE snapshot_rank <= 2
	      ORDER BY lower(kw), source, location_code, location_name, language_code, language_name, search_partners, snapshot_rank`
	rows, err := db.Query(q, only, only)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type snap struct {
		keyword        string
		source         string
		locationCode   string
		locationName   string
		languageCode   string
		languageName   string
		searchPartners string
		volume         float64
		syncedAt       string
	}
	type seriesKey struct {
		keyword, source, locationCode, locationName, languageCode, languageName, searchPartners string
	}
	bySeries := map[seriesKey][]snap{}
	for rows.Next() {
		var kw, source, locationCode, locationName, languageCode, languageName, searchPartners, syncedAt string
		var volume float64
		var snapshotRank int
		if err := rows.Scan(&kw, &volume, &source, &locationCode, &locationName, &languageCode, &languageName, &searchPartners, &syncedAt, &snapshotRank); err != nil {
			return nil, err
		}
		if kw == "" {
			continue
		}
		key := seriesKey{
			keyword: strings.ToLower(kw), source: source,
			locationCode: locationCode, locationName: locationName,
			languageCode: languageCode, languageName: languageName,
			searchPartners: searchPartners,
		}
		bySeries[key] = append(bySeries[key], snap{
			keyword: kw, source: source, locationCode: locationCode, locationName: locationName,
			languageCode: languageCode, languageName: languageName, searchPartners: searchPartners,
			volume: volume, syncedAt: syncedAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]keywordDeltaRow, 0, len(bySeries))
	for _, snaps := range bySeries {
		if len(snaps) < 2 {
			continue
		}
		cur := snaps[0]
		prev := snaps[1]
		if since > 0 {
			observedAt, err := time.Parse(time.RFC3339Nano, cur.syncedAt)
			if err != nil || observedAt.Before(cutoff) {
				continue
			}
		}
		out = append(out, keywordDeltaRow{
			Keyword:         cur.keyword,
			Source:          cur.source,
			LocationCode:    cur.locationCode,
			LocationName:    cur.locationName,
			LanguageCode:    cur.languageCode,
			LanguageName:    cur.languageName,
			SearchPartners:  cur.searchPartners,
			CurrentVolume:   cur.volume,
			PreviousVolume:  prev.volume,
			Delta:           cur.volume - prev.volume,
			CurrentSyncedAt: cur.syncedAt,
		})
	}
	return out, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
