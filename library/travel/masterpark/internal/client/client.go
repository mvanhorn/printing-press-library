// Package client implements a thin HTTP client over the MasterPark
// (netParkV2) website API used by the public reservation flow.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"regexp"
	"strings"
	"time"
)

// LocationsFallbackUsed reports whether the most recent Locations call fell
// back to built-in Lot B/G codeIDs because the live locations script could not
// be parsed. It is reset on each Locations call.
func (c *Client) LocationsFallbackUsed() bool { return c.locationsFallbackUsed }

const (
	// DefaultBaseURL is the public MasterPark website origin.
	DefaultBaseURL = "https://www.masterparking.com"

	reservationPath = "/reservation/book/"
	ajaxPath        = "/wp-content/plugins/netParkV2/ajax.php"
	locationsPath   = "/wp-content/plugins/netParkV2/locations.php"
)

// Known location codeIDs observed from the live site. Used as a stable
// fallback for the --lot B|G shorthand when locations.php is unavailable.
var knownLots = map[string]Location{
	"B": {Name: "MasterPark Lot B", CodeID: "2515-1-889"},
	"G": {Name: "MasterPark Lot G", CodeID: "2525-1-893"},
}

// Location is a parking location offered by MasterPark.
type Location struct {
	Name   string `json:"name"`
	CodeID string `json:"codeID"`
}

// Vehicle describes the vehicle for a quote/reservation.
type Vehicle struct {
	Type string `json:"type"`
}

// Reservation is the reservation payload embedded in ajax requests.
type Reservation struct {
	StartDate string        `json:"start_date"`
	EndDate   string        `json:"end_date"`
	PromoCode string        `json:"promo_code"`
	Source    string        `json:"source"`
	SourceID  string        `json:"source_id"`
	Quote     int           `json:"quote"`
	Services  []interface{} `json:"services"`
	Vehicle   Vehicle       `json:"vehicle"`
}

// QuoteRequest is the body of a getQuotes ajax call.
type QuoteRequest struct {
	Action         string      `json:"action"`
	Method         string      `json:"method"`
	Location       string      `json:"location"`
	MultiLocations interface{} `json:"multi_locations"`
	Reservation    Reservation `json:"reservation"`
	ResRate        bool        `json:"resRate"`
}

// AjaxResponse is the common envelope returned by ajax.php.
type AjaxResponse struct {
	Errors []json.RawMessage `json:"errors"`
	Data   json.RawMessage   `json:"data"`
}

// Client talks to the MasterPark website API.
type Client struct {
	BaseURL string
	HTTP    *http.Client

	nonce                 string
	locationsFallbackUsed bool
}

// New returns a Client with the given base URL and timeout.
func New(baseURL string, timeout time.Duration) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	jar, _ := cookiejar.New(nil)
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP:    &http.Client{Timeout: timeout, Jar: jar},
	}
}

func (c *Client) reservationURL() string { return c.BaseURL + reservationPath }

var nonceRe = regexp.MustCompile(`window\._wpnonce\s*=\s*['"]([^'"]+)['"]`)

// EnsureNonce fetches the reservation page and extracts window._wpnonce,
// caching it for subsequent ajax calls.
func (c *Client) EnsureNonce(ctx context.Context) (string, error) {
	if c.nonce != "" {
		return c.nonce, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.reservationURL(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch reservation page: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}
	m := nonceRe.FindSubmatch(body)
	if m == nil {
		return "", fmt.Errorf("could not locate window._wpnonce on reservation page")
	}
	c.nonce = string(m[1])
	return c.nonce, nil
}

// Locations fetches and parses locations.php. On parse failure it returns the
// known Lot B/G fallback so the shorthand stays usable.
func (c *Client) Locations(ctx context.Context) ([]Location, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+locationsPath, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Referer", c.reservationURL())
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch locations: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	c.locationsFallbackUsed = false
	locs, perr := parseLocations(body)
	if perr != nil || len(locs) == 0 {
		c.locationsFallbackUsed = true
		fmt.Fprintf(os.Stderr, "warning: could not parse locations.php (source=fallback): %v; using known Lot B/G\n", perr)
		return fallbackLots(), nil
	}
	return locs, nil
}

func parseLocations(body []byte) ([]Location, error) {
	rawValue, err := extractLocationAssignment(body)
	if err != nil {
		return nil, err
	}
	raw := normalizeJSObject(rawValue)

	// The live shape is a JS object with a single-quoted "locations" key:
	//   window.NP_PLUGIN_DATA.location = { 'locations': [ {...}, {...} ] };
	// Older/alternate shapes are a bare array or a single object. Handle all.
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err == nil {
		if locsRaw, ok := obj["locations"]; ok {
			return parseLocationArray(locsRaw)
		}
		// Single location object embedded directly.
		return parseLocationArray(raw)
	}

	// Top-level array of location objects.
	return parseLocationArray(raw)
}

// extractLocationAssignment returns the balanced object/array literal assigned
// to window.NP_PLUGIN_DATA.location. locations.php is JavaScript, not strict
// JSON, and includes nested arrays/objects; regex is too greedy for the live
// payload, so scan brackets while respecting strings.
func extractLocationAssignment(body []byte) ([]byte, error) {
	marker := []byte("NP_PLUGIN_DATA.location")
	idx := bytes.Index(body, marker)
	if idx < 0 {
		return nil, fmt.Errorf("location assignment not found")
	}
	rest := body[idx+len(marker):]
	eq := bytes.IndexByte(rest, '=')
	if eq < 0 {
		return nil, fmt.Errorf("location assignment missing equals")
	}
	rest = bytes.TrimSpace(rest[eq+1:])
	if len(rest) == 0 || (rest[0] != '{' && rest[0] != '[') {
		return nil, fmt.Errorf("location assignment is not an object or array")
	}
	open, close := rest[0], byte('}')
	if open == '[' {
		close = ']'
	}
	depth := 0
	inString := byte(0)
	escaped := false
	for i, ch := range rest {
		b := byte(ch)
		if inString != 0 {
			if escaped {
				escaped = false
				continue
			}
			if b == '\\' {
				escaped = true
				continue
			}
			if b == inString {
				inString = 0
			}
			continue
		}
		if b == '\'' || b == '"' {
			inString = b
			continue
		}
		if b == open {
			depth++
		} else if b == close {
			depth--
			if depth == 0 {
				return rest[:i+1], nil
			}
		}
	}
	return nil, fmt.Errorf("location assignment object did not terminate")
}

// normalizeJSObject converts a relaxed JS object/array literal into valid JSON.
// The live locations.php payload wraps its key in single quotes while the inner
// array uses double-quoted JSON, e.g.:
//
//	window.NP_PLUGIN_DATA.location = { 'locations': [ {"name":"...","codeID":"..."} ] };
//
// A blind global single→double quote swap would corrupt that mixed content, so
// this walks the bytes and only rewrites single-quoted JS string literals that
// sit outside of an existing double-quoted JSON string.
func normalizeJSObject(raw []byte) []byte {
	var out bytes.Buffer
	out.Grow(len(raw))
	inDouble := false
	for i := 0; i < len(raw); i++ {
		ch := raw[i]
		switch {
		case inDouble:
			out.WriteByte(ch)
			if ch == '\\' && i+1 < len(raw) {
				i++
				out.WriteByte(raw[i])
				continue
			}
			if ch == '"' {
				inDouble = false
			}
		case ch == '"':
			inDouble = true
			out.WriteByte(ch)
		case ch == '\'':
			// Consume a single-quoted JS string and emit it as a JSON
			// double-quoted string. A JS-only escaped apostrophe (\\') is
			// unescaped because it is not a valid JSON escape sequence once
			// the surrounding string becomes double-quoted.
			out.WriteByte('"')
			i++
			for i < len(raw) && raw[i] != '\'' {
				c := raw[i]
				if c == '\\' && i+1 < len(raw) {
					next := raw[i+1]
					if next == '\'' {
						out.WriteByte(next)
					} else {
						out.WriteByte(c)
						out.WriteByte(next)
					}
					i += 2
					continue
				}
				if c == '"' {
					out.WriteByte('\\')
				}
				out.WriteByte(c)
				i++
			}
			out.WriteByte('"')
		default:
			out.WriteByte(ch)
		}
	}
	return out.Bytes()
}

// parseLocationArray decodes a JSON value that is either an array of location
// objects or a single location object into a slice of Location.
func parseLocationArray(raw []byte) ([]Location, error) {
	var arr []map[string]interface{}
	if err := json.Unmarshal(raw, &arr); err != nil {
		var one map[string]interface{}
		if err2 := json.Unmarshal(raw, &one); err2 != nil {
			return nil, err
		}
		arr = []map[string]interface{}{one}
	}
	var out []Location
	for _, item := range arr {
		loc := Location{
			Name:   firstString(item, "name", "title", "label"),
			CodeID: firstString(item, "codeID", "codeId", "code_id", "id"),
		}
		if loc.Name == "" {
			if details, ok := item["details"].(map[string]interface{}); ok {
				loc.Name = firstString(details, "name", "title", "label")
			}
		}
		if loc.CodeID != "" {
			out = append(out, loc)
		}
	}
	return out, nil
}

func firstString(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

func fallbackLots() []Location {
	return []Location{knownLots["B"], knownLots["G"]}
}

// ResolveLot maps a --lot value to a codeID. It accepts the B/G shorthand
// (case-insensitive) or a raw codeID, which is returned unchanged.
func ResolveLot(lot string) (string, error) {
	lot = strings.TrimSpace(lot)
	if lot == "" {
		return "", fmt.Errorf("lot is required")
	}
	if l, ok := knownLots[strings.ToUpper(lot)]; ok {
		return l.CodeID, nil
	}
	// Assume a raw codeID like 2515-1-889.
	if strings.Contains(lot, "-") {
		return lot, nil
	}
	return "", fmt.Errorf("unknown lot %q (use B, G, or a codeID like 2515-1-889)", lot)
}

// Ajax performs a JSON POST to ajax.php with the required website headers.
func (c *Client) Ajax(ctx context.Context, payload interface{}) (*AjaxResponse, error) {
	nonce, err := c.EnsureNonce(ctx)
	if err != nil {
		return nil, err
	}
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+ajaxPath, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-CSRF-TOKEN", nonce)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Referer", c.reservationURL())
	req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	req.Header.Set("Accept", "application/json, text/plain, */*")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ajax request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ajax %s returned HTTP %d: %s", methodOf(payload), resp.StatusCode, snippet(body))
	}
	var out AjaxResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode ajax response: %w (body: %s)", err, snippet(body))
	}
	return &out, nil
}

// GetQuotes calls the getQuotes method and returns the raw quotes array.
func (c *Client) GetQuotes(ctx context.Context, req QuoteRequest) (json.RawMessage, error) {
	req.Action = "np_ajax"
	req.Method = "getQuotes"
	resp, err := c.Ajax(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("getQuotes returned errors: %s", joinRaw(resp.Errors))
	}
	return resp.Data, nil
}

func methodOf(payload interface{}) string {
	if q, ok := payload.(QuoteRequest); ok {
		return q.Method
	}
	if m, ok := payload.(map[string]interface{}); ok {
		if s, ok := m["method"].(string); ok {
			return s
		}
	}
	return "np_ajax"
}

func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 300 {
		return s[:300] + "…"
	}
	return s
}

func joinRaw(rs []json.RawMessage) string {
	parts := make([]string, 0, len(rs))
	for _, r := range rs {
		parts = append(parts, string(r))
	}
	return strings.Join(parts, ", ")
}
