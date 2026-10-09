// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package locker

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// Ekicube service types (from the site bundle: GENERAL=1, FREELY_GENERAL=3).
var ekicubeServices = map[int]string{1: "standard", 2: "premium", 3: "one_day_open_close", 4: "one_day_open_close_premium"}

// ekicubeSizeClass maps source box keys to the S/M/L/XL family used by --size.
// The source does not publish this mapping; it is our documented grouping.
var ekicubeSizeClass = map[string]string{"ss": "S", "s": "S", "sm": "M", "m": "M", "ml": "L", "l": "L", "lw": "L", "tl": "L", "xl": "XL"}

// BiName holds a Japanese and an English name from the source.
type BiName struct {
	JA *string `json:"ja"`
	EN *string `json:"en"`
}

// EkicubeBox is one size/service availability row.
type EkicubeBox struct {
	Key               string          `json:"size_key"`
	SizeClass         *string         `json:"size_class"`
	ServiceType       int             `json:"service_type"`
	Service           string          `json:"service"`
	ReservableEmpty   int             `json:"reservable_empty"`
	UsageFeeYen       *int            `json:"usage_fee_yen"`
	ReservationFeeYen *int            `json:"reservation_fee_yen"`
	PriceRaw          json.RawMessage `json:"price_raw"`
}

// EkicubeHours is the published business hours.
type EkicubeHours struct {
	Summary *string            `json:"summary"`
	Kind    string             `json:"kind"` // HoursUnknown, HoursFirstToLastTrain, HoursWeekdayRanges or HoursUnparsed
	Days    map[string]*string `json:"days"`
}

// EkicubeLocation is one normalized Multi Ekicube locker bank.
type EkicubeLocation struct {
	ID         int          `json:"id"`
	Lat        float64      `json:"lat"`
	Lon        float64      `json:"lon"`
	DistanceM  *int         `json:"distance_m"`
	Station    BiName       `json:"station"`
	Area       BiName       `json:"area"`
	Name       BiName       `json:"name"`
	Gate       *string      `json:"gate"`
	GateSource string       `json:"gate_source"`
	Hours      EkicubeHours `json:"hours"`
	Boxes      []EkicubeBox `json:"boxes"`
	Caveat     string       `json:"caveat,omitempty"`
}

type rawEkicube struct {
	Total     int `json:"total"`
	PageNo    int `json:"page_no"`
	PageTotal int `json:"page_total"`
	Locations []struct {
		ID         int    `json:"id"`
		Latitude   string `json:"latitude"`
		Longitude  string `json:"longitude"`
		Inside     bool   `json:"inside_ticket_gate"`
		Outside    bool   `json:"outside_ticket_gate"`
		Attributes struct {
			DisplayName   *string                    `json:"display_name"`
			Distance      *int                       `json:"distance"`
			BusinessHours map[string]json.RawMessage `json:"business_hours"`
		} `json:"attributes"`
		Base struct {
			Attributes struct {
				DisplayName *string `json:"display_name"`
			} `json:"attributes"`
		} `json:"base"`
		Area struct {
			Attributes struct {
				DisplayName *string `json:"display_name"`
			} `json:"attributes"`
		} `json:"area"`
		BoxAvailability map[string]json.RawMessage `json:"box_availability"`
	} `json:"locations"`
}

// EkicubeQuery selects locations.
type EkicubeQuery struct {
	Keyword  string
	Lat, Lon *float64
	RadiusM  int
	Gate     string // inside | outside | ""
	Date     string // YYYY-MM-DD (JST)
	MaxPages int
	// StopAfter stops paging once this many locations are read (0 = no early
	// stop). The caller returns only the first StopAfter rows, so with no
	// Gate filter only those rows get English names (fewer slow requests).
	StopAfter int
	// JapaneseOnly skips the English name requests (name.en stays null).
	JapaneseOnly bool
	// Done, for a coordinate query, is called after each page with all banks
	// read so far and the covered radius in metres: every bank closer than
	// coveredM to the query point has been read (+Inf after the last page).
	// The source sorts banks by distance. Return true to stop paging.
	Done func(read []EkicubeLocation, coveredM float64) bool
}

// EkicubePage is a parsed page.
type EkicubePage struct {
	Total, PageNo, PageTotal int
	Locations                []EkicubeLocation
}

// ParseEkicube normalizes one ph2 response in one language.
func ParseEkicube(body []byte, lang string) (EkicubePage, error) {
	var r rawEkicube
	if err := json.Unmarshal(body, &r); err != nil {
		return EkicubePage{}, fmt.Errorf("multiecube response: %w", err)
	}
	p := EkicubePage{Total: r.Total, PageNo: r.PageNo, PageTotal: r.PageTotal, Locations: []EkicubeLocation{}}
	for _, x := range r.Locations {
		lat, errLat := strconv.ParseFloat(x.Latitude, 64)
		lon, errLon := strconv.ParseFloat(x.Longitude, 64)
		if errLat != nil || errLon != nil {
			// A bank without coordinates cannot be matched or used for
			// coverage; skip it rather than place it at (0, 0).
			continue
		}
		loc := EkicubeLocation{ID: x.ID, Lat: lat, Lon: lon, DistanceM: x.Attributes.Distance, GateSource: "multiecube", Caveat: ReservableCaveat}
		set := func(b *BiName, v *string) {
			if v == nil {
				return
			}
			s := strings.TrimSpace(*v)
			if lang == "en" {
				b.EN = &s
			} else {
				b.JA = &s
			}
		}
		set(&loc.Station, x.Base.Attributes.DisplayName)
		set(&loc.Area, x.Area.Attributes.DisplayName)
		set(&loc.Name, x.Attributes.DisplayName)
		switch {
		case x.Inside && x.Outside:
			loc.Gate = strPtr("both")
		case x.Inside:
			loc.Gate = strPtr("inside")
		case x.Outside:
			loc.Gate = strPtr("outside")
		}
		loc.Hours = parseEkicubeHours(x.Attributes.BusinessHours)
		loc.Boxes = parseBoxes(x.BoxAvailability)
		p.Locations = append(p.Locations, loc)
	}
	return p, nil
}

func parseEkicubeHours(bh map[string]json.RawMessage) EkicubeHours {
	h := EkicubeHours{Kind: HoursUnknown, Days: map[string]*string{}}
	if bh == nil {
		return h
	}
	if raw, ok := bh["summary"]; ok {
		var s string
		if json.Unmarshal(raw, &s) == nil && s != "" {
			h.Summary = &s
		}
	}
	allMidnight := true
	n := 0
	for _, d := range []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"} {
		raw, ok := bh[d]
		if !ok {
			continue
		}
		var v struct {
			Open  string `json:"open_time"`
			Close string `json:"close_time"`
		}
		if json.Unmarshal(raw, &v) != nil || v.Open == "" {
			h.Days[d] = nil
			continue
		}
		n++
		s := v.Open + "-" + v.Close
		h.Days[d] = &s
		if v.Open != "00:00" || v.Close != "00:00" {
			allMidnight = false
		}
	}
	switch {
	case h.Summary != nil && (strings.Contains(*h.Summary, "終電") || strings.Contains(strings.ToLower(*h.Summary), "last train")):
		h.Kind = HoursFirstToLastTrain
	case n > 0 && allMidnight:
		h.Kind = HoursUnparsed
	case n > 0:
		h.Kind = HoursWeekdayRanges
	}
	return h
}

func parseBoxes(av map[string]json.RawMessage) []EkicubeBox {
	boxes := []EkicubeBox{}
	keys := make([]string, 0, len(av))
	for k := range av {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return boxOrder(keys[i]) < boxOrder(keys[j]) })
	for _, k := range keys {
		if k == "from_at" || k == "end_at" {
			continue
		}
		var v struct {
			ByService []struct {
				ServiceType int             `json:"service_type"`
				NumEmpty    int             `json:"num_empty"`
				Price       json.RawMessage `json:"price"`
			} `json:"by_service"`
		}
		if json.Unmarshal(av[k], &v) != nil {
			continue
		}
		for _, s := range v.ByService {
			b := EkicubeBox{Key: k, ServiceType: s.ServiceType, Service: ekicubeServices[s.ServiceType], ReservableEmpty: s.NumEmpty, PriceRaw: s.Price}
			if b.Service == "" {
				b.Service = "type_" + strconv.Itoa(s.ServiceType)
			}
			if c, ok := ekicubeSizeClass[k]; ok {
				b.SizeClass = strPtr(c)
			}
			var pr struct {
				Basic *int `json:"basic"`
				Std   *int `json:"std"`
			}
			if json.Unmarshal(s.Price, &pr) == nil {
				b.UsageFeeYen, b.ReservationFeeYen = pr.Std, pr.Basic
			}
			boxes = append(boxes, b)
		}
	}
	return boxes
}

func boxOrder(k string) int {
	order := []string{"ss", "s", "sm", "m", "ml", "l", "lw", "tl", "xl"}
	for i, o := range order {
		if o == k {
			return i
		}
	}
	return 100
}

func (c *Client) ekicubePage(ctx context.Context, q EkicubeQuery, lang string, page int) (EkicubePage, error) {
	v := url.Values{}
	v.Set("lang", lang)
	v.Set("includes_premium", "true")
	v.Set("includes_no_empty", "true")
	v.Set("from_at", q.Date)
	v.Set("end_at", q.Date)
	v.Set("limit", "20")
	v.Set("page_no", strconv.Itoa(page))
	if q.Keyword != "" {
		v.Set("q", q.Keyword)
	}
	if q.Lat != nil && q.Lon != nil {
		v.Set("latitude", strconv.FormatFloat(*q.Lat, 'f', 6, 64))
		v.Set("longitude", strconv.FormatFloat(*q.Lon, 'f', 6, 64))
		if q.RadiusM > 0 {
			v.Set("distance_max", strconv.Itoa(q.RadiusM))
		}
	}
	if q.Gate == "inside" || q.Gate == "outside" {
		v.Set("ticket_gate", q.Gate)
	}
	// service_type is sent as a literal comma list, as the site does. The API
	// also accepts the encoded form (1%2C3) with identical results (checked
	// live 2026-10-10), so the raw source command and MCP tool, which encode
	// the comma, are equivalent.
	u := c.EkicubeBase + "/v1/location/ph2?" + v.Encode() + "&service_type=1,3"
	body, err := c.fetch(ctx, u, map[string]string{"Accept": "application/json", "Origin": "https://multiecube.com", "Referer": "https://multiecube.com/"})
	if err != nil {
		return EkicubePage{}, err
	}
	return ParseEkicube(body, lang)
}

// Ekicube fetches up to MaxPages pages in Japanese and English and merges
// names by location id. Returns locations, the source total and whether more
// pages exist beyond the cap. The source matches a keyword only against names
// in the request language (romaji matches both), so for a non-ASCII keyword
// the English names come from a coordinate query around the Japanese results.
func (c *Client) Ekicube(ctx context.Context, q EkicubeQuery) ([]EkicubeLocation, int, bool, error) {
	if q.MaxPages <= 0 {
		q.MaxPages = 3
	}
	out := []EkicubeLocation{}
	index := map[int]int{}
	total, pageTotal := 0, 1
	asciiKeyword := isASCII(q.Keyword)
	for page := 1; page <= pageTotal && page <= q.MaxPages; page++ {
		ja, err := c.ekicubePage(ctx, q, "ja", page)
		if err != nil {
			return out, total, false, err
		}
		total, pageTotal = ja.Total, ja.PageTotal
		for _, l := range ja.Locations {
			index[l.ID] = len(out)
			out = append(out, l)
		}
		if asciiKeyword && !q.JapaneseOnly {
			en, err := c.ekicubePage(ctx, q, "en", page)
			if err != nil {
				return out, total, false, err
			}
			mergeEN(out, index, en.Locations)
		}
		if q.StopAfter > 0 && len(out) >= q.StopAfter {
			return c.finishEkicube(ctx, q, out, index, asciiKeyword, total, page < pageTotal)
		}
		if q.Done != nil && q.Lat != nil && q.Lon != nil {
			if q.Done(out, CoveredM(*q.Lat, *q.Lon, out, page >= pageTotal)) {
				return c.finishEkicube(ctx, q, out, index, asciiKeyword, total, page < pageTotal)
			}
		}
	}
	return c.finishEkicube(ctx, q, out, index, asciiKeyword, total, pageTotal > q.MaxPages)
}

func (c *Client) finishEkicube(ctx context.Context, q EkicubeQuery, out []EkicubeLocation, index map[int]int, asciiKeyword bool, total int, more bool) ([]EkicubeLocation, int, bool, error) {
	if !asciiKeyword && !q.JapaneseOnly && len(out) > 0 {
		fill := out
		if q.StopAfter > 0 && q.Gate == "" && len(fill) > q.StopAfter {
			fill = fill[:q.StopAfter]
		}
		if err := c.fillEnglishByArea(ctx, q, fill, index); err != nil {
			return out, total, more, err
		}
	}
	return out, total, more, nil
}

func mergeEN(out []EkicubeLocation, index map[int]int, en []EkicubeLocation) {
	for _, l := range en {
		i, ok := index[l.ID]
		if !ok || i >= len(out) {
			continue
		}
		out[i].Station.EN, out[i].Area.EN, out[i].Name.EN = l.Station.EN, l.Area.EN, l.Name.EN
	}
}

func (c *Client) fillEnglishByArea(ctx context.Context, q EkicubeQuery, out []EkicubeLocation, index map[int]int) error {
	var lat, lon float64
	for _, l := range out {
		lat += l.Lat
		lon += l.Lon
	}
	lat, lon = lat/float64(len(out)), lon/float64(len(out))
	maxD := 0.0
	for _, l := range out {
		if d := DistanceM(lat, lon, l.Lat, l.Lon); d > maxD {
			maxD = d
		}
	}
	eq := EkicubeQuery{Lat: &lat, Lon: &lon, RadiusM: int(maxD) + 50, Gate: q.Gate, Date: q.Date}
	missing := func() bool {
		for _, l := range out {
			if l.Name.EN == nil {
				return true
			}
		}
		return false
	}
	pageTotal := 1
	for page := 1; page <= pageTotal && page <= q.MaxPages+1 && missing(); page++ {
		en, err := c.ekicubePage(ctx, eq, "en", page)
		if err != nil {
			return err
		}
		pageTotal = en.PageTotal
		mergeEN(out, index, en.Locations)
	}
	return nil
}

func isASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return false
		}
	}
	return true
}

// coverageMarginM absorbs the source rounding distances to whole metres.
const coverageMarginM = 2.0

// CoveredM is the radius around (lat, lon) inside which every bank has been
// read, given banks read in source order (nearest first). last means the
// final page was read, so the whole query radius is covered.
func CoveredM(lat, lon float64, read []EkicubeLocation, last bool) float64 {
	if last {
		return math.Inf(1)
	}
	covered := 0.0
	for _, l := range read {
		covered = math.Max(covered, DistanceM(lat, lon, l.Lat, l.Lon))
	}
	// The source sorts by its own whole-metre distance, so an unread bank
	// can be up to about a metre nearer than the farthest bank read.
	return math.Max(0, covered-coverageMarginM)
}
