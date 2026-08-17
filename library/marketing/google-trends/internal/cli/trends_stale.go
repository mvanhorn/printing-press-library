// Copyright 2026 Kerry Morrison and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-trends/internal/cliutil"
)

type staleKeyword struct {
	Keyword      string `json:"keyword"`
	Geo          string `json:"geo"`
	Category     int    `json:"category"`
	Property     string `json:"property,omitempty"`
	CompareScope string `json:"compare_scope,omitempty"`
	LastSyncedAt string `json:"last_synced_at"`
	Age          string `json:"age"`
}

// pp:data-source local
func newNovelTrendsStaleCmd(flags *rootFlags) *cobra.Command {
	var flagOlderThan string

	cmd := &cobra.Command{
		Use:         "stale",
		Short:       "Lists tracked keywords whose local data hasn't been refreshed recently",
		Example:     "  google-trends-pp-cli trends stale --older-than 14d --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			olderThanStr := flagOlderThan
			if olderThanStr == "" {
				olderThanStr = "7d"
			}
			olderThanDur, err := cliutil.ParseDurationLoose(olderThanStr)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --older-than %q: %w", flagOlderThan, err))
			}

			ctx := cmd.Context()
			db, err := openStoreForRead(ctx, "google-trends-pp-cli")
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			if db == nil {
				return noLocalMirrorHint(cmd, flags, "google-trends-pp-cli trends interest <keyword>", make([]staleKeyword, 0))
			}
			defer db.Close()

			rows, err := db.List("gt_keyword_query", 0)
			if err != nil {
				return fmt.Errorf("querying tracked keywords: %w", err)
			}

			cutoff := time.Now().Add(-olderThanDur)
			out := make([]staleKeyword, 0)
			for _, raw := range rows {
				var kq gtKeywordQueryRecord
				if err := json.Unmarshal(raw, &kq); err != nil {
					continue
				}
				lastSynced, err := time.Parse(time.RFC3339, kq.LastSyncedAt)
				if err != nil {
					continue
				}
				if lastSynced.After(cutoff) {
					continue
				}
				out = append(out, staleKeyword{
					Keyword:      kq.Keyword,
					Geo:          kq.Geo,
					Category:     kq.Category,
					Property:     kq.Property,
					CompareScope: kq.CompareScope,
					LastSyncedAt: kq.LastSyncedAt,
					Age:          time.Since(lastSynced).Round(time.Second).String(),
				})
			}
			sort.Slice(out, func(i, j int) bool { return out[i].LastSyncedAt < out[j].LastSyncedAt })

			return printLocalResult(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&flagOlderThan, "older-than", "", "Consider keywords stale if not synced within this window (e.g. 7d, 24h); default 7d")
	return cmd
}
