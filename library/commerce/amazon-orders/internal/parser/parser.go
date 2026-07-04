// Package parser extracts structured order, item, shipment, transaction, and
// gift-card data from Amazon's authenticated HTML pages.
//
// Amazon publishes no buyer-side API; the CLI fetches the same HTML pages a
// logged-in browser sees, then parses them here. The parsers are intentionally
// tolerant: Amazon ships A/B variants, locale-specific date formats, and
// promo/banner injections, so each parser walks for known anchor text and
// regex shapes rather than rigid CSS paths.
package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-orders/internal/cliutil"
	"golang.org/x/net/html"
)

// OrderID matches the canonical XXX-XXXXXXX-XXXXXXX shape Amazon uses for
// physical-goods order IDs (D01-style digital-order IDs match too because the
// segments are 3-7-7).
var orderIDRegex = regexp.MustCompile(`\b\d{3}-\d{7}-\d{7}\b|\bD\d{2}-\d{7}-\d{7}\b`)

// asinRegex extracts a 10-char Amazon Standard ID Number (ASIN).
var asinRegex = regexp.MustCompile(`/dp/([A-Z0-9]{10})`)

// moneyRegex matches currency-marked money across Amazon marketplaces (e.g.
// "$1,234.56", "-₹1,234.56", "Rs. 999.00", "INR 2,500.00").
var moneyRegex = regexp.MustCompile(`(?i)(-?)\s*(₹|rs\.?|inr|\$|£|€)\s*([0-9][0-9,]*(?:\.\d{2})?)`)

// dateLikeRegex tolerates "May 5, 2026", "May 5", "Jan 1, 2026".
var dateLikeRegex = regexp.MustCompile(`\b(?:Jan(?:uary)?|Feb(?:ruary)?|Mar(?:ch)?|Apr(?:il)?|May|Jun(?:e)?|Jul(?:y)?|Aug(?:ust)?|Sep(?:tember)?|Oct(?:ober)?|Nov(?:ember)?|Dec(?:ember)?)\s+\d{1,2}(?:,\s*\d{4})?\b`)

// last4Regex matches "ending in 1234" or "****1234" payment hints.
var last4Regex = regexp.MustCompile(`(?:ending in|ending|\*+)\s*(\d{4})`)

// Parse parses an HTML byte slice into a node tree. Returns nil node if the
// input is empty or malformed beyond recovery.
func Parse(b []byte) (*html.Node, error) {
	if len(b) == 0 {
		return nil, fmt.Errorf("empty html")
	}
	return html.Parse(strings.NewReader(string(b)))
}

// Walk visits every node in depth-first order, calling fn. Return false from
// fn to stop traversal early.
func Walk(n *html.Node, fn func(*html.Node) bool) {
	if n == nil {
		return
	}
	if !fn(n) {
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		Walk(c, fn)
	}
}

// FindAll returns every node satisfying match.
func FindAll(n *html.Node, match func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	Walk(n, func(node *html.Node) bool {
		if match(node) {
			out = append(out, node)
		}
		return true
	})
	return out
}

// HasClass returns true if n is an element node and its class attribute
// contains every space-separated token in want (any-order match per token,
// not exact equality).
func HasClass(n *html.Node, want ...string) bool {
	if n == nil || n.Type != html.ElementNode {
		return false
	}
	classAttr := Attr(n, "class")
	if classAttr == "" {
		return false
	}
	have := strings.Fields(classAttr)
	for _, w := range want {
		found := false
		for _, h := range have {
			if h == w {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// HasClassContaining returns true if n has any class containing substr.
func HasClassContaining(n *html.Node, substr string) bool {
	if n == nil || n.Type != html.ElementNode {
		return false
	}
	for _, c := range strings.Fields(Attr(n, "class")) {
		if strings.Contains(c, substr) {
			return true
		}
	}
	return false
}

// Attr returns an attribute value or empty string.
func Attr(n *html.Node, key string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// Text returns the concatenated text content of n with whitespace collapsed.
// Wraps cliutil.CleanText for HTML entity unescaping.
func Text(n *html.Node) string {
	if n == nil {
		return ""
	}
	var sb strings.Builder
	Walk(n, func(x *html.Node) bool {
		if x.Type == html.TextNode {
			sb.WriteString(x.Data)
			sb.WriteString(" ")
		}
		return true
	})
	return cliutil.CleanText(sb.String())
}

// FirstByTag returns the first child (or descendant) element of given tag, or nil.
func FirstByTag(n *html.Node, tag string) *html.Node {
	var found *html.Node
	Walk(n, func(x *html.Node) bool {
		if found != nil {
			return false
		}
		if x.Type == html.ElementNode && x.Data == tag {
			found = x
			return false
		}
		return true
	})
	return found
}

// ExtractOrderID pulls an order ID out of arbitrary text. Empty string if none.
func ExtractOrderID(text string) string {
	return orderIDRegex.FindString(text)
}

// ExtractASIN pulls an ASIN from an Amazon /dp/ URL. Empty string if none.
func ExtractASIN(url string) string {
	m := asinRegex.FindStringSubmatch(url)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

// ExtractMoney returns the first money value in text as a float (e.g. 51.46).
// Returns 0 if no money string is found. Negative for negative amounts.
func ExtractMoney(text string) float64 {
	v, _, ok := ExtractMoneyWithCurrency(text)
	if !ok {
		return 0
	}
	return v
}

// ExtractMoneyWithCurrency returns the first money value and detected ISO-ish
// currency code. ok is false when no supported currency-marked amount exists.
func ExtractMoneyWithCurrency(text string) (amount float64, currency string, ok bool) {
	m := moneyRegex.FindStringSubmatch(text)
	if len(m) < 4 {
		return 0, "", false
	}
	clean := strings.ReplaceAll(m[3], ",", "")
	v, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		return 0, "", false
	}
	if m[1] == "-" {
		v = -v
	}
	return v, currencyForMoneyToken(m[2]), true
}

func currencyForMoneyToken(token string) string {
	switch strings.ToUpper(strings.TrimSpace(token)) {
	case "₹", "RS", "RS.", "INR":
		return "INR"
	case "$":
		return "USD"
	case "£":
		return "GBP"
	case "€":
		return "EUR"
	default:
		return ""
	}
}

// ParseDate tolerates several Amazon date strings ("May 5, 2026", "May 5",
// "May 5 - May 7"). Returns time.Time at UTC midnight, or zero time if not
// parseable.
func ParseDate(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	// If it's a range like "May 5 - May 7, 2026", take the END.
	if i := strings.Index(s, " - "); i >= 0 {
		s = strings.TrimSpace(s[i+3:])
	}
	formats := []string{"January 2, 2006", "Jan 2, 2006", "January 2", "Jan 2"}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			// If year is missing, assume current year.
			if t.Year() == 0 {
				t = time.Date(time.Now().Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
			}
			return t
		}
	}
	return time.Time{}
}

// ExtractLast4 pulls a credit-card last-4 from "ending in NNNN" or "****NNNN".
func ExtractLast4(text string) string {
	m := last4Regex.FindStringSubmatch(text)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

// FirstDateLike returns the first month-day(-year) substring in text.
func FirstDateLike(text string) string {
	return dateLikeRegex.FindString(text)
}
