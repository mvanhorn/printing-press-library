// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package appstore

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const (
	appsBase   = "https://apps.apple.com"
	itunesBase = "https://itunes.apple.com"
	hintsURL   = "https://search.itunes.apple.com/WebObjects/MZSearchHints.woa/wa/hints"
)

// Platforms accepted by the web pages (path segment or ?platform=).
var webPlatforms = map[string]string{
	"iphone":    "iphone",
	"ios":       "iphone",
	"phone":     "iphone",
	"ipad":      "ipad",
	"pad":       "ipad",
	"ipados":    "ipad",
	"mac":       "mac",
	"macos":     "mac",
	"appletv":   "appletv",
	"apple-tv":  "appletv",
	"tv":        "appletv",
	"tvos":      "appletv",
	"watch":     "watch",
	"watchos":   "watch",
	"vision":    "vision",
	"visionos":  "vision",
	"visionpro": "vision",
}

// NormalizePlatform maps a platform name or alias to the web store's value
// (iphone, ipad, mac, appletv, watch, vision). Empty means iphone.
func NormalizePlatform(s string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(s))
	if key == "" {
		return "iphone", nil
	}
	if p, ok := webPlatforms[key]; ok {
		return p, nil
	}
	return "", fmt.Errorf("unknown platform %q; use iphone, ipad, mac, appletv, watch or vision", s)
}

// ChartPlatforms are the platforms with web chart pages.
var ChartPlatforms = []string{"iphone", "ipad", "mac"}

// CanonicalAppURL strips Apple's affiliate/tracking query ("?uo=4", "?uo=2")
// from a store URL.
func CanonicalAppURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	q.Del("uo")
	q.Del("at")
	q.Del("ct")
	u.RawQuery = q.Encode()
	return u.String()
}

// ProductPageURL builds an app page URL. Without a slug Apple answers a 301
// to the canonical URL, which the client follows (one extra request); pass
// the Lookup trackViewUrl through CanonicalAppURL to avoid it.
func ProductPageURL(country, appID, platform string) string {
	u := fmt.Sprintf("%s/%s/app/id%s", appsBase, country, appID)
	if platform != "" && platform != "iphone" {
		u += "?platform=" + url.QueryEscape(platform)
	}
	return u
}

// ChartsPageURL builds a web chart page URL.
func ChartsPageURL(country, platform, genreID, chart, ageBandID string) string {
	q := url.Values{}
	q.Set("chart", chart)
	if ageBandID != "" {
		q.Set("ageBandId", ageBandID)
	}
	return fmt.Sprintf("%s/%s/%s/charts/%s?%s", appsBase, country, platform, genreID, q.Encode())
}

// DeveloperPageURL builds a developer page URL (301s to the slugged URL).
func DeveloperPageURL(country, developerID string) string {
	return fmt.Sprintf("%s/%s/developer/id%s", appsBase, country, developerID)
}

// TodayPageURL builds the Today tab URL for a platform.
func TodayPageURL(country, platform string) string {
	return fmt.Sprintf("%s/%s/%s/today", appsBase, country, platform)
}

// SeeAllPageURL builds a product see-all URL (see-all=reviews,
// customers-also-bought-apps, developer-other-apps ...).
func SeeAllPageURL(country, appID, seeAll, platform string) string {
	q := url.Values{}
	q.Set("see-all", seeAll)
	if platform != "" {
		q.Set("platform", platform)
	}
	return fmt.Sprintf("%s/%s/app/id%s?%s", appsBase, country, appID, q.Encode())
}

// WebSearchURL builds the (robots-disallowed, opt-in) web search page URL.
func WebSearchURL(country, platform, term string) string {
	return fmt.Sprintf("%s/%s/%s/search?term=%s", appsBase, country, platform, url.QueryEscape(term))
}

// ChartFeedURL builds a legacy RSS chart URL.
func ChartFeedURL(country, feed, genreID string, limit int) string {
	u := fmt.Sprintf("%s/%s/rss/%s/limit=%d", itunesBase, country, feed, limit)
	if genreID != "" {
		u += "/genre=" + genreID
	}
	return u + "/json"
}

// ReviewsFeedURL builds a customer-reviews RSS URL.
func ReviewsFeedURL(country, appID string, page int, sort string) string {
	return fmt.Sprintf("%s/%s/rss/customerreviews/page=%d/id=%s/sortby=%s/json", itunesBase, country, page, appID, sort)
}

// LegacyChartFeeds are the legacy RSS chart feed names Apple still serves.
// The Mac feeds other than topfreemacapps were empty in 2025-11.
var LegacyChartFeeds = map[string]bool{
	"topfreeapplications":         true,
	"toppaidapplications":         true,
	"topgrossingapplications":     true,
	"topfreeipadapplications":     true,
	"toppaidipadapplications":     true,
	"topgrossingipadapplications": true,
	"topfreemacapps":              true,
	"toppaidmacapps":              true,
	"topgrossingmacapps":          true,
	"topmacapps":                  true,
	"newapplications":             true,
	"newfreeapplications":         true,
	"newpaidapplications":         true,
}

// ChartFeedName maps a chart (top-free, top-paid, top-grossing) and platform
// (iphone, ipad, mac) to the legacy RSS feed name.
func ChartFeedName(chart, platform string) (string, error) {
	kind := ""
	switch strings.ToLower(strings.TrimSpace(chart)) {
	case "top-free", "topfree", "free":
		kind = "free"
	case "top-paid", "toppaid", "paid":
		kind = "paid"
	case "top-grossing", "topgrossing", "grossing":
		kind = "grossing"
	default:
		return "", fmt.Errorf("unknown chart %q; use top-free, top-paid or top-grossing", chart)
	}
	p, err := NormalizePlatform(platform)
	if err != nil {
		return "", err
	}
	switch p {
	case "iphone":
		return "top" + kind + "applications", nil
	case "ipad":
		return "top" + kind + "ipadapplications", nil
	case "mac":
		return "top" + kind + "macapps", nil
	}
	return "", fmt.Errorf("no legacy RSS chart for platform %q (use iphone, ipad or mac)", platform)
}

// NormalizeChart maps chart aliases to the web chart names (top-free,
// top-paid) or top-grossing (RSS only).
func NormalizeChart(chart string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(chart)) {
	case "", "top-free", "topfree", "free":
		return "top-free", nil
	case "top-paid", "toppaid", "paid":
		return "top-paid", nil
	case "top-grossing", "topgrossing", "grossing":
		return "top-grossing", nil
	}
	return "", fmt.Errorf("unknown chart %q; use top-free, top-paid or top-grossing", chart)
}

// NormalizeReviewSort maps recent|helpful (and the feed spellings) to the
// RSS sortby values.
func NormalizeReviewSort(sort string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(sort)) {
	case "", "recent", "mostrecent", "most-recent", "newest":
		return SortMostRecent, nil
	case "helpful", "mosthelpful", "most-helpful":
		return SortMostHelpful, nil
	}
	return "", fmt.Errorf("unknown review sort %q; use recent or helpful", sort)
}

var numericIDRe = regexp.MustCompile(`^[0-9]{1,12}$`)

// IsAppID reports whether s is a plausible numeric App Store id.
func IsAppID(s string) bool { return numericIDRe.MatchString(s) }

// AppRef is a parsed app reference: a numeric id or a bundle id, plus the
// storefront when it came from a store URL.
type AppRef struct {
	ID       string `json:"id,omitempty"`
	BundleID string `json:"bundleId,omitempty"`
	Country  string `json:"country,omitempty"`
	Raw      string `json:"raw"`
}

var (
	urlIDRe     = regexp.MustCompile(`/id(\d{1,12})(?:[/?#]|$)`)
	bundleIDRe  = regexp.MustCompile(`^[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)+$`)
	urlAppIDRe  = regexp.MustCompile(`/app/(\d{1,12})(?:[/?#]|$)`)
	urlCountryR = regexp.MustCompile(`^/([a-z]{2})/`)
)

// ParseAppRef accepts a numeric id ("324684580"), "id324684580", an
// apps.apple.com or itunes.apple.com URL, or a bundle id
// ("com.spotify.client").
func ParseAppRef(s string) (AppRef, error) {
	raw := strings.TrimSpace(s)
	ref := AppRef{Raw: raw}
	if raw == "" {
		return ref, fmt.Errorf("app reference is empty; pass a numeric id (324684580), id324684580, an App Store URL or a bundle id")
	}
	lower := strings.ToLower(raw)
	if numericIDRe.MatchString(raw) {
		ref.ID = raw
		return ref, nil
	}
	if strings.HasPrefix(lower, "id") && numericIDRe.MatchString(raw[2:]) {
		ref.ID = raw[2:]
		return ref, nil
	}
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "apps.apple.com/") || strings.HasPrefix(lower, "itunes.apple.com/") {
		if !strings.Contains(lower, "://") {
			raw = "https://" + raw
		}
		u, err := url.Parse(raw)
		if err != nil {
			return ref, fmt.Errorf("invalid App Store URL %q: %w", s, err)
		}
		host := strings.ToLower(u.Hostname())
		if host != "apps.apple.com" && host != "itunes.apple.com" {
			return ref, fmt.Errorf("not an App Store URL: %q", s)
		}
		if m := urlIDRe.FindStringSubmatch(u.Path); m != nil {
			ref.ID = m[1]
		} else if m := urlAppIDRe.FindStringSubmatch(u.Path); m != nil {
			ref.ID = m[1]
		} else if id := u.Query().Get("id"); numericIDRe.MatchString(id) {
			ref.ID = id
		}
		if m := urlCountryR.FindStringSubmatch(u.Path); m != nil {
			if cc, err := NormalizeCountry(m[1]); err == nil {
				ref.Country = cc
			}
		}
		if ref.ID == "" {
			return ref, fmt.Errorf("no app id in App Store URL %q", s)
		}
		return ref, nil
	}
	if bundleIDRe.MatchString(raw) && strings.Count(raw, ".") >= 1 && !numericIDRe.MatchString(strings.ReplaceAll(raw, ".", "")) {
		ref.BundleID = raw
		return ref, nil
	}
	return ref, fmt.Errorf("unrecognized app reference %q; pass a numeric id (324684580), id324684580, an App Store URL or a bundle id (com.spotify.client)", s)
}

// lookupLang maps --lang to the Search/Lookup "lang" parameter. Apple
// documents only en_us and ja_jp and rejects other values with HTTP 400, so
// anything else is omitted (pages still get Accept-Language).
func lookupLang(lang string) string {
	l := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(lang), "-", "_"))
	switch l {
	case "en", "en_us":
		return "en_us"
	case "ja", "ja_jp":
		return "ja_jp"
	}
	return ""
}
