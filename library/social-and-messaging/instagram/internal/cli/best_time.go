// Copyright 2026 Mohammed Al Khamis and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

type bestTimeSlot struct {
	Weekday         string  `json:"weekday"`
	Hour            int     `json:"hour"`
	Posts           int     `json:"posts"`
	AvgInteractions float64 `json:"avg_interactions"`
}

var weekdayNames = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

// igTimeLayouts covers the Graph API's +0000 (no-colon) timestamp and the
// RFC3339 colon form the store might also hold.
var igTimeLayouts = []string{"2006-01-02T15:04:05-0700", time.RFC3339, "2006-01-02T15:04:05Z07:00"}

func parseIGTime(s string) (time.Time, bool) {
	for _, l := range igTimeLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func newNovelBestTimeCmd(flags *rootFlags) *cobra.Command {
	var dbFlag, accountFlag string

	cmd := &cobra.Command{
		Use:   "best-time",
		Short: "Surface the weekday/hour slots where your posts historically earn the most engagement.",
		Long: `Bucket stored media by the weekday and hour they were posted, then rank slots
by average total_interactions. Also reports the average gap (in days) between
posts so you can spot under- or over-posting.

Reads the local store. Run 'instagram-pp-cli pull' first to populate media.`,
		Example:     "  instagram-pp-cli best-time --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute best posting times from local store")
				return nil
			}
			db, err := ensureAnalyticsSchema(cmd.Context(), resolveDBPath(dbFlag))
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()

			q := `SELECT COALESCE(posted_at,''), COALESCE(total_interactions,0) FROM ig_brand_media WHERE posted_at IS NOT NULL AND posted_at != ''`
			args2 := []any{}
			if accountFlag != "" {
				q += ` AND slug = ?`
				args2 = append(args2, slugify(accountFlag))
			}
			rows, err := db.DB().QueryContext(cmd.Context(), q, args2...)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()

			type agg struct {
				posts int
				sum   int64
			}
			buckets := map[[2]int]*agg{}
			var times []time.Time
			for rows.Next() {
				var postedAt string
				var interactions int64
				if err := rows.Scan(&postedAt, &interactions); err != nil {
					return apiErr(err)
				}
				t, ok := parseIGTime(postedAt)
				if !ok {
					continue
				}
				times = append(times, t)
				key := [2]int{int(t.Weekday()), t.Hour()}
				a := buckets[key]
				if a == nil {
					a = &agg{}
					buckets[key] = a
				}
				a.posts++
				a.sum += interactions
			}
			if err := rows.Err(); err != nil {
				return apiErr(err)
			}

			out := make([]bestTimeSlot, 0, len(buckets))
			for key, a := range buckets {
				avg := 0.0
				if a.posts > 0 {
					avg = float64(a.sum) / float64(a.posts)
				}
				out = append(out, bestTimeSlot{
					Weekday:         weekdayNames[key[0]],
					Hour:            key[1],
					Posts:           a.posts,
					AvgInteractions: avg,
				})
			}
			sort.SliceStable(out, func(i, j int) bool { return out[i].AvgInteractions > out[j].AvgInteractions })

			// Posting-gap note: avg days between consecutive posts.
			avgGapDays := 0.0
			if len(times) >= 2 {
				sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })
				span := times[len(times)-1].Sub(times[0]).Hours() / 24
				avgGapDays = span / float64(len(times)-1)
			}

			if flags.asJSON {
				env := map[string]any{"slots": out, "count": len(out), "avg_days_between_posts": avgGapDays}
				if len(out) == 0 {
					env["note"] = "no media in store; run 'instagram-pp-cli pull' first"
				}
				return printJSONFiltered(cmd.OutOrStdout(), env, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No media in store. Run 'instagram-pp-cli pull' first.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "WEEKDAY\tHOUR\tPOSTS\tAVG_INTERACTIONS")
			for _, s := range out {
				fmt.Fprintf(tw, "%s\t%02d:00\t%d\t%.1f\n", s.Weekday, s.Hour, s.Posts, s.AvgInteractions)
			}
			_ = tw.Flush()
			fmt.Fprintf(cmd.OutOrStdout(), "\nAverage gap between posts: %.1f days\n", avgGapDays)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbFlag, "db", "", "Path to the local store (defaults to the standard data dir)")
	cmd.Flags().StringVar(&accountFlag, "account", "", "Limit to a single brand slug")
	return cmd
}
