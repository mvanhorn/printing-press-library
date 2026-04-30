package ebay

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// CardSelector matches modern eBay search-result cards as of 2026.
// li.s-card.s-card--horizontal with data-listingid is the live shape.
const cardLiTag = "li"

var (
	// numeric extraction
	priceRe       = regexp.MustCompile(`(?:US\s*)?\$?([0-9][0-9,]*\.?[0-9]*)`)
	bidsRe        = regexp.MustCompile(`(\d+)\s+bids?`)
	soldDateRe    = regexp.MustCompile(`Sold\s+([A-Za-z]+\s+\d+,\s*\d{4})`)
	timeLeftHRe   = regexp.MustCompile(`(\d+)h\s+(\d+)m`)
	timeLeftDayRe = regexp.MustCompile(`(\d+)d\s+(\d+)h`)
	timeLeftMinRe = regexp.MustCompile(`(\d+)m\s+(?:(\d+)s)?`)
	timeLeftSecRe = regexp.MustCompile(`(\d+)s\s+left`)
)

// ParseSearchHTML walks an eBay /sch/i.html response and extracts Listing rows.
// Sponsored "Shop on eBay" placeholder cards are skipped.
func ParseSearchHTML(body []byte) ([]Listing, error) {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	var out []Listing
	walkCards(doc, func(card *html.Node) {
		listing := extractListing(card)
		if listing.ItemID == "" || strings.Contains(listing.Title, "Shop on eBay") {
			return
		}
		out = append(out, listing)
	})
	return out, nil
}

// ParseSoldHTML extracts SoldItem rows from /sch/i.html?LH_Sold=1&LH_Complete=1.
func ParseSoldHTML(body []byte) ([]SoldItem, error) {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	var out []SoldItem
	walkCards(doc, func(card *html.Node) {
		s := extractSold(card)
		if s.ItemID == "" || strings.Contains(s.Title, "Shop on eBay") || s.SoldPrice <= 0 {
			return
		}
		out = append(out, s)
	})
	return out, nil
}

// walkCards traverses the parsed document and yields each top-level
// li.s-card.s-card--horizontal node to fn.
func walkCards(n *html.Node, fn func(*html.Node)) {
	if n == nil {
		return
	}
	if n.Type == html.ElementNode && n.Data == cardLiTag && hasClass(n, "s-card") {
		fn(n)
		return // do not recurse inside a card
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walkCards(c, fn)
	}
}

func extractListing(card *html.Node) Listing {
	l := Listing{
		ItemID:   attr(card, "data-listingid"),
		URL:      cardLink(card),
		Title:    cardTitleText(card),
		Currency: "USD",
	}
	// Whole-card text gives us bid count, time, condition keywords.
	full := visibleText(card)
	l.Price = extractPrice(card)
	if m := bidsRe.FindStringSubmatch(full); len(m) == 2 {
		n, _ := strconv.Atoi(m[1])
		l.Bids = n
		l.Auction = n >= 0 && containsAny(full, "bids", "bid ·")
	}
	if strings.Contains(strings.ToLower(full), "buy it now") {
		l.BIN = true
	}
	if strings.Contains(strings.ToLower(full), "or best offer") {
		l.BestOffer = true
	}
	l.TimeLeft = extractTimeLeft(full)
	if l.TimeLeft != "" {
		l.Auction = true
		l.EndsAt = time.Now().Add(parseTimeLeftDuration(l.TimeLeft))
	}
	l.ImageURL = cardImage(card)
	if pre := strings.ToLower(full); strings.Contains(pre, "pre-owned") || strings.Contains(pre, "used") {
		l.Condition = "used"
	} else if strings.Contains(strings.ToLower(full), "new (other)") {
		l.Condition = "new-other"
	} else if strings.Contains(strings.ToLower(full), "brand new") || strings.Contains(strings.ToLower(full), "new ") {
		l.Condition = "new"
	}
	return l
}

func extractSold(card *html.Node) SoldItem {
	s := SoldItem{
		ItemID:   attr(card, "data-listingid"),
		URL:      cardLink(card),
		Title:    cardTitleText(card),
		Currency: "USD",
	}
	full := visibleText(card)
	s.SoldPrice = extractPrice(card)
	if m := soldDateRe.FindStringSubmatch(full); len(m) == 2 {
		t, err := time.Parse("Jan 2, 2006", strings.ReplaceAll(m[1], "  ", " "))
		if err == nil {
			s.SoldDate = t
		}
	}
	if strings.Contains(strings.ToLower(full), "best offer accepted") || strings.Contains(strings.ToLower(full), "or best offer") {
		s.BestOffer = true
	}
	s.ImageURL = cardImage(card)
	if pre := strings.ToLower(full); strings.Contains(pre, "pre-owned") || strings.Contains(pre, "used") {
		s.Condition = "used"
	} else if strings.Contains(strings.ToLower(full), "new (other)") {
		s.Condition = "new-other"
	} else if strings.Contains(strings.ToLower(full), "brand new") {
		s.Condition = "new"
	}
	return s
}

func extractPrice(card *html.Node) float64 {
	// Find first node with class containing "price" and a $ value.
	var best float64
	walkAll(card, func(n *html.Node) {
		if n.Type == html.ElementNode && hasClassPart(n, "price") {
			t := visibleText(n)
			if m := priceRe.FindStringSubmatch(t); len(m) == 2 {
				v := strings.ReplaceAll(m[1], ",", "")
				if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 && best == 0 {
					best = f
				}
			}
		}
	})
	return best
}

func extractTimeLeft(s string) string {
	for _, re := range []*regexp.Regexp{timeLeftDayRe, timeLeftHRe, timeLeftMinRe, timeLeftSecRe} {
		if m := re.FindString(s); m != "" {
			return strings.TrimSpace(m)
		}
	}
	return ""
}

func parseTimeLeftDuration(s string) time.Duration {
	if m := timeLeftDayRe.FindStringSubmatch(s); len(m) == 3 {
		d, _ := strconv.Atoi(m[1])
		h, _ := strconv.Atoi(m[2])
		return time.Duration(d)*24*time.Hour + time.Duration(h)*time.Hour
	}
	if m := timeLeftHRe.FindStringSubmatch(s); len(m) == 3 {
		h, _ := strconv.Atoi(m[1])
		mi, _ := strconv.Atoi(m[2])
		return time.Duration(h)*time.Hour + time.Duration(mi)*time.Minute
	}
	if m := timeLeftMinRe.FindStringSubmatch(s); len(m) >= 2 {
		mi, _ := strconv.Atoi(m[1])
		return time.Duration(mi) * time.Minute
	}
	if m := timeLeftSecRe.FindStringSubmatch(s); len(m) == 2 {
		ss, _ := strconv.Atoi(m[1])
		return time.Duration(ss) * time.Second
	}
	return 0
}

func cardLink(n *html.Node) string {
	var href string
	walkAll(n, func(x *html.Node) {
		if href != "" {
			return
		}
		if x.Type == html.ElementNode && x.Data == "a" && hasClass(x, "s-card__link") {
			href = strings.SplitN(attr(x, "href"), "?", 2)[0]
		}
	})
	return href
}

func cardTitleText(n *html.Node) string {
	var t string
	walkAll(n, func(x *html.Node) {
		if t != "" {
			return
		}
		if x.Type == html.ElementNode && hasClassPart(x, "title") && !hasClassPart(x, "subtitle") {
			t = strings.TrimSpace(strings.ReplaceAll(visibleText(x), "\n", " "))
			if strings.HasPrefix(t, "NEW LISTING") {
				t = strings.TrimSpace(strings.TrimPrefix(t, "NEW LISTING"))
			}
			if i := strings.Index(t, "Opens in a new window"); i > 0 {
				t = strings.TrimSpace(t[:i])
			}
		}
	})
	return t
}

func cardImage(n *html.Node) string {
	var src string
	walkAll(n, func(x *html.Node) {
		if src != "" {
			return
		}
		if x.Type == html.ElementNode && x.Data == "img" {
			if s := attr(x, "src"); s != "" {
				src = s
				return
			}
			if s := attr(x, "data-src"); s != "" {
				src = s
			}
		}
	})
	return src
}

func walkAll(n *html.Node, fn func(*html.Node)) {
	if n == nil {
		return
	}
	fn(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walkAll(c, fn)
	}
}

func attr(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}

func hasClass(n *html.Node, cls string) bool {
	for _, c := range strings.Fields(attr(n, "class")) {
		if c == cls {
			return true
		}
	}
	return false
}

func hasClassPart(n *html.Node, part string) bool {
	for _, c := range strings.Fields(attr(n, "class")) {
		if strings.Contains(strings.ToLower(c), part) {
			return true
		}
	}
	return false
}

// visibleText collects text nodes under n, joining with single spaces and
// preserving paragraph breaks as newlines.
func visibleText(n *html.Node) string {
	var sb strings.Builder
	var rec func(*html.Node)
	rec = func(x *html.Node) {
		if x == nil {
			return
		}
		if x.Type == html.ElementNode && (x.Data == "script" || x.Data == "style") {
			return
		}
		if x.Type == html.TextNode {
			sb.WriteString(x.Data)
			sb.WriteRune(' ')
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			rec(c)
		}
		if x.Type == html.ElementNode && (x.Data == "p" || x.Data == "div" || x.Data == "li" || x.Data == "br") {
			sb.WriteRune('\n')
		}
	}
	rec(n)
	return strings.TrimSpace(sb.String())
}

func containsAny(s string, subs ...string) bool {
	low := strings.ToLower(s)
	for _, x := range subs {
		if strings.Contains(low, strings.ToLower(x)) {
			return true
		}
	}
	return false
}

// ExtractBidTokens scrapes the embedded srt and forterToken values from an
// item-detail or /bfl/placebid module HTML response. eBay puts the tokens in
// hidden form inputs and JS bootstrap blobs; this picks up either form.
func ExtractBidTokens(body []byte) (srt, forterToken string) {
	s := string(body)
	if m := regexp.MustCompile(`name="srt"[^>]*value="([^"]+)"`).FindStringSubmatch(s); len(m) == 2 {
		srt = m[1]
	} else if m := regexp.MustCompile(`"srt"\s*:\s*"([^"]+)"`).FindStringSubmatch(s); len(m) == 2 {
		srt = m[1]
	}
	if m := regexp.MustCompile(`"forterToken"\s*:\s*"([^"]+)"`).FindStringSubmatch(s); len(m) == 2 {
		forterToken = m[1]
	} else if m := regexp.MustCompile(`forter[_-]?token['"]?\s*[:=]\s*['"]([^'"]+)`).FindStringSubmatch(s); len(m) == 2 {
		forterToken = m[1]
	}
	return
}
