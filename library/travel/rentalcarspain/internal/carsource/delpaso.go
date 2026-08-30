// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package carsource

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"

	xhtml "golang.org/x/net/html"
)

// DelpasoBaseURL is the Delpaso Car Hire origin (Málaga only).
const DelpasoBaseURL = "https://www.delpasocarhire.com"

// Delpaso is the direct-supplier source client for Delpaso Car Hire. Delpaso
// serves a single Málaga location, so it takes no pickup-location or age
// parameter at the quote step. Prices already include total coverage / no
// excess.
type Delpaso struct {
	BaseURL string
	client  *http.Client
}

// NewDelpaso builds a Delpaso client.
func NewDelpaso(hc *http.Client) *Delpaso {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &Delpaso{BaseURL: DelpasoBaseURL, client: hc}
}

func (d *Delpaso) base() string {
	if d.BaseURL != "" {
		return d.BaseURL
	}
	return DelpasoBaseURL
}

var delpasoTokenRe = regexp.MustCompile(`name="_token"[^>]*value="([^"]*)"`)

// delpasoYoungDriver holds Delpaso's published "Conductor Joven" surcharge rule
// for young drivers (ages 21–24; 25+ is standard). Delpaso does not expose an age
// field in its quote flow, so the surcharge is not returned in the priced
// response — it is computed from these published values (€12/day, min €36, max
// €100), shown on the booking page. Update if Delpaso republishes the rate.
const (
	delpasoYoungDriverAgeMax = 25
	delpasoYoungDriverPerDay = 12.0
	delpasoYoungDriverMin    = 36.0
	delpasoYoungDriverMax    = 100.0
)

// delpasoYoungDriverSurcharge returns the obligatory "Conductor Joven" surcharge
// a young driver (under 25) pays over days rental days, computed from Delpaso's
// published €12/day rate capped to [€36, €100]. Returns 0 for standard-age or
// unspecified drivers. This is a published-rate figure, not a value read from the
// quote response (Delpaso takes no age at the quote step).
func delpasoYoungDriverSurcharge(age, days int) float64 {
	if age <= 0 || age >= delpasoYoungDriverAgeMax {
		return 0
	}
	if days < 1 {
		days = 1
	}
	fee := delpasoYoungDriverPerDay * float64(days)
	if fee < delpasoYoungDriverMin {
		fee = delpasoYoungDriverMin
	}
	if fee > delpasoYoungDriverMax {
		fee = delpasoYoungDriverMax
	}
	return fee
}

// delpasoRentalDays returns the whole-day rental length between two dates
// (dd/mm/yyyy or yyyy-mm-dd), minimum 1.
func delpasoRentalDays(pickup, dropoff string) int {
	p, err1 := time.Parse("02/01/2006", isoToDMY(pickup))
	d, err2 := time.Parse("02/01/2006", isoToDMY(dropoff))
	if err1 != nil || err2 != nil {
		return 1
	}
	days := int(d.Sub(p).Hours() / 24)
	if days < 1 {
		return 1
	}
	return days
}

// Quote fetches Delpaso's own prices for a Málaga pickup/dropoff window.
// pickup and dropoff accept dd/mm/yyyy or yyyy-mm-dd; times default to 10:00.
// driverAge, when under 25, adds Delpaso's published young-driver surcharge to
// each total (Delpaso does not price age online — see delpasoYoungDriverSurcharge).
func (d *Delpaso) Quote(ctx context.Context, pickup, dropoff, pickupTime, dropoffTime string, driverAge int) ([]Offer, error) {
	if pickupTime == "" {
		pickupTime = "10:00"
	}
	if dropoffTime == "" {
		dropoffTime = "10:00"
	}
	jar, _ := cookiejar.New(nil)
	hc := *d.client
	hc.Jar = jar
	base := d.base()

	// Step 1: GET homepage for the Laravel CSRF _token and session cookie.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("delpaso homepage: %w", err)
	}
	home, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()
	m := delpasoTokenRe.FindSubmatch(home)
	if m == nil {
		return nil, fmt.Errorf("could not find Delpaso CSRF token (site markup may have changed)")
	}
	token := string(m[1])

	// Step 2: POST /offers for the quote listing (server-rendered HTML).
	form := url.Values{}
	form.Set("_token", token)
	form.Set("pickup_date", isoToDMY(pickup))
	form.Set("pickup_time", pickupTime)
	form.Set("dropoff_date", isoToDMY(dropoff))
	form.Set("dropoff_time", dropoffTime)
	req2, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/offers", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req2.Header.Set("User-Agent", UserAgent)
	resp2, err := hc.Do(req2)
	if err != nil {
		return nil, fmt.Errorf("delpaso /offers: %w", err)
	}
	defer resp2.Body.Close()
	if err := httpStatusError(resp2, "Delpaso"); err != nil {
		return nil, err
	}
	doc, err := xhtml.Parse(io.LimitReader(resp2.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("parsing Delpaso HTML: %w", err)
	}
	offers := parseDelpasoOffers(doc)
	// Add the young-driver surcharge (a flat per-rental fee) to every offer, so
	// young-driver totals reflect Delpaso's true all-in cost.
	if surcharge := delpasoYoungDriverSurcharge(driverAge, delpasoRentalDays(pickup, dropoff)); surcharge > 0 {
		days := delpasoRentalDays(pickup, dropoff)
		for i := range offers {
			offers[i].Total += surcharge
			if days > 0 {
				offers[i].PerDay = offers[i].Total / float64(days)
			}
		}
	}
	return offers, nil
}

// parseDelpasoOffers extracts every car group from a Delpaso /offers document.
// Each group is a <div class="list-car <GROUP>"> block.
func parseDelpasoOffers(doc *xhtml.Node) []Offer {
	cars := findAll(doc, func(n *xhtml.Node) bool {
		return n.Data == "div" && hasClass(n, "list-car")
	})
	out := make([]Offer, 0, len(cars))
	for _, car := range cars {
		if o, ok := parseDelpasoCar(car); ok {
			out = append(out, o)
		}
	}
	return out
}

func parseDelpasoCar(car *xhtml.Node) (Offer, bool) {
	o := Offer{
		Source:        "delpaso",
		Supplier:      "Delpaso",
		SupplierCode:  "PAS",
		Currency:      "EUR",
		FullInsurance: true, // Delpaso quotes total coverage / no excess by default
		Excess:        0,
		ExcessKnown:   true,
	}
	o.URL = DelpasoBaseURL
	// data-trans="1" indicates automatic on Delpaso's markup.
	if attr(car, "data-trans") == "1" {
		o.Transmission = "Automatic"
	} else if attr(car, "data-trans") == "0" {
		o.Transmission = "Manual"
	}
	// Title: "Group A: FIAT 500 (or similar)".
	if t := firstWithClass(car, "title"); t != nil {
		full := textOf(t)
		o.Car = full
		if idx := strings.Index(full, ":"); idx >= 0 {
			o.CarClass = strings.TrimSpace(full[:idx])
			o.Car = strings.TrimSpace(full[idx+1:])
		}
	}
	// Prices.
	if p := firstWithClass(car, "price"); p != nil {
		o.PerDay = parsePrice(textOf(p))
	}
	if t := firstWithClass(car, "total"); t != nil {
		o.Total = parsePrice(textOf(t))
	}
	if disc := firstWithClass(car, "discount"); disc != nil {
		// Delpaso's .discount is the struck-through original per-day rate.
		if v := parsePrice(textOf(disc)); v > 0 {
			o.BaseTotal = v
		}
	}
	return o, o.Total > 0 || o.PerDay > 0
}
