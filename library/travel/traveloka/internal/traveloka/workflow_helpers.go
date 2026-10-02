package traveloka

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	airportSearchPath     = "/api/v2/airport/search-nexus"
	hotelAutocompletePath = "/api/v1/hotel/autocomplete"
	flightInitialPath     = "/api/v2/flight/search/initial"
	flightPollPath        = "/api/v2/flight/search/poll"
	flightPrefetchPath    = "/api/v2/flight/search/redirection"
	hotelSearchPath       = "/api/v2/hotel/searchList"
	hotelRoomsPath        = "/api/v2/hotel/search/rooms"
)

func sourceObject(v any) map[string]any { m, _ := v.(map[string]any); return m }
func sourceList(v any) []any            { a, _ := v.([]any); return a }
func sourceString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if s, err := integerString(v); err == nil {
		return s
	}
	return ""
}
func sourceInt(v any) *int {
	s, err := integerString(v)
	if err != nil {
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return &n
}
func sourceBool(v any) *bool {
	b, ok := v.(bool)
	if !ok {
		return nil
	}
	return &b
}
func firstString(values ...any) string {
	for _, v := range values {
		if s := sourceString(v); s != "" {
			return s
		}
	}
	return ""
}
func sourceFields(m map[string]any, keys ...string) map[string]any {
	r := map[string]any{}
	for _, k := range keys {
		if v, ok := m[k]; ok {
			r[k] = v
		}
	}
	return r
}
func freshID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", apiError("UPSTREAM_ERROR", "cannot create a fresh search identifier", 0, false)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	s := hex.EncodeToString(b)
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:], nil
}
func newQuoteSnapshot(q Query) (*Snapshot, error) {
	id, err := freshID()
	if err != nil {
		return nil, err
	}
	return &Snapshot{ID: id, Kind: q.Kind, RetrievedAt: time.Now().UTC().Format(time.RFC3339Nano), Query: q, Offers: []Offer{}, Coverage: map[string]any{}, Warnings: []string{}, Indicative: true, Freshness: "fresh_retrieval"}, nil
}
func boundedLimit(n, fallback, maximum int) int {
	if n <= 0 {
		return fallback
	}
	if n > maximum {
		return maximum
	}
	return n
}

// ResponseData applies the source success, status and error-envelope policy
// shared by search workflows and authentication validation.
func ResponseData(response map[string]any) (map[string]any, error) {
	return responseData(response)
}

func responseData(response map[string]any) (map[string]any, error) {
	for _, object := range []map[string]any{response, sourceObject(response["data"])} {
		status := strings.ToUpper(sourceString(object["status"]))
		if b, ok := object["success"].(bool); ok && !b {
			return nil, apiError("UPSTREAM_ERROR", "Traveloka returned an unsuccessful response", 200, false)
		}
		switch status {
		case "", "SUCCESS", "OK", "COMPLETED", "COMPLETE", "NO_INVENTORY", "NO_RESULTS":
		default:
			return nil, apiError("UPSTREAM_ERROR", "Traveloka returned source status "+status, 200, false)
		}
		if v, ok := object["error"]; ok && sourceErrorPresent(v) {
			return nil, apiError("UPSTREAM_ERROR", "Traveloka returned a source error", 200, false)
		}
	}
	d := sourceObject(response["data"])
	if d == nil {
		return nil, apiError("MALFORMED_RESPONSE", "Traveloka response has no data object", 200, false)
	}
	return d, nil
}
func sourceErrorPresent(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return x != ""
	case map[string]any:
		return len(x) > 0
	case []any:
		return len(x) > 0
	default:
		return true
	}
}
func checkedMoney(v any, scale any, currency string) (*Money, error) {
	m, err := ParseMoney(v, scale)
	if err != nil {
		return nil, apiError("MALFORMED_RESPONSE", "invalid source money: "+err.Error(), 200, false)
	}
	if m != nil && m.Currency != "" && m.Currency != currency {
		return nil, apiError("CURRENCY_MISMATCH", "source quote currency differs from requested currency", 200, false)
	}
	return m, nil
}
func displayMoney(v any, currency string) (*Money, error) {
	o := sourceObject(v)
	if o == nil {
		if v == nil {
			return nil, nil
		}
		return nil, apiError("MALFORMED_RESPONSE", "source display money must be an object", 200, false)
	}
	if wrapped, ok := o["currencyValue"]; ok {
		return checkedMoney(wrapped, o["numOfDecimalPoint"], currency)
	}
	return checkedMoney(o, o["numOfDecimalPoint"], currency)
}
func sourceDate(v any) string {
	if s, ok := v.(string); ok {
		if d, err := time.Parse("2006-01-02", s); err == nil && d.Format("2006-01-02") == s {
			return s
		}
		return ""
	}
	o := sourceObject(v)
	y, m, d := sourceInt(o["year"]), sourceInt(o["month"]), sourceInt(o["day"])
	if y == nil || m == nil || d == nil {
		return ""
	}
	s := fmt.Sprintf("%04d-%02d-%02d", *y, *m, *d)
	if t, err := time.Parse("2006-01-02", s); err == nil && t.Format("2006-01-02") == s {
		return s
	}
	return ""
}
func sourceTime(v any) string {
	if s, ok := v.(string); ok {
		for _, layout := range []string{"15:04", "15:04:05"} {
			if t, e := time.Parse(layout, s); e == nil && t.Format(layout) == s {
				return s
			}
		}
		return ""
	}
	o := sourceObject(v)
	h, m := sourceInt(o["hour"]), sourceInt(o["minute"])
	if h == nil || m == nil || *h < 0 || *h > 23 || *m < 0 || *m > 59 {
		return ""
	}
	return fmt.Sprintf("%02d:%02d", *h, *m)
}
func dateObject(value string) map[string]any {
	d, _ := time.Parse("2006-01-02", value)
	return map[string]any{"year": strconv.Itoa(d.Year()), "month": strconv.Itoa(int(d.Month())), "day": strconv.Itoa(d.Day())}
}
func queryNights(q Query) int {
	in, _ := time.Parse("2006-01-02", q.CheckIn)
	out, _ := time.Parse("2006-01-02", q.CheckOut)
	return int(out.Sub(in) / (24 * time.Hour))
}
func waitSourceRefresh(ctx context.Context, meta map[string]any) error {
	delay := 300 * time.Millisecond
	if n := sourceInt(meta["refreshDelayMillisecond"]); n != nil && *n >= 0 {
		delay = time.Duration(*n) * time.Millisecond
	} else if n := sourceInt(meta["refreshDelay"]); n != nil && *n >= 0 {
		delay = time.Duration(*n) * time.Second
	}
	if delay > 30*time.Second {
		return apiError("INCOMPLETE_SEARCH", "source refresh delay exceeds the bounded poll window", 0, true)
	}
	t := time.NewTimer(delay)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
