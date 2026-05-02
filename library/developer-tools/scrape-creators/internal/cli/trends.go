package cli

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

func newTrendsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trends",
		Short: "Hashtag and topic trend analytics across platforms",
	}
	cmd.AddCommand(newTrendsDeltaCmd(flags))
	cmd.AddCommand(newTrendsTriangulateCmd(flags))
	return cmd
}

func newTrendsDeltaCmd(flags *rootFlags) *cobra.Command {
	var platform string
	var days int
	cmd := &cobra.Command{
		Use:   "delta <hashtag-or-keyword>",
		Short: "Snapshot a hashtag count and report the delta vs the previous snapshot",
		Long: `Calls the platform's hashtag/keyword search endpoint, records the result
count to trend_snapshots, and reports the delta vs the most recent prior
snapshot up to --days old (default 30). Cron-friendly.`,
		Example:     "  scrape-creators-pp-cli trends delta booktok --platform tiktok --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			topic := args[0]
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			s, err := novelOpenStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()

			path, params := pickTrendEndpoint(platform, topic)
			if path == "" {
				return fmt.Errorf("unsupported platform %q (supported: tiktok, youtube, instagram, reddit, threads)", platform)
			}
			raw, err := c.Get(path, params)
			if err != nil {
				return classifyAPIError(err)
			}
			count := countItems(raw)

			// Read previous snapshot count within the look-back window.
			var prevCount int64
			var prevAt string
			cutoff := fmt.Sprintf("-%d days", days)
			s.DB().QueryRowContext(cmd.Context(),
				`SELECT result_count, snapshot_at FROM trend_snapshots
				 WHERE topic = ? AND platform = ?
				   AND snapshot_at > datetime('now', ?)
				 ORDER BY snapshot_at DESC LIMIT 1`,
				topic, platform, cutoff).Scan(&prevCount, &prevAt)

			now := time.Now().UTC().Format(time.RFC3339)
			_, _ = s.DB().ExecContext(cmd.Context(),
				`INSERT INTO trend_snapshots (topic, platform, result_count, snapshot_at) VALUES (?, ?, ?, ?)`,
				topic, platform, count, now)

			delta := count - prevCount
			result := map[string]any{
				"topic":       topic,
				"platform":    platform,
				"current":     count,
				"previous":    prevCount,
				"previous_at": prevAt,
				"delta":       delta,
				"snapshot_at": now,
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s on %s — %d (was %d, delta %+d)\n", topic, platform, count, prevCount, delta)
			return nil
		},
	}
	cmd.Flags().StringVar(&platform, "platform", "tiktok", "Platform to snapshot on")
	cmd.Flags().IntVar(&days, "days", 30, "Look-back window for the previous snapshot (in days)")
	return cmd
}

func newTrendsTriangulateCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "triangulate <topic>",
		Short: "Snapshot a topic across TikTok + YouTube + Reddit + Threads in parallel",
		Long: `Probes a hashtag/keyword on every searchable platform in parallel and
returns per-platform counts plus a delta against the previous triangulate
snapshot. Reveals which platform a trend is rising fastest on.`,
		Example:     "  scrape-creators-pp-cli trends triangulate \"AI agents\" --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			topic := args[0]
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			s, err := novelOpenStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()

			platforms := []string{"tiktok", "youtube", "reddit", "threads"}
			type row struct {
				Platform string `json:"platform"`
				Count    int64  `json:"count"`
				Previous int64  `json:"previous,omitempty"`
				Delta    int64  `json:"delta"`
			}
			results := make([]row, len(platforms))
			now := time.Now().UTC().Format(time.RFC3339)
			var wg sync.WaitGroup
			for i, p := range platforms {
				wg.Add(1)
				go func(i int, p string) {
					defer wg.Done()
					path, params := pickTrendEndpoint(p, topic)
					if path == "" {
						return
					}
					raw, err := c.Get(path, params)
					if err != nil {
						return
					}
					count := countItems(raw)
					var prev int64
					s.DB().QueryRowContext(cmd.Context(),
						`SELECT result_count FROM trend_snapshots
						 WHERE topic = ? AND platform = ?
						 ORDER BY snapshot_at DESC LIMIT 1`,
						topic, p).Scan(&prev)
					_, _ = s.DB().ExecContext(cmd.Context(),
						`INSERT INTO trend_snapshots (topic, platform, result_count, snapshot_at) VALUES (?, ?, ?, ?)`,
						topic, p, count, now)
					results[i] = row{Platform: p, Count: count, Previous: prev, Delta: count - prev}
				}(i, p)
			}
			wg.Wait()

			final := map[string]any{
				"topic":        topic,
				"snapshot_at":  now,
				"per_platform": results,
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), final, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "trends triangulate: %q\n", topic)
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "PLATFORM\tCOUNT\tPREVIOUS\tDELTA")
			for _, r := range results {
				fmt.Fprintf(tw, "%s\t%d\t%d\t%+d\n", r.Platform, r.Count, r.Previous, r.Delta)
			}
			return tw.Flush()
		},
	}
	return cmd
}

// pickTrendEndpoint maps a platform name + topic to the right search endpoint.
func pickTrendEndpoint(platform, topic string) (string, map[string]string) {
	switch platform {
	case "tiktok":
		return "/v1/tiktok/search/hashtag", map[string]string{"hashtag": topic}
	case "youtube":
		return "/v1/youtube/search/hashtag", map[string]string{"hashtag": topic}
	case "instagram":
		return "/v2/instagram/reels/search", map[string]string{"query": topic}
	case "reddit":
		return "/v1/reddit/search", map[string]string{"query": topic}
	case "threads":
		return "/v1/threads/search", map[string]string{"query": topic}
	}
	return "", nil
}

// countItems returns the number of array elements at the top level (or in a
// well-known wrapping key like "data" or "videos").
func countItems(raw json.RawMessage) int64 {
	var anyVal any
	if json.Unmarshal(raw, &anyVal) != nil {
		return 0
	}
	if arr, ok := anyVal.([]any); ok {
		return int64(len(arr))
	}
	if obj, ok := anyVal.(map[string]any); ok {
		for _, k := range []string{"data", "videos", "results", "items", "posts", "tweets", "hits"} {
			if arr, ok := obj[k].([]any); ok {
				return int64(len(arr))
			}
		}
	}
	return 0
}
