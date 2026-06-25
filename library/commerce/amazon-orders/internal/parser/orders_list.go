package parser

import (
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
// .order-card container.
func ParseOrderList(htmlBytes []byte) (*OrderListPage, error) {
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
		s := parseOrderCard(c)
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

func parseOrderCard(card *html.Node) OrderSummary {
	s := OrderSummary{Currency: "USD"}
	cardText := Text(card)

	// Order ID — most reliable signal.
	s.OrderID = ExtractOrderID(cardText)

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

	// Total: first money string after "TOTAL" label, fall back to first currency-marked amount in card.
	foundTotal := false
	if i := strings.Index(strings.ToUpper(cardText), "TOTAL"); i >= 0 {
		window := cardText[i:min(i+60, len(cardText))]
		if total, currency, ok := ExtractMoneyWithCurrency(window); ok {
			s.Total = total
			foundTotal = true
			if currency != "" {
				s.Currency = currency
			}
		}
	}
	if !foundTotal {
		if total, currency, ok := ExtractMoneyWithCurrency(cardText); ok {
			s.Total = total
			if currency != "" {
				s.Currency = currency
			}
		}
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
	s.DetailURL, s.InvoiceURL, s.TrackURL, s.ASINs, s.ItemTitles = extractCardLinks(card)

	return s
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

func extractCardLinks(card *html.Node) (detailURL, invoiceURL, trackURL string, asins, titles []string) {
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
			detailURL = abs(href)
		case strings.Contains(href, "/gp/css/summary/print.html") && invoiceURL == "":
			invoiceURL = abs(href)
		case strings.Contains(href, "ship-track") && trackURL == "":
			trackURL = abs(href)
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

// abs prepends https://www.amazon.com to a relative URL when needed.
func abs(href string) string {
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	if strings.HasPrefix(href, "//") {
		return "https:" + href
	}
	if strings.HasPrefix(href, "/") {
		return "https://www.amazon.com" + href
	}
	return "https://www.amazon.com/" + href
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
