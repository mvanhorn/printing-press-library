// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package carsource

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// RentalcarsBaseURL is the Rentalcars.com public web API host. Note: the
// identical API on cars.booking.com sits behind an AWS WAF challenge, so
// rentalcars.com is the replayable host.
const RentalcarsBaseURL = "https://www.rentalcars.com"

// RentalcarsMalagaIATA is Málaga Airport's location code in Rentalcars'
// system. Rentalcars searches by IATA directly (pickUpLocationType: IATA).
const RentalcarsMalagaIATA = "AGP"

// Rentalcars is an aggregator source client for Rentalcars.com. Its search is
// a plain public JSON GET (no auth, cookies, or WAF token on rentalcars.com).
type Rentalcars struct {
	BaseURL string
	client  *http.Client
}

// NewRentalcars builds a Rentalcars client.
func NewRentalcars(hc *http.Client) *Rentalcars {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &Rentalcars{BaseURL: RentalcarsBaseURL, client: hc}
}

func (r *Rentalcars) base() string {
	if r.BaseURL != "" {
		return r.BaseURL
	}
	return RentalcarsBaseURL
}

func (r *Rentalcars) getJSON(ctx context.Context, path string, params url.Values, v any) error {
	u := r.base() + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 202 {
		return fmt.Errorf("rentalcars returned a bot-challenge (202); this host requires a cleared browser")
	}
	if err := httpStatusError(resp, "Rentalcars"); err != nil {
		return err
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return err
	}
	return json.Unmarshal(body, v)
}

type rcLocation struct {
	IATA      string `json:"iata"`
	Name      string `json:"name"`
	Country   string `json:"country"`
	PlaceType string `json:"placeType"`
}

// ResolveLocation queries the location-suggestions endpoint for a place name,
// returning matches carrying the IATA code the search uses.
func (r *Rentalcars) ResolveLocation(ctx context.Context, query string) ([]Location, error) {
	params := url.Values{}
	params.Set("term", query)
	params.Set("language", "en")
	params.Set("cor", "gb")
	var raw []rcLocation
	if err := r.getJSON(ctx, "/api/location-suggestions", params, &raw); err != nil {
		return nil, err
	}
	var locs []Location
	for _, l := range raw {
		if l.IATA == "" {
			continue
		}
		locs = append(locs, Location{Code: l.IATA, Description: l.Name, Country: l.Country, IATA: l.IATA})
	}
	return locs, nil
}

type rcPrice struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

type rcSearchResponse struct {
	Matches []struct {
		Route struct {
			PickUpDepotID string `json:"pickUpDepotId"`
		} `json:"route"`
		Vehicle struct {
			Price        rcPrice `json:"price"`
			MakeAndModel string  `json:"makeAndModel"`
			CarClass     string  `json:"carClass"`
			Transmission string  `json:"transmission"`
			MileageType  string  `json:"mileageType"`
			Fuel         string  `json:"fuel"`
			NumberOfSeats string `json:"numberOfSeats"`
			PayWhen      string  `json:"payWhen"`
		} `json:"vehicle"`
		Fees []struct {
			Name  string  `json:"name"`
			Price rcPrice `json:"price"`
		} `json:"fees"`
		Insurance struct {
			Products []struct {
				ProductReference string `json:"productReference"`
				PriceBreakdown   struct {
					Price rcPrice `json:"price"`
				} `json:"priceBreakdown"`
			} `json:"products"`
		} `json:"insurance"`
	} `json:"matches"`
	Depots    map[string]struct {
		SupplierID string    `json:"supplierId"`
		Rating     rcRating  `json:"rating"`
	} `json:"depots"`
	Suppliers map[string]struct {
		Name string `json:"name"`
	} `json:"suppliers"`
}

// rcRating is the per-depot (== per-supplier-at-airport) customer rating.
type rcRating struct {
	Average     float64 `json:"average"`
	NumRatings  int     `json:"numRatings"`
	Cleanliness float64 `json:"cleanliness"`
	Condition   float64 `json:"condition"`
	CollectTime float64 `json:"collectTime"`
	DropOffTime float64 `json:"dropOffTime"`
	Efficiency  float64 `json:"efficiency"`
	Locating    float64 `json:"locating"`
	ValForMoney float64 `json:"valForMoney"`
}

func (r rcRating) categories() map[string]float64 {
	if r.NumRatings == 0 && r.Average == 0 {
		return nil
	}
	return map[string]float64{
		"cleanliness":  r.Cleanliness,
		"condition":    r.Condition,
		"collectTime":  r.CollectTime,
		"dropOffTime":  r.DropOffTime,
		"efficiency":   r.Efficiency,
		"locating":     r.Locating,
		"valForMoney":  r.ValForMoney,
	}
}

// Search runs a Rentalcars search for an IATA location and date range and
// returns normalized offers. locationIATA defaults to Málaga Airport.
func (r *Rentalcars) Search(ctx context.Context, locationIATA, pickup, dropoff, pickupTime, dropoffTime string, driverAge int) ([]Offer, error) {
	if locationIATA == "" {
		locationIATA = RentalcarsMalagaIATA
	}
	if pickupTime == "" {
		pickupTime = "10:00"
	}
	if dropoffTime == "" {
		dropoffTime = "10:00"
	}
	if driverAge <= 0 {
		driverAge = 35
	}
	puDT, err := rcDateTime(pickup, pickupTime)
	if err != nil {
		return nil, fmt.Errorf("pickup: %w", err)
	}
	doDT, err := rcDateTime(dropoff, dropoffTime)
	if err != nil {
		return nil, fmt.Errorf("dropoff: %w", err)
	}
	criteria := map[string]any{
		"driversAge":         driverAge,
		"pickUpLocation":     locationIATA,
		"pickUpLocationType": "IATA",
		"pickUpDateTime":     puDT,
		"dropOffLocation":    locationIATA,
		"dropOffLocationType": "IATA",
		"dropOffDateTime":    doDT,
	}
	critJSON, _ := json.Marshal(criteria)
	params := url.Values{}
	params.Set("searchCriteria", string(critJSON))
	params.Set("filterCriteria", "{}")
	params.Set("serviceFeatures", "{}")
	params.Set("prefcurrency", "EUR")

	var resp rcSearchResponse
	if err := r.getJSON(ctx, "/api/search-results", params, &resp); err != nil {
		return nil, err
	}
	days := rentalDays(pickup, dropoff)
	offers := make([]Offer, 0, len(resp.Matches))
	for _, m := range resp.Matches {
		if m.Vehicle.Price.Amount <= 0 {
			continue
		}
		supplier := ""
		var rating rcRating
		if dep, ok := resp.Depots[m.Route.PickUpDepotID]; ok {
			rating = dep.Rating
			if s, ok := resp.Suppliers[dep.SupplierID]; ok {
				supplier = s.Name
			}
		}
		if supplier == "" {
			supplier = "Rentalcars"
		}
		o := Offer{
			Source:           "rentalcars",
			Supplier:         CanonicalSupplier(supplier),
			URL:              RentalcarsBaseURL,
			SupplierScore:    rating.Average,
			Reviews:          rating.NumRatings,
			RatingCategories: rating.categories(),
			Car:          m.Vehicle.MakeAndModel,
			CarClass:     m.Vehicle.CarClass,
			Transmission: rcTransmission(m.Vehicle.Transmission),
			Total:        m.Vehicle.Price.Amount,
			Currency:     defaultCurrency(m.Vehicle.Price.Currency),
			Seats:        parseInt(m.Vehicle.NumberOfSeats),
		}
		if days > 0 {
			o.PerDay = m.Vehicle.Price.Amount / float64(days)
		}
		if strings.EqualFold(m.Vehicle.MileageType, "UNLIMITED") {
			o.Mileage = "Unlimited Mileage"
		}
		// Excess from the DAMAGE_EXCESS fee.
		for _, f := range m.Fees {
			if f.Name == "DAMAGE_EXCESS" {
				o.Excess = f.Price.Amount
				o.ExcessKnown = true
				o.FullInsurance = f.Price.Amount == 0
				break
			}
		}
		// Full Protection (zero-excess) add-on, when present inline.
		for _, p := range m.Insurance.Products {
			if strings.Contains(strings.ToUpper(p.ProductReference), "FULL_INSURANCE") && p.PriceBreakdown.Price.Amount > 0 {
				o.FullInsuranceTotal = o.Total + p.PriceBreakdown.Price.Amount
				break
			}
		}
		offers = append(offers, o)
	}
	if len(offers) == 0 {
		return nil, fmt.Errorf("rentalcars returned no priceable offers (check dates/location)")
	}
	return offers, nil
}

// SupplierRating is a supplier's customer rating at an airport.
type SupplierRating struct {
	Supplier string
	Score    float64
	Reviews  int
}

var rcAriaRe = regexp.MustCompile(`(?i)(?:for|para) (.+?) (?:at|en) `)
var rcCountRe = regexp.MustCompile(`([\d,\.]+)`)

// SupplierRatingsAt fetches the fuller per-supplier rating set from the
// product-cards endpoint (discovered via HAR), which covers more companies than
// the depot ratings on the main search response — e.g. Goldcar, Hertz, Dollar.
// Best-effort: returns nil on any error.
func (r *Rentalcars) SupplierRatingsAt(ctx context.Context, locationIATA, pickup, dropoff, pickupTime, dropoffTime string, driverAge int) []SupplierRating {
	if locationIATA == "" {
		locationIATA = RentalcarsMalagaIATA
	}
	if driverAge <= 0 {
		driverAge = 35
	}
	puDT, err := rcDateTime(pickup, pickupTime)
	if err != nil {
		return nil
	}
	doDT, err := rcDateTime(dropoff, dropoffTime)
	if err != nil {
		return nil
	}
	crit := map[string]any{
		"driversAge": driverAge, "pickUpLocation": locationIATA, "pickUpLocationType": "IATA",
		"pickUpDateTime": puDT, "dropOffLocation": locationIATA, "dropOffLocationType": "IATA",
		"dropOffDateTime": doDT,
	}
	payload := map[string]any{}
	for k, v := range crit {
		payload[k] = v
	}
	payload["rentalDurationInDays"] = rentalDays(pickup, dropoff)
	pj, _ := json.Marshal(payload)
	seg := base64.RawURLEncoding.EncodeToString(pj)
	cj, _ := json.Marshal(crit)
	params := url.Values{}
	params.Set("searchCriteria", string(cj))
	params.Set("filterCriteria", "{}")
	params.Set("cor", "gb")
	params.Set("locale", "en-gb")

	// Parse card-by-card into json.RawMessage so one malformed card doesn't
	// break the whole array (the response mixes product cards with ads/banners).
	var cards []json.RawMessage
	if err := r.getJSON(ctx, "/api/search-results/core-components/product-cards/"+seg, params, &cards); err != nil {
		return nil
	}
	type rcCard struct {
		Props struct {
			Review struct {
				Score       string `json:"score"` // live API sends this as a string, e.g. "6.6"
				AriaLabel   string `json:"scoreAriaLabel"`
				ReviewCount string `json:"reviewCount"`
			} `json:"review"`
		} `json:"props"`
	}
	seen := map[string]SupplierRating{}
	for _, raw := range cards {
		var c rcCard
		if err := json.Unmarshal(raw, &c); err != nil {
			continue
		}
		rv := c.Props.Review
		score := parsePrice(rv.Score)
		if score <= 0 {
			continue
		}
		m := rcAriaRe.FindStringSubmatch(rv.AriaLabel)
		if m == nil {
			continue
		}
		name := CanonicalSupplier(strings.TrimSpace(m[1]))
		count := 0
		if cm := rcCountRe.FindString(rv.ReviewCount); cm != "" {
			count = parseInt(strings.ReplaceAll(strings.ReplaceAll(cm, ",", ""), ".", ""))
		}
		if cur, ok := seen[name]; !ok || count > cur.Reviews {
			seen[name] = SupplierRating{Supplier: name, Score: score, Reviews: count}
		}
	}
	out := make([]SupplierRating, 0, len(seen))
	for _, r := range seen {
		out = append(out, r)
	}
	return out
}

func rcTransmission(s string) string {
	switch strings.ToUpper(s) {
	case "AUTOMATIC", "AUTO":
		return "Automatic"
	case "MANUAL":
		return "Manual"
	}
	return ""
}

func defaultCurrency(c string) string {
	if c == "" {
		return "EUR"
	}
	return c
}

// rentalDays returns the whole-day rental length between two dates (dd/mm/yyyy
// or yyyy-mm-dd), minimum 1.
func rentalDays(pickup, dropoff string) int {
	p, err1 := time.Parse("02/01/2006", isoToDMY(pickup))
	d, err2 := time.Parse("02/01/2006", isoToDMY(dropoff))
	if err1 != nil || err2 != nil {
		return 0
	}
	days := int(d.Sub(p).Hours() / 24)
	if days < 1 {
		return 1
	}
	return days
}

// rcDateTime builds the YYYY-MM-DDTHH:MM:SS datetime Rentalcars expects.
func rcDateTime(date, hhmm string) (string, error) {
	d := isoToDMY(date)
	parts := strings.Split(d, "/")
	if len(parts) != 3 {
		return "", fmt.Errorf("date %q is not dd/mm/yyyy", date)
	}
	hh, mm := splitTime(hhmm)
	if len(hh) == 1 {
		hh = "0" + hh
	}
	return fmt.Sprintf("%s-%s-%sT%s:%s:00", parts[2], parts[1], parts[0], hh, mm), nil
}
