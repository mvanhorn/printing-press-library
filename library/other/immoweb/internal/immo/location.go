package immo

import (
	"encoding/json"
	"fmt"
	"strings"
)

// LocationMatch is one autocomplete suggestion.
type LocationMatch struct {
	Group        string `json:"group"`
	Type         string `json:"type"`
	Label        string `json:"label"`
	MainLocality string `json:"main_locality"`
	QueryParam   string `json:"query_param"` // postalCodes, provinces, districts
	QueryValue   string `json:"query_value"`
}

// ParseAutocomplete flattens the /search/autocomplete response.
func ParseAutocomplete(raw json.RawMessage) ([]LocationMatch, error) {
	var groups []struct {
		Group   string `json:"group"`
		Results []struct {
			Type         string `json:"type"`
			Label        string `json:"label"`
			MainLocality string `json:"mainLocality"`
			QueryParam   string `json:"queryParam"`
			QueryValue   string `json:"queryValue"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &groups); err != nil {
		return nil, fmt.Errorf("decoding autocomplete: %w", err)
	}
	out := []LocationMatch{}
	for _, g := range groups {
		for _, r := range g.Results {
			out = append(out, LocationMatch{Group: g.Group, Type: r.Type, Label: r.Label, MainLocality: r.MainLocality, QueryParam: r.QueryParam, QueryValue: r.QueryValue})
		}
	}
	return out, nil
}

// PickCommune chooses the postal-code match for a commune name, preferring
// Immoweb's own "(all localities)" grouping, then an exact locality name,
// then the first postal-code suggestion. ok=false when nothing fits.
func PickCommune(name string, matches []LocationMatch) (LocationMatch, bool) {
	want := Fold(name)
	var exact, first *LocationMatch
	for i := range matches {
		m := &matches[i]
		if m.QueryParam != "postalCodes" {
			continue
		}
		label := Fold(m.Label)
		if strings.Contains(label, "all localities") || strings.Contains(label, "toutes localites") || strings.Contains(label, "alle deelgemeenten") {
			base := strings.TrimSpace(strings.Split(label, "(")[0])
			if base == want {
				return *m, true
			}
		}
		if exact == nil && (Fold(m.MainLocality) == want || strings.TrimSpace(strings.Split(label, "(")[0]) == want) {
			exact = m
		}
		if first == nil {
			first = m
		}
	}
	if exact != nil {
		return *exact, true
	}
	if first != nil && strings.HasPrefix(Fold(first.Label), want) {
		return *first, true
	}
	return LocationMatch{}, false
}

// PostalCodes splits a "BE-1000,BE-1050" query value.
func (m LocationMatch) PostalCodes() []string {
	return SplitCSV(m.QueryValue)
}

// brusselsCommunes lists the 19 Brussels-Capital communes (French and Dutch
// names) with the postal codes each one owns. Brussels postal codes cross
// commune borders, and Immoweb's "(all localities)" suggestions add them:
// "Ixelles (all localities)" is 1000+1050, so a commune median would mix in
// the whole City of Brussels. Market-level commands use this table instead.
var brusselsCommunes = []struct {
	names []string
	codes []string
}{
	{[]string{"bruxelles", "brussel", "brussels", "bruxelles ville", "ville de bruxelles", "stad brussel", "city of brussels"}, []string{"BE-1000", "BE-1020", "BE-1120", "BE-1130"}},
	{[]string{"schaerbeek", "schaarbeek"}, []string{"BE-1030"}},
	{[]string{"etterbeek"}, []string{"BE-1040"}},
	{[]string{"ixelles", "elsene"}, []string{"BE-1050"}},
	{[]string{"saint gilles", "sint gillis"}, []string{"BE-1060"}},
	{[]string{"anderlecht"}, []string{"BE-1070"}},
	{[]string{"molenbeek saint jean", "sint jans molenbeek", "molenbeek"}, []string{"BE-1080"}},
	{[]string{"koekelberg"}, []string{"BE-1081"}},
	{[]string{"berchem sainte agathe", "sint agatha berchem"}, []string{"BE-1082"}},
	{[]string{"ganshoren"}, []string{"BE-1083"}},
	{[]string{"jette"}, []string{"BE-1090"}},
	{[]string{"evere"}, []string{"BE-1140"}},
	{[]string{"woluwe saint pierre", "sint pieters woluwe"}, []string{"BE-1150"}},
	{[]string{"auderghem", "oudergem"}, []string{"BE-1160"}},
	{[]string{"watermael boitsfort", "watermaal bosvoorde"}, []string{"BE-1170"}},
	{[]string{"uccle", "ukkel"}, []string{"BE-1180"}},
	{[]string{"forest", "vorst"}, []string{"BE-1190"}},
	{[]string{"woluwe saint lambert", "sint lambrechts woluwe"}, []string{"BE-1200"}},
	{[]string{"saint josse ten noode", "sint joost ten node", "saint josse"}, []string{"BE-1210"}},
}

// BrusselsPostcodes returns the postal codes a Brussels-Capital commune owns,
// or nil when name is not one of the 19 communes.
func BrusselsPostcodes(name string) []string {
	want := Fold(name)
	for _, c := range brusselsCommunes {
		for _, n := range c.names {
			if n == want {
				return append([]string{}, c.codes...)
			}
		}
	}
	return nil
}

// BrusselsLabel renders "Ixelles (1050)" for a Brussels commune.
func BrusselsLabel(name string, codes []string) string {
	bare := make([]string, 0, len(codes))
	for _, c := range codes {
		bare = append(bare, BarePostalCode(c))
	}
	return communeTitle(name) + " (" + strings.Join(bare, ", ") + ")"
}

// communeParticles stay lowercase inside a commune name
// ("Saint-Josse-ten-Noode", "Ville de Bruxelles").
var communeParticles = map[string]bool{"ten": true, "de": true, "du": true, "des": true, "la": true, "le": true, "van": true, "der": true, "den": true}

// communeTitle capitalises every space- or hyphen-separated part of a
// commune name, so "saint-gilles" renders "Saint-Gilles" and
// "woluwe-saint-lambert" renders "Woluwe-Saint-Lambert".
func communeTitle(name string) string {
	var b strings.Builder
	var word []rune
	flush := func() {
		w := string(word)
		switch {
		case w == "":
		case b.Len() > 0 && communeParticles[strings.ToLower(w)]:
			b.WriteString(strings.ToLower(w))
		default:
			b.WriteString(strings.ToUpper(string(word[0])) + string(word[1:]))
		}
		word = word[:0]
	}
	for _, r := range strings.TrimSpace(name) {
		if r == ' ' || r == '-' {
			flush()
			b.WriteRune(r)
			continue
		}
		word = append(word, r)
	}
	flush()
	return b.String()
}
