// Package postparse extracts metadata from Naver Blog post HTML.
//
// Two separate parse paths because Naver renders the same post under
// two different HTML shapes:
//
//   - ParseMobilePost: m.blog.naver.com/<id>/<n>. Modern OpenGraph
//     <meta> tags carry title, snippet, thumbnail. The body lives
//     under <div class="se-main-container">. Tags are inlined into a
//     gsTagName JS literal.
//
//   - ParsePostView: blog.naver.com/PostView.naver?... Desktop shape
//     used for publish-date extraction because the mobile shape renders
//     the date via JavaScript. Comment counts now come from cbox; the
//     legacy em parsing remains as a best-effort fallback only.
//
// The implementation prefers targeted regex over a full HTML parser
// because (1) the fields are unambiguous markers wrapped in stable
// tag shapes, and (2) Naver's HTML is large and full-parse latency
// adds up across batch fetches.
package postparse

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// PostMeta is the structured shape extracted from a mobile post HTML
// body. Fields are empty strings (not "unknown") when extraction
// fails — the caller decides whether to treat an empty field as an
// error or proceed with partial data.
type PostMeta struct {
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	ThumbnailURL string   `json:"thumbnail_url"`
	Tags         []string `json:"tags"`
	Images       []string `json:"images"`
	BodyHTML     string   `json:"body_html"`
	BodyText     string   `json:"body_text"`
}

// PostViewMeta is the structured shape extracted from a PostView.naver
// HTML body. PublishedAtUTC is the zero value (time.Time{}) when the
// publish-date string was missing or unparseable; check IsZero before
// using it.
type PostViewMeta struct {
	CommentCount         int       `json:"comment_count"`
	FloatingCommentCount int       `json:"floating_comment_count"`
	PublishDateStr       string    `json:"publish_date_str,omitempty"`
	PublishedAtUTC       time.Time `json:"published_at_utc"`
}

var (
	// Each <meta> matcher uses two variants — double-quoted and
	// single-quoted attributes — because Go's RE2 dialect doesn't
	// support backreferences (so a single regex with `(["'])...\1`
	// isn't possible). The earlier `["']([^"']+)["']` shape truncated
	// content at the first quote of either kind: titles like
	// `[스페셜로고] '어버이날' …` lost everything after `[스페셜로고] `
	// because the `'` inside the value matched the closing class.
	reMetaPropertyDQ = regexp.MustCompile(`<meta\s+property="([^"]+)"\s+content="([^"]*)"`)
	reMetaPropertySQ = regexp.MustCompile(`<meta\s+property='([^']+)'\s+content='([^']*)'`)
	// Naver sometimes orders the attributes content-first; cover both quote forms.
	reMetaPropertyAltDQ = regexp.MustCompile(`<meta\s+content="([^"]*)"\s+property="([^"]+)"`)
	reMetaPropertyAltSQ = regexp.MustCompile(`<meta\s+content='([^']*)'\s+property='([^']+)'`)
	reGsTagNameDQ       = regexp.MustCompile(`gsTagName\s*=\s*"([^"]+)"`)
	reGsTagNameSQ       = regexp.MustCompile(`gsTagName\s*=\s*'([^']+)'`)
	reSeContainerOpen   = regexp.MustCompile(`<div\s+class=["']se-main-container["']`)
	reImgSrcDQ          = regexp.MustCompile(`(?is)<img\s+[^>]*\bsrc="([^"]+)"[^>]*>`)
	reImgSrcSQ          = regexp.MustCompile(`(?is)<img\s+[^>]*\bsrc='([^']+)'[^>]*>`)
	reHTMLTag           = regexp.MustCompile(`<[^>]+>`)
	reWhitespace        = regexp.MustCompile(`\s+`)
	reCommentCount      = regexp.MustCompile(`<em\s+id=["']commentCount["'][^>]*>([0-9]*)</em>`)
	reFloatingComment   = regexp.MustCompile(`<em\s+id=["']floating_bottom_commentCount["'][^>]*>([0-9]*)</em>`)
	rePublishDate       = regexp.MustCompile(`<span\s+class=["']se_publishDate[^"']*["'][^>]*>([^<]+)</span>`)
)

// ParseMobilePost extracts metadata from m.blog.naver.com/<id>/<n>
// HTML. Returns a non-error PostMeta with empty fields when individual
// fields are missing — the caller decides whether title="" is a hard
// failure. Returns an error only when the input is structurally
// unusable (empty bytes).
func ParseMobilePost(htmlBytes []byte) (PostMeta, error) {
	if len(htmlBytes) == 0 {
		return PostMeta{}, fmt.Errorf("empty HTML")
	}
	src := string(htmlBytes)

	meta := PostMeta{}
	metaByProperty := extractOpenGraphMeta(src)
	meta.Title = metaByProperty["og:title"]
	meta.Description = metaByProperty["og:description"]
	meta.ThumbnailURL = metaByProperty["og:image"]

	if m := reGsTagNameDQ.FindStringSubmatch(src); m != nil {
		appendTags(&meta.Tags, m[1])
	} else if m := reGsTagNameSQ.FindStringSubmatch(src); m != nil {
		appendTags(&meta.Tags, m[1])
	}

	// Body extraction: locate the opening div and brace-balance the
	// matching closing </div>. The naive "substring up to the next
	// </div>" approach fails because se-main-container holds many
	// nested divs.
	if loc := reSeContainerOpen.FindStringIndex(src); loc != nil {
		bodyHTML := extractBalancedDiv(src, loc[0])
		meta.BodyHTML = bodyHTML
		meta.Images = extractBodyImages(bodyHTML)
		meta.BodyText = stripHTMLToText(bodyHTML)
	}

	return meta, nil
}

// ParsePostView extracts publish date from a PostView.naver HTML body
// and keeps the legacy comment-count em fallback for older/static
// captures. PublishedAtUTC is zero when the date couldn't be parsed;
// CommentCount is 0 when the em tag is missing or contains empty text.
// Current live comment counts should be fetched from cbox instead.
func ParsePostView(htmlBytes []byte) (PostViewMeta, error) {
	if len(htmlBytes) == 0 {
		return PostViewMeta{}, fmt.Errorf("empty HTML")
	}
	src := string(htmlBytes)
	meta := PostViewMeta{}

	if m := reCommentCount.FindStringSubmatch(src); m != nil {
		meta.CommentCount = atoiOrZero(strings.TrimSpace(m[1]))
	}
	if m := reFloatingComment.FindStringSubmatch(src); m != nil {
		meta.FloatingCommentCount = atoiOrZero(strings.TrimSpace(m[1]))
	}

	if m := rePublishDate.FindStringSubmatch(src); m != nil {
		raw := strings.TrimSpace(m[1])
		meta.PublishDateStr = raw
		if t, ok := parsePublishDateKST(raw); ok {
			meta.PublishedAtUTC = t.UTC()
		}
	}

	return meta, nil
}

func extractBodyImages(bodyHTML string) []string {
	if bodyHTML == "" {
		return nil
	}
	matches := reImgSrcDQ.FindAllStringSubmatch(bodyHTML, -1)
	matches = append(matches, reImgSrcSQ.FindAllStringSubmatch(bodyHTML, -1)...)
	if len(matches) == 0 {
		return nil
	}
	out := make([]string, 0, len(matches))
	seen := make(map[string]bool, len(matches))
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		imageURL := strings.TrimSpace(html.UnescapeString(m[1]))
		if imageURL == "" || seen[imageURL] || !isContentImageURL(imageURL) {
			continue
		}
		seen[imageURL] = true
		out = append(out, imageURL)
	}
	return out
}

// appendTags splits a comma-separated tag literal and appends each
// non-empty trimmed entry onto dst. Used by both gsTagName matchers
// so the dedupe-on-empty logic lives in one place.
func appendTags(dst *[]string, raw string) {
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			*dst = append(*dst, part)
		}
	}
}

func isContentImageURL(imageURL string) bool {
	lower := strings.ToLower(imageURL)
	// Deny-list: UI icons, profile pictures, stickers, system images.
	if strings.Contains(lower, "/imgs/emot/") ||
		strings.Contains(lower, "sticker") ||
		strings.Contains(lower, "/profile/") ||
		strings.Contains(lower, "blogpfthumb") {
		return false
	}
	// Allow-list: any Naver content-image CDN. Naver uses many subdomains
	// under pstatic.net for blog post images (blogfiles, postfiles,
	// blogthumb, mblogthumb-phinf, postfiles-phinf, etc.). The -phinf
	// variant is the photo-infrastructure / mobile-optimized host. Matching
	// the *.pstatic.net base with the deny-list above is more durable than
	// enumerating subdomains — Naver adds new ones periodically.
	if strings.Contains(lower, ".pstatic.net") {
		return true
	}
	// Also accept blogfiles.naver.net (legacy CDN; still serves originalImage URLs).
	return strings.Contains(lower, ".naver.net")
}

// extractOpenGraphMeta walks every <meta property="..." content="...">
// (in both attribute orderings Naver emits, and in both quote styles)
// and returns a map keyed by property name. Decodes HTML entities in
// content because Naver renders titles with &quot;, &amp; etc.
//
// Two regexes per ordering (DQ + SQ) because Go's RE2 has no
// backreferences and a single `["']` class would truncate content
// values containing an embedded `'`.
func extractOpenGraphMeta(src string) map[string]string {
	out := make(map[string]string)
	for _, m := range reMetaPropertyDQ.FindAllStringSubmatch(src, -1) {
		out[m[1]] = html.UnescapeString(m[2])
	}
	for _, m := range reMetaPropertySQ.FindAllStringSubmatch(src, -1) {
		if _, ok := out[m[1]]; !ok {
			out[m[1]] = html.UnescapeString(m[2])
		}
	}
	for _, m := range reMetaPropertyAltDQ.FindAllStringSubmatch(src, -1) {
		// Don't overwrite values already discovered by the canonical
		// ordering; the canonical ordering is more reliable.
		if _, ok := out[m[2]]; !ok {
			out[m[2]] = html.UnescapeString(m[1])
		}
	}
	for _, m := range reMetaPropertyAltSQ.FindAllStringSubmatch(src, -1) {
		if _, ok := out[m[2]]; !ok {
			out[m[2]] = html.UnescapeString(m[1])
		}
	}
	return out
}

// extractBalancedDiv returns the inner HTML of a <div> starting at
// startIdx (which must point at a '<' character beginning the div).
// Walks forward counting <div...> opens and </div> closes until the
// depth returns to zero, then returns the substring between the
// opening tag's closing '>' and the matching </div>'s opening '<'.
//
// Returns "" if startIdx isn't a div opening or if the HTML is
// truncated mid-tree.
func extractBalancedDiv(src string, startIdx int) string {
	if startIdx >= len(src) {
		return ""
	}
	// Find the end of the opening tag.
	tagEnd := strings.Index(src[startIdx:], ">")
	if tagEnd < 0 {
		return ""
	}
	bodyStart := startIdx + tagEnd + 1
	depth := 1
	i := bodyStart
	for i < len(src) {
		// Find the next div token, opening or closing.
		nextOpen := indexOfCaseInsensitive(src[i:], "<div")
		nextClose := indexOfCaseInsensitive(src[i:], "</div")
		if nextClose < 0 {
			return ""
		}
		if nextOpen >= 0 && nextOpen < nextClose {
			depth++
			// Advance past the <div opener to avoid re-counting.
			i += nextOpen + len("<div")
			continue
		}
		depth--
		closeStart := i + nextClose
		if depth == 0 {
			return src[bodyStart:closeStart]
		}
		i = closeStart + len("</div")
	}
	return ""
}

// indexOfCaseInsensitive is strings.Index with ASCII case insensitivity
// on a small literal needle. Used for the div bracket scan so DIV,
// Div, dIv all count — Naver's HTML is overwhelmingly lowercase but
// neighboring sites that occasionally embed in Naver content use
// uppercase tags.
func indexOfCaseInsensitive(haystack, needle string) int {
	if needle == "" {
		return 0
	}
	hLow := strings.ToLower(haystack)
	nLow := strings.ToLower(needle)
	return strings.Index(hLow, nLow)
}

// reScriptBlock and reStyleBlock drop <script>/<style> blocks before
// generic-tag stripping so JS literals and CSS classes don't pollute
// body_text. Go's RE2 dialect doesn't support backreferences, so we
// match each tag separately rather than `<(script|style)>...</\1>`.
var (
	reScriptBlock = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	reStyleBlock  = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
)

// stripHTMLToText collapses HTML markup into a single whitespace-
// normalized line of plain text. The intent is "body content as a
// human would read it" not faithful rendering — heading markup,
// paragraph breaks, and lists all become a single space. Used for
// sponsored-disclosure scanning and FTS body_text indexing, both of
// which want a flat text stream.
func stripHTMLToText(s string) string {
	if s == "" {
		return ""
	}
	s = reScriptBlock.ReplaceAllString(s, " ")
	s = reStyleBlock.ReplaceAllString(s, " ")
	// Replace remaining tags with a space so adjacent text from
	// different tags doesn't merge into one word.
	s = reHTMLTag.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = reWhitespace.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// atoiOrZero parses s as a positive integer. Returns 0 on parse
// failure, including empty strings — Naver's em tag often holds
// empty content for posts with zero comments.
func atoiOrZero(s string) int {
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	if n < 0 {
		return 0
	}
	return n
}

// parsePublishDateKST parses Naver's publish date string. The format
// observed in the wild is "YYYY. M. D. HH:MM" with single- or
// two-digit month/day/hour. Returns the parsed time in KST (Asia/Seoul)
// and ok=true; on failure returns the zero time and ok=false.
//
// Loads the Seoul timezone lazily and falls back to a fixed UTC+9
// zone when tzdata isn't available (e.g., a minimal container). The
// fixed-offset fallback matches Korea Standard Time exactly because
// Korea does not observe DST.
func parsePublishDateKST(raw string) (time.Time, bool) {
	// Normalize whitespace runs first — Naver renders "2026. 3. 30. 15:35"
	// but the source HTML can carry extra spaces.
	raw = reWhitespace.ReplaceAllString(strings.TrimSpace(raw), " ")
	loc := seoulLocation()
	formats := []string{
		"2006. 1. 2. 15:04",
		"2006. 01. 02. 15:04",
		"2006. 1. 2. 15:04:05",
	}
	for _, layout := range formats {
		if t, err := time.ParseInLocation(layout, raw, loc); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// seoulLocation returns the Asia/Seoul *time.Location, falling back to
// a fixed UTC+9 zone when tzdata is unavailable. The fallback exists
// so a minimal container without /usr/share/zoneinfo doesn't silently
// shift timestamps to UTC.
func seoulLocation() *time.Location {
	if loc, err := time.LoadLocation("Asia/Seoul"); err == nil {
		return loc
	}
	return time.FixedZone("KST", 9*60*60)
}
