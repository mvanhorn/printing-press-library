// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

// Package page is the v2 article-read fetch surface. Medium server-renders every
// article page with the post body embedded as a JSON blob in the HTML
// (window.__APOLLO_STATE__). This package GETs https://medium.com/p/<id> through
// the shared Surf transport (which follows the 302 to the canonical subdomain
// URL), extracts that JSON, walks it to the post's body model, and reconstructs
// Markdown that mirrors the v1 oracle style (# H1, ##/### headings, ![](src) for
// images, > for blockquotes, list items, code fences, plain paragraphs).
//
// As with the rss package, parsing is split from fetching on purpose:
// Parse([]byte, idHint) is a pure function over page bytes (the seam the
// hermetic tests exercise against saved fixtures), and Source.ReadArticle is the
// thin network wrapper. That split keeps `go test ./...` offline-green.
//
// Cookies (the user's own Medium session) are attached to the request so a later
// Tier-1 cookie unlocks the member full body; with no cookie the fetch returns
// Medium's truncated preview and Article.IsPreviewOnly is true.
package page

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/internal/source"
)

// Source is the page-parse implementation of source.Source. It serves
// ReadArticle and declares every other capability false (returning
// source.ErrUnsupported if mis-dispatched), per the contract.
type Source struct {
	client  *http.Client
	cookies source.Cookies
}

// New returns a page Source over the given HTTP client. A nil client is
// acceptable for tests that only exercise the pure parser (ReadArticle will
// lazily build a default Surf transport when actually called over the wire).
func New(client *http.Client) *Source {
	return &Source{client: client}
}

// WithCookies returns a copy of the source that attaches the given Tier-1
// session cookies on the read request, unlocking member full bodies. The
// zero-value source stays fully anonymous (Tier 0).
func (s *Source) WithCookies(c source.Cookies) *Source {
	cp := *s
	cp.cookies = c
	return &cp
}

// Name identifies this source in resolver diagnostics.
func (s *Source) Name() string { return "page" }

// Capabilities advertises ReadArticle only.
func (s *Source) Capabilities() source.Caps {
	return source.Caps{ReadArticle: true}
}

func (s *Source) httpClient() *http.Client {
	if s.client != nil {
		return s.client
	}
	return source.NewHTTPClient(60 * time.Second)
}

// Feed is unsupported by the page source.
func (s *Source) Feed(ctx context.Context, ref string) ([]source.PostSummary, error) {
	return nil, source.ErrUnsupported
}

// Search is unsupported by the page source.
func (s *Source) Search(ctx context.Context, query string, limit int) ([]source.PostSummary, error) {
	return nil, source.ErrUnsupported
}

// AuthorArchive is unsupported by the page source.
func (s *Source) AuthorArchive(ctx context.Context, userIDOrHandle string, max int) ([]source.PostSummary, error) {
	return nil, source.ErrUnsupported
}

// ReadArticle fetches the article page and reconstructs its Markdown. idOrURL
// may be a full Medium URL or a bare article id; both resolve to the canonical
// /p/<id> short link, which Medium 302-redirects to the post's subdomain URL
// (the Surf client follows it).
func (s *Source) ReadArticle(ctx context.Context, idOrURL string) (*source.Article, error) {
	id := ExtractID(idOrURL)
	if id == "" {
		return nil, fmt.Errorf("page: could not extract an article id from %q", idOrURL)
	}
	url := "https://medium.com/p/" + id
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("page: building request: %w", err)
	}
	// Attach optional Tier-1 cookies so a member session unlocks the full body.
	source.AttachCookies(req, s.cookies)
	resp, err := s.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("page: fetching %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("page: %s returned HTTP %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("page: reading body: %w", err)
	}
	return Parse(body, id)
}

// ResolveUserID fetches a Medium profile page (https://medium.com/@<handle>) and
// extracts the author's stable User id from the embedded __APOLLO_STATE__. It is
// the keyless handle->id resolver the author-archive command uses so a @handle
// or username argument works the way v1's id_for endpoint did, without any API
// key. A leading "@" on the handle is stripped.
//
// Resolution is split from parsing on purpose (mirroring ReadArticle/Parse):
// ParseUserID is a pure function over profile-page bytes (the seam the hermetic
// test exercises against a saved fixture), and this method is the thin network
// wrapper.
func (s *Source) ResolveUserID(ctx context.Context, handle string) (string, error) {
	h := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(handle), "@"))
	if h == "" {
		return "", fmt.Errorf("page: empty handle")
	}
	url := "https://medium.com/@" + h
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("page: building profile request: %w", err)
	}
	source.AttachCookies(req, s.cookies)
	resp, err := s.httpClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("page: fetching %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("page: %s returned HTTP %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("page: reading profile body: %w", err)
	}
	id := ParseUserID(body, h)
	if id == "" {
		return "", fmt.Errorf("page: could not resolve a user id for handle %q from its profile page", handle)
	}
	return id, nil
}

// ParseUserID extracts a Medium User id from profile-page bytes. handleHint is
// the (already @-stripped) handle being resolved; when several User entities are
// present (the page also embeds referenced authors), it is used to pick the one
// whose username matches the requested handle. Falls back to the single User:
// entry, then to any User whose id field is non-empty. Returns "" when no user
// id is recoverable. ParseUserID is pure (no network).
func ParseUserID(htmlBytes []byte, handleHint string) string {
	raw := extractAssignment(string(htmlBytes), "__APOLLO_STATE__")
	if raw == "" {
		return ""
	}
	var cache apolloCache
	if err := json.Unmarshal([]byte(raw), &cache); err != nil {
		return ""
	}
	return pickUserID(cache, handleHint)
}

// pickUserID resolves the profile owner's User id from an Apollo cache. When a
// handleHint is given, the only trusted resolution is the User entity whose
// username matches it (case-insensitive); if none matches it returns "" rather
// than guessing, because Medium serves a page embedding unrelated authors for a
// nonexistent handle. With no hint (URL-based resolution), it falls back through
// the User pointed at by ROOT_QUERY's userResult, then the single User: entry,
// and finally any User: entry that carries an id field.
func pickUserID(cache apolloCache, handleHint string) string {
	hint := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(handleHint), "@"))

	// 1. Username match against any User: entity.
	if hint != "" {
		for k, v := range cache {
			if !strings.HasPrefix(k, "User:") {
				continue
			}
			var u map[string]json.RawMessage
			if json.Unmarshal(v, &u) != nil {
				continue
			}
			if strings.ToLower(jsonString(u["username"])) == hint {
				if id := jsonString(u["id"]); id != "" {
					return id
				}
				return strings.TrimPrefix(k, "User:")
			}
		}
		// A handle was requested but no User: entity on this page carries that
		// username. Medium serves HTTP 200 (not 404) for a nonexistent handle,
		// embedding unrelated authors; falling through to the positional
		// fallbacks below would resolve the garbage handle to an arbitrary
		// embedded author and archive the wrong writer's corpus. Report
		// not-found instead — the caller turns "" into a clear usage error.
		return ""
	}

	// The positional fallbacks below run only for hint-less resolution (e.g.
	// resolving from a URL): with no requested handle to verify against, the
	// page's own primary user result is the best available signal.

	// 2. ROOT_QUERY's userResult __ref, if present.
	if root, ok := cache["ROOT_QUERY"]; ok {
		var rq map[string]json.RawMessage
		if json.Unmarshal(root, &rq) == nil {
			for k, v := range rq {
				if !strings.HasPrefix(k, "userResult") {
					continue
				}
				if ref := refKey(v); strings.HasPrefix(ref, "User:") {
					if id := userIDFromEntry(cache, ref); id != "" {
						return id
					}
				}
			}
		}
	}

	// 3. The single User: entry, if there is exactly one.
	var only string
	count := 0
	for k := range cache {
		if strings.HasPrefix(k, "User:") {
			only = k
			count++
		}
	}
	if count == 1 {
		if id := userIDFromEntry(cache, only); id != "" {
			return id
		}
	}

	// 4. Any User: entry carrying an id field.
	for k := range cache {
		if !strings.HasPrefix(k, "User:") {
			continue
		}
		if id := userIDFromEntry(cache, k); id != "" {
			return id
		}
	}
	return ""
}

// userIDFromEntry returns the id of a User: cache entry, falling back to the
// key suffix (User:<id>) when the entity has no explicit id field.
func userIDFromEntry(cache apolloCache, key string) string {
	if entry, ok := cache[key]; ok {
		var u map[string]json.RawMessage
		if json.Unmarshal(entry, &u) == nil {
			if id := jsonString(u["id"]); id != "" {
				return id
			}
		}
	}
	return strings.TrimPrefix(key, "User:")
}

// Parse extracts the article from page bytes and renders its Markdown. idHint is
// the article id the caller asked for; it is used to pick the right Post:<id>
// cache entry and as the Article.ID when the embedded JSON omits it. Parse is
// pure (no network), which is what the hermetic tests exercise.
//
// Extraction order mirrors the design's fallback chain:
//  1. window.__APOLLO_STATE__  — the full body model (preferred).
//  2. __PRELOADED_STATE__      — older/alternate embed shape.
//  3. JSON-LD                  — last-resort metadata-only (no body model).
func Parse(htmlBytes []byte, idHint string) (*source.Article, error) {
	s := string(htmlBytes)

	if art, ok := parseApollo(s, idHint); ok {
		return art, nil
	}
	if art, ok := parsePreloaded(s, idHint); ok {
		return art, nil
	}
	if art, ok := parseJSONLD(s, idHint); ok {
		return art, nil
	}
	return nil, fmt.Errorf("page: no recognizable article payload in HTML (no __APOLLO_STATE__, __PRELOADED_STATE__, or JSON-LD)")
}

// ---- APOLLO_STATE path -----------------------------------------------------

// apolloCache is the normalized Apollo store: a flat map of cacheKey -> entity,
// where nested entities are stored as {"__ref": "<cacheKey>"} pointers we
// dereference while walking. We decode into json.RawMessage values lazily so we
// only fully unmarshal the entities we actually touch.
type apolloCache map[string]json.RawMessage

func parseApollo(s, idHint string) (*source.Article, bool) {
	raw := extractAssignment(s, "__APOLLO_STATE__")
	if raw == "" {
		return nil, false
	}
	var cache apolloCache
	if err := json.Unmarshal([]byte(raw), &cache); err != nil {
		return nil, false
	}
	return parseApolloCache(cache, idHint)
}

// postToArticle is the shared Post->Article walk used by both the
// __APOLLO_STATE__ and __PRELOADED_STATE__ paths. Folding it into one function
// keeps the two embed shapes from diverging — most importantly the
// IsPreviewOnly handling for a locked post with no content node, which the two
// paths previously disagreed on. Returns (nil, false) when the post entity
// cannot be decoded.
func postToArticle(cache apolloCache, postKey, idHint string) (*source.Article, bool) {
	var post map[string]json.RawMessage
	if err := json.Unmarshal(cache[postKey], &post); err != nil {
		return nil, false
	}

	art := &source.Article{ID: idHint}
	if art.ID == "" {
		art.ID = strings.TrimPrefix(postKey, "Post:")
	}
	art.Title = jsonString(post["title"])
	art.URL = jsonString(post["mediumUrl"])
	art.IsLocked = jsonBool(post["isLocked"])
	if ms := jsonInt64(post["firstPublishedAt"]); ms > 0 {
		art.PublishedAt = time.UnixMilli(ms).UTC()
	}
	// Author lives on the creator entity (a __ref into the cache).
	if cr := derefEntity(cache, post["creator"]); cr != nil {
		art.Author = jsonString(cr["name"])
		art.AuthorID = jsonString(cr["id"])
	}

	// The body lives under a content(...) field whose argument JSON varies
	// (the referrer changes per fetch), so we match by the "content(" prefix
	// rather than a fixed key.
	contentRaw := fieldByPrefix(post, "content(")
	if contentRaw == nil {
		// Metadata-only (e.g. a fully locked post with no preview body): still
		// a usable Article, just with no Markdown. A locked post fetched with
		// no content node is, by definition, preview-only.
		art.IsPreviewOnly = art.IsLocked
		return art, true
	}
	var content map[string]json.RawMessage
	if err := json.Unmarshal(contentRaw, &content); err != nil {
		return art, true
	}
	art.IsPreviewOnly = jsonBool(content["isLockedPreviewOnly"])

	paras := collectParagraphs(cache, content["bodyModel"])
	if len(paras) == 0 {
		return art, true
	}
	md := renderMarkdown(paras)
	art.Markdown = md
	art.WordCount = wordCount(md)
	if art.Title == "" {
		// Fall back to the first H-type paragraph as the title.
		for _, p := range paras {
			if strings.HasPrefix(p.Type, "H") && p.Text != "" {
				art.Title = p.Text
				break
			}
		}
	}
	return art, true
}

// pickPostKey returns the Apollo cache key for the post to render. It prefers an
// exact Post:<idHint> match, then the post pointed to by ROOT_QUERY's
// postResult, then the single Post: entry if there is exactly one.
func pickPostKey(cache apolloCache, idHint string) string {
	if idHint != "" {
		if _, ok := cache["Post:"+idHint]; ok {
			return "Post:" + idHint
		}
	}
	if root, ok := cache["ROOT_QUERY"]; ok {
		var rq map[string]json.RawMessage
		if json.Unmarshal(root, &rq) == nil {
			for k, v := range rq {
				if !strings.HasPrefix(k, "postResult") {
					continue
				}
				if ref := refKey(v); strings.HasPrefix(ref, "Post:") {
					if _, ok := cache[ref]; ok {
						return ref
					}
				}
			}
		}
	}
	var only string
	count := 0
	for k := range cache {
		if strings.HasPrefix(k, "Post:") {
			only = k
			count++
		}
	}
	if count == 1 {
		return only
	}
	return ""
}

// paragraph is the flattened body paragraph the renderer consumes.
type paragraph struct {
	Type     string
	Text     string
	Href     string
	ImageID  string // metadata.id for IMG paragraphs
	CodeLang string // codeBlockMetadata.lang for PRE paragraphs
	Markups  []markup
}

type markup struct {
	Type string
	Href string
	// Start/End are code-point offsets into the paragraph text (see
	// applyMarkups for why we convert the JSON's UTF-16 offsets to runes).
	Start int
	End   int
}

// collectParagraphs dereferences bodyModel.paragraphs into flattened paragraphs.
// bodyModel may itself be a __ref; its paragraphs list is a slice of __ref
// pointers into the cache.
func collectParagraphs(cache apolloCache, bodyModelRaw json.RawMessage) []paragraph {
	bm := derefEntity(cache, bodyModelRaw)
	if bm == nil {
		return nil
	}
	var refs []json.RawMessage
	if err := json.Unmarshal(bm["paragraphs"], &refs); err != nil {
		return nil
	}
	out := make([]paragraph, 0, len(refs))
	for _, r := range refs {
		pe := derefEntity(cache, r)
		if pe == nil {
			continue
		}
		out = append(out, flattenParagraph(cache, pe))
	}
	return out
}

func flattenParagraph(cache apolloCache, pe map[string]json.RawMessage) paragraph {
	p := paragraph{
		Type: jsonString(pe["type"]),
		Text: jsonString(pe["text"]),
		Href: jsonString(pe["href"]),
	}
	if meta := derefEntity(cache, pe["metadata"]); meta != nil {
		p.ImageID = jsonString(meta["id"])
	}
	if cb := derefEntity(cache, pe["codeBlockMetadata"]); cb != nil {
		p.CodeLang = jsonString(cb["lang"])
	}
	var rawMks []map[string]json.RawMessage
	if json.Unmarshal(pe["markups"], &rawMks) == nil {
		for _, m := range rawMks {
			p.Markups = append(p.Markups, markup{
				Type:  jsonString(m["type"]),
				Href:  jsonString(m["href"]),
				Start: int(jsonInt64(m["start"])),
				End:   int(jsonInt64(m["end"])),
			})
		}
	}
	return p
}

// ---- Markdown rendering ----------------------------------------------------

// renderMarkdown reconstructs Markdown from the flattened paragraphs, mirroring
// the v1 oracle's style. Paragraphs are blank-line separated; consecutive list
// items of the same kind are grouped into one block.
func renderMarkdown(paras []paragraph) string {
	var blocks []string
	firstH := true
	for _, p := range paras {
		switch p.Type {
		case "H2", "H3", "H4":
			// The article's leading heading is the title (rendered as # H1);
			// subsequent H2/H3 are section headings, H4 a sub-heading.
			text := applyMarkups(p.Text, p.Markups)
			if firstH {
				blocks = append(blocks, "# "+text)
				firstH = false
			} else {
				blocks = append(blocks, headingPrefix(p.Type)+text)
			}
		case "P":
			blocks = append(blocks, applyMarkups(p.Text, p.Markups))
		case "PRE":
			blocks = append(blocks, "```"+p.CodeLang+"\n"+p.Text+"\n```")
		case "BQ", "PQ":
			blocks = append(blocks, "> "+applyMarkups(p.Text, p.Markups))
		case "ULI":
			blocks = append(blocks, "- "+applyMarkups(p.Text, p.Markups))
		case "OLI":
			blocks = append(blocks, "1. "+applyMarkups(p.Text, p.Markups))
		case "IMG":
			src := imageURL(p.ImageID)
			alt := applyMarkups(p.Text, p.Markups)
			blocks = append(blocks, "!["+alt+"]("+src+")")
		case "HR":
			blocks = append(blocks, "---")
		default:
			// Unknown type: emit its text as a plain paragraph rather than
			// dropping content silently.
			if strings.TrimSpace(p.Text) != "" {
				blocks = append(blocks, applyMarkups(p.Text, p.Markups))
			}
		}
	}
	return strings.Join(blocks, "\n\n")
}

func headingPrefix(t string) string {
	switch t {
	case "H2":
		return "## "
	case "H3":
		return "### "
	case "H4":
		return "#### "
	default:
		return "### "
	}
}

func imageURL(id string) string {
	if id == "" {
		return ""
	}
	return "https://miro.medium.com/" + id
}

// applyMarkups overlays Medium's inline markups (bold/italic/links) onto the
// paragraph text. Medium's start/end offsets are UTF-16 code-unit indices (JS
// string semantics); we convert the text to runes and the offsets to rune
// indices so multibyte characters (em dashes, curly quotes) don't shift the
// wrapping. Trailing whitespace inside a span is pushed outside the emphasis
// markers, matching the oracle's CommonMark-style rendering.
func applyMarkups(text string, mks []markup) string {
	if len(mks) == 0 {
		return text
	}
	runes := []rune(text)
	// Map UTF-16 unit offset -> rune index. A rune outside the BMP counts as
	// two UTF-16 units, so we walk the string accumulating unit positions.
	u16ToRune := make(map[int]int, len(runes)+1)
	unit := 0
	for ri, r := range runes {
		u16ToRune[unit] = ri
		if r > 0xFFFF {
			unit += 2
		} else {
			unit++
		}
	}
	u16ToRune[unit] = len(runes)

	resolve := func(u int) (int, bool) {
		if ri, ok := u16ToRune[u]; ok {
			return ri, true
		}
		return 0, false
	}

	// Insertions at rune boundaries: open markers sort before close markers at
	// the same position so adjacent spans nest cleanly.
	type ins struct {
		at   int
		open bool
		text string
	}
	var inserts []ins
	for _, m := range mks {
		open, close := markerFor(m)
		if open == "" && close == "" {
			continue
		}
		startR, ok1 := resolve(m.Start)
		endR, ok2 := resolve(m.End)
		if !ok1 || !ok2 || startR < 0 || endR > len(runes) || startR >= endR {
			continue
		}
		// Push trailing whitespace in the span outside the emphasis markers.
		for endR > startR && isSpace(runes[endR-1]) {
			endR--
		}
		if startR >= endR {
			continue
		}
		inserts = append(inserts, ins{at: startR, open: true, text: open})
		inserts = append(inserts, ins{at: endR, open: false, text: close})
	}
	if len(inserts) == 0 {
		return text
	}
	sort.SliceStable(inserts, func(i, j int) bool {
		if inserts[i].at != inserts[j].at {
			return inserts[i].at < inserts[j].at
		}
		// At the same position, emit closing markers before opening ones so
		// "a**b**_c_" style adjacency renders correctly.
		return !inserts[i].open && inserts[j].open
	})

	var b strings.Builder
	ip := 0
	for ri := 0; ri <= len(runes); ri++ {
		for ip < len(inserts) && inserts[ip].at == ri {
			b.WriteString(inserts[ip].text)
			ip++
		}
		if ri < len(runes) {
			b.WriteRune(runes[ri])
		}
	}
	return b.String()
}

// markerFor returns the (open, close) Markdown markers for a markup. Links wrap
// as [text](href); STRONG as **; EM as _.
func markerFor(m markup) (string, string) {
	switch m.Type {
	case "STRONG":
		return "**", "**"
	case "EM":
		return "_", "_"
	case "A":
		if m.Href == "" {
			return "", ""
		}
		return "[", "](" + m.Href + ")"
	default:
		return "", ""
	}
}

func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

// ---- __PRELOADED_STATE__ and JSON-LD fallbacks -----------------------------

// parsePreloaded attempts the older __PRELOADED_STATE__ embed. Its shape mirrors
// the Apollo cache closely enough that we route through the same Apollo walker
// when it carries a normalized post cache; otherwise we give up (false) and let
// the JSON-LD fallback try.
func parsePreloaded(s, idHint string) (*source.Article, bool) {
	raw := extractAssignment(s, "__PRELOADED_STATE__")
	if raw == "" {
		return nil, false
	}
	// __PRELOADED_STATE__ nests the apollo cache under .apolloState.<...> in
	// some versions; probe for a flat map that contains a Post: key.
	var top map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &top) != nil {
		return nil, false
	}
	// Direct: the top map already has Post: keys.
	if hasPostKey(top) {
		return parseApolloCache(apolloCache(top), idHint)
	}
	// Nested under apolloState.
	if as, ok := top["apolloState"]; ok {
		var cache apolloCache
		if json.Unmarshal(as, &cache) == nil && hasPostKeyCache(cache) {
			return parseApolloCache(cache, idHint)
		}
	}
	return nil, false
}

// parseApolloCache is the shared cache-walk used by both __APOLLO_STATE__ and a
// __PRELOADED_STATE__ that carries a normalized cache. It picks the post key and
// delegates the Post->Article projection to postToArticle so both embed shapes
// produce identical Articles (including IsPreviewOnly for locked posts).
func parseApolloCache(cache apolloCache, idHint string) (*source.Article, bool) {
	postKey := pickPostKey(cache, idHint)
	if postKey == "" {
		return nil, false
	}
	return postToArticle(cache, postKey, idHint)
}

// parseJSONLD is the last-resort, metadata-only fallback. Medium emits an
// Article JSON-LD block; we recover title, author, url, and date but cannot
// reconstruct the body, so Markdown is empty and IsPreviewOnly tracks the
// locked flag if present.
func parseJSONLD(s, idHint string) (*source.Article, bool) {
	const open = `<script type="application/ld+json">`
	i := strings.Index(s, open)
	if i < 0 {
		return nil, false
	}
	rest := s[i+len(open):]
	j := strings.Index(rest, "</script>")
	if j < 0 {
		return nil, false
	}
	var ld map[string]json.RawMessage
	if json.Unmarshal([]byte(rest[:j]), &ld) != nil {
		return nil, false
	}
	art := &source.Article{ID: idHint}
	art.Title = firstNonEmpty(jsonString(ld["headline"]), jsonString(ld["name"]))
	art.URL = jsonString(ld["url"])
	if author := ld["author"]; author != nil {
		var a map[string]json.RawMessage
		if json.Unmarshal(author, &a) == nil {
			art.Author = jsonString(a["name"])
		}
	}
	if dp := jsonString(ld["datePublished"]); dp != "" {
		if t, err := time.Parse(time.RFC3339, dp); err == nil {
			art.PublishedAt = t.UTC()
		}
	}
	if art.Title == "" {
		return nil, false
	}
	return art, true
}

// ---- low-level helpers -----------------------------------------------------

// extractAssignment returns the balanced JSON object assigned to a named JS
// variable: it finds "<name>", then the first '{' after the following '=', and
// scans to the matching '}', respecting string literals and escapes. Returns ""
// when not found or unbalanced.
func extractAssignment(s, name string) string {
	i := strings.Index(s, name)
	if i < 0 {
		return ""
	}
	eq := strings.IndexByte(s[i:], '=')
	if eq < 0 {
		return ""
	}
	start := strings.IndexByte(s[i+eq:], '{')
	if start < 0 {
		return ""
	}
	start += i + eq
	depth := 0
	inStr := false
	esc := false
	for j := start; j < len(s); j++ {
		c := s[j]
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : j+1]
			}
		}
	}
	return ""
}

func refKey(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var ref struct {
		Ref string `json:"__ref"`
	}
	if json.Unmarshal(raw, &ref) == nil {
		return ref.Ref
	}
	return ""
}

// derefEntity resolves a value that may be either an inline object or a
// {"__ref": "<key>"} pointer into the cache, returning the entity's fields.
func derefEntity(cache apolloCache, raw json.RawMessage) map[string]json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	if key := refKey(raw); key != "" {
		entry, ok := cache[key]
		if !ok {
			return nil
		}
		raw = entry
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return nil
	}
	return m
}

// fieldByPrefix returns the first field value whose key starts with prefix.
func fieldByPrefix(m map[string]json.RawMessage, prefix string) json.RawMessage {
	for k, v := range m {
		if strings.HasPrefix(k, prefix) {
			return v
		}
	}
	return nil
}

func hasPostKey(m map[string]json.RawMessage) bool {
	for k := range m {
		if strings.HasPrefix(k, "Post:") {
			return true
		}
	}
	return false
}

func hasPostKeyCache(c apolloCache) bool {
	for k := range c {
		if strings.HasPrefix(k, "Post:") {
			return true
		}
	}
	return false
}

func jsonString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var v string
	if json.Unmarshal(raw, &v) == nil {
		return v
	}
	return ""
}

func jsonBool(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var v bool
	if json.Unmarshal(raw, &v) == nil {
		return v
	}
	return false
}

func jsonInt64(raw json.RawMessage) int64 {
	if len(raw) == 0 {
		return 0
	}
	var v int64
	if json.Unmarshal(raw, &v) == nil {
		return v
	}
	return 0
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func wordCount(md string) int {
	return len(strings.Fields(md))
}

// ExtractID pulls a Medium article id from a full URL or returns a bare id
// unchanged. Medium ids are lowercase-hex runs; we accept the /p/<id> short
// form, the trailing -<id> slug suffix, or a bare id.
func ExtractID(idOrURL string) string {
	s := strings.TrimSpace(idOrURL)
	if s == "" {
		return ""
	}
	// Bare id (no scheme, no slash): accept if it looks like a hex id.
	if !strings.Contains(s, "/") {
		if i := strings.IndexByte(s, '?'); i >= 0 {
			s = s[:i]
		}
		if isHexID(s) {
			return s
		}
		// A bare slug-with-id like "title-<id>".
		if id := idFromTrailingSlug(s); id != "" {
			return id
		}
		return ""
	}
	// /p/<id> short link.
	if id := idFromPSlug(s); id != "" {
		return id
	}
	// Slug form: .../some-title-<id>?source=...
	clean := s
	if i := strings.IndexByte(clean, '?'); i >= 0 {
		clean = clean[:i]
	}
	if i := strings.IndexByte(clean, '#'); i >= 0 {
		clean = clean[:i]
	}
	clean = strings.TrimRight(clean, "/")
	return idFromTrailingSlug(clean)
}

func idFromPSlug(s string) string {
	const marker = "/p/"
	i := strings.Index(s, marker)
	if i < 0 {
		return ""
	}
	rest := s[i+len(marker):]
	if j := strings.IndexAny(rest, "?/#"); j >= 0 {
		rest = rest[:j]
	}
	if isHexID(rest) {
		return rest
	}
	return ""
}

func idFromTrailingSlug(clean string) string {
	if i := strings.LastIndexByte(clean, '-'); i >= 0 {
		cand := clean[i+1:]
		if isHexID(cand) {
			return cand
		}
	}
	if i := strings.LastIndexByte(clean, '/'); i >= 0 {
		cand := clean[i+1:]
		if isHexID(cand) {
			return cand
		}
	}
	return ""
}

func isHexID(s string) bool {
	if len(s) < 6 {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}
