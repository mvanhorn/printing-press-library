// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel-feature command (Phase 3).

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

// QuotaCosts: the official quota costs for the YouTube Data API v3 operations
// that matter most. Source: developers.google.com/youtube/v3/determine_quota_cost.
// Reads default to 1 unit; writes 50; uploads 1600; search 100.
var defaultQuotaCosts = map[string]int{
	"videos-list":            1,
	"videos-get":             1,
	"videos-insert":          1600,
	"videos-update":          50,
	"videos-delete":          50,
	"videos-rate":            50,
	"playlists-list":         1,
	"playlists-insert":       50,
	"playlists-update":       50,
	"playlists-delete":       50,
	"playlist-items-list":    1,
	"playlist-items-insert":  50,
	"playlist-items-update":  50,
	"playlist-items-delete":  50,
	"search-list":            100,
	"channels-list":          1,
	"channels-update":        50,
	"thumbnails-set":         50,
	"comments-list":          1,
	"comment-threads-list":   1,
	"comments-set-mod":       50,
	"comments-insert":        50,
	"comments-update":        50,
	"comments-delete":        50,
	"captions-list":          1,
	"captions-insert":        400,
	"captions-update":        450,
	"captions-delete":        50,
	"channel-banners-insert": 50,
	"watermarks-set":         50,
	"watermarks-unset":       50,
	"members-list":           2,
}

const dailyQuotaDefault = 10000

type quotaEntry struct {
	Timestamp time.Time `json:"ts"`
	Op        string    `json:"op"`
	Cost      int       `json:"cost"`
}

type quotaLog struct {
	Daily   int          `json:"daily_limit"`
	Entries []quotaEntry `json:"entries"`
}

func quotaPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "youtube-creator-pp-cli", "quota.json")
}

func quotaLoad() (*quotaLog, error) {
	path := quotaPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &quotaLog{Daily: dailyQuotaDefault}, nil
		}
		return nil, err
	}
	var q quotaLog
	if err := json.Unmarshal(data, &q); err != nil {
		return nil, err
	}
	if q.Daily == 0 {
		q.Daily = dailyQuotaDefault
	}
	return &q, nil
}

func quotaSave(q *quotaLog) error {
	path := quotaPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(q, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// quotaLogCost records a call cost. Best-effort; never returns an error to
// callers since logging failure must not break the API call.
func quotaLogCost(op string, cost int) {
	q, err := quotaLoad()
	if err != nil {
		return
	}
	q.Entries = append(q.Entries, quotaEntry{
		Timestamp: time.Now(),
		Op:        op,
		Cost:      cost,
	})
	_ = quotaSave(q)
}

// quotaSpentToday returns the sum of entries since midnight local time.
func quotaSpentToday(q *quotaLog) int {
	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	total := 0
	for _, e := range q.Entries {
		if e.Timestamp.After(midnight) {
			total += e.Cost
		}
	}
	return total
}

func newQuotaCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "quota",
		Short: "Quota tracking and cost estimation for YouTube Data API",
		Long: `Quota tracks every API call to a local log at ~/.config/youtube-creator-pp-cli/quota.json.

The default daily quota is 10,000 units; high-volume operations like videos-insert
cost 1600 units, search-list costs 100 units, and most reads cost 1 unit.

See: developers.google.com/youtube/v3/determine_quota_cost`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newQuotaMeterCmd(flags))
	cmd.AddCommand(newQuotaResetCmd(flags))
	cmd.AddCommand(newQuotaCostsCmd(flags))
	return cmd
}

func newQuotaMeterCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "meter",
		Short:   "Show today's quota spend and estimated remaining units",
		Example: "  youtube-creator-pp-cli quota meter --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			q, err := quotaLoad()
			if err != nil {
				return fmt.Errorf("loading quota log: %w", err)
			}
			spent := quotaSpentToday(q)
			remaining := q.Daily - spent
			if remaining < 0 {
				remaining = 0
			}

			// Per-op breakdown
			byOp := map[string]int{}
			midnight := time.Now().Truncate(24 * time.Hour)
			for _, e := range q.Entries {
				if e.Timestamp.After(midnight) {
					byOp[e.Op] += e.Cost
				}
			}
			type opStat struct {
				Op   string `json:"op"`
				Cost int    `json:"cost"`
			}
			var stats []opStat
			for op, c := range byOp {
				stats = append(stats, opStat{Op: op, Cost: c})
			}
			sort.Slice(stats, func(i, j int) bool { return stats[i].Cost > stats[j].Cost })

			result := map[string]any{
				"daily_limit":        q.Daily,
				"spent_today":        spent,
				"remaining_today":    remaining,
				"by_operation":       stats,
				"log_path":           quotaPath(),
				"total_calls_logged": len(q.Entries),
			}
			return flags.printJSON(cmd, result)
		},
	}
	return cmd
}

func newQuotaResetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "reset",
		Short:   "Clear the local quota log (use --yes to skip confirmation)",
		Example: "  youtube-creator-pp-cli quota reset --yes",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if !flags.yes {
				return usageErr(fmt.Errorf("destructive operation; rerun with --yes to confirm"))
			}
			q := &quotaLog{Daily: dailyQuotaDefault}
			if err := quotaSave(q); err != nil {
				return err
			}
			return flags.printJSON(cmd, map[string]any{"reset": true, "path": quotaPath()})
		},
	}
	return cmd
}

func newQuotaCostsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "costs",
		Short:   "Print the per-operation quota cost table",
		Example: "  youtube-creator-pp-cli quota costs --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			type row struct {
				Op   string `json:"op"`
				Cost int    `json:"cost"`
			}
			var rows []row
			for op, c := range defaultQuotaCosts {
				rows = append(rows, row{Op: op, Cost: c})
			}
			sort.Slice(rows, func(i, j int) bool {
				if rows[i].Cost != rows[j].Cost {
					return rows[i].Cost > rows[j].Cost
				}
				return rows[i].Op < rows[j].Op
			})
			return flags.printJSON(cmd, rows)
		},
	}
	return cmd
}
