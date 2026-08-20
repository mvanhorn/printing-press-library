package parser

import (
	"strings"

	"golang.org/x/net/html"
)

// Transaction is one charge / refund on the transactions page.
type Transaction struct {
	Date          string  `json:"date,omitempty"` // ISO YYYY-MM-DD
	DateRaw       string  `json:"dateRaw,omitempty"`
	PaymentMethod string  `json:"paymentMethod,omitempty"`
	PaymentLast4  string  `json:"paymentLast4,omitempty"`
	Amount        float64 `json:"amount"` // signed: negative for charges, positive for refunds
	OrderID       string  `json:"orderId,omitempty"`
	Description   string  `json:"description,omitempty"`
}

// TransactionsPage is the parsed /cpe/yourpayments/transactions page.
type TransactionsPage struct {
	Transactions []Transaction `json:"transactions"`
}

// ParseTransactions parses the transactions list. The Amazon transactions
// page is grouped by date with each row containing payment method + last4 +
// amount + (optional) Order # + description.
func ParseTransactions(htmlBytes []byte) (*TransactionsPage, error) {
	doc, err := Parse(htmlBytes)
	if err != nil {
		return nil, err
	}
	page := &TransactionsPage{}

	// Walk transaction-line item containers.
	rows := FindAll(doc, func(n *html.Node) bool {
		if n.Type != html.ElementNode || n.Data != "div" {
			return false
		}
		return HasClassContaining(n, "apx-transactions-line-item-component-container")
	})

	// Find date-header siblings to attribute each row.
	// Transactions pages render: [date-header] [row] [row] ... [date-header] [row] ...
	// Walk doc, accumulate rows under the most recent date.
	var current string
	var currentRaw string
	FindAll(doc, func(n *html.Node) bool {
		if n.Type != html.ElementNode || n.Data != "div" {
			return false
		}
		if HasClassContaining(n, "apx-transaction-date-container") {
			t := strings.TrimSpace(Text(n))
			currentRaw = t
			if pt := ParseDate(t); !pt.IsZero() {
				current = pt.Format("2006-01-02")
			} else {
				current = ""
			}
			return false
		}
		return false
	})
	_ = current
	_ = currentRaw
	// Simpler approach: re-iterate in document order, collecting date-headers and row blocks.
	dateForRow := map[*html.Node]string{}
	dateRawForRow := map[*html.Node]string{}
	var pendingDate, pendingDateRaw string
	Walk(doc, func(n *html.Node) bool {
		if n.Type != html.ElementNode || n.Data != "div" {
			return true
		}
		if HasClassContaining(n, "apx-transaction-date-container") {
			t := strings.TrimSpace(Text(n))
			pendingDateRaw = t
			if pt := ParseDate(t); !pt.IsZero() {
				pendingDate = pt.Format("2006-01-02")
			} else {
				pendingDate = ""
			}
			return false
		}
		if HasClassContaining(n, "apx-transactions-line-item-component-container") {
			dateForRow[n] = pendingDate
			dateRawForRow[n] = pendingDateRaw
			return false
		}
		return true
	})

	for _, r := range rows {
		text := Text(r)
		if text == "" {
			continue
		}
		// Skip the gift-card disclaimer lead-in if it appears in a row.
		if strings.Contains(strings.ToLower(text), "gift card transactions") && len(text) < 400 {
			// often the leading note blob reuses the container; only skip if it's mostly text.
		}
		t := Transaction{
			Date:    dateForRow[r],
			DateRaw: dateRawForRow[r],
		}
		// Payment method + last4: scan the line until we hit the amount.
		t.PaymentLast4 = ExtractLast4(text)
		// Pull the payment method as the substring before "****" or "ending in"
		pm := text
		if idx := strings.Index(pm, "****"); idx >= 0 {
			pm = pm[:idx]
		} else if idx := strings.Index(pm, " ending in "); idx >= 0 {
			pm = pm[:idx]
		}
		// Trim trailing text like "Completed " prefix
		pm = strings.TrimSpace(pm)
		// Take only the last short phrase that looks like a card name.
		pm = lastSegmentLike(pm)
		t.PaymentMethod = pm

		// Amount.
		t.Amount = ExtractMoney(text)

		// Order ID (optional).
		t.OrderID = ExtractOrderID(text)

		// Description: text after the order ID or after the amount.
		desc := text
		if t.OrderID != "" {
			if i := strings.Index(desc, t.OrderID); i >= 0 {
				desc = desc[i+len(t.OrderID):]
			}
		} else if t.Amount != 0 {
			// Try to take text after the money string.
			if loc := moneyRegex.FindStringIndex(desc); loc != nil {
				desc = desc[loc[1]:]
			}
		}
		desc = strings.TrimSpace(desc)
		if len(desc) > 200 {
			desc = desc[:200] + "..."
		}
		t.Description = desc

		// Skip rows that are pure junk (no amount AND no order ID).
		if t.Amount == 0 && t.OrderID == "" {
			continue
		}
		page.Transactions = append(page.Transactions, t)
	}

	return page, nil
}

// lastSegmentLike returns the last short identifier-like phrase from a string,
// useful for finding the payment method portion in noisy text. Returns the
// input unchanged if it's already short.
func lastSegmentLike(s string) string {
	s = strings.TrimSpace(s)
	if len(s) < 30 {
		return s
	}
	// Take the last 30 chars and find a reasonable break point.
	tail := s[len(s)-30:]
	if i := strings.Index(tail, "Completed"); i >= 0 {
		return strings.TrimSpace(tail[i+len("Completed"):])
	}
	return strings.TrimSpace(tail)
}
