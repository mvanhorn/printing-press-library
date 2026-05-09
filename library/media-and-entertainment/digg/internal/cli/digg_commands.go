package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/cliutil"
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
	root.AddCommand(newPipelineCmd(flags))
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

// ============== story ==============

func newStoryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "story [clusterUrlId]",
		Short:       "Show full detail for one cluster from the local store",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  digg-pp-cli story iq7usf9e
  digg-pp-cli story iq7usf9e --json`,
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
				`SELECT `+clusterSelectCols+`, COALESCE(score_components_json,''), COALESCE(authors_json,'[]'), COALESCE(hacker_news_json,''), COALESCE(techmeme_json,''), COALESCE(raw_json,'')
				 FROM digg_clusters WHERE cluster_url_id = ? OR cluster_id = ?`, id, id)
			var c clusterRow
			var peakRank sql.NullInt64
			var scoreJSON, authorsJSON, hnJSON, tmJSON, rawJSON string
			if err := row.Scan(&c.ClusterID, &c.ClusterURLID, &c.Label, &c.Title, &c.TLDR,
				&c.URL, &c.Permalink, &c.CurrentRank, &peakRank, &c.PreviousRank, &c.Delta,
				&c.GravityScore, &c.NumeratorCount, &c.NumeratorLabel, &c.Pos6h, &c.Pos12h, &c.Pos24h,
				&c.Likes, &c.Views, &c.SourceTitle, &c.ReplacementRationale, &c.ActivityAt, &c.LastSeenAt,
				&scoreJSON, &authorsJSON, &hnJSON, &tmJSON, &rawJSON); err != nil {
				if err == sql.ErrNoRows {
					return fmt.Errorf("cluster not found: %s (run `sync` or pass a clusterUrlId from `top --json --select clusterUrlId`)", id)
				}
				return err
			}
			c.PeakRank = int(peakRank.Int64)

			// Per R9: grow the story envelope with a `posts` field so a
			// single fetch covers ranking + citations. Reuses the same
			// loader as the standalone `posts` command (cache-first
			// with 1h TTL; honors --no-cache; falls back to live
			// fetch). On any error we surface an empty array and a
			// stderr warning rather than failing the whole story view
			// — old story callers shouldn't break when the parser hits
			// an upstream-shape change.
			postsRows, postsMeta := loadPostsForCluster(cmd.Context(), cmd, flags, db, c.ClusterURLID, postsLoadOptions{
				honorNoCache: flags.noCache,
				cacheTTL:     defaultClusterPostsTTL,
			})
			full := map[string]any{
				"cluster":         c,
				"scoreComponents": asJSONString(scoreJSON),
				"authors":         asJSONString(authorsJSON),
				"hackerNews":      asJSONString(hnJSON),
				"techmeme":        asJSONString(tmJSON),
				"posts":           postsRows,
				"postsMeta":       postsMeta,
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), full, flags)
			}
			return renderStoryText(cmd.OutOrStdout(), c, scoreJSON, authorsJSON, hnJSON, tmJSON)
		},
	}
	return cmd
}

func renderStoryText(w io.Writer, c clusterRow, scoreJSON, authorsJSON, hnJSON, tmJSON string) error {
	fmt.Fprintf(w, "%s\n", c.Label)
	if c.Title != "" && c.Title != c.Label {
		fmt.Fprintf(w, "  %s\n", c.Title)
	}
	fmt.Fprintf(w, "  rank %d (peak %d, prev %d, delta %+d)  gravity %.2f\n",
		c.CurrentRank, c.PeakRank, c.PreviousRank, c.Delta, c.GravityScore)
	if c.NumeratorLabel != "" {
		fmt.Fprintf(w, "  %s: %d\n", c.NumeratorLabel, c.NumeratorCount)
	}
	if c.Pos24h > 0 {
		fmt.Fprintf(w, "  positivity 6h=%.2f 12h=%.2f 24h=%.2f\n", c.Pos6h, c.Pos12h, c.Pos24h)
	}
	if c.SourceTitle != "" {
		fmt.Fprintf(w, "  source: %s\n", c.SourceTitle)
	}
	if c.URL != "" {
		fmt.Fprintf(w, "  link:   %s\n", c.URL)
	}
	if c.Permalink != "" {
		fmt.Fprintf(w, "  digg:   %s\n", c.Permalink)
	}
	if c.TLDR != "" {
		fmt.Fprintf(w, "\n%s\n", c.TLDR)
	}
	if c.ReplacementRationale != "" {
		fmt.Fprintf(w, "\nreplacement rationale: %s\n", c.ReplacementRationale)
	}

	// Authors
	if authorsJSON != "" && authorsJSON != "[]" && authorsJSON != "null" {
		var authors []diggparse.ClusterAuthor
		if err := json.Unmarshal([]byte(authorsJSON), &authors); err == nil && len(authors) > 0 {
			fmt.Fprintf(w, "\ncontributors (%d):\n", len(authors))
			for i, a := range authors {
				if i >= 10 {
					fmt.Fprintf(w, "  ... and %d more\n", len(authors)-i)
					break
				}
				name := firstNonEmpty(a.DisplayName, a.Username)
				fmt.Fprintf(w, "  - @%s (%s) [%s]\n", a.Username, name, a.PostType)
			}
		}
	}
	return nil
}

// ============== posts ==============
//
// `posts <clusterUrlId>` returns the X posts attached to one cluster:
// origins, replies, quotes, retweets — with author rank, body text,
// image URLs, repost-context chips, and minted X URLs. Live by default
// (fetch /ai/<id> + parse + cache + return); --data-source local reads
// from cache only. Cache TTL is 1h; --no-cache bypasses entirely.
//
// This is the load-bearing surface for `last30days` citations — it's
// what surfaces "top comments on this story." The structured-array
// shape mirrors the JSON-friendly contract documented in the plan; a
// caller can `jq '.results[] | select(.author.rank <= 100)'` to filter
// to high-credibility AI 1000 voices and get clean citation rows.
//
// The same parser feeds the `story` command's new `posts` envelope
// field via loadPostsForCluster, so a single fetch covers both
// surfaces (R9).

// clusterPostsURLOverride lets tests redirect the live /ai/<id> fetch
// to a local httptest server. Empty string falls through to the
// production base URL. Production code never sets this.
var clusterPostsURLOverride string

// defaultClusterPostsTTL is the in-store cache freshness window for
// per-cluster posts. One hour matches the upstream activity cadence on
// trending clusters: shorter and we'd hammer Digg on legitimate page
// reloads; longer and the citation freshness drifts. --no-cache
// bypasses the read; --data-source local skips the fetch even on a
// stale cache (returns the stored snapshot regardless of age).
const defaultClusterPostsTTL = time.Hour

// postsAuthorRow mirrors diggparse.ClusterPostAuthor for output. We
// declare a CLI-side row type rather than reusing the parser type so
// JSON tags stay independently controllable (e.g. omitempty rules
// differ between the parser layer's "raw" view and the CLI's "agent"
// view).
type postsAuthorRow struct {
	Username        string `json:"username"`
	DisplayName     string `json:"display_name,omitempty"`
	Category        string `json:"category,omitempty"`
	Rank            int    `json:"rank,omitempty"`
	ProfileImageURL string `json:"profile_image_url,omitempty"`
}

type postsRepostContextRow struct {
	RepostingHandle string `json:"reposting_handle"`
	OriginalHandle  string `json:"original_handle"`
}

// postRow is the per-post output shape. Body is a *string so the JSON
// preserves the null-vs-empty distinction documented in the plan;
// jq pipelines that test `select(.body != null)` need this. MediaURLs
// is always an array (possibly empty) so the JSON shape stays
// uniform across posts.
type postRow struct {
	PostXID    string                 `json:"post_x_id"`
	PostType   string                 `json:"post_type,omitempty"`
	PostedAt   string                 `json:"posted_at,omitempty"`
	Author     postsAuthorRow         `json:"author"`
	XURL       string                 `json:"xUrl,omitempty"`
	Body       *string                `json:"body"`
	BodyLoaded bool                   `json:"body_loaded"`
	MediaURLs  []string               `json:"media_urls"`
	Repost     *postsRepostContextRow `json:"repost_context"`
}

// postsEnvelope wraps the result list with provenance metadata.
// meta.source records "live" or "local"; meta.from_cache flags whether
// the rows were served from the in-store cache (vs a fresh fetch).
type postsEnvelope struct {
	Meta    map[string]any `json:"meta"`
	Results []postRow      `json:"results"`
}

// postsLoadOptions controls how loadPostsForCluster sources posts.
// honorNoCache reflects the user's --no-cache flag (bypass cache read,
// always refetch live). cacheTTL is the staleness budget; pass 0 to
// always refetch. forceLocal pins to cache-only (no network) — used
// when the user passes --data-source local on the posts command.
type postsLoadOptions struct {
	honorNoCache bool
	cacheTTL     time.Duration
	forceLocal   bool
}

// loadPostsForCluster is the shared loader used by both the `posts`
// command and the `story` command's new `posts` envelope field. It
// implements the live/cache/local data-source policy described above.
// Returns the slice of postRow + a meta map (source, from_cache,
// fetched_at, parser_warning when applicable). Errors are NOT
// returned here: callers are expected to surface partial-results
// gracefully so a transient parse hiccup doesn't fail the whole
// command. Errors are logged to stderr via cmd.ErrOrStderr().
func loadPostsForCluster(ctx context.Context, cmd *cobra.Command, flags *rootFlags, db *sql.DB, clusterUrlID string, opts postsLoadOptions) ([]postRow, map[string]any) {
	meta := map[string]any{
		"clusterUrlId": clusterUrlID,
	}
	if clusterUrlID == "" {
		meta["source"] = "none"
		meta["error"] = "clusterUrlId is empty; cannot load posts"
		return []postRow{}, meta
	}

	// Cache read (skipped when --no-cache or TTL <= 0).
	if !opts.honorNoCache && opts.cacheTTL > 0 {
		if cached, ok, fetchedAt, cerr := diggstore.GetClusterPosts(db, clusterUrlID, opts.cacheTTL); ok && cerr == nil {
			meta["source"] = "local"
			meta["from_cache"] = true
			meta["fetched_at"] = fetchedAt.UTC().Format(time.RFC3339)
			return promotePostsToRows(cached), meta
		} else if cerr != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: reading cached cluster posts failed: %v\n", cerr)
		}
	}

	// --data-source local: never hit the network. Read whatever is
	// in cache regardless of TTL; if there's nothing, return empty.
	if opts.forceLocal {
		if cached, ok, fetchedAt, _ := diggstore.GetClusterPosts(db, clusterUrlID, 365*24*time.Hour); ok {
			meta["source"] = "local"
			meta["from_cache"] = true
			meta["fetched_at"] = fetchedAt.UTC().Format(time.RFC3339)
			return promotePostsToRows(cached), meta
		}
		meta["source"] = "local"
		meta["from_cache"] = false
		meta["empty_reason"] = "no cached posts for cluster; rerun without --data-source local to fetch"
		return []postRow{}, meta
	}

	// Live fetch.
	c, cerr := flags.newClient()
	if cerr != nil {
		meta["source"] = "live"
		meta["error"] = cerr.Error()
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: building client for /ai/<id> fetch failed: %v\n", cerr)
		return []postRow{}, meta
	}
	url := clusterPostsURLBaseFromOverride() + "/" + clusterUrlID
	posts, perr := c.FetchClusterPostsFrom(ctx, url)
	if perr != nil && len(posts) == 0 {
		meta["source"] = "live"
		meta["error"] = perr.Error()
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: live /ai/%s fetch/parse failed: %v\n", clusterUrlID, perr)
		// Try to fall back to whatever's cached, ignoring TTL — better
		// to return a stale snapshot than nothing.
		if cached, ok, fetchedAt, _ := diggstore.GetClusterPosts(db, clusterUrlID, 365*24*time.Hour); ok {
			meta["source"] = "local"
			meta["from_cache"] = true
			meta["fetched_at"] = fetchedAt.UTC().Format(time.RFC3339)
			meta["fallback_reason"] = "live fetch failed; serving stale cache"
			return promotePostsToRows(cached), meta
		}
		return []postRow{}, meta
	}
	if perr != nil {
		// Partial parse: log the warning, keep the records we got.
		meta["parser_warning"] = perr.Error()
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: /ai/%s partial parse: %v\n", clusterUrlID, perr)
	}

	// Cache the fresh slice for next time.
	if cerr := diggstore.UpsertClusterPosts(db, clusterUrlID, posts, time.Now().UTC()); cerr != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: caching cluster posts failed: %v\n", cerr)
	}

	meta["source"] = "live"
	meta["from_cache"] = false
	meta["fetched_at"] = time.Now().UTC().Format(time.RFC3339)
	return promotePostsToRows(posts), meta
}

// clusterPostsURLBaseFromOverride returns the base /ai URL, honoring
// the test override when set. Production code never sets the
// override; tests substitute a local httptest server.
func clusterPostsURLBaseFromOverride() string {
	if clusterPostsURLOverride != "" {
		return clusterPostsURLOverride
	}
	return "https://di.gg/ai"
}

// promotePostsToRows converts diggparse.ClusterPost values into the
// CLI's postRow output shape. Defensive about MediaURLs: ensures the
// slice is non-nil so the JSON encodes as `[]` instead of `null`.
func promotePostsToRows(in []diggparse.ClusterPost) []postRow {
	out := make([]postRow, 0, len(in))
	for _, p := range in {
		row := postRow{
			PostXID:    p.PostXID,
			PostType:   p.PostType,
			PostedAt:   p.PostedAt,
			XURL:       p.XURL,
			Body:       p.Body,
			BodyLoaded: p.BodyLoaded,
			MediaURLs:  p.MediaURLs,
			Author: postsAuthorRow{
				Username:        p.Author.Username,
				DisplayName:     p.Author.DisplayName,
				Category:        p.Author.Category,
				Rank:            p.Author.Rank,
				ProfileImageURL: p.Author.ProfileImageURL,
			},
		}
		if row.MediaURLs == nil {
			row.MediaURLs = []string{}
		}
		if p.Repost != nil {
			row.Repost = &postsRepostContextRow{
				RepostingHandle: p.Repost.RepostingHandle,
				OriginalHandle:  p.Repost.OriginalHandle,
			}
		}
		out = append(out, row)
	}
	return out
}

// sortPostsBy applies the --by ordering to the slice in place. The
// default ("rank") sorts by author rank ascending, with chronological
// (posted_at ASC) as the tiebreaker so ties / null-rank rows still
// produce deterministic output. "type" prioritizes original tweets
// (the page-prominent posts) and pushes retweets to the bottom. "time"
// is straight chronological.
func sortPostsBy(rows []postRow, by string) error {
	switch by {
	case "", "rank":
		sort.SliceStable(rows, func(i, j int) bool {
			ri, rj := rows[i].Author.Rank, rows[j].Author.Rank
			// Treat 0/missing rank as "below all ranked authors" (largest
			// effective rank) so the highest-credibility voices float to
			// the top.
			if ri == 0 {
				ri = 1<<31 - 1
			}
			if rj == 0 {
				rj = 1<<31 - 1
			}
			if ri != rj {
				return ri < rj
			}
			return rows[i].PostedAt < rows[j].PostedAt
		})
	case "type":
		order := map[string]int{"tweet": 0, "quote": 1, "reply": 2, "retweet": 3}
		sort.SliceStable(rows, func(i, j int) bool {
			oi, oki := order[rows[i].PostType]
			oj, okj := order[rows[j].PostType]
			if !oki {
				oi = 99
			}
			if !okj {
				oj = 99
			}
			if oi != oj {
				return oi < oj
			}
			return rows[i].PostedAt < rows[j].PostedAt
		})
	case "time":
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].PostedAt < rows[j].PostedAt })
	default:
		return fmt.Errorf("--by must be one of: rank, type, time")
	}
	return nil
}

func newPostsCmd(flags *rootFlags) *cobra.Command {
	var by string
	var typeFilter string
	var limit int
	cmd := &cobra.Command{
		Use:         "posts <clusterUrlId>",
		Short:       "X posts attached to one cluster: rank, body, media, repost-context, X URLs",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `List the X posts attached to one cluster, with author rank, body
text, image URLs, repost-context, and minted X URLs.

Live by default — fetches /ai/<clusterUrlId>, parses the embedded RSC
posts array, attaches DOM-rendered bodies and media, caches the result
for 1 hour. --data-source local reads from the cache only (no
network). --no-cache bypasses the cache entirely and forces a fresh
fetch.

Sort options:
  rank   author rank ascending (default; ties broken by posted_at)
  type   originals first (tweet, then quote, then reply, then retweet)
  time   chronological ascending by posted_at`,
		Example: `  digg-pp-cli posts 65idu2x5
  digg-pp-cli posts 65idu2x5 --by rank --limit 5 --json
  digg-pp-cli posts 65idu2x5 --type tweet --json
  digg-pp-cli posts 65idu2x5 --by time --agent
  digg-pp-cli posts 65idu2x5 --no-cache --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id := strings.TrimSpace(args[0])
			if id == "" {
				return usageErr(fmt.Errorf("clusterUrlId is required (e.g. `digg-pp-cli posts 65idu2x5`)"))
			}
			ctx := cmd.Context()
			_, db, closeFn, err := openStore(ctx)
			if err != nil {
				return err
			}
			defer closeFn()

			rows, meta := loadPostsForCluster(ctx, cmd, flags, db, id, postsLoadOptions{
				honorNoCache: flags.noCache,
				cacheTTL:     defaultClusterPostsTTL,
				forceLocal:   flags.dataSource == "local",
			})

			// --type filter runs BEFORE sort so the limit applies to the
			// filtered list (a caller asking for `--type tweet --limit 5`
			// gets up to 5 originals, never fewer because retweets ate
			// into the budget).
			if typeFilter != "" {
				filtered := make([]postRow, 0, len(rows))
				for _, r := range rows {
					if r.PostType == typeFilter {
						filtered = append(filtered, r)
					}
				}
				rows = filtered
			}

			if err := sortPostsBy(rows, by); err != nil {
				return err
			}

			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}

			env := postsEnvelope{
				Meta: map[string]any{
					"clusterUrlId": id,
					"by":           by,
					"count":        len(rows),
				},
				Results: rows,
			}
			// Hoist the loader's source/cache metadata into the envelope.
			for k, v := range meta {
				if k == "clusterUrlId" {
					continue // already set
				}
				env.Meta[k] = v
			}
			if typeFilter != "" {
				env.Meta["type"] = typeFilter
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), env, flags)
			}
			if len(rows) == 0 {
				return emptyHint(cmd, fmt.Sprintf("no posts surfaced for cluster %s.", id))
			}
			for _, r := range rows {
				name := firstNonEmpty(r.Author.DisplayName, r.Author.Username)
				rankStr := "-"
				if r.Author.Rank > 0 {
					rankStr = fmt.Sprintf("#%d", r.Author.Rank)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "[%s] @%-20s %s  %s\n", r.PostType, r.Author.Username, diggTruncate(name, 24), rankStr)
				if r.Body != nil && *r.Body != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", diggTruncate(*r.Body, 200))
				}
				if r.XURL != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", r.XURL)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&by, "by", "rank", "Sort: rank | type | time")
	cmd.Flags().StringVar(&typeFilter, "type", "", "Filter to one post type: tweet | reply | quote | retweet")
	cmd.Flags().IntVar(&limit, "limit", 0, "Cap results (0 = no cap)")
	return cmd
}

// ============== search ==============
//
// `search` hits Digg's undocumented /api/search/stories endpoint by
// default and falls back to the local FTS5 store on network error.
// `--data-source live` forces live (errors propagate); `--data-source
// local` forces FTS5 (no network call). The output envelope is shared
// across both branches so existing --select flags keep working — the
// live branch maps `description` → `tldr` and pulls through `rank`,
// `postCount`, `uniqueAuthors`, `firstPostAge` (which U3 will filter on).

// searchResult is the unified envelope row produced by both the live
// (/api/search/stories) and local (FTS5) branches. JSON tags match the
// upstream documented shape so agents can rely on a stable contract
// regardless of source. Source-specific fields (PostCount, FirstPostAge)
// stay zero/empty on the local path; rank lives on `rank` for live
// (relevance rank) and `currentRank` for local (today's leaderboard
// rank) so callers can tell them apart.
type searchResult struct {
	ClusterID     string `json:"clusterId"`
	ClusterURLID  string `json:"clusterUrlId"`
	Title         string `json:"title,omitempty"`
	Label         string `json:"label,omitempty"`
	TLDR          string `json:"tldr,omitempty"`
	Rank          int    `json:"rank,omitempty"`
	CurrentRank   int    `json:"currentRank,omitempty"`
	PostCount     int    `json:"postCount,omitempty"`
	UniqueAuthors int    `json:"uniqueAuthors,omitempty"`
	FirstPostAge  string `json:"firstPostAge,omitempty"`
}

// searchEnvelope wraps the result list with provenance metadata. Shape
// matches `authors list` so SDK consumers can reuse the parser.
type searchEnvelope struct {
	Meta    map[string]any `json:"meta"`
	Results []searchResult `json:"results"`
}

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var sinceStr string
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Cluster search: live /api/search/stories by default, local FTS5 fallback",
		Long: `Search Digg's full cluster surface for matching stories.

Live by default (--data-source auto): hits /api/search/stories — Digg's
own server-side search backing the di.gg/ai Cmd+K modal. Returns rich
fields (postCount, uniqueAuthors, firstPostAge) covering Digg's full
window, not just the locally-synced snapshot.

Falls back to the local FTS5 store (digg_clusters_fts) on network error,
or when --data-source local is passed. The local path searches today's
synced clusters by title/label/TLDR.

The --since flag filters by how recently each cluster was first posted.
Live mode parses Digg's own firstPostAge ("2d", "26d", "5h"); local
mode filters by the digg_clusters.first_post_at column. Accepts the
formats Digg returns (Nh, Nd, Nw) plus Nm for months (~30 days).`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  digg-pp-cli search "openai gpt-5"
  digg-pp-cli search "<topic>" --since 30d --agent
  digg-pp-cli search "claude" --since 7d --json
  digg-pp-cli search "robotics" --json --select clusterUrlId,title,rank,postCount
  digg-pp-cli search "claude" --data-source local`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" {
				return usageErr(fmt.Errorf("query is required (e.g. `digg-pp-cli search \"openai\"`)"))
			}
			ctx := cmd.Context()

			// Parse --since once up front so a malformed value is surfaced
			// as a usage error before we hit the network or the store. A
			// zero `since` (when the flag is empty) signals "no filter."
			var since time.Duration
			if s := strings.TrimSpace(sinceStr); s != "" {
				d, err := cliutil.ParseDiggAge(s)
				if err != nil {
					return usageErr(fmt.Errorf("--since %q: %w (accepts Nh, Nd, Nw, Nm; e.g. 30d)", sinceStr, err))
				}
				since = d
			}

			// Branch on --data-source.
			ds := flags.dataSource
			if ds == "" {
				ds = "auto"
			}

			var (
				results []searchResult
				source  string
			)

			// Filter must run BEFORE limit so callers don't end up with
			// fewer than --limit results when the upstream returned more.
			// We pass `since` into both branches; live filters in-memory
			// on `firstPostAge`, local filters in-SQL on first_post_at.
			switch ds {
			case "local":
				lr, err := localSearch(ctx, query, limit, since)
				if err != nil {
					return err
				}
				results = lr
				source = "local"

			case "live":
				lr, err := liveSearch(ctx, flags, query, limit, since)
				if err != nil {
					return err
				}
				results = lr
				source = "live"

			default: // "auto"
				lr, err := liveSearch(ctx, flags, query, limit, since)
				if err == nil {
					results = lr
					source = "live"
				} else {
					fmt.Fprintf(cmd.ErrOrStderr(), "falling back to local FTS5: %v\n", err)
					fallback, ferr := localSearch(ctx, query, limit, since)
					if ferr != nil {
						// Live failed and local failed too; surface the live error
						// (more actionable than a "no synced data" hint when network
						// is the actual problem) but include the local error context.
						return fmt.Errorf("live search failed (%v) and local fallback failed: %w", err, ferr)
					}
					results = fallback
					source = "local"
				}
			}

			if len(results) == 0 {
				if flags.asJSON {
					meta := map[string]any{
						"source": source,
						"query":  query,
						"count":  0,
					}
					if sinceStr != "" {
						meta["since"] = sinceStr
					}
					env := searchEnvelope{
						Meta:    meta,
						Results: []searchResult{},
					}
					return printJSONFiltered(cmd.OutOrStdout(), env, flags)
				}
				return emptyHint(cmd, fmt.Sprintf("no matches for %q (source=%s). Try a different query, or `digg-pp-cli sync` if the local store is empty.", query, source))
			}

			if flags.asJSON {
				meta := map[string]any{
					"source": source,
					"query":  query,
					"count":  len(results),
				}
				if sinceStr != "" {
					meta["since"] = sinceStr
				}
				env := searchEnvelope{
					Meta:    meta,
					Results: results,
				}
				return printJSONFiltered(cmd.OutOrStdout(), env, flags)
			}
			for _, r := range results {
				rank := r.Rank
				if rank == 0 {
					rank = r.CurrentRank
				}
				headline := firstNonEmpty(r.Title, r.Label, r.ClusterURLID)
				fmt.Fprintf(cmd.OutOrStdout(), "#%d  %s  [%s]\n", rank, headline, r.ClusterURLID)
				if r.TLDR != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "    %s\n", diggTruncate(r.TLDR, 200))
				}
				if r.FirstPostAge != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "    posts=%d  authors=%d  age=%s\n", r.PostCount, r.UniqueAuthors, r.FirstPostAge)
				}
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Max number of results")
	cmd.Flags().StringVar(&sinceStr, "since", "", "Filter to clusters first posted within this window. Accepts Nh, Nd, Nw, Nm (e.g. 30d, 1w, 12h, 1m). Default empty (no filter).")
	return cmd
}

// searchStoriesURLOverride lets tests redirect the live search URL to
// a local httptest server. Empty string falls through to the default
// "https://di.gg/api/search/stories". Production code never sets this.
var searchStoriesURLOverride string

// liveSearch hits /api/search/stories via the shared client and maps
// the upstream envelope onto searchResult. The optional `since` window
// is applied AFTER the upstream returns and BEFORE limit clamping so a
// caller passing `--limit 5 --since 7d` gets up to 5 results that all
// fall within the window — never fewer than 5 because some 26d-old
// rows ate into the budget.
//
// Filter policy: a record whose firstPostAge fails to parse is KEPT.
// Digg occasionally returns shapes the parser doesn't recognize and
// silently dropping them is worse than surfacing them with the original
// string so the caller can decide.
func liveSearch(ctx context.Context, flags *rootFlags, query string, limit int, since time.Duration) ([]searchResult, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	// When --since is active we have to over-fetch from upstream so the
	// in-memory filter isn't capped by the server's `limit`. Without
	// this, a user asking for `--limit 5 --since 7d` could get fewer
	// than 5 in-window results when older rows were among the first 5
	// the server returned. We don't pass `limit` to upstream in that
	// case; the post-filter caller-side cap below applies instead.
	upstreamLimit := limit
	if since > 0 {
		upstreamLimit = 0
	}
	var resp *client.StoriesSearchResponse
	if searchStoriesURLOverride != "" {
		resp, err = c.SearchStoriesFrom(ctx, searchStoriesURLOverride, query, upstreamLimit)
	} else {
		resp, err = c.SearchStories(ctx, query, upstreamLimit)
	}
	if err != nil {
		return nil, err
	}
	out := make([]searchResult, 0, len(resp.Results))
	for _, r := range resp.Results {
		if since > 0 {
			// firstPostAge may be empty (never observed but defensive)
			// or in a shape the parser doesn't recognize — both cases
			// keep the record. Only successfully-parsed ages > since
			// are dropped.
			if age, perr := cliutil.ParseDiggAge(r.FirstPostAge); perr == nil {
				if age > since {
					continue
				}
			}
		}
		out = append(out, searchResult{
			ClusterID:     r.ClusterID,
			ClusterURLID:  r.ClusterURLID,
			Title:         r.Title,
			TLDR:          r.Description,
			Rank:          r.Rank,
			PostCount:     r.PostCount,
			UniqueAuthors: r.UniqueAuthors,
			FirstPostAge:  r.FirstPostAge,
		})
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

// localSearch runs the existing FTS5 query against the local store.
// Returns an empty slice (not an error) when there are no matches, so
// the caller can render the empty-results envelope or the empty-hint
// message uniformly.
//
// When `since > 0` the query is gated by digg_clusters.first_post_at
// (NOT first_seen_at — that column doesn't exist on this schema; see
// internal/diggstore/store.go EnsureSchema). We compare on the column
// whose semantics match the live `firstPostAge` field: when the cluster
// first observed a post. Records with a NULL or empty first_post_at
// are kept (matching the live-mode "keep on parse failure" policy)
// rather than silently filtered out.
func localSearch(ctx context.Context, query string, limit int, since time.Duration) ([]searchResult, error) {
	_, db, closeFn, err := openStore(ctx)
	if err != nil {
		return nil, err
	}
	defer closeFn()
	q := `SELECT c.cluster_id, c.cluster_url_id, COALESCE(c.label,''), COALESCE(c.tldr,''), COALESCE(c.current_rank,0)
		 FROM digg_clusters_fts f JOIN digg_clusters c ON c.cluster_id = f.cluster_id
		 WHERE digg_clusters_fts MATCH ?`
	args := []any{query}
	if since > 0 {
		// Keep rows missing first_post_at — see comment above.
		q += ` AND (c.first_post_at IS NULL OR c.first_post_at = '' OR c.first_post_at >= ?)`
		args = append(args, time.Now().Add(-since).UTC().Format(time.RFC3339))
	}
	q += ` ORDER BY c.current_rank ASC LIMIT ?`
	args = append(args, limit)

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("FTS query: %w", err)
	}
	defer rows.Close()
	var results []searchResult
	for rows.Next() {
		var r searchResult
		if err := rows.Scan(&r.ClusterID, &r.ClusterURLID, &r.Label, &r.TLDR, &r.CurrentRank); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// ============== events ==============

func newEventsCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var sinceStr string
	var typeFilter string
	cmd := &cobra.Command{
		Use:         "events",
		Short:       "Tail Digg's ingestion-pipeline event stream from the local store",
		Long:        `Read events that were captured during sync from /api/trending/status: cluster_detected, fast_climb (with delta + previousRank → currentRank), post_understanding (X posts being processed), batch_started/batch_breakdown/posts_stored, and embedding_progress.`,
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
		Use:         "authors",
		Short:       "Inspect the Digg AI 1000 — the curated leaderboard of AI accounts on X",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newAuthorsTopCmd(flags))
	cmd.AddCommand(newAuthorsListCmd(flags))
	cmd.AddCommand(newAuthorsGetCmd(flags))
	return cmd
}

func newAuthorsTopCmd(flags *rootFlags) *cobra.Command {
	var by string
	var limit int
	cmd := &cobra.Command{
		Use:         "top",
		Short:       "Top contributors across the Digg AI 1000, ranked by influence, post count, or reach",
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
		Short:       "Full ranked AI 1000 with rich fields (rank, category, bio, vibeDistribution); fetches /ai/1000 by default",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `List the full Digg AI 1000 with per-author rank, category, bio, GitHub URL, and vibe distribution.

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
				// SearchStories, SearchUsers, FetchUserPeerFollowCount).
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
// The off-1000 path adds a "distance to the 1000" view by reading
// the rank-1000 row from the local digg_authors cache (populated by
// `authors list`). On cache miss the command does a one-shot live
// /ai/1000 fetch + upsert (the same call `authors list` makes), then
// re-queries. If both the cache is empty AND the live fetch fails,
// the command still returns a useful payload — just without the
// nearest_in_1000 / peer_follow_gap fields, with
// meta.tier_status_resolved set to false so callers can detect the
// partial result.
//
// The distance metric is `peer_follow_gap` — the difference in
// `followed_by_count` (AI-1000 peer follows), which is what Digg
// actually ranks the AI 1000 by. The subject's peer-follow count
// comes from /u/x/<handle> (the page's <meta name="description">
// reveals it). Total X followers are also surfaced for context but
// are not used as the comparison metric.

// searchUsersURLOverride lets tests redirect the live user-search
// URL to a local httptest server. Empty string falls through to the
// default "https://di.gg/api/search/users". Production code never
// sets this.
var searchUsersURLOverride string

// roster1000URLOverride lets tests redirect the live /ai/1000 fetch
// used on the off-1000 cache-cold fallback path to a local httptest
// server. Empty string falls through to the default
// "https://di.gg/ai/1000". Production code never sets this.
var roster1000URLOverride string

// authorGetResult is the per-author payload returned by
// `authors get`. Field naming mirrors UsersSearchResult upstream
// where it makes sense, with CLI-side additions:
//
//   - XURL: minted from username
//   - GithubURL: looked up from local digg_authors cache when present
//     (the search endpoint doesn't return it; /ai/1000 does)
//   - TierStatus: "in_1000" when CurrentRank != nil, else "off_1000"
//   - SubjectPeerFollowCount: the off-1000 subject's "followed by N
//     tracked AI influencers" count, scraped from the
//     /u/x/<handle> page meta description. Pointer so we can omit
//     it cleanly when the page fetch / parse fails.
//   - NearestIn1000 / PeerFollowGap: present only on off-1000 path,
//     and only when the rank-1000 anchor was resolvable (cache or
//     live fetch). Both omitted when neither source produced a record.
//   - PeerFollowGap is computed as `nearest.peer_follow_count -
//     subject.peer_follow_count`. Positive means "subject needs that
//     many more AI-1000 peer-follows to catch rank 1000"; negative
//     means the off-1000 subject has more peer-follows than the
//     rank-1000 anchor (ranking isn't strictly peer-follow either —
//     Digg uses a weighted score).
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
	XID                    string             `json:"x_id,omitempty"`
	Username               string             `json:"username"`
	DisplayName            string             `json:"display_name,omitempty"`
	ProfileImageURL        string             `json:"profile_image_url,omitempty"`
	FollowersCount         int                `json:"followers_count"`
	Category               *string            `json:"category"`
	CurrentRank            *int               `json:"current_rank"`
	SimilarityScore        float64            `json:"similarity_score"`
	IsPrefixMatch          bool               `json:"is_prefix_match"`
	XURL                   string             `json:"xUrl,omitempty"`
	GithubURL              string             `json:"githubUrl,omitempty"`
	TierStatus             string             `json:"tier_status,omitempty"`
	SubjectPeerFollowCount *int               `json:"subject_peer_follow_count,omitempty"`
	NearestIn1000          *nearestIn1000Anch `json:"nearest_in_1000,omitempty"`
	PeerFollowGap          *int               `json:"peer_follow_gap,omitempty"`
	MatchType              string             `json:"match_type,omitempty"`
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
		Short:       "Look up a handle on /api/search/users; off-1000 results include distance to the 1000",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Look up an X handle in Digg's full author universe (1000 + off-1000).

Live by default — hits /api/search/users, the same vector+prefix
search that backs the di.gg/ai handle picker. Returns a single rich
record on exact match, or an array of fuzzy candidates sorted by
similarity_score when the handle has no exact hit.

For off-1000 handles (current_rank: null), the response also
includes a "distance to the 1000" view: the rank-1000 author's
stats as a comparison anchor and a signed peer_follow_gap (the
difference in AI-1000 peer-follow counts — the metric Digg
actually ranks by). The anchor is read from the local digg_authors
cache (populated by ` + "`digg-pp-cli authors list`" + `); on cache
miss the command does a one-shot live /ai/1000 fetch and populates
the cache before returning. The subject's own peer-follow count is
fetched from the /u/x/<handle> page. If either piece doesn't
resolve, the rest of the record is returned with
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
			// anchor with live fallback, HTTP client) out of the fuzzy
			// loop so a 5-candidate fuzzy fallback pays for one of each
			// rather than five.
			env := computeAuthorGetEnv(ctx, cmd, flags)
			defer env.closeFn()

			if exact != nil {
				record, resolved := buildAuthorGetResult(ctx, cmd, env, *exact, "exact")
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
				record, resolved := buildAuthorGetResult(ctx, cmd, env, r, "fuzzy")
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
// pays for one DB open, one rank-1000 anchor lookup (with optional
// live fallback), and one Client construction — not N of each.
//
// `anchor` is nil when the rank-1000 anchor couldn't be resolved
// (cache cold + live fallback failure). `anchorOK` is the matching
// resolved bit. Callers don't need to recompute either; they only
// gate per-candidate decisions on them.
//
// `db` is owned by the caller (computeAuthorGetEnv's caller is
// responsible for closing it via the returned closeFn). Per-candidate
// reads (`lookupGithubURL`, `loadRank1000AnchorForDB`, FTS hits) all
// share this single SQLite handle so we don't open one per candidate.
//
// `httpClient` is the shared *client.Client used for /u/x/<handle>
// peer-follow fetches across candidates. nil means client construction
// failed at env setup time; per-candidate paths fall back to "subject
// peer-follow not available" rather than rebuilding it.
type authorGetEnv struct {
	db         *sql.DB
	closeFn    func() error
	anchor     *nearestIn1000Anch
	anchorOK   bool
	httpClient *client.Client
}

// computeAuthorGetEnv opens the shared store, resolves the rank-1000
// anchor once (with live fallback when allowed), and constructs the
// shared HTTP Client. The returned env's `closeFn` MUST be called by
// the caller once the per-candidate loop is done.
//
// Failures degrade gracefully:
//
//   - DB open failure → env.db is nil; per-candidate DB lookups skip
//     silently. The closeFn is still safe to call.
//   - Anchor resolution failure (cache miss + live fetch failure) →
//     env.anchor is nil and env.anchorOK is false. Per-candidate
//     off-1000 paths will surface partial data without nearest_in_1000.
//   - Client construction failure → env.httpClient is nil. Per-
//     candidate off-1000 paths will skip the subject peer-follow fetch.
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
	if c, err := flags.newClient(); err == nil {
		env.httpClient = c
	} else {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: building client for authors get peer-follow fetches failed: %v\n", err)
	}
	return env
}

// buildAuthorGetResult promotes one UsersSearchResult into the CLI's
// authorGetResult shape. Mints xUrl, computes tier_status, and (for
// off-1000) attaches the rank-1000 anchor and the subject's
// peer-follow count for the distance view. Returns the result and a
// `tierResolved` bool: true when in_1000 OR when off_1000 with BOTH
// a successful anchor lookup AND a successful subject peer-follow
// fetch; false when either piece of the off-1000 distance view is
// missing. Callers surface that bit on meta.tier_status_resolved.
//
// Failure modes are kept independent so partial data still flows:
// if /u/x/<handle> fails we still emit nearest_in_1000 (caller sees
// the rank-1000 anchor); if the anchor fails we still emit
// subject_peer_follow_count (caller sees the subject's standing).
// peer_follow_gap is only emitted when both sides resolve.
//
// `env` carries shared state hoisted out of the fuzzy-candidate loop
// so calling this for N candidates pays only one DB open, one anchor
// lookup, and one client construction across the batch.
func buildAuthorGetResult(ctx context.Context, cmd *cobra.Command, env authorGetEnv, src client.UsersSearchResult, matchType string) (authorGetResult, bool) {
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

	// In-1000 path: tier_status fixed; we still try a quick local
	// lookup for githubUrl since /api/search/users doesn't return it
	// but the /ai/1000 ingest does. Failure is silent — a missing
	// githubUrl is normal and not a partial-result signal.
	if src.CurrentRank != nil {
		out.TierStatus = "in_1000"
		if gh := lookupGithubURLForDB(ctx, env.db, src.Username); gh != "" {
			out.GithubURL = gh
		}
		return out, true
	}

	// Off-1000 path: surface the pre-computed rank-1000 anchor and
	// fetch the subject's peer-follow count. The two halves can fail
	// separately; we surface whichever resolved and only emit
	// peer_follow_gap when both did.
	out.TierStatus = "off_1000"
	if gh := lookupGithubURLForDB(ctx, env.db, src.Username); gh != "" {
		out.GithubURL = gh
	}
	if env.anchorOK {
		out.NearestIn1000 = env.anchor
	}
	subjectPeerFollow, subjectOK := fetchSubjectPeerFollowCountWithClient(ctx, cmd, env.httpClient, src.Username)
	if subjectOK {
		spf := subjectPeerFollow
		out.SubjectPeerFollowCount = &spf
	}
	if env.anchorOK && subjectOK {
		gap := env.anchor.PeerFollowCount - subjectPeerFollow
		out.PeerFollowGap = &gap
		return out, true
	}
	return out, false
}

// userPeerFollowURLOverride lets tests redirect the live
// /u/x/<handle> fetch used to read the subject's
// `followed_by_count` to a local httptest server. Empty string falls
// through to the default base URL handled inside the client.
// Production code never sets this.
var userPeerFollowURLOverride string

// fetchSubjectPeerFollowCountWithClient calls /u/x/<handle> via the
// caller-supplied shared *client.Client and returns the parsed
// peer-follow count. Logs warnings to stderr on failure (404 is
// silent — that's a legitimate "Digg doesn't track this handle"
// zero, returned through the client) so operators can debug without
// the rest of the response failing.
//
// Returns (n, true) on success, (0, false) on any kind of failure
// (network, non-200/non-404 status, missing meta tag, bad parse, or
// nil client passed in).
//
// Reusing the caller-supplied client across candidates keeps the
// rate limiter, timeout, and impersonated transport pool shared so
// a 5-candidate fuzzy fallback doesn't construct 5 clients.
func fetchSubjectPeerFollowCountWithClient(ctx context.Context, cmd *cobra.Command, c *client.Client, username string) (int, bool) {
	if username == "" || c == nil {
		return 0, false
	}
	var (
		n   int
		err error
	)
	if userPeerFollowURLOverride != "" {
		n, err = c.FetchUserPeerFollowCountFrom(ctx, userPeerFollowURLOverride+"/"+username)
	} else {
		n, err = c.FetchUserPeerFollowCount(ctx, username)
	}
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: /u/x/%s peer-follow fetch failed: %v\n", username, err)
		return 0, false
	}
	return n, true
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
// `peer_follow_count` on the output anchor so the off-1000 distance
// view compares apples to apples.
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
// category, and (when off-1000) the peer-follow distance view.
// Keep it dense so `digg-pp-cli authors get foo` reads at a glance.
//
// Distance view shows the AI-1000 peer-follow gap rather than the
// raw X-followers gap because the AI 1000 is ranked by
// `followed_by_count`, not total followers.
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
	if r.NearestIn1000 != nil && r.PeerFollowGap != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "    nearest_in_1000=@%s (rank %d, peer_follows=%d)  peer_follow_gap=%+d\n",
			r.NearestIn1000.Username, r.NearestIn1000.Rank, r.NearestIn1000.PeerFollowCount, *r.PeerFollowGap)
	} else if r.NearestIn1000 != nil {
		// Anchor resolved but subject's peer-follow fetch failed;
		// still show the rank-1000 anchor so the caller has context.
		fmt.Fprintf(cmd.OutOrStdout(), "    nearest_in_1000=@%s (rank %d, peer_follows=%d)\n",
			r.NearestIn1000.Username, r.NearestIn1000.Rank, r.NearestIn1000.PeerFollowCount)
	}
	if r.SubjectPeerFollowCount != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "    subject_peer_follow_count=%d\n", *r.SubjectPeerFollowCount)
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
	cmd := &cobra.Command{
		Use:         "watch",
		Short:       "Poll /ai on an interval and alert when any cluster moves N+ ranks",
		Long:        "Polls Digg, parses the feed, diffs against the previous local snapshot, and prints any cluster whose absolute rank delta is at-or-above --min-delta (or matches --alert). READ-ONLY: never writes anything to Digg.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  digg-pp-cli watch --interval 60s --min-delta 5
  digg-pp-cli watch --alert 'rank.delta>=10'
  digg-pp-cli watch --interval 30s --iterations 3   # for verify`,
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
			it := 0
			for {
				it++
				html, err := fetchURL(ctx, "https://di.gg/ai")
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

// ============== pipeline ==============

func newPipelineCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "pipeline",
		Short:       "Inspect Digg's ingestion pipeline (status + events)",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newPipelineStatusCmd(flags))
	return cmd
}

func newPipelineStatusCmd(flags *rootFlags) *cobra.Command {
	var watchMode bool
	var interval time.Duration
	cmd := &cobra.Command{
		Use:         "status",
		Short:       "One-screen view of /api/trending/status: isFetching, nextFetchAt, storiesToday, clustersToday",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  digg-pp-cli pipeline status
  digg-pp-cli pipeline status --watch --interval 60s`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			fetchOnce := func() error {
				body, err := fetchURL(cmd.Context(), "https://di.gg/api/trending/status")
				if err != nil {
					return err
				}
				ts, err := diggparse.ParseTrendingStatus(body)
				if err != nil {
					return err
				}
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), ts, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Pipeline status (computed at %s)\n", ts.ComputedAt)
				fmt.Fprintf(cmd.OutOrStdout(), "  isFetching:           %v\n", ts.IsFetching)
				fmt.Fprintf(cmd.OutOrStdout(), "  storiesToday:         %d\n", ts.StoriesToday)
				fmt.Fprintf(cmd.OutOrStdout(), "  clustersToday:        %d\n", ts.ClustersToday)
				fmt.Fprintf(cmd.OutOrStdout(), "  nextFetchAt:          %s\n", ts.NextFetchAt)
				fmt.Fprintf(cmd.OutOrStdout(), "  lastFetchCompletedAt: %s\n", ts.LastFetchCompletedAt)
				fmt.Fprintf(cmd.OutOrStdout(), "  recent events: %d\n", len(ts.Events))
				for i, e := range ts.Events {
					if i >= 5 {
						break
					}
					fmt.Fprintf(cmd.OutOrStdout(), "    %s  %s\n", e.At, e.Type)
				}
				return nil
			}
			if !watchMode {
				return fetchOnce()
			}
			for {
				if err := fetchOnce(); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "status: %v\n", err)
				}
				select {
				case <-cmd.Context().Done():
					return nil
				case <-time.After(interval):
				}
			}
		},
	}
	cmd.Flags().BoolVar(&watchMode, "watch", false, "Re-poll the endpoint on an interval")
	cmd.Flags().DurationVar(&interval, "interval", 60*time.Second, "Poll interval")
	return cmd
}

// ============== open ==============

func newOpenCmd(flags *rootFlags) *cobra.Command {
	var launch bool
	cmd := &cobra.Command{
		Use:         "open [clusterUrlId]",
		Short:       "Print or open the Digg URL for one cluster",
		Long:        "Prints the digg.com permalink for the given cluster. By default does NOT launch a browser; pass --launch to actually open. Per the printing-press side-effect convention.",
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
			id := args[0]
			_, db, closeFn, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer closeFn()
			var perm, urlID string
			err = db.QueryRowContext(cmd.Context(),
				`SELECT COALESCE(permalink,''), COALESCE(cluster_url_id,'')
				 FROM digg_clusters WHERE cluster_url_id = ? OR cluster_id = ?`, id, id).Scan(&perm, &urlID)
			if err != nil {
				if err == sql.ErrNoRows {
					return fmt.Errorf("cluster not found: %s (run sync first)", id)
				}
				return err
			}
			if perm == "" {
				perm = "https://di.gg/ai/" + urlID
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
