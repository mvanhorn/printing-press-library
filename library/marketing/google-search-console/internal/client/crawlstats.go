// PATCH(crawl-stats: HTTP client for the SearchConsoleAggReportUi.batchexecute
// endpoint that powers the GSC Crawl Stats UI). The public Search Console v1
// API does NOT expose the Crawl Stats report's sample URLs. This client wires
// the three rpcids — nDAfwb (URL samples), OLiH4d (time-series), czrWJf
// (aggregate stats) — discovered by chrome-MCP capture against the user's
// authenticated GSC session on 2026-05-21. See the discovery report at
// manuscripts/google-search-console/amend-2026-05-21T1402/discovery/.
//
// Endpoint: POST https://search.google.com/_/SearchConsoleAggReportUi/data/batchexecute
//
// This client uses cookie-based auth (SAPISIDHASH + 7 session cookies), not
// the OAuth bearer token used by the rest of this CLI. The two auth surfaces
// coexist; the OAuth flow is unaffected by this patch.

package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	// crawlStatsEndpoint is the private batchexecute endpoint discovered via
	// chrome-MCP capture against the GSC Crawl Stats UI on 2026-05-21.
	crawlStatsEndpoint = "https://search.google.com/_/SearchConsoleAggReportUi/data/batchexecute"

	// crawlStatsOrigin is the origin used as the third component of the
	// SAPISIDHASH payload. Must match the Origin/Referer the GSC web UI
	// would send.
	crawlStatsOrigin = "https://search.google.com"

	// defaultBuildLabel is a recent GSC build label observed during chrome-MCP
	// capture. The build label rotates whenever Google deploys GSC; the CLI
	// accepts an override via --build-label, and v0.3 will auto-scrape this
	// value from the GSC HTML at first call. Hardcoded fallback is safe
	// because Google appears to accept stale build labels (request still
	// succeeds; only the response renderer differs).
	defaultBuildLabel = "boq_searchconsoleserver_20260520.03_p0"
)

// Drill-down report kinds (the second element of the inner rpc jsonArgs).
const (
	reportKindURLSamples = 69
	reportKindTimeSeries = 49
	reportKindAggregate  = 35
)

// rpcDef is one rpc entry in the outer batchexecute payload.
type rpcDef struct {
	rpcID      string
	reportKind int
	index      string // string-encoded request index
}

// canonicalRPCSet is the three-rpc set Crawl Stats fires for each drill-down
// view (one URL-sample list, one time-series, one aggregate stats block).
var canonicalRPCSet = []rpcDef{
	{rpcID: "nDAfwb", reportKind: reportKindURLSamples, index: "1"},
	{rpcID: "OLiH4d", reportKind: reportKindTimeSeries, index: "2"},
	{rpcID: "czrWJf", reportKind: reportKindAggregate, index: "8"},
}

// Dimension is a filter dimension for crawl-stats drill-downs.
type Dimension string

const (
	DimensionNone          Dimension = ""
	DimensionFileType      Dimension = "file_type"
	DimensionResponseCode  Dimension = "response"
	DimensionGooglebotType Dimension = "googlebot_type"
	DimensionPurpose       Dimension = "purpose"
)

// CrawlStatsRequest carries the inputs for a single batchexecute call.
type CrawlStatsRequest struct {
	Property   string    // e.g. "sc-domain:example.com"
	Dimension  Dimension // empty for the no-drill-down overview
	FilterCode int       // integer code from the discovery report (file_type 1-9, etc.)
	XSRFToken  string    // `at=` body field — extracted from GSC HTML on first call
	BuildLabel string    // bl= query param; empty uses defaultBuildLabel
	SessionID  string    // f.sid= query param; empty omits
	RequestSeq int       // _reqid= query param; monotonic counter; 0 -> use 1
}

// CrawlStatsResponse is the decoded payload from one canonical-three-rpc set.
type CrawlStatsResponse struct {
	Property     string                     `json:"property"`
	Dimension    Dimension                  `json:"filter_dimension,omitempty"`
	FilterCode   int                        `json:"filter_code,omitempty"`
	CapturedAt   time.Time                  `json:"captured_at"`
	Totals       *CrawlStatsTotals          `json:"totals,omitempty"`
	TimeSeries   []CrawlStatsTimePoint      `json:"time_series,omitempty"`
	Samples      []CrawlStatsSample         `json:"samples,omitempty"`
	RawResponses map[string]json.RawMessage `json:"raw,omitempty"` // present only when --raw set
}

// CrawlStatsTotals are the aggregate stats (rpcid czrWJf, report_kind 35).
type CrawlStatsTotals struct {
	CrawlRequests     int64 `json:"crawl_requests"`
	DownloadSizeBytes int64 `json:"download_size_bytes"`
	AvgResponseMs     int64 `json:"avg_response_ms"`
}

// CrawlStatsTimePoint is one point on the time-series line graph.
type CrawlStatsTimePoint struct {
	Date          string `json:"date"`
	CrawlRequests int64  `json:"crawl_requests"`
}

// CrawlStatsSample is one URL sample from rpcid nDAfwb.
type CrawlStatsSample struct {
	URL          string    `json:"url"`
	FetchedAt    time.Time `json:"fetched_at,omitempty"`
	ResponseCode int       `json:"response_code,omitempty"`
	SizeBytes    int64     `json:"size_bytes,omitempty"`
	ResponseMs   int       `json:"response_ms,omitempty"`
}

// CrawlStatsClient calls the private batchexecute endpoint. It does NOT share
// the cache/limiter of the main Client because the cache key shape and rate
// limits are different (the batchexecute endpoint is not rate-limited the
// same way the public API is).
type CrawlStatsClient struct {
	HTTPClient *http.Client
	Jar        *CookieJarFile
	BuildLabel string
	DryRun     bool

	// AuthUser is the X-Goog-AuthUser header value. Defaults to "0" (the
	// primary signed-in Google account); override if the user has multi-login
	// and wants a non-primary account.
	AuthUser string

	// nowFunc is overridable for tests; production callers leave it nil.
	nowFunc func() time.Time
}

// NewCrawlStatsClient builds a client around an already-loaded cookie jar.
// Returns an error when the jar is missing any required cookie.
func NewCrawlStatsClient(jar *CookieJarFile, timeout time.Duration) (*CrawlStatsClient, error) {
	if jar == nil {
		return nil, fmt.Errorf("crawl-stats: cookie jar is nil — set GSC_COOKIE_JAR or run `auth login --chrome`")
	}
	if missing := jar.MissingCookies(); len(missing) > 0 {
		return nil, fmt.Errorf("crawl-stats: cookie jar at %s is missing %d Google session cookie(s): %s — re-export your jar (see README.md crawl-stats section)", jar.Path, len(missing), strings.Join(missing, ", "))
	}
	return &CrawlStatsClient{
		HTTPClient: &http.Client{Timeout: timeout},
		Jar:        jar,
		BuildLabel: defaultBuildLabel,
		AuthUser:   "0",
	}, nil
}

// now returns the current time, honoring nowFunc when set (for tests).
func (c *CrawlStatsClient) now() time.Time {
	if c.nowFunc != nil {
		return c.nowFunc()
	}
	return time.Now().UTC()
}

// Fetch dispatches one canonical-three-rpc batchexecute call and parses the
// response into a CrawlStatsResponse. When DryRun is true, builds the request
// up to the wire and returns a synthetic response with the request body in
// RawResponses["dry_run_body"].
func (c *CrawlStatsClient) Fetch(ctx context.Context, req CrawlStatsRequest) (*CrawlStatsResponse, error) {
	if req.Property == "" {
		return nil, fmt.Errorf("crawl-stats: Property required (e.g. sc-domain:example.com)")
	}
	if req.XSRFToken == "" {
		return nil, fmt.Errorf("crawl-stats: XSRFToken required (read from the `at=` form field in the GSC HTML; pass via --xsrf-token or GSC_XSRF_TOKEN)")
	}

	buildLabel := req.BuildLabel
	if buildLabel == "" {
		buildLabel = c.BuildLabel
	}
	if buildLabel == "" {
		buildLabel = defaultBuildLabel
	}

	// Compose the f.req payload — see discovery report for the shape.
	fReq, err := encodeBatchExecuteFReq(req.Property, req.Dimension, req.FilterCode)
	if err != nil {
		return nil, err
	}

	body := url.Values{}
	body.Set("f.req", fReq)
	body.Set("at", req.XSRFToken)
	bodyEncoded := body.Encode()

	// Compose the query string.
	q := url.Values{}
	q.Set("rpcids", "nDAfwb,OLiH4d,czrWJf")
	q.Set("source-path", "/search-console/settings/crawl-stats")
	if req.SessionID != "" {
		q.Set("f.sid", req.SessionID)
	}
	q.Set("bl", buildLabel)
	q.Set("hl", "en")
	reqid := req.RequestSeq
	if reqid <= 0 {
		reqid = 1
	}
	q.Set("_reqid", strconv.Itoa(reqid))
	q.Set("rt", "c")

	endpoint := crawlStatsEndpoint + "?" + q.Encode()

	if c.DryRun {
		return &CrawlStatsResponse{
			Property:   req.Property,
			Dimension:  req.Dimension,
			FilterCode: req.FilterCode,
			CapturedAt: c.now(),
			RawResponses: map[string]json.RawMessage{
				"dry_run_endpoint": json.RawMessage(`"` + endpoint + `"`),
				"dry_run_body":     json.RawMessage(`"` + jsonEscape(bodyEncoded) + `"`),
			},
		}, nil
	}

	hr, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(bodyEncoded))
	if err != nil {
		return nil, fmt.Errorf("crawl-stats: building request: %w", err)
	}
	hr.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	hr.Header.Set("Cookie", c.Jar.CookieHeader())
	hr.Header.Set("Authorization", SAPISIDHash(c.Jar.SAPISIDValue(), crawlStatsOrigin, c.now()))
	hr.Header.Set("X-Goog-AuthUser", c.AuthUser)
	hr.Header.Set("X-Same-Domain", "1")
	hr.Header.Set("Origin", crawlStatsOrigin)
	hr.Header.Set("Referer", crawlStatsOrigin+"/search-console/settings/crawl-stats")
	hr.Header.Set("User-Agent", "google-search-console-pp-cli/1.0.0 (+crawl-stats)")

	resp, err := c.HTTPClient.Do(hr)
	if err != nil {
		return nil, fmt.Errorf("crawl-stats: POST %s: %w", crawlStatsEndpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("crawl-stats: HTTP %d — cookies likely stale; re-export your cookie jar (try `google-search-console-pp-cli auth login --chrome` for instructions)", resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("crawl-stats: HTTP %d: %s", resp.StatusCode, string(raw))
	}

	parsed, raw, err := decodeBatchExecuteChunks(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("crawl-stats: decoding response chunks: %w", err)
	}

	out := &CrawlStatsResponse{
		Property:     req.Property,
		Dimension:    req.Dimension,
		FilterCode:   req.FilterCode,
		CapturedAt:   c.now(),
		RawResponses: raw,
	}
	if samples, ok := parsed["nDAfwb"]; ok {
		out.Samples = extractSamples(samples)
	}
	if ts, ok := parsed["OLiH4d"]; ok {
		out.TimeSeries = extractTimeSeries(ts)
	}
	if agg, ok := parsed["czrWJf"]; ok {
		out.Totals = extractTotals(agg)
	}
	return out, nil
}

// encodeBatchExecuteFReq builds the f.req body field. The outer shape is:
//
//	[[
//	  ["nDAfwb", "[<property>, 69, <filter_envelope>]", null, "1"],
//	  ["OLiH4d", "[<property>, 49, <filter_envelope>]", null, "2"],
//	  ["czrWJf", "[<property>, 35, <filter_value_or_default>]", null, "8"]
//	]]
//
// The filter_envelope for nDAfwb/OLiH4d is
// `[[22,""],[23,null,null,null,null,null,null,null,<filter_code>]]`; the
// czrWJf rpc takes a bare integer instead of the envelope (see discovery
// report).
func encodeBatchExecuteFReq(property string, dim Dimension, filterCode int) (string, error) {
	// Build filter envelope for nDAfwb / OLiH4d (URL samples + time series).
	// Index 8 of the inner [23,...] array holds the filter value. When
	// dim==DimensionNone we still send the envelope with a null at index 8
	// (matches the GSC web UI's overview call shape).
	var filterIndex8 any
	if dim != DimensionNone {
		filterIndex8 = filterCode
	}
	envelope := []any{
		[]any{22, ""},
		[]any{23, nil, nil, nil, nil, nil, nil, nil, filterIndex8},
	}

	rpcs := make([]any, 0, 3)
	for _, rpc := range canonicalRPCSet {
		var argsAny any
		if rpc.rpcID == "czrWJf" {
			// Aggregate rpc takes (property, report_kind, filter_code_or_2)
			// per the discovery report. When no drill-down dimension, the
			// observed value at this position is 2.
			third := 2
			if dim != DimensionNone {
				third = filterCode
			}
			argsAny = []any{property, rpc.reportKind, third}
		} else {
			argsAny = []any{property, rpc.reportKind, envelope}
		}
		argsJSON, err := json.Marshal(argsAny)
		if err != nil {
			return "", err
		}
		rpcs = append(rpcs, []any{rpc.rpcID, string(argsJSON), nil, rpc.index})
	}
	outer := []any{rpcs}
	b, err := json.Marshal(outer)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// decodeBatchExecuteChunks parses the chunked response. Each chunk is
// `<decimal_length>\n<json_array>\n`, where the json_array carries one
// `["wrb.fr","<rpcId>","<json_string_of_payload>",null,null,"<requestIndex>"]`
// entry. The `<json_string_of_payload>` itself is a JSON-encoded string and
// must be parsed twice to reach the actual payload.
//
// Returns: rpcID -> decoded inner payload (the second parse) and rpcID -> the
// raw inner string (preserved for --raw output mode).
func decodeBatchExecuteChunks(body io.Reader) (map[string]json.RawMessage, map[string]json.RawMessage, error) {
	scan := bufio.NewReader(body)
	parsed := make(map[string]json.RawMessage)
	raw := make(map[string]json.RawMessage)

	// Strip the )]}'  XSSI prefix.
	prefix := make([]byte, 4)
	if _, err := io.ReadFull(scan, prefix); err != nil {
		return nil, nil, fmt.Errorf("reading XSSI prefix: %w", err)
	}
	if !bytes.Equal(prefix, []byte(")]}'")) {
		return nil, nil, fmt.Errorf("missing )]}'  XSSI prefix (got %q)", string(prefix))
	}
	// Drain to next newline after the prefix.
	if _, err := scan.ReadString('\n'); err != nil && err != io.EOF {
		return nil, nil, err
	}

	for {
		lenLine, err := scan.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		lenLine = strings.TrimSpace(lenLine)
		if lenLine == "" {
			continue
		}
		n, err := strconv.Atoi(lenLine)
		if err != nil {
			// Some chunks emit a trailing checksum / empty marker;
			// non-decimal line ends the stream.
			break
		}
		buf := make([]byte, n)
		if _, err := io.ReadFull(scan, buf); err != nil {
			return nil, nil, fmt.Errorf("reading chunk body: %w", err)
		}

		// Parse the outer wrapper array.
		var outer [][]any
		if err := json.Unmarshal(buf, &outer); err != nil {
			// Some chunks are control frames (e.g. ["di",...]); skip silently.
			continue
		}
		for _, row := range outer {
			if len(row) < 3 {
				continue
			}
			tag, _ := row[0].(string)
			if tag != "wrb.fr" {
				continue
			}
			rpcID, _ := row[1].(string)
			innerStr, _ := row[2].(string)
			raw[rpcID] = json.RawMessage(strconvQuote(innerStr))
			var inner json.RawMessage
			if innerStr != "" {
				if err := json.Unmarshal([]byte(innerStr), &inner); err == nil {
					parsed[rpcID] = inner
				}
			}
		}
	}
	return parsed, raw, nil
}

// extractSamples walks the nDAfwb payload structure to pull (URL, optional
// metadata) per sample. The full URL position within the 43-element inner
// array varies; this implementation reads element 0 as the URL (most common
// position observed in chrome-MCP capture) and best-efforts the rest.
//
// The walker is defensive: never panics on unexpected shape, returns the
// samples it could decode and skips malformed entries.
func extractSamples(payload json.RawMessage) []CrawlStatsSample {
	var top []json.RawMessage
	if err := json.Unmarshal(payload, &top); err != nil {
		return nil
	}
	// The nDAfwb payload shape from the discovery report:
	//   [<echo of request args>, [<N samples>], ...]
	// We want index 1 — the primary URL list.
	if len(top) < 2 {
		return nil
	}
	var samples []json.RawMessage
	if err := json.Unmarshal(top[1], &samples); err != nil {
		return nil
	}
	out := make([]CrawlStatsSample, 0, len(samples))
	for _, s := range samples {
		var sampleOuter []json.RawMessage
		if err := json.Unmarshal(s, &sampleOuter); err != nil {
			continue
		}
		if len(sampleOuter) == 0 {
			continue
		}
		// First element is the 43-element sparse array.
		var inner []json.RawMessage
		if err := json.Unmarshal(sampleOuter[0], &inner); err != nil {
			continue
		}
		if len(inner) == 0 {
			continue
		}
		var rawURL string
		if err := json.Unmarshal(inner[0], &rawURL); err == nil && rawURL != "" {
			// PATCH(crawl-stats): preserve metadata from the sparse GSC sample array.
			// The private UI payload has shifted field positions across captures, so keep
			// URL decoding strict while best-effort extracting the remaining scalar values.
			out = append(out, crawlStatsSampleFromInner(rawURL, inner[1:]))
		}
	}
	return out
}

func crawlStatsSampleFromInner(rawURL string, fields []json.RawMessage) CrawlStatsSample {
	sample := CrawlStatsSample{URL: rawURL}
	for _, field := range fields {
		if sample.FetchedAt.IsZero() {
			if t, ok := crawlStatsTime(field); ok {
				sample.FetchedAt = t
				continue
			}
		}

		if n, ok := crawlStatsInt64(field); ok {
			switch {
			case sample.ResponseCode == 0 && n >= 100 && n <= 599:
				sample.ResponseCode = int(n)
			case sample.SizeBytes == 0 && n > 0:
				sample.SizeBytes = n
			case sample.ResponseMs == 0 && n > 0 && n <= 600000:
				sample.ResponseMs = int(n)
			}
		}
	}
	return sample
}

func crawlStatsInt64(raw json.RawMessage) (int64, bool) {
	var n int64
	if err := json.Unmarshal(raw, &n); err == nil {
		return n, true
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return int64(f), true
	}
	return 0, false
}

func crawlStatsTime(raw json.RawMessage) (time.Time, bool) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil && s != "" {
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
			if t, err := time.Parse(layout, s); err == nil {
				return t.UTC(), true
			}
		}
	}
	if n, ok := crawlStatsInt64(raw); ok {
		// Treat large UI timestamps as milliseconds since epoch; smaller values are
		// left for response-code/size/latency extraction.
		if n > 946684800000 {
			return time.UnixMilli(n).UTC(), true
		}
	}
	return time.Time{}, false
}

// extractTimeSeries walks the OLiH4d payload. The series is a list of
// [date_string, crawl_requests] pairs in the inner array. Defensive walking
// matches extractSamples — unknown shapes return an empty slice.
func extractTimeSeries(payload json.RawMessage) []CrawlStatsTimePoint {
	var top []json.RawMessage
	if err := json.Unmarshal(payload, &top); err != nil {
		return nil
	}
	if len(top) < 2 {
		return nil
	}
	var series []json.RawMessage
	if err := json.Unmarshal(top[1], &series); err != nil {
		return nil
	}
	out := make([]CrawlStatsTimePoint, 0, len(series))
	for _, p := range series {
		var pair []json.RawMessage
		if err := json.Unmarshal(p, &pair); err != nil {
			continue
		}
		if len(pair) < 2 {
			continue
		}
		var date string
		var requests int64
		_ = json.Unmarshal(pair[0], &date)
		_ = json.Unmarshal(pair[1], &requests)
		if date != "" {
			out = append(out, CrawlStatsTimePoint{Date: date, CrawlRequests: requests})
		}
	}
	return out
}

// extractTotals walks the czrWJf payload. The aggregate stats live at
// indices 0, 1, 2 of the inner array (crawl_requests, download_size_bytes,
// avg_response_ms) per chrome-MCP capture.
func extractTotals(payload json.RawMessage) *CrawlStatsTotals {
	var top []json.RawMessage
	if err := json.Unmarshal(payload, &top); err != nil {
		return nil
	}
	if len(top) < 3 {
		return nil
	}
	out := &CrawlStatsTotals{}
	_ = json.Unmarshal(top[0], &out.CrawlRequests)
	_ = json.Unmarshal(top[1], &out.DownloadSizeBytes)
	_ = json.Unmarshal(top[2], &out.AvgResponseMs)
	return out
}

// strconvQuote wraps a Go string in JSON-quoted form so it can be stored
// as a json.RawMessage for round-trippable --raw output.
func strconvQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// jsonEscape returns the JSON-string-escaped form of s, without surrounding
// quotes — used for synthesizing dry-run output structures.
func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	if len(b) >= 2 {
		return string(b[1 : len(b)-1])
	}
	return s
}
