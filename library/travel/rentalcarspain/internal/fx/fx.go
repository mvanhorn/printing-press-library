// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

// Package fx provides indicative EUR→currency conversion using the European
// Central Bank's free daily reference rates. EUR is the canonical settlement
// currency for Spanish rentals; conversions here are for display only.
package fx

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ECBDailyURL is the ECB's free daily euro foreign-exchange reference feed
// (no API key). Rates are quoted as units of the currency per 1 EUR.
const ECBDailyURL = "https://www.ecb.europa.eu/stats/eurofxref/eurofxref-daily.xml"

// Rates holds EUR-based reference rates: Rate[c] is how many units of currency
// c equal 1 EUR. EUR itself is always 1.
type Rates struct {
	Date string             // ECB publication date, e.g. "2026-07-14"
	Rate map[string]float64 // currency -> units per 1 EUR (includes "EUR": 1)
}

// Supported reports whether a (case-insensitive) currency code is one we can
// display. EUR is always supported; others require a fetched rate.
func (r Rates) Supported(code string) bool {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "EUR" {
		return true
	}
	_, ok := r.Rate[code]
	return ok
}

// Convert turns an amount in EUR into the target currency. ok is false when the
// currency is unknown; EUR passes through unchanged.
func (r Rates) Convert(amountEUR float64, code string) (float64, bool) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" || code == "EUR" {
		return amountEUR, true
	}
	rate, ok := r.Rate[code]
	if !ok || rate <= 0 {
		return 0, false
	}
	return amountEUR * rate, true
}

type ecbEnvelope struct {
	Cube struct {
		Day struct {
			Time  string `xml:"time,attr"`
			Rates []struct {
				Currency string  `xml:"currency,attr"`
				Rate     float64 `xml:"rate,attr"`
			} `xml:"Cube"`
		} `xml:"Cube"`
	} `xml:"Cube"`
}

// ParseECB parses the ECB daily XML into Rates.
func ParseECB(data []byte) (Rates, error) {
	var env ecbEnvelope
	if err := xml.Unmarshal(data, &env); err != nil {
		return Rates{}, fmt.Errorf("parsing ECB feed: %w", err)
	}
	out := Rates{Date: env.Cube.Day.Time, Rate: map[string]float64{"EUR": 1}}
	for _, c := range env.Cube.Day.Rates {
		if c.Currency != "" && c.Rate > 0 {
			out.Rate[strings.ToUpper(c.Currency)] = c.Rate
		}
	}
	if len(out.Rate) <= 1 {
		return Rates{}, fmt.Errorf("ECB feed contained no rates")
	}
	return out, nil
}

// FetchECB downloads and parses the ECB daily reference rates.
func FetchECB(ctx context.Context, hc *http.Client) (Rates, error) {
	if hc == nil {
		hc = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ECBDailyURL, nil)
	if err != nil {
		return Rates{}, err
	}
	req.Header.Set("Accept", "application/xml")
	resp, err := hc.Do(req)
	if err != nil {
		return Rates{}, fmt.Errorf("ecb fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return Rates{}, fmt.Errorf("ecb HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Rates{}, err
	}
	return ParseECB(data)
}
