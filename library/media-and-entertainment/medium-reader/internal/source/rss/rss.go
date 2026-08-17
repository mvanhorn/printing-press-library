// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

// Package rss is the v2 RSS fetch surface. Medium publishes standard RSS 2.0
// feeds for every author, publication, and tag — no API key, no GraphQL, no
// cookies. This is the most legitimate ($0, public, stable) source for the
// feed command, so the Resolver lists it first for feeds.
//
// The package separates parsing from fetching deliberately: Parse([]byte) is a
// pure function over feed bytes (the seam the hermetic tests exercise against
// saved fixtures), and Source.Feed is the thin network wrapper that fetches a
// URL through the shared transport and hands the bytes to Parse. That split is
// what keeps `go test ./...` offline-green.
package rss

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/internal/source"
)

// Item is one parsed RSS entry, projecting the Medium-relevant elements. Body
// holds the content:encoded full HTML when present (publication/tag feeds for
// free posts); it is empty for feeds that only ship a description snippet.
type Item struct {
	ID          string
	Title       string
	URL         string
	Author      string // dc:creator
	PublishedAt time.Time
	Tags        []string // category[]
	Body        string   // content:encoded
}

// xmlRSS mirrors the RSS 2.0 document shape we consume. Namespaced elements
// (dc:creator, content:encoded) are matched by LOCAL NAME rather than full
// namespace URI: Go's encoding/xml namespace matching is brittle across feeds
// that declare prefixes differently, and local-name matching is the long-
// established, robust convention for RSS readers. The local names "creator"
// and "encoded" are unambiguous within a Medium feed item.
type xmlRSS struct {
	XMLName xml.Name   `xml:"rss"`
	Channel xmlChannel `xml:"channel"`
}

type xmlChannel struct {
	Items []xmlItem `xml:"item"`
}

type xmlItem struct {
	Title      string   `xml:"title"`
	Link       string   `xml:"link"`
	GUID       string   `xml:"guid"`
	Creator    string   `xml:"creator"` // dc:creator (local name)
	PubDate    string   `xml:"pubDate"`
	Categories []string `xml:"category"`
	Encoded    string   `xml:"encoded"` // content:encoded (local name)
}

// pubDateLayouts lists the time formats Medium uses for pubDate, in order of
// likelihood. Medium emits RFC1123 with a "GMT" zone (RFC1123 with numeric
// zone and the bare-day RFC822 variants are kept as defensive fallbacks).
var pubDateLayouts = []string{
	time.RFC1123,  // Mon, 02 Jan 2006 15:04:05 MST  -> "Tue, 16 Jun 2026 16:49:15 GMT"
	time.RFC1123Z, // Mon, 02 Jan 2006 15:04:05 -0700
	time.RFC822,
	time.RFC822Z,
	"Mon, 2 Jan 2006 15:04:05 MST",
	"2006-01-02T15:04:05Z07:00", // RFC3339, defensive
}

// parsePubDate parses a feed pubDate string against the known layouts. It
// returns the zero time (not an error) when no layout matches — a missing or
// malformed date should not drop an otherwise valid item.
func parsePubDate(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range pubDateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// extractArticleID pulls the stable Medium article id from an item. Medium's
// guid is canonical (https://medium.com/p/<id>); we fall back to the same
// pattern in the link, then to the trailing -<id> slug suffix.
func extractArticleID(guid, link string) string {
	if id := idFromPSlug(guid); id != "" {
		return id
	}
	if id := idFromPSlug(link); id != "" {
		return id
	}
	// Slug form: .../some-title-<id>?source=... — the id is the last hex run
	// before any query string.
	clean := link
	if i := strings.IndexByte(clean, '?'); i >= 0 {
		clean = clean[:i]
	}
	if i := strings.LastIndexByte(clean, '-'); i >= 0 {
		cand := clean[i+1:]
		if isHexID(cand) {
			return cand
		}
	}
	return ""
}

// idFromPSlug extracts <id> from a "/p/<id>" URL fragment.
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

// isHexID reports whether s looks like a Medium article id (a non-empty run of
// lowercase hex digits). Medium ids are 12-char hex but we accept any plausible
// hex run to stay robust to id-length changes.
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

// Parse decodes RSS 2.0 feed bytes into Items. It returns an error for input
// that is not a parseable RSS document, so a caller can distinguish "feed had
// no items" (len 0, nil err) from "this was not a feed" (non-nil err).
func Parse(data []byte) ([]Item, error) {
	var doc xmlRSS
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("rss: parsing feed: %w", err)
	}
	if doc.XMLName.Local != "rss" {
		return nil, fmt.Errorf("rss: not an RSS document (root <%s>)", doc.XMLName.Local)
	}
	items := make([]Item, 0, len(doc.Channel.Items))
	for _, xi := range doc.Channel.Items {
		tags := make([]string, 0, len(xi.Categories))
		for _, c := range xi.Categories {
			if c = strings.TrimSpace(c); c != "" {
				tags = append(tags, c)
			}
		}
		items = append(items, Item{
			ID:          extractArticleID(xi.GUID, xi.Link),
			Title:       strings.TrimSpace(xi.Title),
			URL:         strings.TrimSpace(xi.Link),
			Author:      strings.TrimSpace(xi.Creator),
			PublishedAt: parsePubDate(xi.PubDate),
			Tags:        tags,
			Body:        xi.Encoded,
		})
	}
	return items, nil
}

// ToPostSummaries projects parsed Items onto the normalized source.PostSummary
// model the Resolver and command layer consume.
func ToPostSummaries(items []Item) []source.PostSummary {
	out := make([]source.PostSummary, 0, len(items))
	for _, it := range items {
		out = append(out, source.PostSummary{
			ID:          it.ID,
			Title:       it.Title,
			Author:      it.Author,
			URL:         it.URL,
			PublishedAt: it.PublishedAt,
			Tags:        it.Tags,
		})
	}
	return out
}

// Kind classifies a feed reference so the right /feed/... URL can be built.
type Kind int

const (
	// KindPublication is the default: a bare slug maps to /feed/<slug>.
	KindPublication Kind = iota
	// KindUser is an @handle, mapping to /feed/@<handle>.
	KindUser
	// KindTag is a tag/<tag> ref (or one the caller forces), mapping to
	// /feed/tag/<tag>.
	KindTag
)

// ClassifyRef auto-detects the kind of a feed reference using the rules the
// feed command documents: a leading "@" is a user; a "tag/" prefix is a tag;
// everything else is treated as a publication slug. (A bare token with no
// prefix is ambiguous between a user handle and a publication; we default it to
// publication here, and the command's --user/--tag flags or @-prefix let the
// caller disambiguate.)
func ClassifyRef(ref string) Kind {
	ref = strings.TrimSpace(ref)
	switch {
	case strings.HasPrefix(ref, "@"):
		return KindUser
	case strings.HasPrefix(ref, "tag/"):
		return KindTag
	default:
		return KindPublication
	}
}

const feedBase = "https://medium.com/feed"

// FeedURL builds the absolute feed URL for a (kind, ref) pair. It normalizes
// the ref so callers can pass either decorated (@handle, tag/foo) or bare
// (handle, foo) forms for a known kind.
func FeedURL(kind Kind, ref string) string {
	ref = strings.TrimSpace(ref)
	switch kind {
	case KindUser:
		handle := strings.TrimPrefix(ref, "@")
		return feedBase + "/@" + handle
	case KindTag:
		tag := strings.TrimPrefix(ref, "tag/")
		return feedBase + "/tag/" + tag
	default:
		return feedBase + "/" + ref
	}
}

// Source is the RSS implementation of source.Source. It serves Feed and
// declares every other capability false (returning source.ErrUnsupported if
// mis-dispatched), per the contract.
type Source struct {
	client *http.Client
}

// New returns an RSS Source over the given HTTP client. A nil client is
// acceptable for tests that only exercise the pure parser / classifier helpers
// (Feed will lazily build a default client when actually called over the wire).
func New(client *http.Client) *Source {
	return &Source{client: client}
}

// Name identifies this source in resolver diagnostics.
func (s *Source) Name() string { return "rss" }

// Capabilities advertises Feed only.
func (s *Source) Capabilities() source.Caps {
	return source.Caps{Feed: true}
}

// httpClient returns the configured client, or a default Surf transport if the
// source was constructed without one. Building lazily keeps New(nil) valid for
// tests while still giving the command a working wire path.
func (s *Source) httpClient() *http.Client {
	if s.client != nil {
		return s.client
	}
	return source.NewHTTPClient(60 * time.Second)
}

// Feed fetches and parses the RSS feed for ref. The ref is auto-classified
// (@user / tag/<tag> / publication slug); pass an @-prefixed handle or a
// tag/<tag> form to force user/tag, otherwise a bare slug is a publication.
func (s *Source) Feed(ctx context.Context, ref string) ([]source.PostSummary, error) {
	url := FeedURL(ClassifyRef(ref), ref)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("rss: building request: %w", err)
	}
	resp, err := s.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("rss: fetching %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rss: %s returned HTTP %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("rss: reading body: %w", err)
	}
	items, err := Parse(body)
	if err != nil {
		return nil, err
	}
	return ToPostSummaries(items), nil
}

// ReadArticle is unsupported by the RSS source.
func (s *Source) ReadArticle(ctx context.Context, idOrURL string) (*source.Article, error) {
	return nil, source.ErrUnsupported
}

// Search is unsupported by the RSS source.
func (s *Source) Search(ctx context.Context, query string, limit int) ([]source.PostSummary, error) {
	return nil, source.ErrUnsupported
}

// AuthorArchive is unsupported by the RSS source.
func (s *Source) AuthorArchive(ctx context.Context, userIDOrHandle string, max int) ([]source.PostSummary, error) {
	return nil, source.ErrUnsupported
}
