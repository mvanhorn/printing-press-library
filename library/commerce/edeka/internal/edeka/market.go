package edeka

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// PortalURL is the edeka.shop portal that hosts the market finder.
const PortalURL = "https://edeka.shop"

// PlaceholderHost is the host the generated client uses for market-scoped
// endpoints; the client hook replaces it with the selected market's host.
const PlaceholderHost = "market.edeka.shop"

// ErrNoMarket means no market was chosen yet.
var ErrNoMarket = errors.New("no EDEKA market selected: run `markets search <postal-code>`, then `markets select <subdomain>` (or pass --market / set EDEKA_MARKET)")

var marketLabel = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

// NormalizeMarket turns "doerflinger-ulm", "doerflinger-ulm.edeka.shop" or a
// shop URL into the market host "doerflinger-ulm.edeka.shop".
func NormalizeMarket(in string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(in))
	if s == "" {
		return "", ErrNoMarket
	}
	if strings.Contains(s, "://") {
		u, err := url.Parse(s)
		if err != nil {
			return "", fmt.Errorf("invalid market %q: %w", in, err)
		}
		s = u.Hostname()
	}
	s = strings.TrimSuffix(strings.TrimSuffix(s, "/"), ".")
	s = strings.TrimSuffix(s, ".edeka.shop")
	if !marketLabel.MatchString(s) || s == "www" || s == "market" {
		return "", fmt.Errorf("invalid market %q: use the subdomain of the market's shop, for example doerflinger-ulm", in)
	}
	return s + ".edeka.shop", nil
}

// MarketSlug returns the subdomain part of a market host.
func MarketSlug(host string) string { return strings.TrimSuffix(host, ".edeka.shop") }

// statePath is the file holding the selected market when no session file is used.
func statePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "edeka-pp-cli", "market.txt"), nil
}

// ResolveMarket picks the market host in this order: explicit value (flag),
// EDEKA_MARKET, the session file, the saved selection, the only market the
// session has a login for.
func ResolveMarket(explicit string, s *Session) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		return NormalizeMarket(explicit)
	}
	if v := strings.TrimSpace(os.Getenv("EDEKA_MARKET")); v != "" {
		return NormalizeMarket(v)
	}
	if s != nil && s.Market != "" {
		return NormalizeMarket(s.Market)
	}
	if p, err := statePath(); err == nil {
		if b, err := os.ReadFile(filepath.Clean(p)); err == nil && strings.TrimSpace(string(b)) != "" {
			return NormalizeMarket(string(b))
		}
	}
	if s != nil {
		if ms := s.Markets(); len(ms) == 1 {
			return ms[0], nil
		}
	}
	return "", ErrNoMarket
}

// SaveSelectedMarket stores the selection for later commands.
func SaveSelectedMarket(host string) (string, error) {
	p, err := statePath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return "", err
	}
	return p, os.WriteFile(p, []byte(MarketSlug(host)+"\n"), 0o600)
}

// Market is one EDEKA online market from the portal's market finder.
type Market struct {
	Host          string   `json:"host"`
	Slug          string   `json:"slug"`
	Name          string   `json:"name"`
	Street        string   `json:"street"`
	Zip           string   `json:"zip"`
	City          string   `json:"city"`
	Phone         string   `json:"phone,omitempty"`
	Pickup        bool     `json:"pickup"`
	Delivery      bool     `json:"delivery"`
	Express       bool     `json:"express"`
	OpeningHours  string   `json:"opening_hours,omitempty"`
	DeliveryZips  []string `json:"delivery_zips,omitempty"`
	Lat           string   `json:"lat,omitempty"`
	Lng           string   `json:"lng,omitempty"`
	ShopURL       string   `json:"shop_url"`
	MatchedByCity string   `json:"matched_by_city,omitempty"`
	// Nearby is set when no market serves the searched postal code and this
	// market was found through the nearby postal code ServedZip instead.
	Nearby    bool   `json:"nearby,omitempty"`
	ServedZip string `json:"served_zip,omitempty"`
}

// RawMarket is the market finder's JSON row.
type RawMarket struct {
	Active       any    `json:"mxactive"`
	Domain       string `json:"mxdomain"`
	Title        string `json:"mxtitle"`
	Street       string `json:"mxstreet"`
	Zip          string `json:"mxzip"`
	City         string `json:"mxcity"`
	Phone        string `json:"mxfon"`
	Pickup       any    `json:"mxpickup"`
	Delivery     any    `json:"mxdelivery"`
	Express      any    `json:"mxexpress"`
	OpeningTime  string `json:"mxopeningtime"`
	OpeningSat   string `json:"mxopeningtimesa"`
	DeliveryZips string `json:"mxdeliveryplz"`
	Lat          string `json:"mxlat"`
	Lng          string `json:"mxlng"`
}

// City is a town row from the portal's town lookup.
type City struct {
	Name        string `json:"name"`
	Zipcode     string `json:"zipcode"`
	MarketCount int    `json:"marketcount"`
	Title       string `json:"title"`
}

func truthy(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case float64:
		return t != 0
	case string:
		return t != "" && t != "0"
	}
	return false
}

// ToMarket converts a finder row.
func (r RawMarket) ToMarket() Market {
	host := strings.ToLower(strings.TrimSpace(r.Domain))
	m := Market{Host: host, Slug: MarketSlug(host), Name: r.Title, Street: r.Street, Zip: r.Zip, City: r.City, Phone: r.Phone,
		Pickup: truthy(r.Pickup), Delivery: truthy(r.Delivery), Express: truthy(r.Express), Lat: r.Lat, Lng: r.Lng, ShopURL: "https://" + host + "/"}
	if r.OpeningTime != "" {
		m.OpeningHours = "Mon-Fri " + r.OpeningTime
		if r.OpeningSat != "" {
			m.OpeningHours += ", Sat " + r.OpeningSat
		}
	}
	for _, z := range strings.Split(r.DeliveryZips, ",") {
		if z = strings.TrimSpace(z); z != "" {
			m.DeliveryZips = append(m.DeliveryZips, z)
		}
	}
	return m
}

// IsZip reports whether s looks like a German postal code.
func IsZip(s string) bool { return regexp.MustCompile(`^\d{5}$`).MatchString(strings.TrimSpace(s)) }
