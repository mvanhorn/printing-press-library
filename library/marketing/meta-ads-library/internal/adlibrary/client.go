// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

package adlibrary

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/meta-ads-library/internal/cliutil"
)

var (
	reLSDPatterns = []*regexp.Regexp{
		regexp.MustCompile(`"LSD",\[\],\{"token":"([^"]+)"\}`),
		regexp.MustCompile(`\["LSD",\[\],\{"token":"([^"]+)"`),
		regexp.MustCompile(`"lsd":"([^"]+)"`),
		regexp.MustCompile(`name="lsd" value="([^"]+)"`),
	}
	reRevPatterns = []*regexp.Regexp{
		regexp.MustCompile(`"__spin_r":(\d+)`),
		regexp.MustCompile(`"server_revision":(\d+)`),
		regexp.MustCompile(`"revision":(\d+)`),
	}
	reSpinT     = regexp.MustCompile(`"__spin_t":(\d+)`)
	reSpinB     = regexp.MustCompile(`"__spin_b":"([^"]+)"`)
	reHSI       = regexp.MustCompile(`"__hsi":"(\d+)"`)
	reHSIAlt    = regexp.MustCompile(`"hsi":"(\d+)"`)
	reHS        = regexp.MustCompile(`"__hs":"([^"]+)"`)
	reCometReq  = regexp.MustCompile(`"__comet_req":(\d+)`)
	reVToken    = regexp.MustCompile(`"v"\s*:\s*"([a-f0-9]{4,10})"`)
	reASBD      = regexp.MustCompile(`"asbd_id"\s*:\s*"?(\d+)"?`)
	reRangeText = regexp.MustCompile(`(\d[\d,\.]*)\s*([KMBkmb]?)`)
)

const maxMediaBytes int64 = 256 << 20

// Client talks directly to Meta's public Ad Library GraphQL endpoint over plain
// HTTP with automatic facebook.com homepage token bootstrap, adaptive rate
// limiting, and session refresh on 403 / session expiry.
type Client struct {
	BaseURL        string
	HTTP           *http.Client
	StreamHTTP     *http.Client
	limiter        *cliutil.AdaptiveLimiter
	docIDSearch    string
	docIDTypeahead string
	docIDDetails   string

	mu          sync.Mutex
	initialized bool
	initAt      time.Time
	tokens      map[string]string
	reqCounter  int
	maxSession  time.Duration
	maxRetries  int
}

// ClientOption configures a Client instance.
type ClientOption func(*Client)

// WithBaseURL overrides the upstream host (used in tests and custom mirrors).
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) {
		if strings.TrimSpace(baseURL) != "" {
			c.BaseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
		}
	}
}

// WithHTTPClient injects a custom HTTP client (primarily for testing).
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) {
		if hc != nil {
			c.HTTP = hc
			c.StreamHTTP = hc
		}
	}
}

// WithRateLimit sets a polite max requests-per-second ceiling.
func WithRateLimit(rps float64) ClientOption {
	return func(c *Client) {
		if rps > 0 {
			c.limiter = cliutil.NewAdaptiveLimiter(rps)
		}
	}
}

// NewClient constructs a ready-to-use public Meta Ad Library HTTP client.
func NewClient(opts ...ClientOption) *Client {
	jar, _ := cookiejar.New(nil)
	transport := &http.Transport{
		ResponseHeaderTimeout: 25 * time.Second,
		MaxIdleConns:          16,
		IdleConnTimeout:       60 * time.Second,
	}
	c := &Client{
		BaseURL: DefaultBaseURL,
		HTTP: &http.Client{
			Timeout:   30 * time.Second,
			Jar:       jar,
			Transport: transport,
		},
		StreamHTTP: &http.Client{
			Jar:       jar,
			Transport: transport,
		},
		limiter:        cliutil.NewAdaptiveLimiter(1.5),
		docIDSearch:    envOrDefault("META_AD_LIBRARY_DOC_ID_SEARCH", DefaultDocIDSearch),
		docIDTypeahead: envOrDefault("META_AD_LIBRARY_DOC_ID_TYPEAHEAD", DefaultDocIDTypeahead),
		docIDDetails:   envOrDefault("META_AD_LIBRARY_DOC_ID_DETAILS", DefaultDocIDAdDetails),
		tokens:         make(map[string]string),
		maxSession:     25 * time.Minute,
		maxRetries:     2,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func envOrDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

// Bootstrap fetches the facebook.com homepage over plain HTTP to acquire fresh
// cookies (datr, fr, sb) and Relay tokens (lsd, __spin_r, __hsi, __hs).
func (c *Client) Bootstrap(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.bootstrapLocked(ctx)
}

func (c *Client) bootstrapLocked(ctx context.Context) error {
	if err := c.limiter.Wait(ctx); err != nil {
		return err
	}
	homeURL := c.BaseURL + "/"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, homeURL, nil)
	if err != nil {
		return fmt.Errorf("creating bootstrap request: %w", err)
	}
	req.Header.Set("User-Agent", DefaultUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("fetching Meta bootstrap page %s: %w", homeURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		c.limiter.OnRateLimit()
		return &cliutil.RateLimitError{
			URL:        homeURL,
			RetryAfter: cliutil.RetryAfter(resp),
			Body:       "Meta bootstrap page rate-limited",
		}
	}
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("reading bootstrap page: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return fmt.Errorf("bootstrap page %s returned HTTP %d", homeURL, resp.StatusCode)
	}
	c.limiter.OnSuccess()
	tokens := ExtractTokensFromHTML(string(bodyBytes))
	if tokens["lsd"] == "" {
		return fmt.Errorf("bootstrap page %s did not expose an lsd token (HTTP %d, %d bytes)", homeURL, resp.StatusCode, len(bodyBytes))
	}
	c.tokens = tokens
	c.initialized = true
	c.initAt = time.Now()
	c.reqCounter = 0
	return nil
}

// ExtractTokensFromHTML parses anonymous Relay session tokens from facebook.com HTML.
func ExtractTokensFromHTML(html string) map[string]string {
	tokens := make(map[string]string)
	for _, re := range reLSDPatterns {
		if m := re.FindStringSubmatch(html); len(m) > 1 {
			tokens["lsd"] = m[1]
			break
		}
	}
	for _, re := range reRevPatterns {
		if m := re.FindStringSubmatch(html); len(m) > 1 {
			tokens["__rev"] = m[1]
			tokens["__spin_r"] = m[1]
			break
		}
	}
	if m := reSpinT.FindStringSubmatch(html); len(m) > 1 {
		tokens["__spin_t"] = m[1]
	}
	if m := reSpinB.FindStringSubmatch(html); len(m) > 1 {
		tokens["__spin_b"] = m[1]
	}
	if m := reHSI.FindStringSubmatch(html); len(m) > 1 {
		tokens["__hsi"] = m[1]
	} else if m := reHSIAlt.FindStringSubmatch(html); len(m) > 1 {
		tokens["__hsi"] = m[1]
	}
	if m := reHS.FindStringSubmatch(html); len(m) > 1 {
		tokens["__hs"] = m[1]
	}
	if m := reCometReq.FindStringSubmatch(html); len(m) > 1 {
		tokens["__comet_req"] = m[1]
	}
	if m := reVToken.FindStringSubmatch(html); len(m) > 1 {
		tokens["v"] = m[1]
	}
	if m := reASBD.FindStringSubmatch(html); len(m) > 1 {
		tokens["x-asbd-id"] = m[1]
	}
	return tokens
}

// CalculateJazoest computes Meta's jazoest checksum from an lsd token.
func CalculateJazoest(lsd string) string {
	if lsd == "" {
		return "2893"
	}
	sum := 0
	for i := 0; i < len(lsd); i++ {
		sum += int(lsd[i])
	}
	return strconv.Itoa(2 + sum)
}

func (c *Client) ensureSession(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.initialized && time.Since(c.initAt) < c.maxSession && c.tokens["lsd"] != "" {
		return nil
	}
	return c.bootstrapLocked(ctx)
}

func (c *Client) refreshSession(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if jar, err := cookiejar.New(nil); err == nil {
		c.HTTP.Jar = jar
	}
	c.initialized = false
	c.tokens = make(map[string]string)
	return c.bootstrapLocked(ctx)
}

func (c *Client) graphqlPost(ctx context.Context, docID, friendlyName string, variables map[string]any, referer string) (map[string]any, error) {
	if err := c.ensureSession(ctx); err != nil {
		return nil, err
	}

	gqlURL := c.BaseURL + "/api/graphql/"
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, err
		}

		form, lsd, asbd := c.buildPayload(docID, friendlyName, variables)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, gqlURL, strings.NewReader(form.Encode()))
		if err != nil {
			return nil, fmt.Errorf("building GraphQL request: %w", err)
		}
		req.Header.Set("User-Agent", DefaultUserAgent)
		req.Header.Set("Accept", "*/*")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Origin", c.BaseURL)
		req.Header.Set("Referer", referer)
		req.Header.Set("Sec-Fetch-Dest", "empty")
		req.Header.Set("Sec-Fetch-Mode", "cors")
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		req.Header.Set("x-fb-friendly-name", friendlyName)
		req.Header.Set("x-fb-lsd", lsd)
		if asbd != "" {
			req.Header.Set("x-asbd-id", asbd)
		}

		resp, err := c.HTTP.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("GraphQL %s request failed: %w", friendlyName, err)
			if attempt < c.maxRetries {
				if sleepErr := sleepCtx(ctx, cliutil.Backoff(attempt)); sleepErr != nil {
					return nil, sleepErr
				}
				continue
			}
			return nil, lastErr
		}

		rawBytes, readErr := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("reading GraphQL %s response: %w", friendlyName, readErr)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			c.limiter.OnRateLimit()
			wait := cliutil.RetryAfter(resp)
			if attempt < c.maxRetries {
				if sleepErr := sleepCtx(ctx, wait); sleepErr != nil {
					return nil, sleepErr
				}
				continue
			}
			return nil, &cliutil.RateLimitError{
				URL:        gqlURL,
				RetryAfter: wait,
				Body:       truncateBody(string(rawBytes), 200),
			}
		}

		if resp.StatusCode == http.StatusForbidden {
			if attempt < c.maxRetries {
				if refErr := c.refreshSession(ctx); refErr != nil {
					return nil, refErr
				}
				continue
			}
			return nil, fmt.Errorf("Meta GraphQL %s returned HTTP 403 after session refresh", friendlyName)
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("Meta GraphQL %s returned HTTP %d: %s", friendlyName, resp.StatusCode, truncateBody(string(rawBytes), 240))
		}

		cleanText := strings.TrimPrefix(strings.TrimSpace(string(rawBytes)), "for (;;);")
		var parsed map[string]any
		if err := json.Unmarshal([]byte(cleanText), &parsed); err != nil {
			return nil, fmt.Errorf("decoding GraphQL %s JSON: %w", friendlyName, err)
		}

		if code, msg, isRateLimit, isSessionErr := inspectGraphQLErrors(parsed); code != 0 || msg != "" || isRateLimit || isSessionErr {
			if isRateLimit {
				c.limiter.OnRateLimit()
				if attempt < c.maxRetries {
					if sleepErr := sleepCtx(ctx, cliutil.Backoff(attempt+1)); sleepErr != nil {
						return nil, sleepErr
					}
					continue
				}
				return nil, &cliutil.RateLimitError{
					URL:        gqlURL,
					RetryAfter: 5 * time.Second,
					Body:       fmt.Sprintf("Meta GraphQL error %d: %s", code, msg),
				}
			}
			if isSessionErr && attempt < c.maxRetries {
				if refErr := c.refreshSession(ctx); refErr != nil {
					return nil, refErr
				}
				continue
			}
			// Partial data with field-level errors is common in AdLibraryV3AdDetailsQuery;
			// return payload only when data is a non-empty object (not null/empty).
			if len(asMap(parsed["data"])) == 0 {
				if code != 0 {
					return nil, fmt.Errorf("Meta GraphQL %s error %d: %s", friendlyName, code, msg)
				}
				return nil, fmt.Errorf("Meta GraphQL %s error: %s", friendlyName, msg)
			}
		}

		c.limiter.OnSuccess()
		return parsed, nil
	}

	return nil, lastErr
}

func (c *Client) buildPayload(docID, friendlyName string, variables map[string]any) (url.Values, string, string) {
	c.mu.Lock()
	c.reqCounter++
	reqNum := strconv.FormatInt(int64(c.reqCounter), 36)
	lsd := c.tokens["lsd"]
	rev := tokenOrDefault(c.tokens, "__rev", DefaultFallbackRev)
	spinR := tokenOrDefault(c.tokens, "__spin_r", rev)
	spinB := tokenOrDefault(c.tokens, "__spin_b", "trunk")
	spinT := tokenOrDefault(c.tokens, "__spin_t", strconv.FormatInt(time.Now().Unix(), 10))
	hsi := tokenOrDefault(c.tokens, "__hsi", strconv.FormatInt(time.Now().UnixMilli(), 10))
	hs := tokenOrDefault(c.tokens, "__hs", DefaultFallbackHS)
	cometReq := tokenOrDefault(c.tokens, "__comet_req", "94")
	asbd := c.tokens["x-asbd-id"]
	c.mu.Unlock()

	varsJSON, _ := json.Marshal(variables)
	form := url.Values{}
	form.Set("av", "0")
	form.Set("__aaid", "0")
	form.Set("__user", "0")
	form.Set("__a", "1")
	form.Set("__req", reqNum)
	form.Set("__hs", hs)
	form.Set("dpr", "1")
	form.Set("__ccg", "GOOD")
	form.Set("__rev", rev)
	form.Set("__s", randomShortSessionID())
	form.Set("__hsi", hsi)
	form.Set("__comet_req", cometReq)
	form.Set("lsd", lsd)
	form.Set("jazoest", CalculateJazoest(lsd))
	form.Set("__spin_r", spinR)
	form.Set("__spin_b", spinB)
	form.Set("__spin_t", spinT)
	form.Set("__jssesw", "1")
	form.Set("fb_api_caller_class", "RelayModern")
	form.Set("fb_api_req_friendly_name", friendlyName)
	form.Set("server_timestamps", "true")
	form.Set("variables", string(varsJSON))
	form.Set("doc_id", docID)
	return form, lsd, asbd
}

func tokenOrDefault(m map[string]string, k, fallback string) string {
	if v := strings.TrimSpace(m[k]); v != "" {
		return v
	}
	return fallback
}

func inspectGraphQLErrors(parsed map[string]any) (code int, msg string, isRateLimit bool, isSessionErr bool) {
	errs, ok := parsed["errors"].([]any)
	if !ok || len(errs) == 0 {
		return 0, "", false, false
	}
	for _, raw := range errs {
		em, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		c := int(asInt64(em["code"]))
		m := asString(em["message"])
		lower := strings.ToLower(m)
		if c == 1675004 || strings.Contains(lower, "rate limit") {
			return c, m, true, false
		}
		if c == 1357004 || c == 1357001 || strings.Contains(lower, "session") {
			return c, m, false, true
		}
		if code == 0 && msg == "" {
			code = c
			msg = m
		}
	}
	return code, msg, false, false
}

// SearchAdvertisers runs Meta's public Ad Library typeahead query to resolve a
// brand name into Facebook Page IDs and page metadata.
func (c *Client) SearchAdvertisers(ctx context.Context, query, country string) ([]Advertiser, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("advertiser query is required")
	}
	country = normalizeCountry(country)
	variables := map[string]any{
		"queryString": strings.TrimSpace(query),
		"country":     country,
		"adType":      "ALL",
		"isMobile":    false,
	}
	referer := BuildAdLibraryURL("", "", query, country, "all", "all")
	raw, err := c.graphqlPost(ctx, c.docIDTypeahead, "useAdLibraryTypeaheadSuggestionDataSourceQuery", variables, referer)
	if err != nil {
		return nil, err
	}
	return ParseTypeaheadResponse(raw, country), nil
}

// SearchAds queries Meta's public Ad Library GraphQL pagination endpoint,
// automatically walking pages up to params.MaxPages and params.Limit.
func (c *Client) SearchAds(ctx context.Context, params SearchParams) (SearchResult, error) {
	country := normalizeCountry(params.Country)
	adType := normalizeAdType(params.AdType)
	status := normalizeStatus(params.ActiveStatus)
	mediaType := normalizeMediaType(params.MediaType)
	searchType := normalizeSearchType(params.SearchType, len(params.PageIDs) > 0)

	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}
	maxPages := params.MaxPages
	if maxPages <= 0 {
		maxPages = 3
	}
	pageSize := limit
	if pageSize > 30 {
		pageSize = 30
	}
	if pageSize < 5 {
		pageSize = 5
	}

	sessionID := randomUUID()
	collationToken := randomToken(24)
	cursor := strings.TrimSpace(params.Cursor)

	var allAds []Ad
	seenIDs := make(map[string]bool)
	var nextCursor string
	var hasNext bool
	var totalCount int
	pagesRead := 0
	truncatedByLimit := false
	hadGraphQLErrors := false
	lastPageExplicitEnd := false

	if err := c.ensureSession(ctx); err != nil {
		return SearchResult{}, err
	}

	for page := 0; page < maxPages && len(allAds) < limit; page++ {
		c.mu.Lock()
		vTok := tokenOrDefault(c.tokens, "v", DefaultFallbackV)
		c.mu.Unlock()
		pageIDs := params.PageIDs
		if pageIDs == nil {
			pageIDs = []string{}
		}
		langs := params.Languages
		if langs == nil {
			langs = []string{}
		}
		platforms := make([]string, 0, len(params.Platforms))
		for _, p := range params.Platforms {
			if up := strings.ToUpper(strings.TrimSpace(p)); up != "" {
				platforms = append(platforms, up)
			}
		}

		viewAllPageID := "0"
		if len(pageIDs) == 1 && strings.TrimSpace(params.Query) == "" {
			viewAllPageID = pageIDs[0]
		}

		variables := map[string]any{
			"activeStatus":           status,
			"adType":                 adType,
			"bylines":                []string{},
			"collationToken":         collationToken,
			"contentLanguages":       langs,
			"countries":              []string{country},
			"excludedIDs":            []string{},
			"first":                  pageSize,
			"isTargetedCountry":      false,
			"location":               nil,
			"mediaType":              mediaType,
			"multiCountryFilterMode": nil,
			"pageIDs":                pageIDs,
			"potentialReachInput":    []any{},
			"publisherPlatforms":     platforms,
			"queryString":            strings.TrimSpace(params.Query),
			"regions":                []any{},
			"searchType":             searchType,
			"sessionID":              sessionID,
			"source":                 nil,
			"startDate":              nil,
			"v":                      vTok,
			"viewAllPageID":          viewAllPageID,
		}
		if strings.EqualFold(params.SortBy, "impressions") {
			variables["sortData"] = map[string]any{
				"direction": "DESCENDING",
				"mode":      "SORT_BY_TOTAL_IMPRESSIONS",
			}
		}
		if cursor != "" {
			variables["cursor"] = cursor
		}

		referer := BuildAdLibraryURL("", firstOrEmpty(pageIDs), params.Query, country, status, adType)
		raw, err := c.graphqlPost(ctx, c.docIDSearch, "AdLibrarySearchPaginationQuery", variables, referer)
		if err != nil {
			return SearchResult{}, err
		}

		gqlCode, gqlMsg, _, _ := inspectGraphQLErrors(raw)
		if gqlCode != 0 || gqlMsg != "" {
			hadGraphQLErrors = true
		}

		pageAds, pageNext, pageHasNext, explicitEnd, pageTotal, validConn := parseSearchResponseValidated(raw, country, adType, time.Now().UTC())
		if !validConn {
			if gqlCode != 0 {
				return SearchResult{}, fmt.Errorf("Meta GraphQL AdLibrarySearchPaginationQuery missing search_results_connection (error %d: %s)", gqlCode, gqlMsg)
			}
			if gqlMsg != "" {
				return SearchResult{}, fmt.Errorf("Meta GraphQL AdLibrarySearchPaginationQuery missing search_results_connection (%s)", gqlMsg)
			}
			return SearchResult{}, fmt.Errorf("Meta GraphQL AdLibrarySearchPaginationQuery returned missing or malformed search_results_connection")
		}
		if (gqlCode != 0 || gqlMsg != "") && len(pageAds) == 0 {
			if gqlCode != 0 {
				return SearchResult{}, fmt.Errorf("Meta GraphQL AdLibrarySearchPaginationQuery error %d: %s", gqlCode, gqlMsg)
			}
			return SearchResult{}, fmt.Errorf("Meta GraphQL AdLibrarySearchPaginationQuery error: %s", gqlMsg)
		}

		pagesRead++
		if pageTotal > 0 && totalCount == 0 {
			totalCount = pageTotal
		}
		nextCursor = pageNext
		hasNext = pageHasNext
		lastPageExplicitEnd = explicitEnd

		for idx, ad := range pageAds {
			if params.MinCollation > 0 && ad.CollationCount < params.MinCollation {
				continue
			}
			if seenIDs[ad.AdArchiveID] {
				continue
			}
			seenIDs[ad.AdArchiveID] = true
			allAds = append(allAds, ad)
			if len(allAds) >= limit {
				for _, rem := range pageAds[idx+1:] {
					if params.MinCollation > 0 && rem.CollationCount < params.MinCollation {
						continue
					}
					if !seenIDs[rem.AdArchiveID] {
						truncatedByLimit = true
						break
					}
				}
				break
			}
		}

		if !hasNext || nextCursor == "" || len(pageAds) == 0 {
			break
		}
		cursor = nextCursor
	}

	if allAds == nil {
		allAds = make([]Ad, 0)
	}
	isComplete := pagesRead > 0 && lastPageExplicitEnd && !hasNext && !truncatedByLimit && !hadGraphQLErrors
	return SearchResult{
		Ads:         allAds,
		NextCursor:  nextCursor,
		HasNextPage: hasNext,
		IsComplete:  isComplete,
		TotalCount:  totalCount,
		PagesRead:   pagesRead,
	}, nil
}

// GetAdDetails fetches the "See ad details" GraphQL panel for an ad archive ID
// and page ID (EU/UK/BR transparency, beneficiary/payer, demographic reach,
// advertiser info, and violation flags).
func (c *Client) GetAdDetails(ctx context.Context, adArchiveID, pageID, country string) (AdDetails, error) {
	adArchiveID = strings.TrimSpace(adArchiveID)
	pageID = strings.TrimSpace(pageID)
	if adArchiveID == "" || pageID == "" {
		return AdDetails{}, fmt.Errorf("both ad_archive_id and page_id are required for ad details")
	}
	country = normalizeCountry(country)
	variables := map[string]any{
		"adArchiveID":        adArchiveID,
		"pageID":             pageID,
		"country":            country,
		"sessionID":          randomUUID(),
		"source":             nil,
		"isAdNonPolitical":   false,
		"isAdNotAAAEligible": false,
	}
	referer := BuildAdLibraryURL(adArchiveID, "", "", country, "all", "all")
	raw, err := c.graphqlPost(ctx, c.docIDDetails, "AdLibraryV3AdDetailsQuery", variables, referer)
	if err != nil {
		return AdDetails{}, err
	}
	return ParseAdDetailsResponse(raw, adArchiveID, pageID, country, time.Now().UTC())
}

// DownloadMedia downloads creative images and videos for a set of ads using the
// streaming HTTP client (unbounded total transfer time, header-timeout bounded).
func (c *Client) DownloadMedia(ctx context.Context, ads []Ad, outDir string, imagesOnly, videosOnly bool) (MediaDownloadReport, error) {
	report := MediaDownloadReport{
		OutputDir: outDir,
		Files:     make([]MediaFile, 0),
	}
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return report, fmt.Errorf("creating output directory %s: %w", outDir, err)
	}
	dirRoot, err := os.OpenRoot(outDir)
	if err != nil {
		return report, fmt.Errorf("opening output directory %s: %w", outDir, err)
	}
	defer func() { _ = dirRoot.Close() }()

	for _, ad := range ads {
		imgs, vids := ExtractMediaURLs(ad)
		type target struct {
			kind string
			url  string
			idx  int
		}
		var targets []target
		if !videosOnly {
			for i, u := range imgs {
				targets = append(targets, target{kind: "image", url: u, idx: i + 1})
			}
		}
		if !imagesOnly {
			for i, u := range vids {
				targets = append(targets, target{kind: "video", url: u, idx: i + 1})
			}
		}
		for _, t := range targets {
			report.Total++
			ext := guessMediaExt(t.url, t.kind)
			filename := fmt.Sprintf("%s_%s_%02d%s", sanitizeFilename(ad.AdArchiveID), t.kind, t.idx, ext)
			dest := filepath.Join(outDir, filename)

			if _, err := os.Stat(dest); err == nil {
				report.Skipped++
				report.Files = append(report.Files, MediaFile{
					AdArchiveID: ad.AdArchiveID,
					Kind:        t.kind,
					URL:         t.url,
					LocalPath:   dest,
					Status:      "skipped",
				})
				continue
			}

			if err := c.limiter.Wait(ctx); err != nil {
				return report, err
			}
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.url, nil)
			if err != nil {
				report.Failed++
				report.Files = append(report.Files, MediaFile{
					AdArchiveID: ad.AdArchiveID,
					Kind:        t.kind,
					URL:         t.url,
					Status:      "failed",
					Error:       err.Error(),
				})
				continue
			}
			req.Header.Set("User-Agent", DefaultUserAgent)

			resp, err := c.StreamHTTP.Do(req)
			if err != nil {
				if ctx.Err() != nil {
					return report, ctx.Err()
				}
				report.Failed++
				report.Files = append(report.Files, MediaFile{
					AdArchiveID: ad.AdArchiveID,
					Kind:        t.kind,
					URL:         t.url,
					Status:      "failed",
					Error:       err.Error(),
				})
				continue
			}

			if resp.StatusCode == http.StatusTooManyRequests {
				c.limiter.OnRateLimit()
				wait := cliutil.RetryAfter(resp)
				_ = resp.Body.Close()
				return report, &cliutil.RateLimitError{
					URL:        t.url,
					RetryAfter: wait,
					Body:       "media CDN rate-limited",
				}
			}
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				_ = resp.Body.Close()
				report.Failed++
				report.Files = append(report.Files, MediaFile{
					AdArchiveID: ad.AdArchiveID,
					Kind:        t.kind,
					URL:         t.url,
					Status:      "failed",
					Error:       fmt.Sprintf("HTTP %d", resp.StatusCode),
				})
				continue
			}
			c.limiter.OnSuccess()

			tmpName := fmt.Sprintf(".%s.%s.part", filename, randomToken(8))
			f, err := dirRoot.OpenFile(tmpName, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
			if err != nil {
				_ = resp.Body.Close()
				report.Failed++
				report.Files = append(report.Files, MediaFile{
					AdArchiveID: ad.AdArchiveID,
					Kind:        t.kind,
					URL:         t.url,
					Status:      "failed",
					Error:       err.Error(),
				})
				continue
			}
			n, copyErr := io.Copy(f, io.LimitReader(resp.Body, maxMediaBytes+1))
			closeErr := f.Close()
			_ = resp.Body.Close()
			if copyErr == nil && n > maxMediaBytes {
				copyErr = fmt.Errorf("media file exceeded maximum allowed size (%d bytes)", maxMediaBytes)
			}
			if copyErr != nil || closeErr != nil {
				_ = dirRoot.Remove(tmpName)
				if ctx.Err() != nil {
					return report, ctx.Err()
				}
				errMsg := ""
				if copyErr != nil {
					errMsg = copyErr.Error()
				} else {
					errMsg = closeErr.Error()
				}
				report.Failed++
				report.Files = append(report.Files, MediaFile{
					AdArchiveID: ad.AdArchiveID,
					Kind:        t.kind,
					URL:         t.url,
					Status:      "failed",
					Error:       errMsg,
				})
				continue
			}
			if renameErr := dirRoot.Rename(tmpName, filename); renameErr != nil {
				_ = dirRoot.Remove(tmpName)
				report.Failed++
				report.Files = append(report.Files, MediaFile{
					AdArchiveID: ad.AdArchiveID,
					Kind:        t.kind,
					URL:         t.url,
					Status:      "failed",
					Error:       renameErr.Error(),
				})
				continue
			}

			report.Downloaded++
			report.Files = append(report.Files, MediaFile{
				AdArchiveID: ad.AdArchiveID,
				Kind:        t.kind,
				URL:         t.url,
				LocalPath:   dest,
				Bytes:       n,
				ContentType: resp.Header.Get("Content-Type"),
				Status:      "downloaded",
			})
		}
	}

	return report, nil
}

// ParseTypeaheadResponse extracts Advertiser records from a typeahead GraphQL reply.
func ParseTypeaheadResponse(raw map[string]any, country string) []Advertiser {
	out := make([]Advertiser, 0)
	main := asMap(asMap(raw["data"])["ad_library_main"])
	if len(main) == 0 {
		main = asMap(asMap(raw["data"])["adLibraryMain"])
	}
	var items []any
	switch ts := main["typeahead_suggestions"].(type) {
	case map[string]any:
		if pr, ok := ts["page_results"].([]any); ok {
			items = pr
		}
	case []any:
		items = ts
	}
	if len(items) == 0 {
		if edges, ok := asMap(main["typeahead_suggestions_connection"])["edges"].([]any); ok {
			for _, e := range edges {
				if n := asMap(asMap(e)["node"]); len(n) > 0 {
					items = append(items, n)
				}
			}
		}
	}

	for _, it := range items {
		m := asMap(it)
		pid := firstNonEmpty(asString(m["page_id"]), asString(m["pageID"]), asString(m["id"]))
		name := firstNonEmpty(asString(m["name"]), asString(m["page_name"]), asString(m["pageName"]), asString(m["page_alias"]))
		if pid == "" && name == "" {
			continue
		}
		adv := Advertiser{
			PageID:         pid,
			PageName:       name,
			Category:       asString(m["category"]),
			Likes:          asInt64(m["likes"]),
			Verification:   asString(m["verification"]),
			IGUsername:     asString(m["ig_username"]),
			IGFollowers:    asInt64(m["ig_followers"]),
			IGVerification: asBool(m["ig_verification"]),
			PageAlias:      asString(m["page_alias"]),
			ImageURI:       asString(m["image_uri"]),
			EntityType:     asString(m["entity_type"]),
			Country:        country,
			AdLibraryURL:   BuildAdLibraryURL("", pid, "", country, "active", "all"),
		}
		out = append(out, adv)
	}
	return out
}

// ParseSearchResponse extracts normalized Ad rows and pagination metadata from
// an AdLibrarySearchPaginationQuery response.
func ParseSearchResponse(raw map[string]any, country, adType string, now time.Time) (ads []Ad, nextCursor string, hasNext bool, totalCount int) {
	ads, nextCursor, hasNext, _, totalCount, _ = parseSearchResponseValidated(raw, country, adType, now)
	return ads, nextCursor, hasNext, totalCount
}

func parseSearchResponseValidated(raw map[string]any, country, adType string, now time.Time) (ads []Ad, nextCursor string, hasNext bool, hasExplicitEnd bool, totalCount int, validConn bool) {
	ads = make([]Ad, 0)
	main := asMap(asMap(raw["data"])["ad_library_main"])
	conn := asMap(main["search_results_connection"])
	if len(conn) == 0 {
		return ads, "", false, false, 0, false
	}
	edges, edgesOK := conn["edges"].([]any)
	if !edgesOK {
		return ads, "", false, false, 0, false
	}
	validConn = true
	totalCount = int(asInt64(conn["count"]))
	pageInfo := asMap(conn["page_info"])
	nextCursor = asString(pageInfo["end_cursor"])
	if hn, ok := pageInfo["has_next_page"].(bool); ok {
		hasNext = hn
		hasExplicitEnd = !hn
	} else {
		hasNext = asBool(pageInfo["has_next_page"])
	}

	collectedAt := now.UTC().Format(utcTimestampLayout)
	for _, e := range edges {
		node := asMap(asMap(e)["node"])
		collated, _ := node["collated_results"].([]any)
		for _, cr := range collated {
			row := asMap(cr)
			ad := normalizeGraphQLAd(row, country, adType, now, collectedAt)
			if ad.AdArchiveID != "" {
				ads = append(ads, ad)
			}
		}
	}
	return ads, nextCursor, hasNext, hasExplicitEnd, totalCount, validConn
}

func normalizeGraphQLAd(row map[string]any, country, adType string, now time.Time, collectedAt string) Ad {
	snap := asMap(row["snapshot"])
	id := firstNonEmpty(asString(row["ad_archive_id"]), asString(row["id"]), asString(row["adArchiveID"]))
	pageID := firstNonEmpty(asString(row["page_id"]), asString(snap["page_id"]))
	pageName := firstNonEmpty(asString(row["page_name"]), asString(snap["page_name"]))

	startDate := formatEpochOrString(row["start_date"])
	endDate := formatEpochOrString(row["end_date"])
	isActive := asBool(row["is_active"])
	collCount := int(asInt64(row["collation_count"]))
	if collCount <= 0 {
		collCount = 1
	}

	rawLink := firstNonEmpty(asString(snap["link_url"]), asString(row["link_url"]))
	landingURL := UnwrapRedirectURL(rawLink)
	landingDomain := ExtractDomain(landingURL)

	bodyText := asString(asMap(snap["body"])["text"])
	if bodyText == "" {
		bodyText = asString(snap["body"])
	}

	images := make([]string, 0)
	if rawImgs, ok := snap["images"].([]any); ok {
		for _, im := range rawImgs {
			imMap := asMap(im)
			if u := firstNonEmpty(asString(imMap["original_image_url"]), asString(imMap["resized_image_url"]), asString(imMap["url"])); u != "" {
				images = append(images, u)
			}
		}
	}

	videos := make([]string, 0)
	if rawVids, ok := snap["videos"].([]any); ok {
		for _, vd := range rawVids {
			vdMap := asMap(vd)
			if u := firstNonEmpty(asString(vdMap["video_hd_url"]), asString(vdMap["video_sd_url"]), asString(vdMap["url"])); u != "" {
				videos = append(videos, u)
			}
		}
	}

	cards := make([]CreativeCard, 0)
	if rawCards, ok := snap["cards"].([]any); ok {
		for _, rc := range rawCards {
			cm := asMap(rc)
			cardRawLink := asString(cm["link_url"])
			cardCleanLink := UnwrapRedirectURL(cardRawLink)
			card := CreativeCard{
				Body:          asString(cm["body"]),
				Title:         asString(cm["title"]),
				Caption:       asString(cm["caption"]),
				LinkURL:       cardRawLink,
				LandingURL:    cardCleanLink,
				LandingDomain: ExtractDomain(cardCleanLink),
				LinkDesc:      asString(cm["link_description"]),
				CTAText:       asString(cm["cta_text"]),
				ImageURL:      firstNonEmpty(asString(cm["original_image_url"]), asString(cm["resized_image_url"])),
				VideoURL:      asString(cm["video_sd_url"]),
				VideoHDURL:    asString(cm["video_hd_url"]),
				VideoPreview:  asString(cm["video_preview_image_url"]),
			}
			cards = append(cards, card)
			if landingURL == "" && cardCleanLink != "" {
				landingURL = cardCleanLink
				landingDomain = card.LandingDomain
				rawLink = cardRawLink
			}
			if bodyText == "" && card.Body != "" {
				bodyText = card.Body
			}
		}
	}

	displayFormat := strings.ToUpper(strings.TrimSpace(asString(snap["display_format"])))
	if displayFormat == "" {
		switch {
		case len(cards) > 1:
			displayFormat = "CAROUSEL"
		case len(videos) > 0:
			displayFormat = "VIDEO"
		case len(images) > 0:
			displayFormat = "IMAGE"
		default:
			displayFormat = "TEXT"
		}
	}

	var spendLo, spendHi *int64
	if sm := asMap(row["spend"]); len(sm) > 0 {
		if v, ok := optionalInt64(sm["lower_bound"]); ok {
			spendLo = &v
		}
		if v, ok := optionalInt64(sm["upper_bound"]); ok {
			spendHi = &v
		}
	}
	var impLo, impHi *int64
	if im := asMap(row["impressions_with_index"]); len(im) > 0 {
		if text := asString(im["impressions_text"]); text != "" {
			lo, hi := parseRangeText(text)
			impLo, impHi = lo, hi
		}
	}

	platforms := asStringSlice(row["publisher_platform"])
	if len(platforms) == 0 {
		platforms = asStringSlice(row["publisher_platforms"])
	}

	return Ad{
		AdArchiveID:       id,
		CollationID:       asString(row["collation_id"]),
		CollationCount:    collCount,
		PageID:            pageID,
		PageName:          pageName,
		PageProfileURI:    asString(snap["page_profile_uri"]),
		PageLikeCount:     asInt64(snap["page_like_count"]),
		PageCategories:    asStringSlice(snap["page_categories"]),
		IsActive:          isActive,
		StartDate:         startDate,
		EndDate:           endDate,
		DaysRunning:       ComputeDaysRunning(startDate, endDate, isActive, now),
		Country:           country,
		AdType:            adType,
		PublisherPlatform: platforms,
		DisplayFormat:     displayFormat,
		BodyText:          bodyText,
		LinkTitle:         asString(snap["title"]),
		LinkCaption:       asString(snap["caption"]),
		LinkDescription:   asString(snap["link_description"]),
		CTAText:           asString(snap["cta_text"]),
		CTAType:           asString(snap["cta_type"]),
		RawLinkURL:        rawLink,
		LandingURL:        landingURL,
		LandingDomain:     landingDomain,
		ImageURLs:         UniqueStrings(images),
		VideoURLs:         UniqueStrings(videos),
		Cards:             cards,
		Currency:          asString(row["currency"]),
		SpendLower:        spendLo,
		SpendUpper:        spendHi,
		ImpressionsLower:  impLo,
		ImpressionsUpper:  impHi,
		ReachEstimate:     asString(row["reach_estimate"]),
		Byline:            asString(snap["byline"]),
		AdLibraryURL:      BuildAdLibraryURL(id, "", "", country, "all", adType),
		Source:            "public_graphql",
		CollectedAt:       collectedAt,
	}
}

// ParseAdDetailsResponse extracts transparency, beneficiary/payer, demographic
// reach, and violation disclosures from AdLibraryV3AdDetailsQuery.
func ParseAdDetailsResponse(raw map[string]any, adArchiveID, pageID, country string, now time.Time) (AdDetails, error) {
	main := asMap(asMap(raw["data"])["ad_library_main"])
	det := asMap(main["ad_details"])
	if len(det) == 0 {
		return AdDetails{}, fmt.Errorf("ad details unavailable for ad %s (empty ad_details payload)", adArchiveID)
	}

	out := AdDetails{
		AdArchiveID:       adArchiveID,
		PageID:            pageID,
		Country:           country,
		IsViolatingEUSIEP: asBool(det["is_violating_eu_siep"]),
		AIDisclosureElig:  asBool(det["is_siep_advertiser_eligible_for_ai_disclosure"]),
		ViolationTypes:    asStringSlice(det["violation_types"]),
		RawDetails:        det,
		FetchedAt:         now.UTC().Format(time.RFC3339),
	}

	if adv := asMap(det["advertiser"]); len(adv) > 0 {
		if pi := asMap(adv["ad_library_page_info"]); len(pi) > 0 {
			if p := asMap(pi["page_info"]); len(p) > 0 {
				out.PageName = asString(p["page_name"])
			}
		}
		if out.PageName == "" {
			out.PageName = asString(adv["page_name"])
		}
	}

	// Parse aaa_info (EU/UK/BR transparency block).
	if aaa := asMap(det["aaa_info"]); len(aaa) > 0 {
		out.EUTotalReach = asInt64(aaa["eu_total_reach"])
		out.TargetGender = asString(aaa["gender_audience"])
		if ageMin := asInt64(aaa["age_audience_min"]); ageMin > 0 {
			ageMax := asInt64(aaa["age_audience_max"])
			if ageMax > 0 {
				out.TargetAges = []string{fmt.Sprintf("%d-%d", ageMin, ageMax)}
			} else {
				out.TargetAges = []string{fmt.Sprintf("%d+", ageMin)}
			}
		}
		if locs, ok := aaa["location_audience"].([]any); ok {
			for _, l := range locs {
				if lm := asMap(l); len(lm) > 0 {
					if name := asString(lm["name"]); name != "" {
						out.TargetLocations = append(out.TargetLocations, name)
					}
				}
			}
		}
		if bps, ok := aaa["payer_beneficiary_data"].([]any); ok {
			for _, bp := range bps {
				if bpm := asMap(bp); len(bpm) > 0 {
					out.BeneficiaryPayers = append(out.BeneficiaryPayers, BeneficiaryPayer{
						Beneficiary: asString(bpm["beneficiary"]),
						Payer:       asString(bpm["payer"]),
						Current:     true,
					})
				}
			}
		}
		if breakdowns, ok := aaa["age_country_gender_reach_breakdown"].([]any); ok {
			for _, b := range breakdowns {
				bm := asMap(b)
				cCode := asString(bm["country"])
				if ages, ok := bm["age_gender_breakdowns"].([]any); ok {
					for _, a := range ages {
						am := asMap(a)
						ageRange := asString(am["age_range"])
						if m := asInt64(am["male"]); m > 0 {
							out.Demographics = append(out.Demographics, DemographicEntry{Country: cCode, AgeRange: ageRange, Gender: "Male", Reach: m})
						}
						if f := asInt64(am["female"]); f > 0 {
							out.Demographics = append(out.Demographics, DemographicEntry{Country: cCode, AgeRange: ageRange, Gender: "Female", Reach: f})
						}
						if u := asInt64(am["unknown"]); u > 0 {
							out.Demographics = append(out.Demographics, DemographicEntry{Country: cCode, AgeRange: ageRange, Gender: "Unknown", Reach: u})
						}
					}
				}
			}
		}
	}

	// Parse transparency_by_location for UK / BR reach.
	if tbl := asMap(det["transparency_by_location"]); len(tbl) > 0 {
		if uk := asMap(tbl["uk_transparency"]); len(uk) > 0 {
			out.UKTotalReach = asInt64(uk["total_reach"])
		}
		if br := asMap(tbl["br_transparency"]); len(br) > 0 {
			out.BRTotalReach = asInt64(br["total_reach"])
		}
		if eu := asMap(tbl["eu_transparency"]); len(eu) > 0 && out.EUTotalReach == 0 {
			out.EUTotalReach = asInt64(eu["total_reach"])
		}
	}

	// Parse political/issue insights block when present.
	if ins := asMap(det["insights"]); len(ins) > 0 {
		out.Byline = firstNonEmpty(out.Byline, asString(ins["byline"]), asString(ins["disclaimer"]))
		out.Currency = asString(ins["currency"])
		if spend := asMap(ins["spend"]); len(spend) > 0 {
			if v, ok := optionalInt64(spend["lower_bound"]); ok {
				out.SpendLower = &v
			}
			if v, ok := optionalInt64(spend["upper_bound"]); ok {
				out.SpendUpper = &v
			}
		}
		if imp := asMap(ins["impressions"]); len(imp) > 0 {
			if v, ok := optionalInt64(imp["lower_bound"]); ok {
				out.ImpressionsLower = &v
			}
			if v, ok := optionalInt64(imp["upper_bound"]); ok {
				out.ImpressionsUpper = &v
			}
		}
	}

	return out, nil
}

func normalizeCountry(c string) string {
	c = strings.ToUpper(strings.TrimSpace(c))
	if c == "" {
		return "US"
	}
	return c
}

func normalizeAdType(t string) string {
	switch strings.ToUpper(strings.TrimSpace(t)) {
	case "", "ALL":
		return "ALL"
	case "POLITICAL", "POLITICAL_AND_ISSUE_ADS":
		return "POLITICAL_AND_ISSUE_ADS"
	case "HOUSING", "HOUSING_ADS":
		return "HOUSING_ADS"
	case "EMPLOYMENT", "EMPLOYMENT_ADS":
		return "EMPLOYMENT_ADS"
	case "CREDIT", "CREDIT_ADS", "FINANCIAL", "FINANCIAL_PRODUCTS_AND_SERVICES_ADS":
		return "CREDIT_ADS"
	default:
		return strings.ToUpper(strings.TrimSpace(t))
	}
}

func normalizeStatus(s string) string {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "", "ACTIVE":
		return "ACTIVE"
	case "INACTIVE":
		return "INACTIVE"
	case "ALL":
		return "ALL"
	default:
		return "ACTIVE"
	}
}

func normalizeMediaType(m string) string {
	switch strings.ToUpper(strings.TrimSpace(m)) {
	case "", "ALL":
		return "ALL"
	case "IMAGE", "VIDEO", "MEME", "NONE":
		return strings.ToUpper(strings.TrimSpace(m))
	default:
		return "ALL"
	}
}

func normalizeSearchType(st string, hasPageIDs bool) string {
	switch strings.ToUpper(strings.TrimSpace(st)) {
	case "EXACT", "KEYWORD_EXACT_PHRASE":
		return "KEYWORD_EXACT_PHRASE"
	case "PAGE":
		return "PAGE"
	case "KEYWORD", "KEYWORD_UNORDERED":
		return "KEYWORD_UNORDERED"
	default:
		if hasPageIDs {
			return "PAGE"
		}
		return "KEYWORD_UNORDERED"
	}
}

func formatEpochOrString(v any) string {
	switch t := v.(type) {
	case float64:
		if t > 0 {
			return time.Unix(int64(t), 0).UTC().Format(time.RFC3339)
		}
	case int64:
		if t > 0 {
			return time.Unix(t, 0).UTC().Format(time.RFC3339)
		}
	case json.Number:
		if n, err := t.Int64(); err == nil && n > 0 {
			return time.Unix(n, 0).UTC().Format(time.RFC3339)
		}
	case string:
		s := strings.TrimSpace(t)
		if n, err := strconv.ParseInt(s, 10, 64); err == nil && n > 1_000_000_000 {
			return time.Unix(n, 0).UTC().Format(time.RFC3339)
		}
		return s
	}
	return ""
}

func parseRangeText(s string) (*int64, *int64) {
	matches := reRangeText.FindAllStringSubmatch(s, -1)
	vals := make([]int64, 0, 2)
	for _, m := range matches {
		numStr := strings.ReplaceAll(m[1], ",", "")
		f, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			continue
		}
		switch strings.ToUpper(m[2]) {
		case "K":
			f *= 1_000
		case "M":
			f *= 1_000_000
		case "B":
			f *= 1_000_000_000
		}
		vals = append(vals, int64(f))
	}
	if len(vals) >= 2 {
		return &vals[0], &vals[1]
	}
	if len(vals) == 1 {
		return &vals[0], nil
	}
	return nil, nil
}

func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strconv.FormatInt(int64(t), 10)
	case int64:
		return strconv.FormatInt(t, 10)
	case int:
		return strconv.Itoa(t)
	case json.Number:
		return t.String()
	default:
		return ""
	}
}

func asInt64(v any) int64 {
	n, _ := optionalInt64(v)
	return n
}

func optionalInt64(v any) (int64, bool) {
	switch t := v.(type) {
	case float64:
		return int64(t), true
	case int64:
		return t, true
	case int:
		return int64(t), true
	case json.Number:
		if n, err := t.Int64(); err == nil {
			return n, true
		}
	case string:
		s := strings.ReplaceAll(strings.TrimSpace(t), ",", "")
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			return n, true
		}
	}
	return 0, false
}

func asBool(v any) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

func asStringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s := asString(item); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func firstOrEmpty(vals []string) string {
	if len(vals) > 0 {
		return vals[0]
	}
	return ""
}

func randomShortSessionID() string {
	b := make([]byte, 9)
	_, _ = rand.Read(b)
	h := hex.EncodeToString(b)
	return h[:6] + ":" + h[6:12] + ":" + h[12:18]
}

func randomToken(n int) string {
	b := make([]byte, (n+1)/2)
	_, _ = rand.Read(b)
	s := hex.EncodeToString(b)
	if len(s) > n {
		return s[:n]
	}
	return s
}

func randomUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func truncateBody(s string, max int) string {
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}

func guessMediaExt(rawURL, kind string) string {
	if u, err := url.Parse(rawURL); err == nil {
		ext := strings.ToLower(filepath.Ext(u.Path))
		switch ext {
		case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".mp4", ".mov", ".webm":
			return ext
		}
	}
	if kind == "video" {
		return ".mp4"
	}
	return ".jpg"
}

func sanitizeFilename(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "ad"
	}
	return b.String()
}
