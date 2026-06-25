// Hand-authored Google Play public-store client.
//
// Play serves two transports, both replayable over plain HTTP (no auth, no
// resident browser): GET HTML pages whose data lives in AF_initDataCallback
// `ds:` blocks, and a single batchexecute RPC endpoint dispatched by rpcid.
// Responses are positional protojson (anonymous nested arrays); callers index
// into them with the path helpers in parse.go.
package gplay

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-play/internal/cliutil"
)

const (
	baseURL       = "https://play.google.com"
	batchPath     = "/_/PlayStoreUi/data/batchexecute"
	defaultUA     = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	maxRetries    = 3
	playGwError   = "com.google.play.gateway.proto.PlayGatewayError"
	playDataError = "PlayDataError"
)

// Client talks to the public Play Store. Construct with New.
type Client struct {
	HTTP    *http.Client
	Limiter *cliutil.AdaptiveLimiter
	Lang    string // hl, e.g. "en"
	Country string // gl, e.g. "us"
	UA      string
}

// Options configure a Client.
type Options struct {
	Lang     string
	Country  string
	RatePerS float64       // requests/sec; <=0 disables throttling
	Timeout  time.Duration // per-request timeout when ctx has none; 0 leaves http.Client default
}

// New returns a Client with sane Play defaults.
func New(opts Options) *Client {
	lang := opts.Lang
	if lang == "" {
		lang = "en"
	}
	country := opts.Country
	if country == "" {
		country = "us"
	}
	rate := opts.RatePerS
	if rate == 0 {
		rate = 1.0 // default ~1 req/s; Play bans aggressive fan-out
	}
	hc := &http.Client{}
	if opts.Timeout > 0 {
		hc.Timeout = opts.Timeout
	}
	return &Client{
		HTTP:    hc,
		Limiter: cliutil.NewAdaptiveLimiter(rate),
		Lang:    lang,
		Country: country,
		UA:      defaultUA,
	}
}

// getHTML fetches a store HTML page and returns the body. It applies pacing,
// retries on 429/503, and surfaces a typed *cliutil.RateLimitError when
// retries are exhausted.
func (c *Client) getHTML(ctx context.Context, path string, query url.Values) (string, error) {
	if query == nil {
		query = url.Values{}
	}
	if query.Get("hl") == "" {
		query.Set("hl", c.Lang)
	}
	if query.Get("gl") == "" {
		query.Set("gl", c.Country)
	}
	full := baseURL + path + "?" + query.Encode()

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		c.Limiter.Wait()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", c.UA)
		req.Header.Set("Accept", "text/html")
		resp, err := c.HTTP.Do(req)
		if err != nil {
			lastErr = err
			if !sleepBackoff(ctx, attempt) {
				return "", err
			}
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 12<<20))
		_ = resp.Body.Close()

		switch {
		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable:
			c.Limiter.OnRateLimit()
			if attempt == maxRetries {
				return "", &cliutil.RateLimitError{
					URL:        full,
					RetryAfter: cliutil.RetryAfter(resp),
					Body:       truncateBody(string(body)),
				}
			}
			if !sleepBackoff(ctx, attempt) {
				return "", ctx.Err()
			}
			continue
		case resp.StatusCode >= 400:
			return "", fmt.Errorf("play store returned HTTP %d for %s", resp.StatusCode, path)
		}
		c.Limiter.OnSuccess()
		return string(body), nil
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("play store request failed for %s", path)
}

// batchExecute POSTs a single rpcid to the batchexecute endpoint and returns
// the decoded inner payload (the double-encoded JSON at envelope position [2]).
// envelopeTail is the 4th element of the rpc tuple ("generic", "1", or "" to
// omit). When rawBody is non-empty it is sent verbatim as the form body
// (used for the pre-encoded charts payload); otherwise inner is wrapped.
func (c *Client) batchExecute(ctx context.Context, rpcid, inner, envelopeTail, rawBody string) (json.RawMessage, error) {
	q := url.Values{}
	q.Set("rpcids", rpcid)
	q.Set("source-path", "/store/apps")
	q.Set("hl", c.Lang)
	q.Set("gl", c.Country)
	q.Set("rt", "c")
	full := baseURL + batchPath + "?" + q.Encode()

	var bodyStr string
	if rawBody != "" {
		bodyStr = "f.req=" + rawBody
	} else {
		var tuple []any
		if envelopeTail == "" {
			tuple = []any{rpcid, inner}
		} else {
			tuple = []any{rpcid, inner, nil, envelopeTail}
		}
		env := [][]any{{tuple}}
		freq, err := json.Marshal(env)
		if err != nil {
			return nil, err
		}
		bodyStr = "f.req=" + url.QueryEscape(string(freq))
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		c.Limiter.Wait()
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, full, strings.NewReader(bodyStr))
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", c.UA)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
		resp, err := c.HTTP.Do(req)
		if err != nil {
			lastErr = err
			if !sleepBackoff(ctx, attempt) {
				return nil, err
			}
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
		_ = resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			c.Limiter.OnRateLimit()
			if attempt == maxRetries {
				return nil, &cliutil.RateLimitError{URL: full, RetryAfter: cliutil.RetryAfter(resp), Body: truncateBody(string(body))}
			}
			if !sleepBackoff(ctx, attempt) {
				return nil, ctx.Err()
			}
			continue
		}
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("batchexecute %s returned HTTP %d", rpcid, resp.StatusCode)
		}
		// Rate-limit sentinel embedded in a 200 body.
		if strings.Contains(string(body), playGwError) {
			c.Limiter.OnRateLimit()
			if attempt == maxRetries {
				return nil, &cliutil.RateLimitError{URL: full, Body: "PlayGatewayError (rate limited)"}
			}
			if !sleepBackoff(ctx, attempt) {
				return nil, ctx.Err()
			}
			continue
		}
		c.Limiter.OnSuccess()
		payload, err := parseBatchExecute(body, rpcid)
		if err != nil {
			return nil, err
		}
		return payload, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("batchexecute %s failed", rpcid)
}

func sleepBackoff(ctx context.Context, attempt int) bool {
	d := cliutil.Backoff(attempt)
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func truncateBody(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 300 {
		return s[:300]
	}
	return s
}

// --- batchexecute response framing ---

// parseBatchExecute strips the )]}' prefix, walks the (possibly chunked, rt=c)
// stream of JSON values, finds the wrb.fr envelope matching rpcid, and returns
// its double-encoded inner payload decoded one level.
func parseBatchExecute(body []byte, rpcid string) (json.RawMessage, error) {
	s := strings.TrimSpace(string(body))
	s = strings.TrimPrefix(s, ")]}'")
	s = strings.TrimSpace(s)

	dec := json.NewDecoder(strings.NewReader(s))
	for dec.More() {
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			// Stop at the first undecodable token; we've likely consumed all
			// JSON values. Numbers (chunk lengths) decode fine and are skipped.
			break
		}
		t := strings.TrimSpace(string(v))
		if !strings.HasPrefix(t, "[") {
			continue // chunk-length integer or other scalar
		}
		var outer []json.RawMessage
		if err := json.Unmarshal(v, &outer); err != nil {
			continue
		}
		for _, entry := range outer {
			var row []json.RawMessage
			if err := json.Unmarshal(entry, &row); err != nil || len(row) < 3 {
				continue
			}
			var tag string
			if json.Unmarshal(row[0], &tag) != nil || tag != "wrb.fr" {
				continue
			}
			var id string
			if json.Unmarshal(row[1], &id) != nil || id != rpcid {
				continue
			}
			// row[2] is a JSON-encoded string (double-encoded payload), or null.
			if strings.TrimSpace(string(row[2])) == "null" {
				return nil, nil
			}
			var innerStr string
			if err := json.Unmarshal(row[2], &innerStr); err != nil {
				return nil, fmt.Errorf("decoding %s payload: %w", rpcid, err)
			}
			return json.RawMessage(innerStr), nil
		}
	}
	return nil, fmt.Errorf("no %s payload found in batchexecute response", rpcid)
}

// --- AF_initDataCallback extraction ---

var (
	afBlockRe = regexp.MustCompile(`AF_initDataCallback\(\{[\s\S]*?\}\);`)
	afKeyRe   = regexp.MustCompile(`key:\s*'(ds:\d+)'`)
	afDataRe  = regexp.MustCompile(`data:([\s\S]*?), sideChannel:`)
)

// extractAFData parses every AF_initDataCallback block in an HTML page into a
// map keyed by ds: id. Values are the raw `data:` arrays.
func extractAFData(html string) (map[string]json.RawMessage, error) {
	out := map[string]json.RawMessage{}
	for _, block := range afBlockRe.FindAllString(html, -1) {
		km := afKeyRe.FindStringSubmatch(block)
		dm := afDataRe.FindStringSubmatch(block)
		if km == nil || dm == nil {
			continue
		}
		raw := strings.TrimSpace(dm[1])
		if !json.Valid([]byte(raw)) {
			continue
		}
		out[km[1]] = json.RawMessage(raw)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no AF_initDataCallback data found")
	}
	return out, nil
}
