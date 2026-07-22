// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

// Post parsing, HTML→text rendering, and the full-vs-preview access detector.
// Like Medium Reader, this tool never sees pixels: a post body is captured as
// text with image URLs + alt/caption preserved. The detector implements the
// CORRECTED gate from the attended crack (auth-model.md §"Gate-signal
// correction"): anonymous paid posts return a substantial PREVIEW, not an empty
// body, so "non-empty body_html" is NOT proof of access. The reliable signal is
// comparing the rendered body's word count to the post's own `wordcount` field
// (reported identically anon and authed, so it is a stable reference for the
// article's true length), backed by the authed-only per-user field `is_viewed`.
package substack

import (
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"strings"
)

var (
	scriptRE      = regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script\s*>`)
	styleRE       = regexp.MustCompile(`(?is)<style\b[^>]*>.*?</style\s*>`)
	imgTagRE      = regexp.MustCompile(`(?is)<img\b[^>]*>`)
	attrSrcRE     = regexp.MustCompile(`(?is)\bsrc\s*=\s*["']([^"']*)["']`)
	attrAltRE     = regexp.MustCompile(`(?is)\balt\s*=\s*["']([^"']*)["']`)
	blockCloseRE  = regexp.MustCompile(`(?is)</(p|div|h[1-6]|li|ul|ol|figure|figcaption|blockquote|section|article|table|tr)\s*>`)
	brRE          = regexp.MustCompile(`(?is)<br\s*/?>`)
	anyTagRE      = regexp.MustCompile(`(?is)<[^>]+>`)
	imgMarkdownRE = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	urlRE         = regexp.MustCompile(`https?://\S+`)
	wordRE        = regexp.MustCompile(`[\p{L}\p{N}][\p{L}\p{N}'’\-]*`)
	multiNLRE     = regexp.MustCompile(`\n{3,}`)
)

// HTMLToText converts a Substack body_html into readable plain text, preserving
// images as `![alt](src)` markdown and collapsing block structure into
// paragraphs. Dependency-free (stdlib html + regexp) so no x/net/html is pulled
// into the build.
func HTMLToText(bodyHTML string) string {
	if strings.TrimSpace(bodyHTML) == "" {
		return ""
	}
	s := bodyHTML
	// Drop <script>/<style> blocks (tag AND inner text) before anything else:
	// their contents are code/CSS, not prose, and the generic tag stripper below
	// only removes the tags — leaving the inner text to leak into the rendered
	// body and inflate WordCount, which would skew the fullBodyRatio entitlement
	// gate in DetectAccess.
	s = scriptRE.ReplaceAllString(s, "")
	s = styleRE.ReplaceAllString(s, "")
	// Images first, before generic tag stripping erases their attributes.
	s = imgTagRE.ReplaceAllStringFunc(s, func(tag string) string {
		src := ""
		if m := attrSrcRE.FindStringSubmatch(tag); m != nil {
			src = m[1]
		}
		alt := ""
		if m := attrAltRE.FindStringSubmatch(tag); m != nil {
			alt = m[1]
		}
		if src == "" {
			return ""
		}
		return "\n![" + alt + "](" + src + ")\n"
	})
	s = brRE.ReplaceAllString(s, "\n")
	s = blockCloseRE.ReplaceAllString(s, "\n\n")
	s = anyTagRE.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	// Normalize non-breaking spaces (from &nbsp;) to regular spaces so word
	// counting and downstream tooling treat them as ordinary whitespace.
	s = strings.ReplaceAll(s, "\u00A0", " ")
	s = multiNLRE.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// WordCount counts prose words in text, excluding image-markdown and bare URLs
// so the count tracks readable article length (the basis for the preview-vs-
// full ratio against the post's `wordcount` field).
func WordCount(text string) int {
	t := imgMarkdownRE.ReplaceAllString(text, " ")
	t = urlRE.ReplaceAllString(t, " ")
	return len(wordRE.FindAllString(t, -1))
}

// PostMeta is the subset of a single-post object the reader reasons about.
type PostMeta struct {
	ID           string // numeric post id — the key for the authed substack.com by-id fetch
	Slug         string
	Title        string
	Subtitle     string
	Audience     string
	Wordcount    int
	BodyHTML     string
	CanonicalURL string
	PostDate     string
	Subdomain    string
	CustomDomain string
	HasIsViewed  bool // authed-only per-user field: proof the body was served to a logged-in you
}

// ParsePostMeta extracts PostMeta from a raw single-post JSON object.
func ParsePostMeta(raw json.RawMessage) (PostMeta, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return PostMeta{}, err
	}
	strOf := func(k string) string {
		var s string
		if v, ok := m[k]; ok {
			_ = json.Unmarshal(v, &s)
		}
		return s
	}
	pm := PostMeta{
		Slug:         strOf("slug"),
		Title:        strOf("title"),
		Subtitle:     strOf("subtitle"),
		Audience:     strOf("audience"),
		BodyHTML:     strOf("body_html"),
		CanonicalURL: strOf("canonical_url"),
		PostDate:     strOf("post_date"),
		Subdomain:    strOf("subdomain"),
		CustomDomain: strOf("custom_domain"),
	}
	if v, ok := m["wordcount"]; ok {
		var n json.Number
		if json.Unmarshal(v, &n) == nil {
			if i, err := n.Int64(); err == nil {
				pm.Wordcount = int(i)
			}
		}
	}
	if v, ok := m["id"]; ok {
		var n json.Number
		if json.Unmarshal(v, &n) == nil {
			pm.ID = n.String()
		}
	}
	_, pm.HasIsViewed = m["is_viewed"]
	return pm, nil
}

// Access is the runtime full-vs-preview verdict for a post.
type Access struct {
	Full   bool
	Tier   string // "free" | "entitled" | "preview"
	Reason string
}

// fullBodyRatio is how close the rendered word count must come to the post's
// declared `wordcount` to count as the full article. A preview renders ≪
// wordcount (uxmentor: ~13.9k preview chars vs 42.4k full), so 0.8 cleanly
// separates the two while tolerating rendering differences (nav chrome, member
// widgets) between our word tokenizer and Substack's own count.
const fullBodyRatio = 0.8

// DetectAccess implements the corrected gate. renderedWords is
// WordCount(HTMLToText(body)); authed reports whether a Tier-1 session was used.
//
// Two honesty rules, both load-bearing for the tool's core promise:
//   - "entitled" (full body unlocked BY YOUR SESSION) is claimed only when the
//     read was authenticated AND carried the authed-only per-user field
//     is_viewed AND the rendered body reaches the article's declared length. A
//     high word-count ratio on an ANONYMOUS read is never "entitled" — no
//     session unlocked it — so a padded/late-gated preview can't masquerade as
//     session-unlocked access.
//   - The word-count ratio is authoritative for *completeness* (did we receive
//     the whole body?); the per-user is_viewed flag is only the fallback signal
//     when the post carries no wordcount to measure against.
func DetectAccess(meta PostMeta, renderedWords int, authed bool) Access {
	if strings.EqualFold(strings.TrimSpace(meta.Audience), "everyone") {
		return Access{Full: true, Tier: "free", Reason: "free post (audience: everyone)"}
	}
	// Paid post (only_paid / founding / unknown).
	if meta.Wordcount > 0 {
		ratioFull := float64(renderedWords) >= fullBodyRatio*float64(meta.Wordcount)
		switch {
		case authed && meta.HasIsViewed && ratioFull:
			return Access{Full: true, Tier: "entitled", Reason: fmt.Sprintf("full body via your session (%d words ≈ wordcount %d)", renderedWords, meta.Wordcount)}
		case ratioFull:
			// The full body came back without a session (or without the per-user
			// proof) — effectively public. Report it as full, but NEVER
			// "entitled": no session unlocked it, so it must not read as
			// session-unlocked access.
			return Access{Full: true, Tier: "full", Reason: fmt.Sprintf("full public body — no session needed (%d words ≈ wordcount %d)", renderedWords, meta.Wordcount)}
		default:
			return Access{Full: false, Tier: "preview", Reason: fmt.Sprintf("preview only (%d of ~%d words) — %s", renderedWords, meta.Wordcount, entitleHint(authed, meta.HasIsViewed))}
		}
	}
	// No wordcount reference to ratio against — fall back to the authed-only
	// per-user signal.
	if authed && meta.HasIsViewed {
		return Access{Full: true, Tier: "entitled", Reason: "authenticated (per-user fields present); no wordcount to verify length"}
	}
	return Access{Full: false, Tier: "preview", Reason: "paid post — " + entitleHint(authed, meta.HasIsViewed)}
}

func entitleHint(authed, hasIsViewed bool) string {
	if !authed {
		return "no session configured; set SUBSTACK_SESSION or save your substack.sid to the config cookie file to unlock posts you subscribe to"
	}
	if !hasIsViewed {
		return "your session is not entitled to this post (are you a paying subscriber of this publication?)"
	}
	return "your session did not return the full body for this post"
}
