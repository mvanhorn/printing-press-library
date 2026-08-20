// Package delta provides browser-based scraping for delta.com My Trips.
//
// Transport strategy:
//   - Load the legacy findPnr.action interstitial URL in a headed Chrome window.
//   - The page's built-in jQuery auto-submits the pre-filled form to the server.
//   - The server redirects to the trip-details React SPA.
//   - The SPA calls mytrips-api.delta.com/v1/mytrips/travelreservations.
//   - We intercept that XHR and parse its JSON; fall back to DOM scraping if needed.
package delta

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

const reservAPIFrag = "travelreservations"

// GetTrip navigates delta.com My Trips, fills the search form via JavaScript
// (Shadow DOM traversal), intercepts the travelreservations API response,
// and returns structured trip data.
func GetTrip(ctx context.Context, confirmationNo, firstName, lastName string) (*TripResult, error) {
	conf := strings.ToUpper(confirmationNo)
	first := strings.ToUpper(firstName)
	last := strings.ToUpper(lastName)

	browser, cleanup, err := launchBrowser()
	if err != nil {
		return nil, fmt.Errorf("launching browser: %w", err)
	}
	defer cleanup()

	// Do NOT bind the browser to ctx via browser.Context(ctx): that attaches CDP
	// event subscriptions to the context and causes premature teardown when the
	// deadline approaches, breaking the DOM-scrape fallback path.
	page, err := browser.Page(proto.TargetCreateTarget{URL: ""})
	if err != nil {
		return nil, fmt.Errorf("opening browser tab: %w", err)
	}

	// Stealth: suppress webdriver detection signals.
	if err := applyStealthScripts(page); err != nil {
		return nil, fmt.Errorf("stealth setup: %w", err)
	}

	// Intercept the travelreservations XHR before navigating.
	var (
		mu          sync.Mutex
		apiBody     []byte
		apiReceived = make(chan struct{})
		apiOnce     sync.Once
	)

	router := page.HijackRequests()
	router.MustAdd("*"+reservAPIFrag+"*", func(h *rod.Hijack) {
		// LoadResponse (non-panicking) handles context cancellation gracefully.
		if err := h.LoadResponse(&http.Client{}, true); err != nil {
			return
		}
		body := h.Response.Body()
		// Skip empty/preliminary responses (delta.com fires a `{}` prefetch).
		if len(strings.TrimSpace(body)) <= 10 {
			return // Skip empty {} prefetch responses
		}
		apiOnce.Do(func() {
			mu.Lock()
			apiBody = []byte(body)
			mu.Unlock()
			close(apiReceived)
		})
	})
	go router.Run()

	// Navigate to My Trips and submit the search form (shared with GetSeatMap).
	navigateAndSubmitSearch(page, conf, first, last)

	// Derive the XHR wait from the remaining context budget so the DOM fallback
	// fires before the deadline. navigateAndSubmitSearch takes ~10s, so a
	// hardcoded 60s timer would always lose to ctx.Done().
	var xhrWait time.Duration
	if deadline, ok := ctx.Deadline(); ok {
		if budget := time.Until(deadline) - 5*time.Second; budget > 0 {
			xhrWait = budget
		}
	}
	if xhrWait <= 0 {
		return scrapeTripFromDOM(page, conf)
	}

	select {
	case <-apiReceived:
		// Captured the XHR — use structured JSON data.
	case <-time.After(xhrWait):
		// XHR not captured within budget; fall back to DOM text scraping.
		return scrapeTripFromDOM(page, conf)
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	mu.Lock()
	body := apiBody
	mu.Unlock()

	result, err := parseTravelReservations(body, conf)
	if err != nil {
		// JSON parse failed; try DOM.
		return scrapeTripFromDOM(page, conf)
	}
	if len(result.Flights) == 0 {
		// API returned something but no flights — augment via DOM.
		domResult, domErr := scrapeTripFromDOM(page, conf)
		if domErr == nil && len(domResult.Flights) > 0 {
			return domResult, nil
		}
	}
	return result, nil
}

// navigateAndSubmitSearch loads delta.com My Trips, fills the
// confirmation/first/last search form (traversing Shadow DOM), and submits it.
// Errors are non-fatal: the SPA frequently still routes correctly even when an
// individual selector misses, so callers proceed and rely on the XHR wait.
func navigateAndSubmitSearch(page *rod.Page, conf, first, last string) {
	// Send the CDP Page.navigate command directly without waiting for loadEventFired.
	// page.Navigate() blocks until loadEventFired, which delta.com's SPA never
	// fires cleanly (continuous background polling), consuming the entire budget.
	// Sending the raw command returns as soon as Chrome acknowledges the start.
	_, _ = proto.PageNavigate{URL: "https://www.delta.com/my-trips/"}.Call(page)
	time.Sleep(10 * time.Second)

	// Fill the search form using JavaScript Shadow DOM traversal.
	// Delta's My Trips page wraps inputs in custom elements with shadow roots.
	_, _ = page.Eval(`(conf, first, last) => {
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
		return {conf: !!confEl, first: !!firstEl, last: !!lastEl, submit: !!submitEl};
	}`, conf, first, last)
}

// launchBrowser starts Chrome in headed mode (delta.com WAF blocks headless).
// The browser window closes automatically when the CLI finishes.
func launchBrowser() (*rod.Browser, func(), error) {
	l := launcher.New().
		Headless(false).
		Leakless(false). // avoid leakless helper binary (AV false-positive on Windows)
		Set("disable-blink-features", "AutomationControlled").
		Set("disable-infobars", "").
		Set("window-size", "1920,1080").
		Set("start-maximized", "").
		Delete("enable-automation")

	if path, ok := launcher.LookPath(); ok {
		l = l.Bin(path)
	}

	u, err := l.Launch()
	if err != nil {
		return nil, nil, fmt.Errorf("launch: %w", err)
	}

	browser := rod.New().ControlURL(u).MustConnect()
	cleanup := func() {
		browser.MustClose()
		l.Cleanup()
	}
	return browser, cleanup, nil
}

func applyStealthScripts(page *rod.Page) error {
	_, err := page.EvalOnNewDocument(`() => {
		Object.defineProperty(navigator, 'webdriver', { get: () => undefined });
		Object.defineProperty(navigator, 'plugins', { get: () => [1,2,3] });
		Object.defineProperty(navigator, 'languages', { get: () => ['en-US','en'] });
		window.chrome = { runtime: {} };
	}`)
	return err
}

// parseTravelReservations parses the JSON body from mytrips-api.delta.com.
// Schema: travelReservations[0].trips[0].segments[] for flights;
// travelReservations[0].passengers[] for per-passenger data.
func parseTravelReservations(body []byte, confirmationNo string) (*TripResult, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if errMsg, ok := raw["error"].(string); ok {
		return nil, fmt.Errorf("API error: %s", errMsg)
	}

	resArr, _ := raw["travelReservations"].([]interface{})
	if len(resArr) == 0 {
		return nil, fmt.Errorf("no travelReservations in response")
	}
	travelRes, _ := resArr[0].(map[string]interface{})
	if travelRes == nil {
		return nil, fmt.Errorf("invalid travelReservation format")
	}

	reservation, _ := travelRes["reservation"].(map[string]interface{})
	tripType, _ := reservation["tripType"].(string)
	ticketExp, _ := reservation["ticketExpirationDt"].(string)

	paxList, _ := travelRes["passengers"].([]interface{})

	tripsArr, _ := travelRes["trips"].([]interface{})
	if len(tripsArr) == 0 {
		return nil, fmt.Errorf("no trips in response")
	}
	trip0, _ := tripsArr[0].(map[string]interface{})
	if trip0 == nil {
		return nil, fmt.Errorf("invalid trip format")
	}

	// Destination from trip transport destination station.
	destCity, destCode := "", ""
	if td, ok := trip0["transportDestination"].(map[string]interface{}); ok {
		if st, ok := td["station"].(map[string]interface{}); ok {
			destCode, _ = st["code"].(string)
			if pa, ok := st["postalAddress"].(map[string]interface{}); ok {
				destCity, _ = pa["cityName"].(string)
			}
		}
	}

	result := &TripResult{
		ConfirmationNumber: confirmationNo,
		TripType:           titleCase(strings.ReplaceAll(tripType, "_", " ")),
		TicketExpiration:   ticketExp,
		PassengerCount:     len(paxList),
	}
	if destCity != "" && destCode != "" {
		result.Destination = destCity + " (" + destCode + ")"
	} else if destCode != "" {
		result.Destination = destCode
	}

	segments, _ := trip0["segments"].([]interface{})
	total := len(segments)

	for i, segRaw := range segments {
		seg, _ := segRaw.(map[string]interface{})
		if seg == nil {
			continue
		}
		segID := int(apiFloat(seg, "segmentId"))

		mktSeg, _ := seg["marketingSegment"].(map[string]interface{})
		carrierCode := apiStrNested(mktSeg, "carrier", 0, "code")
		opSuffix, _ := mktSeg["operationalSuffix"].(string)
		flightNum, _ := mktSeg["flightNum"].(string)
		fullFlight := opSuffix + flightNum

		operatedBy := ""
		if opCarriers, ok := seg["operatingCarrier"].([]interface{}); ok && len(opCarriers) > 0 {
			if oc, ok := opCarriers[0].(map[string]interface{}); ok {
				opCode, _ := oc["code"].(string)
				opName, _ := oc["name"].(string)
				if opCode != carrierCode {
					operatedBy = opName
				}
			}
		}

		legsArr, _ := seg["legs"].([]interface{})
		if len(legsArr) == 0 {
			continue
		}
		leg, _ := legsArr[0].(map[string]interface{})
		if leg == nil {
			continue
		}

		aircraft := apiStrNested(leg, "transportEquipment", -1, "name")
		duration := isoDurToHuman(apiStr(leg, "onAirDuration"))
		status := titleCase(strings.ReplaceAll(apiStr(leg, "status"), "_", " "))

		// Cabin class name from leg (used to build fare class display string).
		cabinName := ""
		if cc, ok := leg["cabinClass"].(map[string]interface{}); ok {
			cabinName, _ = cc["name"].(string)
		}

		dep := parseAPIStop(leg, "transportOrigin", "boardingTerminal")
		arr := parseAPIStop(leg, "transportDestination", "disembarkTerminal")

		var layover *Layover
		if i < total-1 {
			rawLayoverDur := apiStr(leg, "onGroundDuration")
			if layDur := isoDurToHuman(rawLayoverDur); layDur != "" {
				layMins := isoDurToMinutes(rawLayoverDur)
				isIntl, _ := seg["international"].(bool)
				// MCT thresholds: domestic 45/90 min, international 90/120 min.
				tightThresh, highThresh := 90, 45
				if isIntl {
					tightThresh, highThresh = 120, 90
				}
				risk := "OK"
				if layMins <= highThresh {
					risk = "HIGH"
				} else if layMins <= tightThresh {
					risk = "TIGHT"
				}
				layover = &Layover{
					Duration:      layDur,
					Airport:       arr.Airport,
					City:          arr.City,
					RiskLevel:     risk,
					RiskMinutes:   layMins,
					International: isIntl,
				}
			}
		}

		flight := &Flight{
			FlightIndex:  fmt.Sprintf("%d of %d", i+1, total),
			FlightNumber: fullFlight,
			CarrierCode:  carrierCode,
			Aircraft:     aircraft,
			OperatedBy:   operatedBy,
			Status:       status,
			Duration:     duration,
			Departure:    dep,
			Arrival:      arr,
			Layover:      layover,
			Passengers:   buildPassengers(paxList, segID, carrierCode, cabinName),
		}
		result.Flights = append(result.Flights, flight)
	}
	return result, nil
}

// buildPassengers assembles per-passenger records for one flight segment.
func buildPassengers(paxList []interface{}, segID int, carrierCode, cabinName string) []*PassengerSeg {
	var out []*PassengerSeg
	for _, paxRaw := range paxList {
		pax, _ := paxRaw.(map[string]interface{})
		if pax == nil {
			continue
		}
		given, _ := pax["givenNames"].(string)
		surname, _ := pax["surname"].(string)
		name := titleCase(given + " " + surname)

		eTicket := ""
		if tickets, ok := pax["tickets"].([]interface{}); ok && len(tickets) > 0 {
			if t, ok := tickets[0].(map[string]interface{}); ok {
				if num, _ := t["number"].(string); num != "" {
					eTicket = "#" + num
				}
			}
		}

		loyaltyTier := ""
		if loyals, ok := pax["loyaltyProgramAccounts"].([]interface{}); ok && len(loyals) > 0 {
			if l, ok := loyals[0].(map[string]interface{}); ok {
				tierMap := map[string]string{
					"FF": "SkyMiles Member",
					"SL": "Silver Medallion",
					"GL": "Gold Medallion",
					"PL": "Platinum Medallion",
					"DM": "Diamond Medallion",
				}
				if code, _ := l["tierLevelCode"].(string); code != "" {
					if t, ok := tierMap[code]; ok {
						loyaltyTier = t
					} else {
						loyaltyTier = code
					}
				}
			}
		}

		seat := "--"
		fareClassCode := ""
		if paxTrips, ok := pax["passengerTrips"].([]interface{}); ok && len(paxTrips) > 0 {
			if pt, ok := paxTrips[0].(map[string]interface{}); ok {
				if paxSegs, ok := pt["passengerSegments"].([]interface{}); ok {
					for _, psRaw := range paxSegs {
						ps, _ := psRaw.(map[string]interface{})
						if ps == nil || int(apiFloat(ps, "segmentId")) != segID {
							continue
						}
						if bc, ok := ps["bookedCabinClass"].(map[string]interface{}); ok {
							fareClassCode, _ = bc["code"].(string)
						}
						if paxLegs, ok := ps["passengerLegs"].([]interface{}); ok && len(paxLegs) > 0 {
							if pl, ok := paxLegs[0].(map[string]interface{}); ok {
								if sas, ok := pl["seatAssignments"].([]interface{}); ok && len(sas) > 0 {
									if sa, ok := sas[0].(map[string]interface{}); ok {
										if seatObj, ok := sa["seat"].(map[string]interface{}); ok {
											if n, _ := seatObj["number"].(string); n != "" {
												seat = n
											}
										}
									}
								}
							}
						}
						break
					}
				}
			}
		}

		fareClass := ""
		if cabinName != "" && fareClassCode != "" {
			prefix := ""
			if carrierCode == "DL" {
				prefix = "Delta "
			}
			fareClass = prefix + cabinName + " (" + fareClassCode + ")"
		}

		out = append(out, &PassengerSeg{
			Name:        name,
			Seat:        seat,
			FareClass:   fareClass,
			FareCode:    fareClassCode,
			ETicket:     eTicket,
			LoyaltyTier: loyaltyTier,
		})
	}
	return out
}

// parseAPIStop extracts a FlightStop from a leg's transportOrigin/Destination.
func parseAPIStop(leg map[string]interface{}, transportKey, terminalKey string) FlightStop {
	transport, _ := leg[transportKey].(map[string]interface{})
	if transport == nil {
		return FlightStop{}
	}
	station, _ := transport["station"].(map[string]interface{})
	code, _ := station["code"].(string)
	city := ""
	if pa, ok := station["postalAddress"].(map[string]interface{}); ok {
		cityName, _ := pa["cityName"].(string)
		subDiv := ""
		if cs, ok := pa["countrySubdivision"].(map[string]interface{}); ok {
			if sub, _ := cs["code"].(string); len(sub) == 2 {
				subDiv = sub
			}
		}
		if subDiv != "" {
			city = cityName + ", " + subDiv
		} else {
			country := ""
			if co, ok := pa["country"].(map[string]interface{}); ok {
				country, _ = co["name"].(string)
			}
			if country != "" {
				city = cityName + ", " + country
			} else {
				city = cityName
			}
		}
	}
	if code != "" {
		city = city + " (" + code + ")"
	}

	dtKey := map[string]string{
		"transportOrigin":      "scheduledDepartureLocalDateTime",
		"transportDestination": "scheduledArrivalLocalDateTime",
	}[transportKey]
	rawDT, _ := transport[dtKey].(string)
	date, timeStr := parseLocalDT(rawDT)

	terminal := ""
	if termObj, ok := transport[terminalKey].(map[string]interface{}); ok {
		terminal, _ = termObj["name"].(string)
	}

	return FlightStop{
		Time:     timeStr,
		Date:     date,
		Airport:  code,
		City:     city,
		Terminal: terminal,
	}
}

// --- Small helpers ---

func parseLocalDT(s string) (date, timeStr string) {
	if s == "" {
		return "", ""
	}
	for _, layout := range []string{"2006-01-02T15:04:05.0", "2006-01-02T15:04:05"} {
		t, err := time.Parse(layout, s)
		if err == nil {
			return t.Format("Mon, Jan 2"), t.Format("3:04 PM")
		}
	}
	return s, ""
}

func isoDurToMinutes(s string) int {
	s = strings.TrimPrefix(s, "PT")
	var h, m int
	if idx := strings.Index(s, "H"); idx >= 0 {
		fmt.Sscanf(s[:idx], "%d", &h)
		s = s[idx+1:]
	}
	if idx := strings.Index(s, "M"); idx >= 0 {
		fmt.Sscanf(s[:idx], "%d", &m)
	}
	return h*60 + m
}

func isoDurToHuman(s string) string {
	// "PT2H51M" → "2h 51m"
	s = strings.TrimPrefix(s, "PT")
	var h, m int
	if idx := strings.Index(s, "H"); idx >= 0 {
		fmt.Sscanf(s[:idx], "%d", &h)
		s = s[idx+1:]
	}
	if idx := strings.Index(s, "M"); idx >= 0 {
		fmt.Sscanf(s[:idx], "%d", &m)
	}
	if h > 0 && m > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	} else if h > 0 {
		return fmt.Sprintf("%dh", h)
	} else if m > 0 {
		return fmt.Sprintf("%dm", m)
	}
	return ""
}

func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) == 0 {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
	}
	return strings.Join(words, " ")
}

func apiStr(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	v, _ := m[key].(string)
	return v
}

func apiFloat(m map[string]interface{}, key string) float64 {
	if m == nil {
		return 0
	}
	v, _ := m[key].(float64)
	return v
}

// apiStrNested gets a string from nested path: obj[key].([]interface{})[idx][subKey]
// If idx is -1, obj[key] is treated as a map not an array.
func apiStrNested(m map[string]interface{}, key string, idx int, subKey string) string {
	if m == nil {
		return ""
	}
	val, ok := m[key]
	if !ok {
		return ""
	}
	if idx < 0 {
		if sub, ok := val.(map[string]interface{}); ok {
			return apiStr(sub, subKey)
		}
		return ""
	}
	arr, ok := val.([]interface{})
	if !ok || idx >= len(arr) {
		return ""
	}
	if sub, ok := arr[idx].(map[string]interface{}); ok {
		return apiStr(sub, subKey)
	}
	return ""
}

// scrapeTripFromDOM extracts trip data by parsing the rendered page text.
// Used when the travelreservations XHR is not captured within the timeout.
func scrapeTripFromDOM(page *rod.Page, confirmationNo string) (*TripResult, error) {
	// Give the SPA extra time to render.
	time.Sleep(3 * time.Second)

	// Expand flight details if the button is present.
	page.Eval(`() => {
		const btn = document.querySelector('#toggleFlightDetailsButton') ||
		            Array.from(document.querySelectorAll('[role="button"]')).find(b =>
		                b.textContent && b.textContent.includes('FLIGHT DETAILS'));
		if (btn) btn.click();
	}`)
	time.Sleep(1 * time.Second)

	res, err := page.Eval(`() => document.body.innerText`)
	if err != nil {
		return nil, fmt.Errorf("reading page text: %w", err)
	}
	pageText := res.Value.String()

	trip := parsePageText(pageText, confirmationNo)
	if len(trip.Flights) == 0 {
		return nil, fmt.Errorf("no flight data found — trip may not have loaded (page text length: %d chars)", len(pageText))
	}
	return trip, nil
}

// parsePageText parses the human-readable inner text of the trip details page.
func parsePageText(text, confirmationNo string) *TripResult {
	trip := &TripResult{ConfirmationNumber: confirmationNo}
	lines := strings.Split(text, "\n")

	var currentFlight *Flight
	var currentPax *PassengerSeg

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		if isFlightIndexMarker(line) {
			currentFlight = &Flight{FlightIndex: line}
			currentPax = nil
			trip.Flights = append(trip.Flights, currentFlight)
			continue
		}

		if currentFlight == nil {
			if strings.Contains(line, ",") && len(line) < 60 && trip.Destination == "" {
				trip.Destination = line
			}
			continue
		}

		if isFlightNumber(line) {
			currentFlight.FlightNumber = line
			if len(line) >= 2 {
				currentFlight.CarrierCode = line[:2]
			}
			continue
		}
		if strings.HasPrefix(line, "Operated by") {
			currentFlight.OperatedBy = strings.TrimSpace(strings.TrimPrefix(line, "Operated by"))
			continue
		}
		if strings.Contains(line, " ") && isAircraftType(line) {
			currentFlight.Aircraft = line
			continue
		}
		if strings.EqualFold(line, "on time") || strings.EqualFold(line, "delayed") || strings.EqualFold(line, "cancelled") {
			currentFlight.Status = line
			continue
		}
		if strings.Contains(line, "h ") && strings.HasSuffix(line, "m") && len(line) < 12 {
			currentFlight.Duration = line
			continue
		}
		if strings.EqualFold(line, "Depart") {
			if i+1 < len(lines) {
				currentFlight.Departure.Date = strings.TrimSpace(lines[i+1])
			}
			if i+2 < len(lines) {
				currentFlight.Departure.Time = strings.TrimSpace(lines[i+2])
			}
			if i+3 < len(lines) {
				city := strings.TrimSpace(lines[i+3])
				currentFlight.Departure.Airport = extractAirportCode(city)
				currentFlight.Departure.City = city
			}
			continue
		}
		if strings.EqualFold(line, "Arrive") {
			if i+1 < len(lines) {
				currentFlight.Arrival.Date = strings.TrimSpace(lines[i+1])
			}
			if i+2 < len(lines) {
				currentFlight.Arrival.Time = strings.TrimSpace(lines[i+2])
			}
			if i+3 < len(lines) {
				city := strings.TrimSpace(lines[i+3])
				currentFlight.Arrival.Airport = extractAirportCode(city)
				currentFlight.Arrival.City = city
			}
			continue
		}
		if strings.HasPrefix(line, "Terminal") {
			if currentFlight.Departure.Terminal == "" {
				currentFlight.Departure.Terminal = line
			} else if currentFlight.Arrival.Terminal == "" {
				currentFlight.Arrival.Terminal = line
			}
			continue
		}
		if strings.EqualFold(line, "Layover") && i+1 < len(lines) {
			durLine := strings.TrimSpace(lines[i+1])
			dur := strings.TrimSpace(strings.TrimPrefix(durLine, "|"))
			currentFlight.Layover = &Layover{Duration: dur}
			continue
		}
		if looksLikePassengerName(line) {
			currentPax = &PassengerSeg{Name: line}
			currentFlight.Passengers = append(currentFlight.Passengers, currentPax)
			continue
		}
		if currentPax != nil {
			if isSeatCode(line) {
				currentPax.Seat = line
				continue
			}
			if strings.Contains(line, "Classic") || strings.Contains(line, "Comfort") || strings.Contains(line, "Select") {
				currentPax.FareClass = line
				continue
			}
			if strings.HasPrefix(line, "eTicket:") {
				currentPax.ETicket = strings.TrimSpace(strings.TrimPrefix(line, "eTicket:"))
				continue
			}
			if strings.Contains(line, "SkyMiles") {
				currentPax.LoyaltyTier = line
				continue
			}
			if strings.HasPrefix(line, "Board in Zone") || strings.HasPrefix(line, "Zone ") {
				currentPax.BoardingZone = line
				continue
			}
		}
	}
	return trip
}

func isAircraftType(s string) bool {
	return strings.Contains(s, "Airbus") || strings.Contains(s, "Boeing") ||
		strings.Contains(s, "Embraer") || strings.Contains(s, "ERJ") ||
		strings.HasPrefix(s, "A3") || strings.HasPrefix(s, "B7")
}

func isFlightIndexMarker(s string) bool {
	parts := strings.Fields(s)
	return len(parts) == 3 && parts[1] == "of" && isDigitStr(parts[0]) && isDigitStr(parts[2])
}

func isDigitStr(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

func isFlightNumber(s string) bool {
	if len(s) < 4 || len(s) > 8 {
		return false
	}
	s = strings.ToUpper(s)
	if !('A' <= s[0] && s[0] <= 'Z') || !('A' <= s[1] && s[1] <= 'Z') {
		return false
	}
	for _, c := range s[2:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func extractAirportCode(city string) string {
	start := strings.LastIndex(city, "(")
	end := strings.LastIndex(city, ")")
	if start >= 0 && end > start {
		code := city[start+1 : end]
		if len(code) == 3 {
			return strings.ToUpper(code)
		}
	}
	return ""
}

func looksLikePassengerName(s string) bool {
	words := strings.Fields(s)
	if len(words) < 2 || len(words) > 5 {
		return false
	}
	for _, w := range words {
		if len(w) < 2 {
			return false
		}
		if !('A' <= w[0] && w[0] <= 'Z') {
			return false
		}
		for _, c := range w[1:] {
			if !('a' <= c && c <= 'z') {
				return false
			}
		}
	}
	return true
}

func isSeatCode(s string) bool {
	if s == "--" {
		return true
	}
	if len(s) < 2 || len(s) > 4 {
		return false
	}
	for i, c := range s {
		if i < len(s)-1 {
			if c < '0' || c > '9' {
				return false
			}
		} else {
			if c < 'A' || c > 'Z' {
				return false
			}
		}
	}
	return true
}
