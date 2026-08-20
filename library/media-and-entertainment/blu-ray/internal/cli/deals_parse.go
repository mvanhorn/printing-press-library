package cli

// PATCH: Shared structured parser for Blu-ray.com deals rows.

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"

	xhtml "golang.org/x/net/html"
)

type DealRow struct {
	ReleaseID  int     `json:"release_id"`
	Title      string  `json:"title"`
	Kind       string  `json:"kind"`
	SalePrice  float64 `json:"sale_price"`
	ListPrice  float64 `json:"list_price,omitempty"`
	PercentOff int     `json:"percent_off,omitempty"`
	PostedAgo  string  `json:"posted_ago"`
	RetailerID int     `json:"retailer_id"`
	CoverURL   string  `json:"cover_url,omitempty"`
	DetailURL  string  `json:"detail_url"`
}

var (
	priceRE      = regexp.MustCompile(`\$([0-9]+\.[0-9]{1,2}|[0-9]+|\.[0-9]{1,2})`)
	percentOffRE = regexp.MustCompile(`([0-9]{1,3})\s*%`)
)

func parseDealsHTML(body []byte) ([]DealRow, error) {
	doc, err := parseHTMLLatin1(body)
	if err != nil {
		return nil, err
	}

	var rows []DealRow
	walkHTML(doc, func(n *xhtml.Node) {
		if n.Type != xhtml.ElementNode || !strings.EqualFold(n.Data, "a") || !strings.Contains(attrValue(n, "class"), "active") {
			return
		}
		href := absoluteBluRayURL(attrValue(n, "href"))
		if !strings.Contains(href, "/link/click.php") {
			return
		}
		u, err := url.Parse(href)
		if err != nil {
			return
		}
		rid, _ := strconv.Atoi(u.Query().Get("retailerid"))
		container := nearestDealContainer(n)
		if container == nil {
			container = n.Parent
		}
		row := DealRow{RetailerID: rid}
		row.CoverURL = firstImageSrc(container, "https://www.blu-ray.com/")
		text := cleanHTMLText(nodeText(container))
		prices := priceRE.FindAllStringSubmatch(text, -1)
		if len(prices) > 0 {
			row.SalePrice, _ = strconv.ParseFloat(normalizePriceMatch(prices[0][1]), 64)
		}
		if len(prices) > 1 {
			row.ListPrice, _ = strconv.ParseFloat(normalizePriceMatch(prices[1][1]), 64)
		}
		if m := percentOffRE.FindStringSubmatch(text); len(m) == 2 {
			row.PercentOff, _ = strconv.Atoi(m[1])
		}
		walkHTML(container, func(x *xhtml.Node) {
			if x.Type != xhtml.ElementNode {
				return
			}
			switch strings.ToLower(x.Data) {
			case "a":
				link := absoluteBluRayURL(attrValue(x, "href"))
				if row.ReleaseID == 0 {
					if kind, slug, id, ok := parseReleaseURL(link); ok {
						row.ReleaseID = id
						row.Kind = kind
						row.DetailURL = link
						row.Title = titleFromSlug(slug)
					}
				}
			case "b":
				if row.PostedAgo == "" {
					row.PostedAgo = cleanHTMLText(attrValue(x, "title"))
				}
			}
		})
		if row.ReleaseID > 0 && row.SalePrice > 0 {
			rows = append(rows, row)
		}
	})
	return rows, nil
}

func normalizePriceMatch(s string) string {
	if strings.HasPrefix(s, ".") {
		return "0" + s
	}
	return s
}

func nearestDealContainer(n *xhtml.Node) *xhtml.Node {
	for cur := n; cur != nil; cur = cur.Parent {
		if cur.Type == xhtml.ElementNode {
			switch strings.ToLower(cur.Data) {
			case "tr", "li":
				return cur
			case "div":
				class := strings.ToLower(attrValue(cur, "class"))
				if strings.Contains(class, "deal") || strings.Contains(class, "item") {
					return cur
				}
			}
		}
	}
	return nil
}
