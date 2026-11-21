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
// and source fields are pointers: digg_get_feed does not send them today,
// and the output says null rather than a made-up zero.
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

type feedPageResult struct {
	Posts      []feedPagePost `json:"posts"`
	NextCursor *string        `json:"nextCursor"`
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
	Stop    string `json:"stop"`
	Topic   string `json:"topic"`
	View    string `json:"view"`
	Range   string `json:"range,omitempty"`
	Pages   int    `json:"pages"`
	Posts   int    `json:"posts"`
	ResetAt string `json:"resetAt,omitempty"`
	Error   string `json:"error,omitempty"`
}

type feedPageOpts struct {
	topic, view, feedRange, cursor string
	since                          time.Time
	maxPages                       int
}

func addFeedPaging(cmd *cobra.Command, flags *rootFlags) {
	var o feedPageOpts
	var sinceStr string
	cmd.Long = `Page Digg's feed for one topic through its public MCP tool digg_get_feed and
print one JSON line per post, then one final line with the cursor to resume
from. Run with no subcommand. The raw and story-raw subcommands fetch HTML.

Each post line has kind "post", id, url, topic, title, feedEntryAt,
publishedAt, updatedAt, diggScore, sourceCount, uniqueAuthors and
firstSourceAt. digg_get_feed does not send the last four today, so they are
null; use search for scored results. A post's tldr and description are
never printed.

The final line has kind "cursor", nextCursor, stop (end, empty, since,
max-pages, rate-limited or error), and the page and post counts.

--view newest pages from the newest post back and stops after the first
page that reaches a post older than --since. --view top ranks by Digg Score
over --range; there --since only filters, and --max-pages bounds paging.
Posts are deduplicated by id, because Digg's first page can be cached while
later pages move.

When Digg refuses a page (HTTP 429, or "Retry after" / "Search resets at"),
feed does not retry. It prints the final line with stop "rate-limited", the
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
		return pageFeed(cmd.Context(), mc, o, cmd.OutOrStdout(), time.Now)
	}
	cmd.Flags().StringVar(&o.topic, "topic", "all", "Digg topic to page (one of: "+strings.Join(append(feedTopics(), "all"), ", ")+")")
	cmd.Flags().StringVar(&sinceStr, "since", "", "Only posts published after this: Nh, Nd, Nw, Nm, or an ISO date or datetime")
	cmd.Flags().StringVar(&o.view, "view", "newest", "Feed order: newest | top")
	cmd.Flags().StringVar(&o.feedRange, "range", "", "Window for --view top: today | 24h | 7d | 30d | 1y | all (Digg's default is today)")
	cmd.Flags().IntVar(&o.maxPages, "max-pages", 10, "Most pages to request (20 posts each)")
	cmd.Flags().StringVar(&o.cursor, "cursor", "", "Resume from this nextCursor, printed by an earlier run with the same topic, view and range")
}

// pageFeed requests pages until a stop condition and prints every new post
// as it arrives, then the end line. A refusal returns a rate-limit error.
func pageFeed(ctx context.Context, mc *diggmcp.Client, o feedPageOpts, w io.Writer, now func() time.Time) error {
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
					end.ResetAt = r.ResetAt.UTC().Format(time.RFC3339)
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
		if err := json.Unmarshal(raw, &page); err != nil {
			end.Error = "unreadable digg_get_feed page"
			if perr := finish("error", cursor); perr != nil {
				return perr
			}
			return apiErr(fmt.Errorf("decoding page %d of the %s feed: %w", end.Pages+1, o.topic, err))
		}
		end.Pages++
		crossedSince := false
		for _, p := range page.Posts {
			if p.ID == "" || seen[p.ID] {
				continue
			}
			seen[p.ID] = true
			if !o.since.IsZero() && publishedBefore(p.PublishedAt, o.since) {
				crossedSince = true
				continue
			}
			if err := enc.Encode(feedPostLine{Kind: "post", feedPagePost: p}); err != nil {
				return err
			}
			end.Posts++
		}
		next := ""
		if page.NextCursor != nil {
			next = *page.NextCursor
		}
		switch {
		case next == "":
			return finish("end", "")
		case len(page.Posts) == 0:
			// A cursor with no posts would loop; hand it back instead.
			return finish("empty", next)
		case crossedSince && o.view == "newest":
			return finish("since", next)
		}
		cursor = next
	}
}

// publishedBefore reports whether an RFC 3339 time is before t. An
// unreadable time is kept, not dropped.
func publishedBefore(published string, t time.Time) bool {
	at, err := time.Parse(time.RFC3339Nano, published)
	return err == nil && at.Before(t)
}
