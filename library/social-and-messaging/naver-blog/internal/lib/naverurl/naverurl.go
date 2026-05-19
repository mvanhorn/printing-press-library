// Package naverurl canonicalizes Naver Blog post URLs.
//
// Naver Blog posts surface under three different URL shapes that all
// point at the same underlying content:
//
//   - https://blog.naver.com/<blog_id>/<log_no>           (desktop short form)
//   - https://m.blog.naver.com/<blog_id>/<log_no>         (mobile short form)
//   - https://blog.naver.com/PostView.naver?blogId=<id>&logNo=<n>
//
// CanonicalKey collapses all three to the same (blog_id, log_no) pair
// so the rest of the pipeline (engagement lookups, dedupe by URL,
// canary post fetches) can key off a single shape.
//
// Ports internal/skills/chilly-monthly-report/scripts/lib/naver_url.py
// behavior, including the host whitelist and the digit-only log_no
// guard.
package naverurl

import (
	"fmt"
	"net/url"
	"strings"
)

// allowedHosts pins canonicalization to Naver's two blog hosts. Any
// other host (including blog.naver.jp, naver.me shorteners,
// search.naver.com) returns ok=false — those are different shapes
// with different parsing rules and must be handled by their own
// code paths.
var allowedHosts = map[string]bool{
	"blog.naver.com":   true,
	"m.blog.naver.com": true,
}

// CanonicalKey returns the canonical (blog_id, log_no) for a Naver
// Blog post URL. Returns ok=false when the input is empty, lives on
// a non-Naver host, or doesn't match any of the three known shapes.
//
// The function is liberal about URL parsing — it accepts URLs without
// a scheme (prepends https://), with or without a trailing slash, and
// with URL-encoded path segments. It is strict about content: log_no
// must be all digits, blog_id must be non-empty, and the host must
// be one of the two whitelisted hosts.
func CanonicalKey(raw string) (blogID, logNo string, ok bool) {
	parsed, parts, parsedOK := parseNaverBlogURL(raw)
	if !parsedOK {
		return "", "", false
	}

	// PostView.naver?blogId=X&logNo=Y — the only Naver shape that
	// puts the IDs in the query string rather than the path.
	if len(parts) > 0 && parts[0] == "PostView.naver" {
		q := parsed.Query()
		blogID = strings.TrimSpace(q.Get("blogId"))
		logNo = strings.TrimSpace(q.Get("logNo"))
		if blogID == "" || logNo == "" || !isDigits(logNo) {
			return "", "", false
		}
		return blogID, logNo, true
	}

	// Short form: blog.naver.com/<blog_id>/<log_no> or
	// m.blog.naver.com/<blog_id>/<log_no>. The second path segment
	// must be all digits — guards against profile URLs like
	// blog.naver.com/<id>/categoryList which look superficially
	// similar but are not posts.
	if len(parts) >= 2 && isDigits(parts[1]) {
		return parts[0], parts[1], true
	}
	return "", "", false
}

// MobileURL returns the canonical mobile post URL. The mobile shape
// is the one we fetch HTML from because the desktop shape redirects
// through PostView.naver and the resulting body uses an older HTML
// structure that's harder to parse.
func MobileURL(blogID, logNo string) string {
	return fmt.Sprintf("https://m.blog.naver.com/%s/%s", url.PathEscape(blogID), url.PathEscape(logNo))
}

// PostViewURL returns the canonical desktop PostView URL. Naver's
// desktop page still includes static publish-date markup, but comment
// counts are now populated client-side and should be fetched through
// the cbox comment API instead of inferred from this HTML.
func PostViewURL(blogID, logNo string) string {
	q := url.Values{}
	q.Set("blogId", blogID)
	q.Set("logNo", logNo)
	return "https://blog.naver.com/PostView.naver?" + q.Encode()
}

// parseNaverBlogURL parses a raw URL string, prepending https:// when
// no scheme is present, and returns the parsed *url.URL plus the URL-
// decoded path parts. ok=false when the URL is empty, malformed, or
// lives on a non-whitelisted host.
func parseNaverBlogURL(raw string) (*url.URL, []string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil, false
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, nil, false
	}
	host := strings.ToLower(parsed.Host)
	if !allowedHosts[host] {
		return nil, nil, false
	}
	var parts []string
	for _, p := range strings.Split(parsed.Path, "/") {
		if p == "" {
			continue
		}
		// URL-decode each segment. Naver almost never URL-encodes
		// blog_id or log_no, but PathUnescape covers the rare case of
		// a blog_id with non-ASCII or reserved characters.
		decoded, err := url.PathUnescape(p)
		if err != nil {
			decoded = p
		}
		parts = append(parts, decoded)
	}
	return parsed, parts, true
}

// ExtractBlogID accepts any of:
//
//   - bare blog slug ("selly9401")  → returns "selly9401", ok=true
//   - mobile homepage URL ("https://m.blog.naver.com/selly9401") → returns "selly9401", ok=true
//   - desktop homepage URL ("https://blog.naver.com/selly9401") → same
//   - mobile/desktop post URL ("https://m.blog.naver.com/selly9401/224234460263") → returns "selly9401", ok=true
//   - PostView.naver?blogId=X&logNo=Y → returns blogID
//   - PostList.naver?blogId=X → returns blogID
//
// This is the bridge between user-pasted URLs (the natural input for
// blog-level commands like `blogs-info`, `blogs`, `categories`) and the
// internal API surface, which always works in blog_id slugs.
//
// Returns ok=false for empty input, non-Naver hosts, or unparseable
// shapes. Reserved Naver path segments at depth 1 (PostView.naver,
// PostList.naver, search results) are recognized and handled
// explicitly so we don't return "PostView.naver" as a blog ID.
func ExtractBlogID(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	// Bare slug fast-path: no slashes, no protocol, no question marks.
	// The blog_id slug rules are restrictive enough that this is safe:
	// Naver blog_ids are ASCII alphanumeric + underscore + hyphen, never
	// contain '.', '/', '?', or '@'.
	if !strings.ContainsAny(raw, "/?@") && !strings.Contains(raw, "://") {
		return raw, true
	}
	// Try the full post-URL path first — handles all three post shapes.
	if id, _, ok := CanonicalKey(raw); ok {
		return id, true
	}
	// Fall back to URL-only parsing for homepage / PostList shapes.
	parsed, parts, parsedOK := parseNaverBlogURL(raw)
	if !parsedOK {
		return "", false
	}
	// PostList.naver?blogId=X
	if len(parts) > 0 && (parts[0] == "PostList.naver" || parts[0] == "PostView.naver") {
		if id := strings.TrimSpace(parsed.Query().Get("blogId")); id != "" {
			return id, true
		}
		return "", false
	}
	// Homepage shape: exactly one path segment, which is the slug.
	// Avoid mistaking a reserved Naver path segment (e.g., "search.naver",
	// "SearchTag.naver", "BlogHome.naver") for a blog ID.
	if len(parts) == 1 && !strings.HasSuffix(parts[0], ".naver") {
		return parts[0], true
	}
	return "", false
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
