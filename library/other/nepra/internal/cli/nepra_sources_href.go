// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
// See .printing-press-patches/nepra-sources-catalogue.json.

package cli

import (
	"bytes"
	"strings"

	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// The verbatim href harvester.
//
// WHY THIS FILE EXISTS, given that the tree already has two href readers.
//
// On nepra.org.pk one byte of URL decides whether a document exists:
//
//	/Standards/2022/NEPRA%20PER%202021%20Distribution%20Companies%20.pdf
//	  -> HTTP 200, Content-Length 2277838
//	the same URL with the trailing %20 removed
//	  -> HTTP 404, Content-Length 9
//
// Both shipped readers destroy that byte when it arrives raw:
//
//   - html_extract.go:165 does strings.TrimSpace(attrValue(n, "href")) and then
//     round-trips the result through url.Parse + ResolveReference + String()
//     (normalizeHTMLURL, html_extract.go:203-215).
//   - nepra_events_parse.go:221-223 (firstHref) also TrimSpaces.
//
// A TrimSpace on `href="… Companies .pdf"` silently produces the 404 twin, and
// the caller cannot tell the difference: NEPRA answers a missing document with
// HTTP 404 and a nine-byte body whether it never existed or you asked for it
// one byte wrong.
//
// So this harvester does exactly three things to a raw href and nothing else:
//
//  1. HTML ENTITY DECODING, which the parser has already done. This is the one
//     transformation that is not a URL change: `&amp;` in markup IS the byte
//     `&` in the URL, and NEPRA's own Main.htm writes
//     `Quarterly%20Data%20(XWD%20&amp;%20KE).htm` for a path whose real byte is
//     `&`. eventSurfaces stores `/M&E/Orders%20of%20the%20Authority.php` the
//     same way.
//  2. LEXICAL RESOLUTION against the index's own directory, by string surgery.
//     One leading segment of the base directory is consumed per `../`. No
//     url.Parse, so nothing is re-encoded on the way through.
//  3. RAW SPACE -> %20, and nothing else. This is required, not optional: every
//     one of the 46 pdf hrefs on the State of Industry Reports index carries
//     RAW spaces ("State of Industry Reports/State of Industry Report 2025.pdf"),
//     and the path that actually serves the document is the %20 form. Any other
//     byte outside 0x21..0x7E, and any '%' that does not begin a valid two-hex
//     escape, makes the href NOT ENCODABLE — reported as such rather than
//     repaired, because a repaired guess that 404s is indistinguishable from a
//     document NEPRA never published.
//
// MEASURED, and it settles an open question the survey left: on
// publications/Performance%20Reports.php the FY2020-21 trailing space is
// already PRE-ENCODED as %20 in the markup, so the shipped readers survive that
// one by luck. Five other hrefs on the same page carry RAW internal spaces, and
// all 46 SIR hrefs carry raw spaces. The trailing-space case is guarded here by
// a synthetic fragment in TestSourcesHrefHarvesterPreservesRawTrailingSpace
// because NEPRA does not currently publish one — which is a reason to keep the
// guard, not to drop it.

// harvestedHref is one href exactly as published, plus the path it resolves to.
type harvestedHref struct {
	// HrefRaw is the attribute value verbatim after HTML entity decoding. It is
	// never trimmed.
	HrefRaw string `json:"href_raw"`
	// PathEncoded is the site-rooted path with raw spaces encoded as %20 and
	// every other byte untouched. It is "" when PathEncodable is false.
	PathEncoded string `json:"path_encoded,omitempty"`
	// PathEncodable is false when the href cannot be turned into a request path
	// without guessing.
	PathEncodable bool `json:"path_encodable"`
	// NotEncodableReason says which byte stopped it.
	NotEncodableReason string `json:"not_encodable_reason,omitempty"`
	// OffHost is true for an href pointing at another host. Recorded rather
	// than dropped, so "the index links offsite" stays visible.
	OffHost bool `json:"off_host,omitempty"`
	// Query is anything after '?', kept separate so a path comparison is a
	// path comparison. Fragments are dropped: they are not sent to the server.
	Query string `json:"query,omitempty"`
}

const nepraSourcesHostPrefixHTTPS = "https://nepra.org.pk"
const nepraSourcesHostPrefixHTTP = "http://nepra.org.pk"

// sourcesHarvestHrefs reads every <a href> out of one index page, verbatim.
//
// doc is the DECODED page bytes. indexPath is the site-rooted path the page was
// fetched from, and is used only for lexical resolution of relative refs.
//
// Anchors that cannot be a document reference — empty, in-page fragments,
// javascript: and mailto: — are skipped. Everything else is returned, including
// off-host and non-encodable hrefs, because the caller's job is to notice new
// documents and a silently dropped href is a missed one.
func sourcesHarvestHrefs(doc []byte, indexPath string) ([]harvestedHref, error) {
	root, err := xhtml.Parse(bytes.NewReader(doc))
	if err != nil {
		return nil, err
	}
	baseDir := sourcesHrefBaseDir(indexPath)
	var out []harvestedHref
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if sourcesHrefBearing(n) {
			for _, a := range n.Attr {
				if !strings.EqualFold(a.Key, "href") {
					continue
				}
				// DELIBERATELY NOT strings.TrimSpace(a.Val).
				if h, ok := sourcesResolveHref(a.Val, baseDir); ok {
					out = append(out, h)
				}
				break
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return out, nil
}

// sourcesHrefBearing reports whether a node's href names a document.
//
// <a> is the obvious case. <link rel="File-List"> is the SECOND case and it is
// not an oversight to include it: Excel writes that element to point at the
// workbook's filelist.xml, which is the ONE enumerator on this site that
// actually works, and it is what makes Main.htm's complete href set ten entries
// rather than nine. MEASURED on the committed fixture: nine <a> plus
// `<link rel=File-List href="Main_files/filelist.xml">`.
//
// No other <link> qualifies. The two .php indexes carry 13 rel="stylesheet"
// links and one rel="icon" between them, and a stylesheet is chrome, not a
// published document.
func sourcesHrefBearing(n *xhtml.Node) bool {
	if n.Type != xhtml.ElementNode {
		return false
	}
	if n.DataAtom == atom.A {
		return true
	}
	if n.DataAtom != atom.Link {
		return false
	}
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, "rel") && strings.EqualFold(strings.TrimSpace(a.Val), "File-List") {
			return true
		}
	}
	return false
}

// sourcesHrefBaseDir returns the directory of a page path, with its trailing
// slash. It is pure string surgery.
func sourcesHrefBaseDir(indexPath string) string {
	if i := strings.LastIndexByte(indexPath, '/'); i >= 0 {
		return indexPath[:i+1]
	}
	return "/"
}

// sourcesResolveHref turns one raw href into a harvestedHref, or reports that
// it is not a document reference at all.
func sourcesResolveHref(raw, baseDir string) (harvestedHref, bool) {
	h := harvestedHref{HrefRaw: raw}
	// Only the emptiness test looks at a trimmed copy; the stored value is
	// always the raw one.
	if strings.TrimSpace(raw) == "" {
		return h, false
	}
	lower := strings.ToLower(strings.TrimSpace(raw))
	if strings.HasPrefix(lower, "#") ||
		strings.HasPrefix(lower, "javascript:") ||
		strings.HasPrefix(lower, "mailto:") ||
		strings.HasPrefix(lower, "tel:") ||
		strings.HasPrefix(lower, "data:") {
		return h, false
	}

	ref := raw
	// A fragment is never sent to the server.
	if i := strings.IndexByte(ref, '#'); i >= 0 {
		ref = ref[:i]
	}
	if i := strings.IndexByte(ref, '?'); i >= 0 {
		h.Query = ref[i+1:]
		ref = ref[:i]
	}

	// THE HOST IS MATCHED ON THE AUTHORITY, NOT BY STRING PREFIX.
	//
	// A prefix test against "https://nepra.org.pk" also matches
	// "https://nepra.org.pk.evil.example/..." and
	// "https://nepra.org.pk@evil.example/...", both of which are OFF-host —
	// the first because the registrable domain is evil.example, the second
	// because everything before the '@' is userinfo. Either would have been
	// stripped to a bare path and then treated as a nepra.org.pk document by
	// every consumer of this catalogue.
	switch {
	case nepraSourcesIsOnHost(lower, nepraSourcesHostPrefixHTTPS):
		ref = ref[len(nepraSourcesHostPrefixHTTPS):]
	case nepraSourcesIsOnHost(lower, nepraSourcesHostPrefixHTTP):
		ref = ref[len(nepraSourcesHostPrefixHTTP):]
	case strings.HasPrefix(lower, "http://"), strings.HasPrefix(lower, "https://"), strings.HasPrefix(ref, "//"):
		h.OffHost = true
		h.PathEncodable = false
		h.NotEncodableReason = "off-host reference; this catalogue only describes nepra.org.pk"
		return h, true
	}
	if ref == "" {
		ref = "/"
	}

	if !strings.HasPrefix(ref, "/") {
		ref = sourcesResolveRelative(ref, baseDir)
	}

	encoded, reason := sourcesEncodePathSpaces(ref)
	if reason != "" {
		h.PathEncodable = false
		h.NotEncodableReason = reason
		return h, true
	}
	h.PathEncoded = encoded
	h.PathEncodable = true
	return h, true
}

// sourcesResolveRelative joins a relative reference to a base directory,
// consuming one base segment per "../". It never calls url.Parse.
func sourcesResolveRelative(ref, baseDir string) string {
	dir := baseDir
	if !strings.HasPrefix(dir, "/") {
		dir = "/" + dir
	}
	if !strings.HasSuffix(dir, "/") {
		dir += "/"
	}
	for {
		switch {
		case strings.HasPrefix(ref, "./"):
			ref = ref[2:]
		case strings.HasPrefix(ref, "../"):
			ref = ref[3:]
			// Drop the last segment of dir. "/a/b/" -> "/a/". Bottoming out at
			// "/" is where a "../" too many stops, rather than escaping the
			// root.
			trimmed := strings.TrimSuffix(dir, "/")
			if i := strings.LastIndexByte(trimmed, '/'); i >= 0 {
				dir = trimmed[:i+1]
			} else {
				dir = "/"
			}
		default:
			return dir + ref
		}
	}
}

// sourcesEncodePathSpaces encodes raw spaces as %20 and refuses anything else
// it cannot place in a URL path verbatim.
//
// The '%' rule mirrors validateEncodedSubPath (nepra_encoded_subpath.go:59-70)
// on purpose: a '%' that does not begin a valid escape means the value is
// half-encoded and there is no way to tell which half, so it is refused rather
// than double-encoded. An href already carrying %20 passes through untouched.
func sourcesEncodePathSpaces(ref string) (string, string) {
	var b strings.Builder
	b.Grow(len(ref) + 8)
	for i := 0; i < len(ref); i++ {
		c := ref[i]
		switch {
		case c == ' ':
			b.WriteString("%20")
		case c == '%':
			if i+2 >= len(ref) || !isHexDigit(ref[i+1]) || !isHexDigit(ref[i+2]) {
				return "", "stray % at byte " + itoa(i) + ": the href is not percent-encoded correctly " +
					"and this build will not guess which half was meant"
			}
			b.WriteString(ref[i : i+3])
			i += 2
		case c < 0x21 || c > 0x7E:
			return "", "byte 0x" + hexByte(c) + " at offset " + itoa(i) +
				" must be percent-encoded, and encoding it here would be a guess about the " +
				"page's charset (NEPRA serves no charset header on its data files)"
		default:
			b.WriteByte(c)
		}
	}
	return b.String(), ""
}

func hexByte(c byte) string {
	const hexDigits = "0123456789abcdef"
	return string([]byte{hexDigits[c>>4], hexDigits[c&0x0f]})
}

// sourcesHrefIsPDF reports whether an href names a PDF. The test is
// CASE-INSENSITIVE for classification only — NEPRA's server is case-SENSITIVE
// on the extension, so the path itself is never re-cased.
func sourcesHrefIsPDF(h harvestedHref) bool {
	ref := h.HrefRaw
	if i := strings.IndexByte(ref, '#'); i >= 0 {
		ref = ref[:i]
	}
	if i := strings.IndexByte(ref, '?'); i >= 0 {
		ref = ref[:i]
	}
	return strings.HasSuffix(strings.ToLower(ref), ".pdf")
}

// nepraSourcesIsOnHost reports whether ref begins with the given scheme+host
// AND that the authority ENDS there — the next byte must start the path, the
// query or the fragment, or there must be no next byte at all.
//
// Without the boundary check a bare prefix match accepts
// "https://nepra.org.pk.evil.example/x" (a different registrable domain) and
// "https://nepra.org.pk@evil.example/x" (the host is everything AFTER the
// '@'), and treats both as on-host.
func nepraSourcesIsOnHost(lower, prefix string) bool {
	if !strings.HasPrefix(lower, prefix) {
		return false
	}
	rest := lower[len(prefix):]
	if rest == "" {
		return true
	}
	switch rest[0] {
	case '/', '?', '#':
		return true
	default:
		// '.', ':', '@', '-' and anything else means the authority continues,
		// so this is a different host.
		return false
	}
}
