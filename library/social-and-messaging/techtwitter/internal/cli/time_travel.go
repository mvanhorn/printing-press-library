// Hand-authored novel command: time-travel. A branded port of the homepage
// Time Travel panel — curated tweets for a chosen date, live or fully offline.
//
// pp:data-source auto

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/techtwitter/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/techtwitter/internal/store"
)

func newNovelTimeTravelCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	var topic string
	var tz string

	cmd := &cobra.Command{
		Use:   "time-travel [date]",
		Short: "Show the curated tweets for a specific date (YYYY-MM-DD, today, yesterday, or latest)",
		Long: "Port of the homepage Time Travel panel. Pulls the curated tweets for a date — " +
			"today, yesterday, latest, or YYYY-MM-DD (default latest). Runs live by default; " +
			"with --data-source local it serves the date from the offline mirror instead.",
		Example:     "  techtwitter-pp-cli time-travel 2026-06-07 --limit 10 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch curated tweets for the requested date")
				return nil
			}
			date := "latest"
			if len(args) == 1 {
				date = strings.TrimSpace(args[0])
			}
			if !ttIsValidTravelDate(date) {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid date %q: use today, yesterday, latest, or YYYY-MM-DD", date))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			var tweets []ttTweet
			var source string

			if flags.dataSource == "local" {
				local, err := timeTravelLocal(ctx, ttResolveDB(dbPath), date, topic, limit)
				if err != nil {
					return err
				}
				tweets, source = local, "local"
			} else {
				live, err := timeTravelLive(ctx, flags, date, topic, tz, limit)
				if err != nil {
					if flags.dataSource == "live" {
						return classifyAPIError(err, flags)
					}
					// auto: fall back to the offline mirror.
					fmt.Fprintf(cmd.ErrOrStderr(), "live fetch failed (%v); falling back to local mirror\n", err)
					local, lerr := timeTravelLocal(ctx, ttResolveDB(dbPath), date, topic, limit)
					if lerr != nil {
						return classifyAPIError(fmt.Errorf("live fetch failed (%v) and local fallback failed: %w", err, lerr), flags)
					}
					tweets, source = local, "local"
				} else {
					tweets, source = live, "live"
				}
			}

			if ttWantsJSON(cmd, flags) {
				return ttEmitJSON(cmd, flags, map[string]any{
					"date":   date,
					"source": source,
					"count":  len(tweets),
					"tweets": tweets,
				})
			}
			ttRenderTweetPanel(cmd, fmt.Sprintf("TIME TRAVEL — %s (%s, %d tweets)", date, source, len(tweets)), tweets)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/techtwitter-pp-cli/data.db)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum tweets to return (1-50)")
	cmd.Flags().StringVar(&topic, "topic", "", "Optional keyword/topic filter")
	cmd.Flags().StringVar(&tz, "tz", "", "IANA timezone for day boundaries (default UTC)")
	return cmd
}

func timeTravelLive(ctx context.Context, flags *rootFlags, date, topic, tz string, limit int) ([]ttTweet, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	params := map[string]string{"date": date}
	if limit > 0 {
		params["limit"] = strconv.Itoa(limit)
	}
	if topic != "" {
		params["topic"] = topic
	}
	if tz != "" {
		params["tz"] = tz
	}
	data, err := c.Get(ctx, "/api/command/tweets-by-date", params)
	if err != nil {
		return nil, err
	}
	var env struct {
		Tweets []json.RawMessage `json:"tweets"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return decodeTweets(env.Tweets), nil
}

func timeTravelLocal(ctx context.Context, dbPath, date, topic string, limit int) ([]ttTweet, error) {
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	// Pull stored day-stream and trending tweets, then filter locally by date.
	rows, err := db.DB().QueryContext(ctx,
		`SELECT data FROM resources WHERE resource_type IN ('command-tweets-by-date','tweets','command-hot-takes')`)
	if err != nil {
		return nil, fmt.Errorf("querying mirror: %w", err)
	}
	defer rows.Close()
	raw := make([]json.RawMessage, 0)
	for rows.Next() {
		var d []byte
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		raw = append(raw, json.RawMessage(d))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating resource rows: %w", err)
	}
	all := decodeTweets(raw)

	want := resolveDatePrefix(date)
	out := make([]ttTweet, 0, limit)
	seen := map[string]bool{}
	for _, t := range all {
		if seen[t.ID] {
			continue
		}
		if want != "" && !strings.HasPrefix(t.Timestamp, want) {
			continue
		}
		if topic != "" && !matchesTopic(t, topic) {
			continue
		}
		seen[t.ID] = true
		out = append(out, t)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func decodeTweets(raw []json.RawMessage) []ttTweet {
	out := make([]ttTweet, 0, len(raw))
	for _, r := range raw {
		var t ttTweet
		if err := json.Unmarshal(r, &t); err != nil {
			continue
		}
		t.Engagement = ttEngagement(t.Likes, t.Retweets, t.Comments, t.Bookmarks)
		t.Summary = cliutil.CleanText(t.Summary)
		t.Text = cliutil.CleanText(t.Text)
		out = append(out, t)
	}
	return out
}

// resolveDatePrefix converts a date arg into a YYYY-MM-DD prefix for local
// filtering. "latest" returns "" (no date filter — newest rows win).
func resolveDatePrefix(date string) string {
	switch strings.ToLower(date) {
	case "", "latest":
		return ""
	case "today":
		return time.Now().UTC().Format("2006-01-02")
	case "yesterday":
		return time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	default:
		return date
	}
}

func matchesTopic(t ttTweet, topic string) bool {
	topic = strings.ToLower(topic)
	return strings.Contains(strings.ToLower(t.Text), topic) ||
		strings.Contains(strings.ToLower(t.Summary), topic)
}
