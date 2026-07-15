// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// PATCH(amend-2026-06-11: HTML fallback for Google's gated FlightsFrontendService RPC)
//
// Around 2026-06-09/10 Google started rejecting anonymous POSTs to
// GetShoppingResults and GetCalendarGraph with a travel.frontend.flights.
// ErrorResponse (code 13) envelope — the wrb.fr payload slot that used to
// carry the result JSON string is now null. The block is not specific to this
// client: fli (the Python library this backend is ported from, running
// curl-cffi chrome impersonation) gets the same error, and even Google's own
// web UI inside headless Chromium renders "Oops, something went wrong" for
// the same RPC. Consent cookies, f.sid/bl/_reqid params, and TLS
// fingerprinting do not help.
//
// What still works: a plain GET of the server-rendered search page
// (https://www.google.com/travel/flights/search?tfs=<protobuf>) with consent
// cookies. The page embeds the flight list in an AF_initDataCallback blob
// whose schema is IDENTICAL to the inner payload the RPC used to return, so
// parseOfferRow consumes it unchanged (verified live: 13 AUS→LAX itineraries
// with correct prices, carriers, times).
//
// This file implements that fallback:
//
//   - searchViaHTML: one GET per search, reusing the tfs= protobuf builder
//     from booking_urls.go and the bucket-walk from flights_native.go.
//   - datesViaHTML: cheapest-dates via one page GET per day (bounded
//     concurrency) — heavier than the calendar RPC, but correct.
//
// The RPC paths stay primary. The fallback fires only when the response
// carries the ErrorResponse envelope (errShoppingBlocked), so if Google
// un-gates the RPC the fast path resumes automatically.
//
// Known fidelity limits of the fallback (surfaced via the result Note):
// bags / emissions / layover filters are not encodable in the tfs URL, and
// page prices are per-person (no group-total divide needed).

package gflights

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/encoding/protowire"
)

// errShoppingBlocked is returned by the RPC response parsers when Google's
// envelope carries an ErrorResponse instead of a payload string. Callers fall
// back to the server-rendered HTML path.
var errShoppingBlocked = errors.New(
	"google flights RPC rejected the request (ErrorResponse envelope; Google is blocking non-interactive clients)")

// htmlFallbackNote is attached to results served by the fallback so agents
// and humans can see which path produced the data.
const htmlFallbackNote = "served via server-rendered HTML fallback: Google's flights RPC is currently " +
	"blocking non-interactive clients; bags/emissions/layover filters are not applied on this path"

// htmlFallbackSortNote discloses an unhonored --sort key. The fallback page
// carries no ranking data for best/top_flights/emissions, so those keys
// cannot be reproduced client-side.
const htmlFallbackSortNote = "; the requested sort %q is not available on this path — results are in Google page order"

// errorResponseMarker identifies the gated-RPC envelope. The full type URL is
// type.googleapis.com/travel.frontend.flights.ErrorResponse.
const errorResponseMarker = "travel.frontend.flights.ErrorResponse"

// envelopeBlockedErr inspects a decoded wrb.fr envelope whose payload slot
// was not a string and classifies it: the known ErrorResponse shape maps to
// errShoppingBlocked; anything else gets a descriptive parse error so the
// failure is never silent.
func envelopeBlockedErr(stripped string) error {
	if strings.Contains(stripped, errorResponseMarker) {
		return errShoppingBlocked
	}
	var outer [][]any
	if err := json.Unmarshal([]byte(stripped), &outer); err == nil {
		for _, row := range outer {
			if len(row) < 6 {
				continue
			}
			tag, _ := row[0].(string)
			status, ok := row[5].([]any)
			if tag == "wrb.fr" && ok && len(status) > 0 && int(numericFloat(status[0])) == 13 {
				return errShoppingBlocked
			}
		}
	}
	return fmt.Errorf("response wrb.fr payload is not a string (unrecognized envelope; Google response format may have changed)")
}

// consentCookie is the anonymous-consent cookie pair that lets the
// server-rendered page skip the consent interstitial. These are the
// documented generic "consent granted" values, not session identifiers.
const consentCookie = "CONSENT=YES+cb.20240101-00-p0.en+FX+999; SOCS=CAESEwgDEgk2NzM5OTg2NDUaAmVuIAEaBgiA_LyaBg"

// googleSearchPageURL builds the server-rendered search URL for the
// fallback. Same tfs protobuf as buildGoogleFlightsURL plus stops (field 5)
// and cabin class (field 9 honoring opts.CabinClass), with the caller's
// currency instead of hardcoded USD.
func googleSearchPageURL(opts SearchOptions, currencyCode string) (string, error) {
	if opts.Origin == "" || opts.Destination == "" || opts.DepartureDate == "" {
		return "", errors.New("origin, destination, and departure date are required")
	}
	seat, err := mapSeatType(opts.CabinClass)
	if err != nil {
		return "", err
	}
	stops, err := mapMaxStops(opts.MaxStops)
	if err != nil {
		return "", err
	}
	tripType := googleTripTypeOneWay
	if opts.ReturnDate != "" {
		tripType = googleTripTypeRoundTrip
	}
	pax := opts.Passengers
	if pax < 1 {
		pax = 1
	}

	encodeLeg := func(origin, dest, date string) []byte {
		leg := encodeFlightSlice(origin, dest, date)
		// Stops enum (url.proto field 5) uses the same values as the RPC's
		// maxStops* constants: 1=non-stop, 2=one-or-fewer, 3=two-or-fewer.
		// 0 (any) is the proto default and is omitted.
		if stops != maxStopsAny {
			var withStops []byte
			withStops = protowire.AppendTag(withStops, 5, protowire.VarintType)
			withStops = protowire.AppendVarint(withStops, uint64(stops))
			leg = append(leg, withStops...)
		}
		return leg
	}

	var pb []byte
	outbound := encodeLeg(opts.Origin, opts.Destination, opts.DepartureDate)
	pb = protowire.AppendTag(pb, 3, protowire.BytesType)
	pb = protowire.AppendVarint(pb, uint64(len(outbound)))
	pb = append(pb, outbound...)
	if opts.ReturnDate != "" {
		inbound := encodeLeg(opts.Destination, opts.Origin, opts.ReturnDate)
		pb = protowire.AppendTag(pb, 3, protowire.BytesType)
		pb = protowire.AppendVarint(pb, uint64(len(inbound)))
		pb = append(pb, inbound...)
	}
	for i := 0; i < pax; i++ {
		pb = protowire.AppendTag(pb, 8, protowire.VarintType)
		pb = protowire.AppendVarint(pb, uint64(googleTravelerAdult))
	}
	// Cabin class enum (url.proto field 9) shares the RPC seatType* values.
	pb = protowire.AppendTag(pb, 9, protowire.VarintType)
	pb = protowire.AppendVarint(pb, uint64(seat))
	pb = protowire.AppendTag(pb, 19, protowire.VarintType)
	pb = protowire.AppendVarint(pb, uint64(tripType))

	tfs := base64.RawURLEncoding.EncodeToString(pb)
	return fmt.Sprintf("%s?tfs=%s&curr=%s&hl=en&gl=US", googleFlightsSearchBase, tfs, currencyCode), nil
}

// fetchSearchPage GETs the server-rendered search page through the shared
// utls client with anonymous consent cookies.
var fetchSearchPage = func(ctx context.Context, pageURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return "", fmt.Errorf("building fallback request: %w", err)
	}
	req.Header.Set("User-Agent", chromeUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cookie", consentCookie)

	resp, err := utlsClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching fallback search page: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading fallback search page: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		snippet := string(body)
		if len(snippet) > 200 {
			snippet = snippet[:200] + "..."
		}
		return "", fmt.Errorf("fallback search page returned HTTP %d: %s", resp.StatusCode, snippet)
	}
	return string(body), nil
}

// extractInitDataBlobs pulls every AF_initDataCallback data array out of the
// page. Each blob is returned as raw JSON text. The scanner balances
// brackets while respecting JSON string literals and escapes — page payloads
// routinely contain "]" inside strings.
func extractInitDataBlobs(html string) []string {
	const callbackMark = "AF_initDataCallback("
	const dataMark = "data:"
	var blobs []string
	rest := html
	for {
		ci := strings.Index(rest, callbackMark)
		if ci < 0 {
			break
		}
		rest = rest[ci+len(callbackMark):]
		di := strings.Index(rest, dataMark)
		if di < 0 || di > 200 {
			// data: should appear within the callback's small object literal
			// header; a distant match belongs to something else.
			continue
		}
		seg := rest[di+len(dataMark):]
		// Skip whitespace to the opening bracket.
		j := 0
		for j < len(seg) && (seg[j] == ' ' || seg[j] == '\n' || seg[j] == '\t' || seg[j] == '\r') {
			j++
		}
		if j >= len(seg) || seg[j] != '[' {
			continue
		}
		end, ok := scanBalancedArray(seg[j:])
		if !ok {
			continue
		}
		blobs = append(blobs, seg[j:j+end])
		rest = seg[j+end:]
	}
	return blobs
}

func extractDS1ScriptBlobs(html string) []string {
	const scriptMark = "<script"
	const dataMark = "data:"
	var blobs []string
	rest := html
	for {
		si := strings.Index(rest, scriptMark)
		if si < 0 {
			break
		}
		rest = rest[si+len(scriptMark):]
		closeTag := strings.Index(rest, ">")
		if closeTag < 0 {
			continue
		}
		attrs := rest[:closeTag]
		afterOpen := rest[closeTag+1:]
		endTag := strings.Index(afterOpen, "</script>")
		if endTag < 0 {
			continue
		}
		body := afterOpen[:endTag]
		if nested := strings.Index(body, scriptMark); nested >= 0 {
			rest = afterOpen[nested:]
			continue
		}
		rest = afterOpen[endTag+len("</script>"):]
		if !strings.Contains(attrs, "ds:1") {
			continue
		}
		di := strings.Index(body, dataMark)
		if di < 0 {
			continue
		}
		seg := body[di+len(dataMark):]
		j := 0
		for j < len(seg) && (seg[j] == ' ' || seg[j] == '\n' || seg[j] == '\t' || seg[j] == '\r') {
			j++
		}
		if j >= len(seg) || seg[j] != '[' {
			continue
		}
		end, ok := scanBalancedArray(seg[j:])
		if !ok {
			continue
		}
		blobs = append(blobs, seg[j:j+end])
	}
	return blobs
}

// scanBalancedArray returns the exclusive end offset of the JSON array that
// starts at s[0] == '['. String literals and backslash escapes are honored.
func scanBalancedArray(s string) (int, bool) {
	depth := 0
	inString := false
	escaped := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inString {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i + 1, true
			}
		}
	}
	return 0, false
}

// flightsFromEmbeddedPayload walks one decoded blob with the same bucket
// layout parseOffersResponse uses on the RPC payload (offers at inner[2] and
// inner[3], rows at bucket[0]) and returns the parsed flights.
func flightsFromEmbeddedPayload(inner []any, currency string) []Flight {
	var flights []Flight
	for _, idx := range []int{2, 3} {
		if idx >= len(inner) {
			continue
		}
		bucket, ok := inner[idx].([]any)
		if !ok || len(bucket) == 0 {
			continue
		}
		rows, ok := bucket[0].([]any)
		if !ok {
			continue
		}
		for _, row := range rows {
			f, ok := parseOfferRow(row, currency)
			if !ok {
				continue
			}
			flights = append(flights, f)
		}
	}
	return flights
}

// flightsFromHTML extracts every AF_initDataCallback blob and returns the
// flights from the blob that yields the most itineraries (the page carries
// several unrelated blobs; only one embeds the shopping results).
func flightsFromHTML(html, currency string) []Flight {
	var best []Flight
	blobs := append(extractInitDataBlobs(html), extractDS1ScriptBlobs(html)...)
	for _, blob := range blobs {
		var inner []any
		if err := json.Unmarshal([]byte(blob), &inner); err != nil {
			continue
		}
		flights := flightsFromEmbeddedPayload(inner, currency)
		if len(flights) > len(best) {
			best = flights
		}
	}
	return best
}

// searchViaHTML is the fallback search path. Filters Google's RPC accepted
// but the tfs URL cannot express (airlines, time window) are applied
// client-side; page prices are per-person already, so no group-total divide.
// The returned note discloses the fallback and, when the requested sort key
// has no client-side equivalent, the unhonored sort.
func searchViaHTML(ctx context.Context, opts SearchOptions, currencyCode string) ([]Flight, string, error) {
	pageURL, err := googleSearchPageURL(opts, currencyCode)
	if err != nil {
		return nil, "", err
	}
	html, err := fetchSearchPage(ctx, pageURL)
	if err != nil {
		return nil, "", err
	}
	flights := flightsFromHTML(html, currencyCode)
	if len(flights) == 0 && pageMissingFlightData(html) {
		return nil, "", errors.New("fallback page did not embed flight data — Google likely served a consent " +
			"interstitial (the built-in SOCS consent cookie may have gone stale) or redesigned the page")
	}
	flights = filterFlightsClientSide(flights, opts)
	note := htmlFallbackNote
	if !sortFlightsClientSide(flights, opts.SortBy) {
		note += fmt.Sprintf(htmlFallbackSortNote, opts.SortBy)
	}
	return flights, note, nil
}

// pageMissingFlightData reports whether a fetched search page carries no
// embedded payload at all — the signature of a consent interstitial or a page
// redesign, as opposed to a legitimately empty result set (which still embeds
// AF_initDataCallback blobs).
func pageMissingFlightData(html string) bool {
	return strings.Contains(html, "consent.google.com") ||
		strings.Contains(html, "errorHasStatus: true") ||
		(!strings.Contains(html, "AF_initDataCallback(") && !strings.Contains(html, "ds:1"))
}

// sortFlightsClientSide orders fallback results for the sort keys the page
// data can reproduce. It reports false when the key needs RPC-side ranking
// data (best, top_flights, emissions) so the caller can disclose the
// unhonored sort.
func sortFlightsClientSide(flights []Flight, sortBy string) bool {
	key := strings.ToLower(strings.TrimSpace(sortBy))
	switch key {
	case "", "cheapest":
		sort.SliceStable(flights, func(i, j int) bool { return flights[i].Price < flights[j].Price })
	case "duration":
		sort.SliceStable(flights, func(i, j int) bool { return flights[i].DurationMinutes < flights[j].DurationMinutes })
	case "departure_time":
		sort.SliceStable(flights, func(i, j int) bool { return legTime(flights[i], false) < legTime(flights[j], false) })
	case "arrival_time":
		sort.SliceStable(flights, func(i, j int) bool { return legTime(flights[i], true) < legTime(flights[j], true) })
	default:
		return false
	}
	return true
}

// legTime returns the first departure or last arrival timestamp of a flight.
// Timestamps are "2006-01-02T15:04:05" strings, so lexicographic order is
// chronological order.
func legTime(f Flight, arrival bool) string {
	if len(f.Legs) == 0 {
		return ""
	}
	if arrival {
		return f.Legs[len(f.Legs)-1].ArrivalTime
	}
	return f.Legs[0].DepartureTime
}

// filterFlightsClientSide applies the airline and time-window filters the
// fallback URL cannot encode.
func filterFlightsClientSide(flights []Flight, opts SearchOptions) []Flight {
	if len(opts.Airlines) == 0 && opts.TimeWindow == "" {
		return flights
	}
	allowed := map[string]bool{}
	for _, a := range opts.Airlines {
		allowed[strings.ToUpper(strings.TrimSpace(a))] = true
	}
	var earliest, latest int
	haveWindow := false
	if opts.TimeWindow != "" {
		if e, l, err := parseTimeWindow(opts.TimeWindow); err == nil {
			earliest, latest, haveWindow = e, l, true
		}
	}
	out := flights[:0]
	for _, f := range flights {
		if len(allowed) > 0 {
			ok := len(f.Legs) > 0
			for _, leg := range f.Legs {
				if !allowed[strings.ToUpper(leg.Airline.Code)] {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}
		}
		if haveWindow && len(f.Legs) > 0 {
			dep := f.Legs[0].DepartureTime
			// DepartureTime is "2006-01-02T15:04:05"; hour at [11:13].
			if len(dep) >= 13 {
				hour := int(dep[11]-'0')*10 + int(dep[12]-'0')
				if hour < earliest || hour > latest {
					continue
				}
			}
		}
		out = append(out, f)
	}
	return out
}

// datesViaHTML serves the cheapest-dates query when the calendar RPC is
// blocked: one server-rendered page per day, cheapest itinerary kept.
// Bounded concurrency keeps the fan-out polite; individual day failures are
// skipped so one bad fetch doesn't sink the range, but an all-failure run
// returns the first error. The returned note discloses any skipped error days.
func datesViaHTML(ctx context.Context, opts DatesOptions, from, to time.Time, currencyCode string) ([]DatePrice, string, error) {
	var days []time.Time
	for cur := from; !cur.After(to); cur = cur.AddDate(0, 0, 1) {
		days = append(days, cur)
	}

	const workers = 4
	type dayResult struct {
		idx   int
		price *DatePrice
		err   error
	}
	results := make([]dayResult, len(days))
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	for i, day := range days {
		wg.Add(1)
		go func(i int, day time.Time) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if ctx.Err() != nil {
				results[i] = dayResult{idx: i, err: ctx.Err()}
				return
			}
			searchOpts := SearchOptions{
				Origin:        opts.Origin,
				Destination:   opts.Destination,
				DepartureDate: day.Format("2006-01-02"),
				Airlines:      opts.Airlines,
				CabinClass:    opts.CabinClass,
				MaxStops:      opts.MaxStops,
			}
			flights, _, err := searchViaHTML(ctx, searchOpts, currencyCode)
			if err != nil {
				results[i] = dayResult{idx: i, err: err}
				return
			}
			var cheapest *Flight
			for j := range flights {
				if flights[j].Price <= 0 {
					continue
				}
				if cheapest == nil || flights[j].Price < cheapest.Price {
					cheapest = &flights[j]
				}
			}
			if cheapest == nil {
				results[i] = dayResult{idx: i}
				return
			}
			results[i] = dayResult{idx: i, price: &DatePrice{
				DepartureDate: day.Format("2006-01-02"),
				Price:         cheapest.Price,
				Currency:      currencyCode,
			}}
		}(i, day)
	}
	wg.Wait()

	var out []DatePrice
	var firstErr error
	failedDays := 0
	for _, r := range results {
		if r.err != nil {
			failedDays++
			if firstErr == nil {
				firstErr = r.err
			}
		}
		if r.price != nil {
			out = append(out, *r.price)
		}
	}
	if len(out) == 0 && firstErr != nil {
		return nil, "", fmt.Errorf("dates HTML fallback failed for every day in range: %w", firstErr)
	}
	note := htmlFallbackNote
	if failedDays > 0 {
		note += fmt.Sprintf("; %d day(s) in range could not be fetched and are absent from the result", failedDays)
	}
	return out, note, nil
}
