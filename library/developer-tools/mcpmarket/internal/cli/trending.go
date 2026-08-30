// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source computed
// This command computes results from locally stored history (resource_snapshots)
// built up as the user browses; it does not read a single upstream resource type.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/mcpmarket/internal/cliutil"
)

// entityLikeCount pulls interactionStatistic.userInteractionCount out of a
// stored JSON-LD SoftwareApplication blob.
func entityLikeCount(data json.RawMessage) int {
	var entity struct {
		Name                 string `json:"name"`
		InteractionStatistic struct {
			UserInteractionCount int `json:"userInteractionCount"`
		} `json:"interactionStatistic"`
	}
	if err := json.Unmarshal(data, &entity); err != nil {
		return 0
	}
	return entity.InteractionStatistic.UserInteractionCount
}

func entityName(data json.RawMessage) string {
	var entity struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(data, &entity)
	return entity.Name
}

type trendingEntry struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Was   int    `json:"was"`
	Now   int    `json:"now"`
	Delta int    `json:"delta"`
}

func newNovelTrendingCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var flagType string
	var limit int

	cmd := &cobra.Command{
		Use:         "trending",
		Short:       "See which MCP servers/skills are rising fastest vs. holding steady, computed from your own sync history.",
		Example:     "  mcpmarket-pp-cli trending --since 7d --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "trending")
			}
			since := flagSince
			if since == "" {
				since = "7d"
			}
			dur, err := cliutil.ParseDurationLoose(since)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --since duration %q: %w", since, err))
			}
			resourceType := flagType
			if resourceType == "" {
				resourceType = "server"
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			entries, note, err := computeTrending(ctx, resourceType, dur, limit)
			if err != nil {
				return err
			}
			if note != "" {
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"trending": []trendingEntry{}, "note": note}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), note)
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), entries, flags)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "7d", "how far back to compare, e.g. 24h, 7d, 30d")
	cmd.Flags().StringVar(&flagType, "type", "server", "resource type to compare: server or skill")
	cmd.Flags().IntVar(&limit, "limit", 15, "maximum rising entries to return")
	return cmd
}

func computeTrending(ctx context.Context, resourceType string, since time.Duration, limit int) ([]trendingEntry, string, error) {
	dbPath := defaultDBPath("mcpmarket-pp-cli")
	db, err := storeOpenForNovel(ctx, dbPath)
	if err != nil {
		return nil, "", err
	}
	defer db.Close()

	today, _, err := db.CaptureSnapshot(ctx, resourceType)
	if err != nil {
		return nil, "", fmt.Errorf("capturing today's snapshot: %w", err)
	}

	target := time.Now().UTC().Add(-since).Format("2006-01-02")
	priorDate, ok, err := db.NearestSnapshotDateOnOrBefore(ctx, target)
	if err != nil {
		return nil, "", err
	}
	if !ok || priorDate == today {
		return nil, fmt.Sprintf("not enough history yet to compute trending — only today's snapshot exists. Run `mcpmarket-pp-cli %s list` (or search/leaderboard) again on a later day, then retry.", resourceType), nil
	}

	oldRows, err := db.SnapshotRows(ctx, priorDate, resourceType)
	if err != nil {
		return nil, "", err
	}
	newRows, err := db.SnapshotRows(ctx, today, resourceType)
	if err != nil {
		return nil, "", err
	}

	oldCounts := make(map[string]int, len(oldRows))
	for _, r := range oldRows {
		oldCounts[r.ResourceID] = entityLikeCount(r.Data)
	}

	entries := make([]trendingEntry, 0, len(newRows))
	for _, r := range newRows {
		now := entityLikeCount(r.Data)
		was, seen := oldCounts[r.ResourceID]
		if !seen {
			continue
		}
		entries = append(entries, trendingEntry{
			ID:    r.ResourceID,
			Name:  entityName(r.Data),
			Was:   was,
			Now:   now,
			Delta: now - was,
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Delta > entries[j].Delta })
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	return entries, "", nil
}
