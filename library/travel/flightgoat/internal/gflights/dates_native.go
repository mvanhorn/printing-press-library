// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// Native Go implementation of Google Flights' GetCalendarGraph endpoint —
// what fli (the Python library) exposes as cheapest-dates / date-grid search.
//
// Why this file exists: prior to this, flightgoat shelled out to fli for
// `dates` and `gf-search` commands. That made the binary unusable in an
// MCPB context (no Python, no pipx), and added a runtime dependency users
// had to install separately. This file ports fli's request-builder and
// response-parser to Go so the dependency goes away.
//
// The endpoint is NOT protobuf — it's a deeply nested JSON payload with
// positional fields that Google's frontend frames as `f.req=<URL-encoded
// JSON>`. Validated empirically that vanilla net/http (no utls/Surf) talks
// to the endpoint successfully; the calendar service does not appear to
// enforce TLS-fingerprint anti-bot. If that ever changes, switch to utls.
//
// Field ordering and semantics ported from fli/models/google_flights/dates.py.

package gflights

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	calendarEndpoint    = "https://www.google.com/_/FlightsFrontendUi/data/travel.frontend.flights.FlightsFrontendService/GetCalendarGraph"
	maxDaysPerSearch    = 61
	googleResponsePrefix = ")]}'"
	chromeUserAgent      = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
)

// Enum values mirror fli's google_flights.base. They serialize as ints in the
// payload — using strings causes Google to silently return null prices.
const (
	tripTypeOneWay    = 2
	tripTypeRoundTrip = 1

	seatTypeEconomy        = 1
	seatTypePremiumEconomy = 2
	seatTypeBusiness       = 3
	seatTypeFirst          = 4

	maxStopsAny           = 0
	maxStopsNonStop       = 1
	maxStopsOneOrFewer    = 2
	maxStopsTwoOrFewer    = 3
)

// datesNative is the native-Go replacement for the fli subprocess. Returns
// the same DatesResult shape so callers don't care which backend ran.
func datesNative(ctx context.Context, opts DatesOptions) (*DatesResult, error) {
	from, err := time.Parse("2006-01-02", opts.From)
	if err != nil {
		return nil, fmt.Errorf("parsing from date %q: %w", opts.From, err)
	}
	to, err := time.Parse("2006-01-02", opts.To)
	if err != nil {
		return nil, fmt.Errorf("parsing to date %q: %w", opts.To, err)
	}
	if to.Before(from) {
		return nil, fmt.Errorf("--to %s is before --from %s", opts.To, opts.From)
	}

	// Chunk ranges > maxDaysPerSearch. Google rejects single requests spanning
	// more than 61 days; fli does the same chunking in its Python loop.
	var all []DatePrice
	cur := from
	for !cur.After(to) {
		chunkEnd := cur.AddDate(0, 0, maxDaysPerSearch-1)
		if chunkEnd.After(to) {
			chunkEnd = to
		}
		// The travel_date inside each segment must shift with the chunk so the
		// segment's anchor day is inside the chunk's [from, to] range. fli does
		// the equivalent inside its loop.
		chunk, err := datesChunk(ctx, opts, cur, chunkEnd)
		if err != nil {
			return nil, err
		}
		all = append(all, chunk...)
		cur = chunkEnd.AddDate(0, 0, 1)
	}

	return &DatesResult{
		Success:    true,
		Source:     "native-go",
		DataSource: "google_flights",
		SearchType: "dates",
		Query: SearchQuery{
			Origin:      opts.Origin,
			Destination: opts.Destination,
		},
		Count: len(all),
		Dates: all,
	}, nil
}

// datesChunk fires one POST against the calendar endpoint for a date range
// guaranteed to be <= maxDaysPerSearch days.
func datesChunk(ctx context.Context, opts DatesOptions, from, to time.Time) ([]DatePrice, error) {
	payload, err := buildDatesPayload(opts, from, to)
	if err != nil {
		return nil, fmt.Errorf("building payload: %w", err)
	}
	body := "f.req=" + payload

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, calendarEndpoint, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	req.Header.Set("User-Agent", chromeUserAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling calendar endpoint: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		snippet := string(respBody)
		if len(snippet) > 200 {
			snippet = snippet[:200] + "..."
		}
		return nil, fmt.Errorf("calendar endpoint returned HTTP %d: %s", resp.StatusCode, snippet)
	}

	return parseDatesResponse(respBody)
}

// buildDatesPayload constructs the URL-encoded `f.req` value for a single
// chunk. The shape mirrors fli's DateSearchFilters.format() — see
// fli/models/google_flights/dates.py for the canonical field map.
func buildDatesPayload(opts DatesOptions, from, to time.Time) (string, error) {
	tripType := tripTypeOneWay
	if opts.RoundTrip {
		tripType = tripTypeRoundTrip
	}
	seat, err := mapSeatType(opts.CabinClass)
	if err != nil {
		return "", err
	}
	stops, err := mapMaxStops(opts.MaxStops)
	if err != nil {
		return "", err
	}

	// Anchor the segment travel_date inside the chunk so Google interprets
	// the calendar window as relative to the segment's day. We use `from`
	// as the anchor; Google returns prices for every day in [from, to].
	travelDate := from.Format("2006-01-02")

	var airlinesField any
	if len(opts.Airlines) > 0 {
		airlines := make([]any, 0, len(opts.Airlines))
		for _, a := range opts.Airlines {
			airlines = append(airlines, strings.ToUpper(a))
		}
		airlinesField = airlines
	}

	segment := []any{
		[]any{[]any{[]any{strings.ToUpper(opts.Origin), 0}}},      // [0] departure airport, nested 3 deep
		[]any{[]any{[]any{strings.ToUpper(opts.Destination), 0}}}, // [1] arrival airport
		nil,                                                       // [2] time restrictions
		stops,                                                     // [3] stops
		airlinesField,                                             // [4] airlines
		nil,                                                       // [5] unknown
		travelDate,                                                // [6] travel date (anchor)
		nil,                                                       // [7] max duration
		nil,                                                       // [8] selected flight
		nil,                                                       // [9] layover airports
		nil,                                                       // [10] unknown
		nil,                                                       // [11] unknown
		nil,                                                       // [12] layover duration
		nil,                                                       // [13] emissions filter
		3,                                                         // [14] unknown — fli always sends 3
	}

	// For round trips we'd need a second segment for the return — fli builds
	// it from the same args but with origin/dest swapped. This implementation
	// handles ONE_WAY first; round-trip support is a TODO that mirrors fli's
	// `flight_segments` len-2 case.
	if opts.RoundTrip {
		return "", errors.New("round-trip date searches not yet implemented in native backend")
	}

	filters := []any{
		nil, // [0] placeholder (dates uses nil; flights uses [])
		[]any{
			nil,                          // [0]
			nil,                          // [1]
			tripType,                     // [2] trip type
			nil,                          // [3]
			[]any{},                      // [4]
			seat,                         // [5] seat type
			[]any{passengerAdults(opts), 0, 0, 0}, // [6] passengers: [adults, children, lap, seat]
			nil,                          // [7] price limit
			nil,                          // [8]
			nil,                          // [9]
			nil,                          // [10] bags
			nil,                          // [11]
			nil,                          // [12]
			[]any{segment},               // [13] segments
			nil,                          // [14]
			nil,                          // [15]
			nil,                          // [16]
			1,                            // [17]
		},
		[]any{from.Format("2006-01-02"), to.Format("2006-01-02")},
	}

	innerJSON, err := json.Marshal(filters)
	if err != nil {
		return "", fmt.Errorf("marshaling filters: %w", err)
	}
	wrapped := []any{nil, string(innerJSON)}
	wrappedJSON, err := json.Marshal(wrapped)
	if err != nil {
		return "", fmt.Errorf("marshaling wrapper: %w", err)
	}
	return url.QueryEscape(string(wrappedJSON)), nil
}

func passengerAdults(_ DatesOptions) int {
	// DatesOptions doesn't expose a passenger count yet; fli defaults to 1.
	return 1
}

func mapSeatType(s string) (int, error) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "", "ECONOMY":
		return seatTypeEconomy, nil
	case "PREMIUM_ECONOMY", "PREMIUM-ECONOMY", "PREMIUMECONOMY":
		return seatTypePremiumEconomy, nil
	case "BUSINESS":
		return seatTypeBusiness, nil
	case "FIRST":
		return seatTypeFirst, nil
	default:
		return 0, fmt.Errorf("unknown cabin class %q", s)
	}
}

func mapMaxStops(s string) (int, error) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "", "ANY":
		return maxStopsAny, nil
	case "NON_STOP", "NONSTOP", "NON-STOP":
		return maxStopsNonStop, nil
	case "ONE_STOP_OR_FEWER", "ONE-STOP-OR-FEWER":
		return maxStopsOneOrFewer, nil
	case "TWO_OR_FEWER_STOPS", "TWO-OR-FEWER-STOPS":
		return maxStopsTwoOrFewer, nil
	default:
		return 0, fmt.Errorf("unknown max stops %q", s)
	}
}

// parseDatesResponse unwraps Google's )]}' prefix, drills into the wrb.fr
// envelope, and returns one DatePrice per date that came back with a price.
// Items with null price are silently skipped (mirrors fli).
func parseDatesResponse(body []byte) ([]DatePrice, error) {
	stripped := strings.TrimPrefix(string(body), googleResponsePrefix)
	stripped = strings.TrimSpace(stripped)

	var outer [][]any
	if err := json.Unmarshal([]byte(stripped), &outer); err != nil {
		return nil, fmt.Errorf("parsing outer envelope: %w", err)
	}
	if len(outer) == 0 || len(outer[0]) < 3 {
		return nil, errors.New("response envelope missing wrb.fr entry")
	}
	innerStr, ok := outer[0][2].(string)
	if !ok || innerStr == "" {
		return nil, errors.New("response wrb.fr payload is not a string")
	}

	var inner []any
	if err := json.Unmarshal([]byte(innerStr), &inner); err != nil {
		return nil, fmt.Errorf("parsing inner payload: %w", err)
	}
	if len(inner) == 0 {
		return nil, errors.New("inner payload is empty")
	}

	// fli does data[-1] — the final element holds the date items.
	dateItems, ok := inner[len(inner)-1].([]any)
	if !ok {
		return nil, fmt.Errorf("expected []any for date items, got %T", inner[len(inner)-1])
	}

	var out []DatePrice
	for _, raw := range dateItems {
		item, ok := raw.([]any)
		if !ok || len(item) < 3 {
			continue
		}
		dateStr, _ := item[0].(string)
		if dateStr == "" {
			continue
		}
		price, currency := parsePriceAndCurrency(item[2])
		if price <= 0 {
			continue
		}
		out = append(out, DatePrice{
			DepartureDate: dateStr,
			Price:         price,
			Currency:      currency,
		})
	}
	return out, nil
}

// parsePriceAndCurrency walks item[2] which is shaped as
// [[null, <price>], "<base64 token>"] when a price exists, or null otherwise.
func parsePriceAndCurrency(raw any) (float64, string) {
	priceWrap, ok := raw.([]any)
	if !ok || len(priceWrap) < 1 {
		return 0, ""
	}
	priceArr, ok := priceWrap[0].([]any)
	if !ok || len(priceArr) < 2 {
		return 0, ""
	}
	priceVal, _ := priceArr[1].(float64)
	currency := ""
	if len(priceWrap) >= 2 {
		if token, ok := priceWrap[1].(string); ok {
			currency = extractCurrency(token)
		}
	}
	return priceVal, currency
}

// extractCurrency walks the base64-decoded protobuf inside the price token to
// find field 3 (nested message) -> field 3 (currency string). Mirrors fli's
// extract_currency_from_price_token in fli/core/currency.py.
func extractCurrency(token string) string {
	if token == "" {
		return ""
	}
	// urlsafe base64 may need padding.
	padding := (4 - len(token)%4) % 4
	padded := token + strings.Repeat("=", padding)
	data, err := base64.URLEncoding.DecodeString(padded)
	if err != nil {
		// Fall back to standard b64 in case the token isn't urlsafe.
		data, err = base64.StdEncoding.DecodeString(padded)
		if err != nil {
			return ""
		}
	}
	currency, ok := findCurrencyField(data)
	if !ok {
		return ""
	}
	return strings.ToUpper(currency)
}

// findCurrencyField walks a protobuf payload looking for field 3 (length-
// delimited), recurses into it, and returns field 3 (length-delimited) within
// the nested message as the currency string.
func findCurrencyField(data []byte) (string, bool) {
	off := 0
	for off < len(data) {
		tag, n, err := readVarint(data[off:])
		if err != nil {
			return "", false
		}
		off += n
		fieldNum := tag >> 3
		wireType := tag & 0x7

		if fieldNum == 3 && wireType == 2 {
			length, n, err := readVarint(data[off:])
			if err != nil {
				return "", false
			}
			off += n
			end := off + int(length)
			if end > len(data) {
				return "", false
			}
			nested := data[off:end]
			off = end

			// Recurse one level: in the nested message, look for field 3 (the
			// ISO currency code as a length-delimited string).
			noff := 0
			for noff < len(nested) {
				ntag, nn, nerr := readVarint(nested[noff:])
				if nerr != nil {
					break
				}
				noff += nn
				nfn := ntag >> 3
				nwt := ntag & 0x7
				if nfn == 3 && nwt == 2 {
					nlen, nn2, nerr2 := readVarint(nested[noff:])
					if nerr2 != nil {
						break
					}
					noff += nn2
					nend := noff + int(nlen)
					if nend > len(nested) {
						break
					}
					return string(nested[noff:nend]), true
				}
				noff = skipField(nested, noff, nwt)
				if noff < 0 {
					break
				}
			}
			continue
		}
		off = skipField(data, off, wireType)
		if off < 0 {
			return "", false
		}
	}
	return "", false
}

func readVarint(data []byte) (uint64, int, error) {
	var value uint64
	var shift uint
	for i, b := range data {
		value |= uint64(b&0x7F) << shift
		if b&0x80 == 0 {
			return value, i + 1, nil
		}
		shift += 7
		if shift >= 64 {
			return 0, 0, errors.New("varint too large")
		}
	}
	return 0, 0, errors.New("unexpected end of data")
}

func skipField(data []byte, off int, wireType uint64) int {
	switch wireType {
	case 0: // varint
		_, n, err := readVarint(data[off:])
		if err != nil {
			return -1
		}
		return off + n
	case 1: // fixed64
		if off+8 > len(data) {
			return -1
		}
		return off + 8
	case 2: // length-delimited
		length, n, err := readVarint(data[off:])
		if err != nil {
			return -1
		}
		off += n
		if off+int(length) > len(data) {
			return -1
		}
		return off + int(length)
	case 5: // fixed32
		if off+4 > len(data) {
			return -1
		}
		return off + 4
	default:
		return -1
	}
}
