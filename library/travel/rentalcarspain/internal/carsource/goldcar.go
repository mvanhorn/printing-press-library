// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package carsource

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
)

// GoldcarBaseURL is Goldcar's booking origin.
const GoldcarBaseURL = "https://www.goldcar.es"

// Goldcar's PHP backend historically gated on a realistic browser UA plus the
// x-requested-with AJAX marker. We deliberately send the honest tool UA
// (carsource.UserAgent) instead of impersonating a browser: identifying the
// client truthfully is the acceptable-use line this project holds. If Goldcar
// blocks the honest UA, that is a "no" to be respected by disabling Goldcar —
// not a signal to escalate back to browser spoofing.

// Goldcar is a direct-supplier client for Goldcar. Pricing is a PHP-session
// JSON flow: resolve office by IATA → seed the session with the search → read
// the availability list. Each car's zero-excess ("a todo riesgo") price is the
// cheapest of its all-inclusive Pack* tariffs (PackPrime when offered, else
// PackKeyngo).
type Goldcar struct {
	BaseURL string
	client  *http.Client
}

// NewGoldcar builds a Goldcar client.
func NewGoldcar(hc *http.Client) *Goldcar {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &Goldcar{BaseURL: GoldcarBaseURL, client: hc}
}

func (g *Goldcar) base() string {
	if g.BaseURL != "" {
		return g.BaseURL
	}
	return GoldcarBaseURL
}

func (g *Goldcar) do(ctx context.Context, hc *http.Client, method, path string, body []byte) ([]byte, int, error) {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, g.base()+path, r)
	if err != nil {
		return nil, 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Origin", GoldcarBaseURL)
	req.Header.Set("Referer", GoldcarBaseURL+"/es-es/reservas/disponibilidad/")
	resp, err := hc.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	// Surface throttling as a typed error rather than a bare status the caller
	// would read as "no cars".
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		if rlErr := httpStatusError(resp, "Goldcar"); IsRateLimit(rlErr) {
			return nil, resp.StatusCode, rlErr
		}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	return data, resp.StatusCode, err
}

type goldcarTarifa struct {
	PrecioTotalf   float64 `json:"precio_totalf"`
	TieneCobertura bool    `json:"tiene_cobertura"`
	NombreTarifa   string  `json:"nombre_tarifa"`
}

type goldcarGroup struct {
	Grupo      string                   `json:"grupo"`
	Disponible bool                     `json:"disponible"`
	Detalle    struct {
		Descripcion string `json:"Descripcion"`
		Puertas     string `json:"Puertas"`
		Plazas      string `json:"Plazas"`
		Automatico  bool   `json:"Automatico"`
	} `json:"detalle"`
	Categoria struct {
		Nombre string `json:"Nombre"`
	} `json:"categoria"`
	Tarifas map[string]goldcarTarifa `json:"tarifas"`
}

// goldcarMinDriverAge is the minimum age Goldcar rents to: it declines drivers
// under 21 ("no podemos alquilar vehículos a conductores menores de 21 años").
// Drivers 21–24 rent at the same zero-excess price as standard ages (no
// young-driver surcharge on the zero-excess tier — verified: €231.80 at ages
// 20/22/35), so age affects eligibility here, not price.
const goldcarMinDriverAge = 21

// goldcarAgeBand maps a driver age to Goldcar's edadUsu band code — the value its
// booking API expects, NOT a literal age: 0 = 25+ (standard), 1 = 21–24. The site
// also exposes band 2 (19–20), but Goldcar declines those at airports like Málaga
// without ever pricing them, so all under-25 drivers use band 1. The zero-excess
// price is identical for bands 0 and 1 (verified €231.80 at ages 20/22/35), and
// the under-21 restriction is surfaced via Offer.MinAge rather than by suppressing
// the quote (flag-but-keep). Passing the literal age instead of a band happened to
// return the standard price, but sent a value the API never defines.
func goldcarAgeBand(age int) int {
	if age <= 0 || age >= 25 {
		return 0
	}
	return 1
}

// Quote fetches Goldcar's zero-excess prices for an airport IATA code and date
// range. Times default to 10:00; age defaults to 35.
func (g *Goldcar) Quote(ctx context.Context, iata, pickup, dropoff, pickupTime, dropoffTime string, age int) ([]Offer, error) {
	iata = strings.ToUpper(strings.TrimSpace(iata))
	if iata == "" {
		iata = "AGP"
	}
	if pickupTime == "" {
		pickupTime = "10:00"
	}
	if dropoffTime == "" {
		dropoffTime = "10:00"
	}
	if age <= 0 {
		age = 35
	}
	pDate, err := goldcarDate(pickup)
	if err != nil {
		return nil, fmt.Errorf("pickup: %w", err)
	}
	dDate, err := goldcarDate(dropoff)
	if err != nil {
		return nil, fmt.Errorf("dropoff: %w", err)
	}

	jar, _ := cookiejar.New(nil)
	hc := *g.client
	hc.Jar = jar

	// 1. Resolve office (also seeds the session cookie).
	_, status, err := g.do(ctx, &hc, http.MethodGet, "/api/v1/oficina/q/"+iata+"/es", nil)
	if err != nil {
		return nil, fmt.Errorf("goldcar office: %w", err)
	}
	if status != 200 {
		return nil, fmt.Errorf("goldcar has no office for IATA %q (HTTP %d)", iata, status)
	}

	// 2. Seed the search into the session.
	seed := map[string]any{
		"Lang": "es", "Agencia": false, "referer": "web",
		"pickupplace": iata, "pickupdate": pDate, "pickuptime": pickupTime, "edadUsu": goldcarAgeBand(age),
		"pickupplace_name": iata + " Airport", "pickupType": "1",
		"dropoffplace": iata, "dropoffdate": dDate, "dropofftime": dropoffTime,
		"busCortesia": 0, "zonaVenta": 1, "proveedor": "goldcar",
		"dropoffplace_name": iata + " Airport", "dropoffType": "1",
	}
	buf, _ := json.Marshal(seed)
	if _, status, err = g.do(ctx, &hc, http.MethodPost, "/api/v1/sesion", buf); err != nil {
		return nil, fmt.Errorf("goldcar session: %w", err)
	}

	// 3. Assert driver age + dates via the SPA hash checkUrl.
	checkURL := fmt.Sprintf("%s/es-es/reservas/disponibilidad/#pickupplace/%s/pickupdate/%s%%20%s/dropoffplace/%s/dropoffdate/%s%%20%s/userage/%d/",
		GoldcarBaseURL, iata, pDate, pickupTime, iata, dDate, dropoffTime, age)
	rules := map[string]any{
		"Lang": "es", "Agencia": false, "referer": "web",
		"extras_disponibilidad": []any{}, "extras_seleccionados": []any{},
		"actualizar_tests": "true", "reglas_disponibilidad": "true", "reglas_porcentajeDes": "true",
		"checkUrl": checkURL,
	}
	buf, _ = json.Marshal(rules)
	if _, _, err = g.do(ctx, &hc, http.MethodPost, "/api/v1/sesion", buf); err != nil {
		return nil, fmt.Errorf("goldcar session rules: %w", err)
	}

	// 4. Read the availability list.
	data, status, err := g.do(ctx, &hc, http.MethodGet, "/api/v1/disponibilidad", nil)
	if err != nil {
		return nil, fmt.Errorf("goldcar availability: %w", err)
	}
	if status != 200 {
		return nil, fmt.Errorf("goldcar availability HTTP %d", status)
	}
	var groups []goldcarGroup
	if err := json.Unmarshal(data, &groups); err != nil {
		return nil, fmt.Errorf("goldcar availability parse: %w", err)
	}

	var offers []Offer
	for _, gr := range groups {
		if !gr.Disponible {
			continue
		}
		price, rate := goldcarZeroExcess(gr.Tarifas)
		if price <= 0 {
			continue // no all-inclusive zero-excess tariff at this office
		}
		car := collapseWS(gr.Detalle.Descripcion)
		if car == "" {
			car = "Group " + gr.Grupo
		}
		trans := "Manual"
		if gr.Detalle.Automatico {
			trans = "Automatic"
		}
		offers = append(offers, Offer{
			Source:        "goldcar",
			Supplier:      "Goldcar",
			SupplierCode:  "GLD",
			URL:           GoldcarBaseURL,
			Car:           car,
			CarClass:      gr.Categoria.Nombre,
			Doors:         parseInt(gr.Detalle.Puertas),
			Seats:         parseInt(gr.Detalle.Plazas),
			Transmission:  trans,
			Total:         price,
			Currency:      "EUR",
			FullInsurance: true, // Pack* tariff = "a todo riesgo", zero excess
			Excess:        0,
			ExcessKnown:   true,
			FuelPolicy:    rate,
			MinAge:        goldcarMinDriverAge, // Goldcar declines drivers under 21
		})
	}
	if len(offers) == 0 {
		return nil, fmt.Errorf("goldcar returned no zero-excess offers (check dates/office)")
	}
	return offers, nil
}

// goldcarZeroExcess returns the cheapest all-inclusive (zero-excess) tariff
// price and its rate code from a group's tariff map. Pack* rates are Goldcar's
// "a todo riesgo" full-cover tiers; the cheapest is PackPrime when offered,
// else PackKeyngo. Returns 0 when no zero-excess tariff is present.
func goldcarZeroExcess(tarifas map[string]goldcarTarifa) (float64, string) {
	best := 0.0
	rate := ""
	for code, t := range tarifas {
		if !strings.HasPrefix(code, "Pack") || t.PrecioTotalf <= 0 {
			continue
		}
		if best == 0 || t.PrecioTotalf < best {
			best = t.PrecioTotalf
			rate = code
		}
	}
	return best, rate
}

// goldcarDate converts a dd/mm/yyyy or yyyy-mm-dd date to Goldcar's yyyy-mm-dd.
func goldcarDate(date string) (string, error) {
	d := isoToDMY(date) // dd/mm/yyyy
	parts := strings.Split(d, "/")
	if len(parts) != 3 {
		return "", fmt.Errorf("date %q is not dd/mm/yyyy", date)
	}
	return fmt.Sprintf("%s-%s-%s", parts[2], parts[1], parts[0]), nil
}
