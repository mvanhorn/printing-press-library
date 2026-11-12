package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/diggmcp"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/diggparse"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/diggstore"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/store"

	"github.com/spf13/cobra"
)

// registerDiggCommands wires all the Digg-specific novel commands onto
// the root. Called from root.go after the generated commands have been
// registered. Each command shows up as a top-level subcommand.
func registerDiggCommands(root *cobra.Command, flags *rootFlags) {
	root.AddCommand(newTopCmd(flags))
	root.AddCommand(newRisingCmd(flags))
	root.AddCommand(newStoryCmd(flags))
	root.AddCommand(newPostsCmd(flags))
	root.AddCommand(newSearchCmd(flags))
	root.AddCommand(newEventsCmd(flags))
	root.AddCommand(newEvidenceCmd(flags))
	root.AddCommand(newSentimentCmd(flags))
	root.AddCommand(newCrossrefCmd(flags))
	root.AddCommand(newReplacedCmd(flags))
	root.AddCommand(newHistoryCmd(flags))
	root.AddCommand(newAuthorsCmd(flags))
	root.AddCommand(newAuthorCmd(flags))
	root.AddCommand(newWatchCmd(flags))
	root.AddCommand(newOpenCmd(flags))
	root.AddCommand(newStatsCmd(flags))
}

// readOnlyAnnotations declares the MCP-readonly annotation. Used on
// every digg novel command that does not mutate external state — they
// are all read-only against Digg by design.
func readOnlyAnnotations() map[string]string {
	return map[string]string{"mcp:read-only": "true"}
}

// openStore opens the local SQLite store and ensures the digg schema is
// in place. Returns a store wrapper, the *sql.DB, and a close function.
// Callers MUST call close on success and on error.
func openStore(ctx context.Context) (*store.Store, *sql.DB, func() error, error) {
	dbPath := defaultDBPath("digg-pp-cli")
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, nil, func() error { return nil }, fmt.Errorf("opening local database: %w", err)
	}
	db := s.DB()
	if err := diggstore.EnsureSchema(db); err != nil {
		s.Close()
		return nil, nil, func() error { return nil }, err
	}
	return s, db, s.Close, nil
}

// ============== top ==============

type clusterRow struct {
	ClusterID            string  `json:"clusterId"`
	ClusterURLID         string  `json:"clusterUrlId"`
	Label                string  `json:"label,omitempty"`
	Title                string  `json:"title,omitempty"`
	TLDR                 string  `json:"tldr,omitempty"`
	URL                  string  `json:"url,omitempty"`
	Permalink            string  `json:"permalink,omitempty"`
	CurrentRank          int     `json:"currentRank"`
	PeakRank             int     `json:"peakRank,omitempty"`
	PreviousRank         int     `json:"previousRank,omitempty"`
	Delta                int     `json:"delta"`
	GravityScore         float64 `json:"gravityScore,omitempty"`
	NumeratorCount       int     `json:"numeratorCount,omitempty"`
	NumeratorLabel       string  `json:"numeratorLabel,omitempty"`
	Pos6h                float64 `json:"pos6h,omitempty"`
	Pos12h               float64 `json:"pos12h,omitempty"`
	Pos24h               float64 `json:"pos24h,omitempty"`
	Likes                int     `json:"likes,omitempty"`
	Views                int     `json:"views,omitempty"`
	SourceTitle          string  `json:"sourceTitle,omitempty"`
	ReplacementRationale string  `json:"replacementRationale,omitempty"`
	ActivityAt           string  `json:"activityAt,omitempty"`
	LastSeenAt           string  `json:"lastSeenAt,omitempty"`
}

func scanClusters(rows *sql.Rows) ([]clusterRow, error) {
	defer rows.Close()
	var out []clusterRow
	for rows.Next() {
		var c clusterRow
		var peakRank sql.NullInt64
		if err := rows.Scan(&c.ClusterID, &c.ClusterURLID, &c.Label, &c.Title, &c.TLDR,
			&c.URL, &c.Permalink, &c.CurrentRank, &peakRank, &c.PreviousRank, &c.Delta,
			&c.GravityScore, &c.NumeratorCount, &c.NumeratorLabel, &c.Pos6h, &c.Pos12h, &c.Pos24h,
			&c.Likes, &c.Views, &c.SourceTitle, &c.ReplacementRationale, &c.ActivityAt, &c.LastSeenAt); err != nil {
			return nil, err
		}
		c.PeakRank = int(peakRank.Int64)
		out = append(out, c)
	}
	return out, rows.Err()
}

const clusterSelectCols = `cluster_id, cluster_url_id, COALESCE(label,''), COALESCE(title,''), COALESCE(tldr,''),
COALESCE(url,''), COALESCE(permalink,''), COALESCE(current_rank,0), peak_rank, COALESCE(previous_rank,0), COALESCE(delta,0),
COALESCE(gravity_score,0), COALESCE(numerator_count,0), COALESCE(numerator_label,''), COALESCE(pos6h,0), COALESCE(pos12h,0), COALESCE(pos24h,0),
COALESCE(likes,0), COALESCE(views,0), COALESCE(source_title,''), COALESCE(replacement_rationale,''), COALESCE(activity_at,''), COALESCE(last_seen_at,'')`

func newTopCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "top",
		Short:       "List top clusters from the local store, sorted by current rank",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  # Top 20 clusters
  digg-pp-cli top --limit 20

  # JSON for agents, narrowed to a few fields
  digg-pp-cli top --limit 10 --json --select clusterUrlId,label,currentRank,delta,tldr`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			_, db, closeFn, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer closeFn()
			rows, err := db.QueryContext(cmd.Context(),
				`SELECT `+clusterSelectCols+` FROM digg_clusters
				 WHERE current_rank > 0 AND last_seen_at = (SELECT MAX(last_seen_at) FROM digg_clusters)
				 ORDER BY current_rank ASC LIMIT ?`, limit)
			if err != nil {
				return err
			}
			clusters, err := scanClusters(rows)
			if err != nil {
				return err
			}
			if len(clusters) == 0 {
				return emptyHint(cmd, "no clusters in the local store. Run `digg-pp-cli sync` first.")
			}
			return printClusterOutput(cmd, flags, clusters, renderClusterTable)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Max number of clusters to return")
	return cmd
}

func newRisingCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var minDelta int
	cmd := &cobra.Command{
		Use:         "rising",
		Short:       "List clusters with the largest positive rank delta since their last snapshot",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  digg-pp-cli rising --limit 10
  digg-pp-cli rising --min-delta 5 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			_, db, closeFn, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer closeFn()
			rows, err := db.QueryContext(cmd.Context(),
				`SELECT `+clusterSelectCols+` FROM digg_clusters
				 WHERE delta >= ? AND last_seen_at = (SELECT MAX(last_seen_at) FROM digg_clusters)
				 ORDER BY delta DESC, current_rank ASC LIMIT ?`, minDelta, limit)
			if err != nil {
				return err
			}
			clusters, err := scanClusters(rows)
			if err != nil {
				return err
			}
			if len(clusters) == 0 {
				return emptyHint(cmd, "no rising clusters since last sync. Run `digg-pp-cli sync` again later, or lower --min-delta.")
			}
			return printClusterOutput(cmd, flags, clusters, renderClusterTable)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Max number of clusters to return")
	cmd.Flags().IntVar(&minDelta, "min-delta", 1, "Minimum positive rank delta")
	return cmd
}

// ============== story, posts ==============
//
// `story`, `posts` and `open` act on one story in any topic. They read
// Digg's digg_get_post tool, which needs the story's UUID. The argument
// may be that UUID, the short id from the story's URL, or the URL; the
// last two are looked up in the record `search` keeps of every story it
// returns.

const (
	diggGetPostTool = "digg_get_post"
	// Limits digg_get_post enforces on one page of sources.
	storySourcePageMax   = 20
	storySourceOffsetMax = 499
)

var storyUUIDRe = regexp.MustCompile(`^[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}$`)

// storyRef is what an argument tells us about a story before any request.
// ID is always set; URL is empty when only the UUID is known.
type storyRef struct {
	ID      string
	ShortID string
	URL     string
}

// resolveStory identifies the story an argument names. A UUID is used as
// given. A short id or a story URL must have been returned by an earlier
// `search`, which recorded its UUID and topic.
func resolveStory(ctx context.Context, arg string) (storyRef, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return storyRef{}, usageErr(fmt.Errorf("a story is required: its UUID, the short id from its URL, or the URL"))
	}
	if storyUUIDRe.MatchString(arg) {
		return storyRef{ID: strings.ToLower(arg)}, nil
	}
	ref := storyRef{ShortID: arg}
	if strings.Contains(arg, "://") {
		u, err := url.Parse(arg)
		if err != nil || strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.") != "digg.com" {
			return storyRef{}, usageErr(fmt.Errorf("%q is not a Digg story URL (expected https://digg.com/<topic>/<id>)", arg))
		}
		ref.ShortID, ref.URL = storyShortID(arg), arg
	}
	ctx, cancel := context.WithTimeout(ctx, searchRecordWait)
	defer cancel()
	db, closeFn, err := openStoreWaiting(ctx)
	if err != nil {
		return storyRef{}, err
	}
	defer closeFn()
	rec, ok, err := diggstore.LookupPostRef(db, ref.ShortID)
	if err != nil {
		return storyRef{}, err
	}
	if !ok {
		return storyRef{}, notFoundErr(fmt.Errorf("story %q is not in the local record: run `digg-pp-cli search \"<query>\"` to find it first, or pass the story's UUID (clusterId in search results)", ref.ShortID))
	}
	ref.ID = rec.ID
	if ref.URL == "" && rec.Topic != "" {
		ref.URL = "https://digg.com/" + rec.Topic + "/" + ref.ShortID
	}
	return ref, nil
}

// diggStory is the story part of a digg_get_post response.
type diggStory struct {
	ID          string  `json:"id"`
	URL         string  `json:"url"`
	Topic       string  `json:"topic"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	TLDR        *string `json:"tldr"`
	FeedEntryAt string  `json:"feedEntryAt"`
	PublishedAt string  `json:"publishedAt"`
	UpdatedAt   string  `json:"updatedAt"`
}

// diggStoryLink points at the earlier or later story in a running thread.
type diggStoryLink struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	Title       string `json:"title"`
	PublishedAt string `json:"publishedAt"`
}

// diggSource is one X post or web page Digg attributes a story to.
type diggSource struct {
	ID          string  `json:"id"`
	Kind        string  `json:"kind"`
	URL         *string `json:"url"`
	Author      *string `json:"author"`
	Title       *string `json:"title"`
	Text        *string `json:"text"`
	PublishedAt *string `json:"publishedAt"`
}

// diggPostPage is one digg_get_post response.
type diggPostPage struct {
	Story              diggStory      `json:"story"`
	Follows            *diggStoryLink `json:"follows"`
	Newer              *diggStoryLink `json:"newer"`
	Sources            []diggSource   `json:"sources"`
	NextSourceOffset   *int           `json:"nextSourceOffset"`
	SourceLimitReached bool           `json:"sourceLimitReached"`
}

// fetchStory reads a story and up to limit of its sources, following
// Digg's source offset from page to page. A limit of 0 reads every source
// Digg serves.
func fetchStory(ctx context.Context, mc *diggmcp.Client, id string, limit int) (diggPostPage, error) {
	var out diggPostPage
	for offset := 0; ; {
		pageSize := storySourcePageMax
		if limit > 0 {
			pageSize = min(pageSize, limit-len(out.Sources))
		}
		raw, err := mc.Call(ctx, diggGetPostTool, map[string]any{"id": id, "sourceOffset": offset, "sourceLimit": pageSize})
		if err != nil {
			return out, err
		}
		var page diggPostPage
		if err := json.Unmarshal(raw, &page); err != nil {
			return out, fmt.Errorf("%s: unreadable result: %w", diggGetPostTool, err)
		}
		sources := append(out.Sources, page.Sources...)
		out = page
		out.Sources = sources
		next := page.NextSourceOffset
		// An empty page or an offset that does not advance would loop forever.
		if next == nil || len(page.Sources) == 0 || *next <= offset || *next > storySourceOffsetMax {
			break
		}
		if limit > 0 && len(sources) >= limit {
			break
		}
		offset = *next
	}
	return out, nil
}

// diggCallErr maps a failed Digg call to the CLI's exit codes.
func diggCallErr(err error) error {
	var rl *cliutil.RateLimitError
	if errors.As(err, &rl) {
		return rateLimitErr(err)
	}
	return apiErr(err)
}

type postsAuthorRow struct {
	Username    string `json:"username,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
}

// postRow is one source of a story. The field names are the ones `posts`
// has always printed, which existing consumers read: last30days keeps a
// row only when body, author.username and xUrl are all present. xUrl
// holds the source's own URL, an X post or a web page; kind says which.
// A web page has a title and no author.username.
type postRow struct {
	PostXID  string         `json:"post_x_id,omitempty"`
	Kind     string         `json:"kind"`
	PostedAt string         `json:"posted_at,omitempty"`
	Author   postsAuthorRow `json:"author"`
	XURL     string         `json:"xUrl,omitempty"`
	Title    string         `json:"title,omitempty"`
	Body     *string        `json:"body"`
}

type postsEnvelope struct {
	Meta    map[string]any `json:"meta"`
	Results []postRow      `json:"results"`
}

// xPostHandleAndID reads the author handle and post id out of an X post
// URL (https://x.com/<handle>/status/<id>). Digg gives the handle nowhere
// else.
func xPostHandleAndID(postURL string) (handle, id string) {
	u, err := url.Parse(postURL)
	if err != nil {
		return "", ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 3 || parts[1] != "status" || parts[0] == "i" {
		return "", ""
	}
	return parts[0], parts[2]
}

func postRowsFor(sources []diggSource) []postRow {
	deref := func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	}
	rows := make([]postRow, 0, len(sources))
	for _, s := range sources {
		row := postRow{
			Kind:     s.Kind,
			PostedAt: deref(s.PublishedAt),
			Author:   postsAuthorRow{DisplayName: deref(s.Author)},
			XURL:     deref(s.URL),
			Title:    deref(s.Title),
			Body:     s.Text,
		}
		if s.Kind == "x" {
			row.Author.Username, row.PostXID = xPostHandleAndID(row.XURL)
		}
		rows = append(rows, row)
	}
	return rows
}

func renderPostRows(w io.Writer, rows []postRow) {
	for _, r := range rows {
		who := firstNonEmpty(r.Author.DisplayName, r.Title, r.Kind)
		if r.Author.Username != "" {
			who = "@" + r.Author.Username + "  " + r.Author.DisplayName
		}
		fmt.Fprintf(w, "%s  %s\n", strings.TrimSpace(who), r.PostedAt)
		if r.Title != "" && who != r.Title {
			fmt.Fprintf(w, "  %s\n", diggTruncate(r.Title, 200))
		}
		if r.Body != nil && *r.Body != "" {
			fmt.Fprintf(w, "  %s\n", diggTruncate(*r.Body, 200))
		}
		if r.XURL != "" {
			fmt.Fprintf(w, "  %s\n", r.XURL)
		}
	}
}

func newStoryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "story [story]",
		Short: "Show one story's detail and sources, live from Digg, in any topic",
		Long: `Show one story as Digg publishes it: title, summary, topic, URL and dates,
the earlier and later stories in the same thread when there are any, and its
first ` + strconv.Itoa(storySourcePageMax) + ` sources. Use ` + "`posts`" + ` for every source.

The argument is the story's UUID (clusterId in search results), the short id
from its URL (clusterUrlId), or the URL. A short id or URL works once
` + "`search`" + ` has returned that story.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  digg-pp-cli story iq7usf9e
  digg-pp-cli story iq7usf9e --json
  digg-pp-cli story https://digg.com/sports/iq7usf9e --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			ref, err := resolveStory(ctx, args[0])
			if err != nil {
				return err
			}
			mc, err := newDiggMCPClient(flags)
			if err != nil {
				return err
			}
			page, err := fetchStory(ctx, mc, ref.ID, storySourcePageMax)
			if err != nil {
				return diggCallErr(err)
			}
			s := page.Story
			rows := postRowsFor(page.Sources)
			if flags.asJSON {
				story := map[string]any{
					"id": s.ID, "clusterUrlId": storyShortID(s.URL), "url": s.URL, "topic": s.Topic,
					"title": s.Title, "description": s.Description, "tldr": s.TLDR,
					"feedEntryAt": s.FeedEntryAt, "publishedAt": s.PublishedAt, "updatedAt": s.UpdatedAt,
				}
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"story": story, "follows": page.Follows, "newer": page.Newer, "posts": rows,
				}, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s  [%s]\n", s.Title, s.Topic)
			fmt.Fprintf(w, "  %s\n  published %s\n", s.URL, s.PublishedAt)
			if page.Follows != nil {
				fmt.Fprintf(w, "  follows: %s  %s\n", page.Follows.Title, page.Follows.URL)
			}
			if page.Newer != nil {
				fmt.Fprintf(w, "  newer:   %s  %s\n", page.Newer.Title, page.Newer.URL)
			}
			if s.TLDR != nil && *s.TLDR != "" {
				fmt.Fprintf(w, "\n%s\n", *s.TLDR)
			} else if s.Description != "" {
				fmt.Fprintf(w, "\n%s\n", s.Description)
			}
			if len(rows) > 0 {
				fmt.Fprintf(w, "\nsources:\n")
				renderPostRows(w, rows)
			}
			return nil
		},
	}
	return cmd
}

func newPostsCmd(flags *rootFlags) *cobra.Command {
	var by string
	var typeFilter string
	var limit int
	cmd := &cobra.Command{
		Use:         "posts <story>",
		Short:       "The sources of one story in any topic: text, author, URL and time",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `List the sources Digg attributes one story to, live from Digg. Each row
carries the source's text (body), its URL (xUrl), when it was published
(posted_at) and its kind: an X post (kind "x", with author.username and
author.display_name) or a web page (kind "webpage", with a title).

The argument is the story's UUID (clusterId in search results), the short id
from its URL (clusterUrlId), or the URL. A short id or URL works once
` + "`search`" + ` has returned that story.

Rows come in Digg's order. --by time sorts them oldest first. --by rank,
--by type and --type are accepted and have no effect: Digg no longer
reports author rank or post type.`,
		Example: `  digg-pp-cli posts 65idu2x5
  digg-pp-cli posts 65idu2x5 --limit 5 --json
  digg-pp-cli posts 65idu2x5 --by time --agent
  digg-pp-cli posts https://digg.com/sports/65idu2x5 --agent --select author.username,xUrl,body`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if !slices.Contains([]string{"rank", "type", "time"}, by) {
				return usageErr(fmt.Errorf("--by must be one of: rank, type, time"))
			}
			if limit < 0 {
				return usageErr(fmt.Errorf("--limit must be 0 or more, got %d", limit))
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			ref, err := resolveStory(ctx, args[0])
			if err != nil {
				return err
			}
			mc, err := newDiggMCPClient(flags)
			if err != nil {
				return err
			}
			page, err := fetchStory(ctx, mc, ref.ID, limit)
			if err != nil {
				return diggCallErr(err)
			}
			rows := postRowsFor(page.Sources)
			if by == "time" {
				sort.SliceStable(rows, func(i, j int) bool { return rows[i].PostedAt < rows[j].PostedAt })
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), postsEnvelope{
					Meta: map[string]any{
						"clusterId":    page.Story.ID,
						"clusterUrlId": storyShortID(page.Story.URL),
						"topic":        page.Story.Topic,
						"url":          page.Story.URL,
						"title":        page.Story.Title,
						"source":       "live",
						"by":           by,
						"count":        len(rows),
					},
					Results: rows,
				}, flags)
			}
			if len(rows) == 0 {
				return emptyHint(cmd, fmt.Sprintf("Digg lists no sources for %s.", page.Story.URL))
			}
			renderPostRows(cmd.OutOrStdout(), rows)
			return nil
		},
	}
	cmd.Flags().StringVar(&by, "by", "rank", "Order: time sorts oldest first; rank and type are accepted and keep Digg's order")
	cmd.Flags().StringVar(&typeFilter, "type", "", "Accepted for older callers; has no effect (Digg no longer reports post type)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Cap results (0 = every source Digg serves)")
	return cmd
}

// ============== search ==============
//
// Search asks Digg's own search (the digg_search_posts tool on its public
// MCP server) and prints what it returns. It needs no local copy of the
// feed and covers every public topic.

// searchResult is one search row. The first seven fields keep the names
// the removed /api/search/stories used, which existing consumers read
// (last30days reads clusterUrlId, title, tldr, postCount, uniqueAuthors,
// firstPostAge). The rest are Digg's engagement numbers and dates.
type searchResult struct {
	ClusterID     string   `json:"clusterId"`
	ClusterURLID  string   `json:"clusterUrlId"`
	Title         string   `json:"title"`
	TLDR          string   `json:"tldr,omitempty"`
	PostCount     int      `json:"postCount"`
	UniqueAuthors int      `json:"uniqueAuthors"`
	FirstPostAge  string   `json:"firstPostAge,omitempty"`
	URL           string   `json:"url"`
	Topic         string   `json:"topic"`
	DiggScore     *float64 `json:"diggScore,omitempty"`
	SourceCount   int      `json:"sourceCount"`
	PublishedAt   string   `json:"publishedAt"`
	FirstSourceAt string   `json:"firstSourceAt,omitempty"`
	Description   string   `json:"description"`
}

// searchEnvelope wraps the result list with provenance metadata. Shape
// matches `authors list` so SDK consumers can reuse the parser.
type searchEnvelope struct {
	Meta    map[string]any `json:"meta"`
	Results []searchResult `json:"results"`
}

// diggSearchPost is one story in a digg_search_posts page.
type diggSearchPost struct {
	ID            string   `json:"id"`
	URL           string   `json:"url"`
	Topic         string   `json:"topic"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	TLDR          *string  `json:"tldr"`
	PublishedAt   string   `json:"publishedAt"`
	DiggScore     *float64 `json:"diggScore"`
	SourceCount   int      `json:"sourceCount"`
	UniqueAuthors int      `json:"uniqueAuthors"`
	FirstSourceAt *string  `json:"firstSourceAt"`
}

// diggSearchPage is one digg_search_posts response.
type diggSearchPage struct {
	Posts        []diggSearchPost `json:"posts"`
	NextCursor   *string          `json:"nextCursor"`
	TotalMatches int              `json:"totalMatches"`
	Window       json.RawMessage  `json:"window"`
	Sort         string           `json:"sort"`
}

const (
	diggSearchTool = "digg_search_posts"
	// Limits Digg's search enforces; checking them here turns a server
	// rejection into a usage error before any request.
	searchQueryMin  = 3
	searchQueryMax  = 120
	searchPageMax   = 50
	searchMaxWindow = 90 * 24 * time.Hour
	// searchRecordWait bounds the write and the lookup of story ids, so a
	// busy store cannot hold up a caller indefinitely.
	searchRecordWait = 5 * time.Second
)

var searchSorts = []string{"relevance", "score", "newest"}

// diggMCPURLOverride, when set, replaces Digg's MCP endpoint. Tests only.
var diggMCPURLOverride string

// newDiggMCPClient builds the MCP client on the CLI's limiter and timeout.
func newDiggMCPClient(flags *rootFlags) (*diggmcp.Client, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	return &diggmcp.Client{
		Endpoint:  diggMCPURLOverride,
		Limiter:   c.Limiter(),
		Timeout:   c.ConfiguredTimeout(),
		UserAgent: "digg-pp-cli/" + version,
	}, nil
}

// formatStoryAge renders an age with hour and day tokens only. Under a
// day it rounds up to whole hours and never prints 0h; otherwise it
// counts whole days. It never uses a month token: older readers take "m"
// for months and drop the row.
func formatStoryAge(d time.Duration) string {
	if d < 24*time.Hour {
		h := int((d + time.Hour - 1) / time.Hour)
		if h < 1 {
			h = 1
		}
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dd", int(d/(24*time.Hour)))
}

// parseSearchTime reads a --since or --until value: a relative age (Nh,
// Nd, Nw, Nm) counted back from now, or an ISO date or datetime.
func parseSearchTime(s string, now time.Time) (at time.Time, age time.Duration, err error) {
	if d, derr := cliutil.ParseDiggAge(s); derr == nil {
		return now.Add(-d), d, nil
	}
	if t, terr := time.Parse(time.RFC3339, s); terr == nil {
		return t, 0, nil
	}
	if t, terr := time.Parse("2006-01-02", s); terr == nil {
		return t, 0, nil
	}
	return time.Time{}, 0, fmt.Errorf("%q is not a duration (Nh, Nd, Nw, Nm) or an ISO date (2026-11-01 or 2026-11-01T00:00:00Z)", s)
}

// searchWindowArgs turns --since and --until into digg_search_posts
// window arguments. A bare 24h, 7d or 30d becomes Digg's range; anything
// else is an absolute since. Digg rejects range combined with since or
// until, so --until always travels with an absolute since.
func searchWindowArgs(sinceStr, untilStr string, now time.Time) (map[string]any, error) {
	args := map[string]any{}
	sinceStr, untilStr = strings.TrimSpace(sinceStr), strings.TrimSpace(untilStr)
	oldest := now.Add(-searchMaxWindow)
	var since time.Time
	if sinceStr != "" {
		at, age, err := parseSearchTime(sinceStr, now)
		if err != nil {
			return nil, usageErr(fmt.Errorf("--since: %w", err))
		}
		if age > searchMaxWindow || at.Before(oldest) {
			return nil, usageErr(fmt.Errorf("--since %q is more than 90 days back; Digg search covers the last 90 days", sinceStr))
		}
		if untilStr == "" {
			switch age {
			case 24 * time.Hour:
				args["range"] = "24h"
				return args, nil
			case 7 * 24 * time.Hour:
				args["range"] = "7d"
				return args, nil
			case 30 * 24 * time.Hour:
				args["range"] = "30d"
				return args, nil
			}
		}
		since = at
	}
	if untilStr != "" {
		until, _, err := parseSearchTime(untilStr, now)
		if err != nil {
			return nil, usageErr(fmt.Errorf("--until: %w", err))
		}
		if since.IsZero() {
			// The widest window Digg serves, kept just inside its limit.
			since = oldest.Add(time.Minute)
		}
		args["until"] = until.UTC().Format(time.RFC3339)
	}
	if !since.IsZero() {
		args["since"] = since.UTC().Format(time.RFC3339)
	}
	return args, nil
}

// storyShortID returns the last path segment of a story URL, the id Digg
// uses in links (https://digg.com/<topic>/<shortId>).
func storyShortID(storyURL string) string {
	p := storyURL
	if i := strings.IndexAny(p, "?#"); i >= 0 {
		p = p[:i]
	}
	p = strings.TrimRight(p, "/")
	return p[strings.LastIndex(p, "/")+1:]
}

func searchResultFor(p diggSearchPost, now time.Time) searchResult {
	r := searchResult{
		ClusterID:     p.ID,
		ClusterURLID:  storyShortID(p.URL),
		Title:         p.Title,
		PostCount:     p.SourceCount,
		UniqueAuthors: p.UniqueAuthors,
		URL:           p.URL,
		Topic:         p.Topic,
		DiggScore:     p.DiggScore,
		SourceCount:   p.SourceCount,
		PublishedAt:   p.PublishedAt,
		Description:   p.Description,
	}
	if p.TLDR != nil {
		r.TLDR = *p.TLDR
	}
	if p.FirstSourceAt != nil {
		r.FirstSourceAt = *p.FirstSourceAt
	}
	if at, err := time.Parse(time.RFC3339, p.PublishedAt); err == nil {
		r.FirstPostAge = formatStoryAge(now.Sub(at))
	}
	return r
}

// openStoreWaiting opens the local store. Commands started together on a
// store that does not exist yet race to create it, and the losers see the
// database as locked; this waits for the winner until ctx ends.
func openStoreWaiting(ctx context.Context) (*sql.DB, func() error, error) {
	for {
		_, db, closeFn, err := openStore(ctx)
		if err == nil {
			return db, closeFn, nil
		}
		if msg := err.Error(); !strings.Contains(msg, "SQLITE_BUSY") && !strings.Contains(msg, "database is locked") {
			return nil, nil, err
		}
		select {
		case <-ctx.Done():
			return nil, nil, err
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// recordSearchRefs stores each result's short id, UUID and topic so a
// later command given only a short id can find the story.
func recordSearchRefs(ctx context.Context, results []searchResult) error {
	ctx, cancel := context.WithTimeout(ctx, searchRecordWait)
	defer cancel()
	db, closeFn, err := openStoreWaiting(ctx)
	if err != nil {
		return err
	}
	defer closeFn()
	refs := make([]diggstore.PostRef, 0, len(results))
	for _, r := range results {
		refs = append(refs, diggstore.PostRef{ShortID: r.ClusterURLID, ID: r.ClusterID, Topic: r.Topic})
	}
	return diggstore.RecordPostRefs(ctx, db, refs, time.Now())
}

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var sinceStr, untilStr, sortBy string
	var topics []string
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search Digg stories live, across one or more topics, with Digg Score and source counts",
		Long: `Search Digg stories with Digg's own search.

Every search is live: it needs no prior ` + "`sync`" + ` and reads no local copy.
--topic limits the search to one or more topics (valid: ` + strings.Join(validTopicSlugs(), ", ") + `);
without it every topic is searched.

Each result carries its Digg Score (diggScore), how many sources the story
lists (sourceCount), how many different authors wrote them (uniqueAuthors),
when it was published (publishedAt), when its earliest source appeared
(firstSourceAt), its topic and its URL.

--since and --until take a duration counted back from now (Nh, Nd, Nw, or
Nm for months of 30 days) or an ISO date such as 2026-11-01. They filter on
publishedAt. Digg search covers the last 90 days. --sort is relevance
(default), score (highest Digg Score first) or newest. --limit may exceed
one page; search follows Digg's cursor until it has that many results or
the matches run out.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  digg-pp-cli search "openai gpt-5"
  digg-pp-cli search "<topic>" --since 30d --agent
  digg-pp-cli search "claude" --topic ai,tech --since 7d --sort score --json
  digg-pp-cli search "world cup" --topic sports --since 2026-11-01 --until 2026-11-08
  digg-pp-cli search "robotics" --limit 120 --json --select title,diggScore,url`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if n := utf8.RuneCountInString(query); n < searchQueryMin || n > searchQueryMax {
				return usageErr(fmt.Errorf("query must be %d to %d characters, got %d (e.g. `digg-pp-cli search \"openai\"`)", searchQueryMin, searchQueryMax, n))
			}
			if err := validateTopics(topics); err != nil {
				return err
			}
			if !slices.Contains(searchSorts, sortBy) {
				return usageErr(fmt.Errorf("invalid --sort %q; valid: %s", sortBy, strings.Join(searchSorts, ", ")))
			}
			if limit < 1 {
				return usageErr(fmt.Errorf("--limit must be at least 1, got %d", limit))
			}
			now := time.Now()
			callArgs, err := searchWindowArgs(sinceStr, untilStr, now)
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}
			callArgs["query"] = query
			callArgs["sort"] = sortBy
			// Digg reads a missing topic, like "all", as every topic.
			metaTopics := []string{"all"}
			if len(topics) > 0 && !slices.Contains(topics, "all") {
				callArgs["topic"] = topics
				metaTopics = topics
			}
			meta := map[string]any{
				"source": "live",
				"stale":  false,
				"query":  query,
				"count":  0,
				"topics": metaTopics,
				"sort":   sortBy,
			}
			if sinceStr != "" {
				meta["since"] = sinceStr
			}
			if untilStr != "" {
				meta["until"] = untilStr
			}
			// fail prints the envelope with the error in it, so a JSON
			// reader never mistakes a failed search for an empty one.
			fail := func(err error) error {
				if flags.asJSON {
					meta["error"] = err.Error()
					_ = printJSONFiltered(cmd.OutOrStdout(), searchEnvelope{Meta: meta, Results: []searchResult{}}, flags)
				}
				return diggCallErr(err)
			}

			ctx := cmd.Context()
			mc, err := newDiggMCPClient(flags)
			if err != nil {
				return err
			}
			results := make([]searchResult, 0, limit)
			for cursor, first := "", true; len(results) < limit; first = false {
				callArgs["limit"] = min(searchPageMax, limit-len(results))
				if cursor != "" {
					callArgs["cursor"] = cursor
				}
				raw, err := mc.Call(ctx, diggSearchTool, callArgs)
				if err != nil {
					return fail(err)
				}
				var page diggSearchPage
				if err := json.Unmarshal(raw, &page); err != nil {
					return fail(fmt.Errorf("%s: unreadable result: %w", diggSearchTool, err))
				}
				if first {
					meta["totalMatches"] = page.TotalMatches
					if len(page.Window) > 0 {
						meta["window"] = page.Window
					}
					if page.Sort != "" {
						meta["sort"] = page.Sort
					}
				}
				for _, p := range page.Posts {
					results = append(results, searchResultFor(p, now))
				}
				// An empty page with a cursor would otherwise loop forever.
				if page.NextCursor == nil || *page.NextCursor == "" || len(page.Posts) == 0 {
					break
				}
				cursor = *page.NextCursor
			}
			if len(results) > limit {
				results = results[:limit]
			}
			meta["count"] = len(results)

			// Best-effort: several searches may share one store, and a
			// failed write must not cost the caller its results.
			if err := recordSearchRefs(ctx, results); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warn: could not record story ids in the local store: %v\n", err)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), searchEnvelope{Meta: meta, Results: results}, flags)
			}
			if len(results) == 0 {
				return emptyHint(cmd, fmt.Sprintf("no matches for %q in %s. Try a different query or a wider --since.", query, strings.Join(metaTopics, ",")))
			}
			for _, r := range results {
				score := "-"
				if r.DiggScore != nil {
					score = strconv.FormatFloat(*r.DiggScore, 'f', -1, 64)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s  [%s]\n", firstNonEmpty(r.Title, r.ClusterURLID), r.Topic)
				if r.TLDR != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "    %s\n", diggTruncate(r.TLDR, 200))
				}
				fmt.Fprintf(cmd.OutOrStdout(), "    score=%s  sources=%d  authors=%d  age=%s  %s\n", score, r.SourceCount, r.UniqueAuthors, r.FirstPostAge, r.URL)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Max number of results; may exceed one page of 50")
	cmd.Flags().StringVar(&sinceStr, "since", "", "Only stories published at or after this: a duration back from now (30d, 1w, 12h, 1m) or an ISO date (2026-11-01). At most 90 days back. Default empty (no filter).")
	cmd.Flags().StringVar(&untilStr, "until", "", "Only stories published before this: a duration back from now or an ISO date. Default empty (no filter).")
	cmd.Flags().StringVar(&sortBy, "sort", "relevance", "Order: relevance | score | newest")
	cmd.Flags().StringSliceVar(&topics, "topic", nil, "Comma-separated Digg topics to search (valid: "+strings.Join(validTopicSlugs(), ", ")+"). Default: every topic.")
	return cmd
}

// ============== events ==============

func newEventsCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var sinceStr string
	var typeFilter string
	cmd := &cobra.Command{
		Use:         "events",
		Short:       "List Digg pipeline events stored by sync",
		Long:        `List pipeline events stored in the local database. Sync captures the events each section page embeds in its RSC stream (cluster_detected, fast_climb with delta + previousRank → currentRank). This reads stored events only; Digg removed its live pipeline-status endpoint in October 2026, so run ` + "`sync`" + ` to capture new ones.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  digg-pp-cli events --since 1h --type fast_climb
  digg-pp-cli events --type cluster_detected --limit 10 --json
  digg-pp-cli events --json --select clusterId,label,delta,currentRank,previousRank`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			since := parseSinceWithFallback(sinceStr, 24*time.Hour)
			_, db, closeFn, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer closeFn()

			q := `SELECT id, type, COALESCE(cluster_id,''), COALESCE(label,''), COALESCE(username,''), COALESCE(post_type,''), COALESCE(permalink,''),
				COALESCE(delta,0), COALESCE(current_rank,0), COALESCE(previous_rank,0), COALESCE(count,0), COALESCE(total,0),
				COALESCE(at,''), COALESCE(created_at,''), COALESCE(raw_json,'')
				FROM digg_events WHERE 1=1`
			var argsSQL []any
			if !since.IsZero() {
				q += ` AND at >= ?`
				argsSQL = append(argsSQL, since.UTC().Format(time.RFC3339))
			}
			if typeFilter != "" {
				q += ` AND type = ?`
				argsSQL = append(argsSQL, typeFilter)
			}
			q += ` ORDER BY at DESC LIMIT ?`
			argsSQL = append(argsSQL, limit)

			rows, err := db.QueryContext(cmd.Context(), q, argsSQL...)
			if err != nil {
				return err
			}
			defer rows.Close()
			type evRow struct {
				ID           string `json:"id"`
				Type         string `json:"type"`
				ClusterID    string `json:"clusterId,omitempty"`
				Label        string `json:"label,omitempty"`
				Username     string `json:"username,omitempty"`
				PostType     string `json:"postType,omitempty"`
				Permalink    string `json:"permalink,omitempty"`
				Delta        int    `json:"delta,omitempty"`
				CurrentRank  int    `json:"currentRank,omitempty"`
				PreviousRank int    `json:"previousRank,omitempty"`
				Count        int    `json:"count,omitempty"`
				Total        int    `json:"total,omitempty"`
				At           string `json:"at"`
				CreatedAt    string `json:"createdAt,omitempty"`
			}
			var out []evRow
			for rows.Next() {
				var e evRow
				var rawJSON string
				if err := rows.Scan(&e.ID, &e.Type, &e.ClusterID, &e.Label, &e.Username, &e.PostType, &e.Permalink,
					&e.Delta, &e.CurrentRank, &e.PreviousRank, &e.Count, &e.Total,
					&e.At, &e.CreatedAt, &rawJSON); err != nil {
					return err
				}
				out = append(out, e)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			if len(out) == 0 {
				return emptyHint(cmd, "no events in window. Run `digg-pp-cli sync` first, or widen --since.")
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			for _, e := range out {
				switch e.Type {
				case "fast_climb":
					fmt.Fprintf(cmd.OutOrStdout(), "[%s] fast_climb %+d  %d→%d  %s\n", e.At, e.Delta, e.PreviousRank, e.CurrentRank, e.Label)
				case "cluster_detected":
					fmt.Fprintf(cmd.OutOrStdout(), "[%s] cluster_detected  %s\n", e.At, e.Label)
				case "post_understanding":
					fmt.Fprintf(cmd.OutOrStdout(), "[%s] post @%s [%s]  %s\n", e.At, e.Username, e.PostType, e.Permalink)
				default:
					fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s  count=%d\n", e.At, e.Type, e.Count)
				}
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Max number of events to return")
	cmd.Flags().StringVar(&sinceStr, "since", "24h", "Only events at-or-after this duration ago (e.g. 30m, 6h, 2d) or RFC3339")
	cmd.Flags().StringVar(&typeFilter, "type", "", "Filter to event type (cluster_detected, fast_climb, post_understanding, batch_started, batch_breakdown, posts_stored, embedding_progress)")
	return cmd
}

// ============== evidence ==============

func newEvidenceCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "evidence [clusterUrlId]",
		Short:       "Print Digg's published score components and evidence array for one cluster",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  digg-pp-cli evidence iq7usf9e
  digg-pp-cli evidence iq7usf9e --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id := args[0]
			_, db, closeFn, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer closeFn()
			row := db.QueryRowContext(cmd.Context(),
				`SELECT cluster_id, COALESCE(cluster_url_id,''), COALESCE(label,''),
				 COALESCE(gravity_score,0), COALESCE(numerator_count,0), COALESCE(numerator_label,''),
				 COALESCE(percent_above_average,0), COALESCE(score_components_json,''), COALESCE(evidence_json,'')
				 FROM digg_clusters WHERE cluster_url_id = ? OR cluster_id = ?`, id, id)
			var clusterID, urlID, label string
			var gravity, pct float64
			var numeratorCount int
			var numeratorLabel, scoreJSON, evJSON string
			if err := row.Scan(&clusterID, &urlID, &label, &gravity, &numeratorCount, &numeratorLabel, &pct, &scoreJSON, &evJSON); err != nil {
				if err == sql.ErrNoRows {
					return fmt.Errorf("cluster not found: %s", id)
				}
				return err
			}
			result := map[string]any{
				"clusterId":           clusterID,
				"clusterUrlId":        urlID,
				"label":               label,
				"gravityScore":        gravity,
				"numeratorCount":      numeratorCount,
				"numeratorLabel":      numeratorLabel,
				"percentAboveAverage": pct,
				"scoreComponents":     asJSONString(scoreJSON),
				"evidence":            asJSONString(evJSON),
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s [%s]\n", label, urlID)
			fmt.Fprintf(cmd.OutOrStdout(), "  gravityScore: %.4f\n", gravity)
			if numeratorLabel != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s: %d\n", numeratorLabel, numeratorCount)
			}
			if pct > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "  percentAboveAverage: %.2f\n", pct)
			}
			if scoreJSON != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  scoreComponents:\n    %s\n", indentJSON(scoreJSON, 4))
			}
			if evJSON != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  evidence:\n    %s\n", indentJSON(evJSON, 4))
			}
			return nil
		},
	}
	return cmd
}

// ============== sentiment ==============

func newSentimentCmd(flags *rootFlags) *cobra.Command {
	var window string
	cmd := &cobra.Command{
		Use:         "sentiment [clusterUrlId]",
		Short:       "Print per-time-window positivity ratios for one cluster (pos6h/pos12h/pos24h)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  digg-pp-cli sentiment iq7usf9e
  digg-pp-cli sentiment iq7usf9e --window 6h
  digg-pp-cli sentiment iq7usf9e --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id := args[0]
			_, db, closeFn, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer closeFn()
			row := db.QueryRowContext(cmd.Context(),
				`SELECT cluster_id, COALESCE(cluster_url_id,''), COALESCE(label,''),
				 COALESCE(pos6h,0), COALESCE(pos12h,0), COALESCE(pos24h,0), COALESCE(pos_last,0)
				 FROM digg_clusters WHERE cluster_url_id = ? OR cluster_id = ?`, id, id)
			var clusterID, urlID, label string
			var p6, p12, p24, pLast float64
			if err := row.Scan(&clusterID, &urlID, &label, &p6, &p12, &p24, &pLast); err != nil {
				if err == sql.ErrNoRows {
					return fmt.Errorf("cluster not found: %s", id)
				}
				return err
			}
			result := map[string]any{
				"clusterId":    clusterID,
				"clusterUrlId": urlID,
				"label":        label,
				"pos6h":        p6, "pos12h": p12, "pos24h": p24, "posLast": pLast,
			}
			if window != "" {
				switch window {
				case "6h":
					result["window"] = window
					result["positivity"] = p6
				case "12h":
					result["window"] = window
					result["positivity"] = p12
				case "24h":
					result["window"] = window
					result["positivity"] = p24
				default:
					return fmt.Errorf("--window must be 6h, 12h, or 24h")
				}
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s [%s]\n  pos6h=%.2f  pos12h=%.2f  pos24h=%.2f  posLast=%.2f\n",
				label, urlID, p6, p12, p24, pLast)
			return nil
		},
	}
	cmd.Flags().StringVar(&window, "window", "", "Restrict output to one window: 6h, 12h, or 24h")
	return cmd
}

// ============== crossref ==============

func newCrossrefCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "crossref [clusterUrlId]",
		Short:       "Show this cluster's Hacker News and Techmeme cross-references",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  digg-pp-cli crossref iq7usf9e
  digg-pp-cli crossref iq7usf9e --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id := args[0]
			_, db, closeFn, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer closeFn()
			row := db.QueryRowContext(cmd.Context(),
				`SELECT cluster_id, COALESCE(cluster_url_id,''), COALESCE(label,''),
				 COALESCE(url,''), COALESCE(permalink,''),
				 COALESCE(hacker_news_json,''), COALESCE(techmeme_json,''), COALESCE(external_feeds_json,'')
				 FROM digg_clusters WHERE cluster_url_id = ? OR cluster_id = ?`, id, id)
			var clusterID, urlID, label, sourceURL, perm, hnJSON, tmJSON, extJSON string
			if err := row.Scan(&clusterID, &urlID, &label, &sourceURL, &perm, &hnJSON, &tmJSON, &extJSON); err != nil {
				if err == sql.ErrNoRows {
					return fmt.Errorf("cluster not found: %s", id)
				}
				return err
			}
			result := map[string]any{
				"clusterId":    clusterID,
				"clusterUrlId": urlID,
				"label":        label,
				"source":       sourceURL,
				"diggURL":      perm,
				"hackerNews":   asJSONString(hnJSON),
				"techmeme":     asJSONString(tmJSON),
				"external":     asJSONString(extJSON),
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s [%s]\n", label, urlID)
			fmt.Fprintf(cmd.OutOrStdout(), "  digg:       %s\n", perm)
			if sourceURL != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  source:     %s\n", sourceURL)
			}
			if hnJSON != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  hackerNews: %s\n", indentJSON(hnJSON, 14))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  hackerNews: (not detected by Digg)\n")
			}
			if tmJSON != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  techmeme:   %s\n", indentJSON(tmJSON, 14))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  techmeme:   (not detected by Digg)\n")
			}
			return nil
		},
	}
	return cmd
}

// ============== replaced ==============

func newReplacedCmd(flags *rootFlags) *cobra.Command {
	var sinceStr string
	var limit int
	cmd := &cobra.Command{
		Use:         "replaced",
		Short:       "Stories that were knocked out of the rankings, with Digg's published rationale",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  digg-pp-cli replaced --since 24h
  digg-pp-cli replaced --json --select clusterUrlId,label,rationale,previousRank`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			since := parseSinceWithFallback(sinceStr, 24*time.Hour)
			_, db, closeFn, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer closeFn()
			rows, err := db.QueryContext(cmd.Context(),
				`SELECT cluster_id, COALESCE(cluster_url_id,''), COALESCE(label,''),
				 observed_at, COALESCE(rationale,''), COALESCE(previous_rank,0)
				 FROM digg_replacements WHERE observed_at >= ?
				 ORDER BY observed_at DESC LIMIT ?`,
				since.UTC().Format(time.RFC3339Nano), limit)
			if err != nil {
				return err
			}
			defer rows.Close()
			type repRow struct {
				ClusterID    string `json:"clusterId"`
				ClusterURLID string `json:"clusterUrlId"`
				Label        string `json:"label"`
				ObservedAt   string `json:"observedAt"`
				Rationale    string `json:"rationale"`
				PreviousRank int    `json:"previousRank,omitempty"`
			}
			var out []repRow
			for rows.Next() {
				var r repRow
				if err := rows.Scan(&r.ClusterID, &r.ClusterURLID, &r.Label, &r.ObservedAt, &r.Rationale, &r.PreviousRank); err != nil {
					return err
				}
				out = append(out, r)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			if len(out) == 0 {
				return emptyHint(cmd, "no replacements recorded in window. Replacement archaeology needs at least 2 syncs over time.")
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			for _, r := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s [%s]  was rank #%d\n  rationale: %s\n",
					r.ObservedAt, r.Label, r.ClusterURLID, r.PreviousRank, r.Rationale)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&sinceStr, "since", "24h", "Lookback window (e.g. 6h, 24h, 7d)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max number of replacements to return")
	return cmd
}

// ============== history ==============

func newHistoryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "history [clusterUrlId]",
		Short:       "Show the rank trajectory of one cluster from local snapshot history",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  digg-pp-cli history iq7usf9e
  digg-pp-cli history iq7usf9e --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id := args[0]
			_, db, closeFn, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer closeFn()
			// Resolve clusterUrlId → clusterId
			var clusterID, label, urlID string
			err = db.QueryRowContext(cmd.Context(),
				`SELECT cluster_id, COALESCE(label,''), COALESCE(cluster_url_id,'')
				 FROM digg_clusters WHERE cluster_url_id = ? OR cluster_id = ?`, id, id).Scan(&clusterID, &label, &urlID)
			if err != nil {
				if err == sql.ErrNoRows {
					return fmt.Errorf("cluster not found: %s", id)
				}
				return err
			}
			rows, err := db.QueryContext(cmd.Context(),
				`SELECT fetched_at, COALESCE(current_rank,0), COALESCE(peak_rank,0), COALESCE(delta,0), COALESCE(gravity_score,0)
				 FROM digg_snapshots WHERE cluster_id = ? ORDER BY fetched_at ASC`, clusterID)
			if err != nil {
				return err
			}
			defer rows.Close()
			type snap struct {
				At           string  `json:"at"`
				Rank         int     `json:"rank"`
				PeakRank     int     `json:"peakRank,omitempty"`
				Delta        int     `json:"delta"`
				GravityScore float64 `json:"gravityScore,omitempty"`
			}
			var snaps []snap
			for rows.Next() {
				var s snap
				if err := rows.Scan(&s.At, &s.Rank, &s.PeakRank, &s.Delta, &s.GravityScore); err != nil {
					return err
				}
				snaps = append(snaps, s)
			}
			result := map[string]any{
				"clusterId":    clusterID,
				"clusterUrlId": urlID,
				"label":        label,
				"snapshots":    snaps,
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			if len(snaps) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "%s [%s]\n  no snapshots yet — run sync over time to build history.\n", label, urlID)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s [%s]\n  %d snapshots\n", label, urlID, len(snaps))
			for _, s := range snaps {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  rank=%d  peak=%d  delta=%+d  gravity=%.2f\n",
					s.At, s.Rank, s.PeakRank, s.Delta, s.GravityScore)
			}
			return nil
		},
	}
	return cmd
}

// ============== authors ==============

func newAuthorsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use: "authors",
		// PATCH(digg-rename-and-github-feeds): drop Digg AI 1000 branding.
		Short:       "Inspect the accounts Digg tracks — the curated leaderboard of AI-news influencers on X",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newAuthorsTopCmd(flags))
	cmd.AddCommand(newAuthorsListCmd(flags))
	cmd.AddCommand(newAuthorsGetCmd(flags))
	// PATCH(digg-enhancements): register `authors overlap` subcommand.
	cmd.AddCommand(newAuthorsOverlapCmd(flags))
	return cmd
}

// PATCH(digg-enhancements): clusters where two tracked X accounts both
// contributed. INTERSECT over digg_cluster_authors; no schema changes.
func newAuthorsOverlapCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "overlap <userA> <userB>",
		Short:       "Clusters where both X accounts contributed",
		Annotations: readOnlyAnnotations(),
		Example: `  digg-pp-cli authors overlap karpathy sama
  digg-pp-cli authors overlap karpathy sama --limit 20 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if len(args) < 2 {
				return fmt.Errorf("overlap requires two usernames (got %d); see --help", len(args))
			}
			if dryRunOK(flags) {
				return nil
			}
			userA, userB := args[0], args[1]

			_, db, closeFn, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer closeFn()

			rows, err := db.QueryContext(cmd.Context(), `
SELECT c.cluster_id, COALESCE(c.cluster_url_id,''), COALESCE(c.title,''),
       COALESCE(c.current_rank,0) AS current_rank,
       COALESCE(c.gravity_score,0) AS gravity_score,
       COALESCE(c.label,''), COALESCE(c.last_seen_at,'')
FROM digg_clusters c
WHERE c.cluster_id IN (
  SELECT ca1.cluster_id FROM digg_cluster_authors ca1
  WHERE ca1.username = ?
  INTERSECT
  SELECT ca2.cluster_id FROM digg_cluster_authors ca2
  WHERE ca2.username = ?
)
ORDER BY CASE WHEN c.current_rank = 0 THEN 9999 ELSE c.current_rank END ASC
LIMIT ?`, userA, userB, limit)
			if err != nil {
				return err
			}
			defer rows.Close()

			type overlapRow struct {
				ClusterID    string  `json:"clusterId"`
				ClusterURLID string  `json:"clusterUrlId,omitempty"`
				Title        string  `json:"title,omitempty"`
				CurrentRank  int     `json:"currentRank"`
				GravityScore float64 `json:"gravityScore,omitempty"`
				Label        string  `json:"label,omitempty"`
				LastSeenAt   string  `json:"lastSeenAt,omitempty"`
			}
			var results []overlapRow
			for rows.Next() {
				var r overlapRow
				if err := rows.Scan(&r.ClusterID, &r.ClusterURLID, &r.Title, &r.CurrentRank, &r.GravityScore, &r.Label, &r.LastSeenAt); err != nil {
					return err
				}
				results = append(results, r)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			if len(results) == 0 {
				return emptyHint(cmd, fmt.Sprintf("no overlapping clusters found for %s and %s. Run `digg-pp-cli sync` to populate.", userA, userB))
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Max number of clusters to return")
	return cmd
}

func newAuthorsTopCmd(flags *rootFlags) *cobra.Command {
	var by string
	var limit int
	cmd := &cobra.Command{
		Use:         "top",
		Short:       "Top contributors across Digg's tracked accounts, ranked by influence, post count, or reach",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  digg-pp-cli authors top --by influence --limit 25
  digg-pp-cli authors top --by posts --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			_, db, closeFn, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer closeFn()
			var orderBy string
			switch by {
			case "influence":
				orderBy = "influence DESC"
			case "posts", "count":
				orderBy = "contributed_count DESC"
			case "reach":
				orderBy = "podist DESC"
			default:
				return fmt.Errorf("--by must be one of: influence, posts, reach")
			}
			rows, err := db.QueryContext(cmd.Context(),
				`SELECT username, COALESCE(display_name,''), COALESCE(x_id,''),
				 COALESCE(influence,0), COALESCE(podist,0), COALESCE(contributed_count,0), COALESCE(last_seen_at,'')
				 FROM digg_authors WHERE username != '' ORDER BY `+orderBy+` LIMIT ?`, limit)
			if err != nil {
				return err
			}
			defer rows.Close()
			type authorRow struct {
				Username         string  `json:"username"`
				DisplayName      string  `json:"displayName,omitempty"`
				XID              string  `json:"xId,omitempty"`
				Influence        float64 `json:"influence,omitempty"`
				Podist           float64 `json:"podist,omitempty"`
				ContributedCount int     `json:"contributedCount"`
				LastSeenAt       string  `json:"lastSeenAt,omitempty"`
			}
			var out []authorRow
			for rows.Next() {
				var a authorRow
				if err := rows.Scan(&a.Username, &a.DisplayName, &a.XID, &a.Influence, &a.Podist, &a.ContributedCount, &a.LastSeenAt); err != nil {
					return err
				}
				out = append(out, a)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			if len(out) == 0 {
				return emptyHint(cmd, "no authors known yet. Run `digg-pp-cli sync` to populate.")
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			for i, a := range out {
				name := firstNonEmpty(a.DisplayName, a.Username)
				fmt.Fprintf(cmd.OutOrStdout(), "%2d. @%-25s %s  influence=%.2f  posts=%d\n",
					i+1, a.Username, diggTruncate(name, 30), a.Influence, a.ContributedCount)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&by, "by", "influence", "Sort by: influence | posts | reach")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max number of authors")
	return cmd
}

// ============== authors list ==============
//
// `authors list` exposes the full ranked AI 1000 with rich per-author
// fields (rank, previousRank, rankChange, score, category, categoryRank,
// followers, bio, githubUrl, vibeDistribution). Live by default: hits
// /ai/1000, parses the embedded RSC stream, upserts into digg_authors,
// reads back from the local store. `--data-source local` skips the
// fetch and reads only the cached roster.
//
// This is the load-bearing surface for "biggest movers", "newly listed",
// and category-rank browsing — the upstream page renders all 1,000
// accounts in one shot but has no JSON endpoint, so the RSC parser plus
// local cache is the only way to make these views agent-friendly.

type rosterAuthorRow struct {
	Username           string             `json:"username"`
	DisplayName        string             `json:"displayName,omitempty"`
	XID                string             `json:"xId,omitempty"`
	Rank               int                `json:"rank"`
	PreviousRank       *int               `json:"previousRank"`
	RankChange         *int               `json:"rankChange"`
	Score              float64            `json:"score,omitempty"`
	Category           string             `json:"category,omitempty"`
	CategoryRank       int                `json:"categoryRank,omitempty"`
	CategoryConfidence float64            `json:"categoryConfidence,omitempty"`
	FollowersCount     int                `json:"followersCount,omitempty"`
	FollowedByCount    int                `json:"followedByCount,omitempty"`
	Bio                string             `json:"bio,omitempty"`
	GithubURL          string             `json:"githubUrl,omitempty"`
	ProfileImageURL    string             `json:"profileImageUrl,omitempty"`
	VibeDistribution   map[string]float64 `json:"vibeDistribution,omitempty"`
	VibeTweetCount     int                `json:"vibeTweetCount,omitempty"`
	XURL               string             `json:"xUrl,omitempty"`
	LastSeenAt         string             `json:"lastSeenAt,omitempty"`
}

type rosterEnvelope struct {
	Meta    map[string]any    `json:"meta"`
	Results []rosterAuthorRow `json:"results"`
}

func newAuthorsListCmd(flags *rootFlags) *cobra.Command {
	var by string
	var limit int
	var category string
	var onlyNew bool
	var onlyFallers bool
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "Full ranked roster of Digg-tracked accounts with rich fields (rank, category, bio, vibeDistribution); fetches /ai/1000 by default",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `List the full roster of Digg-tracked accounts with per-author rank, category, bio, GitHub URL, and vibe distribution.

Live by default: fetches /ai/1000, parses the embedded RSC payload, upserts
the roster into the local store, then reads back. Pass --data-source local
to skip the network fetch and use only what's already cached.

Sortable by:
  rank          ascending (default)
  rankChange    biggest movers first (positive=climbed, negative=fell)
  category      grouped by category, then categoryRank ascending
  followers     followers_count descending`,
		Example: `  digg-pp-cli authors list --limit 20
  digg-pp-cli authors list --by rankChange --limit 10 --json
  digg-pp-cli authors list --only-new --json
  digg-pp-cli authors list --category "AI Safety" --json
  digg-pp-cli authors list --data-source local --limit 1000 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			_, db, closeFn, err := openStore(ctx)
			if err != nil {
				return err
			}
			defer closeFn()

			source := "live"
			ds := flags.dataSource
			if ds == "local" {
				source = "local"
			} else {
				// Live fetch + upsert. Route through the shared Client
				// so the request honors --rate-limit / --timeout and
				// shares the impersonated transport with the rest of
				// the digg CLI's page fetchers (FetchClusterPosts,
				// SearchUsers, FetchUserPeerFollowCount).
				// Bypassing the Client here used to skip the limiter,
				// which is dangerous for a 200KB+ /ai/1000 page.
				c, cerr := flags.newClient()
				if cerr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: building client for /ai/1000 fetch failed (%v); falling back to local cache\n", cerr)
					source = "local"
				} else {
					var (
						authors []diggparse.Roster1000Author
						ferr    error
					)
					if roster1000URLOverride != "" {
						authors, ferr = c.FetchRoster1000From(ctx, roster1000URLOverride)
					} else {
						authors, ferr = c.FetchRoster1000(ctx)
					}
					if ferr != nil && len(authors) == 0 {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: live /ai/1000 fetch failed (%v); falling back to local cache\n", ferr)
						source = "local"
					} else {
						if ferr != nil {
							// Partial parse — keep going.
							fmt.Fprintf(cmd.ErrOrStderr(), "warning: /ai/1000 partial parse: %v\n", ferr)
						}
						if _, err := diggstore.UpsertRoster1000(db, authors, time.Now().UTC()); err != nil {
							return fmt.Errorf("persisting roster: %w", err)
						}
					}
				}
			}

			// When --category is set and the user didn't override --by,
			// sort by category_rank within the filter — that's what the
			// upstream "category leaderboard" view shows on di.gg/ai/1000.
			effectiveBy := by
			if category != "" && (by == "" || by == "rank") {
				effectiveBy = "category"
			}
			orderBy, err := rosterOrderBy(effectiveBy)
			if err != nil {
				return err
			}

			var (
				where  []string
				params []any
			)
			where = append(where, "rank > 0")
			if onlyNew {
				where = append(where, "previous_rank IS NULL")
			}
			if onlyFallers {
				where = append(where, "rank_change IS NOT NULL AND rank_change < 0")
			}
			if category != "" {
				where = append(where, "category = ?")
				params = append(params, category)
			}

			query := `SELECT
				username, COALESCE(display_name,''), COALESCE(x_id,''),
				COALESCE(rank,0), previous_rank, rank_change, COALESCE(score,0),
				COALESCE(category,''), COALESCE(category_rank,0), COALESCE(category_confidence,0),
				COALESCE(followers_count,0), COALESCE(followed_by_count,0),
				COALESCE(bio,''), COALESCE(github_url,''), COALESCE(profile_image_url,''),
				COALESCE(vibe_distribution_json,''), COALESCE(vibe_tweet_count,0),
				COALESCE(last_seen_at,'')
				FROM digg_authors WHERE ` + strings.Join(where, " AND ") + ` ORDER BY ` + orderBy + ` LIMIT ?`
			params = append(params, limit)

			rows, err := db.QueryContext(ctx, query, params...)
			if err != nil {
				return fmt.Errorf("querying digg_authors: %w", err)
			}
			defer rows.Close()
			var out []rosterAuthorRow
			for rows.Next() {
				var r rosterAuthorRow
				var prevRank, rankChange sql.NullInt64
				var vibeJSON string
				if err := rows.Scan(
					&r.Username, &r.DisplayName, &r.XID,
					&r.Rank, &prevRank, &rankChange, &r.Score,
					&r.Category, &r.CategoryRank, &r.CategoryConfidence,
					&r.FollowersCount, &r.FollowedByCount,
					&r.Bio, &r.GithubURL, &r.ProfileImageURL,
					&vibeJSON, &r.VibeTweetCount,
					&r.LastSeenAt,
				); err != nil {
					return err
				}
				if prevRank.Valid {
					v := int(prevRank.Int64)
					r.PreviousRank = &v
				}
				if rankChange.Valid {
					v := int(rankChange.Int64)
					r.RankChange = &v
				}
				if vibeJSON != "" {
					var m map[string]float64
					if err := json.Unmarshal([]byte(vibeJSON), &m); err == nil {
						r.VibeDistribution = m
					}
				}
				if r.Username != "" {
					r.XURL = "https://x.com/" + r.Username
				}
				out = append(out, r)
			}
			if err := rows.Err(); err != nil {
				return err
			}

			if len(out) == 0 {
				if source == "local" {
					return emptyHint(cmd, "no authors in local store. Run `digg-pp-cli authors list` (without --data-source local) to ingest /ai/1000 first.")
				}
				return emptyHint(cmd, "no authors after applying filters. Try widening --category or dropping --only-new/--only-fallers.")
			}

			env := rosterEnvelope{
				Meta: map[string]any{
					"source": source,
					"by":     effectiveBy,
					"count":  len(out),
				},
				Results: out,
			}
			if onlyNew {
				env.Meta["onlyNew"] = true
			}
			if onlyFallers {
				env.Meta["onlyFallers"] = true
			}
			if category != "" {
				env.Meta["category"] = category
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), env, flags)
			}
			// Human-readable table.
			for i, a := range out {
				dn := firstNonEmpty(a.DisplayName, a.Username)
				prev := "-"
				if a.PreviousRank != nil {
					prev = fmt.Sprintf("%d", *a.PreviousRank)
				}
				change := ""
				if a.RankChange != nil {
					change = fmt.Sprintf(" (%+d)", *a.RankChange)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%4d. @%-22s %s  prev=%s%s  cat=%s\n",
					i+1, a.Username, diggTruncate(dn, 28), prev, change, a.Category)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&by, "by", "rank", "Sort by: rank | rankChange | category | followers")
	cmd.Flags().IntVar(&limit, "limit", 1000, "Max number of authors to return")
	cmd.Flags().StringVar(&category, "category", "", "Filter to authors with this category (e.g. \"AI Safety\")")
	cmd.Flags().BoolVar(&onlyNew, "only-new", false, "Only authors with previous_rank IS NULL (newly-listed)")
	cmd.Flags().BoolVar(&onlyFallers, "only-fallers", false, "Only authors with rank_change < 0")
	return cmd
}

// rosterOrderBy maps the --by flag to a SQL ORDER BY clause.
// Trailing tiebreaker on `rank ASC` keeps output stable.
func rosterOrderBy(by string) (string, error) {
	switch by {
	case "", "rank":
		return "rank ASC", nil
	case "rankChange", "rank_change":
		// Largest absolute movers first (positive climbs OR negative falls).
		// Nulls sort last so newly-listed accounts don't dominate when the
		// caller asked for movement.
		return "CASE WHEN rank_change IS NULL THEN 1 ELSE 0 END ASC, ABS(rank_change) DESC, rank ASC", nil
	case "category":
		return "category ASC, category_rank ASC, rank ASC", nil
	case "followers":
		return "followers_count DESC, rank ASC", nil
	default:
		return "", fmt.Errorf("--by must be one of: rank, rankChange, category, followers")
	}
}

// ============== authors get ==============
//
// `authors get <handle>` is the per-handle lookup surface backed by
// /api/search/users. It is a sibling of `authors top` (today's
// leaderboard from the local store) and `authors list` (full
// /ai/1000 ingest). `authors get` is *live-only* by design: the
// upstream endpoint is the authority on whether a handle is in the
// 1000, and caching that answer would let stale tier_status leak
// into the output.
//
// The off-1000 path adds the rank-1000 author as a comparison anchor,
// reading the rank-1000 row from the local digg_authors cache (populated by
// `authors list`). On cache miss the command does a one-shot live
// /ai/1000 fetch + upsert (the same call `authors list` makes), then
// re-queries. If both the cache is empty AND the live fetch fails,
// the command still returns a useful payload — just without the
// nearest_in_1000 field, with meta.tier_status_resolved set to false
// so callers can detect the partial result.

// searchUsersURLOverride lets tests redirect the live user-search
// URL to a local httptest server. Empty string falls through to the
// default "https://digg.com/api/search/users". Production code never
// sets this.
var searchUsersURLOverride string

// roster1000URLOverride lets tests redirect the live /ai/1000 fetch
// used on the off-1000 cache-cold fallback path to a local httptest
// server. Empty string falls through to the default
// "https://digg.com/tech/x/rankings". Production code never sets this.
var roster1000URLOverride string

// authorGetResult is the per-author payload returned by
// `authors get`. Field naming mirrors UsersSearchResult upstream
// where it makes sense, with CLI-side additions:
//
//   - XURL: minted from username
//   - GithubURL: looked up from local digg_authors cache when present
//     (the search endpoint doesn't return it; /ai/1000 does)
//   - TierStatus: "in_1000" when CurrentRank != nil, else "off_1000"
//   - NearestIn1000: present only on off-1000 path, and only when
//     the rank-1000 anchor was resolvable (cache or live fetch).
//   - MatchType: "exact" or "fuzzy"; advertised separately at the
//     envelope level (Meta.match_type) too — caller's choice which to
//     read.
//
// `followers_count` (total X followers) is preserved verbatim from
// upstream for context — useful to know who the user is — but is
// no longer used as the comparison metric.
//
// CurrentRank stays a pointer so the JSON output can carry an explicit
// `null` for off-1000 handles (matches upstream shape; downstream
// jq pipelines that test `current_rank == null` keep working).
type authorGetResult struct {
	XID             string             `json:"x_id,omitempty"`
	Username        string             `json:"username"`
	DisplayName     string             `json:"display_name,omitempty"`
	ProfileImageURL string             `json:"profile_image_url,omitempty"`
	FollowersCount  int                `json:"followers_count"`
	Category        *string            `json:"category"`
	CurrentRank     *int               `json:"current_rank"`
	SimilarityScore float64            `json:"similarity_score"`
	IsPrefixMatch   bool               `json:"is_prefix_match"`
	XURL            string             `json:"xUrl,omitempty"`
	GithubURL       string             `json:"githubUrl,omitempty"`
	TierStatus      string             `json:"tier_status,omitempty"`
	NearestIn1000   *nearestIn1000Anch `json:"nearest_in_1000,omitempty"`
	MatchType       string             `json:"match_type,omitempty"`
}

// nearestIn1000Anch is the rank-1000 author's stats, used as a
// comparison anchor for off-1000 handles. The anchor is rank=1000 by
// definition (the cutoff into the 1000), but we still emit the field
// so callers don't have to hard-code the constant.
//
// `peer_follow_count` is the rank-1000 author's `followed_by_count`
// (how many of the AI 1000 follow them). This is the metric the AI
// 1000 is actually ranked by; `followers_count` (total X followers)
// is surfaced for context but is NOT the ranking signal.
type nearestIn1000Anch struct {
	Username        string  `json:"username"`
	Rank            int     `json:"rank"`
	FollowersCount  int     `json:"followers_count"`
	PeerFollowCount int     `json:"peer_follow_count"`
	Score           float64 `json:"score,omitempty"`
}

// authorGetEnvelopeExact is the JSON shape for an exact-match query.
// `result` is a single object so callers don't have to index into an
// array for the common case. Fuzzy matches use authorGetEnvelopeFuzzy
// (results array) — the two shapes are intentionally distinct so a
// jq pipeline can `.result // .results[0]` unambiguously.
type authorGetEnvelopeExact struct {
	Meta   map[string]any  `json:"meta"`
	Result authorGetResult `json:"result"`
}

type authorGetEnvelopeFuzzy struct {
	Meta    map[string]any    `json:"meta"`
	Results []authorGetResult `json:"results"`
}

func newAuthorsGetCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "get <handle>",
		Short:       "Look up a handle on /api/search/users; off-1000 results include the rank-1000 author for comparison",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Look up an X handle in Digg's full author universe (1000 + off-1000).

Live by default — hits /api/search/users, the same vector+prefix
search that backs the di.gg/ai handle picker. Returns a single rich
record on exact match, or an array of fuzzy candidates sorted by
similarity_score when the handle has no exact hit.

For off-1000 handles (current_rank: null), the response also
includes the rank-1000 author's stats as a comparison anchor
(nearest_in_1000). The anchor is read from the local digg_authors
cache (populated by ` + "`digg-pp-cli authors list`" + `); on cache
miss the command does a one-shot live roster fetch and populates
the cache before returning. If the anchor doesn't resolve, the
rest of the record is returned with
meta.tier_status_resolved: false.`,
		Example: `  digg-pp-cli authors get logangraham
  digg-pp-cli authors get mvanhorn --json
  digg-pp-cli authors get LoganGraham --agent
  digg-pp-cli authors get logan --limit 10 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			handle := strings.TrimSpace(args[0])
			if handle == "" {
				return usageErr(fmt.Errorf("handle is required (e.g. `digg-pp-cli authors get logangraham`)"))
			}
			ctx := cmd.Context()

			// Live call to /api/search/users.
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			var resp *client.UsersSearchResponse
			if searchUsersURLOverride != "" {
				resp, err = c.SearchUsersFrom(ctx, searchUsersURLOverride, handle, limit)
			} else {
				resp, err = c.SearchUsers(ctx, handle, limit)
			}
			if err != nil {
				return fmt.Errorf("live /api/search/users failed: %w", err)
			}

			// Pick the exact case-insensitive username match if any;
			// fall back to the upstream-sorted fuzzy list otherwise.
			lowerHandle := strings.ToLower(handle)
			var exact *client.UsersSearchResult
			for i := range resp.Results {
				if strings.ToLower(resp.Results[i].Username) == lowerHandle {
					exact = &resp.Results[i]
					break
				}
			}

			meta := map[string]any{
				"source": "live",
				"query":  handle,
			}

			// Hoist all per-candidate-shared work (DB open, rank-1000
			// anchor with live fallback) out of the fuzzy loop so a
			// 5-candidate fuzzy fallback pays for one of each rather
			// than five.
			env := computeAuthorGetEnv(ctx, cmd, flags)
			defer env.closeFn()

			if exact != nil {
				record, resolved := buildAuthorGetResult(ctx, env, *exact, "exact")
				meta["match_type"] = "exact"
				meta["count"] = 1
				meta["tier_status_resolved"] = resolved
				if flags.asJSON {
					out := authorGetEnvelopeExact{Meta: meta, Result: record}
					return printJSONFiltered(cmd.OutOrStdout(), out, flags)
				}
				printAuthorGetHuman(cmd, record)
				return nil
			}

			// No exact match — emit fuzzy envelope, possibly empty.
			// upstream already sorts by similarity_score desc, but we
			// re-sort defensively in case a future probe shows otherwise.
			fuzzy := make([]authorGetResult, 0, len(resp.Results))
			anyUnresolved := false
			for _, r := range resp.Results {
				record, resolved := buildAuthorGetResult(ctx, env, r, "fuzzy")
				if !resolved && record.CurrentRank == nil {
					anyUnresolved = true
				}
				fuzzy = append(fuzzy, record)
			}
			sort.SliceStable(fuzzy, func(i, j int) bool {
				return fuzzy[i].SimilarityScore > fuzzy[j].SimilarityScore
			})
			if limit > 0 && len(fuzzy) > limit {
				fuzzy = fuzzy[:limit]
			}

			meta["match_type"] = "fuzzy"
			meta["count"] = len(fuzzy)
			// For fuzzy lists tier_status_resolved means "every off-1000
			// row in the list got its anchor". A single unresolved row
			// flips it to false so callers can see the partial result.
			meta["tier_status_resolved"] = !anyUnresolved

			if flags.asJSON {
				out := authorGetEnvelopeFuzzy{Meta: meta, Results: fuzzy}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(fuzzy) == 0 {
				return emptyHint(cmd, fmt.Sprintf("no matches for %q on /api/search/users.", handle))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "no exact match for %q; showing %d fuzzy result(s):\n", handle, len(fuzzy))
			for _, r := range fuzzy {
				printAuthorGetHuman(cmd, r)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 5, "Cap fuzzy results to this many candidates (exact-match path always returns one record)")
	return cmd
}

// authorGetEnv carries the per-invocation shared state used by
// buildAuthorGetResult so a fuzzy-fallback list of N candidates only
// pays for one DB open and one rank-1000 anchor lookup (with optional
// live fallback), not N of each.
//
// `anchor` is nil when the rank-1000 anchor couldn't be resolved
// (cache cold + live fallback failure). `anchorOK` is the matching
// resolved bit.
//
// `db` is owned by the caller (computeAuthorGetEnv's caller is
// responsible for closing it via the returned closeFn).
type authorGetEnv struct {
	db       *sql.DB
	closeFn  func() error
	anchor   *nearestIn1000Anch
	anchorOK bool
}

// computeAuthorGetEnv opens the shared store and resolves the
// rank-1000 anchor once (with live fallback when allowed). The
// returned env's `closeFn` MUST be called by the caller once the
// per-candidate loop is done.
//
// Failures degrade gracefully:
//
//   - DB open failure → env.db is nil; per-candidate DB lookups skip
//     silently. The closeFn is still safe to call.
//   - Anchor resolution failure (cache miss + live fetch failure) →
//     env.anchor is nil and env.anchorOK is false. Per-candidate
//     off-1000 paths will surface partial data without nearest_in_1000.
func computeAuthorGetEnv(ctx context.Context, cmd *cobra.Command, flags *rootFlags) authorGetEnv {
	env := authorGetEnv{closeFn: func() error { return nil }}
	if _, db, closeFn, err := openStore(ctx); err == nil {
		env.db = db
		env.closeFn = closeFn
	} else {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: opening local store for authors get failed: %v\n", err)
	}
	if env.db != nil {
		env.anchor, env.anchorOK = loadRank1000AnchorForDB(ctx, cmd, env.db, true /* allowLiveFallback */, flags)
	}
	return env
}

// buildAuthorGetResult promotes one UsersSearchResult into the CLI's
// authorGetResult shape. Mints xUrl, computes tier_status, and (for
// off-1000) attaches the rank-1000 anchor. Returns the result and a
// `tierResolved` bool: true when in_1000, or when off_1000 and the
// anchor resolved. Callers surface that bit on
// meta.tier_status_resolved.
//
// `env` carries shared state hoisted out of the fuzzy-candidate loop
// so calling this for N candidates pays only one DB open and one
// anchor lookup across the batch.
func buildAuthorGetResult(ctx context.Context, env authorGetEnv, src client.UsersSearchResult, matchType string) (authorGetResult, bool) {
	out := authorGetResult{
		XID:             src.XID,
		Username:        src.Username,
		DisplayName:     src.DisplayName,
		ProfileImageURL: src.ProfileImageURL,
		FollowersCount:  src.FollowersCount,
		Category:        src.Category,
		CurrentRank:     src.CurrentRank,
		SimilarityScore: src.SimilarityScore,
		IsPrefixMatch:   src.IsPrefixMatch,
		MatchType:       matchType,
	}
	if src.Username != "" {
		out.XURL = "https://x.com/" + src.Username
	}
	// The search endpoint doesn't return githubUrl but the roster
	// ingest does. A missing githubUrl is normal, not a partial result.
	if gh := lookupGithubURLForDB(ctx, env.db, src.Username); gh != "" {
		out.GithubURL = gh
	}
	if src.CurrentRank != nil {
		out.TierStatus = "in_1000"
		return out, true
	}
	out.TierStatus = "off_1000"
	if env.anchorOK {
		out.NearestIn1000 = env.anchor
	}
	return out, env.anchorOK
}

// lookupGithubURLForDB checks the digg_authors row for a stored
// github_url for the given username, using a caller-supplied SQLite
// handle. Best-effort: returns "" when the DB handle is nil, the
// user isn't cached, or the column is empty. Never panics, never
// blocks the live flow on a DB error.
//
// Sharing the DB handle across candidates avoids opening one SQLite
// connection per fuzzy-fallback row.
func lookupGithubURLForDB(ctx context.Context, db *sql.DB, username string) string {
	if username == "" || db == nil {
		return ""
	}
	var gh sql.NullString
	row := db.QueryRowContext(ctx,
		`SELECT github_url FROM digg_authors WHERE LOWER(username) = LOWER(?) LIMIT 1`, username)
	if err := row.Scan(&gh); err != nil {
		return ""
	}
	if !gh.Valid {
		return ""
	}
	return gh.String
}

// loadRank1000AnchorForDB returns the rank-1000 author using the
// caller-supplied SQLite handle. When `allowLiveFallback` is true and
// the cache has no rank=1000 row, it does a one-shot live /ai/1000
// fetch + upsert + re-query (the same path `authors list` would
// take), routed through the shared *client.Client so the request
// honors --rate-limit / --timeout.
//
// Returns (anchor, true) on success; (nil, false) when neither
// source resolved a row. Logs the live fetch failure to stderr so
// operators can debug without breaking the rest of the response.
//
// Hoisted out of the per-candidate loop: a fuzzy fallback with N
// candidates pays for at most one anchor lookup (and at most one
// live fetch) instead of N.
func loadRank1000AnchorForDB(ctx context.Context, cmd *cobra.Command, db *sql.DB, allowLiveFallback bool, flags *rootFlags) (*nearestIn1000Anch, bool) {
	if db == nil {
		return nil, false
	}
	if anchor, ok := readRank1000FromDB(ctx, db); ok {
		return anchor, true
	}
	if !allowLiveFallback {
		return nil, false
	}

	// Cache cold: do the same fetch + upsert `authors list` does,
	// through the shared Client so the request goes through the
	// rate limiter and impersonated transport.
	c, err := flags.newClient()
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: building client for /ai/1000 anchor fetch failed: %v\n", err)
		return nil, false
	}
	var (
		authors []diggparse.Roster1000Author
		perr    error
	)
	if roster1000URLOverride != "" {
		authors, perr = c.FetchRoster1000From(ctx, roster1000URLOverride)
	} else {
		authors, perr = c.FetchRoster1000(ctx)
	}
	if perr != nil && len(authors) == 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: live /ai/1000 fetch for rank-1000 anchor failed: %v\n", perr)
		return nil, false
	}
	if perr != nil {
		// Partial parse — keep going.
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: /ai/1000 partial parse for rank-1000 anchor: %v\n", perr)
	}
	if _, err := diggstore.UpsertRoster1000(db, authors, time.Now().UTC()); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: upserting /ai/1000 roster for rank-1000 anchor failed: %v\n", err)
		return nil, false
	}
	return readRank1000FromDB(ctx, db)
}

// readRank1000FromDB pulls just the rank-1000 row's anchor fields.
// Returns (nil, false) when no row exists with rank=1000 — that's
// the signal to the caller to attempt a live fallback. Errors other
// than sql.ErrNoRows are also treated as "not present" so a flaky
// SQLite issue doesn't crash the command.
//
// `followed_by_count` is the metric the AI 1000 is ranked by (how
// many of the AI 1000 follow this user); it's surfaced as
// `peer_follow_count` on the output anchor.
func readRank1000FromDB(ctx context.Context, db *sql.DB) (*nearestIn1000Anch, bool) {
	var (
		username        sql.NullString
		followersCount  sql.NullInt64
		peerFollowCount sql.NullInt64
		score           sql.NullFloat64
	)
	row := db.QueryRowContext(ctx, `SELECT username, COALESCE(followers_count,0), COALESCE(followed_by_count,0), COALESCE(score,0)
		FROM digg_authors WHERE rank = 1000 LIMIT 1`)
	if err := row.Scan(&username, &followersCount, &peerFollowCount, &score); err != nil {
		return nil, false
	}
	if !username.Valid || username.String == "" {
		return nil, false
	}
	return &nearestIn1000Anch{
		Username:        username.String,
		Rank:            1000,
		FollowersCount:  int(followersCount.Int64),
		PeerFollowCount: int(peerFollowCount.Int64),
		Score:           score.Float64,
	}, true
}

// printAuthorGetHuman renders one record in the table-y default
// (non-JSON) shape. Mirrors the rest of the digg CLI: leading rank
// or "off-1000" tag, then handle, display name, followers,
// category, and (when off-1000) the rank-1000 anchor.
// Keep it dense so `digg-pp-cli authors get foo` reads at a glance.
func printAuthorGetHuman(cmd *cobra.Command, r authorGetResult) {
	tierTag := "off-1000"
	if r.CurrentRank != nil {
		tierTag = fmt.Sprintf("#%d", *r.CurrentRank)
	}
	cat := ""
	if r.Category != nil && *r.Category != "" {
		cat = "  cat=" + *r.Category
	}
	dn := firstNonEmpty(r.DisplayName, r.Username)
	fmt.Fprintf(cmd.OutOrStdout(), "%-9s @%-22s %s  followers=%d%s\n",
		tierTag, r.Username, diggTruncate(dn, 28), r.FollowersCount, cat)
	if r.NearestIn1000 != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "    nearest_in_1000=@%s (rank %d, peer_follows=%d)\n",
			r.NearestIn1000.Username, r.NearestIn1000.Rank, r.NearestIn1000.PeerFollowCount)
	}
	if r.XURL != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "    %s\n", r.XURL)
	}
}

// ============== author ==============

func newAuthorCmd(flags *rootFlags) *cobra.Command {
	var sinceStr string
	cmd := &cobra.Command{
		Use:         "author [username]",
		Short:       "Show every cluster a given X account contributed to",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  digg-pp-cli author Scobleizer
  digg-pp-cli author GaryMarcus --since 7d --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			username := strings.TrimPrefix(args[0], "@")
			since := parseSinceWithFallback(sinceStr, 30*24*time.Hour)
			_, db, closeFn, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer closeFn()
			rows, err := db.QueryContext(cmd.Context(),
				`SELECT c.cluster_id, c.cluster_url_id, COALESCE(c.label,''), COALESCE(c.current_rank,0),
				 COALESCE(c.activity_at,''), COALESCE(ca.post_type,''), COALESCE(ca.post_permalink,'')
				 FROM digg_cluster_authors ca
				 JOIN digg_clusters c ON c.cluster_id = ca.cluster_id
				 WHERE ca.username = ? AND COALESCE(c.activity_at, c.last_seen_at) >= ?
				 ORDER BY COALESCE(c.activity_at, c.last_seen_at) DESC`,
				username, since.UTC().Format(time.RFC3339Nano))
			if err != nil {
				return err
			}
			defer rows.Close()
			type contribRow struct {
				ClusterID     string `json:"clusterId"`
				ClusterURLID  string `json:"clusterUrlId"`
				Label         string `json:"label"`
				CurrentRank   int    `json:"currentRank"`
				ActivityAt    string `json:"activityAt,omitempty"`
				PostType      string `json:"postType,omitempty"`
				PostPermalink string `json:"postPermalink,omitempty"`
			}
			var out []contribRow
			for rows.Next() {
				var r contribRow
				if err := rows.Scan(&r.ClusterID, &r.ClusterURLID, &r.Label, &r.CurrentRank, &r.ActivityAt, &r.PostType, &r.PostPermalink); err != nil {
					return err
				}
				out = append(out, r)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			if len(out) == 0 {
				return emptyHint(cmd, fmt.Sprintf("no contributions seen for @%s in the window. Try `--since 30d` or run `digg-pp-cli sync` first.", username))
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "@%s contributed to %d clusters\n", username, len(out))
			for _, r := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "  #%-3d %s [%s] (%s)\n", r.CurrentRank, diggTruncate(r.Label, 80), r.ClusterURLID, r.PostType)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&sinceStr, "since", "30d", "Lookback window")
	return cmd
}

// ============== watch ==============

func newWatchCmd(flags *rootFlags) *cobra.Command {
	var interval time.Duration
	var minDelta int
	var iterations int
	var alertExpr string
	var topic string
	cmd := &cobra.Command{
		Use:         "watch",
		Short:       "Poll one Digg topic on an interval and alert when any cluster moves N+ ranks",
		Long:        "Polls a Digg topic page (--topic, default ai), parses the feed, diffs against the previous local snapshot, and prints any cluster whose absolute rank delta is at-or-above --min-delta (or matches --alert). READ-ONLY: never writes anything to Digg.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  digg-pp-cli watch --interval 60s --min-delta 5
  digg-pp-cli watch --alert 'rank.delta>=10'
  digg-pp-cli watch --topic sports --interval 2m
  digg-pp-cli watch --interval 30s --iterations 3   # for verify`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateTopics([]string{topic}); err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			_, db, closeFn, err := openStore(ctx)
			if err != nil {
				return err
			}
			defer closeFn()
			it := 0
			for {
				it++
				html, err := fetchURL(ctx, diggSiteURL+topicPage(topic))
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "watch: fetch error: %v\n", err)
				} else {
					clusters, _, _, err := diggparse.ParseHomeFeed(html)
					if err == nil {
						alerts := computeWatchAlerts(db, clusters, minDelta)
						now := time.Now().UTC().Format(time.RFC3339)
						if len(alerts) == 0 {
							fmt.Fprintf(cmd.OutOrStdout(), "[%s] watch: %d clusters, no movers >= %d\n", now, len(clusters), minDelta)
						} else {
							fmt.Fprintf(cmd.OutOrStdout(), "[%s] watch: %d alerts\n", now, len(alerts))
							for _, a := range alerts {
								fmt.Fprintf(cmd.OutOrStdout(), "  %+d  %s [%s]\n", a.Delta, a.Label, a.ClusterURLID)
							}
						}
						// Persist snapshots so future polls have history
						for _, c := range clusters {
							_ = diggstore.UpsertCluster(db, c, time.Now())
						}
					}
				}
				if iterations > 0 && it >= iterations {
					return nil
				}
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(interval):
				}
			}
		},
	}
	cmd.Flags().DurationVar(&interval, "interval", 60*time.Second, "Poll interval (e.g. 30s, 2m)")
	cmd.Flags().IntVar(&minDelta, "min-delta", 5, "Minimum |rank delta| to alert on")
	cmd.Flags().IntVar(&iterations, "iterations", 0, "Stop after N iterations (0 = run until interrupted)")
	cmd.Flags().StringVar(&topic, "topic", "ai", "Digg topic to poll (valid: "+strings.Join(validTopicSlugs(), ", ")+")")
	cmd.Flags().StringVar(&alertExpr, "alert", "", "Shorthand for --min-delta: 'rank.delta>=N' sets --min-delta to N")
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if alertExpr != "" {
			// Accept the shorthand form: "rank.delta>=N" or "rank.delta>N"
			parts := strings.Split(alertExpr, ">=")
			if len(parts) != 2 {
				parts = strings.Split(alertExpr, ">")
			}
			if len(parts) == 2 {
				if n, err := fmt.Sscanf(parts[1], "%d", &minDelta); err == nil && n == 1 {
					return nil
				}
			}
			return fmt.Errorf("--alert format must be rank.delta>=N (got %q)", alertExpr)
		}
		return nil
	}
	return cmd
}

type watchAlert struct {
	ClusterID    string
	ClusterURLID string
	Label        string
	Delta        int
}

func computeWatchAlerts(db *sql.DB, current []diggparse.Cluster, minDelta int) []watchAlert {
	prev := make(map[string]int)
	rows, err := db.Query(`SELECT cluster_id, COALESCE(current_rank,0) FROM digg_clusters
		WHERE last_seen_at = (SELECT MAX(last_seen_at) FROM digg_clusters)`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var rank int
		if err := rows.Scan(&id, &rank); err == nil {
			prev[id] = rank
		}
	}
	var out []watchAlert
	for _, c := range current {
		oldRank, hadPrev := prev[c.ClusterID]
		if !hadPrev || c.CurrentRank == 0 {
			continue
		}
		delta := oldRank - c.CurrentRank // climbing → positive
		if abs(delta) < minDelta {
			continue
		}
		out = append(out, watchAlert{
			ClusterID:    c.ClusterID,
			ClusterURLID: c.ClusterURLID,
			Label:        c.Label,
			Delta:        delta,
		})
	}
	sort.Slice(out, func(i, j int) bool { return abs(out[i].Delta) > abs(out[j].Delta) })
	return out
}

// ============== open ==============

func newOpenCmd(flags *rootFlags) *cobra.Command {
	var launch bool
	cmd := &cobra.Command{
		Use:   "open [story]",
		Short: "Print or open the Digg URL for one story in any topic",
		Long: `Prints the digg.com URL for the given story, under the topic the story is
filed in. By default does NOT launch a browser; pass --launch to actually
open. Per the printing-press side-effect convention.

The argument is the story's UUID (clusterId in search results), the short id
from its URL (clusterUrlId), or the URL. A short id or URL works once
` + "`search`" + ` has returned that story.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  # Print the URL only (default; safe in scripts)
  digg-pp-cli open iq7usf9e

  # Actually launch the browser
  digg-pp-cli open iq7usf9e --launch`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			ref, err := resolveStory(ctx, args[0])
			if err != nil {
				return err
			}
			perm := ref.URL
			if perm == "" {
				// Only the UUID is known; Digg has the story's URL.
				mc, err := newDiggMCPClient(flags)
				if err != nil {
					return err
				}
				page, err := fetchStory(ctx, mc, ref.ID, 1)
				if err != nil {
					return diggCallErr(err)
				}
				perm = page.Story.URL
			}
			// Verify-environment short-circuit (printing-press side-effect convention).
			if isVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would launch: %s\n", perm)
				return nil
			}
			if !launch {
				fmt.Fprintf(cmd.OutOrStdout(), "would launch: %s\n", perm)
				fmt.Fprintf(cmd.OutOrStdout(), "(re-run with --launch to actually open in your browser)\n")
				return nil
			}
			return launchBrowser(perm)
		},
	}
	cmd.Flags().BoolVar(&launch, "launch", false, "Actually open the URL in the default browser")
	return cmd
}

// ============== stats ==============

func newStatsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "stats",
		Short:       "Show local store statistics: cluster count, author count, snapshot history depth",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  digg-pp-cli stats
  digg-pp-cli stats --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			_, db, closeFn, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer closeFn()
			var s struct {
				Clusters     int    `json:"clusters"`
				Authors      int    `json:"authors"`
				Snapshots    int    `json:"snapshots"`
				Events       int    `json:"events"`
				Replacements int    `json:"replacements"`
				LastSync     string `json:"lastSync,omitempty"`
			}
			_ = db.QueryRowContext(cmd.Context(), `SELECT COUNT(*) FROM digg_clusters`).Scan(&s.Clusters)
			_ = db.QueryRowContext(cmd.Context(), `SELECT COUNT(*) FROM digg_authors`).Scan(&s.Authors)
			_ = db.QueryRowContext(cmd.Context(), `SELECT COUNT(*) FROM digg_snapshots`).Scan(&s.Snapshots)
			_ = db.QueryRowContext(cmd.Context(), `SELECT COUNT(*) FROM digg_events`).Scan(&s.Events)
			_ = db.QueryRowContext(cmd.Context(), `SELECT COUNT(*) FROM digg_replacements`).Scan(&s.Replacements)
			_ = db.QueryRowContext(cmd.Context(), `SELECT MAX(last_seen_at) FROM digg_clusters`).Scan(&s.LastSync)
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), s, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Local store: %d clusters, %d authors, %d snapshots, %d events, %d replacements\n",
				s.Clusters, s.Authors, s.Snapshots, s.Events, s.Replacements)
			if s.LastSync != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  last sync: %s\n", s.LastSync)
			}
			return nil
		},
	}
	return cmd
}

// ============== shared helpers ==============

func renderClusterTable(w io.Writer, rows []clusterRow) error {
	for _, c := range rows {
		extra := ""
		if c.Delta != 0 {
			extra = fmt.Sprintf("  delta=%+d", c.Delta)
		}
		display := firstNonEmpty(c.Label, c.Title, "(no label)")
		fmt.Fprintf(w, "#%-3d %s [%s]%s\n", c.CurrentRank, diggTruncate(display, 100), c.ClusterURLID, extra)
		if c.TLDR != "" {
			fmt.Fprintf(w, "    %s\n", diggTruncate(c.TLDR, 200))
		}
	}
	return nil
}

func printClusterOutput(cmd *cobra.Command, flags *rootFlags, rows []clusterRow, render func(io.Writer, []clusterRow) error) error {
	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
	}
	return render(cmd.OutOrStdout(), rows)
}

func emptyHint(cmd *cobra.Command, hint string) error {
	fmt.Fprintln(cmd.OutOrStdout(), hint)
	return nil
}

func diggTruncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func firstNonEmpty(xs ...string) string {
	for _, x := range xs {
		if x != "" {
			return x
		}
	}
	return ""
}

func indentJSON(s string, indent int) string {
	if s == "" {
		return ""
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return s
	}
	pad := strings.Repeat(" ", indent)
	pretty, err := json.MarshalIndent(v, pad, "  ")
	if err != nil {
		return s
	}
	return string(pretty)
}

func parseSinceWithFallback(s string, fallback time.Duration) time.Time {
	if s == "" {
		return time.Now().Add(-fallback)
	}
	if d, err := time.ParseDuration(s); err == nil {
		return time.Now().Add(-d)
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	// Custom day suffix
	if strings.HasSuffix(s, "d") {
		var days int
		if _, err := fmt.Sscanf(s, "%dd", &days); err == nil {
			return time.Now().Add(-time.Duration(days) * 24 * time.Hour)
		}
	}
	return time.Now().Add(-fallback)
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func isVerifyEnv() bool {
	return os.Getenv("PRINTING_PRESS_VERIFY") == "1" || os.Getenv("PRINTING_PRESS_VERIFY") == "true"
}

func launchBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	return cmd.Start()
}
