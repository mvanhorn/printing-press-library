// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "strings"

// pp:novel-static-reference
var cityVenues = map[string][]string{
	"seattle": {
		"lumen-field",
		"tmobile-park",
		"climate-pledge-arena",
		"husky-stadiumwa",
		"alaska-airlines-arena",
		"moore-theatre",
		"benaroya-hall-taper-auditorium",
		"wamu-theater-seattle",
		"paramount-theatre-seattle",
		"showbox-sodo",
		"the-neptune-theatre",
	},
	"new york": {
		"madison-square-garden",
		"yankee-stadium",
		"citi-field",
		"barclays-center",
		"metlife-stadium",
	},
	"nyc": {
		"madison-square-garden",
		"yankee-stadium",
		"citi-field",
		"barclays-center",
		"metlife-stadium",
	},
	"los angeles": {
		"sofi-stadium",
		"crypto-com-arena",
		"dodger-stadium",
		"kia-forum",
		"intuit-dome",
	},
	"la": {
		"sofi-stadium",
		"crypto-com-arena",
		"dodger-stadium",
		"kia-forum",
		"intuit-dome",
	},
	"chicago": {
		"united-center",
		"wrigley-field",
		"soldier-field",
		"guaranteed-rate-field",
	},
	"boston": {
		"td-garden",
		"fenway-park",
		"gillette-stadium",
	},
}

func resolveCityVenues(city string) []string {
	normalized := normalizeCity(city)
	switch normalized {
	case "nyc":
		normalized = "new york"
	case "la":
		normalized = "los angeles"
	}
	venues, ok := cityVenues[normalized]
	if !ok {
		return nil
	}
	return venues
}

func normalizeCity(city string) string {
	return strings.ToLower(strings.TrimSpace(city))
}
