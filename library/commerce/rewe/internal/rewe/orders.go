package rewe

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// OrderSummary is one online order from "Deine Einkäufe".
type OrderSummary struct {
	ID     string `json:"id"`
	Status string `json:"status,omitempty"`
	Slot   string `json:"slot,omitempty"`
	URL    string `json:"url"`
}

var (
	orderLinkRe   = regexp.MustCompile(`/shop/mydata/orderdetail/([A-Z0-9-]{6,})`)
	orderIDRe     = regexp.MustCompile(`^[A-Z0-9-]{6,32}$`)
	tagRe         = regexp.MustCompile(`(?s)<script.*?</script>|<style.*?</style>|<[^>]+>`)
	orderStatusRe = regexp.MustCompile(`Bestellnummer:\s*[A-Z0-9-]+\s+(.{3,40}?)\s+(?:Abholtermin|Liefertermin|Mehr Infos|Kategorie)`)
	orderSlotRe   = regexp.MustCompile(`(?:Abholtermin|Liefertermin):\s*([0-9.]+\s*\|\s*[0-9:]+\s*.\s*[0-9:]+)`)
)

// PlainText strips tags and collapses whitespace.
func PlainText(page string) string {
	return strings.Join(strings.Fields(tagRe.ReplaceAllString(page, " ")), " ")
}

// ParseOrderIDs finds order ids linked from the order overview.
func ParseOrderIDs(page string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, m := range orderLinkRe.FindAllStringSubmatch(page, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	return out
}

// ParseOrderDetail reads status and slot from an order detail page.
func ParseOrderDetail(id, page string) OrderSummary {
	t := PlainText(page)
	o := OrderSummary{ID: id, URL: BaseURL + "/shop/mydata/orderdetail/" + id}
	if m := orderStatusRe.FindStringSubmatch(t); m != nil {
		o.Status = strings.TrimSpace(m[1])
	}
	if m := orderSlotRe.FindStringSubmatch(t); m != nil {
		o.Slot = strings.TrimSpace(m[1])
	}
	return o
}

// Orders lists online orders with status.
func (sh *Shop) Orders(ctx context.Context) ([]OrderSummary, error) {
	page, _, err := sh.HTML(ctx, "/shop/mydata/meine-einkaeufe")
	if err != nil {
		return nil, err
	}
	out := []OrderSummary{}
	for _, id := range ParseOrderIDs(page) {
		o, err := sh.Order(ctx, id)
		if err != nil {
			return out, err
		}
		out = append(out, o)
	}
	return out, nil
}

// Order reads one order.
func (sh *Shop) Order(ctx context.Context, id string) (OrderSummary, error) {
	if !orderIDRe.MatchString(id) {
		return OrderSummary{}, errors.New("order id must look like B-AAA-BBB-CCC")
	}
	page, _, err := sh.HTML(ctx, "/shop/mydata/orderdetail/"+url.PathEscape(id))
	if err != nil {
		return OrderSummary{}, err
	}
	return ParseOrderDetail(id, page), nil
}

// CancelOrder sends the "Bestellung stornieren" request (an htmx GET on the
// order page) and returns the confirmation text REWE shows.
func (sh *Shop) CancelOrder(ctx context.Context, id string) (string, error) {
	if !orderIDRe.MatchString(id) {
		return "", errors.New("order id must look like B-AAA-BBB-CCC")
	}
	_, b, err := sh.do(ctx, http.MethodGet, "/shop/omf/cancel/"+url.PathEscape(id), nil, map[string]string{
		"Accept":         "text/html",
		"HX-Request":     "true",
		"HX-Current-URL": BaseURL + "/shop/mydata/orderdetail/" + id,
	})
	if err != nil {
		return "", err
	}
	return PlainText(string(b)), nil
}
