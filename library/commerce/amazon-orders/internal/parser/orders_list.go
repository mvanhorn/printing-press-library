package parser

import (
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// OrderSummary is the per-order data extracted from the order-history listing
// page. Detail-only fields (item.unit_price, payment_method, full ship_to
// address) are populated by ParseOrderDetail, not here.
type OrderSummary struct {
	OrderID     string   `json:"orderId"`
	PlacedDate  string   `json:"placedDate"` // ISO YYYY-MM-DD when parseable; raw string otherwise
	PlacedRaw   string   `json:"placedDateRaw,omitempty"`
	Total       float64  `json:"total"`
	Currency    string   `json:"currency,omitempty"` // "USD" for .com
	ShipTo      string   `json:"shipTo,omitempty"`
	Status      string   `json:"status,omitempty"`      // e.g. "Delivered", "Arriving May 20", "Out for delivery"
	ETADate     string   `json:"etaDate,omitempty"`     // ISO YYYY-MM-DD when status implies a future date
	DeliveredOn string   `json:"deliveredOn,omitempty"` // ISO YYYY-MM-DD
	ItemTitles  []string `json:"itemTitles,omitempty"`
	ASINs       []string `json:"asins,omitempty"`
	DetailURL   string   `json:"detailUrl,omitempty"`
	InvoiceURL  string   `json:"invoiceUrl,omitempty"`
	TrackURL    string   `json:"trackUrl,omitempty"`
}

// OrderListPage is the parsed result of a single /your-orders/orders page.
type OrderListPage struct {
	Orders     []OrderSummary `json:"orders"`
	HasNext    bool           `json:"hasNext"`
	NextStart  int            `json:"nextStartIndex,omitempty"`
	TimeFilter string         `json:"timeFilter,omitempty"`
}

// ParseOrderList walks an order-history HTML page and returns one summary per
// .order-card container. baseURL is the configured marketplace origin (e.g.
// https://www.amazon.ca); relative detail/track/invoice links are resolved
// against it. Pass "" to default to the US storefront.
func ParseOrderList(htmlBytes []byte, baseURL string) (*OrderListPage, error) {
	doc, err := Parse(htmlBytes)
	if err != nil {
		return nil, err
	}
	page := &OrderListPage{}

	cards := FindAll(doc, func(n *html.Node) bool {
		// Match Amazon's order-card containers across A/B variants.
		if n.Type != html.ElementNode || n.Data != "div" {
			return false
		}
		return HasClass(n, "order-card") || HasClass(n, "js-order-card") || HasClassContaining(n, "order-card")
	})

	seen := map[string]bool{}
	for _, c := range cards {
		s := parseOrderCard(c, baseURL)
		if s.OrderID == "" || seen[s.OrderID] {
			continue
		}
		seen[s.OrderID] = true
		page.Orders = append(page.Orders, s)
	}

	// Detect "Next" link presence to set HasNext.
	FindAll(doc, func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return false
		}
		if HasClassContaining(n, "a-last") {
			// .a-disabled means we are at the last page.
			if HasClassContaining(n, "a-disabled") {
				page.HasNext = false
			} else {
				page.HasNext = true
			}
		}
		return true
	})

	return page, nil
}

func parseOrderCard(card *html.Node, baseURL string) OrderSummary {
	s := OrderSummary{Currency: "USD"}
	cardText := Text(card)

	// Order ID — most reliable signal.
	s.OrderID = cardOrderID(card, cardText)

	// Placed date: look for "ORDER PLACED <date>" pair.
	if i := strings.Index(strings.ToUpper(cardText), "ORDER PLACED"); i >= 0 {
		// Take the next 60 chars and find a date-shaped substring.
		window := cardText[i:min(i+80, len(cardText))]
		raw := FirstDateLike(window)
		if raw != "" {
			s.PlacedRaw = raw
			if t := ParseDate(raw); !t.IsZero() {
				s.PlacedDate = t.Format("2006-01-02")
			}
		}
	}

	// Total: first money string after "TOTAL" label, fall back to first $ in card.
	if i := strings.Index(strings.ToUpper(cardText), "TOTAL"); i >= 0 {
		window := cardText[i:min(i+40, len(cardText))]
		s.Total = ExtractMoney(window)
	}
	if s.Total == 0 {
		s.Total = ExtractMoney(cardText)
	}

	// Recipient: SHIP TO label.
	if i := strings.Index(strings.ToUpper(cardText), "SHIP TO"); i >= 0 {
		window := cardText[i:min(i+80, len(cardText))]
		// Skip "SHIP TO" label, take everything until the ORDER # marker.
		window = strings.TrimSpace(strings.TrimPrefix(window, "SHIP TO"))
		window = strings.TrimSpace(strings.TrimPrefix(window, "Ship To"))
		window = strings.TrimSpace(strings.TrimPrefix(window, "Ship to"))
		if j := strings.Index(strings.ToUpper(window), "ORDER #"); j >= 0 {
			window = window[:j]
		}
		s.ShipTo = strings.TrimSpace(window)
	}

	// Status / delivery info: look for status keywords anywhere in the card.
	s.Status, s.ETADate, s.DeliveredOn = extractStatus(cardText)

	// Detail URL, invoice URL, track URL, ASINs/titles.
	s.DetailURL, s.InvoiceURL, s.TrackURL, s.ASINs, s.ItemTitles = extractCardLinks(card, baseURL)

	return s
}

// cardOrderID resolves the order ID for a card. Amazon stopped rendering the
// order ID in the card's visible text — on current markup the card body is
// client-side encrypted and the only stable per-order identifier is the
// card's data-csa-c-slot-id attribute (amzn1.yourorders.order-card.<ID>).
//
// We deliberately do NOT scrape generic <a href> attributes for an order-ID
// shape: the "Buy it again" recommendation links rendered inside every card
// embed the session token (ue_sid), which is itself order-ID-shaped. Scraping
// those would assign every card the same ID and collapse the whole page onto a
// single order. Only unambiguous per-order links (order-details, orderID=) are
// used as a last-resort fallback.
func cardOrderID(card *html.Node, cardText string) string {
	if id := ExtractOrderID(cardText); id != "" {
		return id
	}
	if slot := Attr(card, "data-csa-c-slot-id"); strings.Contains(slot, "order-card.") {
		if id := ExtractOrderID(slot); id != "" {
			return id
		}
	}
	var found string
	FindAll(card, func(n *html.Node) bool {
		if found != "" {
			return false
		}
		if n.Type != html.ElementNode || n.Data != "a" {
			return true
		}
		href := Attr(n, "href")
		if strings.Contains(href, "order-details") || strings.Contains(href, "orderID=") {
			if id := ExtractOrderID(href); id != "" {
				found = id
				return false
			}
		}
		return true
	})
	return found
}

// extractStatus searches a card's text for status keywords and resolves a
// best-effort ETA or delivery date.
func extractStatus(cardText string) (status, eta, delivered string) {
	lower := strings.ToLower(cardText)

	// Hierarchy: Cancelled > Delivered > Out for delivery > Arriving > Shipped > Preparing
	switch {
	case strings.Contains(lower, "cancelled") || strings.Contains(lower, "canceled"):
		return "Cancelled", "", ""
	case strings.Contains(lower, "out for delivery"):
		status = "Out for delivery"
	case strings.Contains(lower, "delivered"):
		status = "Delivered"
		if i := strings.Index(lower, "delivered"); i >= 0 {
			window := cardText[i:min(i+80, len(cardText))]
			if d := FirstDateLike(window); d != "" {
				if t := ParseDate(d); !t.IsZero() {
					delivered = t.Format("2006-01-02")
				}
			}
		}
		return
	case strings.Contains(lower, "arriving"):
		status = "Arriving"
		if i := strings.Index(lower, "arriving"); i >= 0 {
			window := cardText[i:min(i+80, len(cardText))]
			if d := FirstDateLike(window); d != "" {
				if t := ParseDate(d); !t.IsZero() {
					eta = t.Format("2006-01-02")
				}
			}
		}
		return
	case strings.Contains(lower, "shipped"):
		status = "Shipped"
	case strings.Contains(lower, "preparing for shipment") || strings.Contains(lower, "not yet shipped"):
		status = "Preparing"
	case strings.Contains(lower, "returned"):
		status = "Returned"
	case strings.Contains(lower, "refunded"):
		status = "Refunded"
	}
	return
}

func extractCardLinks(card *html.Node, baseURL string) (detailURL, invoiceURL, trackURL string, asins, titles []string) {
	asinSeen := map[string]bool{}
	titleSeen := map[string]bool{}

	FindAll(card, func(n *html.Node) bool {
		if n.Type != html.ElementNode || n.Data != "a" {
			return true
		}
		href := Attr(n, "href")
		if href == "" {
			return true
		}
		switch {
		case strings.Contains(href, "order-details") && detailURL == "":
			detailURL = abs(baseURL, href)
		case strings.Contains(href, "/gp/css/summary/print.html") && invoiceURL == "":
			invoiceURL = abs(baseURL, href)
		case strings.Contains(href, "ship-track") && trackURL == "":
			trackURL = abs(baseURL, href)
		case strings.Contains(href, "/dp/") || strings.Contains(href, "/gp/product/"):
			a := ExtractASIN(href)
			if a != "" && !asinSeen[a] {
				asinSeen[a] = true
				asins = append(asins, a)
			}
			t := strings.TrimSpace(Text(n))
			if t != "" && !titleSeen[t] {
				titleSeen[t] = true
				titles = append(titles, t)
			}
		}
		return true
	})
	return
}

// abs resolves a relative Amazon URL against the configured marketplace origin
// derived from baseURL (e.g. https://www.amazon.ca). Absolute and
// protocol-relative URLs are returned unchanged.
func abs(baseURL, href string) string {
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	if strings.HasPrefix(href, "//") {
		return "https:" + href
	}
	origin := originFromBaseURL(baseURL)
	if strings.HasPrefix(href, "/") {
		return origin + href
	}
	return origin + "/" + href
}

// originFromBaseURL returns scheme://host for the configured base URL, defaulting
// to https://www.amazon.com when baseURL is empty or unparseable. This keeps
// relative order/track/invoice links on the marketplace the CLI is configured
// for instead of hardcoding the US storefront.
func originFromBaseURL(baseURL string) string {
	const fallback = "https://www.amazon.com"
	if baseURL == "" {
		return fallback
	}
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" {
		return fallback
	}
	scheme := u.Scheme
	if scheme == "" {
		scheme = "https"
	}
	return scheme + "://" + u.Host
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
