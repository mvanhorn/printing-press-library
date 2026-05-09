// Copyright 2026 wayne-lai. Licensed under Apache-2.0. See LICENSE.

// Package fjud is an HTTP/HTML client for Taiwan's public judgment search
// site at judgment.judicial.gov.tw (FJUD = Full-text Judgment Database).
//
// It exposes search, single-judgment fetch, and PDF download against the
// site's ASP.NET WebForms surface using stdlib net/http only — no Selenium,
// no clearance cookies, no browser sidecar required.
package fjud

// Court is a single Taiwan court entry as listed on the FJUD search form's
// `jud_court` <select> element.
type Court struct {
	Code string `json:"code"` // 3-letter court code, e.g. "TPS"
	Name string `json:"name"` // Chinese name, e.g. "最高法院"
}

// Courts is the canonical 41-court list extracted at generation time from the
// FJUD search form. Maintained alongside the scraper so callers can resolve
// codes to display names without a network round-trip.
//
// Source: https://judgment.judicial.gov.tw/FJUD/Default_AD.aspx
var Courts = []Court{
	{"JCC", "憲法法庭"},
	{"TPC", "司法院刑事補償法庭"},
	{"TPU", "司法院－訴願決定"},
	{"TPS", "最高法院"},
	{"TPA", "最高行政法院(含改制前行政法院)"},
	{"TPP", "懲戒法院－懲戒法庭"},
	{"TPJ", "懲戒法院－職務法庭"},
	{"TPH", "臺灣高等法院"},
	{"001", "臺灣高等法院－訴願決定"},
	{"TPB", "臺北高等行政法院 高等庭(含改制前臺北高等行政法院)"},
	{"TPT", "臺北高等行政法院 地方庭"},
	{"TCB", "臺中高等行政法院 高等庭(含改制前臺中高等行政法院)"},
	{"TCT", "臺中高等行政法院 地方庭"},
	{"KSB", "高雄高等行政法院 高等庭(含改制前高雄高等行政法院)"},
	{"KST", "高雄高等行政法院 地方庭"},
	{"IPC", "智慧財產及商業法院"},
	{"TCH", "臺灣高等法院 臺中分院"},
	{"TNH", "臺灣高等法院 臺南分院"},
	{"KSH", "臺灣高等法院 高雄分院"},
	{"HLH", "臺灣高等法院 花蓮分院"},
	{"TPD", "臺灣臺北地方法院"},
	{"SLD", "臺灣士林地方法院"},
	{"PCD", "臺灣新北地方法院"},
	{"ILD", "臺灣宜蘭地方法院"},
	{"KLD", "臺灣基隆地方法院"},
	{"TYD", "臺灣桃園地方法院"},
	{"SCD", "臺灣新竹地方法院"},
	{"MLD", "臺灣苗栗地方法院"},
	{"TCD", "臺灣臺中地方法院"},
	{"CHD", "臺灣彰化地方法院"},
	{"NTD", "臺灣南投地方法院"},
	{"ULD", "臺灣雲林地方法院"},
	{"CYD", "臺灣嘉義地方法院"},
	{"TND", "臺灣臺南地方法院"},
	{"KSD", "臺灣高雄地方法院"},
	{"CTD", "臺灣橋頭地方法院"},
	{"HLD", "臺灣花蓮地方法院"},
	{"TTD", "臺灣臺東地方法院"},
	{"PTD", "臺灣屏東地方法院"},
	{"PHD", "臺灣澎湖地方法院"},
	{"KMH", "福建高等法院金門分院"},
	{"KMD", "福建金門地方法院"},
	{"LCD", "福建連江地方法院"},
	{"KSY", "臺灣高雄少年及家事法院"},
}

// CourtName returns the Chinese name for a court code, or the code itself
// when not found.
func CourtName(code string) string {
	for _, c := range Courts {
		if c.Code == code {
			return c.Name
		}
	}
	return code
}

// CourtCodes returns just the codes, in display order.
func CourtCodes() []string {
	out := make([]string, 0, len(Courts))
	for _, c := range Courts {
		out = append(out, c.Code)
	}
	return out
}
