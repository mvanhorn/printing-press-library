package immovlan

import (
	"sort"
	"strings"
)

// brussels maps the 19 communes of the Brussels-Capital Region (and the
// sub-localities that own a postcode) to their postcodes. Immovlan accepts
// bare postcodes in `towns=` (it ignores `municipals=`), so this table lets
// commune names resolve without a network call.
var brussels = []struct {
	Names []string
	Codes []string
}{
	{[]string{"bruxelles", "brussels", "brussel", "bruxelles ville", "brussels city", "ville de bruxelles"}, []string{"1000", "1020", "1120", "1130"}},
	{[]string{"laeken", "laken"}, []string{"1020"}},
	{[]string{"neder over heembeek"}, []string{"1120"}},
	{[]string{"haren"}, []string{"1130"}},
	{[]string{"schaerbeek", "schaarbeek"}, []string{"1030"}},
	{[]string{"etterbeek"}, []string{"1040"}},
	{[]string{"ixelles", "elsene"}, []string{"1050"}},
	{[]string{"saint gilles", "sint gillis", "st gilles"}, []string{"1060"}},
	{[]string{"anderlecht"}, []string{"1070"}},
	{[]string{"molenbeek", "molenbeek saint jean", "sint jans molenbeek"}, []string{"1080"}},
	{[]string{"koekelberg"}, []string{"1081"}},
	{[]string{"berchem sainte agathe", "sint agatha berchem", "berchem"}, []string{"1082"}},
	{[]string{"ganshoren"}, []string{"1083"}},
	{[]string{"jette"}, []string{"1090"}},
	{[]string{"evere"}, []string{"1140"}},
	{[]string{"woluwe saint pierre", "sint pieters woluwe", "woluwe st pierre"}, []string{"1150"}},
	{[]string{"auderghem", "oudergem"}, []string{"1160"}},
	{[]string{"watermael boitsfort", "watermaal bosvoorde", "boitsfort"}, []string{"1170"}},
	{[]string{"uccle", "ukkel"}, []string{"1180"}},
	{[]string{"forest", "vorst"}, []string{"1190"}},
	{[]string{"woluwe saint lambert", "sint lambrechts woluwe", "woluwe st lambert"}, []string{"1200"}},
	{[]string{"saint josse", "saint josse ten noode", "sint joost", "sint joost ten node", "st josse"}, []string{"1210"}},
}

// BrusselsPostcodes returns the postcodes of a Brussels commune name in
// French or Dutch, or nil when the name is not a Brussels commune.
func BrusselsPostcodes(name string) []string {
	f := Fold(name)
	for _, b := range brussels {
		for _, n := range b.Names {
			if n == f {
				return append([]string(nil), b.Codes...)
			}
		}
	}
	return nil
}

// BrusselsCommune returns the French commune name for a Brussels postcode.
func BrusselsCommune(postcode string) string {
	for _, b := range brussels {
		for _, c := range b.Codes {
			if c == postcode {
				parts := strings.Fields(b.Names[0])
				for i, p := range parts {
					parts[i] = strings.ToUpper(p[:1]) + p[1:]
				}
				return strings.Join(parts, "-")
			}
		}
	}
	return ""
}

// BrusselsPostcodeList returns every postcode of the table, sorted, deduplicated.
func BrusselsPostcodeList() []string {
	seen := map[string]bool{}
	out := []string{}
	for _, e := range brussels {
		for _, pc := range e.Codes {
			if !seen[pc] {
				seen[pc] = true
				out = append(out, pc)
			}
		}
	}
	sort.Strings(out)
	return out
}
