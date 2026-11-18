package rewe

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// Offer is one market offer ("Angebot").
type Offer struct {
	NAN         string `json:"nan"`
	Title       string `json:"title"`
	Details     string `json:"details,omitempty"`
	Price       string `json:"price,omitempty"`
	PriceCents  int    `json:"price_cents,omitempty"`
	Label       string `json:"label,omitempty"`
	BonusCredit string `json:"bonus_credit,omitempty"`
	Category    string `json:"category,omitempty"`
	Week        string `json:"week"`
	Market      string `json:"market"`
	Image       string `json:"image,omitempty"`
}

// offerSlot is a placeholder div of the offers page (rendered or not).
type offerSlot struct {
	NAN, Category, Week, Market string
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func hasClass(n *html.Node, c string) bool {
	for _, f := range strings.Fields(attr(n, "class")) {
		if f == c {
			return true
		}
	}
	return false
}

func textOf(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}

func walkAll(n *html.Node, fn func(*html.Node)) {
	fn(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walkAll(c, fn)
	}
}

var priceRe = regexp.MustCompile(`(\d+),(\d{2})\s*€`)

// PriceToCents converts "1,29 €" to 129 (0 when not parseable).
func PriceToCents(s string) int {
	m := priceRe.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	e, _ := strconv.Atoi(m[1])
	c, _ := strconv.Atoi(m[2])
	return e*100 + c
}

// parseTile reads one <article class="cor-offer-renderer-tile">.
func parseTile(art *html.Node) Offer {
	var o Offer
	var details []string
	walkAll(art, func(n *html.Node) {
		if n.Type != html.ElementNode {
			return
		}
		switch {
		case n.Data == "a" && attr(n, "data-offer-nan") != "":
			o.NAN = attr(n, "data-offer-nan")
			o.Title = attr(n, "data-offer-title")
			o.Week = attr(n, "data-offer-week")
			o.Market = attr(n, "data-offer-wwident")
		case hasClass(n, "cor-offer-information__additional"):
			details = append(details, strings.TrimLeft(textOf(n), ", "))
		case hasClass(n, "cor-offer-price__tag-price"):
			o.Price = textOf(n)
		case hasClass(n, "cor-offer-price__tag-label"):
			o.Label = textOf(n)
		case hasClass(n, "cor-loyalty-badge"):
			o.BonusCredit = textOf(n)
		case n.Data == "img" && attr(n, "data-testid") == "offer-image":
			o.Image = attr(n, "data-src")
		}
	})
	if o.Title == "" {
		walkAll(art, func(n *html.Node) {
			if o.Title == "" && n.Type == html.ElementNode && hasClass(n, "cor-offer-information__title") {
				o.Title = textOf(n)
			}
		})
	}
	o.Details = strings.Join(details, ", ")
	o.PriceCents = PriceToCents(o.Price)
	return o
}

// ParseOffersPage returns the rendered offers and the placeholders still to load.
func ParseOffersPage(page string) (rendered []Offer, pending []offerSlot, err error) {
	doc, err := html.Parse(strings.NewReader(page))
	if err != nil {
		return nil, nil, err
	}
	seen := map[string]bool{}
	walkAll(doc, func(n *html.Node) {
		if n.Type != html.ElementNode || n.Data != "div" || !hasClass(n, "sos-offer") {
			return
		}
		slot := offerSlot{NAN: attr(n, "data-offer-nan"), Category: attr(n, "data-category"), Week: attr(n, "data-offer-week"), Market: attr(n, "data-offer-wwident")}
		if slot.NAN == "" {
			return
		}
		key := slot.NAN + "|" + slot.Week
		if seen[key] {
			return
		}
		seen[key] = true
		var art *html.Node
		walkAll(n, func(x *html.Node) {
			if art == nil && x.Type == html.ElementNode && x.Data == "article" && hasClass(x, "cor-offer-renderer-tile") {
				art = x
			}
		})
		if art == nil {
			pending = append(pending, slot)
			return
		}
		o := parseTile(art)
		if o.NAN == "" {
			o.NAN = slot.NAN
		}
		if o.Week == "" {
			o.Week = slot.Week
		}
		if o.Market == "" {
			o.Market = slot.Market
		}
		o.Category = slot.Category
		rendered = append(rendered, o)
	})
	return rendered, pending, nil
}

// ParseTileHTML parses the content of one frontend-include answer.
func ParseTileHTML(fragment string) (Offer, error) {
	doc, err := html.Parse(strings.NewReader(fragment))
	if err != nil {
		return Offer{}, err
	}
	var art *html.Node
	walkAll(doc, func(x *html.Node) {
		if art == nil && x.Type == html.ElementNode && x.Data == "article" {
			art = x
		}
	})
	if art == nil {
		return Offer{}, fmt.Errorf("no offer tile in fragment")
	}
	return parseTile(art), nil
}

// OffersURL resolves the offers page path of a market (the shop redirects
// /angebote/ to /angebote/<city>/<market>/<slug>/ using the market cookie).
func (sh *Shop) OffersURL(ctx context.Context) (string, error) {
	_, final, err := sh.HTML(ctx, "/angebote/")
	if err != nil {
		return "", err
	}
	if strings.Contains(final, "nationale-angebote") {
		return "", fmt.Errorf("no market selected: run `markets select` first")
	}
	return final, nil
}

// Offers loads all offers of the market page, rendering the lazy tiles in
// batches through /api/frontend-includes.
func (sh *Shop) Offers(ctx context.Context, pageURL string, maxLazy int) ([]Offer, error) {
	page, _, err := sh.HTML(ctx, pageURL)
	if err != nil {
		return nil, err
	}
	offers, pending, err := ParseOffersPage(page)
	if err != nil {
		return nil, err
	}
	if maxLazy >= 0 && len(pending) > maxLazy {
		pending = pending[:maxLazy]
	}
	const batch = 20
	for i := 0; i < len(pending); i += batch {
		end := i + batch
		if end > len(pending) {
			end = len(pending)
		}
		reqs := []map[string]any{}
		byID := map[string]offerSlot{}
		for j, s := range pending[i:end] {
			id := fmt.Sprintf("t%d", i+j)
			byID[id] = s
			reqs = append(reqs, map[string]any{"id": id, "name": "offer-tile-by-nan", "namespace": "cor", "params": map[string]string{
				"nan": s.NAN, "wwIdent": s.Market, "showDuration": "auto", "enableDetailDeeplink": "true", "week": s.Week, "crossOutPriceArrowStyle": "false"}})
		}
		var answers []struct {
			ID      string `json:"id"`
			Content string `json:"content"`
		}
		if err := sh.JSON(ctx, http.MethodPost, "/api/frontend-includes", reqs, &answers, nil); err != nil {
			return offers, fmt.Errorf("loading offer tiles: %w", err)
		}
		for _, a := range answers {
			s := byID[a.ID]
			o, err := ParseTileHTML(a.Content)
			if err != nil {
				continue
			}
			if o.NAN == "" {
				o.NAN = s.NAN
			}
			if o.Week == "" {
				o.Week = s.Week
			}
			if o.Market == "" {
				o.Market = s.Market
			}
			o.Category = s.Category
			offers = append(offers, o)
		}
	}
	return offers, nil
}

// MarshalOffers is a helper for raw JSON output.
func MarshalOffers(o []Offer) json.RawMessage {
	b, _ := json.Marshal(o)
	return b
}
