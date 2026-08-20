// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package carsource

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// CentauroAPIBase is Centauro's public booking API host. The /api prefix is
// mandatory — paths without it 404.
const CentauroAPIBase = "https://api.centauro.net/api"

// CentauroMalagaAirportBranch is Centauro's office code for Málaga Airport
// (from GET /api/branch/getall/). A later multi-airport version can resolve
// codes dynamically via ResolveCentauroBranch.
const CentauroMalagaAirportBranch = "35"

// Centauro is a direct-supplier client for Centauro Rent a Car. Its booking
// API is a plain public JSON GET with no auth, token, or browser requirement.
type Centauro struct {
	APIBase string
	client  *http.Client
}

// NewCentauro builds a Centauro client.
func NewCentauro(hc *http.Client) *Centauro {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &Centauro{APIBase: CentauroAPIBase, client: hc}
}

func (c *Centauro) base() string {
	if c.APIBase != "" {
		return c.APIBase
	}
	return CentauroAPIBase
}

func (c *Centauro) getJSON(ctx context.Context, path string, params url.Values, v any) error {
	u := c.base() + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := httpStatusError(resp, "Centauro"); err != nil {
		return err
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	return json.Unmarshal(body, v)
}

type centauroBranch struct {
	Code          json.Number `json:"code"`
	Name          string      `json:"name"`
	City          string      `json:"city"`
	CanMakeBooking bool       `json:"canMakeBooking"`
}

// ResolveCentauroBranch returns branches whose name/city matches a query
// (case-insensitive substring). Useful for the future multi-airport version.
func (c *Centauro) ResolveCentauroBranch(ctx context.Context, query string) ([]Location, error) {
	var out struct {
		OK   bool             `json:"ok"`
		Data []centauroBranch `json:"data"`
	}
	if err := c.getJSON(ctx, "/branch/getall/", nil, &out); err != nil {
		return nil, err
	}
	q := strings.ToLower(strings.TrimSpace(query))
	var locs []Location
	for _, b := range out.Data {
		if !b.CanMakeBooking {
			continue
		}
		if q == "" || strings.Contains(strings.ToLower(b.Name), q) || strings.Contains(strings.ToLower(b.City), q) {
			locs = append(locs, Location{Code: b.Code.String(), Description: b.Name, Country: b.City})
		}
	}
	return locs, nil
}

// BranchCodeForAirport resolves Centauro's office code for an airport, matched
// by the airport name (e.g. "Alicante Airport"). Returns "" when not found.
func (c *Centauro) BranchCodeForAirport(ctx context.Context, airportName string) (string, error) {
	var out struct {
		OK   bool             `json:"ok"`
		Data []centauroBranch `json:"data"`
	}
	if err := c.getJSON(ctx, "/branch/getall/", nil, &out); err != nil {
		return "", err
	}
	// Reduce "Alicante Airport" to its leading city word for matching.
	city := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(airportName, " Airport"), " airport")))
	var fallback string
	for _, b := range out.Data {
		if !b.CanMakeBooking {
			continue
		}
		name := strings.ToLower(b.Name)
		if strings.Contains(name, city) {
			if strings.Contains(name, "airport") {
				return b.Code.String(), nil
			}
			if fallback == "" {
				fallback = b.Code.String()
			}
		}
	}
	if fallback != "" {
		return fallback, nil
	}
	return "", fmt.Errorf("centauro has no bookable office matching %q", airportName)
}

type centauroAvailability struct {
	OK   bool `json:"ok"`
	Data struct {
		Days                      int                   `json:"days"`
		VehicleGroupsAvailability []centauroVehicleGroup `json:"vehicleGroupsAvailability"`
	} `json:"data"`
	Errors []struct {
		Field   string `json:"field"`
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"errors"`
}

type centauroVehicleGroup struct {
	Code            string            `json:"code"`
	Name            string            `json:"name"`
	Amount          float64           `json:"amount"`
	AmountPerDay    float64           `json:"amountPerDay"`
	FranchiseAmount float64           `json:"franchiseAmount"`
	Available       bool              `json:"available"`
	// NoRates marks a group Centauro lists without a rental rate (amount 0,
	// no deposit). Its package prices are not a bookable total, so such groups
	// must be skipped — see bookableCentauroGroup.
	NoRates  bool              `json:"noRates"`
	Packages []centauroPackage `json:"packages"`
	// Services are the group's extra/supplement lines. Obligatory driver-age
	// surcharges live here (see centauroService and centauroMandatoryAgeSurcharge).
	Services []centauroService `json:"services"`
	// Discount is an auto-applied promotion carried live in the availability
	// response (e.g. the "Summer" offer, code W292625). Amount is the flat euro
	// sum the site subtracts from the package total at checkout, so the payable
	// price is packageAmount - Discount.Amount. Reading it live keeps quotes
	// honest: when a promo ends the API returns Amount 0 and the full rate is
	// quoted, with no staleness risk.
	Discount *centauroDiscount `json:"discount"`
}

// centauroDiscount is a group-level promotion. Only Amount (a flat euro sum) is
// safe to subtract from the package total: the percentage field is computed off
// the base rate, not the package, so it does not equal the package discount.
type centauroDiscount struct {
	Code   string  `json:"code"`
	Amount float64 `json:"amount"`
}

// bookableCentauroGroup reports whether a group is a real, priceable rental.
// Centauro returns "no-rates" shadow groups (codes like AS/BS/CS) alongside the
// real ones: they carry amount=0 and noRates=true, yet still expose a Premium
// package price. Reading that price yields a plausible-looking but fictional
// total that is identical at every branch — so require a genuine base rate.
func bookableCentauroGroup(g centauroVehicleGroup) bool {
	return g.Available && !g.NoRates && g.Amount > 0
}

type centauroPackage struct {
	Code         string  `json:"code"`
	Amount       float64 `json:"amount"`
	RetailAmount float64 `json:"retailAmount"`
	AmountPerDay float64 `json:"amountPerDay"`
}

// centauroService is one supplement line in a group's services array. Driver-age
// surcharges appear here: "Conductor joven" (code YD, under 25) and "Conductor
// senior" (code SD, 74+). Both codes are always listed, but the API marks the one
// the quoted birth date makes mandatory with MinimumQuantity >= 1 (and
// Choosable=false). Amount is the flat euro total for the rental (AmountPerDay ×
// days). Centauro adds these to the payable total online — they are NOT a
// counter-collected charge — so an honest all-in quote must include them.
type centauroService struct {
	Code            string  `json:"code"`
	Name            string  `json:"name"`
	Amount          float64 `json:"amount"`
	AmountPerDay    float64 `json:"amountPerDay"`
	MinimumQuantity int     `json:"minimumQuantity"`
	Choosable       bool    `json:"choosable"`
}

// centauroMandatoryAgeSurcharge sums the obligatory driver-age supplements the
// API attached to a group for the quoted birth date. Only YD (young, <25) and SD
// (senior, 74+) are considered, and only when the API made them mandatory
// (MinimumQuantity >= 1) — so a standard-age driver adds nothing and the verified
// standard price is unchanged. Verified against Centauro Málaga (AGP, 7 days):
// Conductor joven +€91, Conductor senior +€49.
func centauroMandatoryAgeSurcharge(g centauroVehicleGroup) float64 {
	total := 0.0
	for _, s := range g.Services {
		if s.MinimumQuantity < 1 {
			continue
		}
		if strings.EqualFold(s.Code, "YD") || strings.EqualFold(s.Code, "SD") {
			total += s.Amount
		}
	}
	return total
}

// Quote fetches Centauro's own zero-excess ("Premium") prices for a Málaga
// pickup/dropoff window. Times default to 10:00. driverAge sets the customer
// birth date the API expects. now must be the current time (for quoteDateTime).
func (c *Centauro) Quote(ctx context.Context, branchCode, pickup, dropoff, pickupTime, dropoffTime string, driverAge int, now time.Time) ([]Offer, error) {
	if branchCode == "" {
		branchCode = CentauroMalagaAirportBranch
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
	puDT, err := centauroDateTime(pickup, pickupTime)
	if err != nil {
		return nil, fmt.Errorf("pickup: %w", err)
	}
	doDT, err := centauroDateTime(dropoff, dropoffTime)
	if err != nil {
		return nil, fmt.Errorf("dropoff: %w", err)
	}
	// The API wants a birth date, not an age; use Jan 1 of the implied year.
	birthYear := now.Year() - driverAge
	params := url.Values{}
	params.Set("agencyCode", "CI")
	params.Set("pickUpBranchCode", branchCode)
	params.Set("dropOffBranchCode", branchCode)
	params.Set("pickUpDateTime", puDT)
	params.Set("dropOffDateTime", doDT)
	params.Set("customerBirthDate", fmt.Sprintf("%04d-01-01", birthYear))
	params.Set("creationMethod", "1")
	params.Set("locale", "es-ES")
	params.Set("quoteDateTime", now.UTC().Format("2006-01-02T15:04:05.000Z"))

	var avail centauroAvailability
	if err := c.getJSON(ctx, "/bookingAvailability/getAvailability", params, &avail); err != nil {
		return nil, err
	}
	if !avail.OK {
		if len(avail.Errors) > 0 {
			e := avail.Errors[0]
			return nil, fmt.Errorf("centauro: %s (%s)", e.Message, e.Field)
		}
		return nil, fmt.Errorf("centauro availability returned ok=false (check dates/branch)")
	}
	var offers []Offer
	for _, g := range avail.Data.VehicleGroupsAvailability {
		if !bookableCentauroGroup(g) {
			continue
		}
		premium := centauroNetPremium(g)
		if premium <= 0 {
			continue
		}
		// Obligatory driver-age surcharges (young/senior) are charged online, in
		// the payable total — add them so young/senior quotes are not understated.
		total := premium + centauroMandatoryAgeSurcharge(g)
		perDay := 0.0
		if avail.Data.Days > 0 {
			perDay = total / float64(avail.Data.Days)
		}
		offers = append(offers, Offer{
			Source:        "centauro",
			Supplier:      "Centauro",
			SupplierCode:  "CYR",
			Car:           g.Name,
			CarClass:      g.Code,
			PerDay:        perDay,
			Total:         total,
			Currency:      "EUR",
			FullInsurance: true, // Premium package = zero excess
			Excess:        0,
			ExcessKnown:   true,
			URL:           "https://www.centauro.net",
		})
	}
	return offers, nil
}

// centauroNetPremium returns the actually-payable zero-excess Premium total for
// a group: the Premium package price minus any auto-applied group discount. This
// is the number Centauro's checkout charges (e.g. Premium 240 - 25 promo = 215).
func centauroNetPremium(g centauroVehicleGroup) float64 {
	premium := centauroPremiumPrice(g)
	if premium <= 0 {
		return premium
	}
	if g.Discount != nil && g.Discount.Amount > 0 && g.Discount.Amount < premium {
		premium -= g.Discount.Amount
	}
	return premium
}

// centauroPremiumPrice returns the zero-excess Premium package list price for a
// group (before any promo), falling back to the group amount only when it is
// itself all-inclusive.
func centauroPremiumPrice(g centauroVehicleGroup) float64 {
	for _, p := range g.Packages {
		if strings.EqualFold(p.Code, "Premium") {
			if p.Amount > 0 {
				return p.Amount
			}
			return p.RetailAmount
		}
	}
	return 0
}

// centauroDateTime builds the local ISO datetime the API expects
// (2026-07-14T16:30:00) from a dd/mm/yyyy or yyyy-mm-dd date plus HH:MM time.
func centauroDateTime(date, hhmm string) (string, error) {
	d := isoToDMY(date) // normalize to dd/mm/yyyy
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
