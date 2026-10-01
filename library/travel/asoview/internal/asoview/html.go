package asoview

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

//go:embed inventory.json
var bundledInventory []byte

func walk(n *html.Node, fn func(*html.Node)) {
	fn(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, fn)
	}
}
func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
func hasClass(n *html.Node, class string) bool {
	for _, c := range strings.Fields(attr(n, "class")) {
		if c == class {
			return true
		}
	}
	return false
}
func nodeText(n *html.Node) string {
	var b strings.Builder
	walk(n, func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
	})
	return strings.Join(strings.Fields(b.String()), " ")
}
func classText(n *html.Node, class string) string {
	out := ""
	walk(n, func(n *html.Node) {
		if out == "" && hasClass(n, class) {
			out = nodeText(n)
		}
	})
	return out
}
func datasource(raw []byte) (Object, *html.Node, error) {
	doc, err := html.Parse(bytes.NewReader(raw))
	if err != nil {
		return nil, nil, fail(5, "parsing source HTML: %v", err)
	}
	var data Object
	walk(doc, func(n *html.Node) {
		if data != nil || n.Type != html.ElementNode || n.Data != "script" || n.FirstChild == nil {
			return
		}
		s := n.FirstChild.Data
		i := strings.Index(s, "ASOVIEW_DATASOURCE = ")
		if i < 0 {
			return
		}
		tail := s[i+len("ASOVIEW_DATASOURCE = "):]
		d := json.NewDecoder(strings.NewReader(tail))
		d.UseNumber()
		_ = d.Decode(&data)
	})
	if data == nil {
		return nil, nil, fail(5, "Asoview HTML has no usable public datasource (layout changed or access shell)")
	}
	return data, doc, nil
}

func (c *Client) Product(ctx context.Context, input string, full bool) (Object, error) {
	kind, id, err := ParseID(input)
	if err != nil {
		return nil, err
	}
	raw, err := c.Get(ctx, "/item/"+kind+"/"+id+"/", url.Values{}, 24*time.Hour)
	if err != nil {
		return nil, err
	}
	ds, _, err := datasource(raw)
	if err != nil {
		return nil, err
	}
	p := object(ds["plan"])
	if kind == "ticket" {
		p = object(ds["ticketType"])
	}
	sourceID := text(p["planCode"])
	if kind == "ticket" {
		sourceID = text(p["code"])
	}
	if sourceID != id || text(p["title"]) == "" {
		return nil, fail(5, "Asoview product identity/schema mismatch for %s", id)
	}
	opts := []Object{}
	advertised := price(p["sellingPrice"], nil, "advertised_from", nil)
	validity := Object{"type": "experience_date_required", "start_text": nil, "end_text": nil, "end_date": nil, "relative_period": nil, "usable_days_text": nil, "sales_period": nil, "unavailable_periods": []any{}}
	entry := Object{"selection": "experience_start_required", "reserved_slot": nil, "timezone": "Asia/Tokyo"}
	inclusions := nullable(p["priceIncluded"])
	conditions := any(nil)
	cancellation := any(p["cancelPolicies"])
	age := AgeBand(text(p["ageLimit"]))
	participants := nullable(p["reservableParticipant"])
	state := nullable(ds["planSummaryReservableType"])
	notAvailable := nullable(ds["notReservableMessage"])
	if kind == "ticket" {
		opts, err = optionsFrom(list(p["tickets"]), false, nil)
		if err != nil {
			return nil, err
		}
		var minimum *int64
		for _, o := range opts {
			v, _ := object(o["price"])["amount"].(*int64)
			if v != nil && (minimum == nil || *v < *minimum) {
				x := *v
				minimum = &x
			}
		}
		advertised = price(nil, nil, "advertised_minimum_across_bands", nil)
		advertised["amount"] = minimum
		ov := object(p["overview"])
		vp := object(p["validityPeriod"])
		scheduled := flag(ds["isScheduledTicket"])
		vtype := "general_admission_validity"
		sel := "no_reserved_slot_in_product"
		if scheduled {
			vtype = "selected_date_required"
			sel = "date_or_time_selection_required"
		}
		validity = Object{"type": vtype, "start_text": nullable(ov["effectivePeriodFromText"]), "end_text": nullable(ov["effectivePeriodToText"]), "end_date": nullable(vp["endDate"]), "relative_period": nil, "is_date_period": vp["isDatePeriod"], "is_term_period": vp["isTermPeriod"], "usable_days_text": nullable(ov["displayAvailableDayText"]), "sales_period": nullable(ov["salesPeriod"]), "unavailable_periods": list(p["unablePeriods"])}
		if flag(vp["isTermPeriod"]) {
			validity["relative_period"] = Object{"value": integer(vp["termPeriod"]), "unit": nullable(vp["termPeriodLabel"]), "source_text": nullable(ov["effectivePeriodToText"])}
		}
		entry = Object{"selection": sel, "reserved_slot": nil, "timezone": "Asia/Tokyo"}
		inclusions = nullable(ov["description"])
		conditions = nullable(p["usingCondition"])
		cancellation = p["cancellationPolicy"]
		age = AgeBand("")
		participants = nil
		state = nullable(p["ticketTypePurchasableType"])
		notAvailable = nullable(p["notPurchasableMessage"])
	}
	related := []Object{}
	for _, v := range list(ds["planList"]) {
		m := object(v)
		rid := text(m["goodsId"])
		if rid == id {
			continue
		}
		if BookingURL(rid) != "" && len(related) < 20 {
			related = append(related, Object{"id": rid, "name_ja": nullable(m["title"]), "booking_url": BookingURL(rid)})
		}
	}
	out := Object{"id": id, "kind": kind, "name_ja": p["title"], "booking_url": BookingURL(id), "base": Object{"id": idString(ds["baseNumber"]), "name_ja": nullable(ds["baseName"])}, "location": Object{"prefecture_id": nullable(ds["prefectureCode"]), "prefecture_ja": nullable(p["prefectureName"]), "region_id": nullable(ds["regionCode"]), "area_id": nullable(ds["areaCode"]), "area_ja": nullable(p["areaName"]), "latitude": ds["baseLatitude"], "longitude": ds["baseLongitude"]}, "category": Object{"id": nullable(ds["categoryCode"]), "genre_id": nullable(ds["genreCode"]), "name_ja": nullable(p["mainGenreName"])}, "advertised_price": advertised, "options": opts, "age_band": age, "participants_text": participants, "inclusions": inclusions, "conditions": conditions, "cancellation": cancellation, "validity": validity, "entry": entry, "duration_text": nullable(p["experienceTime"]), "state": state, "not_available_message": notAvailable, "related_products": related, "related_products_partial": len(list(ds["planList"])) > len(related)+1, "date_party_total": nil, "language": "ja"}
	if full {
		out["description"] = nullable(p["description"])
		out["introductions"] = list(p["introductions"])
		out["user_manual"] = nullable(p["userManual"])
		out["additional_fees"] = list(p["additionalFees"])
		out["user_charges"] = list(p["userCharges"])
	}
	return out, nil
}

type SearchParams struct {
	Region, Category, Query, Date, Cursor, Kind  string
	Page, Offset, Pages, Limit, Adults, Children int
}

func (c *Client) Discover(ctx context.Context, p SearchParams) (Object, error) {
	if err := validLimit(p.Limit, 50); err != nil {
		return nil, err
	}
	if p.Pages < 1 || p.Pages > 5 {
		return nil, fail(2, "--pages must be 1..5")
	}
	if p.Page < 1 || p.Page > 200 {
		return nil, fail(2, "--page must be 1..200")
	}
	if p.Adults < 0 || p.Children < 0 || p.Adults+p.Children < 1 || p.Adults+p.Children > 50 {
		return nil, fail(2, "--adults + --children must be 1..50")
	}
	if len(p.Query) > 300 {
		return nil, fail(2, "--query exceeds 300 bytes")
	}
	if p.Date != "" {
		if _, err := ParseDate(p.Date); err != nil {
			return nil, err
		}
	}
	if p.Kind != "" && p.Kind != "ticket" && p.Kind != "activity" {
		return nil, fail(2, "--kind must be ticket or activity")
	}
	if p.Cursor != "" {
		parts := strings.Split(p.Cursor, ":")
		if len(parts) != 2 {
			return nil, fail(2, "invalid --cursor; use the returned page:offset value")
		}
		var e error
		p.Page, e = strconv.Atoi(parts[0])
		if e != nil || p.Page < 1 || p.Page > 200 {
			return nil, fail(2, "invalid --cursor page")
		}
		p.Offset, e = strconv.Atoi(parts[1])
		if e != nil || p.Offset < 0 || p.Offset > 100 {
			return nil, fail(2, "invalid --cursor offset")
		}
	}
	q := url.Values{"adultQuantity": {strconv.Itoa(p.Adults)}, "childQuantity": {strconv.Itoa(p.Children)}, "sort": {"asoview"}}
	if p.Region != "" {
		r, err := c.resolveInventory("regions", p.Region)
		if err != nil {
			return nil, err
		}
		q.Set("destinationType", text(r["type"]))
		q.Set("destinationId", text(r["id"]))
	}
	if p.Category != "" {
		r, err := c.resolveInventory("categories", p.Category)
		if err != nil {
			return nil, err
		}
		t := text(r["type"])
		if t == "category-group" {
			t = "categoryGroup"
		}
		q.Set("leisureType", t)
		q.Set("leisureId", text(r["id"]))
	}
	if p.Date != "" {
		q.Set("experienceDate", p.Date)
	}
	results := []Object{}
	seen := map[string]bool{}
	scanned := 0
	pagesRead := 0
	var total any
	var next any
	for n := 0; n < p.Pages; n++ {
		page := p.Page + n
		q.Set("page", strconv.Itoa(page))
		raw, err := c.Get(ctx, "/search/", q, 5*time.Minute)
		if err != nil {
			return nil, err
		}
		ds, doc, err := datasource(raw)
		if err != nil {
			return nil, err
		}
		pagesRead++
		// Echo checks catch ignored upstream filter vocabulary instead of returning unrelated cards.
		if _, ok := ds["caughtBasePlanPriceList"].([]any); !ok || integer(strings.ReplaceAll(text(ds["displayFilterTotalCount"]), ",", "")) == nil {
			return nil, fail(5, "Asoview discovery schema changed; matched list or total missing")
		}
		query := object(ds["query"])
		for _, key := range []string{"destinationType", "destinationId", "leisureType", "leisureId", "experienceDate", "adultQuantity", "childQuantity", "page"} {
			if q.Get(key) != "" && text(query[key]) != q.Get(key) && !(key == "childQuantity" && q.Get(key) == "0" && query[key] == nil) {
				return nil, fail(5, "Asoview did not acknowledge search filter %s=%s", key, q.Get(key))
			}
		}
		total = nullable(ds["displayFilterTotalCount"])
		cards, hasNext := parseCards(doc, ds, page)
		offset := 0
		if n == 0 {
			offset = p.Offset
		}
		if offset > len(cards) {
			return nil, fail(2, "--cursor offset exceeds this source page")
		}
		next = nil
		for i := offset; i < len(cards); i++ {
			row := cards[i]
			scanned++
			id := text(row["id"])
			if seen[id] {
				continue
			}
			seen[id] = true
			if p.Kind != "" && row["kind"] != p.Kind {
				continue
			}
			hay := strings.ToLower(text(row["name_ja"]) + " " + text(object(row["base"])["name_ja"]) + " " + text(object(row["category"])["name_ja"]))
			if !strings.Contains(hay, strings.ToLower(strings.TrimSpace(p.Query))) {
				continue
			}
			results = append(results, row)
			if len(results) >= p.Limit {
				if i+1 < len(cards) {
					next = strconv.Itoa(page) + ":" + strconv.Itoa(i+1)
				} else if hasNext {
					next = strconv.Itoa(page+1) + ":0"
				}
				break
			}
		}
		if len(results) >= p.Limit {
			break
		}
		if !hasNext {
			break
		}
		next = strconv.Itoa(page+1) + ":0"
	}
	return Object{"results": results, "coverage": Object{"source_total_venues_text": total, "scanned_cards": scanned, "pages_read": pagesRead, "next_cursor": next, "partial": next != nil, "keyword_scope": "local_substring_over_scanned_cards", "requested_date": strptr(p.Date), "requested_party": Object{"adults": p.Adults, "children": p.Children}, "date_stock_confirmed": false}, "query": Object{"region": strptr(p.Region), "category": strptr(p.Category), "text": strptr(p.Query), "kind": strptr(p.Kind)}}, nil
}
func parseCards(doc *html.Node, ds Object, page int) ([]Object, bool) {
	prices := map[string]Object{}
	for _, k := range []string{"caughtBasePlanPriceList"} {
		for _, v := range list(ds[k]) {
			m := object(v)
			prices[text(m["goodsId"])] = m
		}
	}
	rows := []Object{}
	matchedBases := map[string]bool{}
	unknownBase := false
	next := false
	walk(doc, func(n *html.Node) {
		if n.Type != html.ElementNode {
			return
		}
		if hasClass(n, "search-result-list__item") {
			baseID := ""
			walk(n, func(child *html.Node) {
				if hasClass(child, "search-result-list__image-link") {
					baseID = strings.Trim(strings.TrimPrefix(attr(child, "href"), "/base/"), "/")
				}
			})
			walk(n, func(child *html.Node) {
				if !hasClass(child, "search-result-list__plan-link") {
					return
				}
				href := attr(child, "href")
				kind, id, err := ParseID(Origin + href)
				if err != nil {
					return
				}
				pm, matched := prices[id]
				if !matched {
					return
				}
				name := classText(child, "search-result-list__plan-name")
				if name == "" {
					return
				}
				if baseID != "" {
					matchedBases[baseID] = true
				} else {
					unknownBase = true
				}
				rows = append(rows, Object{"id": id, "kind": kind, "name_ja": name, "booking_url": BookingURL(id), "base": Object{"id": strptr(baseID), "name_ja": strptr(classText(n, "search-result-list__base-name"))}, "location": Object{"prefecture_ja": strptr(classText(n, "search-result-list__prefecture")), "area_ja": strptr(classText(n, "search-result-list__small-area"))}, "category": Object{"name_ja": strptr(classText(child, "search-result-list__plan-genre"))}, "age_band": AgeBand(classText(child, "search-result-list__plan-target-age")), "advertised_price": price(pm["sellingPrice"], nil, "advertised_search_from", nil), "date_party_total": nil})
			})
		}
		if n.Data == "a" {
			u, e := url.Parse(attr(n, "href"))
			if e == nil && u.Path == "/search/" {
				v, _ := strconv.Atoi(u.Query().Get("page"))
				if v == page+1 {
					next = true
				}
			}
		}
	})
	total := integer(strings.ReplaceAll(text(ds["displayFilterTotalCount"]), ",", ""))
	// The total counts venues, while rows count products. Recommendation
	// cards neither establish exhaustion nor invalidate an explicit next link.
	minimumVenues := len(matchedBases)
	if unknownBase && minimumVenues == 0 {
		minimumVenues = 1
	}
	if total != nil && *total <= int64(minimumVenues) {
		next = false
	}
	return rows, next
}

func (c *Client) resolveInventory(kind, input string) (Object, error) {
	inv, _, err := c.loadInventory(kind)
	if err != nil {
		return nil, err
	}
	for _, v := range list(inv[kind]) {
		r := object(v)
		if input == text(r["id"]) || input == text(r["name_ja"]) {
			return r, nil
		}
	}
	return nil, fail(2, "unknown %s %q; use inventory --kind %s --query NAME to find the source ID", kind, input, kind)
}
func (c *Client) Inventory(ctx context.Context, kind, query string, limit int, refresh bool) (Object, error) {
	if kind != "regions" && kind != "categories" {
		return nil, fail(2, "--kind must be regions or categories")
	}
	if err := validLimit(limit, 100); err != nil {
		return nil, err
	}
	inv, basis, err := c.loadInventory(kind)
	if err != nil {
		return nil, err
	}
	fetched := inv["fetched_at"]
	if refresh {
		if c.Offline {
			return nil, fail(2, "--refresh-inventory requires online source access")
		}
		path := "/location/"
		if kind == "categories" {
			path = "/leisure/"
		}
		original := c.Refresh
		c.Refresh = true
		raw, err := c.Get(ctx, path, url.Values{}, time.Hour)
		c.Refresh = original
		if err != nil {
			return nil, err
		}
		doc, e := html.Parse(bytes.NewReader(raw))
		if e != nil {
			return nil, e
		}
		items := []any{}
		seen := map[string]bool{}
		walk(doc, func(n *html.Node) {
			if n.Type != html.ElementNode || n.Data != "a" {
				return
			}
			href := attr(n, "href")
			parts := strings.Split(strings.Trim(href, "/"), "/")
			if len(parts) != 2 {
				return
			}
			id := parts[1]
			typeName := ""
			label := nodeText(n)
			if kind == "regions" && parts[0] == "location" {
				if regexp.MustCompile(`^prf(0[1-9]|[1-3][0-9]|4[0-7])0000$`).MatchString(id) {
					typeName = "prefecture"
				} else if regexp.MustCompile(`^rgn(01|02|04|05|06|07|08|09|10|11|12)$`).MatchString(id) && label != "台湾" {
					typeName = "region"
				}
			}
			if kind == "categories" && parts[0] == "leisure" {
				if regexp.MustCompile(`^[0-9]+$`).MatchString(id) {
					typeName = "category"
				} else if strings.HasPrefix(id, "act") {
					typeName = "genre"
				} else if strings.HasPrefix(id, "grp") {
					typeName = "category-group"
				}
				label = strings.TrimSuffix(label, "の一覧から探す")
			}
			if typeName != "" && label != "" && !seen[id] {
				seen[id] = true
				items = append(items, Object{"id": id, "name_ja": label, "type": typeName, "url": Origin + href})
			}
		})
		if len(items) == 0 {
			return nil, fail(5, "Asoview inventory layout changed; no usable taxonomy")
		}
		inv[kind] = items
		fetched = c.Sources[len(c.Sources)-1].FetchedAt
		basis = "explicit_live_inventory_refresh"
		if !c.NoCache {
			if err = c.persistInventory(kind, Object{kind: items, "fetched_at": fetched}); err != nil {
				return nil, fail(10, "inventory cache write: %v", err)
			}
		}
	}
	if !refresh {
		path := "/location/"
		if kind == "categories" {
			path = "/leisure/"
		}
		c.Sources = append(c.Sources, Source{Origin + path, text(fetched), true, 0})
		if basis == "locally_refreshed_first_party_inventory" {
			c.Stats.CacheHits++
		}
	}
	items := []Object{}
	matches := 0
	for _, v := range list(inv[kind]) {
		r := object(v)
		if !strings.Contains(strings.ToLower(text(r["name_ja"])+" "+text(r["id"])), strings.ToLower(strings.TrimSpace(query))) {
			continue
		}
		matches++
		if len(items) < limit {
			items = append(items, r)
		}
	}
	sort.Slice(items, func(i, j int) bool { return text(items[i]["id"]) < text(items[j]["id"]) })
	return Object{"results": items, "inventory": Object{"kind": kind, "basis": basis, "fetched_at": fetched, "matching_count": matches, "returned_count": len(items), "partial": matches > len(items), "refresh_is_explicit": true}}, nil
}

func (c *Client) loadInventory(kind string) (Object, string, error) {
	var inv Object
	path := filepath.Join(c.CacheDir, "taxonomy-"+kind+".json")
	if !c.NoCache {
		raw, err := readCacheFile(c.CacheDir, filepath.Base(path), 2<<20)
		if err == nil && json.Unmarshal(raw, &inv) == nil && len(list(inv[kind])) > 0 && inv["fetched_at"] != nil {
			return inv, "locally_refreshed_first_party_inventory", nil
		}
	}

	if err := decodeJSON(bundledInventory, &inv); err != nil {
		return nil, "", err
	}
	return inv, "bundled_first_party_inventory", nil
}
func (c *Client) persistInventory(kind string, inv Object) error {
	if err := os.MkdirAll(c.CacheDir, 0700); err != nil {
		return err
	}
	raw, err := json.Marshal(inv)
	if err != nil {
		return err
	}
	if len(raw) > 2<<20 {
		return fail(5, "taxonomy snapshot exceeds 2 MiB")
	}
	f, err := os.CreateTemp(c.CacheDir, ".taxonomy-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err = f.Write(raw); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmp, filepath.Join(c.CacheDir, "taxonomy-"+kind+".json")); err != nil {
		return err
	}
	return pruneCache(c.CacheDir)
}
