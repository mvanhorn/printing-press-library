// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

// Package graphql is the v2 search + author-archive fetch surface. It talks to
// Medium's OWN internal GraphQL endpoint (https://medium.com/_/graphql) — the
// same one medium.com's web app uses — with no API key, no RapidAPI, and no
// proxy. Queries are sent inline (Medium accepts them without a persisted-query
// hash), and the only headers required are the validated /_/graphql set
// (Content-Type/Accept/Origin/Referer), centralised in source.GraphQLHeaders.
//
// As with the rss and page packages, parsing is split from fetching on purpose:
// ParseSearch / ParseAuthorArchive are pure functions over a single GraphQL
// RESPONSE (the seam the hermetic tests exercise against saved fixtures), and
// Source.Search / Source.AuthorArchive are the thin network wrappers that
// paginate and hand each page's bytes to the parser. That split keeps
// `go test ./...` offline-green.
//
// Graceful degradation: Medium can change or remove this internal surface at
// any time (it is unversioned and undocumented). When a request fails — non-200,
// transport error, GraphQL "errors" block, or an unparseable body — the source
// returns source.ErrSurfaceUnavailable (wrapping the underlying cause) so the
// Resolver can fall through and the command layer can print one clear, typed
// message instead of panicking. feed/read, which do not use GraphQL, are
// unaffected.
package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/internal/source"
)

// Endpoint is Medium's internal GraphQL endpoint. Inline queries are accepted;
// nothing rotates (validated live in Gate 0 / Phase 1B).
const Endpoint = "https://medium.com/_/graphql"

// searchPageLimit is the items-per-page Medium's search uses (matches the oracle
// ranking; do NOT send writtenByHighQualityUser — omitting it matches exactly).
const searchPageLimit = 10

// archivePageLimit is the homepagePostsConnection page size validated live.
const archivePageLimit = 25

// maxSearchPages and maxArchivePages are hard iteration ceilings. The
// cursor/page loops stop naturally when pagingInfo.next is null, but an
// unexpected server response that always returns a next cursor would otherwise
// spin forever; these caps are the safety net the spec requires.
const (
	maxSearchPages  = 50
	maxArchivePages = 80
)

// SearchQuery is the confirmed inline search query (page-based pagination, page
// is 0-indexed). Kept verbatim from the build spec / live-validated probe.
const SearchQuery = `query SearchQuery($query:String!,$pagingOptions:SearchPagingOptions!){search(query:$query){... on Search{posts(pagingOptions:$pagingOptions){__typename ... on SearchCommonResult{pagingInfo{next{page limit}}} ... on SearchPost{items{... on Post{id title firstPublishedAt creator{id name username} latestPublishedVersion}}}}}}}`

// AuthorArchiveQuery is the confirmed inline author-archive query (cursor
// pagination via paging.from), widened to also fetch the per-post engagement
// fields author-compare reports — clapCount, voterCount, readingTime, wordCount,
// postResponses.count, and tags. These ride along in the same response (zero
// extra requests) and are anonymous/Tier-0, even for member-locked posts.
//
// Widening trades a larger surface for the data: if Medium ever renames/removes
// one of these fields the server rejects the whole query. AuthorArchive guards
// against that by falling back to authorArchiveQueryMinimal (below) on a query
// rejection, so core archiving never breaks — it just loses engagement that run.
const AuthorArchiveQuery = `query UA($id:ID!,$paging:PagingOptions!){user(id:$id){id name homepagePostsConnection(paging:$paging,includeDistributedResponses:true){posts{id title firstPublishedAt clapCount voterCount readingTime wordCount postResponses{count} tags{normalizedTagSlug} creator{id username}}pagingInfo{next{from limit}}}}}`

// authorArchiveQueryMinimal is the original, minimal author-archive query (only
// id/title/date/creator). It is the resilient fallback: if the engagement-widened
// AuthorArchiveQuery is rejected by the server (a changed/removed engagement
// field), AuthorArchive re-issues this stable subset so mirroring still succeeds.
// Same operation name ("UA") and variable signature as AuthorArchiveQuery, so the
// same do() call and variables work unchanged.
const authorArchiveQueryMinimal = `query UA($id:ID!,$paging:PagingOptions!){user(id:$id){id name homepagePostsConnection(paging:$paging,includeDistributedResponses:true){posts{id title firstPublishedAt creator{id username}}pagingInfo{next{from limit}}}}}`

// Source is the GraphQL implementation of source.Source. It serves Search and
// AuthorArchive and declares every other capability false (returning
// source.ErrUnsupported if mis-dispatched), per the contract.
type Source struct {
	client  *http.Client
	cookies source.Cookies
}

// New returns a GraphQL Source over the given HTTP client. A nil client is
// acceptable for tests that only exercise the pure parsers (Search /
// AuthorArchive will lazily build a default Surf transport when actually called
// over the wire).
func New(client *http.Client) *Source {
	return &Source{client: client}
}

// WithCookies returns a copy of the source that attaches the given Tier-1
// session cookies on each GraphQL request. The zero-value source stays fully
// anonymous (Tier 0); search/author-archive work anonymously either way.
func (s *Source) WithCookies(c source.Cookies) *Source {
	cp := *s
	cp.cookies = c
	return &cp
}

// Name identifies this source in resolver diagnostics.
func (s *Source) Name() string { return "graphql" }

// Capabilities advertises Search and AuthorArchive only.
func (s *Source) Capabilities() source.Caps {
	return source.Caps{Search: true, AuthorArchive: true}
}

func (s *Source) httpClient() *http.Client {
	if s.client != nil {
		return s.client
	}
	return source.NewHTTPClient(60 * time.Second)
}

// Feed is unsupported by the GraphQL source.
func (s *Source) Feed(ctx context.Context, ref string) ([]source.PostSummary, error) {
	return nil, source.ErrUnsupported
}

// ReadArticle is unsupported by the GraphQL source.
func (s *Source) ReadArticle(ctx context.Context, idOrURL string) (*source.Article, error) {
	return nil, source.ErrUnsupported
}

// ---- Search ----------------------------------------------------------------

// searchResponse mirrors the SearchQuery response shape. Only the fields the
// parser projects are decoded; everything else is ignored.
type searchResponse struct {
	Data struct {
		Search struct {
			Posts struct {
				PagingInfo struct {
					Next *struct {
						Page int `json:"page"`
					} `json:"next"`
				} `json:"pagingInfo"`
				Items []searchPost `json:"items"`
			} `json:"posts"`
		} `json:"search"`
	} `json:"data"`
	Errors []gqlError `json:"errors"`
}

type searchPost struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	FirstPublishedAt int64  `json:"firstPublishedAt"`
	Creator          struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Username string `json:"username"`
	} `json:"creator"`
}

type gqlError struct {
	Message string `json:"message"`
}

// ParseSearch is the pure search-response parser the hermetic tests exercise. It
// decodes one SearchQuery response page into normalized PostSummaries plus the
// next page index. nextPage is -1 when there is no next page (pagingInfo.next
// null), mirroring the "stop when next is null" pagination contract.
//
// A GraphQL "errors" block (Medium changed the surface) is a hard, typed
// failure: it returns ErrSurfaceUnavailable so callers degrade gracefully.
func ParseSearch(body []byte) (items []source.PostSummary, nextPage int, err error) {
	var r searchResponse
	if e := json.Unmarshal(body, &r); e != nil {
		return nil, -1, fmt.Errorf("graphql: decoding search response: %w", surfaceErr(e))
	}
	if len(r.Errors) > 0 {
		return nil, -1, fmt.Errorf("graphql: search returned errors: %s: %w", r.Errors[0].Message, source.ErrSurfaceUnavailable)
	}
	out := make([]source.PostSummary, 0, len(r.Data.Search.Posts.Items))
	for _, p := range r.Data.Search.Posts.Items {
		if p.ID == "" {
			continue
		}
		out = append(out, source.PostSummary{
			ID:          p.ID,
			Title:       p.Title,
			Author:      p.Creator.Name,
			AuthorID:    p.Creator.ID,
			Username:    p.Creator.Username,
			URL:         articleURL(p.ID),
			PublishedAt: epochMillis(p.FirstPublishedAt),
		})
	}
	next := -1
	if r.Data.Search.Posts.PagingInfo.Next != nil {
		next = r.Data.Search.Posts.PagingInfo.Next.Page
	}
	return out, next, nil
}

// Search runs the search query, paginating page-by-page until the requested
// limit is reached, pagingInfo.next is null, a page returns no items, or the
// hard page cap is hit. limit <= 0 means "all pages" (bounded by the hard page
// cap), mirroring AuthorArchive's max <= 0 semantics.
func (s *Source) Search(ctx context.Context, query string, limit int) ([]source.PostSummary, error) {
	var all []source.PostSummary
	page := 0
	for i := 0; i < maxSearchPages; i++ {
		vars := map[string]any{
			"query": query,
			"pagingOptions": map[string]any{
				"limit": searchPageLimit,
				"page":  page,
			},
		}
		body, err := s.do(ctx, "SearchQuery", SearchQuery, vars)
		if err != nil {
			return nil, err
		}
		items, next, err := ParseSearch(body)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if limit > 0 && len(all) >= limit {
			all = all[:limit]
			break
		}
		if len(items) == 0 || next < 0 {
			break
		}
		page = next
	}
	return all, nil
}

// ---- AuthorArchive ---------------------------------------------------------

// archiveResponse mirrors the AuthorArchiveQuery response shape.
type archiveResponse struct {
	Data struct {
		User struct {
			ID                      string `json:"id"`
			Name                    string `json:"name"`
			HomepagePostsConnection struct {
				Posts      []archivePost `json:"posts"`
				PagingInfo struct {
					Next *struct {
						From string `json:"from"`
					} `json:"next"`
				} `json:"pagingInfo"`
			} `json:"homepagePostsConnection"`
		} `json:"user"`
	} `json:"data"`
	Errors []gqlError `json:"errors"`
}

type archivePost struct {
	ID               string  `json:"id"`
	Title            string  `json:"title"`
	FirstPublishedAt int64   `json:"firstPublishedAt"`
	ClapCount        int     `json:"clapCount"`
	VoterCount       int     `json:"voterCount"`
	ReadingTime      float64 `json:"readingTime"`
	WordCount        int     `json:"wordCount"`
	PostResponses    struct {
		Count int `json:"count"`
	} `json:"postResponses"`
	Tags []struct {
		Slug string `json:"normalizedTagSlug"`
	} `json:"tags"`
	Creator struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	} `json:"creator"`
}

// errGraphQLRejected marks a response that arrived intact (HTTP 200, valid JSON)
// but carried a GraphQL "errors" block — the server rejected the QUERY itself
// (e.g. an unknown field after Medium renamed one). It wraps ErrSurfaceUnavailable
// so existing callers still degrade gracefully, while letting AuthorArchive tell
// "query rejected" (fall back to the minimal query) apart from a transport/HTTP
// outage (where no fallback would help).
var errGraphQLRejected = fmt.Errorf("graphql: query rejected by server (changed/removed field?): %w", source.ErrSurfaceUnavailable)

// ParseAuthorArchive is the pure author-archive-response parser the hermetic
// tests exercise. It decodes one homepagePostsConnection page into normalized
// PostSummaries plus the next cursor. nextFrom is "" when there is no next page
// (pagingInfo.next null), mirroring the "stop when next is null" contract.
//
// authorName surfaces the user.name field so the caller can label results; it is
// "" when absent. A GraphQL "errors" block is a typed ErrSurfaceUnavailable.
func ParseAuthorArchive(body []byte) (items []source.PostSummary, nextFrom string, authorName string, err error) {
	var r archiveResponse
	if e := json.Unmarshal(body, &r); e != nil {
		return nil, "", "", fmt.Errorf("graphql: decoding author-archive response: %w", surfaceErr(e))
	}
	if len(r.Errors) > 0 {
		return nil, "", "", fmt.Errorf("graphql: author-archive returned errors: %s: %w", r.Errors[0].Message, errGraphQLRejected)
	}
	u := r.Data.User
	out := make([]source.PostSummary, 0, len(u.HomepagePostsConnection.Posts))
	for _, p := range u.HomepagePostsConnection.Posts {
		if p.ID == "" {
			continue
		}
		// Flatten tag slugs, skipping empties. A minimal-query (fallback)
		// response has no tags, so this is nil there — fine.
		var tags []string
		for _, t := range p.Tags {
			if t.Slug != "" {
				tags = append(tags, t.Slug)
			}
		}
		out = append(out, source.PostSummary{
			ID:          p.ID,
			Title:       p.Title,
			Author:      u.Name,
			AuthorID:    p.Creator.ID,
			Username:    p.Creator.Username,
			URL:         articleURL(p.ID),
			PublishedAt: epochMillis(p.FirstPublishedAt),
			Tags:        tags,
			Claps:       p.ClapCount,
			Voters:      p.VoterCount,
			Responses:   p.PostResponses.Count,
			ReadingTime: p.ReadingTime,
			WordCount:   p.WordCount,
		})
	}
	next := ""
	if u.HomepagePostsConnection.PagingInfo.Next != nil {
		next = u.HomepagePostsConnection.PagingInfo.Next.From
	}
	return out, next, u.Name, nil
}

// AuthorArchive walks a writer's homepage post connection by cursor, paginating
// until the requested max is reached, pagingInfo.next is null, a page returns no
// posts, or the hard page cap is hit. max <= 0 means "all pages" (bounded by the
// cap). userID must be a Medium user id (the resolver/CLI handles id resolution
// upstream).
func (s *Source) AuthorArchive(ctx context.Context, userID string, max int) ([]source.PostSummary, error) {
	var all []source.PostSummary
	from := ""
	query := AuthorArchiveQuery // engagement-widened; may fall back to the minimal query
	for i := 0; i < maxArchivePages; i++ {
		paging := map[string]any{"limit": archivePageLimit}
		if from != "" {
			paging["from"] = from
		}
		vars := map[string]any{"id": userID, "paging": paging}

		body, err := s.do(ctx, "UA", query, vars)
		if err != nil {
			return nil, err
		}
		items, next, _, err := ParseAuthorArchive(body)
		if err != nil {
			// If the server REJECTED the widened query itself (e.g. Medium
			// renamed an engagement field), fall back to the stable minimal
			// query — for this page and every subsequent one — so core
			// mirroring still succeeds (engagement just comes back zero). This
			// fires ONLY on a query rejection: a transport/HTTP outage already
			// returned from do() above, so a real outage is never retried (and
			// the per-page request count is never doubled). Pages already
			// fetched before the rejection keep their engagement; only this page
			// onward degrades — the mix across pages is honest, not all-or-nothing.
			if errors.Is(err, errGraphQLRejected) && query != authorArchiveQueryMinimal {
				query = authorArchiveQueryMinimal
				body, err = s.do(ctx, "UA", query, vars)
				if err != nil {
					return nil, err
				}
				items, next, _, err = ParseAuthorArchive(body)
			}
			if err != nil {
				return nil, err
			}
		}
		all = append(all, items...)
		if max > 0 && len(all) >= max {
			all = all[:max]
			break
		}
		if len(items) == 0 || next == "" {
			break
		}
		from = next
	}
	return all, nil
}

// ---- transport -------------------------------------------------------------

// do posts a single GraphQL operation and returns the raw response body. Any
// transport error or non-200 status collapses to ErrSurfaceUnavailable so the
// Resolver degrades gracefully rather than surfacing a raw transport error.
func (s *Source) do(ctx context.Context, opName, query string, vars map[string]any) ([]byte, error) {
	payload := map[string]any{
		"operationName": opName,
		"query":         query,
		"variables":     vars,
	}
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("graphql: marshaling request: %w", surfaceErr(err))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, Endpoint, bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("graphql: building request: %w", surfaceErr(err))
	}
	source.GraphQLHeaders(req)
	source.AttachCookies(req, s.cookies)
	resp, err := s.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("graphql: posting to %s: %w", Endpoint, surfaceErr(err))
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("graphql: %s returned HTTP %d: %w", Endpoint, resp.StatusCode, source.ErrSurfaceUnavailable)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("graphql: reading body: %w", surfaceErr(err))
	}
	return body, nil
}

// ---- helpers ---------------------------------------------------------------

// surfaceErr wraps a cause behind ErrSurfaceUnavailable while preserving the
// root cause for operators (errors.Is(err, source.ErrSurfaceUnavailable) holds,
// and %v on the result still shows the underlying message).
func surfaceErr(cause error) error {
	return fmt.Errorf("%v: %w", cause, source.ErrSurfaceUnavailable)
}

// articleURL builds the canonical /p/<id> short link for an article id. The
// read command resolves this (or a bare id) back to a full article.
func articleURL(id string) string {
	if id == "" {
		return ""
	}
	return "https://medium.com/p/" + id
}

// epochMillis converts a Medium firstPublishedAt epoch-millis to UTC time. Zero
// or negative input yields the zero time (consistent with the rss/page sources).
func epochMillis(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms).UTC()
}
