// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/diggmcp"
	"github.com/spf13/cobra"
)

// `feed` pages Digg's digg_get_feed MCP tool for one topic and prints one
// JSON line per post, then a final line with the cursor to resume from. It
// is built for a collector that keeps its own copy of the feed: it never
// retries a refusal, and it never prints a post's tldr or description.

const (
	diggGetFeedTool = "digg_get_feed"
	// feedPageSize is the most posts digg_get_feed returns per page.
	feedPageSize = 20
)

var (
	feedPageViews  = []string{"newest", "top"}
	feedPageRanges = []string{"today", "24h", "7d", "30d", "1y", "all"}
)

// feedPagePost is one post as digg_get_feed sends it. tldr and description
// are deliberately not decoded, so they cannot reach the output. The score
// and source fields are pointers: digg_get_feed does not send them, the
// ranked feed fills them when it lists the post (feed_scores.go), and
// otherwise the output says null rather than a made-up zero.
type feedPagePost struct {
	ID            string   `json:"id"`
	URL           string   `json:"url"`
	Topic         string   `json:"topic"`
	Title         string   `json:"title"`
	FeedEntryAt   string   `json:"feedEntryAt"`
	PublishedAt   string   `json:"publishedAt"`
	UpdatedAt     string   `json:"updatedAt"`
	DiggScore     *float64 `json:"diggScore"`
	SourceCount   *int     `json:"sourceCount"`
	UniqueAuthors *int     `json:"uniqueAuthors"`
	FirstSourceAt *string  `json:"firstSourceAt"`
}

// feedPageResult is one digg_get_feed page. Both fields are pointers or
// raw so a page missing either one is caught instead of read as the end.
type feedPageResult struct {
	Posts      *[]feedPagePost `json:"posts"`
	NextCursor json.RawMessage `json:"nextCursor"`
}

// feedPostLine is one printed post.
type feedPostLine struct {
	Kind string `json:"kind"` // always "post"
	feedPagePost
}

// feedEndLine is the last printed line. nextCursor is where to resume:
// Digg's next page after a normal stop, or the refused page's own cursor
// after a refusal or error. It is null when the feed has no more pages,
// or when the refused page was the first one and no --cursor was given.
type feedEndLine struct {
	Kind       string  `json:"kind"` // always "cursor"
	NextCursor *string `json:"nextCursor"`
	// Stop is why paging ended: end, empty, since, max-pages,
	// rate-limited or error.
	Stop  string `json:"stop"`
	Topic string `json:"topic"`
	View  string `json:"view"`
	Range string `json:"range,omitempty"`
	Pages int    `json:"pages"`
	Posts int    `json:"posts"`
	// Scored counts printed posts the ranked feed listed; ScorePages and
	// ScoreRange describe that walk. All are omitted with --scores=false.
	Scored     *int   `json:"scored,omitempty"`
	ScorePages *int   `json:"scorePages,omitempty"`
	ScoreRange string `json:"scoreRange,omitempty"`
	ResetAt    string `json:"resetAt,omitempty"`
	Error      string `json:"error,omitempty"`
}

type feedPageOpts struct {
	topic, view, feedRange, cursor string
	since                          time.Time
	maxPages                       int
	scores                         bool
}

func addFeedPaging(cmd *cobra.Command, flags *rootFlags) {
	var o feedPageOpts
	var sinceStr string
	cmd.Long = `Page Digg's feed for one topic through its public MCP tool digg_get_feed and
print one JSON line per post, then one final line with the cursor to resume
from. Run with no subcommand. The raw and story-raw subcommands fetch HTML.

Each post line has kind "post", id, url, topic, title, feedEntryAt,
publishedAt, updatedAt, diggScore, sourceCount, uniqueAuthors and
firstSourceAt. A post's tldr and description are never printed.

Scores: digg_get_feed sends none, so unless --scores=false, feed first walks
the same topic's ranked story feed once (digg.com/api/feed/continuous, the
feed top reads; up to --max-pages pages over the window --since or --range
covers) and fills diggScore, sourceCount and uniqueAuthors for every post
it lists, joined by story UUID, with the short id in the URL as a fallback.
A post the ranked feed did not list keeps null scores. The ranked feed has
no first-source time, so firstSourceAt is always null. Scores need one
topic: with --topic all (the default) feed lists without scores.

The final line has kind "cursor", nextCursor, stop (end, empty, since,
max-pages, rate-limited or error), the page and post counts, and with
scores, how many posts were scored, the ranked pages read and their window.

--view newest pages from the newest post back and stops after the first
page that reaches a post older than --since. --view top ranks by Digg Score
over --range; there --since only filters, and --max-pages bounds paging.
Posts are deduplicated by id, because Digg's first page can be cached while
later pages move.

When Digg refuses a page of either feed (HTTP 429, or "Retry after" /
"Search resets at"), feed does not retry. It prints the final line with stop "rate-limited", the
reset time and the refused page's cursor, and exits 7. Resume with
--cursor after resetAt; calling earlier makes Digg's wait longer.`
	cmd.Example = `  # The newest AI posts from the last day
  digg-pp-cli feed --topic ai --since 24h --max-pages 5

  # Resume a backfill where the last run stopped
  digg-pp-cli feed --topic tech --since 30d --max-pages 50 --cursor "$CURSOR"

  # The top sports posts of the week, three pages
  digg-pp-cli feed --topic sports --view top --range 7d --max-pages 3`
	cmd.Annotations = map[string]string{"mcp:read-only": "true"}
	cmd.Args = cobra.NoArgs
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		topics := append(feedTopics(), "all")
		if !slices.Contains(topics, o.topic) {
			return usageErr(fmt.Errorf("unknown --topic %q; valid: %s", o.topic, strings.Join(topics, ", ")))
		}
		if !slices.Contains(feedPageViews, o.view) {
			return usageErr(fmt.Errorf("invalid --view %q; valid: %s", o.view, strings.Join(feedPageViews, ", ")))
		}
		if o.feedRange != "" {
			if o.view != "top" {
				return usageErr(fmt.Errorf("--range applies only to --view top"))
			}
			if !slices.Contains(feedPageRanges, o.feedRange) {
				return usageErr(fmt.Errorf("invalid --range %q; valid: %s", o.feedRange, strings.Join(feedPageRanges, ", ")))
			}
		}
		if o.scores && o.topic == "all" {
			// Plain `feed` (every topic) lists without scores; asking for
			// scores across every topic explicitly is the unsupported case.
			if cmd.Flags().Changed("scores") {
				return usageErr(fmt.Errorf("scores come from one topic's ranked feed; pass --topic <topic>, or drop --scores to page every topic unscored"))
			}
			o.scores = false
		}
		if o.maxPages < 1 {
			return usageErr(fmt.Errorf("--max-pages must be at least 1, got %d", o.maxPages))
		}
		if s := strings.TrimSpace(sinceStr); s != "" {
			at, _, err := parseSearchTime(s, time.Now())
			if err != nil {
				return usageErr(fmt.Errorf("--since: %w", err))
			}
			o.since = at
		}
		if dryRunOK(flags) {
			return nil
		}
		mc, err := newDiggMCPClient(flags)
		if err != nil {
			return err
		}
		mc.NoRetry = true
		var ranked rankedFetch
		if o.scores {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// One pacer for both feeds, so switching between them keeps
			// the delay --rate-limit asks for.
			mc.Limiter = c.Limiter()
			params := client.FeedParams{Topic: o.topic, View: "top", Range: rankedRange(o, time.Now())}
			ranked = func(ctx context.Context, cursor string) (*client.FeedScorePage, error) {
				return c.FetchFeedScores(ctx, feedURLOverride, params, cursor)
			}
		}
		perr := pageFeed(cmd.Context(), mc, ranked, o, cmd.OutOrStdout(), time.Now)
		// A refused or failed run still printed posts and a resume cursor;
		// --deliver must get them too, so deliver here instead of leaving
		// it to Execute, which delivers only on success.
		if derr := deliverLinesNow(flags); derr != nil {
			if perr == nil {
				return derr
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %v\n", derr)
		}
		return perr
	}
	cmd.Flags().StringVar(&o.topic, "topic", "all", "Digg topic to page (one of: "+strings.Join(append(feedTopics(), "all"), ", ")+")")
	cmd.Flags().StringVar(&sinceStr, "since", "", "Only posts published after this: Nh, Nd, Nw, Nm, or an ISO date or datetime")
	cmd.Flags().StringVar(&o.view, "view", "newest", "Feed order: newest | top")
	cmd.Flags().StringVar(&o.feedRange, "range", "", "Window for --view top: today | 24h | 7d | 30d | 1y | all (Digg's default is today)")
	cmd.Flags().IntVar(&o.maxPages, "max-pages", 10, "Most pages to request (20 posts each)")
	cmd.Flags().StringVar(&o.cursor, "cursor", "", "Resume from this nextCursor, printed by an earlier run with the same topic, view and range")
	cmd.Flags().BoolVar(&o.scores, "scores", true, "Add Digg Score, source and author counts from the topic's ranked feed (one walk, up to --max-pages pages)")
}

// pageFeed requests pages until a stop condition and prints every new post
// as it arrives, then the end line. A refusal returns a rate-limit error.
// A nil ranked skips scores.
func pageFeed(ctx context.Context, mc *diggmcp.Client, ranked rankedFetch, o feedPageOpts, w io.Writer, now func() time.Time) error {
	enc := json.NewEncoder(w)
	end := feedEndLine{Kind: "cursor", Topic: o.topic, View: o.view, Range: o.feedRange}
	finish := func(stop string, cursor string) error {
		end.Stop = stop
		if cursor != "" {
			c := cursor
			end.NextCursor = &c
		}
		return enc.Encode(end)
	}
	var scores *scoreIndex
	if ranked != nil {
		idx, err := walkRanked(ctx, ranked, o.maxPages)
		scored, pages := 0, idx.pages
		end.Scored, end.ScorePages, end.ScoreRange = &scored, &pages, rankedRange(o, now())
		if err != nil {
			// No post is printed unscored by accident: a failed walk ends
			// the run before digg_get_feed is asked, at the same cursor.
			if r, ok := rankedRefusal(err, now()); ok {
				if !r.ResetAt.IsZero() {
					end.ResetAt = r.ResetAt.UTC().Format(time.RFC3339Nano)
				}
				end.Error = r.Message
				if perr := finish("rate-limited", o.cursor); perr != nil {
					return perr
				}
				return rateLimitErr(fmt.Errorf("Digg refused page %d of the %s ranked feed; resets at %s; rerun after that time",
					pages+1, o.topic, r.ResetIn(now())))
			}
			end.Error = err.Error()
			if perr := finish("error", o.cursor); perr != nil {
				return perr
			}
			return apiErr(fmt.Errorf("reading page %d of the %s ranked feed: %w", pages+1, o.topic, err))
		}
		scores = idx
	}
	seen := map[string]bool{}
	cursor := o.cursor
	for {
		if end.Pages >= o.maxPages {
			return finish("max-pages", cursor)
		}
		args := map[string]any{"topic": o.topic, "view": o.view, "limit": feedPageSize}
		if o.feedRange != "" {
			args["range"] = o.feedRange
		}
		if cursor != "" {
			args["cursor"] = cursor
		}
		raw, err := mc.Call(ctx, diggGetFeedTool, args)
		if err != nil {
			if r, ok := diggmcp.AsRefusal(err, now()); ok {
				if !r.ResetAt.IsZero() {
					// Full precision: a rounded-down time would invite an early retry.
					end.ResetAt = r.ResetAt.UTC().Format(time.RFC3339Nano)
				}
				end.Error = r.Message
				if perr := finish("rate-limited", cursor); perr != nil {
					return perr
				}
				return rateLimitErr(fmt.Errorf("Digg refused page %d of the %s feed; resets at %s; resume with --cursor from the last line, after that time",
					end.Pages+1, o.topic, r.ResetIn(now())))
			}
			end.Error = err.Error()
			if perr := finish("error", cursor); perr != nil {
				return perr
			}
			return apiErr(fmt.Errorf("reading page %d of the %s feed: %w", end.Pages+1, o.topic, err))
		}
		var page feedPageResult
		next, err := decodeFeedPage(raw, &page)
		if err != nil {
			end.Error = err.Error()
			if perr := finish("error", cursor); perr != nil {
				return perr
			}
			return apiErr(fmt.Errorf("decoding page %d of the %s feed: %w", end.Pages+1, o.topic, err))
		}
		end.Pages++
		crossedSince := false
		for _, p := range *page.Posts {
			if p.ID == "" || seen[p.ID] {
				continue
			}
			seen[p.ID] = true
			if !o.since.IsZero() && publishedBefore(p.PublishedAt, o.since) {
				crossedSince = true
				continue
			}
			if scores.apply(&p) {
				*end.Scored++
			}
			if err := enc.Encode(feedPostLine{Kind: "post", feedPagePost: p}); err != nil {
				return err
			}
			end.Posts++
		}
		switch {
		case next == "":
			return finish("end", "")
		case len(*page.Posts) == 0:
			// A cursor with no posts would loop; hand it back instead.
			return finish("empty", next)
		case crossedSince && o.view == "newest":
			return finish("since", next)
		}
		cursor = next
	}
}

// decodeFeedPage reads one page and its next cursor ("" at the end). A
// page without posts or nextCursor is an error, not the end of the feed.
func decodeFeedPage(raw []byte, page *feedPageResult) (string, error) {
	if err := json.Unmarshal(raw, page); err != nil {
		return "", fmt.Errorf("unreadable digg_get_feed page: %w", err)
	}
	if page.Posts == nil || page.NextCursor == nil {
		return "", fmt.Errorf("digg_get_feed page is missing posts or nextCursor")
	}
	var next *string
	if err := json.Unmarshal(page.NextCursor, &next); err != nil {
		return "", fmt.Errorf("digg_get_feed nextCursor is not a string: %w", err)
	}
	if next == nil {
		return "", nil
	}
	return *next, nil
}

// deliverLinesNow sends what the command printed to --deliver as JSON
// lines (application/x-ndjson for webhooks, whatever --compact says) and
// stops Execute from sending it again.
func deliverLinesNow(flags *rootFlags) error {
	if flags.deliverBuf == nil {
		return nil
	}
	buf := flags.deliverBuf
	flags.deliverBuf = nil
	if err := Deliver(flags.deliverSink, buf.Bytes(), true); err != nil {
		return fmt.Errorf("deliver to %s:%s failed: %w", flags.deliverSink.Scheme, flags.deliverSink.Target, err)
	}
	return nil
}

// publishedBefore reports whether an RFC 3339 time is before t. An
// unreadable time is kept, not dropped.
func publishedBefore(published string, t time.Time) bool {
	at, err := time.Parse(time.RFC3339Nano, published)
	return err == nil && at.Before(t)
}
