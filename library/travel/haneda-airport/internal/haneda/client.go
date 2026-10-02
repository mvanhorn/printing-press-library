package haneda

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/haneda-airport/internal/cliutil"
)

const maxBody = int64(4 << 20)
const maxTotal = int64(16 << 20)
const maxRequests = 20

type Client struct {
	HTTP     *http.Client
	Origin   string
	limiter  *cliutil.AdaptiveLimiter
	budget   Budget
	airports map[string][]Airport
	airlines map[string][]Airline
}

// NewClient creates a command-scoped bounded client. A zero rate explicitly disables pacing.
func NewClient(origin string, timeout time.Duration, rate float64) (*Client, error) {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("base URL must be an HTTP(S) origin without credentials, query or fragment")
	}
	if timeout <= 0 || timeout > 30*time.Second {
		timeout = 30 * time.Second
	}
	if rate < 0 || rate > 2 {
		rate = 2
	}
	c := &Client{Origin: strings.TrimRight(origin, "/"), limiter: cliutil.NewAdaptiveLimiter(rate), budget: Budget{MaxRequests: maxRequests, MaxBodyBytes: maxBody, MaxTotalBytes: maxTotal}}
	c.HTTP = &http.Client{Timeout: timeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	return c, nil
}

func (c *Client) Budget() Budget { return c.budget }

func (c *Client) request(ctx context.Context, path string, payload any, out any) error {
	if c.budget.Requests >= maxRequests {
		return fmt.Errorf("source request budget exhausted (%d requests)", maxRequests)
	}
	var body []byte
	var err error
	method := http.MethodGet
	if payload != nil {
		method = http.MethodPost
		body, err = json.Marshal(payload)
		if err != nil {
			return err
		}
	}
	if err = c.limiter.Wait(ctx); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Origin+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "haneda-airport-cli/0.1 (+https://www.tokyo-haneda.com/en/flight/index.html)")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.budget.Requests++
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		c.limiter.OnRateLimit()
		return &cliutil.RateLimitError{URL: c.Origin + path, RetryAfter: cliutil.RetryAfter(resp)}
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("source HTTP %d for %s; consult the canonical airport page", resp.StatusCode, path)
	}
	c.limiter.OnSuccess()
	capBytes := maxBody
	if remaining := maxTotal - c.budget.ResponseBytes; remaining < capBytes {
		capBytes = remaining
	}
	if capBytes < 0 {
		capBytes = 0
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, capBytes+1))
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	c.budget.ResponseBytes += int64(len(data))
	if int64(len(data)) > maxBody || c.budget.ResponseBytes > maxTotal {
		return fmt.Errorf("source response exceeded the 4 MiB body or 16 MiB command budget")
	}
	if !json.Valid(data) {
		return fmt.Errorf("source returned non-JSON for %s; the public contract may have changed", path)
	}
	if err = json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func (c *Client) fetchBoard(ctx context.Context, kind, direction, date, flight string) (RawBoard, error) {
	p := map[string]any{"flightType": kindCode(kind), "searchDt": strings.ReplaceAll(date, "-", ""), "airportCodes": []string{}, "airlineCodes": []string{}, "flightNumber": "", "status": []int{0}}
	if flight != "" {
		p["flightNumber"] = flight
		p["exactMatch"] = true
		p["status"] = []int{}
	} else {
		p["arrivalType"] = directionCode(direction)
	}
	var b RawBoard
	if err := c.request(ctx, "/en/app/api/v2/flight/search", p, &b); err != nil {
		return b, err
	}
	if b.Count == nil || b.Flights == nil || *b.Count != len(b.Flights) {
		return b, fmt.Errorf("flight source has missing/inconsistent count or flightlists; completeness cannot be established")
	}
	if len(b.Flights) > 10000 {
		return b, fmt.Errorf("flight source exceeds 10000 records")
	}
	return b, nil
}

// FetchBoard reads a full scoped source board before applying local output/filter bounds.
func (c *Client) FetchBoard(ctx context.Context, q Query, now time.Time, exactFlight string) (BoardResult, error) {
	r := BoardResult{ObservedAt: now.In(JST).Format(time.RFC3339), Coverage: Coverage{Kind: q.Kind, Direction: q.Direction, RequestedDate: q.Date, Origin: c.Origin, QueryMode: "board"}, Sources: []SourceInfo{}, Flights: []Flight{}, MaxScanRecords: q.MaxScan, Notes: boardNotes()}
	if exactFlight != "" {
		r.Coverage.QueryMode = "exact_primary_lookup"
		r.Coverage.FlightLookup = exactFlight
	}
	if err := ValidateQuery(q, now, true); err != nil {
		return r, err
	}
	seen := map[string]bool{}
	for _, kind := range kinds(q.Kind) {
		airports, airlines, err := c.catalogs(ctx, kind)
		if err != nil {
			return r, err
		}
		dirs := directions(q.Direction)
		if exactFlight != "" {
			dirs = []string{"both"}
		}
		for _, direction := range dirs {
			b, err := c.fetchBoard(ctx, kind, direction, q.Date, exactFlight)
			if err != nil {
				return r, err
			}
			stamp := parseTimestamp(b.Date.Date)
			r.Sources = append(r.Sources, SourceInfo{Kind: kind, Direction: direction, URL: Origin + "/en/app/api/v2/flight/search", ReportedAt: stamp, SourceTotal: *b.Count, TimestampSemantics: "response-reported time; per-flight update and actual-time semantics are unpublished"})
			for _, raw := range b.Flights {
				if raw.Kind != kind || (direction != "both" && raw.Direction != direction) {
					return r, fmt.Errorf("source board contains an unexpected kind/direction")
				}
				if r.ScannedRecords >= q.MaxScan {
					r.ScanCapHit = true
					continue
				}
				r.ScannedRecords++
				f, err := normalizeFlight(raw, airports, airlines, stamp)
				if err != nil {
					return r, err
				}
				if seen[f.ID] {
					return r, fmt.Errorf("source contains duplicate stable flight identity %s", f.ID)
				}
				seen[f.ID] = true
				r.Flights = append(r.Flights, f)
			}
		}
	}
	r.Complete = !r.ScanCapHit
	r.Budget = c.Budget()
	return r, nil
}

// FetchSummary returns the provider's disruption summary without treating absent alerts as punctuality.
func (c *Client) FetchSummary(ctx context.Context) (map[string]any, error) {
	var d map[string]any
	if err := c.request(ctx, "/en/app_resource/flight/flightStatus/flight_status.json", nil, &d); err != nil {
		return nil, err
	}
	for _, k := range []string{"gettingAt", "domesticTotalCount", "internationalTotalCount", "domesticFlights", "internationalFlights"} {
		if _, ok := d[k]; !ok {
			return nil, fmt.Errorf("disruption source is missing %s", k)
		}
	}
	return d, nil
}

func kindCode(k string) int {
	if k == "domestic" {
		return 1
	}
	return 2
}
func directionCode(k string) int {
	if k == "departure" {
		return 1
	}
	return 2
}
func kinds(k string) []string {
	if k == "all" {
		return []string{"domestic", "international"}
	}
	return []string{k}
}
func directions(d string) []string {
	if d == "both" {
		return []string{"departure", "arrival"}
	}
	return []string{d}
}
