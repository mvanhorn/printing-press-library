package travel

import (
	"bytes"
	"encoding/json"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

func assignment(root *html.Node, name, raw string) (json.RawMessage, error) {
	pattern := regexp.MustCompile(`(?:^|[;\s])` + regexp.QuoteMeta(name) + `\s*=\s*`)
	var result json.RawMessage
	for _, script := range descendants(root, func(n *html.Node) bool { return n.Data == "script" }) {
		s := rawText(script)
		for _, m := range pattern.FindAllStringIndex(s, -1) {
			if result != nil {
				return nil, parseFailure(raw, "duplicate "+name+" assignment")
			}
			remaining := s[m[1]:]
			if len(remaining) > 2<<20 {
				remaining = remaining[:2<<20]
			}
			decoder := json.NewDecoder(strings.NewReader(remaining))
			if e := decoder.Decode(&result); e != nil {
				return nil, parseFailure(raw, "malformed or oversized "+name+" JSON literal")
			}
		}
	}
	if result == nil {
		return nil, parseFailure(raw, "missing "+name+" JSON literal")
	}
	return result, nil
}
func conditionValue(raw json.RawMessage) (string, bool) {
	var values []string
	if json.Unmarshal(raw, &values) != nil || len(values) != 1 {
		return "", false
	}
	return values[0], true
}
func conditions(root *html.Node, d document, q OfferQuery) error {
	raw, e := assignment(root, "hinfo.conditions", d.Source.URL)
	if e != nil {
		return e
	}
	var c map[string]json.RawMessage
	if json.Unmarshal(raw, &c) != nil || c == nil {
		return parseFailure(d.Source.URL, "invalid source conditions")
	}
	for _, key := range []string{"isDated", "isHotelFixed"} {
		if !bytes.Equal(bytes.TrimSpace(c[key]), []byte("true")) {
			return sourceError("query_mismatch", d.Source.URL, 0, "source did not acknowledge a dated hotel query", nil)
		}
	}
	if !bytes.Equal(bytes.TrimSpace(c["isDayuse"]), []byte("false")) {
		return sourceError("query_mismatch", d.Source.URL, 0, "source returned day-use or missing stay mode", nil)
	}
	expected := offerValues(q)
	expected.Set("f_no", q.HotelID)
	for key, values := range expected {
		if key == "f_page_no" {
			continue
		}
		actual, ok := conditionValue(c[key])
		if !ok || actual != values[0] {
			return sourceError("query_mismatch", d.Source.URL, 0, "source conditions differ for "+key, nil)
		}
	}
	if value, ok := c["f_page_no"]; ok {
		actual, valid := conditionValue(value)
		if !valid || actual != integer(q.Page) {
			return sourceError("query_mismatch", d.Source.URL, 0, "source page number differs", nil)
		}
	}
	if n := nodeByClass(root, "plan-pagination__page--current"); n != nil {
		actual, e := strconv.Atoi(text(n))
		if e != nil || actual != q.Page {
			return sourceError("query_mismatch", d.Source.URL, 0, "source pagination current page differs", nil)
		}
	} else if q.Page > 1 {
		return parseFailure(d.Source.URL, "source page marker is missing")
	}
	return nil
}

type roomQuote struct {
	Inclusive *int64 `json:"sumTotalChargeTaxInclusive"`
	Exclusive *int64 `json:"sumTotalChargeTaxExclusive"`
}
type planQuotes struct {
	Rooms map[string]roomQuote `json:"rooms"`
}
type hotelQuotes struct {
	Plans map[string]planQuotes `json:"plans"`
}

func validReservation(room *html.Node, q OfferQuery, planID, roomID string) (bool, error) {
	forms := descendants(room, func(n *html.Node) bool { return n.Data == "form" })
	for _, form := range forms {
		name := attr(form, "name")
		if name == "" || !strings.EqualFold(attr(form, "method"), "post") {
			continue
		}
		action, e := url.Parse(attr(form, "action"))
		if e != nil || action.Scheme != "https" || action.Hostname() != "aps1.travel.rakuten.co.jp" || action.User != nil || action.Port() != "" || action.Path != "/portal/my/ry_kensaku.k4" {
			continue
		}
		active := false
		for _, a := range descendants(room, func(n *html.Node) bool { return n.Data == "a" || n.Data == "button" }) {
			if attr(a, "disabled") != "" || hasClass(a, "disabled") {
				continue
			}
			href := attr(a, "href")
			needle := "document['" + name + "'].submit()"
			other := "document[\"" + name + "\"].submit()"
			if (attr(a, "data-locate") == "plan-reservation-button" || hasClass(a, "yoyakulLink")) && (strings.Contains(href, needle) || strings.Contains(href, other)) {
				active = true
				break
			}
		}
		if !active {
			continue
		}
		values := map[string]string{}
		for _, input := range descendants(form, func(n *html.Node) bool { return n.Data == "input" }) {
			key := attr(input, "name")
			if _, exists := values[key]; exists {
				return false, parseFailure("", "duplicate reservation input "+key)
			}
			values[key] = attr(input, "value")
		}
		expected := offerValues(q)
		expected.Del("f_page_no")
		expected.Del("f_flg")
		for _, key := range []string{"f_nen1", "f_tuki1", "f_hi1", "f_nen2", "f_tuki2", "f_hi2"} {
			expected.Del(key)
		}
		expected.Set("f_no", q.HotelID)
		expected.Set("f_camp_id", planID)
		expected.Set("f_syu", roomID)
		expected.Set("f_hi1", q.Checkin)
		expected.Set("f_hi2", q.Checkout)
		for key, want := range expected {
			if values[key] != want[0] {
				return false, sourceError("query_mismatch", "", 0, "reservation form differs for "+key, nil)
			}
		}
		return true, nil
	}
	return false, nil
}
func priceEvidence(room *html.Node, q OfferQuery, quote roomQuote) (Price, error) {
	p := Price{Currency: "JPY", ConsumptionTax: "included", AccommodationTax: "unknown", OtherTaxes: "unknown", OptionalFees: "unknown"}
	if quote.Inclusive == nil || *quote.Inclusive <= 0 || (quote.Exclusive != nil && (*quote.Exclusive < 0 || *quote.Exclusive > *quote.Inclusive)) {
		return p, parseFailure("", "actionable room has a missing, invalid or nonpositive keyed stay quote")
	}
	label := text(byLocate(room, "plan-price-detail"))
	p.SourceLabel = label
	nightsIn, _ := time.Parse("2006-01-02", q.Checkin)
	nightsOut, _ := time.Parse("2006-01-02", q.Checkout)
	nights := int(nightsOut.Sub(nightsIn) / (24 * time.Hour))
	nm := regexp.MustCompile(`([0-9]+)\s*泊`).FindStringSubmatch(label)
	am := regexp.MustCompile(`大人\s*([0-9]+)\s*人`).FindStringSubmatch(label)
	if len(nm) != 2 || len(am) != 2 || nm[1] != integer(nights) || am[1] != integer(q.AdultsPerRoom) || (!strings.Contains(label, "税込") && !strings.Contains(label, "税金込")) {
		return p, parseFailure("", "stay-price label does not confirm nights, adults and tax-inclusive amount")
	}
	visible := text(nodeByClass(room, "ndPrice"))
	m := regexp.MustCompile(`合計\s*([0-9,]+)\s*円`).FindStringSubmatch(visible)
	if len(m) != 2 {
		return p, parseFailure("", "actionable room has no baseline whole-stay price label")
	}
	amount, e := strconv.ParseInt(strings.ReplaceAll(m[1], ",", ""), 10, 64)
	if e != nil || amount != *quote.Inclusive {
		return p, parseFailure("", "visible baseline price differs from keyed whole-stay quote")
	}
	p.PerRoomStayJPY = *quote.Inclusive
	baseline := nodeByClass(room, "cmn_rbAndNvrWrap")
	if baseline != nil {
		m = regexp.MustCompile(`1人あたり\s*([0-9,]+)\s*円`).FindStringSubmatch(text(baseline))
		if len(m) == 2 {
			value, e := strconv.ParseInt(strings.ReplaceAll(m[1], ",", ""), 10, 64)
			if e == nil && value > 0 {
				p.PerPersonStayJPY = &value
			}
		}
	}
	return p, nil
}
func meals(room *html.Node) Meals {
	m := Meals{}
	n := byLocate(room, "roomType-option-meal")
	if n == nil {
		return m
	}
	t := text(n)
	if !strings.Contains(t, "食事") && !strings.Contains(t, "朝食") && !strings.Contains(t, "夕食") {
		return m
	}
	m.SourceLabel = pointer(boundedText(t, 2000))
	for _, pair := range []struct {
		word   string
		target **bool
	}{{"朝食", &m.Breakfast}, {"夕食", &m.Dinner}} {
		var b bool
		if strings.Contains(t, pair.word+"なし") {
			b = false
			*pair.target = &b
		} else if strings.Contains(t, pair.word+"あり") || strings.Contains(t, pair.word+"付") {
			b = true
			*pair.target = &b
		}
	}
	return m
}
func ancestorPlan(room *html.Node) *html.Node {
	for n := room.Parent; n != nil; n = n.Parent {
		if hasClass(n, "planThumb") {
			return n
		}
	}
	return nil
}
func roomName(room *html.Node) *string {
	n := first(room, func(x *html.Node) bool { return x.Data == "h4" || x.Data == "h3" })
	if n != nil {
		return pointer(text(n))
	}
	n = first(room, func(x *html.Node) bool { return x.Data == "a" && attr(x, "title") != "" })
	if n != nil {
		return pointer(attr(n, "title"))
	}
	return nil
}
func planDescription(plan *html.Node) *string {
	if plan == nil {
		return nil
	}
	for _, loc := range []string{"plan-description", "planDetail"} {
		if n := byLocate(plan, loc); n != nil {
			return pointer(text(n))
		}
	}
	for _, class := range []string{"htlPlnDtlPrv", "htlPlnDetail", "htlPlnDscr", "plan-description"} {
		if n := nodeByClass(plan, class); n != nil {
			return pointer(text(n))
		}
	}
	// Real pages label the plan's long prose independently from room rows.
	n := first(plan, func(x *html.Node) bool { return attr(x, "data-role") == "planDetail" })
	if n != nil {
		return pointer(text(n))
	}
	return nil
}
func parseOffers(d document, q OfferQuery) (OfferResult, error) {
	root, e := parseDOM(d)
	if e != nil {
		return OfferResult{}, e
	}
	out := OfferResult{Status: StatusOK, Offers: []Offer{}, Query: q, Source: d.Source}
	if e = conditions(root, d, q); e != nil {
		return out, e
	}
	raw, e := assignment(root, "hinfo.hotels", d.Source.URL)
	if e != nil {
		return out, e
	}
	var hotels map[string]hotelQuotes
	if json.Unmarshal(raw, &hotels) != nil || hotels == nil {
		return out, parseFailure(d.Source.URL, "malformed hotel/plan/room quote map")
	}
	quotedHotel, exists := hotels[q.HotelID]
	if !exists && len(hotels) > 0 {
		return out, sourceError("query_mismatch", d.Source.URL, 0, "quote hotel identifier differs", nil)
	}
	rows := descendants(root, func(n *html.Node) bool { return hasClass(n, "rm-type-wrapper") })
	if len(rows) > 2000 {
		return out, parseFailure(d.Source.URL, "room rows exceed bounded parser limit")
	}
	offers := []Offer{}
	seen := map[string]bool{}
	sourcePlans := map[string]bool{}
	for _, plan := range descendants(root, func(n *html.Node) bool { return hasClass(n, "planThumb") }) {
		if id := attr(plan, "id"); hotelIDPattern.MatchString(id) {
			sourcePlans[id] = true
		}
	}
	unavailable := 0
	catalogRows := 0
	for _, room := range rows {
		// The dated page appends an observed noplan-* undated room catalog.
		// It has no plan identity and must never be presented as inventory.
		if strings.HasPrefix(attr(room, "id"), "noplan-") {
			catalogRows++
			continue
		}
		plan := ancestorPlan(room)
		if plan == nil {
			return out, parseFailure(d.Source.URL, "room "+attr(room, "id")+" has no keyed plan ancestor")
		}
		planID := attr(plan, "id")
		roomID := strings.TrimPrefix(attr(room, "id"), planID+"-")
		if !hotelIDPattern.MatchString(planID) || roomID == "" || attr(room, "id") != planID+"-"+roomID || len(roomID) > 200 {
			return out, parseFailure(d.Source.URL, "invalid plan/room identity")
		}
		sourcePlans[planID] = true
		t := text(room)
		if strings.Contains(t, "空室なし") || strings.Contains(t, "ご希望の日程に該当する空室が見つかりません") {
			unavailable++
			continue
		}
		actionable, err := validReservation(room, q, planID, roomID)
		if err != nil {
			if se, ok := err.(*SourceError); ok {
				se.URL = d.Source.URL
			}
			return out, err
		}
		quote, quoteFound := quotedHotel.Plans[planID].Rooms[roomID]
		if !actionable {
			if quoteFound && quote.Inclusive != nil && *quote.Inclusive > 0 {
				return out, parseFailure(d.Source.URL, "positive dated room quote has no actionable reservation control")
			}
			continue
		}
		if !quoteFound {
			return out, parseFailure(d.Source.URL, "actionable reservation has no hotel/plan/room keyed quote")
		}
		price, err := priceEvidence(room, q, quote)
		if err != nil {
			if se, ok := err.(*SourceError); ok {
				se.URL = d.Source.URL
			}
			return out, err
		}
		key := planID + "\x00" + roomID
		if seen[key] {
			return out, parseFailure(d.Source.URL, "duplicate offer tuple")
		}
		seen[key] = true
		title := first(plan, func(n *html.Node) bool { return n.Data == "h3" || n.Data == "h2" || n.Data == "h4" })
		anchor := attr(room, "id")
		u, _ := url.Parse(d.Source.URL)
		u.Fragment = anchor
		offer := Offer{HotelID: q.HotelID, PlanID: planID, RoomID: roomID, PlanName: pointer(text(title)), RoomName: roomName(room), Description: planDescription(plan), RoomDescription: pointer(text(byLocate(room, "roomType-Remark"))), Price: price, Meals: meals(room), SourceURL: d.Source.URL, BookingURL: u.String(), RoomAnchor: anchor, Query: q, SourceCaveat: pointer("Verify cancellation rules and any additional taxes or fees on Rakuten Travel at booking; plan rules are unassociated in this source response.")}
		offers = append(offers, offer)
	}
	if len(offers) == 0 {
		emptyRoot := nodeByClass(root, "planList")
		if emptyRoot == nil {
			emptyRoot = nodeByClass(root, "no-vacancy__description")
		}
		if len(rows) > 0 {
			emptyRoot = root
		}
		t := text(emptyRoot)
		explicit := strings.Contains(t, "空室なし") || strings.Contains(t, "空室が見つかりません") || strings.Contains(t, "条件に該当するプランはありません") || strings.Contains(t, "条件に該当するプランがありません") || strings.Contains(t, "該当する宿泊プランはありません")
		if !explicit || len(rows) > 0 && unavailable != len(rows)-catalogRows {
			return out, parseFailure(d.Source.URL, "no verified bookable rows and no explicit no-availability state")
		}
		out.Status = StatusNoAvailability
	}
	start, end, page := window(len(offers), q.Offset, q.Limit, sourceNext(root, d, "f_page_no", q.Page), q.Page, "plans", len(sourcePlans), len(rows), sourceTotal(root))
	out.Offers = append(out.Offers, offers[start:end]...)
	out.Page = page
	return out, nil
}
