// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/reddit/internal/store"
)

type postingBucket struct {
	Key          string  `json:"key"` // "Mon@14" or "Mon" depending on --by
	SampleCount  int     `json:"sample_count"`
	MedianScore  float64 `json:"median_score"`
	P75Score     float64 `json:"p75_score"`
	TopScore     int     `json:"top_score"`
	TopPermalink string  `json:"top_permalink"`
}

type postingStatsReport struct {
	Subreddit  string          `json:"subreddit,omitempty"`
	By         string          `json:"by"` // "hour" | "dow" | "dow_hour"
	Buckets    []postingBucket `json:"buckets"`
	TotalPosts int             `json:"total_posts"`
}

// newMePostingStatsCmd computes per-bucket score statistics from the user's
// synced submitted history. Bucketing keys: --by hour, --by dow, or --by
// dow_hour (default). Median + 75th-percentile + top score per bucket.
//
// No Reddit endpoint returns this; it's pure local aggregation.
func newMePostingStatsCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		sub    string
		by     string
		minN   int
	)
	cmd := &cobra.Command{
		Use:   "posting-stats",
		Short: "Compute best-time-to-post stats from your synced submission history",
		Long: `Aggregate your own submitted history by (subreddit × day-of-week × hour)
and report median + 75th-percentile + top score per bucket. The output answers
"when does my Reddit posting actually perform best on a given sub?"

Requires 'sync' to have populated user_submitted entries for your own account.`,
		Example: `  reddit-pp-cli me posting-stats
  reddit-pp-cli me posting-stats --sub programming --by hour --agent
  reddit-pp-cli me posting-stats --by dow`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("reddit-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return apiErr(fmt.Errorf("opening store: %w", err))
			}
			defer db.Close()

			report, err := computePostingStats(db, sub, by, minN)
			if err != nil {
				return apiErr(err)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			renderPostingStats(cmd.OutOrStdout(), report)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&sub, "sub", "", "Filter to a specific subreddit")
	cmd.Flags().StringVar(&by, "by", "dow_hour", "Bucket key: hour | dow | dow_hour")
	cmd.Flags().IntVar(&minN, "min-samples", 2, "Minimum samples per bucket to report")
	return cmd
}

func computePostingStats(db *store.Store, sub, by string, minN int) (*postingStatsReport, error) {
	rows, err := db.DB().Query(
		`SELECT data FROM resources WHERE resource_type = 'user_submitted' OR resource_type LIKE 'user_submitted.%'`,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning user_submitted: %w", err)
	}
	defer rows.Close()

	type point struct {
		score     int
		permalink string
		t         time.Time
		sr        string
	}
	pts := []point{}
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			continue
		}
		var env struct {
			Data struct {
				Children []struct {
					Data struct {
						Score      int     `json:"score"`
						Permalink  string  `json:"permalink"`
						CreatedUTC float64 `json:"created_utc"`
						Subreddit  string  `json:"subreddit"`
					} `json:"data"`
				} `json:"children"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(data), &env); err != nil {
			continue
		}
		for _, c := range env.Data.Children {
			if c.Data.CreatedUTC == 0 {
				continue
			}
			if sub != "" && !strings.EqualFold(c.Data.Subreddit, sub) {
				continue
			}
			pts = append(pts, point{
				score:     c.Data.Score,
				permalink: c.Data.Permalink,
				t:         time.Unix(int64(c.Data.CreatedUTC), 0).UTC(),
				sr:        c.Data.Subreddit,
			})
		}
	}

	if len(pts) == 0 {
		// Pre-seed empty buckets so an empty result has the shape the
		// caller expects (24 hours for --by hour, 7 days for --by dow,
		// 168 for dow_hour). Without this an empty buckets array is
		// indistinguishable from "the aggregator silently dropped data"
		// — users could not tell whether the sub has no posts or the
		// bucketing key was wrong.
		return &postingStatsReport{
			Subreddit: sub,
			By:        by,
			Buckets:   emptyPostingBuckets(by),
		}, nil
	}

	buckets := map[string][]point{}
	for _, p := range pts {
		key := postingBucketKey(p.t, by)
		buckets[key] = append(buckets[key], p)
	}

	out := postingStatsReport{Subreddit: sub, By: by, TotalPosts: len(pts), Buckets: []postingBucket{}}
	for k, ps := range buckets {
		if len(ps) < minN {
			continue
		}
		scores := make([]int, len(ps))
		topIdx := 0
		for i, p := range ps {
			scores[i] = p.score
			if p.score > ps[topIdx].score {
				topIdx = i
			}
		}
		sort.Ints(scores)
		median := percentile(scores, 0.5)
		p75 := percentile(scores, 0.75)
		out.Buckets = append(out.Buckets, postingBucket{
			Key:          k,
			SampleCount:  len(ps),
			MedianScore:  median,
			P75Score:     p75,
			TopScore:     ps[topIdx].score,
			TopPermalink: ps[topIdx].permalink,
		})
	}
	sort.Slice(out.Buckets, func(i, j int) bool {
		return out.Buckets[i].MedianScore > out.Buckets[j].MedianScore
	})
	return &out, nil
}

func postingBucketKey(t time.Time, by string) string {
	dow := t.Weekday().String()[:3]
	hour := fmt.Sprintf("%02d", t.Hour())
	switch by {
	case "hour":
		return hour
	case "dow":
		return dow
	default:
		return fmt.Sprintf("%s@%s", dow, hour)
	}
}

// emptyPostingBuckets returns the full key-space for a given --by mode
// with zero counts. Lets the JSON shape stay consistent whether the
// user has 0 posts or 1000 in the queried sub. The aggregator emits
// only buckets where samples >= --min-samples, so this seeding only
// applies on the no-data path — the rendered table still hides empty
// buckets from human output via the TotalPosts == 0 guard in
// renderPostingStats.
func emptyPostingBuckets(by string) []postingBucket {
	days := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	out := []postingBucket{}
	switch by {
	case "hour":
		for h := 0; h < 24; h++ {
			out = append(out, postingBucket{Key: fmt.Sprintf("%02d", h)})
		}
	case "dow":
		for _, d := range days {
			out = append(out, postingBucket{Key: d})
		}
	default: // "dow_hour"
		for _, d := range days {
			for h := 0; h < 24; h++ {
				out = append(out, postingBucket{Key: fmt.Sprintf("%s@%02d", d, h)})
			}
		}
	}
	return out
}

func percentile(sorted []int, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * p)
	return float64(sorted[idx])
}

func renderPostingStats(w io.Writer, r *postingStatsReport) {
	if r.TotalPosts == 0 {
		fmt.Fprintln(w, "No submissions in local store. Run 'reddit-pp-cli sync' first.")
		return
	}
	fmt.Fprintf(w, "Posting stats (sub=%s by=%s total=%d)\n", or(r.Subreddit, "<all>"), r.By, r.TotalPosts)
	fmt.Fprintln(w, "Bucket    Samples  Median   P75      Top")
	for _, b := range r.Buckets {
		fmt.Fprintf(w, "%-9s %-8d %-8.1f %-8.1f %d\n", b.Key, b.SampleCount, b.MedianScore, b.P75Score, b.TopScore)
	}
}

func or(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
