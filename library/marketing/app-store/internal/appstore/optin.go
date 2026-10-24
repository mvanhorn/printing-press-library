// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package appstore

// Opt-in surfaces. apps.apple.com's robots.txt disallows */search?*, /api/*,
// /v1/*, /WebObjects/* and /includes/*; the owner approved the web search
// page and /api at the absorb gate only as explicit opt-ins (--source web |
// --source api), never by default, never in sync or bulk capture.
//
// The client enforces this for every request, not only in the methods below:
// fetch refuses any URL OptInURL reports (Page, PageOfKind, the typed page
// methods and every redirect hop included) until EnableOptInSurfaces is
// called, and /api is paced at 0.5 requests/s. The search-hints endpoint
// (search.itunes.apple.com) is opt-in by command: only the explicit
// `suggest` command calls Hints.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// WebSearch fetches the web store's search page (closer to on-device keyword
// order than the Search API) and returns the ranked app ids.
func (c *Client) WebSearch(ctx context.Context, country, platform, term string) (*SearchPage, error) {
	if !c.optInEnabled() {
		return nil, ErrOptInRequired
	}
	cc, err := c.resolveCountry(country)
	if err != nil {
		return nil, err
	}
	p, err := NormalizePlatform(platform)
	if err != nil {
		return nil, err
	}
	term = strings.TrimSpace(term)
	if term == "" {
		return nil, fmt.Errorf("search term is empty")
	}
	_, parsed, err := c.fetchPage(ctx, WebSearchURL(cc, p, term), searchKindWant)
	if err != nil {
		return nil, err
	}
	page, ok := parsed.value.(SearchPage)
	if !ok {
		return nil, fmt.Errorf("web search page did not parse as a search page")
	}
	if page.Term == "" {
		page.Term = term
	}
	return &page, nil
}

// apiErrors turns an /api {"errors": [...]} body into an *EnvelopeError.
func apiErrors(top map[string]any, surface string) error {
	errs := asArray(top["errors"])
	if len(errs) == 0 {
		return nil
	}
	parts := make([]string, 0, 4)
	for _, k := range []string{"status", "code", "title", "detail"} {
		if s := strings.TrimSpace(strAt(errs[0], k)); s != "" {
			parts = append(parts, k+"="+s)
		}
	}
	return &EnvelopeError{Surface: surface, Reason: "Apple answered with errors: " + strings.Join(parts, " ")}
}

// apiHeaders are what the web app sends to its JSON proxy: an empty bearer
// (Apple's edge supplies the token) and the store origin.
var apiHeaders = map[string]string{
	"Authorization": "Bearer ",
	"Origin":        "https://apps.apple.com",
	"Referer":       "https://apps.apple.com/",
}

// APIReviewsPage is one page of the /api reviews resource. It adds developer
// responses, which no other surface carries. FetchedAt is when Apple served
// it (a cached page keeps its original fetch time).
type APIReviewsPage struct {
	Country   string    `json:"country"`
	AppID     string    `json:"appId"`
	Offset    int       `json:"offset"`
	Next      string    `json:"next,omitempty"`
	Reviews   []Review  `json:"reviews"`
	FetchedAt time.Time `json:"-"`
	FromCache bool      `json:"-"`
}

// APIReviews fetches up to 20 reviews from the opt-in /api reviews resource.
// sort is recent (default) or helpful.
func (c *Client) APIReviews(ctx context.Context, country, appID string, offset, limit int, sort string) (*APIReviewsPage, error) {
	if !c.optInEnabled() {
		return nil, ErrOptInRequired
	}
	cc, err := c.resolveCountry(country)
	if err != nil {
		return nil, err
	}
	id := strings.TrimPrefix(strings.TrimSpace(appID), "id")
	if !IsAppID(id) {
		return nil, fmt.Errorf("invalid App Store id %q (want digits)", appID)
	}
	if limit <= 0 || limit > 20 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	apiSort := "recent"
	switch strings.ToLower(strings.TrimSpace(sort)) {
	case "", "recent", "mostrecent":
	case "helpful", "mosthelpful":
		apiSort = "helpful"
	default:
		apiSort = strings.ToLower(strings.TrimSpace(sort))
	}
	v := url.Values{}
	v.Set("platform", "web")
	v.Set("sort", apiSort)
	v.Set("limit", strconv.Itoa(limit))
	v.Set("offset", strconv.Itoa(offset))
	v.Set("l", firstNonEmpty(c.lang, "en-US"))
	rawURL := fmt.Sprintf("%s/api/apps/v1/catalog/%s/apps/%s/reviews?%s", appsBase, cc, id, v.Encode())
	res, err := c.fetch(ctx, fetchRequest{URL: rawURL, Accept: acceptJSON, Headers: apiHeaders})
	if err != nil {
		return nil, err
	}
	page, perr := ParseAPIReviews(res.Body)
	outcome := OutcomeOK
	if perr == nil && len(page.Reviews) == 0 {
		outcome = OutcomeVerifiedEmpty
	}
	oldest, newest := reviewSpan(page.Reviews)
	c.finish(ctx, res, outcome, len(page.Reviews), oldest, newest, perr)
	if perr != nil {
		return nil, perr
	}
	page.Country, page.AppID, page.Offset = cc, id, offset
	page.FetchedAt, page.FromCache = res.FetchedAt, res.FromCache
	return &page, nil
}

// ParseAPIReviews reads either the dedicated reviews resource
// ({next, data:[user-reviews]}) or an app resource that includes reviews
// (data[0].relationships.reviews). The body must carry a "data" array; an
// {"errors": [...]} body, an empty object, null, or data items none of which
// is a user review are *EnvelopeError values, not an empty page.
func ParseAPIReviews(body []byte) (APIReviewsPage, error) {
	const surface = "App Store API reviews"
	top, err := decodeJSONAny(body, surface)
	if err != nil {
		return APIReviewsPage{}, err
	}
	if err := apiErrors(top, surface); err != nil {
		return APIReviewsPage{}, err
	}
	items, ok := top["data"].([]any)
	if !ok {
		return APIReviewsPage{}, &EnvelopeError{Surface: surface, Reason: "the body has no data array"}
	}
	page := APIReviewsPage{Reviews: make([]Review, 0), Next: strOf(top["next"])}
	if len(items) > 0 && strAt(items[0], "type") == "apps" {
		rel := objAt(items[0], "relationships", "reviews")
		included, ok := rel["data"].([]any)
		if !ok {
			return APIReviewsPage{}, &EnvelopeError{Surface: surface, Reason: "the app resource carries no included reviews"}
		}
		page.Next = strAt(rel, "next")
		items = included
	}
	for _, it := range items {
		if t := strAt(it, "type"); t != "" && t != "user-reviews" {
			continue
		}
		attr := objAt(it, "attributes")
		r := Review{
			ID:         strAt(it, "id"),
			Rating:     int(intOf(attr["rating"])),
			Title:      cleanText(strAt(attr, "title")),
			Body:       cleanMultiline(strAt(attr, "review")),
			Author:     cleanText(strAt(attr, "userName")),
			ReviewedAt: normalizeDate(strAt(attr, "date")),
			Edited:     boolOf(attr["isEdited"]),
		}
		if resp := objAt(attr, "developerResponse"); resp != nil {
			r.DeveloperResponse = cleanMultiline(strAt(resp, "body"))
		}
		if r.ID != "" {
			page.Reviews = append(page.Reviews, r)
		}
	}
	if len(items) > 0 && len(page.Reviews) == 0 {
		return APIReviewsPage{}, &EnvelopeError{Surface: surface, Reason: fmt.Sprintf("none of the %d data items is a user review with an id", len(items))}
	}
	return page, nil
}

// APIAppOptions selects the /api app resource views and relationships.
type APIAppOptions struct {
	Platform string   // iphone (default)
	Views    []string // default categorizations, top-in-app-purchasables
	Include  []string
	Extend   []string
}

// APIApp is the parsed /api app resource: App Store tags, the full top
// in-app purchase list with exact prices, chart positions and the rating
// histogram (1-star first in this resource; Histogram here is 5-star first).
type APIApp struct {
	AppID          string          `json:"appId"`
	Name           string          `json:"name"`
	ArtistName     string          `json:"artistName,omitempty"`
	URL            string          `json:"url,omitempty"`
	Genres         []APIGenre      `json:"genres"`
	Tags           []string        `json:"tags"`
	InAppPurchases []APIIAP        `json:"inAppPurchases"`
	ChartPosition  *ChartPosition  `json:"chartPosition,omitempty"`
	RatingAverage  float64         `json:"ratingAverage"`
	RatingCount    int64           `json:"ratingCount"`
	Histogram      []int64         `json:"histogram"`
	Raw            json.RawMessage `json:"-"`
	FetchedAt      time.Time       `json:"-"`
	FromCache      bool            `json:"-"`
}

// APIGenre is a genre from the categorizations view.
type APIGenre struct {
	ID       string `json:"genreId"`
	Name     string `json:"name"`
	ParentID string `json:"parentId,omitempty"`
}

// APIIAP is one in-app purchase from the top-in-app-purchasables view.
type APIIAP struct {
	Position       int     `json:"position"`
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Price          float64 `json:"price"`
	PriceFormatted string  `json:"priceFormatted"`
	Currency       string  `json:"currency,omitempty"`
	IsSubscription bool    `json:"isSubscription"`
}

// APIApp fetches the opt-in /api app resource.
func (c *Client) APIApp(ctx context.Context, country, appID string, opts APIAppOptions) (*APIApp, error) {
	if !c.optInEnabled() {
		return nil, ErrOptInRequired
	}
	cc, err := c.resolveCountry(country)
	if err != nil {
		return nil, err
	}
	id := strings.TrimPrefix(strings.TrimSpace(appID), "id")
	if !IsAppID(id) {
		return nil, fmt.Errorf("invalid App Store id %q (want digits)", appID)
	}
	platform, err := NormalizePlatform(opts.Platform)
	if err != nil {
		return nil, err
	}
	views := opts.Views
	if len(views) == 0 {
		views = []string{"categorizations", "top-in-app-purchasables"}
	}
	v := url.Values{}
	v.Set("platform", platform)
	v.Set("additionalPlatforms", "appletv,ipad,iphone,mac,realityDevice,watch")
	v.Set("views", strings.Join(views, ","))
	if len(opts.Include) > 0 {
		v.Set("include", strings.Join(opts.Include, ","))
	}
	if len(opts.Extend) > 0 {
		v.Set("extend", strings.Join(opts.Extend, ","))
	}
	v.Set("l", firstNonEmpty(c.lang, "en-US"))
	rawURL := fmt.Sprintf("%s/api/apps/v1/catalog/%s/apps/%s?%s", appsBase, cc, id, v.Encode())
	res, err := c.fetch(ctx, fetchRequest{URL: rawURL, Accept: acceptJSON, Headers: apiHeaders})
	if err != nil {
		return nil, err
	}
	app, perr := ParseAPIApp(res.Body)
	outcome := OutcomeOK
	if perr == nil && app.AppID == "" {
		outcome = OutcomeVerifiedEmpty
	}
	c.finish(ctx, res, outcome, len(app.InAppPurchases), "", "", perr)
	if perr != nil {
		return nil, perr
	}
	if app.AppID == "" {
		return nil, &NotFoundError{URL: rawURL, What: "app " + id + " in storefront " + cc}
	}
	app.FetchedAt, app.FromCache = res.FetchedAt, res.FromCache
	return &app, nil
}

// ParseAPIApp reads the /api app resource ({data:[{id, attributes, views}]}).
// An empty data array is "no such app"; a body without a data array, an
// {"errors": [...]} body, or a first item that is not an "apps" resource or
// has no id is an *EnvelopeError.
func ParseAPIApp(body []byte) (APIApp, error) {
	const surface = "App Store API app"
	top, err := decodeJSONAny(body, surface)
	if err != nil {
		return APIApp{}, err
	}
	if err := apiErrors(top, surface); err != nil {
		return APIApp{}, err
	}
	data, ok := top["data"].([]any)
	if !ok {
		return APIApp{}, &EnvelopeError{Surface: surface, Reason: "the body has no data array"}
	}
	app := APIApp{Genres: make([]APIGenre, 0), Tags: make([]string, 0), InAppPurchases: make([]APIIAP, 0), Histogram: make([]int64, 0, 5), Raw: body}
	if len(data) == 0 {
		return app, nil
	}
	d := data[0]
	if t := strAt(d, "type"); t != "" && t != "apps" {
		return APIApp{}, &EnvelopeError{Surface: surface, Reason: fmt.Sprintf("the first data item is a %q resource, not an app", t)}
	}
	attr := objAt(d, "attributes")
	app.AppID = strAt(d, "id")
	if app.AppID == "" {
		return APIApp{}, &EnvelopeError{Surface: surface, Reason: "the first data item has no id"}
	}
	app.Name = cleanText(strAt(attr, "name"))
	app.ArtistName = cleanText(strAt(attr, "artistName"))
	app.URL = strAt(attr, "url")
	if cp := objAt(attr, "chartPositions", "appStore"); cp != nil {
		app.ChartPosition = &ChartPosition{
			Chart:     strAt(cp, "chart"),
			GenreID:   strAt(cp, "genre"),
			GenreName: cleanText(strAt(cp, "genreName")),
			Position:  int(intOf(cp["position"])),
		}
	}
	if ur := objAt(attr, "userRating"); ur != nil {
		app.RatingAverage = floatOf(ur["value"])
		app.RatingCount = intOf(ur["ratingCount"])
		counts := asArray(ur["ratingCountList"]) // 1-star first
		for i := len(counts) - 1; i >= 0; i-- {
			app.Histogram = append(app.Histogram, intOf(counts[i]))
		}
	}
	for _, it := range arrAt(d, "views", "categorizations", "data") {
		switch strAt(it, "type") {
		case "genres":
			app.Genres = append(app.Genres, APIGenre{ID: strAt(it, "id"), Name: cleanText(strAt(it, "attributes", "name")), ParentID: strAt(it, "attributes", "parentId")})
		case "tags":
			if name := cleanText(strAt(it, "attributes", "name")); name != "" {
				app.Tags = append(app.Tags, name)
			}
		}
	}
	for _, it := range arrAt(d, "views", "top-in-app-purchasables", "data") {
		attrs := objAt(it, "attributes")
		offer := objAt(attrs, "offers", "0")
		app.InAppPurchases = append(app.InAppPurchases, APIIAP{
			Position:       len(app.InAppPurchases) + 1,
			ID:             strAt(it, "id"),
			Name:           cleanText(strAt(attrs, "name")),
			Price:          floatOf(offer["price"]),
			PriceFormatted: cleanText(strAt(offer, "priceFormatted")),
			Currency:       strAt(offer, "currencyCode"),
			IsSubscription: boolOf(attrs["isSubscription"]),
		})
	}
	return app, nil
}
