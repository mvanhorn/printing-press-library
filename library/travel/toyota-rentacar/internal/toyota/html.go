package toyota

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/toyota-rentacar/internal/cliutil"
	"golang.org/x/net/html"
)

func parseHTML(data []byte) (*html.Node, error) {
	n, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return nil, &SourceError{"cannot parse Toyota HTML: " + err.Error()}
	}
	return n, nil
}

func attr(n *html.Node, key string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
func hasAttr(n *html.Node, key string) bool {
	if n == nil {
		return false
	}
	for _, a := range n.Attr {
		if a.Key == key {
			return true
		}
	}
	return false
}
func nodes(n *html.Node, pred func(*html.Node) bool) []*html.Node {
	out := make([]*html.Node, 0)
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if x == nil {
			return
		}
		if pred(x) {
			out = append(out, x)
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return out
}
func byID(n *html.Node, id string) *html.Node {
	all := nodes(n, func(x *html.Node) bool { return attr(x, "id") == id })
	if len(all) > 0 {
		return all[0]
	}
	return nil
}
func ids(n *html.Node, id string) []*html.Node {
	return nodes(n, func(x *html.Node) bool { return attr(x, "id") == id })
}
func byClass(n *html.Node, class string) *html.Node {
	all := nodes(n, func(x *html.Node) bool { return strings.Contains(" "+attr(x, "class")+" ", " "+class+" ") })
	if len(all) > 0 {
		return all[0]
	}
	return nil
}
func text(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if x == nil || x.Type == html.CommentNode || x.Data == "script" || x.Data == "style" {
			return
		}
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
			b.WriteByte(' ')
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(cliutil.CleanText(b.String())), " ")
}
func valueID(n *html.Node, id string) string { return attr(byID(n, id), "value") }
func textID(n *html.Node, id string) string  { return text(byID(n, id)) }

// Embedded JSON remains data when wrapped in a script node. Human text
// intentionally omits scripts, so it cannot read this source contract.
func rawText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if x == nil {
			return
		}
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
		}
		for child := x.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(n)
	return strings.TrimSpace(b.String())
}

func integer(s string) *int {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	if s == "" {
		return nil
	}
	v, e := strconv.Atoi(s)
	if e != nil || v < 0 {
		return nil
	}
	return &v
}
func firstInteger(s string) *int {
	m := regexp.MustCompile(`[0-9]+`).FindString(s)
	return integer(m)
}

// successfulForm reproduces HTML successful controls, with state kept in memory.
func successfulForm(doc *html.Node) url.Values {
	f := url.Values{}
	form := byID(doc, "PageForm")
	if form == nil {
		return f
	}
	for _, n := range nodes(form, func(n *html.Node) bool {
		return n.Type == html.ElementNode && (n.Data == "input" || n.Data == "select" || n.Data == "textarea")
	}) {
		name := attr(n, "name")
		if name == "" || hasAttr(n, "disabled") {
			continue
		}
		if n.Data == "input" {
			kind := strings.ToLower(attr(n, "type"))
			switch kind {
			case "button", "submit", "image", "reset", "file":
				continue
			}
			if (kind == "radio" || kind == "checkbox") && !hasAttr(n, "checked") {
				continue
			}
			v := attr(n, "value")
			if (kind == "radio" || kind == "checkbox") && !hasAttr(n, "value") {
				v = "on"
			}
			f.Add(name, v)
		} else if n.Data == "textarea" {
			f.Add(name, text(n))
		} else {
			opts := nodes(n, func(x *html.Node) bool { return x.Data == "option" && !hasAttr(x, "disabled") })
			var selected []*html.Node
			for _, o := range opts {
				if hasAttr(o, "selected") {
					selected = append(selected, o)
				}
			}
			if len(selected) == 0 && len(opts) > 0 {
				selected = opts[:1]
			}
			for _, o := range selected {
				v := attr(o, "value")
				if !hasAttr(o, "value") {
					v = text(o)
				}
				f.Add(name, v)
			}
		}
	}
	return f
}
func setID(f url.Values, doc *html.Node, id, v string) error {
	n := byID(doc, id)
	name := attr(n, "name")
	if name == "" {
		return &SourceError{"Toyota form changed: missing " + id + "; use the canonical booking page"}
	}
	f.Set(name, v)
	return nil
}
func postFields(doc *html.Node, target string) url.Values {
	f := successfulForm(doc)
	f.Set("__EVENTTARGET", target)
	f.Set("__EVENTARGUMENT", "")
	return f
}
func sourceProblem(doc *html.Node, finalURL string) error {
	if strings.Contains(finalURL, "/error_") {
		return &SourceError{"Toyota returned its " + strings.TrimSuffix(strings.TrimPrefix(strings.Split(finalURL, "/eng/")[len(strings.Split(finalURL, "/eng/"))-1], "error_"), ".aspx") + " page; retry or use " + Origin + BookingPath}
	}
	if msg := textID(doc, "lblErrMsg"); msg != "" {
		return &SourceError{"Toyota: " + msg + "; verify shops/dates/options on " + Origin + BookingPath}
	}
	if byID(doc, "PageForm") == nil && strings.Contains(finalURL, "/reservation/") {
		return &SourceError{"Toyota search returned an unexpected document; use " + Origin + BookingPath}
	}
	return nil
}

func ParseShops(data []byte) ([]Shop, string, error) {
	doc, err := parseHTML(data)
	if err != nil {
		return nil, "", err
	}
	context := ""
	for _, n := range nodes(doc, func(x *html.Node) bool { return x.Data == "h1" }) {
		if strings.Contains(text(n), "Around") {
			context = text(n)
			break
		}
	}
	if msg := textID(doc, "lblErrMsg"); msg != "" {
		return nil, context, &SourceError{"Toyota: " + msg}
	}
	shops := make([]Shop, 0)
	for _, card := range ids(doc, "divBoxShop") {
		id := valueID(card, "hdnRCode") + ":" + valueID(card, "hdnECode")
		if _, _, err := ParseShopID(id); err != nil {
			return nil, context, &SourceError{"Toyota shop card has no valid stable identity"}
		}
		u, _ := ShopURL(id, false)
		shop := Shop{ID: id, Name: text(byClass(card, "box_shop__title__eng")), NameJP: text(byClass(card, "box_shop__title__jp")), Hours: []string{}, Services: []string{}, URL: u, OneWayReturns: "unknown"}
		if shop.Name == "" || shop.NameJP == "" {
			return nil, context, &SourceError{"Toyota shop card names are missing"}
		}
		dls := nodes(card, func(n *html.Node) bool { return n.Data == "dl" })
		if len(dls) > 0 {
			dds := nodes(dls[0], func(n *html.Node) bool { return n.Data == "dd" })
			if len(dds) > 0 {
				for _, n := range nodes(dds[0], func(n *html.Node) bool { return n.Data == "p" }) {
					s := text(n)
					if strings.Contains(s, "Closed") {
						shop.Closure = s
					} else if s != "" {
						shop.Hours = append(shop.Hours, s)
					}
				}
				if len(shop.Hours) == 0 {
					s := text(dds[0])
					parts := regexp.MustCompile(`(?:[0-9]{2}:[0-9]{2}-[0-9]{2}:[0-9]{2}\([^)]*\)|Closed\s*:[^;]+)`).FindAllString(s, -1)
					for _, p := range parts {
						if strings.HasPrefix(p, "Closed") {
							shop.Closure = p
						} else {
							shop.Hours = append(shop.Hours, p)
						}
					}
				}
			}
			if len(dds) > 1 {
				shop.Phone = text(dds[1])
			}
			if len(dds) > 2 {
				shop.MapCode = strings.TrimPrefix(text(dds[2]), "Map code: ")
			}
		}
		if len(dls) > 1 {
			shop.Address = text(dls[1])
			shop.Address = strings.TrimPrefix(shop.Address, " ")
		}
		for _, n := range nodes(card, func(n *html.Node) bool { return n.Data == "a" }) {
			s := attr(n, "title")
			if s == "" {
				s = attr(n, "aria-label")
			}
			if s == "" {
				for _, im := range nodes(n, func(x *html.Node) bool { return x.Data == "img" }) {
					s = attr(im, "alt")
					if s != "" {
						break
					}
				}
			}
			if s == "Oneway" || s == "ETC" || s == "Accessibility" || s == "Transportation Service" {
				shop.Services = append(shop.Services, s)
			}
		}
		for _, n := range nodes(card, func(n *html.Node) bool { return n.Data == "li" || n.Data == "p" }) {
			s := text(n)
			if strings.HasPrefix(s, "*One-way:") {
				shop.OneWayNote = s
				break
			}
		}
		switch {
		case strings.Contains(shop.OneWayNote, "returns not possible"):
			shop.OneWayReturns = "not_possible"
		case strings.Contains(shop.OneWayNote, "only inside"):
			shop.OneWayReturns = "inside_prefecture_only"
		case strings.Contains(shop.OneWayNote, "inside / outside"):
			shop.OneWayReturns = "inside_and_outside_prefecture"
		}
		notices := nodes(card, func(n *html.Node) bool {
			return strings.Contains(" "+attr(n, "class")+" ", " box_notice ") && !strings.Contains(attr(n, "style"), "display:none")
		})
		for _, n := range notices {
			if s := text(n); s != "" {
				if shop.Notice != "" {
					shop.Notice += "; "
				}
				shop.Notice += s
			}
		}
		shops = append(shops, shop)
	}
	return shops, context, nil
}

func ParseOffers(data []byte, modelLimit int) ([]ClassOffer, error) {
	doc, err := parseHTML(data)
	if err != nil {
		return nil, err
	}
	offers := make([]ClassOffer, 0)
	for _, card := range ids(doc, "divClassSelect") {
		class := strings.TrimSuffix(textID(card, "lblCarClassCd"), " Class")
		if class == "" {
			return nil, &SourceError{"Toyota class card has no class code"}
		}
		button := byID(card, "lnkSelect")
		availability := "unknown"
		switch strings.TrimSpace(text(button)) {
		case "Select":
			if !strings.Contains(attr(button, "class"), "is_disabled") {
				availability = "available"
			}
		case "Fully booked":
			availability = "fully_booked"
		}
		price := integer(textID(card, "lblEstimatePrice"))
		if price == nil {
			return nil, &SourceError{"Toyota class price is missing or not numeric for " + class}
		}
		offer := ClassOffer{Class: class, Availability: availability, SourceEstimateJPY: price, PriceLabel: "Rental Price", TaxIncluded: true, Capacity: firstInteger(textID(card, "lblPassengers")), RepresentativeModels: []string{}, ModelSelectionFee: textID(card, "lblOptionFee")}
		seen := map[string]bool{}
		for _, n := range ids(card, "lblCarName") {
			m := text(n)
			if m != "" && !seen[m] {
				seen[m] = true
				if len(offer.RepresentativeModels) < modelLimit {
					offer.RepresentativeModels = append(offer.RepresentativeModels, m)
				}
			}
		}
		offers = append(offers, offer)
	}
	if len(offers) == 0 {
		return nil, &SourceError{"Toyota returned no recognizable class cards; no availability conclusion can be drawn"}
	}
	return offers, nil
}

func checkContext(doc *html.Node, period Period, pickup, dropoff Shop) error {
	if valueID(doc, "hdDepDate") != dateWire(period.Pickup) || valueID(doc, "hdRetDate") != dateWire(period.Dropoff) {
		return &SourceError{"Toyota changed the requested dates; no quotes returned. Re-enter dates at " + pickup.URL}
	}
	r, e, _ := ParseShopID(pickup.ID)
	rr, ee, _ := ParseShopID(dropoff.ID)
	if valueID(doc, "txtHdnDepRCode") != r || valueID(doc, "txtHdnDepECode") != e || valueID(doc, "txtHdnRetRCode") != rr || valueID(doc, "txtHdnRetECode") != ee {
		return &SourceError{"Toyota changed the requested shop pair; no quotes returned. Re-enter shops at " + pickup.URL}
	}
	return nil
}

func requireToken(text, token string) error {
	if !strings.Contains(text, token) {
		return &SourceError{fmt.Sprintf("Toyota policy layout changed: missing %q; open the source page for current guidance", token)}
	}
	return nil
}

type OperatingWindow struct {
	Date   string `json:"date_jst"`
	Opens  string `json:"opens_jst"`
	Closes string `json:"closes_jst"`
}

// The published calendar gives shop operating windows, not vehicle stock slots.
func checkOperatingWindow(doc *html.Node, id, flag string, at time.Time) (OperatingWindow, error) {
	var calendar struct {
		Open map[string][]string `json:"open"`
	}
	if err := json.Unmarshal([]byte(rawText(byID(doc, id))), &calendar); err != nil || len(calendar.Open) == 0 {
		return OperatingWindow{}, &SourceError{"Toyota's " + flag + " operating calendar is missing or changed; no pickup/return time promise can be made"}
	}
	window, ok := calendar.Open[at.In(jst).Format("20060102")]
	if !ok || len(window) != 2 {
		return OperatingWindow{}, &SourceError{"Toyota publishes no operating window for " + flag + " on " + at.In(jst).Format("2006-01-02") + "; the shop may be closed. No quote issued."}
	}
	clockRE := regexp.MustCompile(`^(?:[01][0-9]|2[0-3])[0-5][0-9]$`)
	if !clockRE.MatchString(window[0]) || !clockRE.MatchString(window[1]) || window[1] < window[0] {
		return OperatingWindow{}, &SourceError{"Toyota's operating window format changed; verify the requested time on the source page"}
	}
	hhmm := at.In(jst).Format("1504")
	if hhmm < window[0] || hhmm > window[1] {
		return OperatingWindow{}, &InputError{flag + " is outside Toyota's published shop hours " + window[0][:2] + ":" + window[0][2:] + "–" + window[1][:2] + ":" + window[1][2:] + " JST"}
	}
	return OperatingWindow{Date: at.In(jst).Format("2006-01-02"), Opens: window[0][:2] + ":" + window[0][2:], Closes: window[1][:2] + ":" + window[1][2:]}, nil
}
