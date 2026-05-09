// Cluster posts parser: extracts the structured `posts` array from the
// /ai/<clusterUrlId> page's RSC payload, then enriches each post with
// the body text + media URLs + repost-context that the page renders in
// the DOM.
//
// Two layers per post:
//
//  1. **Structured metadata** lives in the `posts` JSON array embedded
//     in self.__next_f.push([1, "..."]) chunks: post_x_id, posted_at,
//     post_type (tweet|reply|quote|retweet), author_username,
//     author_display_name, author_category, author_profile_image_url,
//     author_rank.
//
//  2. **Rendered content** lives in the static HTML the server ships:
//     the tweet body text in <p class="wrap-anywhere ..."> elements,
//     image hrefs in <a href="https://pbs.twimg.com/media/...">
//     anchors, and repost-context chips with @HANDLE / "Reposted from"
//     icons. Bodies are correlated to posts by walking from the X-status
//     anchor (<a href="https://x.com/<user>/status/<id>" target="_blank">)
//     to the next such anchor (or end of document) and scanning that
//     window. Some posts are lazy-loaded behind Digg's "EXPAND DATA"
//     button and have no body in the default render — those keep
//     `body: nil` and `body_loaded: false` rather than being dropped.
//
// Tolerances:
//   - All RSC fields default to zero/empty when absent.
//   - Malformed RSC chunks are skipped; valid records are still returned
//     alongside an error wrapping the bad indexes.
//   - DOM correlation is best-effort: when the X anchor for a post can't
//     be located, body and media stay empty but the post is still
//     returned with its structured metadata.
package diggparse

import (
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"sort"
	"strings"
)

// ClusterPostAuthor is the author block embedded in a cluster post.
type ClusterPostAuthor struct {
	Username        string `json:"username"`
	DisplayName     string `json:"display_name,omitempty"`
	Category        string `json:"category,omitempty"`
	Rank            int    `json:"rank,omitempty"`
	ProfileImageURL string `json:"profile_image_url,omitempty"`
}

// RepostContext captures who reposted whose post. Surfaces the chip
// pair on retweet/quote rows: e.g. `@DanielleFong reposting @himanshustwts`.
type RepostContext struct {
	RepostingHandle string `json:"reposting_handle"`
	OriginalHandle  string `json:"original_handle"`
}

// ClusterPost is one X post attached to a /ai/<clusterUrlId> story.
//
// Body and BodyLoaded are JSON-pointer-shaped so a missing body shows
// as `null` rather than an empty string — important for callers that
// distinguish "tweet has no text" from "tweet body wasn't rendered."
// Media URLs are deduplicated within a post and ordered by first
// appearance in the DOM. RepostContext is nil except on retweets/quotes.
type ClusterPost struct {
	PostXID    string             `json:"post_x_id"`
	PostType   string             `json:"post_type,omitempty"`
	PostedAt   string             `json:"posted_at,omitempty"`
	Author     ClusterPostAuthor  `json:"author"`
	XURL       string             `json:"xUrl,omitempty"`
	Body       *string            `json:"body"`
	BodyLoaded bool               `json:"body_loaded"`
	MediaURLs  []string           `json:"media_urls"`
	Repost     *RepostContext     `json:"repost_context"`
	RawJSON    json.RawMessage    `json:"-"`
}

// rscPost mirrors the JSON shape inside the RSC `posts` array. Lives
// privately so the public ClusterPost type can carry CLI-side additions
// (xUrl mint, body+media correlation, repost context) without polluting
// the upstream contract.
type rscPost struct {
	PostXID               string `json:"post_x_id"`
	PostedAt              string `json:"posted_at"`
	PostType              string `json:"post_type"`
	AuthorUsername        string `json:"author_username"`
	AuthorDisplayName     string `json:"author_display_name"`
	AuthorCategory        string `json:"author_category"`
	AuthorProfileImageURL string `json:"author_profile_image_url"`
	AuthorRank            int    `json:"author_rank"`
}

// ExtractClusterPosts walks the decoded RSC stream and returns every
// post object found. The marker is `"post_x_id":"` — distinctive enough
// in /ai/<clusterUrlId> payloads that we don't get false positives.
// Returns the slice and, when chunks failed to decode, an error
// wrapping the bad indexes; valid records are still returned alongside.
func ExtractClusterPosts(decoded string) ([]rscPost, error) {
	objs := scanObjectsContaining(decoded, `"post_x_id":"`)
	out := make([]rscPost, 0, len(objs))
	seen := make(map[string]bool, len(objs))
	var badIdxs []int
	for i, raw := range objs {
		var p rscPost
		if err := json.Unmarshal(raw, &p); err != nil {
			badIdxs = append(badIdxs, i)
			continue
		}
		if p.PostXID == "" || p.AuthorUsername == "" {
			// scanObjectsContaining captured something with the marker
			// but the parent envelope had a different shape. Skip.
			continue
		}
		if seen[p.PostXID] {
			continue
		}
		seen[p.PostXID] = true
		out = append(out, p)
	}
	if len(badIdxs) > 0 {
		return out, fmt.Errorf("cluster posts: %d malformed RSC chunk(s) at indexes %v", len(badIdxs), badIdxs)
	}
	return out, nil
}

// xAnchorRE finds <a href="https://x.com/<user>/status/<id>" target="_blank">.
// The trailing target="_blank" guard discriminates these "go to X" anchors
// from inline /status/ URLs that appear elsewhere in body text and from
// internal Digg-hosted /u/x/<handle> profile links.
var xAnchorRE = regexp.MustCompile(`<a href="https://x\.com/[^/]+/status/(\d+)" target="_blank"`)

// bodyStrictRE matches the prominent "Original post" body class used in
// the fieldset render. Single match per page (Digg renders only the top
// post in this style); fallback for the other posts uses bodyCompactRE.
var bodyStrictRE = regexp.MustCompile(`<p class="wrap-anywhere whitespace-pre-wrap font-sans text-base leading-6 text-foreground">([^<]*)</p>`)

// bodyCompactRE matches the row-list "preview body" used in the
// expand-on-hover post list. The class string is a strict prefix so we
// don't accidentally pick up other wrap-anywhere variants.
var bodyCompactRE = regexp.MustCompile(`<p class="wrap-anywhere text-foreground">([^<]*)</p>`)

// mediaRE matches <a href="https://pbs.twimg.com/media/...">: anchor
// (not <img src=...>) — the page renders thumbnails as anchors so users
// can click through to the original image. Catching the anchor href is
// also more reliable because the same URL appears as a thumbnail bar at
// the page top, which uses inline style refs we'd otherwise have to
// manually exclude.
var mediaRE = regexp.MustCompile(`<a href="(https://pbs\.twimg\.com/media/[A-Za-z0-9_-]+\.[A-Za-z0-9]+)" target="_blank"`)

// repostFromRE marks the chip with the lucide-repeat-2 icon and the
// "Reposted from" or "Reposted by" aria label. Either label signals
// a chip-pair: the reposting handle's /u/x/ link comes BEFORE the
// chip; the original handle's /u/x/ link is rendered as a sibling
// chip just before the icon (so both handles appear in the
// pre-window, in order: reposter, then original).
//
// Quote rows render a similar pair with "Quoted from" — supported by
// the same regex via the alternation.
var repostFromRE = regexp.MustCompile(`aria-label="(Reposted from|Reposted by|Quoted from)"`)

// uxHandleRE pulls the handle out of an internal /u/x/<handle> link.
// The Digg page normalizes these to lowercase, so we must reuse the
// RSC author_username casing for the canonical forms.
var uxHandleRE = regexp.MustCompile(`href="/u/x/([A-Za-z0-9_]+)"`)

// xAnchorPosition returns the byte offset of the `<a href=...>` opening
// tag for the X-status anchor that points at the given post id, or -1
// if none. The offset is the start of the anchor element (NOT the
// inner /status/ substring) so anchor-window slicing across posts
// stays consistent with offsets pulled from xAnchorRE.FindAllStringIndex.
func xAnchorPosition(html []byte, postXID string) int {
	htmlStr := string(html)
	needle := fmt.Sprintf(`/status/%s" target="_blank"`, postXID)
	idx := strings.Index(htmlStr, needle)
	if idx < 0 {
		return -1
	}
	// Walk backward to the opening `<a` of the same tag. The needle
	// always lives inside an <a href="..."> attribute, so a bounded
	// reverse-scan is safe.
	start := idx
	for start > 0 && start > idx-300 {
		if strings.HasPrefix(htmlStr[start:], `<a href=`) {
			return start
		}
		start--
	}
	return idx
}

// attachDOMFields walks the DOM once, slices each post's render-window
// from its X anchor to the next post's X anchor (or end of document),
// and scans that window for body + media + repost context. The first
// post in DOM order also pulls from the strict-class body since Digg
// renders the top post in a fieldset distinct from the row list.
//
// Anchor resolution has two paths:
//
//  1. The exact `/status/<post_x_id>" target="_blank"` anchor — this
//     is the common case for tweets, replies, and quotes whose ID is
//     the same as the renderable URL.
//  2. For retweets the post_x_id refers to the retweet event, but the
//     page's X anchor points to the *original* post. In that case
//     we fall back to any anchor whose handle matches the post's
//     reposted/quoted handle (the chip pair gives us the original).
//     Failing that, retweets/quotes that look like the page's
//     "primary" post get the strict-class body anchor as a last
//     resort.
func attachDOMFields(htmlBytes []byte, posts []*ClusterPost) {
	type anchor struct {
		offset int
		idx    int // index in posts slice
	}
	htmlStr := string(htmlBytes)

	// Locate every post's anchor offset.
	anchors := make([]anchor, 0, len(posts))
	unresolved := make([]int, 0)
	for i, p := range posts {
		off := xAnchorPosition(htmlBytes, p.PostXID)
		if off < 0 {
			unresolved = append(unresolved, i)
			continue
		}
		anchors = append(anchors, anchor{offset: off, idx: i})
	}
	// For posts whose post_x_id has no anchor (typical retweet case
	// where the DOM points to the *original* post URL), search every
	// X-status anchor in the page and pick whichever one *isn't* claimed
	// by a resolved post yet. Retweets/quotes are usually the page's
	// only such row; we prefer the FIRST unclaimed anchor in DOM order
	// so the resulting render-window slicing still works.
	if len(unresolved) > 0 {
		claimed := make(map[int]bool, len(anchors))
		for _, a := range anchors {
			claimed[a.offset] = true
		}
		allAnchors := xAnchorRE.FindAllStringIndex(htmlStr, -1)
		for _, idx := range unresolved {
			for _, m := range allAnchors {
				off := m[0]
				if claimed[off] {
					continue
				}
				// Move the offset to where the inner /status anchor
				// matches our needle pattern (`/status/<id>" target=`).
				// Using m[0] is the start of `<a href="https://x.com/...`,
				// which is what xAnchorPosition returns for direct hits
				// minus the prefix offset; both work as window starts.
				claimed[off] = true
				anchors = append(anchors, anchor{offset: off, idx: idx})
				break
			}
		}
	}
	sort.Slice(anchors, func(i, j int) bool { return anchors[i].offset < anchors[j].offset })

	// For each post anchor, slice from its offset to the NEXT post's
	// anchor offset (or end of doc). That window contains the body
	// and media for *this* post, modulo a strict-class body which can
	// also live just BEFORE the anchor on the very first post.
	for i, a := range anchors {
		end := len(htmlStr)
		if i+1 < len(anchors) {
			end = anchors[i+1].offset
		}
		window := htmlStr[a.offset:end]
		p := posts[a.idx]

		// Body: try the strict (top "Original post") class first within
		// the window. If not found, fall back to the compact preview
		// class. If neither matches, body stays nil (body_loaded=false).
		//
		// Bodies are run through html.UnescapeString because Next.js SSR
		// HTML-encodes &, <, >, ", and ' in the rendered text. Without
		// decoding, callers see &amp; / &lt; / &gt; / &quot; / &#39;
		// instead of the original characters.
		if i == 0 {
			// The strict-class body for the top post may be rendered
			// just BEFORE its anchor (inside the fieldset). Scan from
			// the top of the document up to the next-anchor end so we
			// catch it.
			topWindow := htmlStr[:end]
			if m := bodyStrictRE.FindStringSubmatch(topWindow); len(m) > 1 {
				body := html.UnescapeString(m[1])
				p.Body = &body
				p.BodyLoaded = true
			}
		}
		if p.Body == nil {
			if m := bodyStrictRE.FindStringSubmatch(window); len(m) > 1 {
				body := html.UnescapeString(m[1])
				p.Body = &body
				p.BodyLoaded = true
			}
		}
		if p.Body == nil {
			if m := bodyCompactRE.FindStringSubmatch(window); len(m) > 1 {
				body := html.UnescapeString(m[1])
				p.Body = &body
				p.BodyLoaded = true
			}
		}

		// Media: every pbs.twimg href anchor inside the window, deduped
		// in first-appearance order. The capture is restricted to
		// `[A-Za-z0-9_-]+\.[A-Za-z0-9]+` so it can't contain a literal
		// `&amp;` query separator today, but we still decode entities
		// defensively in case the regex is broadened to accept query
		// strings later.
		seen := make(map[string]bool)
		for _, m := range mediaRE.FindAllStringSubmatch(window, -1) {
			if len(m) < 2 {
				continue
			}
			decoded := html.UnescapeString(m[1])
			if !seen[decoded] {
				seen[decoded] = true
				p.MediaURLs = append(p.MediaURLs, decoded)
			}
		}
	}

	// Repost / quote context: each chip renders TWO /u/x/<handle>
	// anchors immediately before the icon — the reposting handle's
	// chip first, then the original handle's chip, then the icon
	// span itself. We pull the LAST TWO /u/x/ hrefs in the
	// pre-chip window to extract the pair.
	//
	// Multiple chips can appear per cluster (compact row list +
	// expanded fieldset rendering both emit one). We dedupe by the
	// (reposting, original) handle pair and zip the remaining
	// distinct chips against retweet/quote-typed posts in DOM order.
	chipMatches := repostFromRE.FindAllStringIndex(htmlStr, -1)
	chips := make([]*RepostContext, 0, len(chipMatches))
	seenChips := make(map[string]bool, len(chipMatches))
	for _, m := range chipMatches {
		chipStart := m[0]
		preStart := chipStart - 1500
		if preStart < 0 {
			preStart = 0
		}
		preWin := htmlStr[preStart:chipStart]
		preHandles := uxHandleRE.FindAllStringSubmatch(preWin, -1)
		if len(preHandles) < 2 {
			continue
		}
		// The chip pair is the last two /u/x/ anchors before the icon.
		lastTwo := preHandles[len(preHandles)-2:]
		repost := &RepostContext{
			RepostingHandle: lastTwo[0][1],
			OriginalHandle:  lastTwo[1][1],
		}
		key := strings.ToLower(repost.RepostingHandle) + "->" + strings.ToLower(repost.OriginalHandle)
		if seenChips[key] {
			continue
		}
		seenChips[key] = true
		chips = append(chips, repost)
	}

	// Attach chips to retweet/quote posts in DOM order. When there's
	// just one retweet/quote on the page (the common case for cluster
	// pages), the single chip pair attaches to it cleanly. For multi-
	// retweet pages we associate by zip-order against the chip list.
	chipIdx := 0
	for _, a := range anchors {
		p := posts[a.idx]
		if p.PostType != "retweet" && p.PostType != "quote" {
			continue
		}
		if chipIdx >= len(chips) {
			break
		}
		repost := *chips[chipIdx]
		chipIdx++
		// Re-case from the RSC author_username when the reposting
		// handle matches: Digg lowercases /u/x/ paths but preserves
		// case in author_username. The original_handle stays
		// lowercase; consumers normalize it via X URL anyway.
		if strings.EqualFold(repost.RepostingHandle, p.Author.Username) {
			repost.RepostingHandle = p.Author.Username
		}
		p.Repost = &repost
	}
}

// ParseClusterPosts is the convenience entry for /ai/<clusterUrlId>.
// Decodes the RSC stream, extracts each post's structured metadata,
// then walks the DOM to attach body/media/repost-context. Returns the
// posts in the same order they appear in the RSC array (which mirrors
// the upstream "natural" ordering — usually chronological), so callers
// who want sort-by-rank or sort-by-time apply that themselves.
//
// On a partial parse (some chunks malformed but at least one valid
// record), returns the valid records along with an error wrapping the
// bad chunk indexes. On an empty parse (no `posts` array found at
// all), returns a typed error so callers can distinguish a structural
// drift from a genuinely empty cluster.
func ParseClusterPosts(html []byte) ([]ClusterPost, error) {
	decoded := DecodeRSC(html)
	if decoded == "" {
		return nil, fmt.Errorf("no RSC pushes found in cluster HTML (%d bytes); page shape may have changed", len(html))
	}
	rsc, err := ExtractClusterPosts(decoded)
	if len(rsc) == 0 {
		if err != nil {
			return nil, fmt.Errorf("cluster posts parse produced 0 records: %w", err)
		}
		return nil, fmt.Errorf("cluster posts parse produced 0 records; page shape may have changed")
	}

	// Promote to the public type (with empty body/media; attachDOMFields
	// fills them in next).
	out := make([]ClusterPost, len(rsc))
	ptrs := make([]*ClusterPost, len(rsc))
	for i, r := range rsc {
		cp := ClusterPost{
			PostXID:  r.PostXID,
			PostType: r.PostType,
			PostedAt: r.PostedAt,
			Author: ClusterPostAuthor{
				Username:        r.AuthorUsername,
				DisplayName:     r.AuthorDisplayName,
				Category:        r.AuthorCategory,
				Rank:            r.AuthorRank,
				ProfileImageURL: r.AuthorProfileImageURL,
			},
			MediaURLs: []string{},
		}
		if r.AuthorUsername != "" && r.PostXID != "" {
			cp.XURL = fmt.Sprintf("https://x.com/%s/status/%s", r.AuthorUsername, r.PostXID)
		}
		out[i] = cp
		ptrs[i] = &out[i]
	}

	attachDOMFields(html, ptrs)

	return out, err
}
