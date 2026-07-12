// Package indiapassivefunds is a hand-written sibling client for
// data.indiapassivefunds.com. This API's auth does not fit any spec
// auth.type: it is a credential-less runtime-minted Bearer token (no user
// secret), minted via a same-origin Next.js route on a DIFFERENT host
// (www.indiapassivefunds.com) than the data API itself
// (data.indiapassivefunds.com). Per AGENTS.md's custom-auth-flow pattern,
// this whole layer is hand-written rather than generated.
package indiapassivefunds

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/passive-indices/internal/cliutil"
)

const (
	loginURL     = "https://www.indiapassivefunds.com/pages/api/login"
	dataBaseURL  = "https://data.indiapassivefunds.com/api/v1/etf"
	userAgent    = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	tokenRefresh = 30 * time.Second // refresh this long before the token's own expiry
)

// Client calls indiapassivefunds.com's data API, minting and caching its own
// short-lived Bearer token (no user credential involved).
type Client struct {
	httpClient *http.Client
	limiter    *cliutil.AdaptiveLimiter

	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

// New returns a Client paced at ratePerSec requests/second (0 disables pacing).
func New(timeout time.Duration, ratePerSec float64) *Client {
	var limiter *cliutil.AdaptiveLimiter
	if ratePerSec > 0 {
		limiter = cliutil.NewAdaptiveLimiter(ratePerSec)
	}
	return &Client{
		httpClient: &http.Client{Timeout: timeout},
		limiter:    limiter,
	}
}

type loginResponse struct {
	Status   bool   `json:"status"`
	Message  string `json:"message"`
	Response struct {
		Token      string `json:"token"`
		Expiration string `json:"expiration"`
	} `json:"response"`
}

// ensureToken mints a fresh token if none is cached or the cached one is
// close to expiry. Thread-safe.
func (c *Client) ensureToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.token != "" && time.Now().Add(tokenRefresh).Before(c.expiresAt) {
		return c.token, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, loginURL, bytes.NewReader([]byte("{}")))
	if err != nil {
		return "", fmt.Errorf("building token-mint request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", "https://www.indiapassivefunds.com/")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("minting token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading token-mint response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("token mint returned HTTP %d: %s", resp.StatusCode, truncate(string(body), 300))
	}

	var lr loginResponse
	if err := json.Unmarshal(body, &lr); err != nil {
		return "", fmt.Errorf("parsing token-mint response: %w", err)
	}
	if !lr.Status || lr.Response.Token == "" {
		return "", fmt.Errorf("token mint failed: %s", lr.Message)
	}

	expiry := time.Now().Add(20 * time.Hour) // conservative fallback
	if parsed, err := time.Parse(time.RFC3339, lr.Response.Expiration); err == nil {
		expiry = parsed
	}

	c.token = lr.Response.Token
	c.expiresAt = expiry
	return c.token, nil
}

func (c *Client) wait() {
	if c.limiter != nil {
		c.limiter.Wait()
	}
}

func (c *Client) onResult(status int) {
	if c.limiter == nil {
		return
	}
	if status == http.StatusTooManyRequests {
		c.limiter.OnRateLimit()
		return
	}
	if status >= 200 && status < 300 {
		c.limiter.OnSuccess()
	}
}

// get calls a data.indiapassivefunds.com endpoint with the given query
// params, minting/attaching the Bearer token automatically. Retries once on
// 401 in case the cached token expired mid-flight.
func (c *Client) get(ctx context.Context, endpoint string, params url.Values) (json.RawMessage, error) {
	for attempt := 0; attempt < 2; attempt++ {
		token, err := c.ensureToken(ctx)
		if err != nil {
			return nil, err
		}

		reqURL := dataBaseURL + "/" + endpoint
		if len(params) > 0 {
			reqURL += "?" + params.Encode()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, fmt.Errorf("building request for %s: %w", endpoint, err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", userAgent)

		c.wait()
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("calling %s: %w", endpoint, err)
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		c.onResult(resp.StatusCode)
		if readErr != nil {
			return nil, fmt.Errorf("reading %s response: %w", endpoint, readErr)
		}

		if resp.StatusCode == http.StatusUnauthorized && attempt == 0 {
			// Cached token rejected; force a fresh mint and retry once.
			c.mu.Lock()
			c.token = ""
			c.mu.Unlock()
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			return nil, &cliutil.RateLimitError{URL: reqURL}
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("%s returned HTTP %d: %s", endpoint, resp.StatusCode, truncate(string(body), 300))
		}
		return json.RawMessage(body), nil
	}
	return nil, fmt.Errorf("%s: exhausted retries after 401", endpoint)
}

// ColumnMeta describes one field-code column in a list response, e.g. f_29
// -> "Name". indiapassivefunds' list endpoints return field-coded rows
// alongside this metadata array.
type ColumnMeta struct {
	Field       string `json:"field"`
	DisplayName string `json:"displayName"`
	DataType    string `json:"dataType"`
	Format      string `json:"format"`
}

// ListEnvelope is the common shape of list endpoints: paging info, column
// metadata for decoding field codes, and field-coded data rows.
type ListEnvelope struct {
	PagingInfo struct {
		TotalRecords int `json:"totalRecords"`
		PageNo       int `json:"pageNo"`
		PageSize     int `json:"pageSize"`
		RecordCount  int `json:"recordCount"`
		NoOfPages    int `json:"noOfPages"`
	} `json:"pagingInfo"`
	Columns []ColumnMeta                 `json:"columns"`
	Data    []map[string]json.RawMessage `json:"data"`
}

// Decode flattens one field-coded row using the envelope's column metadata,
// returning a map keyed by human displayName instead of raw field codes.
func (e *ListEnvelope) Decode(row map[string]json.RawMessage) map[string]any {
	byField := make(map[string]string, len(e.Columns))
	for _, col := range e.Columns {
		byField[col.Field] = col.DisplayName
	}
	out := make(map[string]any, len(row))
	for field, raw := range row {
		name := field
		if dn, ok := byField[field]; ok && dn != "" {
			name = dn
		}
		var v any
		if err := json.Unmarshal(raw, &v); err == nil {
			out[name] = v
		}
	}
	return out
}

func parseListEnvelope(raw json.RawMessage) (*ListEnvelope, error) {
	var outer struct {
		Status   bool         `json:"status"`
		Message  string       `json:"message"`
		Response ListEnvelope `json:"response"`
	}
	if err := json.Unmarshal(raw, &outer); err != nil {
		return nil, fmt.Errorf("parsing response envelope: %w", err)
	}
	if !outer.Status {
		return nil, fmt.Errorf("upstream reported failure: %s", outer.Message)
	}
	return &outer.Response, nil
}

// Dashboard fetches the AUM/overview dashboard widgets.
func (c *Client) Dashboard(ctx context.Context) (json.RawMessage, error) {
	return c.get(ctx, "dashboard", url.Values{"cachedResponse": {"false"}})
}

// FilterOption is one enumerated choice in the screener filter taxonomy,
// e.g. {"text": "Nifty 50 TRI", "value": 320} in the underlyingIndex list.
type FilterOption struct {
	Text  string `json:"text"`
	Value any    `json:"value"`
}

// ScreenerFilterTaxonomy is the parsed shape of screeners/filters.
type ScreenerFilterTaxonomy struct {
	FundType        []FilterOption `json:"fundType"`
	AssetType       []FilterOption `json:"assetType"`
	UnderlyingIndex []FilterOption `json:"underlyingIndex"`
	AMC             []FilterOption `json:"amc"`
}

// ScreenerFilters fetches the filter taxonomy (fundType, assetType,
// underlyingIndex, amc, ...).
func (c *Client) ScreenerFilters(ctx context.Context) (*ScreenerFilterTaxonomy, error) {
	raw, err := c.get(ctx, "screeners/filters", url.Values{"cachedResponse": {"false"}})
	if err != nil {
		return nil, err
	}
	var outer struct {
		Status   bool                   `json:"status"`
		Message  string                 `json:"message"`
		Response ScreenerFilterTaxonomy `json:"response"`
	}
	if err := json.Unmarshal(raw, &outer); err != nil {
		return nil, fmt.Errorf("parsing screener filters: %w", err)
	}
	if !outer.Status {
		return nil, fmt.Errorf("upstream reported failure: %s", outer.Message)
	}
	return &outer.Response, nil
}

// FindUnderlyingIndexValue matches a human index name (e.g. "NIFTY 50") to
// its numeric underlyingIndex filter value (e.g. 320 for "Nifty 50 TRI").
// indiapassivefunds enumerates benchmark indices as "<Index Name> TRI" (or
// occasionally without the TRI suffix); this does a normalized substring
// match and prefers the shortest/most exact match to avoid "NIFTY 50"
// matching "NIFTY 500" or "NIFTY 50 Equal Weight".
func FindUnderlyingIndexValue(taxonomy *ScreenerFilterTaxonomy, indexName string) (value any, matchedText string, found bool) {
	target := normalizeIndexName(indexName)
	var bestText string
	var bestValue any
	bestLen := -1
	for _, opt := range taxonomy.UnderlyingIndex {
		if opt.Text == "" || opt.Value == nil {
			continue
		}
		candidate := normalizeIndexName(opt.Text)
		// Exact match on "<target> tri" or "<target>" alone.
		if candidate == target || candidate == target+" tri" {
			return opt.Value, opt.Text, true
		}
		// Otherwise track the shortest candidate that starts with the
		// target followed by a space or end-of-string, so "nifty 50"
		// does not greedily match "nifty 500 tri".
		if strings.HasPrefix(candidate, target+" ") {
			if bestLen == -1 || len(candidate) < bestLen {
				bestLen = len(candidate)
				bestText = opt.Text
				bestValue = opt.Value
			}
		}
	}
	if bestValue != nil {
		return bestValue, bestText, true
	}
	return nil, "", false
}

// letterDigitBoundaryRe finds a letter immediately followed by a digit, or a
// digit immediately followed by a letter. niftyindices and indiapassivefunds
// disagree on whether a space belongs at this boundary (e.g. niftyindices'
// "MIDSMALLCAP400" vs indiapassivefunds' "MidSmallcap 400" for the same
// index), so normalizeIndexName inserts a space at every such boundary to
// make both sides compare equal regardless of which convention either site
// happens to use.
var letterDigitBoundaryRe = regexp.MustCompile(`([a-z])(\d)|(\d)([a-z])`)

func normalizeIndexName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimSuffix(s, " tri")
	s = letterDigitBoundaryRe.ReplaceAllString(s, "$1$3 $2$4")
	return strings.Join(strings.Fields(s), " ")
}

// FindAMCValue matches a human AMC name fragment (e.g. "HDFC") to its
// numeric amc filter value. amc, like underlyingIndex, is a
// server-enumerated {text, value} list, not a free-text query param.
func FindAMCValue(taxonomy *ScreenerFilterTaxonomy, amcName string) (value any, matchedText string, found bool) {
	target := strings.ToLower(strings.TrimSpace(amcName))
	if target == "" {
		return nil, "", false
	}
	for _, opt := range taxonomy.AMC {
		if opt.Text == "" || opt.Value == nil {
			continue
		}
		if strings.Contains(strings.ToLower(opt.Text), target) {
			return opt.Value, opt.Text, true
		}
	}
	return nil, "", false
}

// SymbolLookupParams narrows a fund/scheme search.
type SymbolLookupParams struct {
	SearchTerm     string
	PageNo         int
	PageSize       int
	InstrumentType string
}

// SymbolLookup searches funds/schemes by name fragment.
func (c *Client) SymbolLookup(ctx context.Context, p SymbolLookupParams) (*ListEnvelope, error) {
	if p.PageNo == 0 {
		p.PageNo = 1
	}
	if p.PageSize == 0 {
		p.PageSize = 20
	}
	if p.InstrumentType == "" {
		p.InstrumentType = "all"
	}
	params := url.Values{
		"searchTerm":     {p.SearchTerm},
		"pageNo":         {strconv.Itoa(p.PageNo)},
		"pageSize":       {strconv.Itoa(p.PageSize)},
		"instrumentType": {p.InstrumentType},
		"cachedResponse": {"false"},
	}
	raw, err := c.get(ctx, "symbollookup", params)
	if err != nil {
		return nil, err
	}
	return parseListEnvelope(raw)
}

// FundDetail fetches one fund's full detail row (field-coded).
// FundDetail is the parsed shape of a fund detail response. The raw
// response is a nested, inconsistently-shaped structure (sections mix flat
// fields and embedded objects); this extracts the fields the CLI's absorbed
// and transcendence features need, keeping the raw envelope available too.
type FundDetail struct {
	SchemeID      string
	SchemeName    string
	CategoryName  string
	SchemeType    string
	Riskometer    string
	BenchmarkText string // e.g. "Nifty 50 TRI" — the join key to niftyindices
	NAV           string
	NAVDate       string
	AUM           string
	AUMDate       string
	ExpenseRatio  float64 // from ratios.data, most recent ReportedDate
	TrackingError float64
	TrackingDiff  float64
	RatiosAsOf    string
	SectorWeights []NamedPercent
	TopHoldings   []NamedPercent
	SimilarFunds  []SimilarFund
	Raw           json.RawMessage
}

// NamedPercent is a name/percentage pair (sector weight, portfolio holding).
type NamedPercent struct {
	Name    string  `json:"name"`
	Percent float64 `json:"percent"`
}

// SimilarFund is a peer fund entry from the similarfunds section.
type SimilarFund struct {
	SchemeID   string `json:"scheme_id"`
	SchemeName string `json:"scheme_name"`
	Category   string `json:"category"`
}

// FundDetail fetches and parses one fund's full detail.
func (c *Client) FundDetail(ctx context.Context, schemeID string) (*FundDetail, error) {
	params := url.Values{"schemeId": {schemeID}, "cachedResponse": {"false"}}
	raw, err := c.get(ctx, "funddetail", params)
	if err != nil {
		return nil, err
	}

	var outer struct {
		Status   bool            `json:"status"`
		Message  string          `json:"message"`
		Response json.RawMessage `json:"response"`
	}
	if err := json.Unmarshal(raw, &outer); err != nil {
		return nil, fmt.Errorf("parsing fund detail envelope: %w", err)
	}
	if !outer.Status {
		return nil, fmt.Errorf("upstream reported failure: %s", outer.Message)
	}

	fd := &FundDetail{SchemeID: schemeID, Raw: outer.Response}
	var resp map[string]json.RawMessage
	if err := json.Unmarshal(outer.Response, &resp); err != nil {
		return nil, fmt.Errorf("parsing fund detail response: %w", err)
	}

	// header.data[0]: schemename, CategoryName, SchemeType, riskometer
	if headerRow := firstDataRow(resp["header"]); headerRow != nil {
		fd.SchemeName = fieldByDisplayName(headerRow, resp["header"], "schemename")
		fd.CategoryName = fieldByDisplayName(headerRow, resp["header"], "categoryname")
		fd.SchemeType = fieldByDisplayName(headerRow, resp["header"], "schemetype")
		fd.Riskometer = fieldByDisplayName(headerRow, resp["header"], "riskometer")
	}

	// funddescription.data is a mixed-shape array; deep-scan for a column
	// whose displayName mentions "benchmark" or is exactly "index".
	fd.BenchmarkText = deepFindBenchmarkIndex(resp["funddescription"])

	// fundamentals.data is an array of {f_01: label, f_02: value, f_03: asof}
	// triplets rather than field-coded columns.
	if triplets := labelValueTriplets(resp["fundamentals"]); triplets != nil {
		for label, lv := range triplets {
			lower := strings.ToLower(label)
			switch {
			case lower == "nav":
				fd.NAV, fd.NAVDate = lv.value, lv.asOf
			case strings.Contains(lower, "aum"):
				fd.AUM, fd.AUMDate = lv.value, lv.asOf
			}
		}
	}

	// ratios.data: array of rows keyed by field code, one row per reported
	// month. Take the most recent non-null Tracking Error / Difference /
	// Expense Ratio.
	if ratioRow, asOf := latestRatiosRow(resp["ratios"]); ratioRow != nil {
		fd.RatiosAsOf = asOf
		fd.ExpenseRatio = numericByDisplayNameContains(ratioRow, resp["ratios"], "total expense ratio")
		fd.TrackingError = numericByDisplayNameContains(ratioRow, resp["ratios"], "tracking error")
		fd.TrackingDiff = numericByDisplayNameContains(ratioRow, resp["ratios"], "tracking difference")
	}

	fd.SectorWeights = namedPercentRows(resp["sectorholding"], "sector_name", "percentage")
	fd.TopHoldings = namedPercentRows(resp["portfolio"], "instrument name", "investment %")
	fd.SimilarFunds = parseSimilarFunds(resp["similarfunds"])

	return fd, nil
}

type sectionEnvelope struct {
	Columns []ColumnMeta                 `json:"columns"`
	Data    []map[string]json.RawMessage `json:"data"`
}

func firstDataRow(raw json.RawMessage) map[string]json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var sec sectionEnvelope
	if err := json.Unmarshal(raw, &sec); err != nil || len(sec.Data) == 0 {
		return nil
	}
	return sec.Data[0]
}

// fieldByDisplayName looks up a row's value by the section's column
// metadata, matching case-insensitively against displayName.
func fieldByDisplayName(row map[string]json.RawMessage, sectionRaw json.RawMessage, wantDisplayName string) string {
	var sec sectionEnvelope
	if err := json.Unmarshal(sectionRaw, &sec); err != nil {
		return ""
	}
	want := strings.ToLower(wantDisplayName)
	for _, col := range sec.Columns {
		if strings.ToLower(col.DisplayName) == want {
			if raw, ok := row[col.Field]; ok {
				return rawToString(raw)
			}
		}
	}
	return ""
}

func rawToString(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return string(raw)
}

func rawToFloat(raw json.RawMessage) (float64, bool) {
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return f, true
	}
	return 0, false
}

// deepFindBenchmarkIndex recursively scans a section for any column whose
// displayName contains "benchmark" or equals "index", returning the first
// matching data value. The funddescription section nests a "section1"
// object mid-array with its own columns/data rather than a flat shape.
func deepFindBenchmarkIndex(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return ""
	}
	return scanForBenchmark(generic)
}

func scanForBenchmark(v any) string {
	switch t := v.(type) {
	case map[string]any:
		// If this object looks like a section (has columns+data), check
		// its columns for a benchmark/index displayName.
		if cols, ok := t["columns"].([]any); ok {
			if rows, ok := t["data"].([]any); ok && len(rows) > 0 {
				for _, colAny := range cols {
					col, ok := colAny.(map[string]any)
					if !ok {
						continue
					}
					dn, _ := col["displayName"].(string)
					dnLower := strings.ToLower(dn)
					if strings.Contains(dnLower, "benchmark") || dnLower == "index" {
						field, _ := col["field"].(string)
						if row, ok := rows[0].(map[string]any); ok {
							if val, ok := row[field]; ok {
								if s, ok := val.(string); ok && s != "" {
									return s
								}
							}
						}
					}
				}
			}
		}
		for _, sub := range t {
			if found := scanForBenchmark(sub); found != "" {
				return found
			}
		}
	case []any:
		for _, sub := range t {
			if found := scanForBenchmark(sub); found != "" {
				return found
			}
		}
	}
	return ""
}

type labelValue struct {
	value string
	asOf  string
}

// labelValueTriplets parses the fundamentals section's {f_01:label,
// f_02:value, f_03:asof} row shape into a label->value/asof map.
func labelValueTriplets(raw json.RawMessage) map[string]labelValue {
	if len(raw) == 0 {
		return nil
	}
	var sec struct {
		Data []map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &sec); err != nil {
		return nil
	}
	out := make(map[string]labelValue)
	for _, row := range sec.Data {
		label, hasLabel := row["f_01"]
		value, hasValue := row["f_02"]
		if !hasLabel || !hasValue {
			continue
		}
		lv := labelValue{value: rawToString(value)}
		if asOf, ok := row["f_03"]; ok {
			lv.asOf = rawToString(asOf)
		}
		out[rawToString(label)] = lv
	}
	return out
}

// latestRatiosRow returns the ratios.data row with the most recent
// ReportedDate (field f_0) whose values are non-null, plus that date.
func latestRatiosRow(raw json.RawMessage) (map[string]json.RawMessage, string) {
	if len(raw) == 0 {
		return nil, ""
	}
	var sec sectionEnvelope
	if err := json.Unmarshal(raw, &sec); err != nil {
		return nil, ""
	}
	var best map[string]json.RawMessage
	var bestDate string
	for _, row := range sec.Data {
		dateRaw, ok := row["f_0"]
		if !ok {
			continue
		}
		date := rawToString(dateRaw)
		// Skip rows where every non-date value is null (future reporting
		// periods with no data yet).
		hasValue := false
		for k, v := range row {
			if k == "f_0" {
				continue
			}
			if string(v) != "null" {
				hasValue = true
				break
			}
		}
		if !hasValue {
			continue
		}
		if date > bestDate {
			bestDate = date
			best = row
		}
	}
	return best, bestDate
}

func numericByDisplayNameContains(row map[string]json.RawMessage, sectionRaw json.RawMessage, substr string) float64 {
	var sec sectionEnvelope
	if err := json.Unmarshal(sectionRaw, &sec); err != nil {
		return 0
	}
	want := strings.ToLower(substr)
	for _, col := range sec.Columns {
		if strings.Contains(strings.ToLower(col.DisplayName), want) {
			if raw, ok := row[col.Field]; ok {
				if f, ok := rawToFloat(raw); ok {
					return f
				}
			}
		}
	}
	return 0
}

// namedPercentRows parses a section's rows into name/percent pairs, matching
// columns by displayName substring (case-insensitive).
func namedPercentRows(raw json.RawMessage, nameContains, pctContains string) []NamedPercent {
	if len(raw) == 0 {
		return nil
	}
	var sec sectionEnvelope
	if err := json.Unmarshal(raw, &sec); err != nil {
		return nil
	}
	var nameField, pctField string
	for _, col := range sec.Columns {
		dn := strings.ToLower(col.DisplayName)
		if nameField == "" && strings.Contains(dn, nameContains) {
			nameField = col.Field
		}
		if pctField == "" && strings.Contains(dn, pctContains) {
			pctField = col.Field
		}
	}
	if nameField == "" || pctField == "" {
		return nil
	}
	out := make([]NamedPercent, 0, len(sec.Data))
	for _, row := range sec.Data {
		nameRaw, ok1 := row[nameField]
		pctRaw, ok2 := row[pctField]
		if !ok1 || !ok2 {
			continue
		}
		pct, _ := rawToFloat(pctRaw)
		out = append(out, NamedPercent{Name: rawToString(nameRaw), Percent: pct})
	}
	return out
}

// parseSimilarFunds extracts the similarfunds section's peer fund list.
func parseSimilarFunds(raw json.RawMessage) []SimilarFund {
	if len(raw) == 0 {
		return nil
	}
	var sec sectionEnvelope
	if err := json.Unmarshal(raw, &sec); err != nil {
		return nil
	}
	var nameField, catField, idField string
	for _, col := range sec.Columns {
		dn := strings.ToLower(col.DisplayName)
		switch {
		case nameField == "" && strings.Contains(dn, "schemename"):
			nameField = col.Field
		case catField == "" && strings.Contains(dn, "categoryname"):
			catField = col.Field
		case idField == "" && strings.Contains(dn, "scheme_id"):
			idField = col.Field
		}
	}
	out := make([]SimilarFund, 0, len(sec.Data))
	for _, row := range sec.Data {
		sf := SimilarFund{}
		if nameField != "" {
			if v, ok := row[nameField]; ok {
				sf.SchemeName = rawToString(v)
			}
		}
		if catField != "" {
			if v, ok := row[catField]; ok {
				sf.Category = rawToString(v)
			}
		}
		if idField != "" {
			if v, ok := row[idField]; ok {
				sf.SchemeID = rawToString(v)
			}
		}
		if sf.SchemeName != "" {
			out = append(out, sf)
		}
	}
	return out
}

// NFOParams narrows a New Fund Offer listing.
type NFOParams struct {
	Type     string
	PageNo   int
	PageSize int
}

// NFO lists New Fund Offers.
func (c *Client) NFO(ctx context.Context, p NFOParams) (*ListEnvelope, error) {
	if p.Type == "" {
		p.Type = "all"
	}
	if p.PageNo == 0 {
		p.PageNo = 1
	}
	if p.PageSize == 0 {
		p.PageSize = 50
	}
	params := url.Values{
		"type":           {p.Type},
		"pageNo":         {strconv.Itoa(p.PageNo)},
		"pageSize":       {strconv.Itoa(p.PageSize)},
		"cachedResponse": {"false"},
	}
	raw, err := c.get(ctx, "nfo", params)
	if err != nil {
		return nil, err
	}
	return parseListEnvelope(raw)
}

// ScreenParams narrows a fund screen. UnderlyingIndex is the numeric filter
// value from ScreenerFilters().UnderlyingIndex (resolve via
// FindUnderlyingIndexValue), not the index's display name.
type ScreenParams struct {
	TemplateID        int
	FundTypeID        int
	AssetTypeID       int
	AMC               any
	UnderlyingIndex   any
	PageNo, PageSize  int
	SortBy, SortOrder string
}

// Screen filters funds by AMC/asset-type/underlying-index/etc.
func (c *Client) Screen(ctx context.Context, p ScreenParams) (*ListEnvelope, error) {
	if p.PageNo == 0 {
		p.PageNo = 1
	}
	if p.PageSize == 0 {
		p.PageSize = 50
	}
	params := url.Values{
		"templateId":     {strconv.Itoa(p.TemplateID)},
		"pageNo":         {strconv.Itoa(p.PageNo)},
		"pageSize":       {strconv.Itoa(p.PageSize)},
		"cachedResponse": {"false"},
	}
	if p.FundTypeID != 0 {
		params.Set("fundTypeId", strconv.Itoa(p.FundTypeID))
	}
	if p.AssetTypeID != 0 {
		params.Set("assetTypeId", strconv.Itoa(p.AssetTypeID))
	}
	if p.AMC != nil {
		params.Set("amc", fmt.Sprintf("%v", p.AMC))
	}
	if p.UnderlyingIndex != nil {
		params.Set("underlyingIndex", fmt.Sprintf("%v", p.UnderlyingIndex))
	}
	if p.SortBy != "" {
		params.Set("sortBy", p.SortBy)
		params.Set("sortOrder", p.SortOrder)
	}
	raw, err := c.get(ctx, "screeners", params)
	if err != nil {
		return nil, err
	}
	return parseListEnvelope(raw)
}

// screenerBulkTemplateIDs are the screeners endpoint's three non-redundant
// data templates: Overview (fund identity, benchmark category, tracking
// error/difference, AUM, TER, live price), Returns (1Y/3Y/5Y/since-inception
// %), and Risk (standard deviation, Sharpe ratio, beta). Template 12
// (Expenses Ratio) is deliberately excluded: its AUM/TER fields are a strict
// subset of Overview's and add nothing when merged.
var screenerBulkTemplateIDs = []int{10, 11, 13}

// ScreenerBulkPageSize comfortably covers every published fund/ETF in one
// request per template (~705 total as of writing; screeners/filters carries
// no per-template max, so this is a generous fixed ceiling, not a discovered
// API limit). Exported so callers filtering on a single dimension (e.g.
// underlyingIndex) can request the same ceiling instead of an arbitrary page
// size that risks truncating results for a widely-tracked index.
const ScreenerBulkPageSize = 5000

// ScreenAll fetches every fund/ETF matching the given filter (same
// AMC/underlyingIndex/fundTypeId/assetTypeId semantics as ScreenParams,
// TemplateID/PageNo/PageSize/SortBy/SortOrder are ignored) across all three
// screener templates, merging the results by scheme_id into one row per
// fund. Unlike Screen (single template, caller-paged), this is the "give me
// the complete, correctly-decoded screener data" entry point: one row per
// fund carries every field the screeners endpoint publishes, not just
// whichever template happened to be requested.
func (c *Client) ScreenAll(ctx context.Context, filter ScreenParams) ([]map[string]any, error) {
	extra := url.Values{}
	if filter.FundTypeID != 0 {
		extra.Set("fundTypeId", strconv.Itoa(filter.FundTypeID))
	}
	if filter.AssetTypeID != 0 {
		extra.Set("assetTypeId", strconv.Itoa(filter.AssetTypeID))
	}
	if filter.AMC != nil {
		extra.Set("amc", fmt.Sprintf("%v", filter.AMC))
	}
	if filter.UnderlyingIndex != nil {
		extra.Set("underlyingIndex", fmt.Sprintf("%v", filter.UnderlyingIndex))
	}
	if filter.SortBy != "" {
		extra.Set("sortBy", filter.SortBy)
		extra.Set("sortOrder", filter.SortOrder)
	}

	merged := make(map[string]map[string]any)
	order := make([]string, 0, ScreenerBulkPageSize)

	for _, templateID := range screenerBulkTemplateIDs {
		params := url.Values{
			"templateId":     {strconv.Itoa(templateID)},
			"pageNo":         {"1"},
			"pageSize":       {strconv.Itoa(ScreenerBulkPageSize)},
			"cachedResponse": {"false"},
		}
		for k, v := range extra {
			params[k] = v
		}
		raw, err := c.get(ctx, "screeners", params)
		if err != nil {
			return nil, fmt.Errorf("fetching screener template %d: %w", templateID, err)
		}
		env, err := parseListEnvelope(raw)
		if err != nil {
			return nil, fmt.Errorf("parsing screener template %d: %w", templateID, err)
		}
		mergeScreenerEnvelope(env, merged, &order)
	}

	rows := make([]map[string]any, 0, len(order))
	for _, key := range order {
		rows = append(rows, merged[key])
	}
	return rows, nil
}

// mergeScreenerEnvelope decodes one screener template's rows and folds them
// into merged, keyed by scheme_id. A scheme_id seen in an earlier template
// keeps its first-seen position in order; a later template only adds or
// overwrites fields for that same key, so callers can loop templates in a
// fixed sequence and get output ordered by the first (richest) template.
func mergeScreenerEnvelope(env *ListEnvelope, merged map[string]map[string]any, order *[]string) {
	for _, row := range env.Data {
		decoded := env.Decode(row)
		schemeID, ok := decoded["scheme_id"]
		if !ok {
			continue
		}
		key := fmt.Sprintf("%v", schemeID)
		existing, seen := merged[key]
		if !seen {
			existing = make(map[string]any, len(decoded))
			merged[key] = existing
			*order = append(*order, key)
		}
		for field, value := range decoded {
			existing[field] = value
		}
	}
}

// TimeSeries fetches a fund's NAV/AUM time series. The full envelope
// (status/message/response) is returned as-is on success so callers keep the
// header/types/period metadata alongside the series data; a status:false
// envelope (e.g. an invalid schemeId) is rejected as an error instead of
// being printed as if it succeeded.
func (c *Client) TimeSeries(ctx context.Context, schemeID, tenure string) (json.RawMessage, error) {
	params := url.Values{"schemeId": {schemeID}, "cachedResponse": {"false"}}
	if tenure != "" {
		params.Set("tenure", tenure)
	}
	raw, err := c.get(ctx, "timeseries", params)
	if err != nil {
		return nil, err
	}
	var outer struct {
		Status  bool   `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &outer); err != nil {
		return nil, fmt.Errorf("parsing timeseries response: %w", err)
	}
	if !outer.Status {
		return nil, fmt.Errorf("upstream reported failure: %s", outer.Message)
	}
	return raw, nil
}

// FundCompare fetches comparison data for a fund.
func (c *Client) FundCompare(ctx context.Context, schemeID string) (json.RawMessage, error) {
	params := url.Values{"schemeId": {schemeID}, "cachedResponse": {"false"}}
	return c.get(ctx, "fundcompare", params)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
