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

// AutoreisenBaseURL is the Autoreisen (Canary Islands) booking origin.
const AutoreisenBaseURL = "https://www.autoreisen.com"

// autoreisenOffices maps IATA airport codes to Autoreisen office codes.
// Autoreisen serves the Canary Islands only.
var autoreisenOffices = map[string]string{
	"TFS": "2",  // Tenerife South
	"TFN": "1",  // Tenerife North
	"LPA": "18", // Gran Canaria (Las Palmas)
	"ACE": "3",  // Lanzarote
	"FUE": "19", // Fuerteventura
}

// Autoreisen is a direct-supplier client for Autoreisen Car Hire. Its base
// price already includes zero-excess all-risk insurance, so quotes are fully
// insured with no upgrade.
type Autoreisen struct {
	BaseURL string
	client  *http.Client
}

// NewAutoreisen builds an Autoreisen client.
func NewAutoreisen(hc *http.Client) *Autoreisen {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &Autoreisen{BaseURL: AutoreisenBaseURL, client: hc}
}

func (a *Autoreisen) base() string {
	if a.BaseURL != "" {
		return a.BaseURL
	}
	return AutoreisenBaseURL
}

// AutoreisenOfficeFor returns the Autoreisen office code for an IATA airport,
// or "" when Autoreisen does not serve it (it is Canary-Islands-only).
func AutoreisenOfficeFor(iata string) string {
	return autoreisenOffices[strings.ToUpper(strings.TrimSpace(iata))]
}

var autoreisenModelRe = regexp.MustCompile(`(?s)selec_modelo\(\d+,'([^']+)'.*?<span>\s*€\s*([0-9.,]+)\s*</span>`)

// Quote fetches Autoreisen's fully-insured (zero-excess) prices for a Canary
// airport IATA code and date range. Times default to 10:00.
func (a *Autoreisen) Quote(ctx context.Context, iata, pickup, dropoff, pickupTime, dropoffTime string) ([]Offer, error) {
	office := AutoreisenOfficeFor(iata)
	if office == "" {
		return nil, fmt.Errorf("Autoreisen has no office at %s (serves TFS, TFN, LPA, ACE, FUE)", strings.ToUpper(iata))
	}
	if pickupTime == "" {
		pickupTime = "10:00"
	}
	if dropoffTime == "" {
		dropoffTime = "10:00"
	}
	pd, pm, err := autoreisenDate(pickup)
	if err != nil {
		return nil, fmt.Errorf("pickup: %w", err)
	}
	dd, dm, err := autoreisenDate(dropoff)
	if err != nil {
		return nil, fmt.Errorf("dropoff: %w", err)
	}
	form := url.Values{}
	form.Set("ofi_rec", office)
	form.Set("ofi_dev", "9999") // same office
	form.Set("dia_inicio", pd)
	form.Set("mes_inicio", pm)
	form.Set("hora_inicio", pickupTime)
	form.Set("dia_final", dd)
	form.Set("mes_final", dm)
	form.Set("hora_final", dropoffTime)
	form.Set("dia_naci", "0")
	form.Set("mes_naci", "0")
	form.Set("ano_naci", "0")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.base()+"/car-hire/rates-fleet.php", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", UserAgent)
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("autoreisen quote: %w", err)
	}
	defer resp.Body.Close()
	if err := httpStatusError(resp, "Autoreisen"); err != nil {
		return nil, err
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	var offers []Offer
	for _, m := range autoreisenModelRe.FindAllStringSubmatch(string(body), -1) {
		name := collapseWS(html.UnescapeString(m[1]))
		// Names look like "A- Citroen C3 5P A/C" — strip the leading group letter.
		if i := strings.Index(name, "-"); i >= 0 && i <= 3 {
			name = strings.TrimSpace(name[i+1:])
		}
		total := parsePrice(m[2])
		if total <= 0 {
			continue
		}
		offers = append(offers, Offer{
			Source:        "autoreisen",
			Supplier:      "Autoreisen",
			SupplierCode:  "AUT",
			Car:           name,
			Total:         total,
			Currency:      "EUR",
			FullInsurance: true, // base price is zero-excess all-risk
			Excess:        0,
			ExcessKnown:   true,
			URL:           AutoreisenBaseURL,
		})
	}
	if len(offers) == 0 {
		return nil, fmt.Errorf("autoreisen returned no priceable offers (check dates/office)")
	}
	return offers, nil
}

// autoreisenDate splits a dd/mm/yyyy or yyyy-mm-dd date into the day and the
// "MM-YYYY" month string Autoreisen expects.
func autoreisenDate(date string) (day, monthYear string, err error) {
	d := isoToDMY(date) // dd/mm/yyyy
	parts := strings.Split(d, "/")
	if len(parts) != 3 {
		return "", "", fmt.Errorf("date %q is not dd/mm/yyyy", date)
	}
	return strings.TrimLeft(parts[0], "0"), fmt.Sprintf("%s-%s", parts[1], parts[2]), nil
}
