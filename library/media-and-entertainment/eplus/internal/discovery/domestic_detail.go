package discovery

import (
	"encoding/json"
	"fmt"
	"golang.org/x/net/html"
	"regexp"
	"strings"
)

var sessionURLRE = regexp.MustCompile(`/sf/detail/(\d{10})-P003(\d{4})P021(\d{3})`)

func sessionID(u string) string {
	m := sessionURLRE.FindStringSubmatch(u)
	if m == nil {
		return ""
	}
	return m[1] + "/" + m[2] + "/" + m[3]
}

func parseDomesticDetail(n *html.Node, u string) ([]Row, error) {
	articles := classes(n, "block-ticket-article")
	if len(articles) == 0 {
		return nil, fmt.Errorf("domestic performance articles missing; use a public /sf/detail ID or check the event page for parser drift")
	}
	event := eventID(u)
	out := []Row{}
	ld := []Row{}
	for _, s := range nodes(n, func(x *html.Node) bool { return x.Data == "script" && attr(x, "type") == "application/ld+json" }) {
		var v Row
		if json.Unmarshal([]byte(rawText(s)), &v) == nil {
			ld = append(ld, v)
		}
	}
	for _, a := range articles {
		r := newSession()
		r["source"] = "domestic"
		r["event_id"] = event
		r["name"] = nullable(heading(n))
		r["url"] = u
		date := parseDate(ct(a, "block-ticket-article__date"))
		r["date"] = nullable(date)
		tim := ct(a, "block-ticket-article__time")
		if m := regexp.MustCompile(`開演[：:]\s*(\d{1,2}:\d{2})`).FindStringSubmatch(tim); m != nil {
			r["start_at"] = at(date, m[1])
		}
		if m := regexp.MustCompile(`開場\s*[：:]?\s*(\d{1,2}:\d{2})`).FindStringSubmatch(tim); m != nil {
			r["doors_at"] = at(date, m[1])
		}
		r["region"] = nullable(strings.Trim(ct(a, "block-ticket-article__region"), "（）() "))
		vu := absolute(u, attr(first(a, "block-ticket-article__place"), "href"))
		r["venue"] = Row{"name": nullable(ct(a, "block-ticket-article__venue")), "url": nullable(vu)}
		sid := ""
		ldCandidates := []Row{}
		for _, v := range ld {
			loc := obj(v["location"])
			start := str(v["startDate"])
			if date != "" && strings.HasPrefix(start, date) && str(loc["name"]) == ct(a, "block-ticket-article__venue") && (r["start_at"] == nil || start == str(r["start_at"])) {
				if sessionID(str(v["url"])) != "" {
					ldCandidates = append(ldCandidates, v)
				}
			}
		}
		if len(ldCandidates) == 1 {
			v := ldCandidates[0]
			sid = sessionID(str(v["url"]))
			r["url"] = v["url"]
		}

		// Booking parameters identify the performance even when JSON-LD is absent.
		for _, s := range classes(a, "block-ticket") {
			book := domesticBooking(s)
			parts := bookingParts(book)
			if parts["P3"] != nil && parts["P21"] != nil {
				sid = event + "/" + str(parts["P3"]) + "/" + str(parts["P21"])
				r["url"] = "https://eplus.jp/sf/detail/" + event + "-P003" + str(parts["P3"]) + "P021" + str(parts["P21"])
				break
			}
		}
		if sid == "" {
			sid = event + "/" + fallbackID(date, str(r["start_at"]), vu, ct(a, "block-ticket-article__title"), attr(a, "id"), attr(a, "class"))
			r["identity_kind"] = "derived"
			if len(ldCandidates) > 1 {
				r["identity_ambiguous"] = true
			}
		} else {
			r["identity_kind"] = "source"
		}
		r["id"] = sid
		sales := []Row{}
		for _, s := range classes(a, "block-ticket") {
			name := ct(s, "block-ticket__title")
			kind := saleKind(ct(s, "label-ticket"), "")
			label := ct(s, "ticket-status")
			status, inv := saleState(label, "", kind, false, true)
			start, end := window(ct(s, "block-ticket__time"))
			book := domesticBooking(s)
			parts := bookingParts(book)
			round := str(parts["P7"])
			if round == "" {
				round = fallbackID(name, str(start), str(end))
			}
			sale := newSale(sid+"/round/"+round, name, kind, status, inv, start, end, nullable(book))
			sale["source_round_code"] = nullable(str(parts["P7"]))
			sale["source_status"] = nullable(label)
			sale["source_kind"] = nullable(ct(s, "label-ticket"))
			sale["booking_url_kind"] = "sale_handoff"
			for _, icon := range classes(s, "ticket-icon__item") {
				sale["collection"] = append(sale["collection"].([]string), text(icon))
			}
			for _, btn := range nodes(s, func(x *html.Node) bool { return x.Data == "button" }) {
				if t := attr(btn, "data-title"); t != "" {
					sale["eligibility"] = append(sale["eligibility"].([]string), t)
				}
			}
			sales = append(sales, sale)
		}
		r["sales"] = sales
		if notes := ct(a, "block-ticket-article__detail"); notes != "" {
			r["notes"] = []string{notes}
		}
		for _, previous := range out {
			if previous["id"] == r["id"] {
				return nil, fmt.Errorf("ambiguous performance identity: multiple same-day articles lack distinct source IDs or times; inspect the event page")
			}
		}
		out = append(out, r)
	}
	return out, nil
}
func domesticBooking(n *html.Node) string {
	for _, btn := range nodes(n, func(x *html.Node) bool { return x.Data == "button" || x.Data == "a" }) {
		if m := bookingRE.FindString(attr(btn, "onclick")); m != "" {
			return m
		}
		if m := bookingRE.FindString(attr(btn, "href")); m != "" {
			return m
		}
	}
	return ""
}
