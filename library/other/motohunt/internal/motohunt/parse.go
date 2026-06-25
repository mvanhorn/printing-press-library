// Copyright 2026 richardadonnell. Licensed under Apache-2.0. See LICENSE.
// Hand-written goquery parsers, verified against live HTML 2026-06-10.

package motohunt

import (
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/motohunt/internal/cliutil"

	"github.com/PuerkitoBio/goquery"
)

// dealRatingFromBadge maps a deal-rating badge to a normalized rating, or ""
// when the badge isn't a deal rating (e.g. "Low Mileage", "Popular").
func dealRatingFromBadge(b string) string {
	switch strings.ToLower(strings.TrimSpace(b)) {
	case "great price":
		return "Great Price"
	case "good price":
		return "Good Price"
	case "fair price":
		return "Fair Price"
	case "high price":
		return "High Price"
	}
	return ""
}

// ParseCards extracts listing cards from a search results document. Each card
// is a `#srp-results-container div.card` containing a `span.save-bike`. The
// detail link (and the postId) repeats many times per card (carousel images),
// so id/url are read once per card.
func ParseCards(doc *goquery.Document, site SiteConfig) []Listing {
	out := make([]Listing, 0)
	doc.Find("#srp-results-container div.card").Each(func(_ int, card *goquery.Selection) {
		save := card.Find("span.save-bike").First()
		if save.Length() == 0 {
			return // not a listing card (e.g. nested card-body matched div.card)
		}
		l := Listing{}

		// id: span.save-bike[postId="p:13194784"] -> strip p:. Fall back to href.
		if pid, ok := save.Attr("postId"); ok {
			l.ID = strings.TrimPrefix(strings.TrimSpace(pid), "p:")
		}
		// detail url: first a[href^="/l/"] in the card. Dedupe by taking First.
		if href, ok := card.Find(`a[href^="/l/"]`).First().Attr("href"); ok {
			l.URL = site.Base + href
			if l.ID == "" {
				l.ID = idFromDetailPath(href)
			}
		}

		l.Title = cliutil.CleanText(card.Find(".sc-title").First().Text())

		// .sc-line2 rows carry price + (condition | mileage). Iterate each
		// .sc-line2 and classify its columns.
		card.Find(".sc-line2").Each(func(_ int, row *goquery.Selection) {
			row.Children().Each(func(_ int, col *goquery.Selection) {
				// Mileage: a column whose svg <title> is "Mileage". The svg
				// carries a literal "Mileage" <title> that bleeds into col.Text();
				// clone and drop the svg so only the value (e.g. "16,285m") survives.
				if col.Find("svg title").FilterFunction(func(_ int, t *goquery.Selection) bool {
					return strings.EqualFold(strings.TrimSpace(t.Text()), "Mileage")
				}).Length() > 0 {
					mileageCol := col.Clone()
					mileageCol.Find("svg").Remove()
					if m := normalizeSpace(cliutil.CleanText(mileageCol.Text())); m != "" && l.Mileage == "" {
						l.Mileage = m
					}
					return
				}
				txt := cliutil.CleanText(col.Text())
				if txt == "" {
					return
				}
				// Price column: contains a $ amount (current price, bold).
				if l.Price == "" && strings.Contains(txt, "$") {
					l.Price = firstDollarAmount(col)
					return
				}
				// Condition column: "Used" / "New" (no $, no svg).
				if l.Condition == "" && isCondition(txt) {
					l.Condition = txt
				}
			})
		})

		// badges
		card.Find(".sc-badges-container .badge").Each(func(_ int, b *goquery.Selection) {
			txt := cliutil.CleanText(b.Text())
			if txt == "" {
				return
			}
			l.Badges = append(l.Badges, txt)
			if dr := dealRatingFromBadge(txt); dr != "" && l.DealRating == "" {
				l.DealRating = dr
			}
		})

		// location: the row whose .sc-loc-icon-div carries an svg <title>Location</title>.
		loc := card.Find(".sc-loc-icon-div").First()
		if loc.Length() > 0 {
			l.Location = cliutil.CleanText(loc.Find(".sc-line-truncate").First().Text())
			if l.Location == "" {
				l.Location = cliutil.CleanText(loc.Text())
			}
		}

		// dealer: the seller line — .sc-seller-icon marks the row; the dealer
		// name is the sibling .sc-line-truncate.
		seller := card.Find(".sc-seller-icon").First()
		if seller.Length() > 0 {
			l.Dealer = cliutil.CleanText(seller.Parent().Find(".sc-line-truncate").First().Text())
		}

		// image: first img.srp-listing-img / carousel img.
		if src, ok := card.Find("img.srp-listing-img").First().Attr("src"); ok {
			l.Image = strings.TrimSpace(src)
		}
		if l.Image == "" {
			if src, ok := card.Find("img").First().Attr("src"); ok {
				l.Image = strings.TrimSpace(src)
			}
		}

		if l.ID != "" || l.Title != "" {
			out = append(out, l)
		}
	})
	return out
}

// ParseDetail extracts the full listing detail. id is the listing id the URL
// was built from (the page itself doesn't always echo it cleanly).
func ParseDetail(doc *goquery.Document, site SiteConfig, id string) ListingDetail {
	d := ListingDetail{ID: id, URL: site.DetailURL(id)}
	d.Title = cliutil.CleanText(doc.Find("h1").First().Text())
	if d.Title == "" {
		d.Title = cliutil.CleanText(doc.Find("h2").First().Text())
	}

	// Walk every .lp-specs-row: first <b>Label:</b> col, value is the row text
	// after the label is stripped (the value lives in the next column).
	doc.Find(".lp-specs-row").Each(func(_ int, row *goquery.Selection) {
		label := cliutil.CleanText(row.Find("b").First().Text())
		label = strings.TrimSuffix(strings.TrimSpace(label), ":")
		if label == "" {
			return
		}
		val := specValue(row)
		switch strings.ToLower(label) {
		case "location":
			d.Location = val
		case "condition":
			d.Condition = val
		case "dealer":
			d.Dealer = val
		case "price":
			// "Please visit ... for latest price." => no listed price.
			if !strings.HasPrefix(strings.ToLower(val), "please visit") && strings.Contains(val, "$") {
				d.Price = firstDollarString(val)
			}
		case "mileage":
			d.Mileage = val
		case "color":
			d.Color = val
		case "age":
			d.Age = val
		case "stock #", "stock":
			d.StockNumber = val
		case "vin":
			d.VIN = firstToken(val)
		case "base msrp":
			d.BaseMSRP = firstDollarString(val)
		case "alp":
			d.ALP = firstDollarString(val)
		}
	})

	// Certified Pre-Owned: badge present anywhere on the page.
	doc.Find(".badge").EachWithBreak(func(_ int, b *goquery.Selection) bool {
		if strings.EqualFold(cliutil.CleanText(b.Text()), "Certified Pre-Owned") {
			d.CertifiedPreOwned = true
			return false
		}
		return true
	})

	// deal_rating: a deal-rating badge on the detail page, if any.
	doc.Find(".badge").EachWithBreak(func(_ int, b *goquery.Selection) bool {
		if dr := dealRatingFromBadge(cliutil.CleanText(b.Text())); dr != "" {
			d.DealRating = dr
			return false
		}
		return true
	})

	// description: the listing description block.
	if desc := doc.Find(".lp-description, #description, .listing-description").First(); desc.Length() > 0 {
		d.Description = cliutil.CleanText(desc.Text())
	}

	// images: gallery image srcs (dedupe, skip placeholders).
	seen := map[string]bool{}
	doc.Find("img").Each(func(_ int, img *goquery.Selection) {
		src, ok := img.Attr("src")
		if !ok {
			src, _ = img.Attr("data-src")
		}
		src = strings.TrimSpace(src)
		if src == "" || seen[src] || !strings.Contains(src, "googleapis.com") {
			return
		}
		seen[src] = true
		d.Images = append(d.Images, src)
	})

	return d
}

// ParseMakes reads the canonical make list from the search page's
// `<select name="make">` dropdown. Disabled options ("Popular Makes" headers)
// and the empty "Any" option are skipped.
func ParseMakes(doc *goquery.Document) []Make {
	out := make([]Make, 0)
	seen := map[string]bool{}
	doc.Find(`select[name="make"] option, select#make option`).Each(func(_ int, opt *goquery.Selection) {
		if _, disabled := opt.Attr("disabled"); disabled {
			return
		}
		val, _ := opt.Attr("value")
		val = strings.TrimSpace(val)
		name := cliutil.CleanText(opt.Text())
		if val == "" || name == "" || seen[val] {
			return
		}
		seen[val] = true
		out = append(out, Make{Slug: facetSlug(val), Name: name})
	})
	return out
}

// ParseModels reads models from a model-selector fragment. Each model is a
// `button[data-name][data-id]`; the id strips the "vm:" prefix. The enclosing
// `<h5>` section heading (a style/category) is attached for context.
func ParseModels(doc *goquery.Document) []Model {
	out := make([]Model, 0)
	seen := map[string]bool{}
	section := ""
	// Walk the body in document order so each model picks up the most recent
	// preceding <h5> section heading.
	doc.Find("h5, button[data-name]").Each(func(_ int, s *goquery.Selection) {
		if goquery.NodeName(s) == "h5" {
			section = cliutil.CleanText(s.Text())
			return
		}
		name, _ := s.Attr("data-name")
		name = cliutil.CleanText(name)
		id, _ := s.Attr("data-id")
		id = strings.TrimPrefix(strings.TrimSpace(id), "vm:")
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		out = append(out, Model{Slug: id, Name: name, Section: section})
	})
	return out
}

// --- small helpers ---

// specValue returns the value text of a .lp-specs-row by removing the <b>label</b>
// portion and trailing helper prose ("see map", text-muted explainers).
func specValue(row *goquery.Selection) string {
	clone := row.Clone()
	clone.Find("b").Remove()
	clone.Find(".text-muted").Remove() // strip the explainer paragraph under MSRP/ALP
	clone.Find("small").Remove()       // strip "see map" link
	clone.Find("form").Remove()        // strip the VIN-report form
	return cliutil.CleanText(clone.Text())
}

// firstDollarAmount returns the first $-amount text from a column, preferring
// the bold current-price span over any struck-through original.
func firstDollarAmount(col *goquery.Selection) string {
	// The current price is the non-struck bold span; the original is inside <s>.
	if bold := col.Find("span[style*='font-weight']").First(); bold.Length() > 0 {
		if t := firstDollarString(cliutil.CleanText(bold.Text())); t != "" {
			return t
		}
	}
	return firstDollarString(cliutil.CleanText(col.Text()))
}

// firstDollarString returns the first "$N,NNN" token in s, or s trimmed if none.
func firstDollarString(s string) string {
	i := strings.Index(s, "$")
	if i < 0 {
		return strings.TrimSpace(s)
	}
	j := i + 1
	for j < len(s) && (s[j] >= '0' && s[j] <= '9' || s[j] == ',' || s[j] == '.') {
		j++
	}
	return strings.TrimSpace(s[i:j])
}

// normalizeSpace collapses any run of interior whitespace (newlines, tabs,
// repeated spaces left by HTML structure) down to a single space and trims the
// ends. Used for scraped card values whose source markup wraps the text across
// lines.
func normalizeSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// firstToken returns the first whitespace-delimited token (VIN may have a
// trailing report link's text appended).
func firstToken(s string) string {
	for _, f := range strings.Fields(s) {
		return f
	}
	return ""
}

// isCondition reports whether txt is a vehicle condition label.
func isCondition(txt string) bool {
	switch strings.ToLower(strings.TrimSpace(txt)) {
	case "used", "new", "certified", "certified pre-owned", "cpo":
		return true
	}
	return false
}

// idFromDetailPath pulls {id} out of /l/{id}/{slug}.
func idFromDetailPath(href string) string {
	href = strings.TrimPrefix(href, "/")
	parts := strings.Split(href, "/")
	if len(parts) >= 2 && parts[0] == "l" {
		return parts[1]
	}
	return ""
}
