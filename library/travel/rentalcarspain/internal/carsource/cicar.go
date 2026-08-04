// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package carsource

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// CicarBaseURL is CICAR's (Canary Islands) booking origin.
const CicarBaseURL = "https://www.cicar.com"

// cicarOffice maps an IATA airport to CICAR's zone/island/office identifiers.
// CICAR serves the Canary Islands; its rates are all-inclusive with zero excess
// (no franquicia), so the single price per car is already fully insured.
type cicarOffice struct {
	Zona   string // island-group code, e.g. TFE
	Isla   string // island name, e.g. Tenerife
	Office string // office code, e.g. T3
}

var cicarOffices = map[string]cicarOffice{
	"TFS": {Zona: "TFE", Isla: "Tenerife", Office: "T3"},        // Tenerife South
	"TFN": {Zona: "TFE", Isla: "Tenerife", Office: "T2"},        // Tenerife North
	"LPA": {Zona: "LPA", Isla: "Gran Canaria", Office: "G8"},    // Gran Canaria
	"ACE": {Zona: "ACE", Isla: "Lanzarote", Office: "L1"},       // Lanzarote
	"FUE": {Zona: "FUE", Isla: "Fuerteventura", Office: "F3"},   // Fuerteventura
}

// Cicar is a direct-supplier client for CICAR. Its stateless booking flow
// posts a search to /ES/action/booking2 and scrapes the returned car list;
// each car carries one all-inclusive (zero-excess) total price.
type Cicar struct {
	BaseURL string
	client  *http.Client
}

// NewCicar builds a CICAR client.
func NewCicar(hc *http.Client) *Cicar {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &Cicar{BaseURL: CicarBaseURL, client: hc}
}

func (c *Cicar) base() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	return CicarBaseURL
}

// CicarOfficeFor reports whether CICAR serves an IATA airport (Canary Islands).
func CicarOfficeFor(iata string) (cicarOffice, bool) {
	o, ok := cicarOffices[strings.ToUpper(strings.TrimSpace(iata))]
	return o, ok
}

// Each car card exposes ordered hidden spans: price → model id → group → name.
var cicarCarRe = regexp.MustCompile(`(?s)precioSinFormato"[^>]*>([^<]+)</span>\s*<span class="idModeloSeleccionado"[^>]*>([^<]+)</span>\s*<span class="grupoSeleccionado"[^>]*>([^<]+)</span>\s*<span class="nombreModeloSeleccionado"[^>]*>([^<]+)</span>`)

// Quote fetches CICAR's all-inclusive (zero-excess) prices for a Canary airport
// IATA code and date range. Times default to 10:00.
func (c *Cicar) Quote(ctx context.Context, iata, pickup, dropoff, pickupTime, dropoffTime string) ([]Offer, error) {
	office, ok := CicarOfficeFor(iata)
	if !ok {
		return nil, fmt.Errorf("CICAR has no office at %s (serves TFS, TFN, LPA, ACE, FUE)", strings.ToUpper(iata))
	}
	if pickupTime == "" {
		pickupTime = "10:00"
	}
	if dropoffTime == "" {
		dropoffTime = "10:00"
	}
	pDate := isoToDMY(pickup) // dd/mm/yyyy
	dDate := isoToDMY(dropoff)
	pHH, pMM := splitTime(pickupTime)
	dHH, dMM := splitTime(dropoffTime)

	form := url.Values{}
	form.Set("reservaOficinaEnt", office.Office)
	form.Set("reservaOficinaDev", office.Office)
	form.Set("horaminutoini", pickupTime)
	form.Set("horaminutofin", dropoffTime)
	form.Set("fecha_ini", pDate)
	form.Set("fecha_fin", dDate)
	form.Set("fromhome", "fromhome") // required guard flag
	form.Set("zona", office.Zona)
	form.Set("zonadev", office.Zona)
	form.Set("reservaIsla", office.Isla)
	form.Set("fechaIni", pDate)
	form.Set("horaIni", pHH)
	form.Set("minutoIni", pMM)
	form.Set("fechaFin", dDate)
	form.Set("horaFin", dHH)
	form.Set("minutoFin", dMM)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base()+"/ES/action/booking2", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", CicarBaseURL)
	req.Header.Set("Referer", CicarBaseURL+"/")
	req.Header.Set("User-Agent", UserAgent)
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cicar quote: %w", err)
	}
	defer resp.Body.Close()
	if err := httpStatusError(resp, "CICAR"); err != nil {
		return nil, err
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	var offers []Offer
	for _, m := range cicarCarRe.FindAllStringSubmatch(string(body), -1) {
		total := parsePrice(m[1])
		group := collapseWS(html.UnescapeString(m[3]))
		name := collapseWS(html.UnescapeString(m[4]))
		if total <= 0 || name == "" {
			continue
		}
		offers = append(offers, Offer{
			Source:        "cicar",
			Supplier:      "CICAR",
			SupplierCode:  "CIC",
			URL:           CicarBaseURL,
			Car:           name,
			CarClass:      "Group " + group,
			Total:         total,
			Currency:      "EUR",
			FullInsurance: true, // CICAR rates are all-inclusive, no franquicia
			Excess:        0,
			ExcessKnown:   true,
		})
	}
	if len(offers) == 0 {
		return nil, fmt.Errorf("cicar returned no priceable offers (check dates/office)")
	}
	return offers, nil
}
