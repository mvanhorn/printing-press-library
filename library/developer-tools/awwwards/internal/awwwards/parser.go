// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// Package awwwards parses awwwards.com server-rendered HTML into typed design data.
//
// The most redesign-stable target is the JSON embedded in each listing card's
// data-collectable-model-value attribute; detail pages are parsed from
// class-structured markup (scores, jury notes, palette, tags, credits).
package awwwards

import (
	"encoding/json"
	"html"
	"regexp"
	"strconv"
	"strings"
)

// FlexID accepts the numeric ids on website cards and the UUID string ids on
// element cards.
type FlexID string

// UnmarshalJSON accepts a JSON number or string.
func (f *FlexID) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	*f = FlexID(s)
	return nil
}

// Card is one listing entry (website or element) from the embedded card JSON.
type Card struct {
	ID              FlexID            `json:"id"`
	Slug            string            `json:"slug"`
	Title           string            `json:"title"`
	CreatedAt       int64             `json:"createdAt"`
	Tags            []string          `json:"tags"`
	Type            string            `json:"type"`
	Images          map[string]string `json:"images"`
	CollectableID   string            `json:"collectableIdentifier"`
	MainImage       string            `json:"main_image"`
	User            *CardUser         `json:"user,omitempty"`
	InspirationSlug string            `json:"-"` // element cards only: slug of /inspiration/<slug>
	ExternalURL     string            `json:"-"` // element cards only: the live site URL next to the card
}

// CardUser is the submitting user on element cards.
type CardUser struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Type        string `json:"type"`
}

// Thumbnail returns the best-known thumbnail path for the card.
func (c Card) Thumbnail() string {
	if t := c.Images["thumbnail"]; t != "" {
		return t
	}
	return c.MainImage
}

// Thumbnail CDN size variants.
const (
	ThumbSizeSmall = "440_330"
	ThumbSizeLarge = "880_660"
)

// ThumbnailURL builds a fetchable CDN URL for a card thumbnail path.
// Size is ThumbSizeSmall (default when empty) or ThumbSizeLarge.
func ThumbnailURL(path, size string) string {
	if path == "" {
		return ""
	}
	if size == "" {
		size = ThumbSizeSmall
	}
	return "https://assets.awwwards.com/awards/media/cache/thumb_" + size + "/" + strings.TrimPrefix(path, "/")
}

// Scores is the jury score breakdown. Awwwards weights: Design 40%,
// Usability 30%, Creativity 20%, Content 10%.
type Scores struct {
	Design     float64 `json:"design"`
	Usability  float64 `json:"usability"`
	Creativity float64 `json:"creativity"`
	Content    float64 `json:"content"`
	Overall    float64 `json:"overall"`
}

// WeightedOverall computes the official Awwwards weighting from dimension scores.
func (s Scores) WeightedOverall() float64 {
	return 0.4*s.Design + 0.3*s.Usability + 0.2*s.Creativity + 0.1*s.Content
}

// JuryVote is one juror's scores on a site detail page.
type JuryVote struct {
	Name    string    `json:"name"`
	Profile string    `json:"profile"`
	Country string    `json:"country"`
	Scores  []float64 `json:"scores"` // design, usability, creativity, content, overall (as rendered)
}

// Credit is one "made by" credit on a detail page.
type Credit struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
}

// Tag is a Technologies & Tools entry: display label plus its filter slug.
type Tag struct {
	Slug  string `json:"slug"`
	Label string `json:"label"`
}

// Detail is a parsed /sites/<slug> page.
type Detail struct {
	Slug        string     `json:"slug"`
	Title       string     `json:"title"`
	ExternalURL string     `json:"external_url"`
	Award       string     `json:"award"`
	Scores      Scores     `json:"scores"`
	Jury        []JuryVote `json:"jury"`
	Palette     []string   `json:"palette"`
	Tags        []Tag      `json:"tags"`
	Credits     []Credit   `json:"credits"`
}

var (
	reCardAttr    = regexp.MustCompile(`data-collectable-model-value="([^"]+)"`)
	reCardBlock   = regexp.MustCompile(`(?s)<li[^>]*js-collectable.*?</li>`)
	reInspHref    = regexp.MustCompile(`href="/inspiration/([a-zA-Z0-9_-]+)"`)
	reExtHref     = regexp.MustCompile(`href="(https?://[^"]+)"`)
	reH1          = regexp.MustCompile(`(?s)<h1[^>]*>\s*<a href="([^"]+)"[^>]*>\s*([^<]+?)\s*</a>`)
	reH1Plain     = regexp.MustCompile(`(?s)<h1[^>]*>\s*([^<]+?)\s*</h1>`)
	reTitleTag    = regexp.MustCompile(`<title>[^<]*Awwwards\s+([A-Za-z ]+)</title>`)
	reOverallDims = regexp.MustCompile(`layout-overall__score"><strong>([0-9.]+)\s*/\s*10</strong>`)
	reJuryBlock   = regexp.MustCompile(`(?s)list-jury-notes__item.*?</li>`)
	reJuryName    = regexp.MustCompile(`href="/([a-zA-Z0-9_.-]+)/"><strong>([^<]+)</strong>`)
	reJuryCountry = regexp.MustCompile(`(?s)list-jury-notes__from">\s*from\s*<strong>([^<]+)</strong>`)
	// Nominee/honorable-mention vote items render the voter name outside the
	// anchor and the country as `<i>from</i> <strong>X</strong>`.
	reVoteName    = regexp.MustCompile(`(?s)<div class="info">\s*<div>\s*<strong>([^<]+)</strong>`)
	reVoteProfile = regexp.MustCompile(`href="/([a-zA-Z0-9_.-]+)/"`)
	reVoteCountry = regexp.MustCompile(`(?s)<i>from</i>\s*<strong>([^<]+)</strong>`)
	reScoreItem   = regexp.MustCompile(`grid-score__item[^"]*">([0-9.]+)<`)
	rePaletteHex  = regexp.MustCompile(`list-palette__name"><strong>HEX</strong>\s*(#[0-9A-Fa-f]{6})`)
	reTagLink     = regexp.MustCompile(`href="/websites/([^/"]+)/" class="button button--tag">([^<]+)<`)
	reCreditBlock = regexp.MustCompile(`(?s)<ul class="users-credits__details">.*?</ul>`)
	reCreditLink  = regexp.MustCompile(`href="/([a-zA-Z0-9_.-]+)/" aria-label="([^"]*)"`)
)

// ParseCards extracts all embedded listing cards from a listing page
// (/websites/..., /elements/<type>/, collection grids). For element cards it
// also captures the adjacent /inspiration/<slug> link and external site URL.
func ParseCards(doc string) []Card {
	var cards []Card
	blocks := reCardBlock.FindAllString(doc, -1)
	if len(blocks) == 0 {
		// Fall back to bare attribute scan when the <li> wrapper changes shape.
		for _, m := range reCardAttr.FindAllStringSubmatch(doc, -1) {
			if c, ok := decodeCard(m[1]); ok {
				cards = append(cards, c)
			}
		}
		return cards
	}
	for _, b := range blocks {
		m := reCardAttr.FindStringSubmatch(b)
		if m == nil {
			continue
		}
		c, ok := decodeCard(m[1])
		if !ok {
			continue
		}
		if im := reInspHref.FindStringSubmatch(b); im != nil {
			c.InspirationSlug = im[1]
		}
		for _, em := range reExtHref.FindAllStringSubmatch(b, -1) {
			u := em[1]
			if strings.Contains(u, "awwwards.com") || strings.Contains(u, "assets.awwwards") {
				continue
			}
			c.ExternalURL = u
			break
		}
		cards = append(cards, c)
	}
	return cards
}

func decodeCard(attr string) (Card, bool) {
	var c Card
	if err := json.Unmarshal([]byte(html.UnescapeString(attr)), &c); err != nil {
		return Card{}, false
	}
	if c.Slug == "" && c.CollectableID == "" {
		return Card{}, false
	}
	if c.Slug == "" {
		c.Slug = c.CollectableID
	}
	return c, true
}

// ParentSlugForElement derives an element card's parent site slug from its
// /inspiration/<slug> URL, which is "<type-label-slug>-<parent-site-slug>".
// typeSlug is the /elements/<type>/ segment (underscores allowed, e.g. 404_page).
func ParentSlugForElement(inspirationSlug, typeSlug string) string {
	if inspirationSlug == "" {
		return ""
	}
	prefix := strings.ToLower(strings.ReplaceAll(typeSlug, "_", "-")) + "-"
	low := strings.ToLower(inspirationSlug)
	if strings.HasPrefix(low, prefix) {
		return inspirationSlug[len(prefix):]
	}
	return ""
}

// ParseDetail extracts the full design profile from a /sites/<slug> page.
func ParseDetail(slug, doc string) Detail {
	d := Detail{Slug: slug}

	if m := reH1.FindStringSubmatch(doc); m != nil {
		d.ExternalURL = m[1]
		d.Title = html.UnescapeString(strings.TrimSpace(m[2]))
	} else if m := reH1Plain.FindStringSubmatch(doc); m != nil {
		d.Title = html.UnescapeString(strings.TrimSpace(m[1]))
	}

	if m := reTitleTag.FindStringSubmatch(doc); m != nil {
		d.Award = strings.TrimSpace(m[1])
	}

	// Dimension scores render in rubric order: Design, Usability, Creativity, Content.
	dims := reOverallDims.FindAllStringSubmatch(doc, -1)
	if len(dims) >= 4 {
		d.Scores.Design = parseFloat(dims[0][1])
		d.Scores.Usability = parseFloat(dims[1][1])
		d.Scores.Creativity = parseFloat(dims[2][1])
		d.Scores.Content = parseFloat(dims[3][1])
	}
	// Overall renders in several chart bars whose order is not stable; the
	// official rubric weighting over the four parsed dimensions is exact.
	if d.Scores.Design > 0 {
		d.Scores.Overall = round2(d.Scores.WeightedOverall())
	}

	for _, b := range reJuryBlock.FindAllString(doc, -1) {
		var v JuryVote
		if m := reJuryName.FindStringSubmatch(b); m != nil {
			v.Profile = m[1]
			v.Name = html.UnescapeString(m[2])
		} else if m := reVoteName.FindStringSubmatch(b); m != nil {
			v.Name = html.UnescapeString(strings.TrimSpace(m[1]))
			if pm := reVoteProfile.FindStringSubmatch(b); pm != nil {
				v.Profile = pm[1]
			}
		}
		if m := reJuryCountry.FindStringSubmatch(b); m != nil {
			v.Country = html.UnescapeString(strings.TrimSpace(m[1]))
		} else if m := reVoteCountry.FindStringSubmatch(b); m != nil {
			v.Country = html.UnescapeString(strings.TrimSpace(m[1]))
		}
		for _, sm := range reScoreItem.FindAllStringSubmatch(b, -1) {
			v.Scores = append(v.Scores, parseFloat(sm[1]))
		}
		if v.Name != "" {
			d.Jury = append(d.Jury, v)
		}
	}

	seenHex := map[string]bool{}
	for _, m := range rePaletteHex.FindAllStringSubmatch(doc, -1) {
		hex := strings.ToUpper(m[1])
		if !seenHex[hex] {
			seenHex[hex] = true
			d.Palette = append(d.Palette, hex)
		}
	}

	seenTag := map[string]bool{}
	for _, m := range reTagLink.FindAllStringSubmatch(doc, -1) {
		slug := m[1]
		if !seenTag[slug] {
			seenTag[slug] = true
			d.Tags = append(d.Tags, Tag{Slug: slug, Label: html.UnescapeString(strings.TrimSpace(m[2]))})
		}
	}

	if cb := reCreditBlock.FindString(doc); cb != "" {
		seenCred := map[string]bool{}
		for _, m := range reCreditLink.FindAllStringSubmatch(cb, -1) {
			if !seenCred[m[1]] {
				seenCred[m[1]] = true
				d.Credits = append(d.Credits, Credit{Username: m[1], DisplayName: html.UnescapeString(m[2])})
			}
		}
	}

	return d
}

// TagFilterSlug converts a tag display label to its /websites/<slug>/ filter form.
func TagFilterSlug(label string) string {
	s := strings.ToLower(strings.TrimSpace(label))
	s = strings.ReplaceAll(s, "&", "and")
	s = strings.ReplaceAll(s, ".", "-")
	s = strings.ReplaceAll(s, " ", "-")
	s = regexp.MustCompile(`[^a-z0-9_-]`).ReplaceAllString(s, "")
	return regexp.MustCompile(`-+`).ReplaceAllString(s, "-")
}

// TechVocabulary is the curated set of tag labels that count as technologies
// for trend/tech queries (tags on awwwards merge styles, categories, and tech).
var TechVocabulary = map[string]bool{
	"gsap": true, "three.js": true, "three-js": true, "webflow": true, "webgl": true,
	"react": true, "vue.js": true, "vue-js": true, "nuxt": true, "next.js": true, "next-js": true,
	"wordpress": true, "shopify": true, "svelte": true, "lottie": true, "css": true,
	"javascript": true, "typescript": true, "p5.js": true, "pixi.js": true, "html5": true,
	"craft-cms": true, "prismic": true, "sanity": true, "contentful": true, "framer": true,
	"readymag": true, "squarespace": true, "wix": true, "elementor": true, "matter.js": true,
	"anime.js": true, "barba.js": true, "locomotive-scroll": true, "lenis": true, "swup": true,
	"astro": true, "gatsby": true, "tailwind-css": true, "alpine.js": true, "jquery": true,
	"strapi": true, "storyblok": true, "kirby": true, "laravel": true, "spline": true, "unity": true,
	"blender": true, "cinema-4d": true, "houdini": true, "curtains.js": true, "ogl": true,
}

// IsTech reports whether a tag label or slug is in the tech vocabulary.
func IsTech(tag string) bool {
	t := strings.ToLower(strings.TrimSpace(tag))
	if TechVocabulary[t] {
		return true
	}
	return TechVocabulary[TagFilterSlug(tag)]
}

func parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func round2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}
