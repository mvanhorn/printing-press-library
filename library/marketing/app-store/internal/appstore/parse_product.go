// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package appstore

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ProductPage is everything the App Store product page model carries beyond
// the Lookup API: subtitle, Editor's Choice, the star histogram, the chart
// badge, privacy labels, version history, in-app purchase prices, in-app
// events, capabilities, accessibility, editorial placements and media.
type ProductPage struct {
	AdamID            string         `json:"adamId"`
	BundleID          string         `json:"bundleId,omitempty"`
	Storefront        string         `json:"storefront,omitempty"`
	Language          string         `json:"language,omitempty"`
	Title             string         `json:"title"`
	Subtitle          string         `json:"subtitle,omitempty"`
	DeveloperTagline  string         `json:"developerTagline,omitempty"`
	DeveloperName     string         `json:"developerName,omitempty"`
	DeveloperID       string         `json:"developerId,omitempty"`
	EditorsChoice     bool           `json:"editorsChoice"`
	CanonicalURL      string         `json:"canonicalUrl,omitempty"`
	AppPlatforms      []string       `json:"appPlatforms"`
	PriceText         string         `json:"priceText,omitempty"`
	IsFree            bool           `json:"isFree"`
	HasInAppPurchases bool           `json:"hasInAppPurchases"`
	RatingAverage     float64        `json:"ratingAverage"`
	TotalRatings      int64          `json:"totalRatings"`
	Histogram         []int64        `json:"histogram"`
	Description       string         `json:"description,omitempty"`
	ContentRating     string         `json:"contentRating,omitempty"`
	Badges            []Badge        `json:"badges"`
	ChartPosition     *ChartPosition `json:"chartPosition,omitempty"`
	Information       []Annotation   `json:"information"`
	AgeRating         *AgeRating     `json:"ageRating,omitempty"`
	InAppPurchases    []IAPItem      `json:"inAppPurchases"`
	// IAPNoneShown is true when the page positively shows no in-app
	// purchases: its Information section rendered, holds no In-App
	// Purchases annotation, and the offer says the app has none.
	IAPNoneShown    bool           `json:"-"`
	Capabilities    []Capability   `json:"capabilities"`
	Accessibility   Accessibility  `json:"accessibility"`
	Privacy         Privacy        `json:"privacy"`
	Versions        []VersionEntry `json:"versions"`
	Events          []AppEvent     `json:"events"`
	FeaturedIn      []FeaturedCard `json:"featuredIn"`
	SimilarItems    []AppLockup    `json:"similarItems"`
	MoreByDeveloper []AppLockup    `json:"moreByDeveloper"`
	Media           []MediaSet     `json:"media"`

	// FetchedAt is when the page was fetched (zero for fixtures); the store
	// uses it as the snapshot capture time.
	FetchedAt time.Time `json:"-"`
	FromCache bool      `json:"-"`
	// DataAsOf is FetchedAt minus the page copy's CDN Age.
	DataAsOf time.Time `json:"-"`
}

// Badge is one item of the information ribbon under the title.
type Badge struct {
	Key     string `json:"key"`
	Type    string `json:"type,omitempty"`
	Heading string `json:"heading,omitempty"`
	Caption string `json:"caption,omitempty"`
	Value   string `json:"value,omitempty"`
}

// ChartPosition is the "#N in <genre>" chart badge of a product page.
type ChartPosition struct {
	Chart     string `json:"chart"`
	GenreID   string `json:"genreId"`
	GenreName string `json:"genreName,omitempty"`
	Position  int    `json:"position"`
	Platform  string `json:"platform,omitempty"`
}

// String renders the badge compactly ("top-free/6011/1").
func (c *ChartPosition) String() string {
	if c == nil {
		return ""
	}
	return fmt.Sprintf("%s/%s/%d", c.Chart, c.GenreID, c.Position)
}

// Annotation is one row of the Information section (Seller, Size, Category,
// Compatibility, Languages, Age Rating, In-App Purchases, Copyright).
type Annotation struct {
	Title   string   `json:"title"`
	Summary string   `json:"summary,omitempty"`
	Lines   []string `json:"lines,omitempty"`
}

// AgeRating is the Age Rating annotation: the rating, Apple's sentence and
// the grouped descriptors ("In-App Controls", "Infrequent", "Frequent").
type AgeRating struct {
	Rating   string             `json:"rating"`
	Summary  string             `json:"summary,omitempty"`
	Sections []AgeRatingSection `json:"sections"`
}

// AgeRatingSection is one heading of the age-rating detail and its items.
type AgeRatingSection struct {
	Heading string   `json:"heading"`
	Items   []string `json:"items"`
}

// IAPItem is one in-app purchase price row (the page lists the top 10).
type IAPItem struct {
	Position int    `json:"position"`
	Name     string `json:"name"`
	Price    string `json:"price"`
}

// Capability is one "Supports" item (Siri, SharePlay, Game Center, ...).
type Capability struct {
	Type    string `json:"type,omitempty"`
	Title   string `json:"title"`
	Caption string `json:"caption,omitempty"`
}

// Accessibility reports whether the developer declared accessibility
// support and which features. Most third-party apps declare none.
type Accessibility struct {
	Declared bool     `json:"declared"`
	Features []string `json:"features"`
	Summary  string   `json:"summary,omitempty"`
}

// Privacy is the App Privacy (nutrition label) section.
type Privacy struct {
	PolicyURL string        `json:"policyUrl,omitempty"`
	Types     []PrivacyType `json:"privacyTypes"`
	// NoneShown is true when the page renders its App Privacy section and
	// that section lists nothing at all ("No Details Provided"): a positive
	// observation of no label, unlike a section the parser could not read
	// (items of an unknown kind) or a page without the section.
	NoneShown bool `json:"-"`
	// Unreadable is true when the section holds label items of a kind the
	// parser does not know and none it does: a parse gap, never "no labels".
	Unreadable bool `json:"-"`
}

// PrivacyType is one label type (Data Used to Track You, Data Linked to You,
// Data Not Linked to You, Data Not Collected). Categories is the union over
// purposes when the type is grouped by purpose.
type PrivacyType struct {
	Identifier string            `json:"identifier"`
	Title      string            `json:"title"`
	Detail     string            `json:"detail,omitempty"`
	Categories []PrivacyCategory `json:"categories"`
	Purposes   []PrivacyPurpose  `json:"purposes,omitempty"`
}

// PrivacyCategory is a data category (Contact Info, Identifiers ...).
type PrivacyCategory struct {
	Identifier string   `json:"identifier"`
	Title      string   `json:"title"`
	DataTypes  []string `json:"dataTypes"`
}

// PrivacyPurpose groups categories by purpose (Analytics, App Functionality ...).
type PrivacyPurpose struct {
	Identifier string            `json:"identifier"`
	Title      string            `json:"title"`
	Categories []PrivacyCategory `json:"categories"`
}

// VersionEntry is one version-history row. ReleasedAt is RFC3339 UTC.
type VersionEntry struct {
	Version    string `json:"version"`
	ReleasedAt string `json:"releasedAt"`
	Notes      string `json:"notes,omitempty"`
}

// ListingVersion is an app's current version as the fresher of its Lookup
// row and its product page shows it.
type ListingVersion struct {
	Version    string
	ReleasedAt string
	Notes      string
	// FromPage is true when the product page's newest version entry replaced
	// the Lookup copy's version.
	FromPage bool
	// LookupVersion is the Lookup copy's version.
	LookupVersion string
}

// CurrentVersion picks an app's current version (with its release date and
// notes) from its Lookup row and, when fetched, its product page. Apple's
// Lookup CDN serves per-URL copies up to 24 h old: live on 2026-10-23 a
// single-id Lookup copy from 06:35Z still said 9.1.92 while the page fetched
// in the same command listed 9.1.94, released 17:59Z. So the page's newest
// version entry wins when the page's data is newer than the Lookup copy's
// (both data times known) and names another version, or when the Lookup row
// has no version; otherwise the Lookup row's version stands.
func CurrentVersion(lk LookupResult, page *ProductPage) ListingVersion {
	v := ListingVersion{Version: lk.Version, ReleasedAt: lk.CurrentVersionReleaseDate, Notes: lk.ReleaseNotes, LookupVersion: lk.Version}
	if page == nil || len(page.Versions) == 0 || page.Versions[0].Version == "" {
		return v
	}
	newest := page.Versions[0] // parseVersions sorts newest first
	if newest.Version == lk.Version {
		return v
	}
	pageNewer := !page.DataAsOf.IsZero() && !lk.DataAsOf.IsZero() && page.DataAsOf.After(lk.DataAsOf)
	if lk.Version != "" && !pageNewer {
		return v
	}
	return ListingVersion{Version: newest.Version, ReleasedAt: newest.ReleasedAt, Notes: newest.Notes, FromPage: true, LookupVersion: lk.Version}
}

// AppEvent is one in-app event card.
type AppEvent struct {
	EventID   string `json:"eventId"`
	Title     string `json:"title"`
	Subtitle  string `json:"subtitle,omitempty"`
	Kind      string `json:"kind,omitempty"`
	StartDate string `json:"startDate,omitempty"`
	EndDate   string `json:"endDate,omitempty"`
	Badge     string `json:"badge,omitempty"`
	Detail    string `json:"detail,omitempty"`
}

// FeaturedCard is an editorial story that features the app.
type FeaturedCard struct {
	StoryID string `json:"storyId"`
	Title   string `json:"title"`
	Heading string `json:"heading,omitempty"`
}

// AppLockup is a compact app card (similar apps, charts, developer shelves).
// RatingCount is Apple's display text ("1.6k"); exact counts come from Lookup.
type AppLockup struct {
	Position      int     `json:"position"`
	AdamID        string  `json:"adamId"`
	BundleID      string  `json:"bundleId,omitempty"`
	Title         string  `json:"title"`
	Subtitle      string  `json:"subtitle,omitempty"`
	DeveloperName string  `json:"developerName,omitempty"`
	Rating        float64 `json:"rating"`
	RatingCount   string  `json:"ratingCount,omitempty"`
	EditorsChoice bool    `json:"editorsChoice"`
	AgeRating     string  `json:"ageRating,omitempty"`
	PriceText     string  `json:"priceText,omitempty"`
	IsFree        bool    `json:"isFree"`
	HasIAP        bool    `json:"hasInAppPurchases"`
	Platform      string  `json:"platform,omitempty"`
}

// MediaSet is the screenshots and preview videos for one device family
// (phone, pad, mac, tv, watch, vision).
type MediaSet struct {
	Device      string   `json:"device"`
	Screenshots []string `json:"screenshots"`
	Videos      []string `json:"videos,omitempty"`
}

// ParseProductPage parses a ProductPageIntent model.
func ParseProductPage(m *PageModel) (ProductPage, error) {
	if err := expectKind(m, KindProductPage); err != nil {
		return ProductPage{}, err
	}
	d := m.Data
	sm := objAt(d, "shelfMapping")
	if sm == nil {
		return ProductPage{}, fmt.Errorf("product page model has no shelfMapping")
	}
	lockup := objAt(d, "lockup")
	p := ProductPage{
		AdamID:           firstNonEmpty(strAt(lockup, "adamId"), m.IntentString("id")),
		BundleID:         strAt(lockup, "bundleId"),
		Storefront:       m.IntentString("storefront"),
		Language:         m.IntentString("language"),
		Title:            cleanText(firstNonEmpty(strAt(lockup, "title"), strAt(d, "title"))),
		Subtitle:         cleanText(strAt(lockup, "subtitle")),
		DeveloperTagline: cleanText(strAt(lockup, "developerTagline")),
		EditorsChoice:    boolOf(at(lockup, "isEditorsChoice")),
		CanonicalURL:     strAt(d, "canonicalURL"),
		AppPlatforms:     make([]string, 0),
		Histogram:        make([]int64, 0, 5),
		Badges:           make([]Badge, 0),
		Information:      make([]Annotation, 0),
		InAppPurchases:   make([]IAPItem, 0),
		Capabilities:     make([]Capability, 0),
		Versions:         make([]VersionEntry, 0),
		Events:           make([]AppEvent, 0),
		FeaturedIn:       make([]FeaturedCard, 0),
		SimilarItems:     make([]AppLockup, 0),
		MoreByDeveloper:  make([]AppLockup, 0),
		Media:            make([]MediaSet, 0),
		FetchedAt:        m.FetchedAt,
		FromCache:        m.FromCache,
		DataAsOf:         m.DataAsOf,
	}
	if p.AdamID == "" {
		return ProductPage{}, fmt.Errorf("product page model has no app id")
	}
	for _, v := range arrAt(d, "appPlatforms") {
		if s := strOf(v); s != "" {
			p.AppPlatforms = append(p.AppPlatforms, s)
		}
	}
	offer := objAt(d, "titleOfferDisplayProperties")
	if offer == nil {
		offer = objAt(lockup, "offerDisplayProperties")
	}
	p.PriceText = cleanText(firstNonEmpty(strAt(offer, "priceFormatted"), strAt(offer, "titles", "standard")))
	p.IsFree = boolOf(at(offer, "isFree"))
	p.HasInAppPurchases = boolOf(at(offer, "hasInAppPurchases"))
	if dev := objAt(d, "developerAction"); dev != nil {
		p.DeveloperName = cleanText(strAt(dev, "title"))
		p.DeveloperID = strAt(dev, "destination", "id")
	}

	// Ratings: productRatings.items[0] (ratingCounts are 5-star first).
	if r := objAt(sm, "productRatings", "items", "0"); r != nil {
		p.RatingAverage = floatOf(r["ratingAverage"])
		p.TotalRatings = intOf(r["totalNumberOfRatings"])
		for _, v := range asArray(r["ratingCounts"]) {
			p.Histogram = append(p.Histogram, intOf(v))
		}
	}

	if desc := objAt(sm, "description", "items", "0"); desc != nil {
		p.Description = cleanMultiline(strAt(desc, "paragraph", "text"))
	}

	parseRibbon(sm, &p)
	iapAnnotation := parseInformation(sm, &p)
	p.IAPNoneShown = len(p.Information) > 0 && !iapAnnotation && len(p.InAppPurchases) == 0 && !p.HasInAppPurchases
	p.Capabilities = parseCapabilities(sm)
	p.Accessibility = parseAccessibility(sm)
	p.Privacy = parsePrivacy(sm)
	p.Versions = parseVersions(sm)
	p.Events = parseEvents(sm)
	p.FeaturedIn = parseFeaturedIn(sm)
	p.SimilarItems = parseLockups(arrAt(sm, "similarItems", "items"))
	p.MoreByDeveloper = parseLockups(arrAt(sm, "moreByDeveloper", "items"))
	p.Media = parseMedia(sm)
	if p.DeveloperName == "" {
		for _, a := range p.Information {
			if a.Title == "Seller" {
				p.DeveloperName = a.Summary
			}
		}
	}
	return p, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func parseRibbon(sm jsonObject, p *ProductPage) {
	for _, it := range arrAt(sm, "informationRibbon", "items") {
		b := Badge{
			Key:     strAt(it, "key"),
			Type:    strAt(it, "type"),
			Heading: cleanText(strAt(it, "heading")),
			Caption: cleanText(strAt(it, "caption")),
		}
		content := objAt(it, "content")
		switch {
		case content["ratingFormatted"] != nil:
			b.Value = strOf(content["ratingFormatted"])
		case content["contentRating"] != nil:
			b.Value = strOf(content["contentRating"])
		case content["position"] != nil:
			b.Value = strOf(content["position"])
		case content["paragraphText"] != nil:
			b.Value = strOf(content["paragraphText"])
		}
		p.Badges = append(p.Badges, b)
		switch b.Key {
		case "contentRating":
			p.ContentRating = b.Value
		case "chartPosition":
			dest := objAt(it, "clickAction", "destination")
			pos, _ := strconv.Atoi(strings.TrimPrefix(b.Value, "#"))
			p.ChartPosition = &ChartPosition{
				Chart:     strAt(dest, "chart"),
				GenreID:   strAt(dest, "genreId"),
				GenreName: b.Caption,
				Position:  pos,
				Platform:  strAt(dest, "platform"),
			}
			if p.ChartPosition.Chart == "" {
				p.ChartPosition.Chart = chartFromURL(strAt(it, "clickAction", "pageUrl"))
			}
		}
	}
}

func chartFromURL(u string) string {
	if idx := strings.Index(u, "chart="); idx >= 0 {
		v := u[idx+len("chart="):]
		if amp := strings.IndexByte(v, '&'); amp >= 0 {
			v = v[:amp]
		}
		return v
	}
	return ""
}

// annotationLines renders an annotation's items as plain lines: linkableText
// markup is stripped, textPair becomes "leading: trailing", buttons and
// spacers are skipped.
func annotationLines(items []any) []string {
	lines := make([]string, 0)
	for _, it := range items {
		switch kindOf(it) {
		case "linkableText":
			raw := strAt(it, "linkableText", "styledText", "rawText")
			for _, l := range strings.Split(stripMarkup(raw), "\n") {
				if l = cleanText(l); l != "" {
					lines = append(lines, l)
				}
			}
		case "textEncapsulation":
			if t := cleanText(strAt(it, "text")); t != "" {
				lines = append(lines, t)
			}
		case "textPair":
			lead := cleanText(strAt(it, "leadingText"))
			trail := cleanText(strAt(it, "trailingText"))
			if lead != "" || trail != "" {
				lines = append(lines, strings.TrimSpace(lead+": "+trail))
			}
		}
	}
	return lines
}

// hasItemKind reports whether an annotation has an item of the given $kind.
func hasItemKind(items []any, kind string) bool {
	for _, it := range items {
		if kindOf(it) == kind {
			return true
		}
	}
	return false
}

// parseInformation reads the Information annotations. Titles are localized
// on non-English storefronts ("In-App Purchases" is not always English), so
// the IAP list is recognized by its textPair rows and the age rating by its
// textEncapsulation badge, with the English titles as a fallback. It reports
// whether an In-App Purchases annotation was present, even one whose rows it
// could not read.
func parseInformation(sm jsonObject, p *ProductPage) bool {
	iapAnnotation := false
	for _, it := range arrAt(sm, "information", "items") {
		title := cleanText(strAt(it, "title"))
		items := arrAt(it, "items")
		a := Annotation{Title: title, Summary: cleanText(strAt(it, "summary"))}
		section := title
		switch {
		case hasItemKind(items, "textPair"):
			section = "In-App Purchases"
		case hasItemKind(items, "textEncapsulation"):
			section = "Age Rating"
		}
		switch section {
		case "In-App Purchases":
			iapAnnotation = true
			pos := 0
			for _, row := range items {
				if kindOf(row) != "textPair" {
					continue // the trailing "Learn More" button
				}
				pos++
				p.InAppPurchases = append(p.InAppPurchases, IAPItem{
					Position: pos,
					Name:     cleanText(strAt(row, "leadingText")),
					Price:    cleanText(strAt(row, "trailingText")),
				})
			}
			if len(p.InAppPurchases) > 0 {
				p.HasInAppPurchases = true
			}
		case "Age Rating":
			p.AgeRating = parseAgeRating(a.Summary, items)
		}
		a.Lines = annotationLines(items)
		p.Information = append(p.Information, a)
	}
	return iapAnnotation
}

func parseAgeRating(summary string, items []any) *AgeRating {
	ar := &AgeRating{Rating: summary, Sections: make([]AgeRatingSection, 0)}
	for _, it := range items {
		switch kindOf(it) {
		case "textEncapsulation":
			if t := cleanText(strAt(it, "text")); t != "" {
				ar.Rating = t
			}
		case "linkableText":
			raw := strAt(it, "linkableText", "styledText", "rawText")
			if !strings.Contains(raw, "**") {
				if ar.Summary == "" {
					ar.Summary = cleanMultiline(stripMarkup(raw))
				}
				continue
			}
			var cur *AgeRatingSection
			for _, line := range strings.Split(raw, "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				if strings.HasPrefix(line, "**") && strings.HasSuffix(line, "**") {
					ar.Sections = append(ar.Sections, AgeRatingSection{Heading: cleanText(strings.Trim(line, "*")), Items: make([]string, 0)})
					cur = &ar.Sections[len(ar.Sections)-1]
					continue
				}
				if cur == nil {
					ar.Sections = append(ar.Sections, AgeRatingSection{Heading: "", Items: make([]string, 0)})
					cur = &ar.Sections[len(ar.Sections)-1]
				}
				cur.Items = append(cur.Items, cleanText(stripMarkup(line)))
			}
		}
	}
	return ar
}

func parseCapabilities(sm jsonObject) []Capability {
	out := make([]Capability, 0)
	for _, it := range arrAt(sm, "capabilities", "items") {
		title := cleanText(strAt(it, "title"))
		if title == "" {
			continue
		}
		out = append(out, Capability{
			Type:    strAt(it, "type"),
			Title:   title,
			Caption: cleanText(stripMarkup(strAt(it, "caption", "styledText", "rawText"))),
		})
	}
	return out
}

func parseAccessibility(sm jsonObject) Accessibility {
	acc := Accessibility{Features: make([]string, 0)}
	header := strAt(sm, "accessibilityHeader", "items", "0", "text", "styledText", "rawText")
	acc.Summary = strings.TrimSpace(strings.TrimSuffix(cleanText(stripMarkup(header)), "Learn More"))
	seen := map[string]bool{}
	for _, group := range arrAt(sm, "accessibilityFeatures", "items") {
		for _, f := range arrAt(group, "features") {
			t := cleanText(strAt(f, "title"))
			if t != "" && !seen[t] {
				seen[t] = true
				acc.Features = append(acc.Features, t)
			}
		}
	}
	lower := strings.ToLower(acc.Summary)
	switch {
	case len(acc.Features) > 0:
		acc.Declared = true
	case strings.Contains(lower, "not yet indicated") || strings.Contains(lower, "has not"):
		acc.Declared = false
	case strings.Contains(lower, "indicated that this app supports"):
		acc.Declared = true
	}
	return acc
}

func parsePrivacyCategories(items []any) []PrivacyCategory {
	out := make([]PrivacyCategory, 0)
	for _, c := range items {
		cat := PrivacyCategory{
			Identifier: strAt(c, "identifier"),
			Title:      cleanText(strAt(c, "title")),
			DataTypes:  make([]string, 0),
		}
		for _, dt := range arrAt(c, "dataTypes") {
			if s := cleanText(strOf(dt)); s != "" {
				cat.DataTypes = append(cat.DataTypes, s)
			}
		}
		out = append(out, cat)
	}
	return out
}

// mergeCategories unions categories (by identifier, first-seen order) and
// their data types.
func mergeCategories(dst []PrivacyCategory, src []PrivacyCategory) []PrivacyCategory {
	index := map[string]int{}
	for i, c := range dst {
		index[c.Identifier] = i
	}
	for _, c := range src {
		i, ok := index[c.Identifier]
		if !ok {
			cp := PrivacyCategory{Identifier: c.Identifier, Title: c.Title, DataTypes: append(make([]string, 0, len(c.DataTypes)), c.DataTypes...)}
			dst = append(dst, cp)
			index[c.Identifier] = len(dst) - 1
			continue
		}
		have := map[string]bool{}
		for _, dt := range dst[i].DataTypes {
			have[dt] = true
		}
		for _, dt := range c.DataTypes {
			if !have[dt] {
				dst[i].DataTypes = append(dst[i].DataTypes, dt)
				have[dt] = true
			}
		}
	}
	return dst
}

func parsePrivacyType(it any) PrivacyType {
	pt := PrivacyType{
		Identifier: strAt(it, "identifier"),
		Title:      cleanText(strAt(it, "title")),
		Detail:     cleanText(strAt(it, "detail")),
		Categories: make([]PrivacyCategory, 0),
	}
	pt.Categories = mergeCategories(pt.Categories, parsePrivacyCategories(arrAt(it, "categories")))
	for _, purpose := range arrAt(it, "purposes") {
		pp := PrivacyPurpose{
			Identifier: strAt(purpose, "identifier"),
			Title:      cleanText(strAt(purpose, "title")),
			Categories: parsePrivacyCategories(arrAt(purpose, "categories")),
		}
		pt.Purposes = append(pt.Purposes, pp)
		pt.Categories = mergeCategories(pt.Categories, pp.Categories)
	}
	return pt
}

func privacyPolicyURL(header any) string {
	for _, v := range objAt(header, "bodyText", "linkedSubstrings") {
		if u := strAt(v, "url"); strings.HasPrefix(u, "http") {
			return u
		}
	}
	return ""
}

func parsePrivacy(sm jsonObject) Privacy {
	pv := Privacy{Types: make([]PrivacyType, 0)}
	header := objAt(sm, "privacyHeader")
	pv.PolicyURL = privacyPolicyURL(at(header, "items", "0"))
	// unread counts label items of a kind the parser does not know: with
	// them, an empty Types is a parse gap, not an observed absence.
	unread := 0
	// The detail page (seeAllAction.pageData) carries data types and purposes.
	for _, shelf := range arrAt(header, "seeAllAction", "pageData", "shelves") {
		for _, it := range arrAt(shelf, "items") {
			switch kindOf(it) {
			case "PrivacyType":
				pv.Types = append(pv.Types, parsePrivacyType(it))
			case "PrivacyHeader":
				if pv.PolicyURL == "" {
					pv.PolicyURL = privacyPolicyURL(it)
				}
			default:
				unread++
			}
		}
	}
	if len(pv.Types) == 0 {
		// Fall back to the product page cards (categories without data types).
		for _, it := range arrAt(sm, "privacyTypes", "items") {
			if kindOf(it) == "PrivacyType" {
				pv.Types = append(pv.Types, parsePrivacyType(it))
			} else {
				unread++
			}
		}
	}
	pv.NoneShown = header != nil && len(pv.Types) == 0 && unread == 0
	pv.Unreadable = len(pv.Types) == 0 && unread > 0
	return pv
}

func parseVersionItem(it any) (VersionEntry, bool) {
	version := cleanText(strAt(it, "primarySubtitle"))
	version = strings.TrimSpace(strings.TrimPrefix(version, "Version "))
	if version == "" {
		return VersionEntry{}, false
	}
	date := strAt(it, "secondarySubtitle")
	return VersionEntry{
		Version:    version,
		ReleasedAt: normalizeDate(date),
		Notes:      cleanMultiline(strAt(it, "text")),
	}, true
}

func parseVersions(sm jsonObject) []VersionEntry {
	out := make([]VersionEntry, 0)
	seen := map[string]bool{}
	for _, shelf := range arrAt(sm, "mostRecentVersion", "seeAllAction", "pageData", "shelves") {
		for _, it := range arrAt(shelf, "items") {
			if v, ok := parseVersionItem(it); ok && !seen[v.Version] {
				seen[v.Version] = true
				out = append(out, v)
			}
		}
	}
	if len(out) == 0 {
		for _, it := range arrAt(sm, "mostRecentVersion", "items") {
			if v, ok := parseVersionItem(it); ok && !seen[v.Version] {
				seen[v.Version] = true
				out = append(out, v)
			}
		}
	}
	// Newest first by release date; unparseable dates keep their order.
	sort.SliceStable(out, func(i, j int) bool {
		ti, iok := parseAppleTime(out[i].ReleasedAt)
		tj, jok := parseAppleTime(out[j].ReleasedAt)
		if !iok || !jok {
			return false
		}
		return ti.After(tj)
	})
	return out
}

func parseEvents(sm jsonObject) []AppEvent {
	out := make([]AppEvent, 0)
	for _, it := range arrAt(sm, "appEvents", "items") {
		if kindOf(it) != "AppEvent" {
			continue
		}
		out = append(out, AppEvent{
			EventID:   strAt(it, "appEventId"),
			Title:     cleanText(strAt(it, "title")),
			Subtitle:  cleanText(strAt(it, "subtitle")),
			Kind:      cleanText(strAt(it, "kind")),
			StartDate: normalizeDate(strAt(it, "startDate")),
			EndDate:   normalizeDate(strAt(it, "endDate")),
			Badge:     strAt(it, "appEventBadgeKind"),
			Detail:    cleanText(strAt(it, "detail")),
		})
	}
	return out
}

var storyIDRe = regexp.MustCompile(`/story/id(\d+)`)

func storyIDOf(card any) string {
	dest := objAt(card, "clickAction", "destination")
	if kindOf(dest) == KindArticlePage {
		if id := strAt(dest, "id"); id != "" {
			return id
		}
	}
	if m := storyIDRe.FindStringSubmatch(strAt(card, "clickAction", "pageUrl")); m != nil {
		return m[1]
	}
	return ""
}

func parseFeaturedIn(sm jsonObject) []FeaturedCard {
	out := make([]FeaturedCard, 0)
	for _, it := range arrAt(sm, "featuredIn", "items") {
		out = append(out, FeaturedCard{
			StoryID: storyIDOf(it),
			Title:   cleanText(strAt(it, "title")),
			Heading: cleanText(strAt(it, "heading")),
		})
	}
	return out
}

// lockupFrom reads one Lockup card.
func lockupFrom(it any) AppLockup {
	offer := objAt(it, "offerDisplayProperties")
	l := AppLockup{
		AdamID:        strAt(it, "adamId"),
		BundleID:      strAt(it, "bundleId"),
		Title:         cleanText(strAt(it, "title")),
		Subtitle:      cleanText(strAt(it, "subtitle")),
		DeveloperName: cleanText(strAt(it, "developerName")),
		Rating:        floatOf(at(it, "rating")),
		RatingCount:   strAt(it, "ratingCount"),
		EditorsChoice: boolOf(at(it, "isEditorsChoice")),
		AgeRating:     strAt(it, "ageRating"),
		PriceText:     cleanText(firstNonEmpty(strAt(offer, "priceFormatted"), strAt(offer, "titles", "standard"))),
		IsFree:        boolOf(at(offer, "isFree")),
		HasIAP:        boolOf(at(offer, "hasInAppPurchases")),
		Platform:      strAt(it, "clickAction", "destination", "platform"),
	}
	if l.Subtitle == "" {
		l.Subtitle = cleanText(strAt(it, "developerTagline"))
	}
	return l
}

func parseLockups(items []any) []AppLockup {
	out := make([]AppLockup, 0, len(items))
	for _, it := range items {
		if kindOf(it) != "Lockup" || strAt(it, "adamId") == "" {
			continue
		}
		l := lockupFrom(it)
		l.Position = len(out) + 1
		out = append(out, l)
	}
	return out
}

// artworkURL fills an mzstatic artwork template ("…/{w}x{h}{c}.{f}").
func artworkURL(art any) string {
	tmpl := strAt(art, "template")
	if tmpl == "" {
		return ""
	}
	w := strAt(art, "width")
	h := strAt(art, "height")
	if w == "" || w == "0" {
		w = "1242"
	}
	if h == "" || h == "0" {
		h = "2688"
	}
	f := "jpg"
	if format := strAt(art, "variants", "0", "format"); format != "" && format != "jpeg" {
		f = format
	}
	r := strings.NewReplacer("{w}", w, "{h}", h, "{c}", "bb", "{f}", f)
	return r.Replace(tmpl)
}

func parseMedia(sm jsonObject) []MediaSet {
	keys := make([]string, 0)
	for k := range sm {
		if strings.HasPrefix(k, "product_media_") {
			keys = append(keys, k)
		}
	}
	order := map[string]int{"phone": 0, "pad": 1, "mac": 2, "vision": 3, "tv": 4, "watch": 5}
	device := func(k string) string {
		return strings.Trim(strings.TrimPrefix(k, "product_media_"), "_")
	}
	sort.Slice(keys, func(i, j int) bool {
		oi, iok := order[device(keys[i])]
		oj, jok := order[device(keys[j])]
		if iok && jok {
			return oi < oj
		}
		if iok != jok {
			return iok
		}
		return keys[i] < keys[j]
	})
	out := make([]MediaSet, 0, len(keys))
	for _, k := range keys {
		set := MediaSet{Device: device(k), Screenshots: make([]string, 0)}
		for _, it := range arrAt(sm, k, "items") {
			if shot := objAt(it, "screenshot"); shot != nil {
				if u := artworkURL(shot); u != "" {
					set.Screenshots = append(set.Screenshots, u)
				}
			}
			if v := strAt(it, "video", "videoUrl"); v != "" {
				set.Videos = append(set.Videos, v)
			}
		}
		if len(set.Screenshots) > 0 || len(set.Videos) > 0 {
			out = append(out, set)
		}
	}
	return out
}

// HistogramSum adds the five star counts.
func (p ProductPage) HistogramSum() int64 {
	var n int64
	for _, v := range p.Histogram {
		n += v
	}
	return n
}

// ScreenshotCount counts screenshots across devices (compact output).
func (p ProductPage) ScreenshotCount() int {
	n := 0
	for _, m := range p.Media {
		n += len(m.Screenshots)
	}
	return n
}
