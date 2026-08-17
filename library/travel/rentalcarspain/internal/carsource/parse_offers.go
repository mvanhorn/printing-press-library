// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package carsource

import (
	"regexp"
	"strings"

	xhtml "golang.org/x/net/html"
)

// excessRe extracts the numeric excess from a coverage tooltip such as
// "...Collision Damage Waiver and Theft Protection with an excess of
// 1200.00 €...". The bounded, non-greedy gap skips any inline <span> markup
// the title attribute carries without matching digits inside it.
var excessRe = regexp.MustCompile(`(?is)excess of.{0,90}?(\d{1,4}[.,]\d{2})\s*€`)

// parseOffers extracts every offer from a DoYouSpain results document. Each
// offer is an <article> carrying a data-prv supplier code.
func parseOffers(doc *xhtml.Node) []Offer {
	articles := findAll(doc, func(n *xhtml.Node) bool {
		return n.Data == "article" && attr(n, "data-prv") != ""
	})
	out := make([]Offer, 0, len(articles))
	for _, art := range articles {
		if o, ok := parseOfferArticle(art); ok {
			out = append(out, o)
		}
	}
	return out
}

// doyouspainSupplierName pulls the supplier name from the logo img alt inside
// the offer's .cl--car-rent-logo element, falling back to the data-prv code map,
// and returns it in canonical form so it matches the other sources.
func doyouspainSupplierName(art *xhtml.Node, code string) string {
	raw := ""
	if logo := firstWithClass(art, "cl--car-rent-logo"); logo != nil {
		for _, img := range findAll(logo, func(n *xhtml.Node) bool { return n.Data == "img" }) {
			if alt := collapseWS(attr(img, "alt")); alt != "" && !strings.Contains(alt, "|") {
				raw = alt
				break
			}
		}
	}
	if raw == "" {
		// Fallback: any img whose alt is a short supplier-ish name.
		for _, img := range findAll(art, func(n *xhtml.Node) bool { return n.Data == "img" }) {
			alt := collapseWS(attr(img, "alt"))
			if alt != "" && !strings.Contains(alt, "|") && len(alt) < 30 {
				raw = alt
				break
			}
		}
	}
	if raw == "" {
		raw = SupplierName(code)
	}
	return CanonicalSupplier(raw)
}

func parseOfferArticle(art *xhtml.Node) (Offer, bool) {
	o := Offer{Source: "doyouspain", Currency: "EUR"}
	o.URL = DoYouSpainBaseURL
	o.SupplierCode = attr(art, "data-prv")
	// Prefer the real supplier name from the logo alt text (e.g. "ALAMO",
	// "KEDDY by Europcar") over the internal data-prv code map, which is
	// incomplete; fall back to the code map, then the raw code.
	o.Supplier = doyouspainSupplierName(art, o.SupplierCode)

	// Car name from the .cl--name <h2 title="...">, class from .cl--name-type.
	if h2 := findAll(art, func(n *xhtml.Node) bool { return n.Data == "h2" }); len(h2) > 0 {
		if t := attr(h2[0], "title"); t != "" {
			o.Car = collapseWS(t)
		} else {
			o.Car = textOf(h2[0])
		}
	}
	if nt := firstWithClass(art, "cl--name-type"); nt != nil {
		o.CarClass = textOf(nt)
	}

	// Features: <ul class="features"><li value="4">4 seats</li>...
	if feats := firstWithClass(art, "features"); feats != nil {
		for _, li := range findAll(feats, func(n *xhtml.Node) bool { return n.Data == "li" }) {
			val := attr(li, "value")
			txt := strings.ToLower(textOf(li))
			switch {
			case strings.Contains(txt, "seat"):
				o.Seats = parseInt(val + " " + txt)
			case strings.Contains(txt, "door"):
				o.Doors = parseInt(val + " " + txt)
			case val == "M" || strings.Contains(txt, "manual"):
				o.Transmission = "Manual"
			case val == "A" || strings.Contains(txt, "automat"):
				o.Transmission = "Automatic"
			}
		}
	}

	// Services: fuel policy, mileage.
	if serv := firstWithClass(art, "cl--info-serv"); serv != nil {
		for _, li := range findAll(serv, func(n *xhtml.Node) bool { return n.Data == "li" }) {
			txt := textOf(li)
			low := strings.ToLower(txt)
			switch {
			case strings.Contains(low, "mileage"):
				o.Mileage = collapseWS(txt)
			case strings.Contains(low, "fuel policy"):
				o.FuelPolicy = collapseWS(strings.TrimPrefix(txt, "Fuel Policy:"))
			}
		}
	}

	// Free cancellation appears in the "Includes" list.
	for _, li := range findAll(art, func(n *xhtml.Node) bool { return n.Data == "li" }) {
		if strings.Contains(strings.ToLower(textOf(li)), "free cancellation") {
			o.FreeCancel = true
			break
		}
	}

	// Excess / deductible. The coverage tooltip reads e.g. "...Collision
	// Damage Waiver and Theft Protection with an excess of 1200.00 €...".
	// A "no excess" / "zero excess" phrasing means the offer is fully insured.
	// Scan both element text and title attributes across the offer.
	for _, el := range findAll(art, func(n *xhtml.Node) bool { return n.Type == xhtml.ElementNode }) {
		hay := textOf(el) + " " + attr(el, "title")
		low := strings.ToLower(hay)
		if !o.ExcessKnown && (strings.Contains(low, "no excess") || strings.Contains(low, "zero excess") || strings.Contains(low, "no deductible")) {
			o.Excess = 0
			o.ExcessKnown = true
			o.FullInsurance = true
		}
		if m := excessRe.FindStringSubmatch(hay); m != nil {
			if v := parsePrice(m[1]); v > 0 {
				o.Excess = v
				o.ExcessKnown = true
				o.FullInsurance = false
			}
		}
	}

	// Prices: euro fields. .pr-euros = current total, .old-price-euros =
	// struck-through base, .price-day-euros = per day.
	for _, el := range findAll(art, func(n *xhtml.Node) bool { return n.Type == xhtml.ElementNode }) {
		switch {
		case hasClass(el, "old-price-euros"):
			if v := parsePrice(textOf(el)); v > 0 {
				o.BaseTotal = v
			}
		case hasClass(el, "pr-euros"):
			if v := parsePrice(textOf(el)); v > 0 && o.Total == 0 {
				o.Total = v
			}
		case hasClass(el, "price-day-euros"):
			if v := parsePrice(textOf(el)); v > 0 && o.PerDay == 0 {
				o.PerDay = v
			}
		case hasClass(el, "score-total"):
			o.SupplierScore = parsePrice(textOf(el))
		case hasClass(el, "reviews-total"):
			o.Reviews = parseInt(textOf(el))
		}
	}

	return o, o.Total > 0 || o.PerDay > 0
}
