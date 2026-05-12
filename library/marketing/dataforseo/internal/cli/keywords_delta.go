// Copyright 2026 mazzsterr. Licensed under Apache-2.0. See LICENSE.
// Hand-written transcendence command. Not generated.

package cli

import (
	"context"
	"github.com/mvanhorn/printing-press-library/library/marketing/dataforseo/internal/store"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type keywordDeltaRow struct {
	Keyword         string  `json:"keyword"`
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
			headers := []string{"KEYWORD", "CURRENT", "PREVIOUS", "DELTA", "SYNCED_AT"}
			out := make([][]string, 0, len(rows))
			for _, r := range rows {
				out = append(out, []string{
					r.Keyword,
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

// computeKeywordDeltas walks every resource whose JSON payload looks like a
// Google Ads search volume row (carries a "keyword" + "search_volume" field)
// and pairs the latest two snapshots per keyword. Implementation works against
// the generic resources table so it survives any resource-type naming the
// generator picks. If the store has fewer than 2 snapshots per keyword,
// returns an empty slice.
func computeKeywordDeltas(s *store.Store, since time.Duration, only string) ([]keywordDeltaRow, error) {
	db := s.DB()
	cutoff := time.Now().Add(-since).UTC().Format(time.RFC3339)
	q := `SELECT json_extract(data, '$.keyword') AS kw,
	             json_extract(data, '$.search_volume') AS sv,
	             updated_at
	      FROM resources
	      WHERE json_extract(data, '$.keyword') IS NOT NULL
	        AND json_extract(data, '$.search_volume') IS NOT NULL`
	rows, err := db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type snap struct {
		volume    float64
		syncedAt  string
		syncedRaw time.Time
	}
	byKeyword := map[string][]snap{}
	for rows.Next() {
		var kwRaw, svRaw, syncedAt string
		if err := rows.Scan(&kwRaw, &svRaw, &syncedAt); err != nil {
			return nil, err
		}
		var kw string
		_ = json.Unmarshal([]byte(`"`+kwRaw+`"`), &kw)
		kw = strings.TrimSpace(kwRaw)
		if kw == "" {
			continue
		}
		if only != "" && !strings.EqualFold(kw, only) {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(svRaw), 64)
		if err != nil {
			continue
		}
		t, _ := time.Parse(time.RFC3339, syncedAt)
		byKeyword[kw] = append(byKeyword[kw], snap{volume: v, syncedAt: syncedAt, syncedRaw: t})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]keywordDeltaRow, 0, len(byKeyword))
	for kw, snaps := range byKeyword {
		if len(snaps) < 2 {
			continue
		}
		sort.Slice(snaps, func(i, j int) bool {
			return snaps[i].syncedRaw.After(snaps[j].syncedRaw)
		})
		cur := snaps[0]
		prev := snaps[1]
		if since > 0 && cur.syncedAt < cutoff {
			continue
		}
		out = append(out, keywordDeltaRow{
			Keyword:         kw,
			CurrentVolume:   cur.volume,
			PreviousVolume:  prev.volume,
			Delta:           cur.volume - prev.volume,
			CurrentSyncedAt: cur.syncedAt,
		})
	}
	return out, nil
}
