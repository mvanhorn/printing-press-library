package parser

import (
	"strings"

	"golang.org/x/net/html"
)

// GiftCardActivity is one entry in the gift card activity log.
type GiftCardActivity struct {
	Date        string  `json:"date,omitempty"`
	DateRaw     string  `json:"dateRaw,omitempty"`
	Kind        string  `json:"kind,omitempty"` // "added", "applied", "refund", or "expired"
	Amount      float64 `json:"amount"`         // signed
	OrderID     string  `json:"orderId,omitempty"`
	Description string  `json:"description,omitempty"`
}

// GiftCardPage is the parsed /gc/balance page.
type GiftCardPage struct {
	Balance  float64            `json:"balance"`
	Currency string             `json:"currency,omitempty"`
	Activity []GiftCardActivity `json:"activity,omitempty"`
}

// ParseGiftCards parses /gc/balance.
func ParseGiftCards(htmlBytes []byte) (*GiftCardPage, error) {
	doc, err := Parse(htmlBytes)
	if err != nil {
		return nil, err
	}
	page := &GiftCardPage{Currency: "USD"}
	docText := Text(doc)

	// Balance: text near "Total balance" or "balance is".
	for _, label := range []string{"Total Gift Card balance", "Total balance", "Your gift card balance"} {
		if i := strings.Index(docText, label); i >= 0 {
			window := docText[i:min(i+80, len(docText))]
			b := ExtractMoney(window)
			if b != 0 {
				page.Balance = b
				break
			}
		}
	}

	// Activity rows: every <tr> in the activity table, plus div fallbacks.
	rows := FindAll(doc, func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return false
		}
		if n.Data == "tr" {
			return true
		}
		if n.Data == "div" && HasClassContaining(n, "gc-activity") {
			return true
		}
		return false
	})

	for _, r := range rows {
		text := strings.TrimSpace(Text(r))
		if text == "" {
			continue
		}
		amount := ExtractMoney(text)
		if amount == 0 {
			continue
		}
		raw := FirstDateLike(text)
		date := ""
		if raw != "" {
			if t := ParseDate(raw); !t.IsZero() {
				date = t.Format("2006-01-02")
			}
		}
		// Determine kind.
		kind := ""
		lower := strings.ToLower(text)
		switch {
		case strings.Contains(lower, "applied"):
			kind = "applied"
		case strings.Contains(lower, "added"):
			kind = "added"
		case strings.Contains(lower, "refund"):
			kind = "refund"
		case strings.Contains(lower, "expired"):
			kind = "expired"
		default:
			if amount < 0 {
				kind = "applied"
			} else {
				kind = "added"
			}
		}

		page.Activity = append(page.Activity, GiftCardActivity{
			Date:        date,
			DateRaw:     raw,
			Kind:        kind,
			Amount:      amount,
			OrderID:     ExtractOrderID(text),
			Description: truncate(text, 200),
		})
	}

	return page, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
