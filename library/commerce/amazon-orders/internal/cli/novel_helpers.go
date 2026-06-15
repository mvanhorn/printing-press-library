// Hand-written helpers for the novel commands. These commands operate
// across multiple windows of the order-history listing: where-is-my-stuff,
// arriving-soon, late, find, spend, top-items.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-orders/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-orders/internal/parser"
)

var _ = context.Background // keep context import in case future helpers use it

// fetchOrderListPages walks the live /your-orders/orders endpoint for the
// given timeFilter, paginating until either there are no more orders, or
// maxPages is reached. Returns the merged list of OrderSummary records.
func fetchOrderListPages(ctx context.Context, c *client.Client, timeFilter string, maxPages int) ([]parser.OrderSummary, error) {
	if maxPages <= 0 {
		maxPages = 6
	}
	var all []parser.OrderSummary
	seen := map[string]bool{}
	for page := 0; page < maxPages; page++ {
		startIndex := page * 10
		params := map[string]string{}
		if timeFilter != "" {
			params["timeFilter"] = timeFilter
		}
		if startIndex > 0 {
			params["startIndex"] = fmt.Sprintf("%d", startIndex)
		}
		raw, err := c.Get("/your-orders/orders", params)
		if err != nil {
			// If the first page fails, surface; if a later page fails, stop and return what we have.
			if page == 0 {
				return nil, err
			}
			break
		}
		// A logged-out/expired session is answered with HTTP 200 and an Amazon
		// sign-in/claim page. On the first page that's an auth failure; surface
		// it instead of rolling up an empty spend report. On later pages, stop
		// and return what we have.
		if ierr := parser.AuthInterstitialError(raw); ierr != nil {
			if page == 0 {
				return nil, ierr
			}
			break
		}
		listPage, perr := parser.ParseOrderList(raw)
		if perr != nil {
			return nil, perr
		}
		if len(listPage.Orders) == 0 {
			break
		}
		newCount := 0
		for _, o := range listPage.Orders {
			if o.OrderID == "" || seen[o.OrderID] {
				continue
			}
			seen[o.OrderID] = true
			all = append(all, o)
			newCount++
		}
		if !listPage.HasNext || newCount == 0 {
			break
		}
	}
	return all, nil
}

// inflightOrders filters orders to those still in-transit (not Delivered, not
// Cancelled, not Returned, not Refunded). Always returns a non-nil slice so
// JSON output is `[]`, not `null`.
func inflightOrders(orders []parser.OrderSummary) []parser.OrderSummary {
	out := []parser.OrderSummary{}
	for _, o := range orders {
		switch o.Status {
		case "Delivered", "Cancelled", "Returned", "Refunded", "":
			continue
		}
		out = append(out, o)
	}
	return out
}

// PATCH(greptile-arriving-soon-inflight): pre-filter through inflightOrders so Cancelled-after-shipping orders with a future ETA in the listing HTML don't surface as "arriving".
// arrivingByDay filters in-flight orders whose ETA is within [today, today + N
// days] inclusive. Sorted by ETA ascending. Always returns a non-nil slice.
func arrivingByDay(orders []parser.OrderSummary, days int) []parser.OrderSummary {
	now := time.Now().UTC()
	cutoff := now.AddDate(0, 0, days)
	out := []parser.OrderSummary{}
	// Match the inflight-only contract that lateOrders and where-is-my-stuff
	// already enforce: an order Amazon marked Cancelled after it shipped can
	// still carry a future ETA date, but it is not "arriving".
	for _, o := range inflightOrders(orders) {
		if o.ETADate == "" {
			continue
		}
		eta, err := time.Parse("2006-01-02", o.ETADate)
		if err != nil {
			continue
		}
		if eta.Before(now.AddDate(0, 0, -1)) || eta.After(cutoff) {
			continue
		}
		out = append(out, o)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ETADate < out[j].ETADate })
	return out
}

// PATCH(greptile-lateorders-date-boundary): compare ETA against today's midnight, not the current instant — time.Parse("2006-01-02") returns midnight UTC, so eta.Before(now) marked today-ETA orders as late once UTC passed 00:00 even though the delivery window had not closed.
// lateOrders filters in-flight orders past their ETA. Always returns a non-nil
// slice.
func lateOrders(orders []parser.OrderSummary) []parser.OrderSummary {
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	out := []parser.OrderSummary{}
	for _, o := range inflightOrders(orders) {
		if o.ETADate == "" {
			continue
		}
		eta, err := time.Parse("2006-01-02", o.ETADate)
		if err != nil {
			continue
		}
		// eta is midnight UTC of the ETA date; today is midnight UTC of the
		// current date. An order is only "late" once its ETA date has fully
		// passed — strictly before today, never on it.
		if eta.Before(today) {
			out = append(out, o)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ETADate < out[j].ETADate })
	return out
}

// marshalIndent is shared formatting for novel-command JSON output when not
// going through the standard print pipeline.
func marshalIndent(v interface{}) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}
