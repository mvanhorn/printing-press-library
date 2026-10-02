// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package ferry

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

func (c *Client) Timetable(ctx context.Context, r Route, line string) (Timetable, error) {
	b, e := c.fetch(ctx, r.SourceURL, nil)
	if e != nil {
		return Timetable{}, e
	}
	return ParseTimetable(b, r, line)
}

func hiddenForm(b []byte, expected string) (url.Values, error) {
	doc, e := parseHTML(b)
	if e != nil {
		return nil, e
	}
	if strings.Contains(strings.ToLower(text(doc)), "reservation system is not available for maintenance") {
		return nil, fmt.Errorf("official booking system is under maintenance; no fare or sailing lookup submitted")
	}
	for _, f := range elements(doc, "form") {
		u, e := url.Parse(attr(f, "action"))
		if e != nil {
			continue
		}
		u = (&url.URL{Scheme: "https", Host: "booking.ferry-sunflower.co.jp"}).ResolveReference(u)
		if u.String() != BookingBase+expected || strings.ToLower(attr(f, "method")) != "post" {
			continue
		}
		v := url.Values{}
		for _, n := range elements(f, "input") {
			if attr(n, "type") == "hidden" && attr(n, "name") != "" {
				v.Set(attr(n, "name"), attr(n, "value"))
			}
		}
		if v.Get("__RequestVerificationToken") == "" || v.Get("req_t") == "" {
			return nil, fmt.Errorf("anonymous source session fields missing; no request sent")
		}
		return v, nil
	}
	return nil, fmt.Errorf("official anonymous form changed or session failed; expected read-only planning step absent")
}
func validateTripFields(b []byte, line string, p Party) error {
	doc, e := parseHTML(b)
	if e != nil {
		return e
	}
	wanted := map[string]string{"Ouro_Line": line, "RiyoNaiyo.Number_Of_Adults": strconv.Itoa(p.Adults), "RiyoNaiyo.Number_Of_Children": strconv.Itoa(p.Children), "RiyoNaiyo.Number_Of_Yoji": strconv.Itoa(p.Toddlers), "RiyoNaiyo.Number_Of_Nyuji": strconv.Itoa(p.Infants), "RiyoNaiyo.Number_Of_PetCage": strconv.Itoa(p.PetCages)}
	if p.Mode == "car" {
		wanted["Car_Length"] = carCodes[p.CarCategory]
	}
	for _, f := range bikeFields {
		wanted[f] = "0"
	}
	if p.Mode == "bike" {
		wanted[bikeFields[p.BikeCategory]] = strconv.Itoa(p.Bikes)
	}
	for _, s := range elements(doc, "select") {
		name := attr(s, "name")
		if v, ok := wanted[name]; ok {
			found := false
			for _, o := range elements(s, "option") {
				if attr(o, "value") == v {
					found = true
				}
			}
			if !found {
				return fmt.Errorf("official form no longer supports the requested category/count; no lookup submitted")
			}
			delete(wanted, name)
		}
	}
	if len(wanted) > 0 {
		return fmt.Errorf("official trip form field names changed; no lookup submitted")
	}
	mode := map[string]string{"car": "01", "bike": "02", "foot": "03"}[p.Mode]
	gotMode := false
	gotOneWay := false
	gotDate := false
	for _, n := range elements(doc, "input") {
		switch attr(n, "name") {
		case "JosenNaiyo":
			if attr(n, "value") == mode {
				gotMode = true
			}
		case "katamichiofuku":
			if attr(n, "value") == "katamichi" {
				gotOneWay = true
			}
		case "Ouro_BoardingDate":
			gotDate = true
		}
	}
	if !gotMode || !gotOneWay || !gotDate {
		return fmt.Errorf("official trip form mode/date contract changed; no lookup submitted")
	}
	return nil
}
func (c *Client) Quote(ctx context.Context, r Route, line, date string, p Party) (Quote, error) {
	d, e := ParseDate(date)
	if e != nil {
		return Quote{}, e
	}
	if e = p.Validate(); e != nil {
		return Quote{}, e
	}
	if line != r.OutboundLine && line != r.InboundLine {
		return Quote{}, fmt.Errorf("invalid route direction code")
	}
	now := time.Now().In(JST)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, JST)
	if d.Before(today) {
		return Quote{}, fmt.Errorf("fare lookup cannot quote a past boarding date")
	}
	if d.After(today.AddDate(0, 3, 0)) {
		return Quote{}, fmt.Errorf("date is beyond the source's published three-month booking window; use calendar for fare bands")
	}
	b, e := c.fetch(ctx, BookingURL, nil)
	if e != nil {
		return Quote{}, e
	}
	form, e := hiddenForm(b, "/web/yoyaku/Reserve0000/Reserve")
	if e != nil {
		return Quote{}, e
	}
	b, e = c.fetch(ctx, BookingBase+"/web/yoyaku/Reserve0000/Reserve", form)
	if e != nil {
		return Quote{}, e
	}
	form, e = hiddenForm(b, "/web/yoyaku/Reserve1030/MoveNext")
	if e != nil {
		return Quote{}, e
	}
	if e = validateTripFields(b, line, p); e != nil {
		return Quote{}, e
	}
	form.Set("katamichiofuku", "katamichi")
	form.Set("Ouro_BoardingDate", d.Format("2006/01/02(Mon)"))
	form.Set("Ouro_Line", line)
	mode := map[string]string{"car": "01", "bike": "02", "foot": "03"}[p.Mode]
	form.Set("JosenNaiyo", mode)
	form.Set("Car_Length", carCodes[p.CarCategory])
	for _, f := range bikeFields {
		form.Set(f, "0")
	}
	if p.Mode == "bike" {
		form.Set(bikeFields[p.BikeCategory], strconv.Itoa(p.Bikes))
	}
	for field, n := range map[string]int{"RiyoNaiyo.Number_Of_Adults": p.Adults, "RiyoNaiyo.Number_Of_Children": p.Children, "RiyoNaiyo.Number_Of_Yoji": p.Toddlers, "RiyoNaiyo.Number_Of_Nyuji": p.Infants, "RiyoNaiyo.Number_Of_PetCage": p.PetCages} {
		form.Set(field, strconv.Itoa(n))
	}
	form.Set("Ouro_UseLiner", "false")
	b, e = c.fetch(ctx, BookingBase+"/web/yoyaku/Reserve1030/MoveNext", form)
	if e != nil {
		return Quote{}, e
	}
	return ParseQuote(b, r, line, d, p)
}

var sailingRE = regexp.MustCompile(`^(\d{2}/\d{2})\(([A-Za-z]{3})\)\s+(.+?)\s+(\d{2}:\d{2})\s+Departure\s*>>\s*(.+?)\s+(\d{2}:\d{2})\s+Arrival\s*[（(](.+?)[）)]$`)
var yenRE = regexp.MustCompile(`[\\¥￥]\s*([0-9][0-9,]*)`)
var classRE = regexp.MustCompile(`/Content/img/cabin/([0-9]+)/([0-9]+)/`)
var slugRE = regexp.MustCompile(`[^a-z0-9]+`)
var numberRE = regexp.MustCompile(`^[0-9]+$`)

func slug(s string) string {
	return strings.Trim(slugRE.ReplaceAllString(strings.ToLower(s), "-"), "-")
}

func availabilityStatus(symbol string) string {
	switch {
	case symbol == "○" || symbol == "◯":
		return "available_snapshot"
	case symbol == "×" || symbol == "✕":
		return "unavailable_snapshot"
	case symbol == "－" || symbol == "-":
		return "not_sold_snapshot"
	case strings.Contains(strings.ToLower(symbol), "waitlist"):
		return "waitlist_snapshot"
	case numberRE.MatchString(symbol):
		return "limited_snapshot"
	default:
		return "unknown"
	}
}
func ParseQuote(b []byte, r Route, line string, d time.Time, p Party) (Quote, error) {
	doc, e := parseHTML(b)
	if e != nil {
		return Quote{}, e
	}
	body := text(doc)
	if strings.Contains(body, "Time-out has occurred") {
		return Quote{}, fmt.Errorf("official anonymous session expired; retry a fresh lookup")
	}
	if !strings.Contains(body, "Total Price") {
		return Quote{}, fmt.Errorf("official source returned no fare table (date, route or party unavailable); no fare or sailing inferred")
	}
	out := Quote{RouteID: r.ID, Line: line, BoardingDate: d.Format("2006-01-02"), Party: p, Currency: "JPY", Sailings: []Sailing{}, SourceNotes: []string{}, PriceBasis: "Official 'Discounted Price' displayed for this entered one-way party and vehicle category; no fare arithmetic performed.", Unknowns: []string{"Fuel adjustment, tax and fee breakdown are not separately stated on the returned fare table.", "Displayed cabin/vehicle availability is an observed source snapshot, with no hold or guarantee.", "Room-charge campaigns, special discounts and exact booking eligibility may require the reservation center."}}
	sequence := nodes(doc, func(n *html.Node) bool { return tag(n, "h2") || tag(n, "table") || tag(n, "p") })
	for _, n := range sequence {
		if tag(n, "h2") {
			m := sailingRE.FindStringSubmatch(sailingText(n))
			if m == nil {
				continue
			}
			if m[1] != d.Format("01/02") || m[2] != d.Format("Mon") {
				return Quote{}, fmt.Errorf("source sailing date did not match the requested Japan service date")
			}
			pair := map[string][2]string{"11": {"Kobe", "Oita"}, "12": {"Oita", "Kobe"}, "21": {"Osaka1", "Beppu"}, "22": {"Beppu", "Osaka1"}, "31": {"Osaka2", "Shibushi"}, "32": {"Shibushi", "Osaka2"}}[line]
			expectedFrom, expectedTo := pair[0], pair[1]
			if !strings.HasPrefix(m[3], expectedFrom) || !strings.HasPrefix(m[5], expectedTo) {
				return Quote{}, fmt.Errorf("source route did not match the requested direction")
			}
			dep, e := time.ParseInLocation("2006-01-02 15:04", d.Format("2006-01-02")+" "+m[4], JST)
			if e != nil {
				return Quote{}, fmt.Errorf("invalid source departure time")
			}
			arr, e := time.ParseInLocation("2006-01-02 15:04", d.Format("2006-01-02")+" "+m[6], JST)
			if e != nil {
				return Quote{}, fmt.Errorf("invalid source arrival time")
			}
			offset := 0
			if !arr.After(dep) {
				arr = arr.AddDate(0, 0, 1)
				offset = 1
			}
			out.Sailings = append(out.Sailings, Sailing{ID: line + "/" + d.Format("2006-01-02") + "/" + m[4] + "/" + slug(m[7]), Ship: m[7], From: m[3], To: m[5], Departure: dep.Format(time.RFC3339), Arrival: arr.Format(time.RFC3339), ArrivalDayOffset: offset, RecommendedTerminalArrival: dep.Add(-time.Hour).Format(time.RFC3339), CabinFares: []CabinFare{}})
		}
		if tag(n, "table") {
			grid, err := tableGrid(n)
			if err != nil {
				return Quote{}, err
			}
			for i, row := range grid {
				if len(row) >= 2 && row[0] == "Season" && strings.Contains(row[1], "Applicable discount") && i+1 < len(grid) && len(grid[i+1]) >= 2 {
					out.Season = foldBand(grid[i+1][0])
					out.DiscountLabel = grid[i+1][1]
				}
			}
			if len(out.Sailings) > 0 {
				for _, cells := range tableRows(n) {
					if len(cells) == 6 && (text(cells[1]) == "Vehicle" || text(cells[1]) == "Motorbike") && text(cells[4]) != "" {
						symbol := text(cells[4])
						out.Sailings[len(out.Sailings)-1].VehicleAvailability = &AvailabilitySnapshot{SourceLabel: text(cells[1]), Symbol: symbol, Status: availabilityStatus(symbol)}
					}
				}
			}
			if len(out.Sailings) == 0 || !strings.Contains(text(n), "Discounted Price") {
				continue
			}
			for _, cells := range tableRows(n) {
				if len(cells) != 6 {
					continue
				}
				price := yenRE.FindStringSubmatch(text(cells[3]))
				if price == nil {
					continue
				}
				amount, err := strconv.Atoi(strings.ReplaceAll(price[1], ",", ""))
				if err != nil || amount <= 0 {
					return Quote{}, fmt.Errorf("invalid source cabin fare")
				}
				name := text(cells[1])
				code := ""
				group := ""
				for _, img := range elements(cells[2], "img") {
					if m := classRE.FindStringSubmatch(attr(img, "src")); m != nil {
						group = m[1]
						code = m[2]
						break
					}
				}
				id := slug(name)
				if code != "" {
					id = group + "/" + code + "/" + id
				}
				symbol := text(cells[4])
				status := availabilityStatus(symbol)
				s := &out.Sailings[len(out.Sailings)-1]
				if len(s.CabinFares) >= 40 {
					return Quote{}, fmt.Errorf("source cabin fare table exceeded supported 40-row cap")
				}
				s.CabinFares = append(s.CabinFares, CabinFare{ID: id, SourceClassCode: code, Name: name, RoomType: text(cells[0]), DisplayedPriceJPY: amount, AvailabilitySymbol: symbol, Availability: status, Info: short(text(cells[5]), 260)})
			}
		}
		if tag(n, "p") {
			s := text(n)
			if strings.HasPrefix(s, "・") {
				out.SourceNotes = append(out.SourceNotes, short(s, 250))
			}
		}
	}
	if len(out.Sailings) == 0 {
		return Quote{}, fmt.Errorf("official fare result has no parseable sailing; no normal schedule substituted")
	}
	if out.Season == "" {
		return Quote{}, fmt.Errorf("official fare result seasonal band missing")
	}
	if p.Mode != "foot" {
		for _, s := range out.Sailings {
			if s.VehicleAvailability == nil {
				out.Unknowns = append(out.Unknowns, "The source vehicle availability indicator is missing for sailing "+s.ID+".")
			}
		}
	}
	return out, nil
}
func foldBand(s string) string {
	r := strings.NewReplacer("Ａ", "A", "Ｂ", "B", "Ｃ", "C", "Ｄ", "D", "Ｅ", "E").Replace(strings.TrimSpace(s))
	return r
}

var eventsRE = regexp.MustCompile(`(?s)const\s+calendarEvents([1-4])\s*=\s*\[(.*?)\];`)
var eventRE = regexp.MustCompile(`\{\s*title:\s*'([A-E])'\s*,\s*start:\s*'(\d{4}-\d{2}-\d{2})'\s*\}`)

func ParseCalendar(js []byte, which string, from, to time.Time) (Calendar, error) {
	out := Calendar{Dates: []SeasonDate{}, MissingDates: []string{}, RequestedFrom: from.Format("2006-01-02"), RequestedTo: to.Format("2006-01-02")}
	all := map[string]string{}
	for _, a := range eventsRE.FindAllSubmatch(js, -1) {
		if string(a[1]) != which {
			continue
		}
		for _, m := range eventRE.FindAllSubmatch(a[2], -1) {
			date := string(m[2])
			if _, e := ParseDate(date); e != nil {
				return out, fmt.Errorf("invalid source calendar date")
			}
			if old, ok := all[date]; ok && old != string(m[1]) {
				return out, fmt.Errorf("conflicting source fare bands for %s", date)
			}
			all[date] = string(m[1])
		}
	}
	if len(all) == 0 {
		return out, fmt.Errorf("official calendar shape changed: direction events missing")
	}
	var keys []string
	for k := range all {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out.PublishedFrom = keys[0]
	out.PublishedTo = keys[len(keys)-1]
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		date := d.Format("2006-01-02")
		band, ok := all[date]
		if !ok {
			out.MissingDates = append(out.MissingDates, date)
			continue
		}
		out.Dates = append(out.Dates, SeasonDate{date, d.Weekday().String(), band, band == "E"})
	}
	out.Complete = len(out.MissingDates) == 0
	return out, nil
}
func (c *Client) Calendar(ctx context.Context, r Route, line string, from, to time.Time) (Calendar, error) {
	if to.Before(from) || to.Sub(from) > 92*24*time.Hour {
		return Calendar{}, fmt.Errorf("calendar window must be ordered and at most 93 inclusive days")
	}
	fee := PublicBase + "/en/route/" + r.ID + "/fee/"
	b, e := c.fetch(ctx, fee, nil)
	if e != nil {
		return Calendar{}, e
	}
	doc, e := parseHTML(b)
	if e != nil {
		return Calendar{}, e
	}
	var src string
	for _, n := range elements(doc, "script") {
		s := attr(n, "src")
		if strings.HasPrefix(s, "/route/inc/cal-config/") {
			src = s
			break
		}
	}
	if src == "" {
		return Calendar{}, fmt.Errorf("official seasonal calendar script missing")
	}
	base, _ := url.Parse(PublicBase)
	u, e := url.Parse(src)
	if e != nil {
		return Calendar{}, e
	}
	u = base.ResolveReference(u)
	js, e := c.fetch(ctx, u.String(), nil)
	if e != nil {
		return Calendar{}, e
	}
	which := "1"
	if r.ID != "osaka-beppu" {
		which = "3"
	}
	if line == r.InboundLine {
		n, _ := strconv.Atoi(which)
		which = strconv.Itoa(n + 1)
	}
	out, e := ParseCalendar(js, which, from, to)
	if e != nil {
		return out, e
	}
	out.RouteID = r.ID
	out.Line = line
	for _, t := range elements(article(doc), "table") {
		g, e := tableGrid(t)
		if e != nil {
			return out, e
		}
		if len(g) == 3 && len(g[0]) >= 4 && g[0][0] == "Discount" {
			for i := 1; i < len(g[0]); i++ {
				if i < len(g[1]) && i < len(g[2]) {
					out.DiscountConditions = append(out.DiscountConditions, g[0][i]+": "+g[1][i]+"; source excluded seasons: "+g[2][i])
				}
			}
		}
	}
	return out, nil
}

type Cabin struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Category        string `json:"category"`
	RoomType        string `json:"room_type,omitempty"`
	OccupancySource string `json:"occupancy_source"`
	MinOccupancy    *int   `json:"min_occupancy,omitempty"`
	MaxOccupancy    *int   `json:"max_occupancy,omitempty"`
	OccupancyBasis  string `json:"occupancy_basis"`
}

var occupancyRE = regexp.MustCompile(`(?i)(\d+)\s*(?:-|to)\s*(\d+)\s*(?:-?person|persons)`)
var fixedOccupancyRE = regexp.MustCompile(`(?i)^(\d+)\s*(?:persons?|people)\b`)

func ParseCabins(b []byte) ([]Cabin, error) {
	doc, e := parseHTML(b)
	if e != nil {
		return nil, e
	}
	a := article(doc)
	out := []Cabin{}
	current := ""
	anchor := ""
	group := ""
	index := map[string]int{}
	put := func(name, cap, roomtype string) {
		if name == "" {
			return
		}
		key := name
		category := "private"
		basis := "source room capacity; booking minimum and eligibility are not inferred"
		lowType := strings.ToLower(roomtype)
		coreName := strings.ToLower(strings.TrimSpace(strings.Split(name, "(")[0]))
		lowCapacity := strings.ToLower(cap)
		if strings.Contains(lowType, "shared room") || strings.Contains(lowType, "dormitory") || strings.Contains(lowCapacity, "by each section") || strings.Contains(lowCapacity, "per compartment") || coreName == "private bed" && strings.Contains(lowCapacity, "all seats reserved") || coreName == "private bed / all seats reserved" || coreName == "tourist" || coreName == "tourist room" || coreName == "tourist bed" {
			category = "dormitory"
			basis = "seat/section or shared-room capacity; preserve source wording"
		} else if strings.Contains(lowType, "semi-private") {
			category = "semi_private"
		}
		item := Cabin{ID: anchor + "/" + slug(name), Name: name, Category: category, RoomType: roomtype, OccupancySource: short(cap, 170), OccupancyBasis: basis}
		if m := occupancyRE.FindStringSubmatch(cap); m != nil && category != "dormitory" {
			min, _ := strconv.Atoi(m[1])
			max, _ := strconv.Atoi(m[2])
			item.MinOccupancy = &min
			item.MaxOccupancy = &max
		} else if m := fixedOccupancyRE.FindStringSubmatch(cap); m != nil && category != "dormitory" {
			max, _ := strconv.Atoi(m[1])
			item.MaxOccupancy = &max
		}
		if i, ok := index[key]; ok {
			out[i] = item
		} else {
			index[key] = len(out)
			out = append(out, item)
		}
	}
	for _, n := range nodes(a, func(n *html.Node) bool { return tag(n, "h2") || tag(n, "h3") || tag(n, "p") || tag(n, "table") }) {
		if tag(n, "h2") {
			group = text(n)
			current = group
			anchor = attr(n, "id")
			if anchor == "" {
				anchor = slug(group)
			}
		}
		if tag(n, "h3") {
			name := text(n)
			if name != "" {
				current = name
			}
		}
		if tag(n, "p") {
			s := text(n)
			low := strings.ToLower(s)
			if strings.Contains(low, "occupancy") || strings.HasPrefix(low, "occupancy :") || strings.Contains(low, "all seats reserved") {
				put(current, s, "")
			}
		}
		if tag(n, "table") {
			g, e := tableGrid(n)
			if e != nil {
				return nil, e
			}
			capacity := ""
			rt := ""
			for _, row := range g {
				if len(row) < 2 {
					continue
				}
				if strings.EqualFold(row[0], "Capacity") || strings.EqualFold(row[0], "Occupancy") {
					capacity = row[1]
				}
				if row[0] == "Room Type" {
					rt = row[1]
				}
			}
			if capacity != "" {
				put(current, capacity, rt)
			}
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("official cabin occupancy descriptions missing")
	}
	if len(out) > 40 {
		return nil, fmt.Errorf("official cabin category limit exceeded")
	}
	return out, nil
}
func (c *Client) Cabins(ctx context.Context, r Route) ([]Cabin, error) {
	b, e := c.fetch(ctx, PublicBase+"/en/route/"+r.ID+"/cabin/", nil)
	if e != nil {
		return nil, e
	}
	return ParseCabins(b)
}

type Access struct {
	Mode     string `json:"mode"`
	Guidance string `json:"guidance"`
}
type Terminal struct {
	Port       Port     `json:"port"`
	SourceName string   `json:"source_name"`
	Address    string   `json:"address"`
	Phone      string   `json:"phone"`
	Access     []Access `json:"access"`
}

func ParsePorts(b []byte, r Route) ([]Terminal, error) {
	doc, e := parseHTML(b)
	if e != nil {
		return nil, e
	}
	a := article(doc)
	out := []Terminal{}
	for _, box := range nodes(a, func(n *html.Node) bool {
		return tag(n, "div") && strings.Contains(" "+attr(n, "class")+" ", " tabBox ")
	}) {
		item := Terminal{Access: []Access{}}
		for _, h := range elements(box, "h2") {
			item.SourceName = text(h)
			break
		}
		for _, t := range elements(box, "table") {
			g, e := tableGrid(t)
			if e != nil {
				return nil, e
			}
			for _, row := range g {
				if len(row) >= 2 {
					switch row[0] {
					case "Address":
						item.Address = strings.TrimSpace(strings.TrimPrefix(row[1], ":"))
					case "Office Phone No.":
						item.Phone = strings.TrimSpace(strings.TrimPrefix(row[1], ":"))
					}
				}
			}
		}
		for _, h := range elements(box, "h3") {
			mode := text(h)
			if !strings.HasPrefix(mode, "By ") && !strings.HasPrefix(mode, "From ") {
				continue
			}
			var parts []string
			for n := h.NextSibling; n != nil; n = n.NextSibling {
				if tag(n, "h3") || tag(n, "h2") {
					break
				}
				if n.Type == html.ElementNode {
					if s := text(n); s != "" {
						parts = append(parts, s)
					}
				}
			}
			if len(parts) > 0 {
				item.Access = append(item.Access, Access{mode, short(strings.Join(parts, " "), 350)})
			}
		}
		if item.Address != "" {
			out = append(out, item)
		}
	}
	if len(out) != 2 {
		return nil, fmt.Errorf("official port page shape changed: expected two addressed terminals")
	}
	var origin, destination *Terminal
	for i := range out {
		isOrigin := terminalNameMatches(out[i].SourceName, r.Origin)
		isDestination := terminalNameMatches(out[i].SourceName, r.Destination)
		if isOrigin == isDestination {
			return nil, fmt.Errorf("official terminal name is unknown or ambiguous; no address assigned")
		}
		if isOrigin {
			if origin != nil {
				return nil, fmt.Errorf("official port page repeats the origin terminal; no address assigned")
			}
			out[i].Port = r.Origin
			origin = &out[i]
		} else {
			if destination != nil {
				return nil, fmt.Errorf("official port page repeats the destination terminal; no address assigned")
			}
			out[i].Port = r.Destination
			destination = &out[i]
		}
	}
	if origin == nil || destination == nil {
		return nil, fmt.Errorf("official port page does not identify both route terminals")
	}
	return []Terminal{*origin, *destination}, nil
}
func terminalNameMatches(name string, p Port) bool {
	n := strings.ReplaceAll(slug(name), "-", "")
	// Only observed official headings identify a terminal. A changed title
	// fails closed rather than accepting another terminal number or a name
	// that merely mentions the expected destination.
	known := map[string]string{
		"sunflowerterminalosakaterminal1": "osaka-terminal1",
		"sunflowerterminalosakaterminal2": "osaka-terminal2",
		"beppukankoko":                    "beppu",
		"kobeportrokkoisland":             "kobe",
		"oitaport":                        "oita",
		"kagoshimashibushiport":           "shibushi",
	}
	return p.ID != "" && known[n] == p.ID
}
func (c *Client) Ports(ctx context.Context, r Route) ([]Terminal, error) {
	b, e := c.fetch(ctx, PublicBase+"/en/route/"+r.ID+"/boarding/", nil)
	if e != nil {
		return nil, e
	}
	return ParsePorts(b, r)
}

type CancellationBand struct {
	Timing     string `json:"timing"`
	FixedJPY   *int   `json:"fixed_jpy,omitempty"`
	Percent    *int   `json:"percent,omitempty"`
	MinimumJPY *int   `json:"minimum_jpy,omitempty"`
	SourceURL  string `json:"source_url"`
}
type Conditions struct {
	Checkin             map[string]any     `json:"checkin"`
	Cancellation        []CancellationBand `json:"cancellation"`
	Changes             string             `json:"changes"`
	Baggage             map[string]any     `json:"baggage"`
	PassengerCategories map[string]string  `json:"passenger_categories"`
	VehicleCategories   map[string]string  `json:"vehicle_categories"`
	Unknowns            []string           `json:"unknowns"`
}

func ParseConditions(en, ja []byte) (Conditions, error) {
	eDoc, e := parseHTML(en)
	if e != nil {
		return Conditions{}, e
	}
	jDoc, e := parseHTML(ja)
	if e != nil {
		return Conditions{}, e
	}
	et := text(article(eDoc))
	jt := text(jDoc)
	if !strings.Contains(et, "arrive one hour before departure") || !strings.Contains(et, "within 30 minutes") {
		return Conditions{}, fmt.Errorf("source check-in guidance changed; no deadline inferred")
	}
	if !strings.Contains(jt, "3辺の長さの和が2メートル以下") || !strings.Contains(jt, "重量が30キログラム以下") || !strings.Contains(jt, "重量の和が20キログラム以下") {
		return Conditions{}, fmt.Errorf("source cabin-baggage conditions changed; no allowance inferred")
	}
	if !strings.Contains(jt, "発航する日の7日前") || !strings.Contains(jt, "発航する日の前々日") || !strings.Contains(jt, "券面記載金額の1割") || !strings.Contains(jt, "券面記載金額の3割") {
		return Conditions{}, fmt.Errorf("source normal ticket refund bands changed; no cancellation charge inferred")
	}
	fixed := regexp.MustCompile(`発航する日の7日前までの請求に係る払戻し\s+(\d+)円`).FindStringSubmatch(jt)
	middle := regexp.MustCompile(`発航する日の前々日までの請求に係る払戻し\s+券面記載金額の(\d+)割.{0,70}?その額が(\d+)円`).FindStringSubmatch(jt)
	late := regexp.MustCompile(`発航時刻までの請求に係る払戻し\s+券面記載金額の(\d+)割.{0,70}?その額が(\d+)円`).FindStringSubmatch(jt)
	if fixed == nil || middle == nil || late == nil || !strings.Contains(et, "After departure") || !strings.Contains(et, "100%") {
		return Conditions{}, fmt.Errorf("source refund amount/minimum shape changed; no charge inferred")
	}
	amount, _ := strconv.Atoi(fixed[1])
	m10, _ := strconv.Atoi(middle[2])
	m30, _ := strconv.Atoi(late[2])
	r10, _ := strconv.Atoi(middle[1])
	r30, _ := strconv.Atoi(late[1])
	p10 := r10 * 10
	p30 := r30 * 10
	p100 := 100
	out := Conditions{Checkin: map[string]any{"recommended_arrival_minutes_before_departure": 60, "boarding_starts_minutes_before_departure": 60, "boarding_may_be_refused_within_minutes": 30, "basis": "Official English guidance; confirm earlier seasonal/vehicle requirements with booking instructions."}, Cancellation: []CancellationBand{{"7 or more days before sailing", &amount, nil, nil, ConditionsURL}, {"6 through 2 days before sailing", nil, &p10, &m10, ConditionsURL}, {"1 day before sailing through departure time", nil, &p30, &m30, ConditionsURL}, {"After departure: no normal refund", nil, &p100, nil, PublicBase + "/en/reservation/"}}, Changes: "Contact the reservation center immediately for changes/cancellations. Normal passenger ticket changes before use may be allowed once, subject to conditions and fare differences; campaign/agency tickets may differ.", Baggage: map[string]any{"carried_item_sum_of_three_dimensions_max_m": 2, "carried_item_max_weight_kg": 30, "free_carried_baggage_aggregate_weight_kg": 20, "basis": "Operator-wide passenger carriage conditions: carried-item definition differs from aggregate free allowance.", "exceptions": "Passenger-use wheelchair and qualifying assistance dog are separately defined; dangerous goods and other prohibited items are restricted.", "vehicle_access": "Confirm retrieval rules before leaving a vehicle; this command does not establish onboard car-deck access."}, PassengerCategories: map[string]string{"adult": "Age 12+ excluding elementary-school students; portal describes junior high-school or older.", "child": "Elementary-school student, including an elementary-school student aged 12; portal label Child (6–12) is approximate.", "toddler": "Preschool child aged 1+; normally one accompanied child per adult may be free, except allocated seats/beds and other conditions.", "infant": "Under 1 year; separate reserved seat/bed may incur a child fare."}, VehicleCategories: map[string]string{"car": "One passenger car, strictly <3m/<4m/<5m/<6m according to vehicle certificate. Tractor/trailer/series900 and unsupported lengths require the reservation center.", "bike": "Source categories over750cc, up-to750cc, up-to125cc and bicycle; two-wheeled vehicles cannot exceed adult count."}, Unknowns: []string{"Meal costs are excluded from normal transport fares; no meals are requested by the quote command.", "Fuel adjustment, tax and fee breakdown must be confirmed from the final booking price.", "The English cancellation summary omits the day-before band; the linked Japanese common passenger conditions establish the normal-ticket band and source minimum fee.", "Agency/campaign/group/room-charge ticket conditions and refund fee unit require the reservation center; this is not a quote for an existing booking."}}
	return out, nil
}
func (c *Client) Conditions(ctx context.Context) (Conditions, error) {
	en, e := c.fetch(ctx, PublicBase+"/en/reservation/", nil)
	if e != nil {
		return Conditions{}, e
	}
	ja, e := c.fetch(ctx, ConditionsURL, nil)
	if e != nil {
		return Conditions{}, e
	}
	return ParseConditions(en, ja)
}
