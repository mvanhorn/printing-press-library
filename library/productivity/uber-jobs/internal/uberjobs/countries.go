// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package uberjobs

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// country is one ISO 3166-1 entry. Name is the spelling the Uber careers
// site uses where it lists the country; aliases cover other common forms.
type country struct {
	Name    string
	Alpha2  string
	Alpha3  string
	Aliases []string
}

var countryTable = []country{
	{"Afghanistan", "AF", "AFG", nil},
	{"Albania", "AL", "ALB", nil},
	{"Algeria", "DZ", "DZA", nil},
	{"Andorra", "AD", "AND", nil},
	{"Angola", "AO", "AGO", nil},
	{"Argentina", "AR", "ARG", nil},
	{"Armenia", "AM", "ARM", nil},
	{"Australia", "AU", "AUS", nil},
	{"Austria", "AT", "AUT", nil},
	{"Azerbaijan", "AZ", "AZE", nil},
	{"Bahamas", "BS", "BHS", []string{"The Bahamas"}},
	{"Bahrain", "BH", "BHR", nil},
	{"Bangladesh", "BD", "BGD", nil},
	{"Barbados", "BB", "BRB", nil},
	{"Belarus", "BY", "BLR", nil},
	{"Belgium", "BE", "BEL", nil},
	{"Belize", "BZ", "BLZ", nil},
	{"Benin", "BJ", "BEN", nil},
	{"Bhutan", "BT", "BTN", nil},
	{"Bolivia", "BO", "BOL", []string{"Bolivia, Plurinational State of"}},
	{"Bosnia and Herzegovina", "BA", "BIH", nil},
	{"Botswana", "BW", "BWA", nil},
	{"Brazil", "BR", "BRA", []string{"Brasil"}},
	{"Brunei", "BN", "BRN", []string{"Brunei Darussalam"}},
	{"Bulgaria", "BG", "BGR", nil},
	{"Burkina Faso", "BF", "BFA", nil},
	{"Cambodia", "KH", "KHM", nil},
	{"Cameroon", "CM", "CMR", nil},
	{"Canada", "CA", "CAN", nil},
	{"Chile", "CL", "CHL", nil},
	{"China", "CN", "CHN", []string{"Mainland China", "People's Republic of China"}},
	{"Colombia", "CO", "COL", nil},
	{"Costa Rica", "CR", "CRI", nil},
	{"Côte d'Ivoire", "CI", "CIV", []string{"Ivory Coast", "Cote d'Ivoire"}},
	{"Croatia", "HR", "HRV", nil},
	{"Cuba", "CU", "CUB", nil},
	{"Cyprus", "CY", "CYP", nil},
	{"Czechia", "CZ", "CZE", []string{"Czech Republic"}},
	{"Denmark", "DK", "DNK", nil},
	{"Dominican Republic", "DO", "DOM", nil},
	{"Ecuador", "EC", "ECU", nil},
	{"Egypt", "EG", "EGY", nil},
	{"El Salvador", "SV", "SLV", nil},
	{"Estonia", "EE", "EST", nil},
	{"Ethiopia", "ET", "ETH", nil},
	{"Fiji", "FJ", "FJI", nil},
	{"Finland", "FI", "FIN", nil},
	{"France", "FR", "FRA", nil},
	{"Georgia", "GE", "GEO", nil},
	{"Germany", "DE", "DEU", nil},
	{"Ghana", "GH", "GHA", nil},
	{"Greece", "GR", "GRC", nil},
	{"Guatemala", "GT", "GTM", nil},
	{"Honduras", "HN", "HND", nil},
	{"Hong Kong", "HK", "HKG", []string{"Hong Kong SAR", "Hong Kong SAR China"}},
	{"Hungary", "HU", "HUN", nil},
	{"Iceland", "IS", "ISL", nil},
	{"India", "IN", "IND", nil},
	{"Indonesia", "ID", "IDN", nil},
	{"Iraq", "IQ", "IRQ", nil},
	{"Ireland", "IE", "IRL", []string{"Republic of Ireland"}},
	{"Israel", "IL", "ISR", nil},
	{"Italy", "IT", "ITA", nil},
	{"Jamaica", "JM", "JAM", nil},
	{"Japan", "JP", "JPN", nil},
	{"Jordan", "JO", "JOR", nil},
	{"Kazakhstan", "KZ", "KAZ", nil},
	{"Kenya", "KE", "KEN", nil},
	{"Korea, Republic of", "KR", "KOR", []string{"South Korea", "Korea", "Republic of Korea"}},
	{"Kuwait", "KW", "KWT", nil},
	{"Kyrgyzstan", "KG", "KGZ", nil},
	{"Laos", "LA", "LAO", []string{"Lao People's Democratic Republic"}},
	{"Latvia", "LV", "LVA", nil},
	{"Lebanon", "LB", "LBN", nil},
	{"Libya", "LY", "LBY", nil},
	{"Liechtenstein", "LI", "LIE", nil},
	{"Lithuania", "LT", "LTU", nil},
	{"Luxembourg", "LU", "LUX", nil},
	{"Macao", "MO", "MAC", []string{"Macau"}},
	{"Madagascar", "MG", "MDG", nil},
	{"Malawi", "MW", "MWI", nil},
	{"Malaysia", "MY", "MYS", nil},
	{"Maldives", "MV", "MDV", nil},
	{"Malta", "MT", "MLT", nil},
	{"Mauritius", "MU", "MUS", nil},
	{"Mexico", "MX", "MEX", []string{"México"}},
	{"Moldova", "MD", "MDA", []string{"Moldova, Republic of"}},
	{"Monaco", "MC", "MCO", nil},
	{"Mongolia", "MN", "MNG", nil},
	{"Montenegro", "ME", "MNE", nil},
	{"Morocco", "MA", "MAR", nil},
	{"Mozambique", "MZ", "MOZ", nil},
	{"Myanmar", "MM", "MMR", []string{"Burma"}},
	{"Namibia", "NA", "NAM", nil},
	{"Nepal", "NP", "NPL", nil},
	{"Netherlands", "NL", "NLD", []string{"The Netherlands", "Holland"}},
	{"New Zealand", "NZ", "NZL", nil},
	{"Nicaragua", "NI", "NIC", nil},
	{"Nigeria", "NG", "NGA", nil},
	{"North Macedonia", "MK", "MKD", []string{"Macedonia"}},
	{"Norway", "NO", "NOR", nil},
	{"Oman", "OM", "OMN", nil},
	{"Pakistan", "PK", "PAK", nil},
	{"Panama", "PA", "PAN", nil},
	{"Paraguay", "PY", "PRY", nil},
	{"Peru", "PE", "PER", nil},
	{"Philippines", "PH", "PHL", []string{"The Philippines"}},
	{"Poland", "PL", "POL", nil},
	{"Portugal", "PT", "PRT", nil},
	{"Puerto Rico", "PR", "PRI", nil},
	{"Qatar", "QA", "QAT", nil},
	{"Romania", "RO", "ROU", nil},
	{"Russia", "RU", "RUS", []string{"Russian Federation"}},
	{"Rwanda", "RW", "RWA", nil},
	{"Saudi Arabia", "SA", "SAU", []string{"Kingdom of Saudi Arabia", "KSA"}},
	{"Senegal", "SN", "SEN", nil},
	{"Serbia", "RS", "SRB", nil},
	{"Singapore", "SG", "SGP", nil},
	{"Slovakia", "SK", "SVK", []string{"Slovak Republic"}},
	{"Slovenia", "SI", "SVN", nil},
	{"South Africa", "ZA", "ZAF", nil},
	{"Spain", "ES", "ESP", nil},
	{"Sri Lanka", "LK", "LKA", nil},
	{"Sweden", "SE", "SWE", nil},
	{"Switzerland", "CH", "CHE", nil},
	{"Taiwan", "TW", "TWN", []string{"Taiwan, Province of China", "Republic of China"}},
	{"Tanzania", "TZ", "TZA", []string{"Tanzania, United Republic of"}},
	{"Thailand", "TH", "THA", nil},
	{"Trinidad and Tobago", "TT", "TTO", nil},
	{"Tunisia", "TN", "TUN", nil},
	{"Türkiye", "TR", "TUR", []string{"Turkey", "Turkiye", "Republic of Türkiye"}},
	{"Uganda", "UG", "UGA", nil},
	{"Ukraine", "UA", "UKR", nil},
	{"United Arab Emirates", "AE", "ARE", []string{"UAE"}},
	{"United Kingdom", "GB", "GBR", []string{"UK", "Great Britain", "Britain", "England", "Scotland", "Wales", "Northern Ireland", "United Kingdom of Great Britain and Northern Ireland"}},
	{"United States", "US", "USA", []string{"United States of America", "USA", "US", "America"}},
	{"Uruguay", "UY", "URY", nil},
	{"Uzbekistan", "UZ", "UZB", nil},
	{"Venezuela", "VE", "VEN", []string{"Venezuela, Bolivarian Republic of"}},
	{"Vietnam", "VN", "VNM", []string{"Viet Nam"}},
	{"Zambia", "ZM", "ZMB", nil},
	{"Zimbabwe", "ZW", "ZWE", nil},
}

var (
	byAlpha3 = map[string]country{}
	byAlpha2 = map[string]country{}
	byName   = map[string]country{}
)

func init() {
	for _, c := range countryTable {
		byAlpha3[c.Alpha3] = c
		byAlpha2[c.Alpha2] = c
		byName[nameKey(c.Name)] = c
		for _, a := range c.Aliases {
			byName[nameKey(a)] = c
		}
	}
}

// nameKey folds case, accents, and punctuation so "Türkiye", "turkiye", and
// "Korea, Republic of" match their table spellings.
func nameKey(s string) string {
	decomposed := norm.NFD.String(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range decomposed {
		switch {
		case unicode.Is(unicode.Mn, r):
			continue
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
		case unicode.IsSpace(r):
			b.WriteByte(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// CountryNameToISO3 maps a site country name (or alias) to ISO alpha-3.
func CountryNameToISO3(name string) (string, bool) {
	c, ok := byName[nameKey(name)]
	if !ok {
		return "", false
	}
	return c.Alpha3, true
}

// ToISO3 maps an ISO alpha-2 or alpha-3 code (any case) to alpha-3; unknown
// codes come back upper-cased so callers can report them.
func ToISO3(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if c, ok := byAlpha3[code]; ok {
		return c.Alpha3
	}
	if c, ok := byAlpha2[code]; ok {
		return c.Alpha3
	}
	return code
}

// ResolveCountry turns user input (ISO3, ISO2, or a country name) into the
// ISO3 code and the name the careers site uses for its countries filter.
func ResolveCountry(input string) (iso3, siteName string, ok bool) {
	in := strings.TrimSpace(input)
	if in == "" {
		return "", "", false
	}
	up := strings.ToUpper(in)
	if c, found := byAlpha3[up]; found && len(in) == 3 {
		return c.Alpha3, c.Name, true
	}
	if c, found := byAlpha2[up]; found && len(in) == 2 {
		return c.Alpha3, c.Name, true
	}
	if c, found := byName[nameKey(in)]; found {
		return c.Alpha3, c.Name, true
	}
	return "", "", false
}

// SiteNameForISO3 returns the careers-site spelling for an ISO3 code.
func SiteNameForISO3(iso3 string) (string, bool) {
	c, ok := byAlpha3[strings.ToUpper(strings.TrimSpace(iso3))]
	if !ok {
		return "", false
	}
	return c.Name, true
}
