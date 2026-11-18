package edeka

import (
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// Product is one article tile of a search, category or offers grid.
type Product struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	PriceCents    int    `json:"price_cents"`
	Price         string `json:"price"`
	UnitPrice     string `json:"unit_price,omitempty"`
	OnOffer       bool   `json:"on_offer"`
	WeightArticle bool   `json:"weight_article"`
	InBasket      int    `json:"in_basket,omitempty"`
	URL           string `json:"url,omitempty"`
	Image         string `json:"image,omitempty"`
}

// Grid is one page of product tiles plus paging hints.
type Grid struct {
	Products []Product `json:"products"`
	Page     int       `json:"page"`
	HasNext  bool      `json:"has_next"`
}

// ParseProductGrid reads the product tiles of a search, category or offers page.
func ParseProductGrid(page string, pageNr int) (Grid, error) {
	doc, err := parseHTML(page)
	if err != nil {
		return Grid{}, err
	}
	g := Grid{Products: []Product{}, Page: pageNr}
	seen := map[string]bool{}
	for _, box := range findAll(doc, byClass("product-box"), true) {
		p := Product{}
		if in := findFirst(box, byTagAttr("input", "name", "aid")); in != nil {
			p.ID = strings.TrimSpace(attr(in, "value"))
		}
		if a := findFirst(box, func(n *html.Node) bool {
			return n.Data == "a" && n.Parent != nil && hasClass(n.Parent, "title")
		}); a != nil {
			p.Title = text(a)
			p.URL = cleanURL(attr(a, "href"))
		}
		if ap := findFirst(box, byClass("article-price")); ap != nil {
			p.Price = text(ap)
			p.PriceCents, _ = ParseCents(p.Price)
			if p.PriceCents > 0 {
				p.Price = FormatCents(p.PriceCents)
			}
		}
		if pu := findFirst(box, byClass("pricePerUnit")); pu != nil {
			p.UnitPrice = strings.ReplaceAll(text(pu), " / ", "/")
			p.UnitPrice = strings.ReplaceAll(p.UnitPrice, "€/ ", "€/")
		}
		if pic := findFirst(box, byClass("picture")); pic != nil {
			if img := findFirst(pic, func(n *html.Node) bool { return n.Data == "img" }); img != nil {
				p.Image = attr(img, "src")
			}
		}
		p.OnOffer = findFirst(box, func(n *html.Node) bool {
			return hasClass(n, "badge-text") && strings.EqualFold(attr(n, "aria-label"), "Angebot")
		}) != nil
		if strings.Contains(p.URL, "/Angebote/") {
			p.OnOffer = true
		}
		p.WeightArticle = findFirst(box, byClass("mx-weightarticle-badge")) != nil
		if am := findFirst(box, func(n *html.Node) bool {
			return n.Data == "input" && strings.HasPrefix(attr(n, "id"), "amountToBasketList-")
		}); am != nil {
			p.InBasket, _ = strconv.Atoi(strings.TrimSpace(attr(am, "value")))
		}
		if p.ID == "" || p.Title == "" || seen[p.ID] {
			continue
		}
		seen[p.ID] = true
		g.Products = append(g.Products, p)
	}
	next := "pgNr=" + strconv.Itoa(pageNr+1)
	g.HasNext = strings.Contains(page, next+"\"") || strings.Contains(page, next+"&") || strings.Contains(page, next+"'")
	return g, nil
}

// BasketItem is one basket line.
type BasketItem struct {
	ID             string `json:"id"`
	BasketItemID   string `json:"basket_item_id,omitempty"`
	Title          string `json:"title"`
	Quantity       int    `json:"quantity"`
	UnitPriceCents int    `json:"unit_price_cents"`
	TotalCents     int    `json:"total_cents"`
	PricePerUnit   string `json:"price_per_unit,omitempty"`
	URL            string `json:"url,omitempty"`
}

// Basket is the basket page (or the order review's basket section).
type Basket struct {
	Items         []BasketItem `json:"items"`
	ItemsCents    int          `json:"items_cents"`
	DeliveryCents int          `json:"delivery_fee_cents"`
	TotalCents    int          `json:"total_cents"`
	Total         string       `json:"total"`
	Count         int          `json:"count"`
}

var aproductsRe = regexp.MustCompile(`^aproducts\[([^\]]+)\]\[(aid|basketitemid|am)\]$`)
var mengeRe = regexp.MustCompile(`Menge\s+(?:Menge\s+)?(\d+)`)

// ParseBasket reads the basket lines and totals from the basket page or the
// order review page.
func ParseBasket(page string) (*Basket, error) {
	doc, err := parseHTML(page)
	if err != nil {
		return nil, err
	}
	b := &Basket{Items: []BasketItem{}}
	for _, li := range findAll(doc, func(n *html.Node) bool { return n.Data == "li" && hasClass(n, "basket-item") }, true) {
		it := BasketItem{}
		for _, in := range findAll(li, func(n *html.Node) bool { return n.Data == "input" }, false) {
			m := aproductsRe.FindStringSubmatch(attr(in, "name"))
			if m == nil {
				continue
			}
			switch m[2] {
			case "aid":
				it.ID = attr(in, "value")
			case "basketitemid":
				it.BasketItemID = attr(in, "value")
			case "am":
				it.Quantity, _ = strconv.Atoi(strings.TrimSpace(attr(in, "value")))
			}
		}
		if a := findFirst(li, byClass("basket-item-title")); a != nil {
			it.Title = text(a)
			it.URL = cleanURL(attr(a, "href"))
			if it.ID == "" {
				it.ID = strings.TrimPrefix(attr(a, "id"), "basket-item-title-")
			}
		}
		if it.ID == "" {
			it.ID = strings.TrimPrefix(attr(li, "aria-labelledby"), "basket-item-title-")
		}
		if u := findFirst(li, func(n *html.Node) bool { return strings.HasPrefix(attr(n, "id"), "unitPriceValue_") }); u != nil {
			it.UnitPriceCents, _ = ParseCents(text(u))
		}
		if t := findFirst(li, func(n *html.Node) bool { return strings.HasPrefix(attr(n, "id"), "totalPriceValue_") }); t != nil {
			it.TotalCents, _ = ParseCents(text(t))
		}
		if pu := findFirst(li, byID("productPriceUnit")); pu != nil {
			it.PricePerUnit = strings.ReplaceAll(text(pu), "€/ ", "€/")
		}
		if it.Quantity == 0 {
			if m := mengeRe.FindStringSubmatch(text(li)); m != nil {
				it.Quantity, _ = strconv.Atoi(m[1])
			}
		}
		if it.ID == "" && it.Title == "" {
			continue
		}
		b.Items = append(b.Items, it)
		b.Count += it.Quantity
	}
	if n := findFirst(doc, byID("basketTotalProductsGross")); n != nil {
		b.ItemsCents, _ = ParseCents(text(n))
	}
	if n := findFirst(doc, byID("basketDeliveryGross")); n != nil {
		b.DeliveryCents, _ = ParseCents(text(n))
	}
	if n := findFirst(doc, byID("basketGrandTotal")); n != nil {
		b.TotalCents, _ = ParseCents(text(n))
	}
	if b.TotalCents == 0 && len(b.Items) > 0 {
		for _, it := range b.Items {
			b.TotalCents += it.TotalCents
		}
	}
	b.Total = FormatCents(b.TotalCents)
	return b, nil
}

// Slot is one bookable time window.
type Slot struct {
	Date      string `json:"date"`
	From      string `json:"from"`
	Until     string `json:"until"`
	Type      string `json:"type"`
	Timestamp int64  `json:"timestamp"`
	Available bool   `json:"available"`
	FeeCents  int    `json:"fee_cents"`
	Premium   bool   `json:"premium,omitempty"`
}

// Calendar holds the slots of the market's calendar widget.
type Calendar struct {
	Pickup         []Slot   `json:"pickup"`
	Delivery       []Slot   `json:"delivery,omitempty"`
	PickupAddress  string   `json:"pickup_address,omitempty"`
	DeliveryAreas  []Area   `json:"delivery_areas,omitempty"`
	DeliveryNotice string   `json:"delivery_notice,omitempty"`
	Types          []string `json:"types"`
}

// Area is a delivery area (postal code and town) of the market.
type Area struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type rawSlot struct {
	Available bool   `json:"available"`
	Time      string `json:"time"`
	Timestamp int64  `json:"timestamp"`
	Timespan  struct {
		From  string `json:"from"`
		Until string `json:"until"`
	} `json:"timespan"`
	SlotPricelist json.RawMessage `json:"slotPricelist"`
}

var calendarDataRe = regexp.MustCompile(`window\['(pickup|delivery|express|station)Data'\]\['(all|premium)'\]\s*=\s*(\{.*?\}|\[\]|null);\s*\n`)

// ParseCalendar reads the slot JSON the calendar widget embeds
// (window['pickupData']['all'] = {...}) and the delivery areas.
func ParseCalendar(page string) (*Calendar, error) {
	c := &Calendar{Pickup: []Slot{}, Types: []string{}}
	found := false
	for _, m := range calendarDataRe.FindAllStringSubmatch(page, -1) {
		typ, group, body := m[1], m[2], m[3]
		if body == "[]" || body == "null" {
			continue
		}
		var days map[string][]rawSlot
		if err := json.Unmarshal([]byte(body), &days); err != nil {
			continue
		}
		found = true
		slots := windows(days, typ, group == "premium")
		switch typ {
		case "pickup":
			c.Pickup = append(c.Pickup, slots...)
		case "delivery", "express":
			c.Delivery = append(c.Delivery, slots...)
		}
	}
	if doc, err := parseHTML(page); err == nil {
		if a := findFirst(doc, byClass("pickup__address")); a != nil {
			if ad := findFirst(a, func(n *html.Node) bool { return n.Data == "address" }); ad != nil {
				c.PickupAddress = text(ad)
			}
		}
		for _, l := range findAll(doc, byClass("delivery-area-link"), true) {
			c.DeliveryAreas = append(c.DeliveryAreas, Area{ID: attr(l, "data-id"), Name: text(l)})
		}
		if findFirst(doc, byID("label-pickup")) != nil {
			c.Types = append(c.Types, "pickup")
		}
		if findFirst(doc, byID("label-delivery")) != nil {
			c.Types = append(c.Types, "delivery")
		}
	}
	if !found && len(c.DeliveryAreas) == 0 {
		return nil, errors.New("the calendar widget carried no slot data")
	}
	sortSlots(c.Pickup)
	sortSlots(c.Delivery)
	return c, nil
}

// windows merges the per-time rows of one day into the shop's time windows
// (a window is available when any of its rows is).
func windows(days map[string][]rawSlot, typ string, premium bool) []Slot {
	var out []Slot
	for date, rows := range days {
		idx := map[string]int{}
		for _, r := range rows {
			from, until := r.Timespan.From, r.Timespan.Until
			if from == "" {
				from = r.Time
			}
			key := from + "-" + until
			i, ok := idx[key]
			if !ok {
				out = append(out, Slot{Date: date, From: from, Until: until, Type: typ, Premium: premium, FeeCents: slotFee(r.SlotPricelist)})
				i = len(out) - 1
				idx[key] = i
			}
			if r.Available && (!out[i].Available || r.Timestamp < out[i].Timestamp) {
				out[i].Available = true
				out[i].Timestamp = r.Timestamp
			}
			if out[i].Timestamp == 0 {
				out[i].Timestamp = r.Timestamp
			}
		}
	}
	return out
}

func slotFee(raw json.RawMessage) int {
	var list []map[string]any
	if json.Unmarshal(raw, &list) != nil {
		return 0
	}
	for _, e := range list {
		for _, k := range []string{"price", "mxprice", "brutto"} {
			switch v := e[k].(type) {
			case float64:
				return int(v*100 + 0.5)
			case string:
				if c, ok := ParseCents(strings.ReplaceAll(v, ".", ",")); ok {
					return c
				}
			}
		}
	}
	return 0
}

func sortSlots(s []Slot) {
	sort.Slice(s, func(i, j int) bool {
		if s[i].Date != s[j].Date {
			return s[i].Date < s[j].Date
		}
		return s[i].From < s[j].From
	})
}

// Category is one entry of the shop's category tree.
type Category struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Level int    `json:"level"`
	URL   string `json:"url"`
}

// ParseCategories reads the main navigation tree (levels 1-3).
func ParseCategories(page, host string) []Category {
	doc, err := parseHTML(page)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []Category
	for _, a := range findAll(doc, func(n *html.Node) bool { return n.Data == "a" && hasClass(n, "nav-link") }, true) {
		level := 0
		for l := 1; l <= 4; l++ {
			if hasClass(a, "is-level-"+strconv.Itoa(l)) {
				level = l
			}
		}
		href := cleanURL(attr(a, "href"))
		if level == 0 || href == "" || seen[href] {
			continue
		}
		path := href
		if i := strings.Index(path, host); i >= 0 {
			path = path[i+len(host):]
		}
		if !strings.HasPrefix(path, "/") || path == "/" {
			continue
		}
		name := attr(a, "data-title")
		if name == "" {
			name = text(a)
		}
		seen[href] = true
		out = append(out, Category{Name: name, Path: path, Level: level, URL: href})
	}
	return out
}

// ProductDetail is the product page's structured data.
type ProductDetail struct {
	ID           string `json:"id,omitempty"`
	SKU          string `json:"sku"`
	Title        string `json:"title"`
	PriceCents   int    `json:"price_cents"`
	Price        string `json:"price"`
	UnitPrice    string `json:"unit_price,omitempty"`
	Currency     string `json:"currency"`
	Availability string `json:"availability"`
	Description  string `json:"description,omitempty"`
	Image        string `json:"image,omitempty"`
	URL          string `json:"url"`
}

// ParseProductDetail reads schema.org microdata from a product page.
func ParseProductDetail(page string) (*ProductDetail, error) {
	doc, err := parseHTML(page)
	if err != nil {
		return nil, err
	}
	d := &ProductDetail{}
	if n := findFirst(doc, byID("productTitle")); n != nil {
		d.Title = text(n)
	}
	if n := findFirst(doc, func(n *html.Node) bool { return attr(n, "itemprop") == "sku" }); n != nil {
		d.SKU = attr(n, "content")
	}
	if n := findFirst(doc, func(n *html.Node) bool { return attr(n, "itemprop") == "price" }); n != nil {
		v := attr(n, "content")
		if v == "" {
			v = text(n)
		}
		if f, err := strconv.ParseFloat(strings.ReplaceAll(v, ",", "."), 64); err == nil {
			d.PriceCents = int(f*100 + 0.5)
			d.Price = FormatCents(d.PriceCents)
		}
	}
	if n := findFirst(doc, func(n *html.Node) bool { return attr(n, "itemprop") == "priceCurrency" }); n != nil {
		d.Currency = text(n)
	}
	if n := findFirst(doc, func(n *html.Node) bool { return attr(n, "itemprop") == "availability" }); n != nil {
		av := attr(n, "href")
		if av == "" {
			av = attr(n, "content")
		}
		d.Availability = av[strings.LastIndex(av, "/")+1:]
	}
	if n := findFirst(doc, func(n *html.Node) bool { return attr(n, "itemprop") == "description" }); n != nil {
		d.Description = text(n)
		if len(d.Description) > 2000 {
			d.Description = d.Description[:2000]
		}
	}
	if n := findFirst(doc, func(n *html.Node) bool { return n.Data == "img" && attr(n, "itemprop") == "image" }); n != nil {
		d.Image = attr(n, "src")
	}
	if n := findFirst(doc, func(n *html.Node) bool { return attr(n, "itemprop") == "url" }); n != nil {
		d.URL = cleanURL(attr(n, "content") + attr(n, "href"))
	}
	if n := findFirst(doc, byTagAttr("input", "name", "aid")); n != nil {
		d.ID = attr(n, "value")
	}
	if n := findFirst(doc, byID("productPriceUnit")); n != nil {
		d.UnitPrice = strings.ReplaceAll(text(n), " / ", "/")
	}
	if d.Title == "" && d.SKU == "" {
		return nil, errors.New("the page is not a product page")
	}
	return d, nil
}

// PaymentOption is one payment method of the checkout.
type PaymentOption struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Selected bool   `json:"selected"`
	AtPickup bool   `json:"pay_at_handover"`
}

// ParsePaymentOptions reads the payment radio buttons of the payment step.
func ParsePaymentOptions(page string) []PaymentOption {
	doc, err := parseHTML(page)
	if err != nil {
		return nil
	}
	var out []PaymentOption
	seen := map[string]bool{}
	for _, in := range findAll(doc, func(n *html.Node) bool { return n.Data == "input" && attr(n, "name") == "paymentid" }, true) {
		id := attr(in, "value")
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		name := attr(in, "title")
		if name == "" {
			if l := findFirst(doc, byTagAttr("label", "for", attr(in, "id"))); l != nil {
				name = text(l)
			}
		}
		_, checked := attrOK(in, "checked")
		out = append(out, PaymentOption{ID: id, Name: name, Selected: checked, AtPickup: id == "mxcash" || id == "mxlocalcashcard"})
	}
	return out
}

func attrOK(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

// FormFields returns the name/value pairs of the first form that matches,
// the way a browser would submit it (hidden, text and checked inputs).
func FormFields(page string, match func(id, name string) bool) (map[string][]string, bool) {
	doc, err := parseHTML(page)
	if err != nil {
		return nil, false
	}
	form := findFirst(doc, func(n *html.Node) bool { return n.Data == "form" && match(attr(n, "id"), attr(n, "name")) })
	if form == nil {
		return nil, false
	}
	out := map[string][]string{}
	for _, in := range findAll(form, func(n *html.Node) bool { return n.Data == "input" || n.Data == "select" || n.Data == "textarea" }, true) {
		name := attr(in, "name")
		if name == "" {
			continue
		}
		typ := strings.ToLower(attr(in, "type"))
		switch in.Data {
		case "select":
			if opt := findFirst(in, func(n *html.Node) bool { _, ok := attrOK(n, "selected"); return n.Data == "option" && ok }); opt != nil {
				out[name] = append(out[name], attr(opt, "value"))
			} else if opt := findFirst(in, func(n *html.Node) bool { return n.Data == "option" }); opt != nil {
				out[name] = append(out[name], attr(opt, "value"))
			}
			continue
		case "textarea":
			out[name] = append(out[name], text(in))
			continue
		}
		if typ == "submit" || typ == "button" || typ == "image" || typ == "file" {
			continue
		}
		if typ == "checkbox" || typ == "radio" {
			if _, ok := attrOK(in, "checked"); !ok {
				continue
			}
		}
		out[name] = append(out[name], attr(in, "value"))
	}
	return out, true
}

// OrderReview is what the shop shows on the last checkout step.
type OrderReview struct {
	Slot           string       `json:"slot"`
	ShippingMethod string       `json:"shipping_method,omitempty"`
	Payment        string       `json:"payment"`
	Items          []BasketItem `json:"items"`
	ItemsCents     int          `json:"items_cents"`
	FeeCents       int          `json:"fee_cents"`
	TotalCents     int          `json:"total_cents"`
	Total          string       `json:"total"`
	Ready          bool         `json:"ready_to_order"`
}

// ParseOrderReview reads the order review page (cl=order).
func ParseOrderReview(page string) (*OrderReview, error) {
	doc, err := parseHTML(page)
	if err != nil {
		return nil, err
	}
	r := &OrderReview{Items: []BasketItem{}}
	if lab := findFirst(doc, func(n *html.Node) bool {
		return n.Data == "div" && strings.TrimSpace(text(n)) == "Ausgewählter Termin"
	}); lab != nil {
		for s := lab.NextSibling; s != nil; s = s.NextSibling {
			if s.Type == html.ElementNode {
				r.Slot = text(s)
				break
			}
		}
	}
	if img := findFirst(doc, byClass("order-payment__icon")); img != nil {
		r.Payment = attr(img, "title")
	}
	t := text(doc)
	for _, m := range []string{"Selbstabholung", "Lieferservice", "Express"} {
		if strings.Contains(strings.ToLower(t), strings.ToLower("Ihre Auswahl "+m)) {
			r.ShippingMethod = m
		}
	}
	b, err := ParseBasket(page)
	if err == nil {
		r.Items = b.Items
		r.ItemsCents = b.ItemsCents
		r.FeeCents = b.DeliveryCents
		r.TotalCents = b.TotalCents
	}
	r.Total = FormatCents(r.TotalCents)
	_, r.Ready = FormFields(page, func(id, _ string) bool { return id == "orderConfirmAgbBottom" || id == "orderConfirmAgbTop" })
	if !r.Ready && r.Slot == "" {
		return nil, errors.New("the page is not the order review")
	}
	return r, nil
}

// OrderSummary is one row of the order history.
type OrderSummary struct {
	Number string `json:"number"`
	Date   string `json:"date,omitempty"`
	Status string `json:"status,omitempty"`
	Total  string `json:"total,omitempty"`
	Text   string `json:"text"`
}

var orderNrRe = regexp.MustCompile(`(?i)(?:Bestellnummer|Bestell-Nr\.?|Auftragsnummer)(?:\s+lautet)?\s*:?\s*([A-Za-z-]*[0-9][0-9A-Za-z-]*)`)
var dateRe = regexp.MustCompile(`\b\d{2}\.\d{2}\.\d{4}\b`)

// ParseOrderHistory reads the order history page. The shop renders one card
// per order; each card that names an order number becomes a row.
func ParseOrderHistory(page string) ([]OrderSummary, int, error) {
	doc, err := parseHTML(page)
	if err != nil {
		return nil, 0, err
	}
	count := -1
	if h := findFirst(doc, byClass("order__title")); h != nil {
		if m := regexp.MustCompile(`(\d+)\s+Bestellung`).FindStringSubmatch(text(h)); m != nil {
			count, _ = strconv.Atoi(m[1])
		}
	}
	var out []OrderSummary
	seen := map[string]bool{}
	cands := findAll(doc, func(n *html.Node) bool {
		return (n.Data == "li" || n.Data == "div" || n.Data == "tr" || n.Data == "section") && orderNrRe.MatchString(text(n)) && len(text(n)) < 1500
	}, false)
	// keep only the innermost cards: drop a candidate that contains another one
	isAncestor := func(a, b *html.Node) bool {
		for p := b.Parent; p != nil; p = p.Parent {
			if p == a {
				return true
			}
		}
		return false
	}
	var inner []*html.Node
	for _, c := range cands {
		hasChild := false
		for _, d := range cands {
			if d != c && isAncestor(c, d) {
				hasChild = true
				break
			}
		}
		if !hasChild {
			inner = append(inner, c)
		}
	}
	for _, n := range inner {
		t := text(n)
		m := orderNrRe.FindStringSubmatch(t)
		if m == nil || seen[m[1]] {
			continue
		}
		// keep the innermost card that names exactly one order
		if len(orderNrRe.FindAllString(t, -1)) != 1 {
			continue
		}
		seen[m[1]] = true
		o := OrderSummary{Number: m[1], Text: t}
		if d := dateRe.FindString(t); d != "" {
			o.Date = d
		}
		if c, ok := ParseCents(t[strings.LastIndex(t, m[1])+len(m[1]):]); ok {
			o.Total = FormatCents(c)
		}
		out = append(out, o)
	}
	if count < 0 {
		count = len(out)
	}
	return out, count, nil
}
