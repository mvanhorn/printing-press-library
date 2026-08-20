package delta

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// GetSeatMap opens delta.com My Trips, navigates to the trip, clicks "View Seats"
// for the target flight, and returns the full seat availability map from the DOM.
//
// flightIndex is 1-based (1 = first flight in the itinerary). Pass 0 to default to 1.
func GetSeatMap(ctx context.Context, confirmationNo, firstName, lastName string, flightIndex int) (*SeatMapResult, error) {
	conf := strings.ToUpper(confirmationNo)
	first := strings.ToUpper(firstName)
	last := strings.ToUpper(lastName)
	if flightIndex <= 0 {
		flightIndex = 1
	}

	browser, cleanup, err := launchBrowser()
	if err != nil {
		return nil, fmt.Errorf("launching browser: %w", err)
	}
	defer cleanup()

	// Do NOT bind the browser to ctx via browser.Context(ctx): that attaches CDP
	// event subscriptions to the context and causes premature teardown when the
	// deadline approaches.  We respect ctx manually via pollCondition selects.
	page, err := browser.Page(proto.TargetCreateTarget{URL: ""})
	if err != nil {
		return nil, fmt.Errorf("opening browser tab: %w", err)
	}
	if err := applyStealthScripts(page); err != nil {
		return nil, fmt.Errorf("stealth setup: %w", err)
	}

	// ── Phase 1: Navigate to My Trips and fill the search form ──────────────

	// Raw CDP navigate — returns immediately without blocking on loadEventFired.
	// delta.com's SPA never fires loadEventFired cleanly (background polling).
	navCmd := proto.PageNavigate{URL: "https://www.delta.com/my-trips/"}
	if _, navErr := navCmd.Call(page); navErr != nil {
		return nil, fmt.Errorf("navigate command: %w", navErr)
	}

	// Poll for the search form inputs (up to 20 s).
	pollCondition(ctx, page, 10, 2*time.Second, `() => {
		function findInShadow(root, sel) {
			const el = root.querySelector(sel);
			if (el) return el;
			for (const n of root.querySelectorAll('*')) {
				if (n.shadowRoot) { const f = findInShadow(n.shadowRoot, sel); if (f) return f; }
			}
			return null;
		}
		const sels = ['input[name="confirmationNo"]','input[id*="confirm"]','input[placeholder*="confirmation" i]'];
		for (const s of sels) { if (findInShadow(document, s)) return true; }
		return false;
	}`)

	// Fill the search form (Shadow DOM traversal) and submit.
	page.Eval(`(conf, first, last) => {
		function findInShadow(root, selector) {
			const el = root.querySelector(selector);
			if (el) return el;
			for (const node of root.querySelectorAll('*')) {
				if (node.shadowRoot) {
					const found = findInShadow(node.shadowRoot, selector);
					if (found) return found;
				}
			}
			return null;
		}
		function setInput(el, value) {
			const desc = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value');
			if (desc && desc.set) desc.set.call(el, value);
			el.value = value;
			['input','change','keyup'].forEach(t => el.dispatchEvent(new Event(t, {bubbles:true})));
		}
		const confSels = ['input[name="confirmationNo"]','input[id*="confirm"]','input[placeholder*="confirmation" i]','input[aria-label*="confirmation" i]'];
		const firstSels = ['input[name="firstName"]','input[id*="first"]','input[placeholder*="first" i]','input[aria-label*="first" i]'];
		const lastSels  = ['input[name="lastName"]','input[id*="last"]','input[placeholder*="last" i]','input[aria-label*="last" i]'];
		let confEl, firstEl, lastEl;
		for (const s of confSels)  { confEl  = findInShadow(document, s); if (confEl)  break; }
		for (const s of firstSels) { firstEl = findInShadow(document, s); if (firstEl) break; }
		for (const s of lastSels)  { lastEl  = findInShadow(document, s); if (lastEl)  break; }
		if (confEl)  setInput(confEl,  conf);
		if (firstEl) setInput(firstEl, first);
		if (lastEl)  setInput(lastEl,  last);
		const btnSels = ['button[type="submit"]','button.submit-btn','input[type="submit"]','[role="button"][class*="submit"]','button[data-id*="search"]'];
		let submitEl;
		for (const s of btnSels) { submitEl = findInShadow(document, s); if (submitEl) break; }
		if (!submitEl) {
			submitEl = Array.from(document.querySelectorAll('button')).find(b => /find|search|look up/i.test(b.textContent));
		}
		if (submitEl) submitEl.click();
	}`, conf, first, last)

	// ── Phase 2: Wait for trip details to render ─────────────────────────────

	// Dismiss cookie dialog if present.
	time.Sleep(2 * time.Second)
	page.Eval(`() => {
		const btns = Array.from(document.querySelectorAll('button'));
		for (const btn of btns) {
			const txt = (btn.textContent || '').trim();
			const parent = btn.closest('[class*="cookie"],[class*="consent"],[id*="cookie"]');
			if (parent && /accept|agree|ok/i.test(txt)) { btn.click(); return; }
			if (txt === 'Accept' || txt === 'Accept All' || txt === 'Accept Cookies') { btn.click(); return; }
		}
	}`)

	// Wait for trip content: substantial body text AND flight keywords.
	// Allow up to 60 s (20 × 3 s) for the SPA to render the trip.
	time.Sleep(3 * time.Second)
	pollCondition(ctx, page, 20, 3*time.Second, `() => {
		const txt = (document.body.innerText || '');
		if (txt.length < 2000) return false;
		const lower = txt.toLowerCase();
		return (lower.includes('depart') || lower.includes('departure') || lower.includes('dl')) &&
		       (lower.includes('arrive') || lower.includes('arrival') || lower.includes('view seats'));
	}`)

	// ── Phase 3: Extract passenger seats from the trip details page ──────────

	// Capture passenger seat assignments before navigating away.
	// The trip summary shows lines like "JOHN DOE  15C  Delta Main (Q)".
	paxSeats := extractPassengerSeatsFromDOM(page)

	// Also capture route and flight number for the target flight.
	tripResult := scrapeTripMetaFromDOM(page, conf, flightIndex)

	// ── Phase 4: Expand flight details and click "View Seats" ────────────────

	// Expand "Show flight details" if collapsed.
	page.Eval(`() => {
		const btns = Array.from(document.querySelectorAll('button, [role="button"], a'));
		const expandBtn = btns.find(b => /show.*(flight|all).*detail|flight.*detail|view.*detail/i.test(b.textContent || ''));
		if (expandBtn) { expandBtn.click(); return; }
		const byId = document.querySelector('#toggleFlightDetailsButton');
		if (byId) { byId.click(); }
	}`)
	time.Sleep(2 * time.Second)

	// Click "View Seats" for the target flight (flightIndex is 1-based, JS arg).
	page.Eval(`(idx) => {
		const patterns = [/view seat/i, /seat map/i, /select seat/i, /choose seat/i,
		                  /change seat/i, /seat selection/i, /upgrade seat/i];
		const els = Array.from(document.querySelectorAll('a, button, [role="button"]'));
		const matches = els.filter(el => {
			const txt = (el.textContent || '').trim();
			const lbl = (el.getAttribute('aria-label') || '');
			const href = (el.getAttribute('href') || '');
			if (/upgrade/i.test(txt) && !/view/i.test(txt)) return false;
			return patterns.some(p => p.test(txt) || p.test(lbl)) || /seat/i.test(href);
		});
		if (!matches.length) return false;
		const target = matches[Math.min(idx - 1, matches.length - 1)];
		target.scrollIntoView();
		target.click();
		return true;
	}`, flightIndex)

	// ── Phase 5: Wait for seat map page to render ────────────────────────────

	time.Sleep(3 * time.Second)
	seatPage := page
	if pages, err2 := browser.Pages(); err2 == nil && len(pages) > 1 {
		seatPage = pages[len(pages)-1]
	}

	// Wait for enough seat elements to confirm the full map rendered.
	// Require ≥10 elements whose text content is exactly a seat designator.
	pollCondition(ctx, seatPage, 20, 2*time.Second, `() => {
		let n = 0;
		for (const el of document.querySelectorAll('div,span,td,li,button,g,rect,circle')) {
			const txt = (el.textContent||'').trim();
			if (/^\d{1,2}[A-K]$/i.test(txt)) n++;
			if (n >= 10) return true;
		}
		return false;
	}`)

	// Allow extra rendering time.
	time.Sleep(2 * time.Second)

	// ── Phase 6: Scrape the seat map DOM ─────────────────────────────────────

	return scrapeSeatMapDOM(seatPage, conf, tripResult, paxSeats)
}

// extractPassengerSeatsFromDOM reads the trip-details page and returns a set of
// seat designators (e.g. {"15C": true}) assigned to passengers on this trip.
func extractPassengerSeatsFromDOM(page *rod.Page) map[string]bool {
	res, err := page.Eval(`() => {
		const seatRe = /\b(\d{1,2}[A-K])\b/gi;
		const seats = new Set();
		// Strategy 1: look for table/list cells adjacent to passenger name cells.
		for (const el of document.querySelectorAll('td, li, span, div')) {
			const txt = (el.textContent || '').trim();
			const m = txt.match(/^(\d{1,2}[A-K])$/i);
			if (m) seats.add(m[1].toUpperCase());
		}
		return Array.from(seats);
	}`)
	m := make(map[string]bool)
	if err != nil || res == nil {
		return m
	}
	var seats []string
	b, _ := res.Value.MarshalJSON()
	if err := json.Unmarshal(b, &seats); err != nil {
		return m
	}
	for _, s := range seats {
		m[strings.ToUpper(s)] = true
	}
	return m
}

// scrapeTripMetaFromDOM reads the trip-details page for the flight metadata
// (flight number, departure/arrival airports) of the target flight index.
func scrapeTripMetaFromDOM(page *rod.Page, conf string, flightIndex int) *TripResult {
	res, err := page.Eval(`(idx) => {
		const txt = document.body.innerText || '';
		const lines = txt.split('\n').map(l => l.trim()).filter(Boolean);

		// Find flight numbers like "DL5597".
		const flightRe = /\b(DL|OO|9E|MQ|WN|AA|UA|B6|AS|NK|F9)\d{3,4}\b/g;
		const flights = [];
		const seen = new Set();
		for (const line of lines) {
			const m = line.match(/\b([A-Z]{2}\d{3,4})\b/);
			if (m && !seen.has(m[1])) {
				seen.add(m[1]);
				flights.push({ flightNumber: m[1] });
			}
		}

		// Find route pairs: two 3-letter IATA codes.
		// Formats: "JAX → BOS", "JAX BOS", "(JAX) ... (BOS)" (may span lines).
		let dep = '', arr = '';
		// 1. Check individual lines for inline pairs.
		for (const line of lines.slice(0, 50)) {
			const m1 = line.match(/\b([A-Z]{3})\s*[→\-]\s*([A-Z]{3})\b/);
			if (m1) { dep = m1[1]; arr = m1[2]; break; }
			const m2 = line.match(/\(([A-Z]{3})\).*?\(([A-Z]{3})\)/);
			if (m2) { dep = m2[1]; arr = m2[2]; break; }
		}
		// 2. Full-body scan: find two (XXX) codes that appear close together
		//    (the route display shows both airport codes within ~200 chars).
		if (!dep) {
			const allCodes = [];
			const re = /\(([A-Z]{3})\)/g;
			let mm;
			while ((mm = re.exec(txt)) !== null) allCodes.push({code: mm[1], pos: mm.index});
			for (let i = 0; i < allCodes.length - 1; i++) {
				if (allCodes[i+1].pos - allCodes[i].pos < 200) {
					dep = allCodes[i].code; arr = allCodes[i+1].code; break;
				}
			}
		}

		// Return the flight at the target index (1-based).
		const f = flights[Math.max(0, idx - 1)] || flights[0] || {};
		return { flightNumber: f.flightNumber || '', dep, arr };
	}`, flightIndex)

	trip := &TripResult{ConfirmationNumber: conf}
	if err == nil && res != nil {
		var raw struct {
			FlightNumber string `json:"flightNumber"`
			Dep          string `json:"dep"`
			Arr          string `json:"arr"`
		}
		b, _ := res.Value.MarshalJSON()
		if json.Unmarshal(b, &raw) == nil {
			trip.Flights = []*Flight{{
				FlightNumber: raw.FlightNumber,
				Departure:    FlightStop{Airport: raw.Dep},
				Arrival:      FlightStop{Airport: raw.Arr},
			}}
		}
	}
	return trip
}

// pollCondition evaluates jsExpr on page up to maxTries times, sleeping sleep
// between tries. Returns true if the expression returned true before running out.
// Respects ctx cancellation.
func pollCondition(ctx context.Context, page *rod.Page, maxTries int, sleep time.Duration, jsExpr string) bool {
	for i := 0; i < maxTries; i++ {
		select {
		case <-ctx.Done():
			return false
		default:
		}
		res, err := page.Eval(jsExpr)
		if err == nil && res != nil && res.Value.Bool() {
			return true
		}
		time.Sleep(sleep)
	}
	return false
}

// parseSeatMapAPIJSON tries to parse a captured seat-map XHR response.
func parseSeatMapAPIJSON(body []byte, conf string, trip *TripResult, flightIdx int) (*SeatMapResult, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	result := newSeatMapResult(conf, trip, flightIdx, nil)
	paxSeats := passengerSeatSet(trip, flightIdx)
	cabinsRaw, _ := raw["cabins"].([]interface{})
	if len(cabinsRaw) == 0 {
		if data, ok := raw["data"].(map[string]interface{}); ok {
			cabinsRaw, _ = data["cabins"].([]interface{})
		}
	}
	for _, cr := range cabinsRaw {
		cabin, _ := cr.(map[string]interface{})
		if cabin == nil {
			continue
		}
		cabinName := resolveCabinName(apiStr(cabin, "cabinClass"), apiStr(cabin, "name"), apiStr(cabin, "description"))
		mc := &SeatMapCabin{Name: cabinName}
		rowsRaw, _ := cabin["rows"].([]interface{})
		for _, rr := range rowsRaw {
			row, _ := rr.(map[string]interface{})
			if row == nil {
				continue
			}
			rowNum := int(apiFloat(row, "rowNumber"))
			if rowNum == 0 {
				rowNum = int(apiFloat(row, "number"))
			}
			mr := &SeatMapRow{Number: rowNum}
			seatsRaw, _ := row["seats"].([]interface{})
			for _, sr := range seatsRaw {
				seat, _ := sr.(map[string]interface{})
				if seat == nil {
					continue
				}
				seatNum := strings.ToUpper(apiStr(seat, "seatNumber"))
				if seatNum == "" {
					seatNum = strings.ToUpper(apiStr(seat, "number"))
				}
				if seatNum == "" {
					continue
				}
				status := resolveAPIStatus(seat, paxSeats[seatNum])
				_, col := parseSeatNum(seatNum)
				ms := &SeatMapSeat{
					Number:  seatNum,
					Status:  status,
					Type:    seatColumnType(col),
					ExitRow: asBoolField(seat, "exitRow") || asBoolField(seat, "isExitRow"),
				}
				mr.Seats = append(mr.Seats, ms)
				if ms.ExitRow {
					mr.ExitRow = true
				}
				result.TotalSeats++
				tallySeat(result, status)
			}
			if len(mr.Seats) > 0 {
				mc.Rows = append(mc.Rows, mr)
			}
		}
		if len(mc.Rows) > 0 {
			result.Cabins = append(result.Cabins, mc)
		}
	}
	return result, nil
}

// scrapeSeatMapDOM extracts seat data from the rendered delta.com seat map page.
// paxSeats is a set of seat numbers belonging to the passenger (marked "your-seat").
func scrapeSeatMapDOM(page *rod.Page, conf string, trip *TripResult, paxSeats map[string]bool) (*SeatMapResult, error) {
	res, err := page.Eval(seatMapDOMScript)
	if err != nil {
		return nil, fmt.Errorf("DOM extraction: %w", err)
	}

	var jsData struct {
		Seats []struct {
			Number  string `json:"number"`
			Status  string `json:"status"`
			Cabin   string `json:"cabin"`
			ExitRow bool   `json:"exitRow"`
		} `json:"seats"`
		URL string `json:"url"`
		Dep string `json:"dep"`
		Arr string `json:"arr"`
	}
	rawJSON, _ := res.Value.MarshalJSON()
	if err := json.Unmarshal(rawJSON, &jsData); err != nil {
		return nil, fmt.Errorf("parsing DOM data: %w", err)
	}
	if len(jsData.Seats) == 0 {
		return nil, fmt.Errorf("no seats found on page (%s) — the seat map may not be available for this flight yet", jsData.URL)
	}

	// If no passenger seats were supplied, build from trip data.
	if paxSeats == nil {
		paxSeats = passengerSeatSet(trip, 1)
	}

	result := newSeatMapResult(conf, trip, 1, paxSeats)
	// If trip metadata lacked route info, use what the seat map page itself shows.
	if result.Route == "" && (jsData.Dep != "" || jsData.Arr != "") {
		result.Route = jsData.Dep + "→" + jsData.Arr
	}
	cabinRows := make(map[string]map[int][]*SeatMapSeat)
	var cabinOrder []string
	seenCabin := make(map[string]bool)

	for _, rs := range jsData.Seats {
		rowNum, col := parseSeatNum(rs.Number)
		if rowNum == 0 {
			continue
		}
		status := rs.Status
		if paxSeats[rs.Number] {
			status = "your-seat"
		}
		cabin := rs.Cabin
		if cabin == "" {
			cabin = inferCabin(rowNum)
		}
		if !seenCabin[cabin] {
			seenCabin[cabin] = true
			cabinOrder = append(cabinOrder, cabin)
		}
		if cabinRows[cabin] == nil {
			cabinRows[cabin] = make(map[int][]*SeatMapSeat)
		}
		ms := &SeatMapSeat{
			Number:  rs.Number,
			Status:  status,
			Type:    seatColumnType(col),
			ExitRow: rs.ExitRow,
		}
		cabinRows[cabin][rowNum] = append(cabinRows[cabin][rowNum], ms)
		result.TotalSeats++
		tallySeat(result, status)
	}

	for _, cabinName := range cabinOrder {
		rows := cabinRows[cabinName]
		rowNums := make([]int, 0, len(rows))
		for rn := range rows {
			rowNums = append(rowNums, rn)
		}
		sort.Ints(rowNums)

		mc := &SeatMapCabin{Name: cabinName}
		for _, rn := range rowNums {
			seats := rows[rn]
			sort.Slice(seats, func(i, j int) bool { return seats[i].Number < seats[j].Number })
			mr := &SeatMapRow{Number: rn, Seats: seats}
			for _, s := range seats {
				if s.ExitRow {
					mr.ExitRow = true
				}
			}
			mc.Rows = append(mc.Rows, mr)
		}
		result.Cabins = append(result.Cabins, mc)
	}
	return result, nil
}

// --- helpers ---

func newSeatMapResult(conf string, trip *TripResult, flightIdx int, _ map[string]bool) *SeatMapResult {
	r := &SeatMapResult{ConfirmationNumber: conf}
	if trip != nil && len(trip.Flights) > 0 {
		f := trip.Flights[0]
		r.FlightNumber = f.FlightNumber
		r.Aircraft = f.Aircraft
		if f.Departure.Airport != "" || f.Arrival.Airport != "" {
			r.Route = f.Departure.Airport + "→" + f.Arrival.Airport
		}
	}
	return r
}

func passengerSeatSet(trip *TripResult, flightIdx int) map[string]bool {
	m := make(map[string]bool)
	if trip == nil || flightIdx <= 0 || flightIdx > len(trip.Flights) {
		return m
	}
	for _, pax := range trip.Flights[flightIdx-1].Passengers {
		if pax.Seat != "--" && pax.Seat != "" {
			m[strings.ToUpper(pax.Seat)] = true
		}
	}
	return m
}

func tallySeat(r *SeatMapResult, status string) {
	switch status {
	case "available":
		r.AvailableSeats++
	case "occupied":
		r.OccupiedSeats++
	case "blocked":
		r.BlockedSeats++
	}
}

// parseSeatNum splits "15C" into (15, "C"). Returns (0,"") on invalid input.
func parseSeatNum(s string) (row int, col string) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if len(s) < 2 {
		return 0, ""
	}
	last := string(s[len(s)-1])
	if last < "A" || last > "K" {
		return 0, ""
	}
	n, err := strconv.Atoi(s[:len(s)-1])
	if err != nil || n < 1 || n > 99 {
		return 0, ""
	}
	return n, last
}

func seatColumnType(col string) string {
	switch col {
	case "A", "F":
		return "window"
	case "C", "D":
		return "aisle"
	case "B", "E":
		return "middle"
	default:
		return ""
	}
}

// inferCabin is a rough row-number heuristic when the DOM provides no cabin info.
func inferCabin(row int) string {
	if row <= 4 {
		return "First Class"
	}
	return "Main Cabin"
}

func resolveCabinName(code, name, desc string) string {
	for _, s := range []string{code, name, desc} {
		switch strings.ToUpper(strings.TrimSpace(s)) {
		case "FIRST", "F", "FIRST_CLASS":
			return "First Class"
		case "BUSINESS", "J", "BUSINESS_CLASS", "DELTA_ONE", "DELTAONE":
			return "Business Class (Delta One)"
		case "COMFORT", "W", "COMFORT_PLUS", "DELTA_COMFORT", "DELTACOMFORT":
			return "Delta Comfort+"
		case "MAIN", "Y", "MAIN_CABIN", "ECONOMY", "COACH":
			return "Main Cabin"
		}
		if s != "" {
			return titleCase(strings.ReplaceAll(s, "_", " "))
		}
	}
	return "Main Cabin"
}

func resolveAPIStatus(seat map[string]interface{}, isPassengerSeat bool) string {
	if isPassengerSeat {
		return "your-seat"
	}
	for _, key := range []string{"status", "seatStatus", "availability"} {
		switch strings.ToUpper(apiStr(seat, key)) {
		case "AVAILABLE", "OPEN", "Y":
			return "available"
		case "OCCUPIED", "TAKEN", "BOOKED", "UNAVAILABLE", "N":
			return "occupied"
		case "BLOCKED", "RESTRICTED", "CREW":
			return "blocked"
		}
	}
	if avail, ok := seat["available"].(bool); ok {
		if avail {
			return "available"
		}
		return "occupied"
	}
	return "available"
}

func asBoolField(m map[string]interface{}, key string) bool {
	v, ok := m[key].(bool)
	return ok && v
}

// seatMapDOMScript extracts seat data from the rendered delta.com seat map.
// It tries multiple DOM patterns to handle different versions of the SPA.
const seatMapDOMScript = `() => {
	const seats = [];
	const seen = new Set();

	function getStatus(el) {
		const cls = (el.className || '').toLowerCase();
		const lbl = (el.getAttribute('aria-label') || '').toLowerCase();
		if (el.getAttribute('aria-selected') === 'true' || cls.includes('selected') || cls.includes('your-seat')) return 'your-seat';
		if (el.disabled || el.getAttribute('aria-disabled') === 'true') return 'occupied';
		if (cls.includes('occupied') || cls.includes('taken') || cls.includes('booked') ||
		    cls.includes('unavailable') || lbl.includes('occupied') || lbl.includes('unavailable') ||
		    lbl.includes('not available')) return 'occupied';
		if (cls.includes('blocked') || cls.includes('restricted') || cls.includes('crew') ||
		    cls.includes('closed') || lbl.includes('blocked') || lbl.includes('crew')) return 'blocked';
		return 'available';
	}

	function getCabin(el) {
		let node = el.parentElement;
		for (let i = 0; i < 15 && node; i++) {
			for (const src of [
				node.getAttribute('aria-label') || '',
				node.getAttribute('data-cabin') || '',
				node.getAttribute('data-class') || '',
				node.className || '',
			]) {
				const s = src.toLowerCase();
				if (s.includes('first')) return 'First Class';
				if (s.includes('one') || (s.includes('delta') && s.includes('business'))) return 'Business Class (Delta One)';
				if (s.includes('business')) return 'Business Class (Delta One)';
				if (s.includes('comfort')) return 'Delta Comfort+';
				if (s.includes('main') || s.includes('economy') || s.includes('coach')) return 'Main Cabin';
			}
			const hdr = node.querySelector('h2,h3,h4,[class*="cabin-name"],[class*="cabinName"],[class*="cabin-header"]');
			if (hdr) {
				const t = (hdr.textContent || '').toLowerCase();
				if (t.includes('first')) return 'First Class';
				if (t.includes('one') || t.includes('business')) return 'Business Class (Delta One)';
				if (t.includes('comfort')) return 'Delta Comfort+';
				if (t.includes('main') || t.includes('economy')) return 'Main Cabin';
			}
			node = node.parentElement;
		}
		return '';
	}

	function addSeat(num, status, cabin, exitRow) {
		const n = num.toUpperCase().trim();
		if (!n || seen.has(n)) return;
		seen.add(n);
		seats.push({ number: n, status, cabin, exitRow: !!exitRow });
	}

	// Strategy 1: data attributes.
	for (const el of document.querySelectorAll('[data-seat-number],[data-seat-id],[data-seat],[data-seatnumber]')) {
		const raw = el.getAttribute('data-seat-number') || el.getAttribute('data-seat-id') ||
		            el.getAttribute('data-seat') || el.getAttribute('data-seatnumber') || '';
		const m = raw.match(/^(\d{1,2}[A-K])$/i);
		if (!m) continue;
		addSeat(m[1], getStatus(el), getCabin(el), (el.className||'').toLowerCase().includes('exit'));
	}

	// Strategy 2: aria-label containing a seat designator and status info.
	for (const el of document.querySelectorAll('[aria-label]')) {
		const lbl = el.getAttribute('aria-label') || '';
		const m = lbl.match(/\b(\d{1,2}[A-K])\b/i);
		if (!m) continue;
		addSeat(m[1], getStatus(el), getCabin(el), lbl.toLowerCase().includes('exit'));
	}

	// Strategy 3: elements whose trimmed text is exactly a seat designator.
	for (const el of document.querySelectorAll('div,span,td,li,button,g,rect,circle,text')) {
		if (el.children.length > 2) continue;
		const txt = (el.textContent || '').trim();
		if (!/^\d{1,2}[A-K]$/i.test(txt)) continue;
		addSeat(txt, getStatus(el), getCabin(el), false);
	}

	// Strategy 4: SVG title elements.
	for (const el of document.querySelectorAll('title')) {
		const txt = (el.textContent || '').trim();
		const m = txt.match(/\b(\d{1,2}[A-K])\b/i);
		if (!m) continue;
		const parent = el.parentElement;
		if (parent) addSeat(m[1], getStatus(parent), getCabin(parent), false);
	}

	// Try to extract route (two 3-letter IATA codes) from visible page text.
	// delta.com formats vary: "JAX → BOS", "JAX BOS", "Jacksonville (JAX) to Boston (BOS)".
	let dep = '', arr = '';
	const bodyText = document.body.innerText || '';
	const bodyLines = bodyText.split('\n').map(l=>l.trim()).filter(Boolean);
	// 1. Per-line patterns (inline pairs).
	for (const line of bodyLines.slice(0, 80)) {
		const m1 = line.match(/\b([A-Z]{3})\s*[→\-]\s*([A-Z]{3})\b/);
		if (m1) { dep = m1[1]; arr = m1[2]; break; }
		const m2 = line.match(/\(([A-Z]{3})\).*?\(([A-Z]{3})\)/);
		if (m2) { dep = m2[1]; arr = m2[2]; break; }
	}
	// 2. Full-body scan: find two (XXX) codes close together (route display).
	if (!dep) {
		const allCodes = [];
		const re = /\(([A-Z]{3})\)/g;
		let mm;
		while ((mm = re.exec(bodyText)) !== null) allCodes.push({code: mm[1], pos: mm.index});
		for (let i = 0; i < allCodes.length - 1; i++) {
			if (allCodes[i+1].pos - allCodes[i].pos < 200) {
				dep = allCodes[i].code; arr = allCodes[i+1].code; break;
			}
		}
	}

	return { seats, url: window.location.href, total: seats.length, dep, arr };
}`
