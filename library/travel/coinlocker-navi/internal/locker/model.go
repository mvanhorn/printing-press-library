// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package locker

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// NoRecordDateNote is attached to every coinlocker-navi result.
const NoRecordDateNote = "Coin Locker Navi pages show no per-record update date; fetched_at is the observation time, not the data date"

// ReservableCaveat is attached to every Multi Ekicube count.
const ReservableCaveat = "Multi Ekicube counts reservable empty boxes only; 0 does not mean the bank is full (source: \"There may still be lockers available even though there are no available lockers in the search results\")"

// Size is one locker size row as published.
type Size struct {
	Label    *string `json:"label"`
	LabelJA  string  `json:"label_ja"`
	PriceYen *int    `json:"price_yen"`
	Count    *int    `json:"count"`
	Raw      string  `json:"raw"`
}

// Hours is the published usage time.
type Hours struct {
	Raw       *string `json:"raw"`
	Kind      string  `json:"kind"` // clock_range | first_to_last_train | 24h | unknown | unparsed
	Open      *string `json:"open"`
	Close     *string `json:"close"`
	Overnight bool    `json:"overnight"`
}

// Locker is one normalized coinlocker-navi record.
type Locker struct {
	ID               string     `json:"id"`
	URL              string     `json:"url"`
	Name             string     `json:"name"`
	Note             *string    `json:"note"`
	Service          string     `json:"service"`
	ServiceJA        string     `json:"service_ja"`
	WalkUp           *bool      `json:"walk_up"`
	DistanceM        *int       `json:"distance_m"`
	Lat              *float64   `json:"lat"`
	Lon              *float64   `json:"lon"`
	MapURL           *string    `json:"map_url"`
	Sizes            []Size     `json:"sizes"`
	SizesKnown       bool       `json:"sizes_known"`
	Payment          []string   `json:"payment"`
	PaymentJA        []string   `json:"payment_ja"`
	PaymentKnown     bool       `json:"payment_known"`
	ICCard           *bool      `json:"ic_card"`
	ChangeMachine    *bool      `json:"change_machine"`
	ChangeMachineRaw *string    `json:"change_machine_raw"`
	Hours            Hours      `json:"hours"`
	Gate             *string    `json:"gate"`
	GateSource       *string    `json:"gate_source"`
	GateConflict     *string    `json:"gate_conflict,omitempty"`
	OpenDuring       *string    `json:"open_during,omitempty"`
	Live             *LiveMatch `json:"live,omitempty"`
}

var sizeLabels = map[string]string{"小": "S", "中": "M", "大": "L", "特大": "XL"}

// SizeRank orders the normalized labels; 0 means unknown.
func SizeRank(label string) int {
	switch strings.ToUpper(label) {
	case "S":
		return 1
	case "M":
		return 2
	case "L":
		return 3
	case "XL":
		return 4
	}
	return 0
}

var (
	priceRe = regexp.MustCompile(`([0-9,]+)\s*円`)
	countRe = regexp.MustCompile(`([0-9,]+)\s*個`)
)

func atoiPtr(s string) *int {
	s = strings.ReplaceAll(s, ",", "")
	n, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return &n
}

// NewSize normalizes a size label and its detail text such as "400円/39個".
func NewSize(labelJA, detail string) Size {
	labelJA = strings.TrimSpace(labelJA)
	detail = strings.TrimSpace(detail)
	s := Size{LabelJA: labelJA, Raw: strings.TrimSpace(labelJA + " " + detail)}
	if l, ok := sizeLabels[labelJA]; ok {
		v := l
		s.Label = &v
	}
	if m := priceRe.FindStringSubmatch(detail); m != nil {
		s.PriceYen = atoiPtr(m[1])
	}
	if m := countRe.FindStringSubmatch(detail); m != nil {
		s.Count = atoiPtr(m[1])
	}
	return s
}

var paymentNames = map[string]string{
	"現金":       "cash",
	"クレジットカード": "credit_card",
	"suica":    "suica",
	"pasmo":    "pasmo",
	"icoca":    "icoca",
	"pitapa":   "pitapa",
	"toica":    "toica",
	"manaca":   "manaca",
	"kitaca":   "kitaca",
	"sugoca":   "sugoca",
	"nimoca":   "nimoca",
	"はやかけん":    "hayakaken",
	"iruca":    "iruca",
	"交通系ic":    "transit_ic",
	"電子マネー":    "e_money",
}

var icCards = map[string]bool{
	"suica": true, "pasmo": true, "icoca": true, "pitapa": true, "toica": true, "manaca": true,
	"kitaca": true, "sugoca": true, "nimoca": true, "hayakaken": true, "iruca": true, "transit_ic": true,
}

// ParsePayment splits a payment cell such as " 現金 &nbsp; Suica &nbsp; ".
// It returns normalized names, raw tokens, whether the field is known, and the
// IC-card verdict (nil when unknown).
func ParsePayment(raw string) (norm []string, rawTokens []string, known bool, ic *bool) {
	norm = []string{}
	rawTokens = []string{}
	raw = strings.ReplaceAll(raw, " ", " ")
	raw = strings.ReplaceAll(raw, "&nbsp;", " ")
	for _, tok := range strings.Fields(raw) {
		if tok == "情報なし" {
			continue
		}
		rawTokens = append(rawTokens, tok)
		key := strings.ToLower(tok)
		if n, ok := paymentNames[key]; ok {
			norm = append(norm, n)
		} else {
			norm = append(norm, "other:"+tok)
		}
	}
	if len(rawTokens) == 0 {
		return norm, rawTokens, false, nil
	}
	has := false
	for _, n := range norm {
		if icCards[n] {
			has = true
		}
	}
	return norm, rawTokens, true, &has
}

// ParseChangeMachine maps 両替機 text.
func ParseChangeMachine(raw string) (*bool, *string) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "情報なし" {
		return nil, nil
	}
	r := raw
	switch {
	case strings.Contains(raw, "なし"):
		f := false
		return &f, &r
	case strings.Contains(raw, "あり"):
		t := true
		return &t, &r
	}
	return nil, &r
}

// ServiceFromJA maps the service icon label.
func ServiceFromJA(ja string) (service string, walkUp *bool) {
	ja = strings.TrimSpace(ja)
	t, f := true, false
	switch {
	case ja == "コインロッカー":
		return "coin_locker", &t
	case strings.Contains(strings.ToLower(ja), "ecbo"):
		return "ecbo_cloak", &f
	case ja == "":
		return "unknown", nil
	}
	return "other", nil
}

// isUnknown reports whether a cell is the source's "no information" marker.
func isUnknown(s string) bool {
	s = strings.TrimSpace(s)
	return s == "" || s == "情報なし"
}

func strPtr(s string) *string { return &s }

// JST is Japan Standard Time (UTC+9, no daylight saving).
var JST = time.FixedZone("JST", 9*3600)
