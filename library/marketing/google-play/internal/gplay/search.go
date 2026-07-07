package gplay

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Price filters for search.
const (
	PriceAll  = "0"
	PriceFree = "1"
	PricePaid = "2"
)

// NormalizePrice maps a user-facing price filter to its wire value.
func NormalizePrice(p string) (string, bool) {
	switch strings.ToLower(p) {
	case "all", "":
		return PriceAll, true
	case "free":
		return PriceFree, true
	case "paid":
		return PricePaid, true
	}
	return "", false
}

// Search returns up to num app results for a term. Page 1 comes from the
// search HTML (ds:4); further pages follow the qnKhOb continuation RPC.
func (c *Client) Search(ctx context.Context, term, price string, num int) ([]LiteApp, error) {
	if term == "" {
		return nil, fmt.Errorf("term is required")
	}
	wire, ok := NormalizePrice(price)
	if !ok {
		return nil, fmt.Errorf("unknown price filter %q (use all, free, or paid)", price)
	}
	if num <= 0 {
		num = 30
	}
	if num > 250 {
		num = 250
	}

	q := url.Values{}
	q.Set("q", term)
	q.Set("c", "apps")
	q.Set("price", wire)
	html, err := c.getHTML(ctx, "/store/search", q)
	if err != nil {
		return nil, err
	}
	ds, err := extractAFData(html)
	if err != nil {
		return nil, fmt.Errorf("parsing search page: %w", err)
	}
	raw, ok := ds["ds:4"]
	if !ok {
		return nil, fmt.Errorf("search results block (ds:4) not found (store may have changed)")
	}
	apps, token := parseSearchPage1(decode(raw))
	out := apps
	for len(out) < num && token != "" {
		page, next, err := c.searchNext(ctx, token, num-len(out))
		if err != nil {
			return out, err
		}
		if len(page) == 0 {
			break
		}
		out = append(out, page...)
		token = next
	}
	if len(out) > num {
		out = out[:num]
	}
	return out, nil
}

// parseSearchPage1 finds the section in ds:4[0][1] whose [22][0] holds apps and
// returns the lite apps plus a continuation token.
func parseSearchPage1(root node) ([]LiteApp, string) {
	sections := root.path(0, 1)
	for _, sec := range sections.arr() {
		apps := sec.path(22, 0)
		if !apps.isArray() || apps.len() == 0 {
			continue
		}
		var out []LiteApp
		for _, e := range apps.arr() {
			la := parseSearchApp(e)
			if la.AppID != "" {
				out = append(out, la)
			}
		}
		token := sec.path(22, 1, 3, 1).str()
		if len(out) > 0 {
			return out, token
		}
	}
	return nil, ""
}

// parseSearchApp maps a page-1 search app entry. The page-1 shape matches the
// chart entry shape (wrapped under [0]).
func parseSearchApp(e node) LiteApp {
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
	la.Price = e.path(0, 8, 1, 0, 0).float() / 1e6
	la.Free = la.Price == 0
	if la.URL != "" && !strings.HasPrefix(la.URL, "http") {
		la.URL = baseURL + la.URL
	}
	return la
}

// searchNext follows the qnKhOb continuation RPC.
func (c *Client) searchNext(ctx context.Context, token string, want int) ([]LiteApp, string, error) {
	n := want
	if n <= 0 || n > 100 {
		n = 100
	}
	inner := fmt.Sprintf(
		`[[null,[[10,[10,%s]],true,null,[96,27,4,8,57,30,110,79,11,16,49,1,3,9,12,104,55,56,51,10,34,77]],null,%q]]`,
		strconv.Itoa(n), token,
	)
	payload, err := c.batchExecute(ctx, "qnKhOb", inner, "generic", "")
	if err != nil {
		return nil, "", err
	}
	if payload == nil {
		return nil, "", nil
	}
	root := decode(payload)
	apps := root.path(0, 0, 0)
	var out []LiteApp
	for _, e := range apps.arr() {
		la := parseContinuationApp(e)
		if la.AppID != "" {
			out = append(out, la)
		}
	}
	next := root.path(0, 0, 7, 1).str()
	return out, next, nil
}

// parseContinuationApp maps the qnKhOb continuation item shape (appList MAPPINGS).
func parseContinuationApp(e node) LiteApp {
	la := LiteApp{
		AppID:     e.path(12, 0).str(),
		Title:     e.path(2).cleanStr(),
		URL:       e.path(9, 4, 2).str(),
		Icon:      e.path(1, 1, 0, 3, 2).str(),
		Developer: e.path(4, 0, 0, 0).cleanStr(),
		Summary:   e.path(4, 1, 1, 1, 1).cleanStr(),
		ScoreText: e.path(6, 0, 2, 1, 0).str(),
		Score:     e.path(6, 0, 2, 1, 1).float(),
	}
	if la.URL != "" && !strings.HasPrefix(la.URL, "http") {
		la.URL = baseURL + la.URL
	}
	la.Free = true
	return la
}
