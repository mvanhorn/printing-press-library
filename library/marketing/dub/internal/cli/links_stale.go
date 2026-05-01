// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

// links_stale queries the local /store cache populated by `sync` and classifies
// each link as stale based on click counts, archive status, and expiry. It does
// not call the live API; the data layer is the store.Open-backed SQLite
// database opened via openStoreForRead.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type staleLink struct {
	ID         string `json:"id"`
	Domain     string `json:"domain"`
	Key        string `json:"key"`
	ShortLink  string `json:"shortLink,omitempty"`
	URL        string `json:"url"`
	Clicks     int    `json:"clicks"`
	Archived   bool   `json:"archived"`
	ExpiresAt  string `json:"expiresAt,omitempty"`
	CreatedAt  string `json:"createdAt,omitempty"`
	LastSynced string `json:"lastSynced,omitempty"`
	Reason     string `json:"reason"`
}

func newLinksStaleCmd(flags *rootFlags) *cobra.Command {
	var days int
	var minClicks int
	var includeArchived bool

	cmd := &cobra.Command{
		Use:   "stale",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Find links nobody clicks — dead-link detection",
		Long: `Find links with zero or near-zero clicks. Combines several signals:
  - clicks below threshold over the last --days
  - archived links still receiving traffic (cleanup candidates)
  - expired links (expiresAt in the past)

Reads from the local store. Run sync first.`,
		Example: `  # Find links with zero clicks in the workspace
  dub-pp-cli links stale --json

  # Tighter threshold: less than 5 clicks
  dub-pp-cli links stale --min-clicks 5 --json --select id,domain,key,url,clicks

  # Just dead expired links
  dub-pp-cli links stale --days 30 --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStoreForRead("dub-pp-cli")
			if err != nil {
				return apiErr(fmt.Errorf("open local store: %w", err))
			}
			if db == nil {
				return notFoundErr(fmt.Errorf("no local store found. run `dub-pp-cli sync --full` first"))
			}
			defer db.Close()

			rows, err := db.DB().Query(`
				SELECT id, data, archived, expires_at, synced_at
				FROM links`)
			if err != nil {
				return apiErr(fmt.Errorf("query links: %w", err))
			}
			defer rows.Close()

			now := time.Now().UTC()
			cutoff := now.Add(-time.Duration(days) * 24 * time.Hour)

			stale := make([]staleLink, 0)
			for rows.Next() {
				var (
					id, raw, expiresAt, syncedAt string
					archived                     int
				)
				if err := rows.Scan(&id, &raw, &archived, &expiresAt, &syncedAt); err != nil {
					continue
				}
				var obj map[string]any
				if err := json.Unmarshal([]byte(raw), &obj); err != nil {
					continue
				}
				clicks := intField(obj, "clicks")
				createdAt := stringField(obj, "createdAt")
				domain := stringField(obj, "domain")
				key := stringField(obj, "key")
				shortLink := stringField(obj, "shortLink")
				url := stringField(obj, "url")

				reason, hit := classifyStale(obj, clicks, archived == 1, minClicks, expiresAt, createdAt, cutoff, includeArchived)
				if !hit {
					continue
				}

				stale = append(stale, staleLink{
					ID:         id,
					Domain:     domain,
					Key:        key,
					ShortLink:  shortLink,
					URL:        url,
					Clicks:     clicks,
					Archived:   archived == 1,
					ExpiresAt:  expiresAt,
					CreatedAt:  createdAt,
					LastSynced: syncedAt,
					Reason:     reason,
				})
			}
			sort.Slice(stale, func(i, j int) bool {
				return stale[i].Clicks < stale[j].Clicks
			})

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, stale)
			}

			if len(stale) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no stale links found")
				return nil
			}
			headers := []string{"ID", "DOMAIN", "KEY", "CLICKS", "REASON", "URL"}
			rowsTbl := make([][]string, 0, len(stale))
			for _, s := range stale {
				url := s.URL
				if len(url) > 50 {
					url = url[:47] + "..."
				}
				rowsTbl = append(rowsTbl, []string{s.ID, s.Domain, s.Key, fmt.Sprintf("%d", s.Clicks), s.Reason, url})
			}
			return flags.printTable(cmd, headers, rowsTbl)
		},
	}

	cmd.Flags().IntVar(&days, "days", 90, "Consider stale if created more than N days ago and clicks below threshold")
	cmd.Flags().IntVar(&minClicks, "min-clicks", 1, "Click threshold; links with strictly fewer than this are flagged")
	cmd.Flags().BoolVar(&includeArchived, "include-archived", true, "Include archived links in the stale set")
	return cmd
}

// classifyStale decides whether a link is stale and why.
// Pure logic, exported indirectly via the unit test in links_stale_test.go.
func classifyStale(obj map[string]any, clicks int, archived bool, minClicks int, expiresAt, createdAt string, cutoff time.Time, includeArchived bool) (string, bool) {
	if !includeArchived && archived {
		return "", false
	}
	reasons := make([]string, 0, 3)
	if archived && clicks > 0 {
		reasons = append(reasons, "archived-but-trafficked")
	}
	if expiresAt != "" {
		if t, err := time.Parse(time.RFC3339, expiresAt); err == nil && t.Before(time.Now().UTC()) {
			reasons = append(reasons, "expired")
		}
	}
	if clicks < minClicks {
		if createdAt != "" {
			if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
				if t.Before(cutoff) {
					reasons = append(reasons, fmt.Sprintf("low-clicks-since-%s", t.Format("2006-01-02")))
				}
			}
		} else {
			reasons = append(reasons, "low-clicks")
		}
	}
	if len(reasons) == 0 {
		return "", false
	}
	return strings.Join(reasons, ","), true
}

func stringField(obj map[string]any, key string) string {
	if v, ok := obj[key].(string); ok {
		return v
	}
	return ""
}

func intField(obj map[string]any, key string) int {
	switch v := obj[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return int(i)
		}
	}
	return 0
}

func floatField(obj map[string]any, key string) float64 {
	switch v := obj[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return f
		}
	}
	return 0
}
