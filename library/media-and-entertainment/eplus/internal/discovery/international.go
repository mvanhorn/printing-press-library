package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/eplus/internal/cliutil"
	"golang.org/x/net/html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var internationalCategories = map[string]string{"concert": "concert", "theatre": "art-theater", "art": "art-theater", "sports": "sports", "event": "japan-culture", "culture": "japan-culture", "anime": "anime-games", "festival": "music-festival"}
var ibSlugRE = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
var productIDRE = regexp.MustCompile(`^[0-9]{1,10}$`)

// InternationalURL accepts public tour slugs and product links, retaining session selection.
func InternationalURL(id string) (string, error) {
	if productIDRE.MatchString(id) {
		return productURL(id, "0"), nil
	}
	if ibSlugRE.MatchString(id) && !reservedIBPath(id) {
		return "https://ib.eplus.jp/" + id, nil
	}
	u, e := url.Parse(id)
	if e != nil || u.Scheme != "https" || u.Host != "ib.eplus.jp" || u.User != nil {
		return "", fmt.Errorf("use an international product ID, tour slug or https://ib.eplus.jp public detail URL")
	}
	q := u.Query()
	if q.Get("dispatch") == "products.view" && u.Path == "/index.php" && productIDRE.MatchString(q.Get("product_id")) {
		date := q.Get("date")
		if date == "" {
			date = "0"
		}
		if !productIDRE.MatchString(date) {
			return "", fmt.Errorf("invalid international session date selector")
		}
		return productURL(q.Get("product_id"), date), nil
	}
	slug := strings.Trim(u.Path, "/")
	if u.RawQuery == "" && ibSlugRE.MatchString(slug) && !reservedIBPath(slug) {
		return "https://ib.eplus.jp/" + slug, nil
	}
	return "", fmt.Errorf("unsupported international detail URL; use the tour or products.view link from international search")
}
func reservedIBPath(s string) bool {
	switch strings.ToLower(s) {
	case "cart", "checkout", "orders", "profiles", "login", "logout", "index.php", "clear-cart":
		return true
	}
	return false
}
func productURL(id, date string) string {
	return "https://ib.eplus.jp/index.php?" + url.Values{"dispatch": {"products.view"}, "product_id": {id}, "date": {date}, "sl": {"en"}, "cr": {"JPY"}}.Encode()
}

func (c *Client) InternationalSearch(ctx context.Context, o SearchOptions, maxScan int) (Result, error) {
	if e := ValidateInternationalSearch(o, maxScan); e != nil {
		return Result{}, e
	}
	u := "https://ib.eplus.jp/"
	if o.Category != "" {
		p := internationalCategories[o.Category]
		if p == "" {
			return Result{}, fmt.Errorf("international --category must be concert, theatre, art, sports, culture, event, anime or festival")
		}
		u += p
	}
	b, ob, e := c.Fetch(ctx, u)
	if e != nil {
		return Result{}, e
	}
	n, e := document(b)
	if e != nil {
		return Result{}, e
	}
	catalog, e := parseCatalog(n, u)
	if e != nil {
		return Result{}, e
	}
	out := []Row{}
	obs := []Observation{ob}
	warnings := []string{"international offerings are a separate catalog; absence here says nothing about domestic listings or overseas eligibility"}
	partial := false
	scanned := 0
	detailReads := 0
	failures := []Row{}
	for _, r := range catalog {
		if scanned >= maxScan || len(out) >= o.Limit {
			partial = true
			break
		}
		scanned++
		hay := strings.ToLower(str(r["name"]))
		if o.Keyword != "" && !strings.Contains(hay, strings.ToLower(o.Keyword)) {
			continue
		}
		if o.Artist != "" && !strings.Contains(hay, strings.ToLower(o.Artist)) {
			continue
		}
		if o.From != "" || o.To != "" || o.Venue != "" || o.Location != "" {
			detailReads++
			detail, err := c.InternationalDetail(ctx, str(r["url"]))
			if err != nil {
				var rate *cliutil.RateLimitError
				if errors.As(err, &rate) {
					return Result{}, err
				}
				failures = append(failures, Row{"url": r["url"], "error": err.Error()})
				partial = true
				continue
			}
			obs = append(obs, detail.Meta["observations"].([]Observation)...)
			for _, d := range detail.Data {
				if matches(d, o) {
					out = append(out, d)
					if len(out) >= o.Limit {
						partial = true
						break
					}
				}
			}
			continue
		}
		out = append(out, r)
	}
	if len(failures) > 0 && len(out) == 0 {
		return Result{}, fmt.Errorf("all matching international detail reads failed: %s", failures[0]["error"])
	}
	if len(failures) > 0 {
		warnings = append(warnings, fmt.Sprintf("%d of %d international detail reads failed; results are partial", len(failures), detailReads))
	}
	r := result(c, out, obs, len(catalog), partial, warnings)
	r.Meta["source"] = "international"
	r.Meta["search_url"] = u
	r.Meta["scanned_records"] = scanned
	r.Meta["max_scan"] = maxScan
	r.Meta["fetch_failures"] = failures
	if len(out) == 0 {
		r.Meta["note"] = "no match in this bounded international catalog; refine the filters or raise --max-scan; domestic offerings remain separate"
	}
	return r, nil
}
func parseCatalog(n *html.Node, u string) ([]Row, error) {
	if first(n, "top-product-list") == nil {
		return nil, fmt.Errorf("international catalog contract missing; public page may have changed")
	}
	out := []Row{}
	seen := map[string]bool{}
	for _, a := range classes(n, "col-product") {
		links := nodes(a, func(x *html.Node) bool { return x.Data == "a" && attr(x, "href") != "" })
		if len(links) == 0 {
			continue
		}
		link := absolute(u, attr(links[0], "href"))
		if _, e := InternationalURL(link); e != nil {
			continue
		}
		if seen[link] {
			continue
		}
		seen[link] = true
		ga := first(a, "ga-product-click")
		name := attr(ga, "data-name")
		if name == "" {
			name = text(links[0])
		}
		if name == "" {
			return nil, fmt.Errorf("international catalog entry has no name")
		}
		r := newSession()
		r["id"] = "ib:tour:" + strings.TrimPrefix(link, "https://ib.eplus.jp/")
		r["event_id"] = r["id"]
		r["source_tour_id"] = nullable(attr(ga, "data-id"))
		r["source"] = "international"
		r["name"] = name
		r["url"] = link
		r["category"] = nullable(attr(ga, "data-category"))
		r["record_type"] = "tour"
		r["detail_loaded"] = false
		out = append(out, r)
	}
	return out, nil
}
func (c *Client) InternationalDetail(ctx context.Context, id string) (Result, error) {
	u, e := InternationalURL(id)
	if e != nil {
		return Result{}, e
	}
	b, ob, e := c.Fetch(ctx, u)
	if e != nil {
		return Result{}, e
	}
	n, e := document(b)
	if e != nil {
		return Result{}, e
	}
	obs := []Observation{ob}
	rows := []Row{}
	warnings := []string{}
	parsed, e := url.Parse(u)
	if e != nil {
		return Result{}, e
	}
	if parsed.Query().Get("dispatch") == "products.view" {
		row, comb, e := parseProduct(n, u)
		if e != nil {
			return Result{}, e
		}
		rows = append(rows, row)
		if comb != "" {
			priceURL := "https://ib.eplus.jp/index.php?" + url.Values{"dispatch": {"products.get_ticket_type"}, "product_id": {parsed.Query().Get("product_id")}, "combination": {comb}, "is_ajax": {"1"}, "cr": {"JPY"}, "sl": {"en"}}.Encode()
			b, pob, err := c.fetch(ctx, priceURL, map[string]string{"X-Requested-With": "XMLHttpRequest"}, true)
			obs = append(obs, pob)
			if err != nil {
				var rate *cliutil.RateLimitError
				if errors.As(err, &rate) {
					return Result{}, err
				}
				warnings = append(warnings, "ticket price/inventory read failed: "+err.Error())
			} else if err = applyTicketData(row, b, comb); err != nil {
				warnings = append(warnings, err.Error())
			}
		} else {
			warnings = append(warnings, "variant price and inventory require selection on the website; static zero prices are placeholders, not free tickets")
		}
	} else {
		rows, e = parseTour(n, u)
		if e != nil {
			return Result{}, e
		}
		warnings = append(warnings, "tour schedules are summaries; inspect each product URL for exact times, ticket prices and event-specific terms")
	}
	r := result(c, rows, obs, len(rows), len(warnings) > 0 && parsed.Query().Get("dispatch") == "products.view", warnings)
	r.Meta["source"] = "international"
	return r, nil
}
func parseTour(n *html.Node, u string) ([]Row, error) {
	schedules := classes(n, "group-schedule-row")
	if len(schedules) == 0 {
		return nil, fmt.Errorf("international tour schedule missing; use a catalog tour URL or a products.view link")
	}
	out := []Row{}
	for _, a := range schedules {
		button := first(a, "group-schedule-btn")
		link, e := InternationalURL(absolute(u, attr(button, "href")))
		if e != nil {
			continue
		}
		pu, _ := url.Parse(link)
		r := newSession()
		r["id"] = "ib:product:" + pu.Query().Get("product_id") + "/date/" + pu.Query().Get("date")
		r["product_id"] = pu.Query().Get("product_id")
		r["event_id"] = "ib:tour:" + strings.TrimPrefix(u, "https://ib.eplus.jp/")
		r["source"] = "international"
		r["record_type"] = "product_summary"
		r["name"] = nullable(ct(a, "group-schedule-date"))
		r["url"] = link
		r["venue"] = Row{"name": nullable(ct(a, "group-schedule-name"))}
		r["region"] = nullable(ct(n, "group-schedule-title"))
		dates := englishDateRE.FindAllString(ct(a, "group-schedule-mute"), -1)
		if len(dates) > 0 {
			r["date"] = nullable(parseDate(dates[0]))
			r["date_end"] = nullable(parseDate(dates[len(dates)-1]))
		}
		r["detail_loaded"] = false
		out = append(out, r)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("international schedule has no supported product handoff links")
	}
	return out, nil
}
func byID(n *html.Node, id string) *html.Node {
	v := nodes(n, func(x *html.Node) bool { return attr(x, "id") == id })
	if len(v) > 0 {
		return v[0]
	}
	return nil
}
func parseProduct(n *html.Node, u string) (Row, string, error) {
	h := first(n, "ty-product-block-title")
	if h == nil {
		return nil, "", fmt.Errorf("international product title missing; source returned another page")
	}
	pu, _ := url.Parse(u)
	id := pu.Query().Get("product_id")
	r := newSession()
	r["id"] = "ib:product:" + id + "/date/" + pu.Query().Get("date")
	r["product_id"] = id
	r["source"] = "international"
	r["record_type"] = "product"
	r["name"] = text(h)
	r["url"] = u
	r["detail_loaded"] = true
	r["overseas_bookability"] = "conditional"
	desc := text(byID(n, "content_description"))
	agreement := text(byID(n, "agreement-content-description"))
	note := ct(n, "ty-product-block__note")
	r["conditions"] = []string{}
	if desc != "" {
		r["conditions"] = append(r["conditions"].([]string), desc)
	}
	if agreement != "" {
		r["eligibility"] = []string{agreement}
	}
	r["terms_status"] = "partial"
	r["notes"] = []string{note}
	if m := regexp.MustCompile(`(?i)Start Time:\s*([^*]+)`).FindStringSubmatch(desc); m != nil {
		if ds := englishDateRE.FindStringSubmatch(m[1]); ds != nil {
			r["date"] = nullable(parseDate(ds[0]))
			r["start_at"] = at(str(r["date"]), ds[4]+":"+ds[5])
		}
	}
	if r["date"] == nil {
		for _, x := range nodes(n, func(x *html.Node) bool { return x.Data == "meta" && attr(x, "name") == "description" }) {
			dates := englishDateRE.FindAllString(attr(x, "content"), -1)
			if len(dates) > 0 {
				r["date"] = nullable(parseDate(dates[0]))
				if len(dates) > 1 {
					r["date_end"] = nullable(parseDate(dates[1]))
				}
			}
		}
	}
	// Venue/ticket IDs come from real option fields; default prompt options are skipped.
	tickets := []Row{}
	for _, g := range classes(n, "ty-product-options__item") {
		label := ct(g, "ty-control-group__label")
		value := ct(g, "ty-control-group__item")
		if value == "" {
			value = strings.TrimSpace(strings.TrimPrefix(text(g), label))
		}
		if strings.HasPrefix(label, "Venue") {
			r["venue"] = Row{"name": nullable(value)}
		}
		if strings.HasPrefix(label, "Ticket type") && value != "" && !strings.Contains(value, "Select") {
			tickets = append(tickets, Row{"name": value, "price": nil, "currency": "JPY"})
		}
	}
	r["tickets"] = tickets
	start, end := window(note)
	status, inv := "unknown", "unknown"
	label := ct(n, "ty-product-coming-soon")
	if strings.Contains(strings.ToLower(label), "over") {
		status = "closed"
	} else if strings.Contains(strings.ToLower(label), "coming") {
		status = "upcoming"
	}
	sale := newSale(str(r["id"])+"/sale", "International ticket sale", "first_come", status, inv, start, end, u)
	sale["source_status"] = nullable(label)
	sale["booking_url_kind"] = "product_handoff"
	r["sales"] = []Row{sale}
	if strings.Contains(strings.ToLower(desc+note), "charge included") {
		r["fees"] = []string{"source states handling/service charge included in ticket price"}
	}
	comb := attr(byID(n, "only_seat_type"), "data-combination-id")
	if !regexp.MustCompile(`^\d+(?:_\d+){1,15}$`).MatchString(comb) {
		comb = ""
	}
	return r, comb, nil
}
func applyTicketData(r Row, b []byte, comb string) error {
	var envelope Row
	if json.Unmarshal(b, &envelope) != nil {
		return fmt.Errorf("ticket price endpoint did not return JSON; price and inventory remain unknown")
	}
	d := obj(envelope["data"])
	if d["status"] != true {
		return fmt.Errorf("ticket price endpoint has no confirmed selected variant; price and inventory remain unknown")
	}
	raw := strings.ReplaceAll(str(d["price"]), ",", "")
	price, e := strconv.ParseFloat(raw, 64)
	if e != nil || price < 0 || str(d["currency_code"]) != "JPY" {
		return fmt.Errorf("ticket price/currency contract changed; price remains unknown")
	}
	r["tickets"] = []Row{{"id": comb, "name": nullable(ticketName(r)), "price": price, "currency": "JPY", "price_source": "selected_variant"}}
	sale := r["sales"].([]Row)[0]
	amount, ok := d["amount"].(float64)
	sale["source_amount"] = d["amount"]
	// A current selection response is a seat signal only while the round is open.
	if sale["status"] != "closed" && sale["status"] != "upcoming" {
		if start := sourceDateTime(str(d["start_selling"])); start != nil {
			sale["starts_at"] = start
		}
		if end := sourceDateTime(str(d["end_selling"])); end != nil {
			sale["ends_at"] = end
		}
		start, end := sale["starts_at"], sale["ends_at"]
		switch {
		case start != nil && timeNow().Before(mustTime(str(start))):
			sale["status"] = "upcoming"
		case end != nil && timeNow().After(mustTime(str(end))):
			sale["status"] = "closed"
		case ok && amount == 0:
			sale["status"] = "sold_out"
			sale["inventory"] = "unavailable"
		case ok && amount > 0:
			sale["status"] = "accepting"
			sale["inventory"] = "available"
		}
	}
	r["inventory"] = sale["inventory"]
	return nil
}
func ticketName(r Row) string {
	a, _ := r["tickets"].([]Row)
	if len(a) > 0 {
		return str(a[0]["name"])
	}
	return ""
}
func sourceDateTime(s string) any {
	if t, e := time.ParseInLocation("2006/01/02 15:04:05", s, jst); e == nil {
		return t.Format(time.RFC3339)
	}
	return nil
}
func mustTime(s string) time.Time { t, _ := time.Parse(time.RFC3339, s); return t }

func (c *Client) Policies(ctx context.Context) (Result, error) {
	u := "https://ib.eplus.jp/faq"
	b, ob, e := c.Fetch(ctx, u)
	if e != nil {
		return Result{}, e
	}
	n, e := document(b)
	if e != nil {
		return Result{}, e
	}
	body := ct(n, "ty-wysiwyg-content")
	if !strings.Contains(body, "payment methods") {
		for _, x := range classes(n, "ty-wysiwyg-content") {
			if strings.Contains(text(x), "payment methods") {
				body = text(x)
				break
			}
		}
	}
	if !strings.Contains(body, "VISA") || !strings.Contains(body, "overseas") {
		return Result{}, fmt.Errorf("international FAQ contract changed; consult %s", u)
	}
	eligibility := faqAnswer(body, "Japanese")
	payment := faqAnswer(body, "payment methods")
	fees := faqAnswer(body, "fee other")
	collection := faqAnswer(body, "enter the venue")
	if eligibility == "" || payment == "" || fees == "" || collection == "" {
		return Result{}, fmt.Errorf("international FAQ sections changed; consult %s", u)
	}
	rows := []Row{{"id": "international-access", "url": u, "scope": "international_generic", "eligibility": eligibility, "payment": payment, "fees": fees, "collection": collection, "domestic_overseas_bookability": "unknown"}}
	return result(c, rows, []Observation{ob}, 1, false, []string{"generic FAQ guidance does not override event-specific restrictions or prove domestic bookability"}), nil
}

func ValidateInternationalSearch(o SearchOptions, maxScan int) error {
	if o.Limit < 1 || o.Limit > 100 || maxScan < 1 || maxScan > 20 {
		return fmt.Errorf("--limit must be 1..100 and --max-scan must be 1..20")
	}
	if e := ValidateDate(o.From); e != nil {
		return e
	}
	if e := ValidateDate(o.To); e != nil {
		return e
	}
	if o.From != "" && o.To != "" && o.From > o.To {
		return fmt.Errorf("--from must be on or before --to")
	}

	if o.Category != "" && internationalCategories[o.Category] == "" {
		return fmt.Errorf("unknown international --category %q", o.Category)
	}
	return nil
}

func faqAnswer(body, phrase string) string {
	for _, chunk := range regexp.MustCompile(`(?i)Q:`).Split(body, -1) {
		parts := regexp.MustCompile(`(?i)\s*A:`).Split(chunk, 2)
		if len(parts) == 2 && strings.Contains(strings.ToLower(parts[0]), strings.ToLower(phrase)) {
			return strings.TrimSpace(parts[1])
		}
	}
	return ""
}
