// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

// Package cdcparse parses CDC Pakistan's HTML surfaces: the admin-ajax
// downloads fragments and the statistics table.
//
// TRANSPORT NOTE THAT SHAPES THIS PACKAGE. Every CDC path is Cloudflare
// challenged. A challenge response is HTTP 403 with an HTML body, which a
// selector-based parser reads as "zero items". Conflating the two is how a real
// month gets recorded as "CDC published nothing", so IsChallenge is exported and
// callers MUST test it before trusting an empty item list.
package cdcparse

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	xhtml "golang.org/x/net/html"

	"github.com/mvanhorn/printing-press-library/library/payments/cdc-pakistan/internal/cliutil"
)

// ErrChallenge means the response is a bot-protection interstitial, not data.
var ErrChallenge = errors.New("cdcparse: Cloudflare challenge response, not content")

var challengeRe = regexp.MustCompile(`(?i)just a moment|cf_chl_opt|cf-mitigated|challenge-platform`)

// IsChallenge reports whether body is a Cloudflare interstitial. Callers check
// this BEFORE concluding a bucket is empty.
func IsChallenge(body string) bool {
	head := body
	if len(head) > 4096 {
		head = head[:4096]
	}
	return challengeRe.MatchString(head)
}

// Document is one item from a downloads listing.
type Document struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	// MetaDayMonth is the listing's own date text, e.g. "30 December". It
	// carries NO YEAR -- CDC omits it -- so it cannot be resolved alone.
	MetaDayMonth string `json:"meta_day_month"`
	// UploadYear/UploadMonth come from the /assets/uploads/YYYY/MM/ path. They
	// are the UPLOAD date, which can differ from the document's own as-of date
	// (observed: meta "30 April" filed under 2026/05). Both are persisted; the
	// as-of date is derived separately and never collapsed onto either.
	UploadYear  string `json:"upload_year,omitempty"`
	UploadMonth string `json:"upload_month,omitempty"`
	// LegacyPath marks the ~14% of documents served from /assets/uploads/misc/,
	// /assets/uploads/publications/ or bare http:// with NO date-bearing path.
	LegacyPath bool   `json:"legacy_path"`
	Category   string `json:"category"`
	YearParam  int    `json:"year_param"`
}

var uploadPathRe = regexp.MustCompile(`/assets/uploads/(\d{4})/(\d{2})/`)

// ParseDownloads extracts documents from one admin-ajax HTML fragment.
//
// It returns ErrChallenge for an interstitial so an empty result can never be
// mistaken for an empty bucket.
func ParseDownloads(body, category string, yearParam int) ([]Document, error) {
	if IsChallenge(body) {
		return nil, ErrChallenge
	}
	root, err := xhtml.Parse(strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parsing downloads fragment: %w", err)
	}

	var out []Document
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode && n.Data == "div" && hasClass(n, "download_list") {
			if d, ok := documentFrom(n); ok {
				d.Category = category
				d.YearParam = yearParam
				out = append(out, d)
			}
			return // items do not nest
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return out, nil
}

func documentFrom(item *xhtml.Node) (Document, bool) {
	var d Document
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode {
			switch {
			case n.Data == "h4" && d.Title == "":
				d.Title = cliutil.CleanText(textOf(n))
			case n.Data == "div" && hasClass(n, "meta") && d.MetaDayMonth == "":
				d.MetaDayMonth = cliutil.CleanText(textOf(n))
			case n.Data == "a" && d.URL == "":
				if href := attr(n, "href"); href != "" {
					d.URL = href
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(item)

	if d.URL == "" || d.Title == "" {
		return d, false
	}
	if m := uploadPathRe.FindStringSubmatch(d.URL); m != nil {
		d.UploadYear, d.UploadMonth = m[1], m[2]
	} else {
		d.LegacyPath = true
	}
	return d, true
}

// BlockHash is a stable fingerprint of a fragment's item URLs. Two consecutive
// pages sharing a hash means the paginator is echoing, which is exactly what
// the /downloads-category/<cat>/page/N/ URL path does for every N.
func BlockHash(docs []Document) string {
	urls := make([]string, 0, len(docs))
	for _, d := range docs {
		urls = append(urls, d.URL)
	}
	sum := sha256.Sum256([]byte(strings.Join(urls, "|")))
	return hex.EncodeToString(sum[:])[:16]
}

func hasClass(n *xhtml.Node, want string) bool {
	for _, f := range strings.Fields(attr(n, "class")) {
		if f == want {
			return true
		}
	}
	return false
}

func attr(n *xhtml.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func textOf(n *xhtml.Node) string {
	var sb strings.Builder
	var walk func(*xhtml.Node)
	walk = func(x *xhtml.Node) {
		if x.Type == xhtml.TextNode {
			sb.WriteString(x.Data)
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
}

// ParseYearParam is a small helper so callers do not scatter Atoi error paths.
func ParseYearParam(s string) (int, error) {
	y, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || y < 1990 || y > 2100 {
		return 0, fmt.Errorf("year %q is not a plausible CDC listing year (2007-2026 observed)", s)
	}
	return y, nil
}
