package rewe

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Product is a search hit reduced to what an order needs.
type Product struct {
	ProductID  string `json:"product_id"`
	Name       string `json:"name"`
	Brand      string `json:"brand,omitempty"`
	ListingID  string `json:"listing_id"`
	PriceCents int    `json:"price_cents"`
	Grammage   string `json:"grammage,omitempty"`
	OrderLimit int    `json:"order_limit,omitempty"`
}

// ParseProducts reads /shop/api/products.
func ParseProducts(b []byte) ([]Product, error) {
	var v struct {
		Embedded struct {
			Products []struct {
				ID          string `json:"id"`
				ProductName string `json:"productName"`
				Brand       struct {
					Name string `json:"name"`
				} `json:"brand"`
				Embedded struct {
					Articles []struct {
						Embedded struct {
							Listing struct {
								ID      string `json:"id"`
								Pricing struct {
									CurrentRetailPrice int    `json:"currentRetailPrice"`
									Grammage           string `json:"grammage"`
								} `json:"pricing"`
								Limitations struct {
									OrderLimit int `json:"orderLimit"`
								} `json:"limitations"`
							} `json:"listing"`
						} `json:"_embedded"`
					} `json:"articles"`
				} `json:"_embedded"`
			} `json:"products"`
		} `json:"_embedded"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, err
	}
	out := []Product{}
	for _, p := range v.Embedded.Products {
		pr := Product{ProductID: p.ID, Name: p.ProductName, Brand: p.Brand.Name}
		if len(p.Embedded.Articles) > 0 {
			l := p.Embedded.Articles[0].Embedded.Listing
			pr.ListingID, pr.PriceCents, pr.Grammage, pr.OrderLimit = l.ID, l.Pricing.CurrentRetailPrice, l.Pricing.Grammage, l.Limitations.OrderLimit
		}
		out = append(out, pr)
	}
	return out, nil
}

// SearchProducts searches the selected market's catalogue.
func (sh *Shop) SearchProducts(ctx context.Context, query string, perPage int) ([]Product, error) {
	if perPage <= 0 {
		perPage = 20
	}
	q := url.Values{"search": {query}, "serviceTypes": {"PICKUP"}, "objectsPerPage": {strconv.Itoa(perPage)}, "page": {"1"}}
	_, b, err := sh.do(ctx, http.MethodGet, "/shop/api/products?"+q.Encode(), nil, nil)
	if err != nil {
		return nil, err
	}
	return ParseProducts(b)
}

var foldReplacer = strings.NewReplacer("ä", "ae", "ö", "oe", "ü", "ue", "ß", "ss", "-", " ", "!", " ", ",", " ", ".", " ")

func tokens(s string) []string {
	return strings.Fields(foldReplacer.Replace(strings.ToLower(s)))
}

// BestMatch picks the product that best fits a free-text item: all query
// words in the name first, then the most words, keeping REWE's relevance order
// on ties. Products without a listing (not orderable) are skipped.
func BestMatch(query string, products []Product) (Product, bool) {
	q := tokens(query)
	best, bestScore := -1, -1
	for i, p := range products {
		if p.ListingID == "" {
			continue
		}
		name := " " + strings.Join(tokens(p.Name+" "+p.Brand), " ") + " "
		score := 0
		for _, t := range q {
			switch {
			case strings.Contains(name, " "+t):
				score += 2
			case strings.Contains(name, t):
				score++
			case len(t) >= 7 && strings.Contains(name, " "+t[:5]):
				// German compounds: "hafermilch" should find "Hafer Drink".
				score++
			}
		}
		if score > bestScore {
			best, bestScore = i, score
		}
	}
	if best < 0 || bestScore == 0 {
		return Product{}, false
	}
	return products[best], true
}

// ListItem is one parsed shopping-list line.
type ListItem struct {
	Query    string `json:"query"`
	Quantity int    `json:"quantity"`
}

var (
	leadQtyRe  = regexp.MustCompile(`^(\d+)\s*(?:x|×|stk\.?|stück)?\s+(.+)$`)
	trailQtyRe = regexp.MustCompile(`^(.+?)\s+(?:x\s*)?(\d+)\s*(?:x|stk\.?|stück)?$`)
)

// ParseList turns shopping-list text into items; blank lines and bullets are ignored.
func ParseList(text string) []ListItem {
	var out []ListItem
	for _, line := range strings.Split(text, "\n") {
		l := strings.TrimSpace(line)
		l = strings.TrimLeft(l, "-*•·[] ")
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		item := ListItem{Query: l, Quantity: 1}
		if m := leadQtyRe.FindStringSubmatch(strings.ToLower(l)); m != nil {
			n, _ := strconv.Atoi(m[1])
			item = ListItem{Query: strings.TrimSpace(l[len(l)-len(m[2]):]), Quantity: n}
		} else if m := trailQtyRe.FindStringSubmatch(l); m != nil {
			n, _ := strconv.Atoi(m[2])
			item = ListItem{Query: strings.TrimSpace(m[1]), Quantity: n}
		}
		if item.Quantity <= 0 {
			item.Quantity = 1
		}
		if item.Quantity > 99 {
			item.Quantity = 99
		}
		out = append(out, item)
	}
	return out
}

// FormatCents renders 189 as "1,89 €".
func FormatCents(c int) string {
	return fmt.Sprintf("%d,%02d €", c/100, c%100)
}
