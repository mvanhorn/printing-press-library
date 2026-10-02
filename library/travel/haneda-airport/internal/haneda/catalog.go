package haneda

import (
	"context"
	"fmt"
	"strings"
	"unicode"
)

type catalogItem struct {
	Text     string        `json:"text"`
	Value    string        `json:"value"`
	Code     string        `json:"airportCode"`
	Prefix   string        `json:"value02"`
	URL      string        `json:"url"`
	Children []catalogItem `json:"children"`
}
type rawCatalog map[string]struct {
	Items []catalogItem `json:"items"`
}

func catalogCode(kind string) string {
	if kind == "domestic" {
		return "dms"
	}
	return "int"
}
func fold(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, strings.TrimSpace(s))
}
func (c *Client) catalog(ctx context.Context, kind, entity string) (rawCatalog, error) {
	var d rawCatalog
	if err := c.request(ctx, "/site_resource/flight/data/"+catalogCode(kind)+"/"+entity+"_list_search.json", nil, &d); err != nil {
		return nil, err
	}
	if d["en"].Items == nil || d["ja"].Items == nil {
		return nil, fmt.Errorf("%s catalog has no bilingual items", entity)
	}
	return d, nil
}

// FetchAirports preserves both airportCode and the different provider city search value.
func (c *Client) FetchAirports(ctx context.Context, kind string) ([]Airport, error) {
	if kind != "domestic" && kind != "international" {
		return nil, fmt.Errorf("airport catalog kind must be domestic or international")
	}
	d, err := c.catalog(ctx, kind, "city")
	if err != nil {
		return nil, err
	}
	ja := map[string]string{}
	for _, region := range d["ja"].Items {
		for _, a := range region.Children {
			ja[a.Code] = a.Text
		}
	}
	out := []Airport{}
	for _, region := range d["en"].Items {
		for _, a := range region.Children {
			if a.Code == "" || a.Value == "" {
				return nil, fmt.Errorf("airport catalog has a missing stable/search code")
			}
			out = append(out, Airport{Code: a.Code, SearchValue: a.Value, Name: a.Text, NameJA: ja[a.Code], Region: region.Text, Kind: kind})
		}
	}
	return out, nil
}

// FetchAirlines keeps provider/ICAO codes separate from the flight-number prefix.
func (c *Client) FetchAirlines(ctx context.Context, kind string) ([]Airline, error) {
	if kind != "domestic" && kind != "international" {
		return nil, fmt.Errorf("airline catalog kind must be domestic or international")
	}
	d, err := c.catalog(ctx, kind, "company")
	if err != nil {
		return nil, err
	}
	ja := map[string]string{}
	for _, a := range d["ja"].Items {
		ja[a.Value] = a.Text
	}
	out := []Airline{}
	for _, a := range d["en"].Items {
		out = append(out, Airline{Code: a.Value, Prefix: a.Prefix, Name: a.Text, NameJA: ja[a.Value], URL: a.URL, Kind: kind})
	}
	return out, nil
}
func (c *Client) catalogs(ctx context.Context, kind string) ([]Airport, []Airline, error) {
	if c.airports != nil && c.airlines != nil {
		a, aok := c.airports[kind]
		l, lok := c.airlines[kind]
		if aok && lok {
			return a, l, nil
		}
	}
	a, err := c.FetchAirports(ctx, kind)
	if err != nil {
		return nil, nil, err
	}
	l, err := c.FetchAirlines(ctx, kind)
	if err != nil {
		return nil, nil, err
	}
	if c.airports == nil {
		c.airports = map[string][]Airport{}
		c.airlines = map[string][]Airline{}
	}
	c.airports[kind], c.airlines[kind] = a, l
	return a, l, nil
}
func findAirport(name string, airports []Airport) *Airport {
	for _, a := range airports {
		if fold(a.Name) == fold(name) || fold(a.Code) == fold(name) || fold(a.SearchValue) == fold(name) || fold(a.NameJA) == fold(name) {
			v := a
			return &v
		}
	}
	return nil
}
func findAirline(code string, airlines []Airline) Airline {
	for _, a := range airlines {
		if a.Code == strings.TrimSpace(code) {
			return a
		}
	}
	return Airline{Code: strings.TrimSpace(code)}
}
