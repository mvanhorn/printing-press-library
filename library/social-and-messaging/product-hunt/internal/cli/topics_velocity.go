// Copyright 2026 actionsslave. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/product-hunt/internal/store"
	"github.com/spf13/cobra"
)

type weekBucket struct {
	Week      string  `json:"week"`       // ISO week: "2026-W18"
	PostCount int     `json:"postCount"`
	TotalVotes int    `json:"totalVotes"`
	AvgVotes  float64 `json:"avgVotes"`
}

type velocityResult struct {
	Topic       string       `json:"topic"`
	Weeks       []weekBucket `json:"weeks"`
	PostDelta   int          `json:"postDelta"`
	VoteDelta   float64      `json:"avgVoteDelta"`
	Trend       string       `json:"trend"` // "up", "down", "stable"
}

func newTopicsVelocityCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var weeks int

	cmd := &cobra.Command{
		Use:   "velocity <topic-slug>",
		Short: "Week-over-week post count and vote delta for a topic",
		Long: `Queries the local store for posts in a topic, groups them by week,
and reports how post count and average votes have changed. Answers:
"Is this category heating up or cooling down?"

Reads from local store (run 'sync' first).`,
		Example: strings.Trim(`
  product-hunt-pp-cli topics velocity ai
  product-hunt-pp-cli topics velocity developer-tools --weeks 8 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			if dbPath == "" {
				dbPath = defaultDBPath("product-hunt-pp-cli")
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			topic := strings.ToLower(args[0])

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT data FROM resources WHERE resource_type = 'posts'`)
			if err != nil {
				return fmt.Errorf("querying posts: %w", err)
			}
			defer rows.Close()

			cutoff := time.Now().UTC().AddDate(0, 0, -weeks*7)

			// week -> (count, totalVotes)
			type weekData struct {
				count      int
				totalVotes int
			}
			weekMap := make(map[string]*weekData)
			weekOrder := []string{}

			for rows.Next() {
				var raw string
				if err := rows.Scan(&raw); err != nil {
					continue
				}
				var post struct {
					CreatedAt string `json:"createdAt"`
					VotesCount int   `json:"votesCount"`
					Topics    struct {
						Edges []struct {
							Node struct {
								Slug string `json:"slug"`
							} `json:"node"`
						} `json:"edges"`
					} `json:"topics"`
				}
				if err := json.Unmarshal([]byte(raw), &post); err != nil {
					continue
				}

				// Check topic match
				inTopic := false
				for _, e := range post.Topics.Edges {
					if strings.ToLower(e.Node.Slug) == topic {
						inTopic = true
						break
					}
				}
				if !inTopic {
					continue
				}

				ts, err := time.Parse(time.RFC3339, post.CreatedAt)
				if err != nil {
					continue
				}
				if ts.Before(cutoff) {
					continue
				}

				_, isoWeek := ts.ISOWeek()
				weekKey := fmt.Sprintf("%d-W%02d", ts.Year(), isoWeek)
				if _, ok := weekMap[weekKey]; !ok {
					weekMap[weekKey] = &weekData{}
					weekOrder = append(weekOrder, weekKey)
				}
				weekMap[weekKey].count++
				weekMap[weekKey].totalVotes += post.VotesCount
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading posts: %w", err)
			}

			sort.Strings(weekOrder)

			buckets := make([]weekBucket, 0, len(weekOrder))
			for _, wk := range weekOrder {
				d := weekMap[wk]
				avg := 0.0
				if d.count > 0 {
					avg = float64(d.totalVotes) / float64(d.count)
				}
				buckets = append(buckets, weekBucket{
					Week:       wk,
					PostCount:  d.count,
					TotalVotes: d.totalVotes,
					AvgVotes:   avg,
				})
			}

			result := velocityResult{
				Topic: args[0],
				Weeks: buckets,
			}

			// Compute delta: last week vs second-to-last week
			if len(buckets) >= 2 {
				last := buckets[len(buckets)-1]
				prev := buckets[len(buckets)-2]
				result.PostDelta = last.PostCount - prev.PostCount
				result.VoteDelta = last.AvgVotes - prev.AvgVotes
				switch {
				case result.PostDelta > 0:
					result.Trend = "up"
				case result.PostDelta < 0:
					result.Trend = "down"
				default:
					result.Trend = "stable"
				}
			}

			if len(buckets) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No posts found for topic %q in the last %d weeks.\n(Run 'sync' first if the store is empty.)\n", args[0], weeks)
				return nil
			}

			data, err := json.Marshal(result)
			if err != nil {
				return err
			}

			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&weeks, "weeks", 8, "Number of weeks to analyze")
	return cmd
}
