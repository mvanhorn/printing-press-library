// Package serpparse extracts post hits from a Naver mobile search
// results page (m.search.naver.com/search.naver?where=m_view&query=…).
//
// The SERP's exact class names shift between A/B test buckets, but
// every hit always carries an m.blog.naver.com/<id>/<n> URL — that
// stable URL pattern is the load-bearing anchor of this extractor.
//
// As of 2026-05 Naver's mobile SERP renders each blog hit inside a
// `<div data-template-id="ugcItem">` block. Inside the block the title
// lives in a span whose class list contains `sds-comps-text-type-headline1`
// and the body snippet lives in a span whose class list contains
// `sds-comps-text-type-body1`. Both wrap query highlights in `<mark>` —
// we strip all tags so the caller sees the plain text. Legacy fallback
// (api_txt_lines total_tit / dsc_txt + raw anchor text) is still tried
// in case Naver routes a request to an older A/B bucket.
package serpparse

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/lib/naverurl"
)

// SearchResult is a single SERP hit. Rank is 1-indexed in the order
// the URLs appeared on the page. HashtagMatch is set by callers that
// fan a hashtag query into multiple per-tag SERPs and merge results;
// it stays empty for keyword searches.
type SearchResult struct {
	Rank         int    `json:"rank"`
	URL          string `json:"url"`
	BlogID       string `json:"blog_id"`
	LogNo        string `json:"log_no"`
	Title        string `json:"title,omitempty"`
	Snippet      string `json:"snippet,omitempty"`
	HashtagMatch string `json:"hashtag_match,omitempty"`
}

// reMobileBlogURL matches the canonical mobile post URL pattern that
// every SERP hit carries. The "12,}" digit count is empirical: Naver
// log numbers are 12-13 digits as of 2026. We accept 12 or more so
// future inflation doesn't break the matcher.
var reMobileBlogURL = regexp.MustCompile(`https://m\.blog\.naver\.com/[\w-]+/\d{12,}`)

// reUGCItem marks the start of a single SERP hit block in the modern
// "sds-comps" layout. Each occurrence is the anchor for one m.blog
// URL + title + snippet trio.
var reUGCItem = regexp.MustCompile(`data-template-id="ugcItem"`)

// reHeadline1 / reBody1 pull the inner HTML of the first title/snippet
// span inside an ugcItem block. The `(?s)` flag lets `.` match newlines
// — Naver's HTML is mostly one giant line, but defensive against
// future minifier changes. We capture inner HTML and let stripTags
// drop `<mark>` highlights afterward.
var (
	reHeadline1 = regexp.MustCompile(`(?s)class="[^"]*sds-comps-text-type-headline1[^"]*"[^>]*>(.*?)</span>`)
	reBody1     = regexp.MustCompile(`(?s)class="[^"]*sds-comps-text-type-body1[^"]*"[^>]*>(.*?)</span>`)
)

// reTotalTitle and reDscText preserve the legacy api_txt_lines markup
// in case Naver routes a request to an older A/B bucket. These fire
// only when the modern selectors return empty.
var (
	reTotalTitle = regexp.MustCompile(`class=["']api_txt_lines\s+total_tit[^"']*["'][^>]*>\s*([^<\s][^<]*)<`)
	reDscText    = regexp.MustCompile(`class=["']api_txt_lines\s+dsc_txt[^"']*["'][^>]*>\s*([^<\s][^<]*)<`)
	// Anchor-text fallback for hits whose title we can't pull from a
	// .total_tit span. Naver wraps the URL in <a ...>title text</a>;
	// we capture the text immediately after the open tag through the
	// next <.
	reAnchorTitleTpl = `<a[^>]*href=["']%s["'][^>]*>\s*([^<\s][^<]*)`
)

// reTagStrip removes any `<...>` tag from a string. Used to drop
// `<mark>` highlights without losing the text they wrap.
var reTagStrip = regexp.MustCompile(`<[^>]+>`)

// ParseSERP extracts every hit from the SERP HTML. query is unused by
// the parser itself — accepted so the signature matches the spec and
// so a future implementation can do query-aware ranking. Returns an
// empty slice (not nil) when no URLs match, plus a non-nil error only
// when the input is structurally unusable.
func ParseSERP(htmlBytes []byte, query string) ([]SearchResult, error) {
	if len(htmlBytes) == 0 {
		return nil, fmt.Errorf("empty HTML")
	}
	src := string(htmlBytes)

	// Strategy A: walk ugcItem blocks (modern sds-comps layout).
	//
	// Block-scoped extraction is more reliable than ordinal pairing
	// across the whole page because the page also contains shopping
	// cards, "related searches" rails, and webdoc hits that have
	// their own headline1/body1 spans. Pairing the Nth blog URL
	// with the Nth title across all of those will misalign.
	resultsByURL := make(map[string]SearchResult)
	order := make([]string, 0)
	itemStarts := reUGCItem.FindAllStringIndex(src, -1)
	for i, m := range itemStarts {
		end := len(src)
		if i+1 < len(itemStarts) {
			end = itemStarts[i+1][0]
		}
		block := src[m[0]:end]
		urlMatch := reMobileBlogURL.FindString(block)
		if urlMatch == "" {
			continue
		}
		blogID, logNo, ok := naverurl.CanonicalKey(urlMatch)
		if !ok {
			continue
		}
		if _, dup := resultsByURL[urlMatch]; dup {
			continue
		}
		r := SearchResult{
			URL:    urlMatch,
			BlogID: blogID,
			LogNo:  logNo,
		}
		if tm := reHeadline1.FindStringSubmatch(block); tm != nil {
			r.Title = cleanInner(tm[1])
		}
		if sm := reBody1.FindStringSubmatch(block); sm != nil {
			r.Snippet = cleanInner(sm[1])
		}
		resultsByURL[urlMatch] = r
		order = append(order, urlMatch)
	}

	// Strategy B fallback: for any URLs that appear on the page but
	// weren't inside an ugcItem block (legacy bucket or unexpected
	// layout), pull them out and apply the old api_txt_lines / anchor
	// fallback. This preserves the original behaviour for buckets
	// that still render the older shell.
	allURLs := reMobileBlogURL.FindAllStringIndex(src, -1)
	if len(resultsByURL) == 0 && len(allURLs) > 0 {
		titleSpans := extractMatchedText(src, reTotalTitle)
		snippetSpans := extractMatchedText(src, reDscText)
		seen := make(map[string]bool)
		for i, idx := range allURLs {
			urlStr := src[idx[0]:idx[1]]
			if seen[urlStr] {
				continue
			}
			seen[urlStr] = true
			blogID, logNo, ok := naverurl.CanonicalKey(urlStr)
			if !ok {
				continue
			}
			r := SearchResult{URL: urlStr, BlogID: blogID, LogNo: logNo}
			if i < len(titleSpans) {
				r.Title = titleSpans[i]
			}
			if r.Title == "" {
				r.Title = anchorTitleFor(src, urlStr)
			}
			if i < len(snippetSpans) {
				r.Snippet = snippetSpans[i]
			}
			resultsByURL[urlStr] = r
			order = append(order, urlStr)
		}
	} else {
		// Modern path took some hits, but the page may also contain
		// loose URLs outside any ugcItem (sub-block "more from this
		// blog" rails). Skip those — they're duplicates of hits we
		// already captured.
		for _, idx := range allURLs {
			urlStr := src[idx[0]:idx[1]]
			if _, ok := resultsByURL[urlStr]; ok {
				continue
			}
			// Anchor-only hits: keep them but without title/snippet
			// to match the legacy contract that "URL always wins".
			blogID, logNo, ok := naverurl.CanonicalKey(urlStr)
			if !ok {
				continue
			}
			resultsByURL[urlStr] = SearchResult{URL: urlStr, BlogID: blogID, LogNo: logNo}
			order = append(order, urlStr)
		}
	}

	results := make([]SearchResult, 0, len(order))
	rank := 0
	for _, u := range order {
		r := resultsByURL[u]
		rank++
		r.Rank = rank
		results = append(results, r)
	}
	return results, nil
}

// cleanInner strips embedded HTML (e.g., `<mark>` highlights), decodes
// entities, and trims whitespace.
func cleanInner(inner string) string {
	stripped := reTagStrip.ReplaceAllString(inner, "")
	return html.UnescapeString(strings.TrimSpace(stripped))
}

// extractMatchedText returns the capture-group-1 strings for every
// match of re against src, in DOM order. HTML entities are decoded so
// the caller sees "협찬 & 칠리" instead of "협찬 &amp; 칠리".
func extractMatchedText(src string, re *regexp.Regexp) []string {
	matches := re.FindAllStringSubmatch(src, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) < 2 {
			out = append(out, "")
			continue
		}
		out = append(out, html.UnescapeString(strings.TrimSpace(m[1])))
	}
	return out
}

// anchorTitleFor scans for <a href="<url>">...title text</a> and
// returns the inner text. Fallback when api_txt_lines.total_tit is
// absent — some Naver SERP buckets render the title only as anchor
// text. Returns "" if no match. The URL is regex-escaped because URLs
// contain dots and (rarely) other metacharacters.
func anchorTitleFor(src, urlStr string) string {
	pattern := fmt.Sprintf(reAnchorTitleTpl, regexp.QuoteMeta(urlStr))
	re, err := regexp.Compile(pattern)
	if err != nil {
		return ""
	}
	m := re.FindStringSubmatch(src)
	if m == nil {
		return ""
	}
	return html.UnescapeString(strings.TrimSpace(m[1]))
}
