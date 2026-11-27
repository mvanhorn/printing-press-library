// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

// Package adlibrary implements a zero-auth pure-HTTP client for Meta's public
// Ad Library GraphQL surface plus local SQLite persistence and competitive
// intelligence analytics.
package adlibrary

import (
	"net/url"
	"sort"
	"strings"
	"time"
)

// Default GraphQL document IDs verified against facebook.com/api/graphql/.
// Operators may override these via META_AD_LIBRARY_DOC_ID_SEARCH,
// META_AD_LIBRARY_DOC_ID_TYPEAHEAD, and META_AD_LIBRARY_DOC_ID_DETAILS if Meta
// rotates Relay hashes.
const (
	DefaultBaseURL         = "https://www.facebook.com"
	DefaultDocIDSearch     = "25464068859919530" // AdLibrarySearchPaginationQuery
	DefaultDocIDTypeahead  = "9755915494515334"  // useAdLibraryTypeaheadSuggestionDataSourceQuery
	DefaultDocIDAdDetails  = "25068828942793558" // AdLibraryV3AdDetailsQuery
	DefaultFallbackRev     = "1053716622"
	DefaultFallbackHS      = "20476.HYP:comet_plat_default_pkg.2.1...0"
	DefaultFallbackV       = "fbece7"
	DefaultUserAgent       = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	AdLibraryPortalBaseURL = "https://www.facebook.com/ads/library/"
)

// CreativeCard represents a single card in a carousel or dynamic creative ad.
type CreativeCard struct {
	Body          string `json:"body,omitempty"`
	Title         string `json:"title,omitempty"`
	Caption       string `json:"caption,omitempty"`
	LinkURL       string `json:"link_url,omitempty"`
	LandingURL    string `json:"landing_url,omitempty"`
	LandingDomain string `json:"landing_domain,omitempty"`
	LinkDesc      string `json:"link_description,omitempty"`
	CTAText       string `json:"cta_text,omitempty"`
	ImageURL      string `json:"image_url,omitempty"`
	VideoURL      string `json:"video_url,omitempty"`
	VideoHDURL    string `json:"video_hd_url,omitempty"`
	VideoPreview  string `json:"video_preview_url,omitempty"`
}

// Ad represents a normalized Meta Ad Library ad record persisted locally and
// emitted by CLI commands.
type Ad struct {
	AdArchiveID       string         `json:"ad_archive_id"`
	CollationID       string         `json:"collation_id,omitempty"`
	CollationCount    int            `json:"collation_count"`
	PageID            string         `json:"page_id"`
	PageName          string         `json:"page_name"`
	PageProfileURI    string         `json:"page_profile_uri,omitempty"`
	PageLikeCount     int64          `json:"page_like_count,omitempty"`
	PageCategories    []string       `json:"page_categories,omitempty"`
	IsActive          bool           `json:"is_active"`
	StartDate         string         `json:"start_date,omitempty"`
	EndDate           string         `json:"end_date,omitempty"`
	DaysRunning       int            `json:"days_running"`
	Country           string         `json:"country,omitempty"`
	AdType            string         `json:"ad_type,omitempty"`
	PublisherPlatform []string       `json:"publisher_platform,omitempty"`
	DisplayFormat     string         `json:"display_format,omitempty"`
	BodyText          string         `json:"body_text,omitempty"`
	LinkTitle         string         `json:"link_title,omitempty"`
	LinkCaption       string         `json:"link_caption,omitempty"`
	LinkDescription   string         `json:"link_description,omitempty"`
	CTAText           string         `json:"cta_text,omitempty"`
	CTAType           string         `json:"cta_type,omitempty"`
	RawLinkURL        string         `json:"raw_link_url,omitempty"`
	LandingURL        string         `json:"landing_url,omitempty"`
	LandingDomain     string         `json:"landing_domain,omitempty"`
	ImageURLs         []string       `json:"image_urls,omitempty"`
	VideoURLs         []string       `json:"video_urls,omitempty"`
	Cards             []CreativeCard `json:"cards,omitempty"`
	Currency          string         `json:"currency,omitempty"`
	SpendLower        *int64         `json:"spend_lower,omitempty"`
	SpendUpper        *int64         `json:"spend_upper,omitempty"`
	ImpressionsLower  *int64         `json:"impressions_lower,omitempty"`
	ImpressionsUpper  *int64         `json:"impressions_upper,omitempty"`
	ReachEstimate     string         `json:"reach_estimate,omitempty"`
	Byline            string         `json:"byline,omitempty"`
	AdLibraryURL      string         `json:"ad_library_url,omitempty"`
	Source            string         `json:"source,omitempty"`
	CollectedAt       string         `json:"collected_at,omitempty"`
}

// Advertiser represents a Facebook Page returned by Ad Library typeahead.
type Advertiser struct {
	PageID         string `json:"page_id"`
	PageName       string `json:"page_name"`
	Category       string `json:"category,omitempty"`
	Likes          int64  `json:"likes,omitempty"`
	Verification   string `json:"verification,omitempty"`
	IGUsername     string `json:"ig_username,omitempty"`
	IGFollowers    int64  `json:"ig_followers,omitempty"`
	IGVerification bool   `json:"ig_verification,omitempty"`
	PageAlias      string `json:"page_alias,omitempty"`
	ImageURI       string `json:"image_uri,omitempty"`
	Country        string `json:"country,omitempty"`
	AdLibraryURL   string `json:"ad_library_url,omitempty"`
	EntityType     string `json:"entity_type,omitempty"`
}

// BeneficiaryPayer captures EU/UK DSA beneficiary and payer disclosures.
type BeneficiaryPayer struct {
	Beneficiary string `json:"beneficiary,omitempty"`
	Payer       string `json:"payer,omitempty"`
	Current     bool   `json:"current,omitempty"`
}

// DemographicEntry represents an age/gender/country reach breakdown slice.
type DemographicEntry struct {
	Country    string  `json:"country,omitempty"`
	AgeRange   string  `json:"age_range,omitempty"`
	Gender     string  `json:"gender,omitempty"`
	Reach      int64   `json:"reach,omitempty"`
	Percentage float64 `json:"percentage,omitempty"`
}

// AdDetails represents the "See ad details" GraphQL transparency payload.
type AdDetails struct {
	AdArchiveID       string             `json:"ad_archive_id"`
	PageID            string             `json:"page_id"`
	PageName          string             `json:"page_name,omitempty"`
	Country           string             `json:"country,omitempty"`
	Byline            string             `json:"byline,omitempty"`
	EUTotalReach      int64              `json:"eu_total_reach,omitempty"`
	UKTotalReach      int64              `json:"uk_total_reach,omitempty"`
	BRTotalReach      int64              `json:"br_total_reach,omitempty"`
	TargetAges        []string           `json:"target_ages,omitempty"`
	TargetGender      string             `json:"target_gender,omitempty"`
	TargetLocations   []string           `json:"target_locations,omitempty"`
	BeneficiaryPayers []BeneficiaryPayer `json:"beneficiary_payers,omitempty"`
	Demographics      []DemographicEntry `json:"demographics,omitempty"`
	ViolationTypes    []string           `json:"violation_types,omitempty"`
	IsViolatingEUSIEP bool               `json:"is_violating_eu_siep,omitempty"`
	AIDisclosureElig  bool               `json:"ai_disclosure_eligible,omitempty"`
	SpendLower        *int64             `json:"spend_lower,omitempty"`
	SpendUpper        *int64             `json:"spend_upper,omitempty"`
	ImpressionsLower  *int64             `json:"impressions_lower,omitempty"`
	ImpressionsUpper  *int64             `json:"impressions_upper,omitempty"`
	Currency          string             `json:"currency,omitempty"`
	RawDetails        map[string]any     `json:"raw_details,omitempty"`
	FetchedAt         string             `json:"fetched_at,omitempty"`
}

// SearchParams describes a public Meta Ad Library search query.
type SearchParams struct {
	Query        string   `json:"query,omitempty"`
	Country      string   `json:"country,omitempty"`
	AdType       string   `json:"ad_type,omitempty"`
	ActiveStatus string   `json:"active_status,omitempty"`
	MediaType    string   `json:"media_type,omitempty"`
	SearchType   string   `json:"search_type,omitempty"`
	PageIDs      []string `json:"page_ids,omitempty"`
	Platforms    []string `json:"platforms,omitempty"`
	Languages    []string `json:"languages,omitempty"`
	SortBy       string   `json:"sort_by,omitempty"`
	Limit        int      `json:"limit,omitempty"`
	MaxPages     int      `json:"max_pages,omitempty"`
	Cursor       string   `json:"cursor,omitempty"`
	MinCollation int      `json:"min_collation,omitempty"`
}

// SearchResult wraps paginated ads returned from GraphQL.
type SearchResult struct {
	Ads         []Ad   `json:"ads"`
	NextCursor  string `json:"next_cursor,omitempty"`
	HasNextPage bool   `json:"has_next_page"`
	IsComplete  bool   `json:"is_complete"`
	TotalCount  int    `json:"total_count,omitempty"`
	PagesRead   int    `json:"pages_read"`
}

// MediaFile describes a single downloaded (or planned) creative asset.
type MediaFile struct {
	AdArchiveID string `json:"ad_archive_id"`
	Kind        string `json:"kind"` // image | video
	URL         string `json:"url"`
	LocalPath   string `json:"local_path,omitempty"`
	Bytes       int64  `json:"bytes,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Status      string `json:"status"` // downloaded | skipped | dry-run | failed
	Error       string `json:"error,omitempty"`
}

// MediaDownloadReport summarizes a media download run.
type MediaDownloadReport struct {
	OutputDir  string      `json:"output_dir"`
	Total      int         `json:"total"`
	Downloaded int         `json:"downloaded"`
	Skipped    int         `json:"skipped"`
	Failed     int         `json:"failed"`
	Files      []MediaFile `json:"files"`
}

// UnwrapRedirectURL decodes Meta's l.facebook.com / lm.facebook.com redirect
// wrappers and strips tracking-only fbclid parameters when returning the clean
// landing URL.
func UnwrapRedirectURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return trimmed
	}
	host := strings.ToLower(u.Hostname())
	isMetaRedirect := (u.Path == "/l.php" || strings.HasPrefix(host, "l.") || strings.HasPrefix(host, "lm.")) &&
		(strings.HasSuffix(host, ".facebook.com") || strings.HasSuffix(host, ".instagram.com") || strings.HasSuffix(host, ".threads.net"))
	if isMetaRedirect {
		if target := strings.TrimSpace(u.Query().Get("u")); target != "" {
			trimmed = target
			u, err = url.Parse(trimmed)
			if err != nil {
				return trimmed
			}
		}
	}
	// Remove noisy fbclid while keeping intentional funnel UTMs intact.
	q := u.Query()
	if q.Has("fbclid") {
		q.Del("fbclid")
		u.RawQuery = q.Encode()
		return u.String()
	}
	return trimmed
}

// ExtractDomain returns the normalized lowercase hostname (without leading
// "www.") for a URL, or empty string if unparseable.
func ExtractDomain(raw string) string {
	clean := UnwrapRedirectURL(raw)
	if clean == "" {
		return ""
	}
	if !strings.Contains(clean, "://") {
		clean = "https://" + clean
	}
	u, err := url.Parse(clean)
	if err != nil {
		return ""
	}
	host := strings.ToLower(strings.TrimSpace(u.Hostname()))
	host = strings.TrimPrefix(host, "www.")
	return host
}

// NormalizeURLPath returns a normalized URL path (preserving case, without a
// trailing slash unless root "/") for funnel grouping.
func NormalizeURLPath(raw string) string {
	clean := UnwrapRedirectURL(raw)
	if clean == "" {
		return "/"
	}
	if !strings.Contains(clean, "://") {
		clean = "https://" + clean
	}
	u, err := url.Parse(clean)
	if err != nil {
		return "/"
	}
	p := strings.TrimSpace(u.Path)
	if p == "" || p == "/" {
		return "/"
	}
	p = strings.TrimRight(p, "/")
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

// ExtractUTMParams pulls standard UTM parameters from an unwrapped URL.
func ExtractUTMParams(raw string) map[string]string {
	clean := UnwrapRedirectURL(raw)
	out := make(map[string]string)
	if clean == "" {
		return out
	}
	if !strings.Contains(clean, "://") {
		clean = "https://" + clean
	}
	u, err := url.Parse(clean)
	if err != nil {
		return out
	}
	q := u.Query()
	for _, k := range []string{"utm_source", "utm_medium", "utm_campaign", "utm_content", "utm_term"} {
		if v := strings.TrimSpace(q.Get(k)); v != "" {
			out[k] = v
		}
	}
	return out
}

// ParseAdLibraryURL parses any https://www.facebook.com/ads/library/?... URL
// into structured SearchParams and an optional direct ad archive ID.
func ParseAdLibraryURL(rawURL string) (SearchParams, string, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return SearchParams{}, "", err
	}
	q := u.Query()
	params := SearchParams{
		Country:      "US",
		AdType:       "ALL",
		ActiveStatus: "ACTIVE",
		MediaType:    "ALL",
		SearchType:   "KEYWORD_UNORDERED",
	}
	if c := strings.ToUpper(strings.TrimSpace(q.Get("country"))); c != "" {
		params.Country = c
	}
	if kw := strings.TrimSpace(q.Get("q")); kw != "" {
		params.Query = kw
	}
	if pid := strings.TrimSpace(q.Get("view_all_page_id")); pid != "" && pid != "0" {
		params.PageIDs = []string{pid}
		params.SearchType = "PAGE"
	}
	switch strings.ToLower(strings.TrimSpace(q.Get("active_status"))) {
	case "active":
		params.ActiveStatus = "ACTIVE"
	case "inactive":
		params.ActiveStatus = "INACTIVE"
	case "all":
		params.ActiveStatus = "ALL"
	}
	switch strings.ToLower(strings.TrimSpace(q.Get("ad_type"))) {
	case "political_and_issue_ads":
		params.AdType = "POLITICAL_AND_ISSUE_ADS"
	case "housing":
		params.AdType = "HOUSING_ADS"
	case "employment":
		params.AdType = "EMPLOYMENT_ADS"
	case "credit", "financial_products_and_services_ads":
		params.AdType = "CREDIT_ADS"
	case "all":
		params.AdType = "ALL"
	}
	switch strings.ToLower(strings.TrimSpace(q.Get("media_type"))) {
	case "image":
		params.MediaType = "IMAGE"
	case "video":
		params.MediaType = "VIDEO"
	case "meme":
		params.MediaType = "MEME"
	case "none":
		params.MediaType = "NONE"
	case "all":
		params.MediaType = "ALL"
	}
	switch strings.ToLower(strings.TrimSpace(q.Get("search_type"))) {
	case "keyword_exact_phrase":
		params.SearchType = "KEYWORD_EXACT_PHRASE"
	case "keyword_unordered":
		params.SearchType = "KEYWORD_UNORDERED"
	case "page":
		params.SearchType = "PAGE"
	}
	if sortMode := strings.ToLower(strings.TrimSpace(q.Get("sort_data[mode]"))); sortMode == "total_impressions" {
		params.SortBy = "impressions"
	}
	platKeys := make([]string, 0, len(q))
	for k := range q {
		if k == "publisher_platforms" || strings.HasPrefix(k, "publisher_platforms[") {
			platKeys = append(platKeys, k)
		}
	}
	sort.Strings(platKeys)
	for _, k := range platKeys {
		for _, plat := range q[k] {
			if p := strings.ToUpper(strings.TrimSpace(plat)); p != "" {
				params.Platforms = append(params.Platforms, p)
			}
		}
	}
	params.Platforms = UniqueStrings(params.Platforms)
	adID := strings.TrimSpace(q.Get("id"))
	return params, adID, nil
}

// BuildAdLibraryURL constructs a clickable Meta Ad Library browser deep link
// for a single ad ID, a Facebook Page ID, or a keyword query.
func BuildAdLibraryURL(adID, pageID, query, country, status, adType string) string {
	if strings.TrimSpace(adID) != "" {
		return AdLibraryPortalBaseURL + "?id=" + url.QueryEscape(strings.TrimSpace(adID))
	}
	v := url.Values{}
	if strings.TrimSpace(status) == "" {
		status = "active"
	}
	v.Set("active_status", strings.ToLower(strings.TrimSpace(status)))
	if strings.TrimSpace(adType) == "" {
		adType = "all"
	}
	v.Set("ad_type", mapAdTypeToURL(adType))
	if strings.TrimSpace(country) == "" {
		country = "US"
	}
	v.Set("country", strings.ToUpper(strings.TrimSpace(country)))
	if strings.TrimSpace(pageID) != "" {
		v.Set("view_all_page_id", strings.TrimSpace(pageID))
		v.Set("search_type", "page")
	} else {
		v.Set("search_type", "keyword_unordered")
		if strings.TrimSpace(query) != "" {
			v.Set("q", strings.TrimSpace(query))
		}
	}
	return AdLibraryPortalBaseURL + "?" + v.Encode()
}

func mapAdTypeToURL(adType string) string {
	switch strings.ToUpper(strings.TrimSpace(adType)) {
	case "POLITICAL_AND_ISSUE_ADS", "POLITICAL":
		return "political_and_issue_ads"
	case "HOUSING_ADS", "HOUSING":
		return "housing"
	case "EMPLOYMENT_ADS", "EMPLOYMENT":
		return "employment"
	case "CREDIT_ADS", "CREDIT", "FINANCIAL_PRODUCTS_AND_SERVICES_ADS":
		return "credit"
	default:
		return "all"
	}
}

// ExtractOpeningHook mechanically extracts the opening hook from ad body copy
// (first non-empty line or first sentence, capped at 140 characters) along with
// a normalized deduplication key. Zero LLM calls.
func ExtractOpeningHook(body string) (display string, normKey string) {
	s := strings.TrimSpace(body)
	if s == "" {
		return "", ""
	}
	// Take the first non-empty line.
	lines := strings.Split(s, "\n")
	firstLine := ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			firstLine = line
			break
		}
	}
	if firstLine == "" {
		return "", ""
	}
	// Split on first sentence boundary (. ! ?) when followed by whitespace.
	candidate := firstLine
	for i := 0; i < len(candidate)-1; i++ {
		ch := candidate[i]
		if (ch == '.' || ch == '!' || ch == '?') && (candidate[i+1] == ' ' || candidate[i+1] == '\t') {
			if i >= 12 {
				candidate = strings.TrimSpace(candidate[:i+1])
				break
			}
		}
	}
	runes := []rune(candidate)
	if len(runes) > 140 {
		candidate = strings.TrimSpace(string(runes[:137])) + "..."
	}
	display = candidate
	// Build whitespace-collapsed, lowercased dedup key.
	normKey = strings.ToLower(strings.Join(strings.Fields(display), " "))
	return display, normKey
}

// ComputeDaysRunning calculates how many days an ad has been active (or ran
// before stopping) based on its UTC RFC3339 or YYYY-MM-DD start/end timestamps.
func ComputeDaysRunning(startDate, endDate string, isActive bool, now time.Time) int {
	start, ok := parseFlexibleTime(startDate)
	if !ok {
		return 0
	}
	end := now
	if !isActive {
		if parsedEnd, ok := parseFlexibleTime(endDate); ok && !parsedEnd.Before(start) {
			end = parsedEnd
		}
	}
	if end.Before(start) {
		return 0
	}
	days := int(end.Sub(start).Hours() / 24)
	if days < 1 && end.Sub(start) > 0 {
		return 1
	}
	return days
}

func parseFlexibleTime(raw string) (time.Time, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05-0700",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

// ExtractMediaURLs returns deduplicated image and video URLs from an Ad
// (including its carousel cards).
func ExtractMediaURLs(ad Ad) (images []string, videos []string) {
	imgSeen := make(map[string]bool)
	vidSeen := make(map[string]bool)
	addImg := func(u string) {
		u = strings.TrimSpace(u)
		if u != "" && !imgSeen[u] {
			imgSeen[u] = true
			images = append(images, u)
		}
	}
	addVid := func(u string) {
		u = strings.TrimSpace(u)
		if u != "" && !vidSeen[u] {
			vidSeen[u] = true
			videos = append(videos, u)
		}
	}
	for _, u := range ad.ImageURLs {
		addImg(u)
	}
	for _, u := range ad.VideoURLs {
		addVid(u)
	}
	for _, c := range ad.Cards {
		addImg(c.ImageURL)
		if c.VideoHDURL != "" {
			addVid(c.VideoHDURL)
		} else {
			addVid(c.VideoURL)
		}
	}
	return images, videos
}

// UniqueStrings returns a sorted deduplicated slice of non-empty strings.
func UniqueStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
