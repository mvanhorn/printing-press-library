package gplay

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// Collection wire values for the top-charts RPC.
var collectionWire = map[string]string{
	"TOP_FREE":     "topselling_free",
	"TOP_PAID":     "topselling_paid",
	"GROSSING":     "topgrossing",
	"TOP_GROSSING": "topgrossing",
}

// NormalizeCollection maps a user-facing collection name to its wire value.
func NormalizeCollection(c string) (string, bool) {
	wire, ok := collectionWire[strings.ToUpper(strings.TrimSpace(c))]
	return wire, ok
}

// CollectionNames returns the accepted collection inputs.
func CollectionNames() []string { return []string{"TOP_FREE", "TOP_PAID", "GROSSING"} }

// TopCharts returns the ranked chart for a collection + category via the vyAe2
// batchexecute RPC. num is capped by Google around ~660.
func (c *Client) TopCharts(ctx context.Context, collection, category string, num int) ([]LiteApp, error) {
	wire, ok := NormalizeCollection(collection)
	if !ok {
		return nil, fmt.Errorf("unknown collection %q (use TOP_FREE, TOP_PAID, or GROSSING)", collection)
	}
	if category == "" {
		category = "APPLICATION"
	}
	category = strings.ToUpper(category)
	if num <= 0 {
		num = 50
	}

	body := chartsFReqTemplate
	body = strings.ReplaceAll(body, "${num}", strconv.Itoa(num))
	body = strings.ReplaceAll(body, "${collection}", wire)
	body = strings.ReplaceAll(body, "${category}", category)

	payload, err := c.batchExecute(ctx, "vyAe2", "", "", body)
	if err != nil {
		return nil, err
	}
	if payload == nil {
		return nil, fmt.Errorf("no chart data for %s/%s (category may be invalid)", collection, category)
	}
	root := decode(payload)
	apps := root.path(0, 1, 0, 28, 0)
	if !apps.isArray() {
		return nil, fmt.Errorf("chart layout not recognized for %s/%s (store may have changed)", collection, category)
	}
	var out []LiteApp
	for _, e := range apps.arr() {
		la := parseChartApp(e)
		if la.AppID != "" {
			out = append(out, la)
		}
	}
	return out, nil
}

func parseChartApp(e node) LiteApp {
	la := LiteApp{
		AppID:     e.path(0, 0, 0).str(),
		Title:     e.path(0, 3).cleanStr(),
		URL:       e.path(0, 10, 4, 2).str(),
		Icon:      e.path(0, 1, 3, 2).str(),
		Developer: e.path(0, 14).cleanStr(),
		Summary:   e.path(0, 13, 1).cleanStr(),
		ScoreText: e.path(0, 4, 0).str(),
		Score:     e.path(0, 4, 1).float(),
		Currency:  e.path(0, 8, 1, 0, 1).str(),
	}
	priceMicros := e.path(0, 8, 1, 0, 0).float()
	la.Price = priceMicros / 1e6
	la.Free = la.Price == 0
	if la.URL != "" && !strings.HasPrefix(la.URL, "http") {
		la.URL = baseURL + la.URL
	}
	return la
}
