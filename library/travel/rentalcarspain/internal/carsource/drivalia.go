// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package carsource

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// DrivaliaAPIBase is Drivalia's public short-term rental backend.
const DrivaliaAPIBase = "https://digital-backend.drivalia.com/digital-backend/api/v1/short-term"

// DrivaliaTenantES is the tenant header value for Spain (required).
const DrivaliaTenantES = "drivalia_es"

// DrivaliaMalagaAirportID is Drivalia's office id for Málaga Airport (code AGP),
// from POST /search/get-location. A later multi-airport version can resolve it
// dynamically via ResolveDrivaliaLocation.
const DrivaliaMalagaAirportID = "1183004186"

// Drivalia is a direct-supplier client for Drivalia Rent. Pricing is a plain
// public JSON flow (no auth/cookies): find office → list vehicles → enrich each
// offer with the zero-excess ("C1") insurance ancillary.
type Drivalia struct {
	APIBase string
	Tenant  string
	client  *http.Client
}

// NewDrivalia builds a Drivalia client.
func NewDrivalia(hc *http.Client) *Drivalia {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &Drivalia{APIBase: DrivaliaAPIBase, Tenant: DrivaliaTenantES, client: hc}
}

func (d *Drivalia) base() string {
	if d.APIBase != "" {
		return d.APIBase
	}
	return DrivaliaAPIBase
}

func (d *Drivalia) tenant() string {
	if d.Tenant != "" {
		return d.Tenant
	}
	return DrivaliaTenantES
}

// drivaliaMeta is the request envelope the Drivalia web app sends. The
// language field is load-bearing: without it the API returns a different,
// wrong fleet with fallback model names (NBMR comes back as the literal
// "acriss", EBMR as "Lancia Y", etc.). Sending language "es-ES" yields the
// real Spanish fleet names that match www.drivalia.com.
func drivaliaMeta() map[string]any {
	return map[string]any{
		"language":   "es-ES",
		"frontendID": "CUSTOMER_PORTAL",
	}
}

func (d *Drivalia) postJSON(ctx context.Context, path string, body any, out any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.base()+path, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-Id", d.tenant())
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := httpStatusError(resp, "Drivalia"); err != nil {
		return err
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

type drivaliaLocation struct {
	Code      string `json:"code"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	Country   string `json:"country"`
	IsAirport bool   `json:"isAirport"`
}

// ResolveDrivaliaLocation returns offices whose code/name matches a query.
func (d *Drivalia) ResolveDrivaliaLocation(ctx context.Context, query string) ([]Location, error) {
	var out struct {
		Data []drivaliaLocation `json:"data"`
	}
	if err := d.postJSON(ctx, "/search/get-location", map[string]any{"meta": drivaliaMeta(), "data": map[string]any{"searchText": ""}}, &out); err != nil {
		return nil, err
	}
	q := strings.ToLower(strings.TrimSpace(query))
	var locs []Location
	for _, l := range out.Data {
		if q == "" || strings.Contains(strings.ToLower(l.Name), q) || strings.EqualFold(l.Code, query) {
			locs = append(locs, Location{Code: l.ID, Description: l.Name, Country: l.Country, IATA: l.Code})
		}
	}
	return locs, nil
}

// OfficeIDForAirport resolves Drivalia's office id for an airport IATA code
// (offices carry the IATA as their code). Returns "" when not found.
func (d *Drivalia) OfficeIDForAirport(ctx context.Context, iata string) (string, error) {
	var out struct {
		Data []drivaliaLocation `json:"data"`
	}
	if err := d.postJSON(ctx, "/search/get-location", map[string]any{"meta": drivaliaMeta(), "data": map[string]any{"searchText": ""}}, &out); err != nil {
		return "", err
	}
	iata = strings.ToUpper(strings.TrimSpace(iata))
	for _, l := range out.Data {
		if l.IsAirport && strings.EqualFold(l.Code, iata) {
			return l.ID, nil
		}
	}
	return "", fmt.Errorf("drivalia has no airport office for IATA %q", iata)
}

// drivaliaRate is one payment option for a vehicle. The live API names the
// PAY_NOW / PAY_ON_ARRIVAL discriminator "paymentTiming"; "type" is kept as a
// defensive fallback in case the field name varies.
type drivaliaRate struct {
	PaymentTiming string `json:"paymentTiming"` // PAY_NOW | PAY_ON_ARRIVAL
	Type          string `json:"type"`          // alternate name (fallback)
	Price         struct {
		Value int64 `json:"value"` // integer cents
	} `json:"price"`
}

type drivaliaVehicle struct {
	Acriss      string         `json:"acriss"`
	Description string         `json:"description"`
	OfferID     string         `json:"offerId"`
	Rates       []drivaliaRate `json:"rates"`
}

// payNowCents returns the PAY_NOW base rate in cents. PAY_NOW is not always the
// first rate (the API may list PAY_ON_ARRIVAL first, which is pricier), so match
// it explicitly on paymentTiming; fall back to the first rate only when no
// PAY_NOW option is present.
func (v drivaliaVehicle) payNowCents() int64 {
	for _, r := range v.Rates {
		if r.PaymentTiming == "PAY_NOW" || r.Type == "PAY_NOW" {
			return r.Price.Value
		}
	}
	if len(v.Rates) > 0 {
		return v.Rates[0].Price.Value
	}
	return 0
}

// Quote fetches Drivalia's own zero-excess prices for a Málaga window. Times
// default to 10:00; driverAge sets the 24+ flag. The fully-insured total is the
// PAY_NOW base rate plus the "C1" zero-excess insurance ancillary, plus any
// obligatory young/senior-driver surcharge Drivalia adds to the online total for
// under-24 / senior drivers — all in cents.
func (d *Drivalia) Quote(ctx context.Context, locationID, pickup, dropoff, pickupTime, dropoffTime string, driverAge int) ([]Offer, error) {
	if locationID == "" {
		locationID = DrivaliaMalagaAirportID
	}
	if pickupTime == "" {
		pickupTime = "10:00"
	}
	if dropoffTime == "" {
		dropoffTime = "10:00"
	}
	over24 := driverAge == 0 || driverAge >= 24
	puDT, err := drivaliaDateTime(pickup, pickupTime)
	if err != nil {
		return nil, fmt.Errorf("pickup: %w", err)
	}
	doDT, err := drivaliaDateTime(dropoff, dropoffTime)
	if err != nil {
		return nil, fmt.Errorf("dropoff: %w", err)
	}
	var vehResp struct {
		Code string            `json:"code"`
		Data []drivaliaVehicle `json:"data"`
	}
	reqBody := map[string]any{"meta": drivaliaMeta(), "data": map[string]any{
		"vehicleType":       "CAR",
		"pickupDate":        puDT,
		"dropoffDate":       doDT,
		"pickupLocationId":  locationID,
		"dropoffLocationId": locationID,
		"corporateAgreement": "",
		"over24DriverAge":   over24,
		"productCodes":      []string{"all"},
	}}
	if err := d.postJSON(ctx, "/search/get-vehicle-by-location", reqBody, &vehResp); err != nil {
		return nil, err
	}
	if len(vehResp.Data) == 0 {
		return nil, fmt.Errorf("drivalia returned no vehicles (check dates/office)")
	}

	// Enrich each offer with its zero-excess (C1) insurance price, in parallel.
	offers := make([]Offer, len(vehResp.Data))
	var wg sync.WaitGroup
	for i, v := range vehResp.Data {
		wg.Add(1)
		go func(i int, v drivaliaVehicle) {
			defer wg.Done()
			base := v.payNowCents()
			c1, ageExtra := d.enrichedOfferPricing(ctx, v.OfferID, over24)
			if base <= 0 || c1 < 0 {
				return
			}
			// c1 is the chosen zero-excess cover; ageExtra is any obligatory
			// young/senior-driver surcharge Drivalia adds to the online total.
			total := float64(base+c1+ageExtra) / 100.0
			offers[i] = Offer{
				Source:        "drivalia",
				Supplier:      "Drivalia",
				SupplierCode:  "DRV",
				URL:           "https://www.drivalia.com",
				Car:           v.Description,
				CarClass:      v.Acriss,
				Total:         total,
				Currency:      "EUR",
				FullInsurance: true,
				Excess:        0,
				ExcessKnown:   true,
			}
		}(i, v)
	}
	wg.Wait()

	out := make([]Offer, 0, len(offers))
	for _, o := range offers {
		if o.Total > 0 {
			out = append(out, o)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("drivalia returned vehicles but no priceable zero-excess offers")
	}
	return out, nil
}

// drivaliaAncillary is one add-on in an enriched-offer response. The API returns
// each ancillary once per underlying productCode (so codes repeat) — callers must
// dedupe by Code before summing.
type drivaliaAncillary struct {
	Code        string `json:"code"`
	Type        string `json:"type"`
	SubType     string `json:"subType"`
	Label       string `json:"label"`
	IsMandatory bool   `json:"isMandatory"`
	Price       struct {
		Value int64 `json:"value"`
	} `json:"price"`
}

// enrichedOfferPricing returns the zero-excess ("C1" Super Cover) insurance price
// and the sum of any obligatory non-insurance surcharge, both in cents. c1 is -1
// when it cannot be resolved (the caller drops the offer). The obligatory extra
// captures Drivalia's mandatory YOUNG DRIVER / SENIOR DRIVER fee: those
// ancillaries flip isMandatory=true when the sent over24DriverAge flag makes them
// apply, and Drivalia charges them in the online total — not at the counter
// (verified: FIAT 500, 7 days, under-24 → YOUNG DRIVER +€83.65). A standard-age
// driver has no mandatory extra, so the verified standard price is unchanged.
func (d *Drivalia) enrichedOfferPricing(ctx context.Context, offerID string, over24 bool) (c1 int64, mandatoryExtra int64) {
	c1 = -1
	if offerID == "" {
		return -1, 0
	}
	var out struct {
		Data struct {
			Ancillaries []drivaliaAncillary `json:"ancillaries"`
		} `json:"data"`
	}
	body := map[string]any{"meta": drivaliaMeta(), "data": map[string]any{"keyless": false, "offerId": offerID, "over24DriverAge": over24}}
	if err := d.postJSON(ctx, "/search/enriched-offer", body, &out); err != nil {
		return -1, 0
	}
	return drivaliaPricing(out.Data.Ancillaries)
}

// drivaliaPricing extracts the zero-excess C1 insurance price and the sum of
// obligatory non-insurance surcharges (cents) from an offer's ancillary list.
// c1 is -1 when absent. Each code is counted once: the API repeats every
// ancillary per productCode, so a naive sum would double-count.
func drivaliaPricing(ancillaries []drivaliaAncillary) (c1 int64, mandatoryExtra int64) {
	c1 = -1
	seen := map[string]bool{}
	for _, a := range ancillaries {
		if strings.EqualFold(a.Type, "Insurance") && strings.EqualFold(a.Code, "C1") {
			if c1 < 0 {
				c1 = a.Price.Value
			}
			continue
		}
		// An obligatory non-insurance fee (young/senior-driver surcharge) the API
		// added for this driver's age.
		if a.IsMandatory && !seen[a.Code] {
			seen[a.Code] = true
			mandatoryExtra += a.Price.Value
		}
	}
	return c1, mandatoryExtra
}

// drivaliaDateTime builds an ISO-8601 datetime with Spain's summer offset
// (+02:00) from a dd/mm/yyyy or yyyy-mm-dd date plus HH:MM time.
func drivaliaDateTime(date, hhmm string) (string, error) {
	d := isoToDMY(date)
	parts := strings.Split(d, "/")
	if len(parts) != 3 {
		return "", fmt.Errorf("date %q is not dd/mm/yyyy", date)
	}
	hh, mm := splitTime(hhmm)
	if len(hh) == 1 {
		hh = "0" + hh
	}
	// Compute the Europe/Madrid UTC offset for the date so winter (+01:00) and
	// summer (+02:00) are both correct.
	offset := madridOffset(parts[2], parts[1], parts[0])
	return fmt.Sprintf("%s-%s-%sT%s:%s:00.000%s", parts[2], parts[1], parts[0], hh, mm, offset), nil
}

// madridOffset returns "+02:00" during CEST (summer) or "+01:00" (winter) for a
// given yyyy, mm, dd. Falls back to +01:00 when the date can't be parsed.
func madridOffset(y, m, dd string) string {
	t, err := time.Parse("2006-01-02", fmt.Sprintf("%s-%s-%s", y, m, dd))
	if err != nil {
		return "+01:00"
	}
	if loc, err := time.LoadLocation("Europe/Madrid"); err == nil {
		_, off := t.In(loc).Zone()
		if off == 7200 {
			return "+02:00"
		}
		return "+01:00"
	}
	// Rough DST fallback: CEST runs late March–late October.
	if t.Month() >= time.April && t.Month() <= time.October {
		return "+02:00"
	}
	return "+01:00"
}
