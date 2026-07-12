// Package niftyindices is a hand-written sibling client for niftyindices.com
// surfaces the generator's spec-driven endpoint mirror cannot express: the
// /BackPage/ historical PageMethods, whose request body is a `cinfo` field
// containing an escaped JSON string composed from other params.
//
// IMPORTANT: the correct path is /BackPage/ (capital P, no .aspx extension).
// The legacy /Backpage.aspx/ variant now redirects to a Sitefinity login page
// and must never be used.
package niftyindices

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/passive-indices/internal/cliutil"
)

const (
	baseURL     = "https://www.niftyindices.com"
	userAgent   = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	dateLayout  = "02-Jan-2006"
	minStartYYY = "01-Jan-2000"
)

// Client calls niftyindices.com's historical PageMethods. All three
// endpoints (OHLC, TRI, PE/PB/DivYield) require no authentication.
type Client struct {
	httpClient *http.Client
	limiter    *cliutil.AdaptiveLimiter
	rateLimit  float64
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
		rateLimit:  ratePerSec,
	}
}

// OHLCRow is one row of historical index level data.
type OHLCRow struct {
	IndexName string `json:"index_name"`
	Date      string `json:"date"`
	Open      string `json:"open"`
	High      string `json:"high"`
	Low       string `json:"low"`
	Close     string `json:"close"`
}

// TRIRow is one row of Total Return Index data.
type TRIRow struct {
	IndexName        string `json:"index_name"`
	Date             string `json:"date"`
	TotalReturnIndex string `json:"tri"`
	NTRValue         string `json:"ntr_value"`
}

// ValuationRow is one row of P/E, P/B, Dividend Yield data.
type ValuationRow struct {
	IndexName string `json:"index_name"`
	Date      string `json:"date"`
	PE        string `json:"pe"`
	PB        string `json:"pb"`
	DivYield  string `json:"div_yield"`
}

// ConstituentRow is one row of the per-index constituent CSV.
type ConstituentRow struct {
	CompanyName string `json:"company_name"`
	Industry    string `json:"industry"`
	Symbol      string `json:"symbol"`
	Series      string `json:"series"`
	ISIN        string `json:"isin"`
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

// cinfoBody builds the {"cinfo": "{...}"} request body the /BackPage/
// PageMethods expect: cinfo's VALUE is itself a JSON-encoded string.
func cinfoBody(indexName string, from, to time.Time) ([]byte, error) {
	inner := map[string]string{
		"name":      indexName,
		"startDate": from.Format(dateLayout),
		"endDate":   to.Format(dateLayout),
		"indexName": indexName,
	}
	innerJSON, err := json.Marshal(inner)
	if err != nil {
		return nil, fmt.Errorf("encoding cinfo inner payload: %w", err)
	}
	outer := map[string]string{"cinfo": string(innerJSON)}
	return json.Marshal(outer)
}

func (c *Client) postBackPage(ctx context.Context, method string, indexName string, from, to time.Time) ([]byte, error) {
	body, err := cinfoBody(indexName, from, to)
	if err != nil {
		return nil, err
	}
	url := baseURL + "/BackPage/" + method
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("building request for %s: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", baseURL+"/reports/historical-data")

	c.wait()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling %s: %w", method, err)
	}
	defer resp.Body.Close()
	c.onResult(resp.StatusCode)

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading %s response: %w", method, err)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, &cliutil.RateLimitError{URL: url}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s returned HTTP %d: %s", method, resp.StatusCode, truncate(string(data), 300))
	}
	return data, nil
}

// errUnknownIndexName is returned when a historical-endpoint call yields zero
// rows and the requested name does not match any currently published NSE
// index. The BackPage historical endpoints return HTTP 200 with an empty
// array for both an unknown index name and a valid name with no data in the
// requested range, so the only way to tell them apart is to cross-check
// against the live index list.
type errUnknownIndexName struct {
	IndexName string
}

func (e *errUnknownIndexName) Error() string {
	return fmt.Sprintf("index %q not found among currently published NSE indices (check spelling, e.g. \"NIFTY 50\")", e.IndexName)
}

// rejectIfUnknownIndex is called only when a historical fetch returned zero
// rows, to distinguish an unknown index name from a legitimately empty
// result for a known index (e.g. a narrow date range).
func (c *Client) rejectIfUnknownIndex(ctx context.Context, indexName string) error {
	quotes, err := c.LiveWatch(ctx)
	if err != nil {
		// Can't validate; don't mask the original empty result with a
		// secondary fetch failure.
		return nil
	}
	for _, q := range quotes {
		if strings.EqualFold(q.IndexName, indexName) {
			return nil
		}
	}
	return &errUnknownIndexName{IndexName: indexName}
}

// History fetches historical OHLC index levels for the given date range.
func (c *Client) History(ctx context.Context, indexName string, from, to time.Time) ([]OHLCRow, error) {
	raw, err := c.postBackPage(ctx, "getHistoricaldatatabletoString", indexName, from, to)
	if err != nil {
		return nil, err
	}
	var wire []struct {
		IndexName string `json:"INDEX_NAME"`
		Date      string `json:"HistoricalDate"`
		Open      string `json:"OPEN"`
		High      string `json:"HIGH"`
		Low       string `json:"LOW"`
		Close     string `json:"CLOSE"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return nil, fmt.Errorf("parsing historical response: %w", err)
	}
	if len(wire) == 0 {
		if err := c.rejectIfUnknownIndex(ctx, indexName); err != nil {
			return nil, err
		}
	}
	rows := make([]OHLCRow, 0, len(wire))
	for _, w := range wire {
		rows = append(rows, OHLCRow{IndexName: w.IndexName, Date: w.Date, Open: w.Open, High: w.High, Low: w.Low, Close: w.Close})
	}
	return rows, nil
}

// TRI fetches Total Return Index history for the given date range.
func (c *Client) TRI(ctx context.Context, indexName string, from, to time.Time) ([]TRIRow, error) {
	raw, err := c.postBackPage(ctx, "getTotalReturnIndexString", indexName, from, to)
	if err != nil {
		return nil, err
	}
	var wire []struct {
		IndexName string `json:"Index Name"`
		Date      string `json:"Date"`
		TRI       string `json:"TotalReturnsIndex"`
		NTR       string `json:"NTR_Value"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return nil, fmt.Errorf("parsing TRI response: %w", err)
	}
	if len(wire) == 0 {
		if err := c.rejectIfUnknownIndex(ctx, indexName); err != nil {
			return nil, err
		}
	}
	rows := make([]TRIRow, 0, len(wire))
	for _, w := range wire {
		rows = append(rows, TRIRow{IndexName: w.IndexName, Date: w.Date, TotalReturnIndex: w.TRI, NTRValue: w.NTR})
	}
	return rows, nil
}

// Valuation fetches P/E, P/B, Dividend Yield history for the given date range.
func (c *Client) Valuation(ctx context.Context, indexName string, from, to time.Time) ([]ValuationRow, error) {
	raw, err := c.postBackPage(ctx, "getpepbHistoricaldataDBtoString", indexName, from, to)
	if err != nil {
		return nil, err
	}
	var wire []struct {
		IndexName string `json:"Index Name"`
		Date      string `json:"DATE"`
		PE        string `json:"pe"`
		PB        string `json:"pb"`
		DivYield  string `json:"divYield"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return nil, fmt.Errorf("parsing valuation response: %w", err)
	}
	if len(wire) == 0 {
		if err := c.rejectIfUnknownIndex(ctx, indexName); err != nil {
			return nil, err
		}
	}
	rows := make([]ValuationRow, 0, len(wire))
	for _, w := range wire {
		rows = append(rows, ValuationRow{IndexName: w.IndexName, Date: w.Date, PE: w.PE, PB: w.PB, DivYield: w.DivYield})
	}
	return rows, nil
}

// Constituents fetches the constituent CSV for the given index slug (e.g. "nifty50").
func (c *Client) Constituents(ctx context.Context, slug string) ([]ConstituentRow, error) {
	url := fmt.Sprintf("%s/IndexConstituent/ind_%slist.csv", baseURL, slug)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building constituents request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	c.wait()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching constituents: %w", err)
	}
	defer resp.Body.Close()
	c.onResult(resp.StatusCode)

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, &cliutil.RateLimitError{URL: url}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("constituents fetch returned HTTP %d: %s", resp.StatusCode, truncate(string(body), 300))
	}

	// niftyindices serves an unknown slug as a soft-404: HTTP 200 with an
	// HTML error page (Content-Type text/html) instead of the octet-stream
	// CSV a real slug returns. Detect it before handing the body to the CSV
	// reader, which otherwise fails with a cryptic "bare \" in
	// non-quoted-field" that gives no hint the slug itself is wrong.
	if ct := resp.Header.Get("Content-Type"); strings.Contains(strings.ToLower(ct), "text/html") {
		return nil, fmt.Errorf("no constituent list published for slug %q (the index name may be misspelled, or this index does not publish a constituent CSV)", slug)
	}

	r := csv.NewReader(resp.Body)
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parsing constituents CSV: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("empty constituents CSV for slug %q", slug)
	}
	rows := make([]ConstituentRow, 0, len(records)-1)
	for _, rec := range records[1:] {
		if len(rec) < 5 {
			continue
		}
		rows = append(rows, ConstituentRow{
			CompanyName: rec[0],
			Industry:    rec[1],
			Symbol:      rec[2],
			Series:      rec[3],
			ISIN:        rec[4],
		})
	}
	return rows, nil
}

// SectorConstituent is one company's weight within a sector group, from the
// live sector-weight feed.
type SectorConstituent struct {
	Name   string  `json:"name"`
	Weight float64 `json:"weight"`
}

// SectorGroup is one sector's aggregate weight plus its per-company weights,
// from the live sector-weight feed.
type SectorGroup struct {
	Sector       string              `json:"sector"`
	Weight       float64             `json:"weight"`
	Constituents []SectorConstituent `json:"constituents"`
}

// sectorWeightLabelRe splits a feed label like "ABCAPITAL 2.25%" or
// "Financial Services 33.16%" into its name and percent-weight parts.
var sectorWeightLabelRe = regexp.MustCompile(`^(.*?)\s+([\d.]+)%$`)

// SectorWeights fetches the live per-constituent weight breakdown for an
// index, grouped by sector. Unlike Constituents (the /IndexConstituent/
// CSV, which carries company/industry/symbol/ISIN but no weight field),
// this feed publishes actual weights — and covers indices (e.g. strategy
// indices like NIFTY ALPHA 50) that have no published constituent CSV at
// all. The response is JSONP wrapped in a `modelDataAvailable(...)` callback
// carrying two arguments (the data object, then an unrelated metadata
// object) and uses trailing commas in its array/object literals, so it is
// parsed as JS-object-literal-ish text, not handed directly to
// encoding/json.
func (c *Client) SectorWeights(ctx context.Context, indexName string) ([]SectorGroup, error) {
	reqURL := fmt.Sprintf("https://liveindexsa.niftyindices.com/jsonfiles/Sector/SectorialIndexData%s_Sector.js", url.PathEscape(indexName))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building sector-weights request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	c.wait()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching sector weights: %w", err)
	}
	defer resp.Body.Close()
	c.onResult(resp.StatusCode)

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, &cliutil.RateLimitError{URL: reqURL}
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading sector-weights response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("sector-weights fetch returned HTTP %d: %s", resp.StatusCode, truncate(string(body), 300))
	}

	wire, err := parseSectorWeightsJSONP(body)
	if err != nil {
		return nil, fmt.Errorf("no sector-weight data published for %q: %w", indexName, err)
	}

	groups := make([]SectorGroup, 0, len(wire.Groups))
	for _, g := range wire.Groups {
		sectorName, sectorWeight := parseSectorWeightLabel(g.Label)
		constituents := make([]SectorConstituent, 0, len(g.Groups))
		for _, sub := range g.Groups {
			name, weight := parseSectorWeightLabel(sub.Label)
			constituents = append(constituents, SectorConstituent{Name: name, Weight: weight})
		}
		groups = append(groups, SectorGroup{Sector: sectorName, Weight: sectorWeight, Constituents: constituents})
	}
	return groups, nil
}

type sectorWeightWire struct {
	Groups []struct {
		Label  string `json:"label"`
		Weight any    `json:"weight"`
		Groups []struct {
			Label string `json:"label"`
		} `json:"groups"`
	} `json:"groups"`
}

// parseSectorWeightsJSONP extracts and lenient-parses the first ({...})
// argument of a `modelDataAvailable(...)` callback body: bracket-matches
// the first JSON-object argument (ignoring the second, unrelated metadata
// argument), then strips trailing commas before `]`/`}` until none remain,
// since the feed's array/object literals are JS syntax, not strict JSON.
func parseSectorWeightsJSONP(body []byte) (*sectorWeightWire, error) {
	start := bytes.IndexByte(body, '(')
	if start < 0 {
		return nil, fmt.Errorf("response is not a modelDataAvailable(...) callback")
	}
	start++

	depth := 0
	inString := false
	escaped := false
	end := -1
	for i := start; i < len(body); i++ {
		ch := body[i]
		if inString {
			switch {
			case escaped:
				escaped = false
			case ch == '\\':
				escaped = true
			case ch == '"':
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = i + 1
			}
		}
		if end != -1 {
			break
		}
	}
	if end == -1 {
		return nil, fmt.Errorf("could not find a balanced JSON object in callback body")
	}

	inner := body[start:end]
	prevLen := -1
	for len(inner) != prevLen {
		prevLen = len(inner)
		inner = trailingCommaRe.ReplaceAll(inner, []byte("$1"))
	}

	var wire sectorWeightWire
	if err := json.Unmarshal(inner, &wire); err != nil {
		return nil, fmt.Errorf("parsing sector-weight payload: %w", err)
	}
	if len(wire.Groups) == 0 {
		return nil, fmt.Errorf("empty sector-weight payload")
	}
	return &wire, nil
}

var trailingCommaRe = regexp.MustCompile(`,(\s*[\]}])`)

// parseSectorWeightLabel splits "ABCAPITAL 2.25%" into ("ABCAPITAL", 2.25).
// Returns the original label with a zero weight if it doesn't match the
// expected "<name> <percent>%" shape.
func parseSectorWeightLabel(label string) (string, float64) {
	m := sectorWeightLabelRe.FindStringSubmatch(label)
	if m == nil {
		return label, 0
	}
	var weight float64
	fmt.Sscanf(m[2], "%f", &weight)
	return m[1], weight
}

// LiveQuote is one row of the live index snapshot.
type LiveQuote struct {
	IndexName     string  `json:"indexName"`
	IndexSymbol   string  `json:"indexSymbol"`
	Last          float64 `json:"last"`
	Variation     float64 `json:"variation"`
	PercChange    float64 `json:"percChange"`
	Open          float64 `json:"open"`
	High          float64 `json:"high"`
	Low           float64 `json:"low"`
	PreviousClose float64 `json:"previousClose"`
	YearHigh      float64 `json:"yearHigh"`
	YearLow       float64 `json:"yearLow"`
	PE            string  `json:"pe"`
	PB            string  `json:"pb"`
	DivYield      string  `json:"divYield"`
	Category      string  `json:"category"`
	AsOf          string  `json:"asOf"`
}

// LiveWatch fetches the current snapshot for every published NSE index.
// Sourced from nseindia.com's api/allIndices, not niftyindices.com's own
// live-blob feed (iislliveblob.niftyindices.com/jsonfiles/LiveIndicesWatch.json):
// that feed was found to serve a stale snapshot frozen months in the past
// across every index it carries, confirmed by a direct fetch bypassing this
// client entirely. nseindia.com's allIndices reflects the actual last
// trading session and needs no special headers or session/cookie handling.
func (c *Client) LiveWatch(ctx context.Context) ([]LiveQuote, error) {
	const url = "https://www.nseindia.com/api/allIndices"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building live-watch request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	c.wait()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching live watch: %w", err)
	}
	defer resp.Body.Close()
	c.onResult(resp.StatusCode)

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, &cliutil.RateLimitError{URL: url}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("live-watch fetch returned HTTP %d: %s", resp.StatusCode, truncate(string(body), 300))
	}

	var wire struct {
		Timestamp string `json:"timestamp"`
		Data      []struct {
			Index         string  `json:"index"`
			IndexSymbol   string  `json:"indexSymbol"`
			Last          float64 `json:"last"`
			Variation     float64 `json:"variation"`
			PercentChange float64 `json:"percentChange"`
			Open          float64 `json:"open"`
			High          float64 `json:"high"`
			Low           float64 `json:"low"`
			PreviousClose float64 `json:"previousClose"`
			YearHigh      float64 `json:"yearHigh"`
			YearLow       float64 `json:"yearLow"`
			PE            string  `json:"pe"`
			PB            string  `json:"pb"`
			DY            string  `json:"dy"`
			Key           string  `json:"key"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wire); err != nil {
		return nil, fmt.Errorf("parsing live-watch response: %w", err)
	}
	quotes := make([]LiveQuote, 0, len(wire.Data))
	for _, d := range wire.Data {
		quotes = append(quotes, LiveQuote{
			IndexName:     d.Index,
			IndexSymbol:   d.IndexSymbol,
			Last:          d.Last,
			Variation:     d.Variation,
			PercChange:    d.PercentChange,
			Open:          d.Open,
			High:          d.High,
			Low:           d.Low,
			PreviousClose: d.PreviousClose,
			YearHigh:      d.YearHigh,
			YearLow:       d.YearLow,
			PE:            d.PE,
			PB:            d.PB,
			DivYield:      d.DY,
			Category:      d.Key,
			AsOf:          wire.Timestamp,
		})
	}
	return quotes, nil
}

// Slugify converts a human index name like "NIFTY 50" to the constituent
// CSV slug form "nifty50" (lowercase, spaces removed).
func Slugify(indexName string) string {
	s := strings.ToLower(indexName)
	s = strings.ReplaceAll(s, " ", "")
	return s
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
