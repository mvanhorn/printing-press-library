// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

// Hand-written implementation of `find hashtag`. Fans a comma-separated
// --tags list into per-tag SERP fetches by default (union semantics).
// With --require-all, fires a single joined-tag SERP query (Naver
// treats whitespace as AND), then fetches each candidate post and
// confirms via the post's gsTagName tag list whether every required
// tag is actually attached. The intersection-by-SERP-URL approach used
// previously was mathematically correct but semantically wrong:
// Naver's per-tag SERPs only show ~22 results each, and the
// intersection of two tag SERPs almost always misses posts that carry
// both tags but ranked off-page on one of them. The fetch-and-confirm
// path mirrors the user's intent ("posts whose tag list is a superset
// of these tags") rather than the SERP-set intersection.

package cli

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/client"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/lib/postparse"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/lib/serpparse"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/store"
)

func newFindHashtagCmd(flags *rootFlags) *cobra.Command {
	var (
		flagWhere      string
		flagTags       string
		flagRequireAll bool
		flagMonth      string
		flagLimit      int
	)

	cmd := &cobra.Command{
		Use:   "hashtag",
		Short: "Search Naver Blog posts by hashtag(s) — union by default, intersection with --require-all.",
		Long: `Search Naver Blog posts by hashtag. Accepts a comma-separated list and prefixes each with '#' before querying Naver integrated search.

Default behavior (union): fan a per-tag SERP query and dedupe by URL. Each returned hit's hashtag_match field lists which tag(s) the SERP returned it for.

With --require-all (intersection): fire ONE joined-tag SERP (Naver treats whitespace as AND), then fetch each candidate post's mobile HTML and confirm via the post's gsTagName tag list that every required tag is actually attached. This catches posts that carry both tags but ranked off-page on either tag's per-tag SERP — a failure mode of the naive set-intersection approach. --limit applies AFTER the tag-list filter.

A "match" is case-insensitive and ignores the leading '#'.`,
		Example: `  naver-blog-pp-cli find hashtag --tags 협찬,체험단
  naver-blog-pp-cli find hashtag --tags 칠리,신상 --require-all`,
		Annotations: map[string]string{
			"pp:endpoint":   "find.hashtag",
			"pp:method":     "GET",
			"pp:path":       naverSERPBaseURL,
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Honor --dry-run before required-flag validation so verify
			// dry-run probes succeed without forcing a sample tags value.
			if dryRunOK(flags) {
				return nil
			}
			if strings.TrimSpace(flagTags) == "" {
				if flags.asJSON {
					_ = printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"error": "tags is required",
						"usage": fmt.Sprintf("%s --tags <tag1>[,<tag2>...]", cmd.CommandPath()),
					}, flags)
				}
				return usageErr(fmt.Errorf("required flag --tags not set"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			tags := splitTags(flagTags)
			if len(tags) == 0 {
				return usageErr(fmt.Errorf("no valid tags after parsing %q", flagTags))
			}
			var results []serpparse.SearchResult
			if flagRequireAll {
				results, err = runHashtagIntersection(ctx, c, tags, flagWhere, flagLimit)
			} else {
				results, err = runHashtagUnion(ctx, c, tags, flagWhere, flagLimit)
			}
			if err != nil {
				return classifyAPIError(err, flags)
			}
			_ = flagMonth // applied downstream by 'posts get'
			cacheHashtagResults(ctx, results)
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&flagWhere, "where", "m_view", "Naver search vertical")
	cmd.Flags().StringVar(&flagTags, "tags", "", "Comma-separated tag(s) without '#'; CLI adds '#' before querying. Required.")
	cmd.Flags().BoolVar(&flagRequireAll, "require-all", false, "Return only posts whose tag list contains every required tag (fetch-and-confirm)")
	cmd.Flags().StringVar(&flagMonth, "month", "", "Optional YYYY-MM filter applied downstream by 'posts get'")
	cmd.Flags().IntVar(&flagLimit, "limit", 22, "Max results after merge/intersect")
	return cmd
}

// splitTags parses --tags into a normalized list. Leading "#" is
// stripped from each tag so callers can pass either "#협찬" or "협찬"
// without changing semantics.
func splitTags(raw string) []string {
	out := make([]string, 0)
	seen := make(map[string]bool)
	for _, t := range strings.Split(raw, ",") {
		t = strings.TrimSpace(t)
		t = strings.TrimPrefix(t, "#")
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

// runHashtagUnion fans one SERP per tag and merges by URL.
//
// Behavior unchanged from Pass 1. Each hit's hashtag_match tracks the
// tag(s) that brought it into the union.
func runHashtagUnion(ctx context.Context, c *client.Client, tags []string, where string, limit int) ([]serpparse.SearchResult, error) {
	type urlState struct {
		result    serpparse.SearchResult
		tagsMatch map[string]bool
	}
	state := make(map[string]*urlState)
	order := make([]string, 0)
	for _, tag := range tags {
		hits, err := fetchTagSERP(ctx, c, tag, where)
		if err != nil {
			return nil, fmt.Errorf("fetch tag %q: %w", tag, err)
		}
		for _, hit := range hits {
			if st, ok := state[hit.URL]; ok {
				st.tagsMatch[tag] = true
				continue
			}
			state[hit.URL] = &urlState{
				result:    hit,
				tagsMatch: map[string]bool{tag: true},
			}
			order = append(order, hit.URL)
		}
	}
	out := make([]serpparse.SearchResult, 0, len(order))
	for _, u := range order {
		st := state[u]
		matched := make([]string, 0, len(st.tagsMatch))
		for t := range st.tagsMatch {
			matched = append(matched, t)
		}
		sort.Strings(matched)
		st.result.HashtagMatch = strings.Join(matched, ",")
		st.result.Rank = len(out) + 1
		out = append(out, st.result)
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func cacheHashtagResults(ctx context.Context, results []serpparse.SearchResult) {
	if len(results) == 0 || cliutil.IsVerifyEnv() {
		return
	}
	db, err := store.OpenWithContext(ctx, defaultDBPath("naver-blog-pp-cli"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "warn: hashtag cache hydration skipped: opening db: %v\n", err)
		return
	}
	defer db.Close()
	for _, hit := range results {
		if strings.TrimSpace(hit.Title) == "" {
			continue
		}
		obj := map[string]any{
			"id":        hit.URL,
			"url":       hit.URL,
			"blog_id":   hit.BlogID,
			"log_no":    hit.LogNo,
			"title":     hit.Title,
			"body_text": hit.Snippet,
		}
		if hit.HashtagMatch != "" {
			obj["tags"] = joinPostTags(strings.Split(hit.HashtagMatch, ","))
		}
		if err := upsertPostCacheObject(db, obj); err != nil {
			fmt.Fprintf(os.Stderr, "warn: hashtag cache hydration failed for %s: %v\n", hit.URL, err)
		}
	}
}

// runHashtagIntersection runs ONE joined SERP and then per-post
// gsTagName confirmation. Concurrency for the per-post fetches is
// capped via the client's adaptive limiter, which already paces
// requests at the global --rate-limit floor; here we cap goroutine
// count to 5 to avoid burst CPU on the HTML parse.
func runHashtagIntersection(ctx context.Context, c *client.Client, tags []string, where string, limit int) ([]serpparse.SearchResult, error) {
	if len(tags) == 0 {
		return nil, fmt.Errorf("at least one tag required")
	}
	// Build a "#tag1 #tag2 ..." query — Naver's space is AND.
	parts := make([]string, 0, len(tags))
	for _, t := range tags {
		parts = append(parts, "#"+t)
	}
	joined := strings.Join(parts, " ")
	hits, err := fetchJoinedTagSERP(ctx, c, joined, where)
	if err != nil {
		return nil, fmt.Errorf("fetch joined SERP: %w", err)
	}
	// Confirm via gsTagName for each hit.
	want := make(map[string]bool, len(tags))
	for _, t := range tags {
		want[strings.ToLower(strings.TrimPrefix(t, "#"))] = true
	}
	type confirmed struct {
		hit  serpparse.SearchResult
		tags []string
	}
	results, errs := cliutil.FanoutRun(
		ctx,
		hits,
		func(h serpparse.SearchResult) string { return h.URL },
		func(ctx context.Context, h serpparse.SearchResult) (*confirmed, error) {
			htmlBytes, err := getHTMLBytes(c, h.URL)
			if err != nil {
				return nil, fmt.Errorf("fetch post: %w", err)
			}
			meta, err := postparse.ParseMobilePost(htmlBytes)
			if err != nil {
				return nil, fmt.Errorf("parse post: %w", err)
			}
			postTags := make(map[string]bool, len(meta.Tags))
			for _, t := range meta.Tags {
				postTags[strings.ToLower(strings.TrimPrefix(t, "#"))] = true
			}
			for need := range want {
				if !postTags[need] {
					return nil, nil // not a match, but not an error
				}
			}
			return &confirmed{hit: h, tags: meta.Tags}, nil
		},
		cliutil.WithConcurrency(5),
	)
	cliutil.FanoutReportErrors(os.Stderr, errs)
	out := make([]serpparse.SearchResult, 0, len(results))
	for _, r := range results {
		if r.Value == nil {
			continue
		}
		hit := r.Value.hit
		// Re-rank in confirmed order and surface the post's full tag
		// list as hashtag_match so the caller can see what the post
		// actually carries beyond the required set.
		hit.Rank = len(out) + 1
		hit.HashtagMatch = strings.Join(r.Value.tags, ",")
		out = append(out, hit)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

// fetchJoinedTagSERP issues one SERP request with all required tags
// joined into a single query string. Naver treats whitespace as AND so
// the SERP returns posts that contain all hashtags somewhere on the
// page; the caller is responsible for confirming via gsTagName that
// they're actually in the post's tag list (vs. body mentions).
func fetchJoinedTagSERP(ctx context.Context, c *client.Client, joinedQuery, where string) ([]serpparse.SearchResult, error) {
	q := url.Values{}
	q.Set("where", where)
	q.Set("query", joinedQuery)
	absURL := naverSERPBaseURL + "?" + q.Encode()
	html, err := getHTMLBytes(c, absURL)
	if err != nil {
		return nil, err
	}
	results, err := serpparse.ParseSERP(html, joinedQuery)
	if err != nil {
		return nil, err
	}
	_ = ctx
	return results, nil
}

func fetchTagSERP(ctx context.Context, c *client.Client, tag, where string) ([]serpparse.SearchResult, error) {
	q := url.Values{}
	q.Set("where", where)
	q.Set("query", "#"+tag)
	absURL := naverSERPBaseURL + "?" + q.Encode()
	html, err := getHTMLBytes(c, absURL)
	if err != nil {
		return nil, err
	}
	results, err := serpparse.ParseSERP(html, "#"+tag)
	if err != nil {
		return nil, err
	}
	_ = ctx
	return results, nil
}
