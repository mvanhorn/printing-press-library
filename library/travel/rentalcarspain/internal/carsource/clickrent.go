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
)

// ClickrentAPIBase is Clickrent's public booking backend.
const ClickrentAPIBase = "https://api.clickrent.es"

// clickrentBrand is the brand selector required on every business endpoint.
const clickrentBrand = "CLICKRENT"

// clickrentZeroExcessItem is the coverage-item id that marks a rate as
// zero-excess ("¡Sin Franquicia!" / Excess Reduction Coverage). A rate whose
// includedItems (walking any pack) contains this id has no excess.
const clickrentZeroExcessItem = 291

// Clickrent is a direct-supplier client for Clickrent. Pricing is a plain
// public JSON flow (no auth): resolve office by IATA → list car groups with
// per-rate prices → keep the rate whose coverage bundle includes the
// zero-excess item.
type Clickrent struct {
	APIBase string
	client  *http.Client
}

// NewClickrent builds a Clickrent client.
func NewClickrent(hc *http.Client) *Clickrent {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &Clickrent{APIBase: ClickrentAPIBase, client: hc}
}

func (c *Clickrent) base() string {
	if c.APIBase != "" {
		return c.APIBase
	}
	return ClickrentAPIBase
}

func (c *Clickrent) getJSON(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base()+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", "https://clickrent.es")
	req.Header.Set("Referer", "https://clickrent.es/")
	req.Header.Set("User-Agent", UserAgent)
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := httpStatusError(resp, "Clickrent"); err != nil {
		return err
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

type clickrentLocation struct {
	ID       int    `json:"id"`
	IataCode string `json:"iataCode"`
	Name     string `json:"name"`
	City     string `json:"city"`
}

// OfficeIDForAirport resolves Clickrent's office id for an IATA airport code.
// Returns 0 when Clickrent has no office there.
func (c *Clickrent) OfficeIDForAirport(ctx context.Context, iata string) (int, error) {
	var locs []clickrentLocation
	if err := c.getJSON(ctx, "/api/location/all-locations?brand="+clickrentBrand, &locs); err != nil {
		return 0, err
	}
	iata = strings.ToUpper(strings.TrimSpace(iata))
	for _, l := range locs {
		if strings.EqualFold(l.IataCode, iata) {
			return l.ID, nil
		}
	}
	return 0, fmt.Errorf("clickrent has no office for IATA %q", iata)
}

// clickrentItem is a coverage/inclusion item; packs nest more items.
type clickrentItem struct {
	ID       int             `json:"id"`
	IsPack   bool            `json:"isPack"`
	ItemsPack []clickrentItem `json:"itemsPack"`
}

// containsItem reports whether a set of included items (walking any packs)
// contains the given item id.
func clickrentContainsItem(items []clickrentItem, id int) bool {
	for _, it := range items {
		if it.ID == id {
			return true
		}
		if len(it.ItemsPack) > 0 && clickrentContainsItem(it.ItemsPack, id) {
			return true
		}
	}
	return false
}

type clickrentRate struct {
	ID            int             `json:"id"`
	Name          string          `json:"name"`
	IncludedItems []clickrentItem `json:"includedItems"`
}

type clickrentGroup struct {
	ID             int    `json:"id"`
	CommercialName string `json:"commercialName"`
	Name           string `json:"name"`
	AcrissCode     string `json:"acrissCode"`
	Seats          string `json:"seats"`
	Gears          string `json:"gears"`
	Doors          int    `json:"doors"`
	MinAge         int    `json:"minAge"`
	Available      bool   `json:"available"`
	Rates          []struct {
		ID     int     `json:"id"`
		Precio float64 `json:"precio"`
	} `json:"rates"`
}

// Quote fetches Clickrent's zero-excess prices for an airport IATA code and
// date range. Times default to 10:00. Clickrent bakes coverage into the rate:
// the fully-insured total is the price of the rate whose bundle includes the
// zero-excess item (291).
func (c *Clickrent) Quote(ctx context.Context, iata, pickup, dropoff, pickupTime, dropoffTime string) ([]Offer, error) {
	office, err := c.OfficeIDForAirport(ctx, iata)
	if err != nil {
		return nil, err
	}
	if pickupTime == "" {
		pickupTime = "10:00"
	}
	if dropoffTime == "" {
		dropoffTime = "10:00"
	}
	puDT, err := clickrentDateTime(pickup, pickupTime)
	if err != nil {
		return nil, fmt.Errorf("pickup: %w", err)
	}
	doDT, err := clickrentDateTime(dropoff, dropoffTime)
	if err != nil {
		return nil, fmt.Errorf("dropoff: %w", err)
	}
	q := url.Values{}
	q.Set("pickupDatetime", puDT)
	q.Set("dropoffDatetime", doDT)
	q.Set("pickupLocation", fmt.Sprintf("%d", office))
	q.Set("dropoffLocation", fmt.Sprintf("%d", office))
	q.Set("brand", clickrentBrand)

	var resp struct {
		Rates  []clickrentRate  `json:"rates"`
		Groups []clickrentGroup `json:"groups"`
	}
	if err := c.getJSON(ctx, "/api/bookings/groups?"+q.Encode(), &resp); err != nil {
		return nil, err
	}
	if len(resp.Groups) == 0 {
		return nil, fmt.Errorf("clickrent returned no cars (check dates/office)")
	}

	// Set of rate ids that are zero-excess (bundle contains item 291).
	zeroExcess := map[int]bool{}
	for _, r := range resp.Rates {
		if clickrentContainsItem(r.IncludedItems, clickrentZeroExcessItem) {
			zeroExcess[r.ID] = true
		}
	}

	var offers []Offer
	for _, g := range resp.Groups {
		if !g.Available {
			continue
		}
		// Cheapest zero-excess rate for this car.
		best := 0.0
		for _, r := range g.Rates {
			if !zeroExcess[r.ID] || r.Precio <= 0 {
				continue
			}
			if best == 0 || r.Precio < best {
				best = r.Precio
			}
		}
		if best <= 0 {
			continue
		}
		offers = append(offers, Offer{
			Source:        "clickrent",
			Supplier:      "Clickrent",
			SupplierCode:  "CLK",
			URL:           "https://clickrent.es",
			Car:           collapseWS(g.CommercialName),
			CarClass:      g.AcrissCode,
			Transmission:  clickrentTransmission(g.Gears),
			Doors:         g.Doors,
			MinAge:        g.MinAge,
			Total:         best,
			Currency:      "EUR",
			FullInsurance: true,
			Excess:        0,
			ExcessKnown:   true,
		})
	}
	if len(offers) == 0 {
		return nil, fmt.Errorf("clickrent returned cars but no zero-excess rate")
	}
	return offers, nil
}

func clickrentTransmission(gears string) string {
	switch strings.ToUpper(strings.TrimSpace(gears)) {
	case "A":
		return "Automatic"
	case "M":
		return "Manual"
	}
	return ""
}

// clickrentDateTime formats a date + HH:MM as Clickrent's "YYYY-MM-DD HH:MM".
func clickrentDateTime(date, hhmm string) (string, error) {
	d := isoToDMY(date)
	parts := strings.Split(d, "/")
	if len(parts) != 3 {
		return "", fmt.Errorf("date %q is not dd/mm/yyyy", date)
	}
	hh, mm := splitTime(hhmm)
	if len(hh) == 1 {
		hh = "0" + hh
	}
	return fmt.Sprintf("%s-%s-%s %s:%s", parts[2], parts[1], parts[0], hh, mm), nil
}
