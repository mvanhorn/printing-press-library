package parser

import (
	"strings"

	"golang.org/x/net/html"
)

// OrderItem is one line item on an order-detail page.
type OrderItem struct {
	ASIN       string  `json:"asin,omitempty"`
	Title      string  `json:"title"`
	Quantity   int     `json:"quantity,omitempty"`
	UnitPrice  float64 `json:"unitPrice,omitempty"`
	Seller     string  `json:"seller,omitempty"`
	Condition  string  `json:"condition,omitempty"`
	ProductURL string  `json:"productUrl,omitempty"`
}

// ShipmentSummary is one box / parcel within an order.
type ShipmentSummary struct {
	Status      string   `json:"status"`            // "Delivered", "Arriving May 20", "Out for delivery"
	ETADate     string   `json:"etaDate,omitempty"` // ISO YYYY-MM-DD
	DeliveredOn string   `json:"deliveredOn,omitempty"`
	TrackURL    string   `json:"trackUrl,omitempty"`
	ItemTitles  []string `json:"itemTitles,omitempty"`
}

// OrderDetail is the parsed order-details page.
type OrderDetail struct {
	OrderID       string            `json:"orderId"`
	PlacedDate    string            `json:"placedDate,omitempty"`
	PlacedRaw     string            `json:"placedDateRaw,omitempty"`
	ShipTo        string            `json:"shipTo,omitempty"`
	PaymentMethod string            `json:"paymentMethod,omitempty"`
	PaymentLast4  string            `json:"paymentLast4,omitempty"`
	Items         []OrderItem       `json:"items,omitempty"`
	Shipments     []ShipmentSummary `json:"shipments,omitempty"`
	ItemSubtotal  float64           `json:"itemSubtotal,omitempty"`
	Shipping      float64           `json:"shipping,omitempty"`
	TaxAmount     float64           `json:"tax,omitempty"`
	GrandTotal    float64           `json:"grandTotal,omitempty"`
	Currency      string            `json:"currency,omitempty"`
}

// ParseOrderDetail walks an order-details HTML page and returns a structured
// OrderDetail. Fields not present in the input are left zero / empty.
func ParseOrderDetail(htmlBytes []byte) (*OrderDetail, error) {
	doc, err := Parse(htmlBytes)
	if err != nil {
		return nil, err
	}
	od := &OrderDetail{Currency: "USD"}
	docText := Text(doc)

	od.OrderID = ExtractOrderID(docText)
	if i := strings.Index(strings.ToUpper(docText), "ORDER PLACED"); i >= 0 {
		window := docText[i:min(i+60, len(docText))]
		raw := FirstDateLike(window)
		if raw != "" {
			od.PlacedRaw = raw
			if t := ParseDate(raw); !t.IsZero() {
				od.PlacedDate = t.Format("2006-01-02")
			}
		}
	}

	// Ship-to: text after "Ship to" up to the next "Payment method" or "Order"
	if i := strings.Index(docText, "Ship to"); i >= 0 {
		window := docText[i+len("Ship to"):]
		end := len(window)
		for _, marker := range []string{"Payment method", "Change shipping address", "Order #"} {
			if j := strings.Index(window, marker); j > 0 && j < end {
				end = j
			}
		}
		od.ShipTo = strings.TrimSpace(window[:end])
	}

	// Payment method + last 4.
	if i := strings.Index(docText, "Payment method"); i >= 0 {
		window := docText[i+len("Payment method") : min(i+200, len(docText))]
		// First fragment until next section keyword.
		end := len(window)
		for _, marker := range []string{"Earns", "Order Summary", "Item(s) Subtotal", "Shipping & Handling"} {
			if j := strings.Index(window, marker); j > 0 && j < end {
				end = j
			}
		}
		pm := strings.TrimSpace(window[:end])
		// "Prime Visaending in 1234" -> "Prime Visa" + "1234"
		if last4 := ExtractLast4(pm); last4 != "" {
			od.PaymentLast4 = last4
			pm = strings.TrimSpace(strings.SplitN(pm, "ending in", 2)[0])
			pm = strings.TrimSpace(strings.SplitN(pm, "ending", 2)[0])
		}
		od.PaymentMethod = pm
	}

	// Money totals: scan text for known labels and the next $ amount.
	od.ItemSubtotal = moneyAfter(docText, "Item(s) Subtotal")
	od.Shipping = moneyAfter(docText, "Shipping & Handling")
	if t := moneyAfter(docText, "Estimated tax to be collected"); t != 0 {
		od.TaxAmount = t
	} else {
		od.TaxAmount = moneyAfter(docText, "Sales Tax")
	}
	od.GrandTotal = moneyAfter(docText, "Grand Total")
	if od.GrandTotal == 0 {
		od.GrandTotal = moneyAfter(docText, "Order Total")
	}

	// Items: every distinct ASIN-bearing /dp/ link within the items section.
	itemSeen := map[string]bool{}
	FindAll(doc, func(n *html.Node) bool {
		if n.Type != html.ElementNode || n.Data != "a" {
			return true
		}
		href := Attr(n, "href")
		if !strings.Contains(href, "/dp/") {
			return true
		}
		asin := ExtractASIN(href)
		if asin == "" || itemSeen[asin] {
			return true
		}
		itemSeen[asin] = true
		title := strings.TrimSpace(Text(n))
		if title == "" {
			return true
		}
		od.Items = append(od.Items, OrderItem{
			ASIN:       asin,
			Title:      title,
			ProductURL: abs(href),
		})
		return true
	})

	// Shipments: each "Arriving …" / "Delivered …" header is one shipment.
	shipmentBlocks := FindAll(doc, func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return false
		}
		// Heuristic: divs whose text starts with "Arriving" or "Delivered".
		if HasClassContaining(n, "shipment") || HasClassContaining(n, "shipment-tracking") {
			return true
		}
		return false
	})
	for _, sb := range shipmentBlocks {
		t := Text(sb)
		if t == "" {
			continue
		}
		st, eta, del := extractStatus(t)
		if st == "" {
			continue
		}
		// Skip duplicates by exact text.
		dup := false
		for _, existing := range od.Shipments {
			if existing.Status == st && existing.ETADate == eta && existing.DeliveredOn == del {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		ship := ShipmentSummary{Status: st, ETADate: eta, DeliveredOn: del}
		// Track URL on this block, if any.
		FindAll(sb, func(n *html.Node) bool {
			if ship.TrackURL != "" {
				return false
			}
			if n.Type == html.ElementNode && n.Data == "a" {
				href := Attr(n, "href")
				if strings.Contains(href, "ship-track") {
					ship.TrackURL = abs(href)
				}
			}
			return true
		})
		od.Shipments = append(od.Shipments, ship)
	}

	return od, nil
}

// moneyAfter returns the first money value appearing after label in text.
func moneyAfter(text, label string) float64 {
	i := strings.Index(text, label)
	if i < 0 {
		return 0
	}
	window := text[i:min(i+len(label)+60, len(text))]
	return ExtractMoney(window)
}
