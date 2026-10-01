package onsen

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

var idRE = regexp.MustCompile(`^onsen[0-9]{6}$`)
var pathIDRE = regexp.MustCompile(`/((?:onsen)[0-9]{6})/`)
var numberRE = regexp.MustCompile(`[0-9]+(?:\.[0-9]+)?`)
var priceRE = regexp.MustCompile(`([0-9][0-9,]*)円`)
var dateRE = regexp.MustCompile(`([0-9]{4})年\s*([0-9]{1,2})月\s*([0-9]{1,2})日`)

// FacilityID accepts source IDs or canonical source facility URLs, never arbitrary URLs.
func FacilityID(input string) (string, error) {
	if idRE.MatchString(input) {
		return input, nil
	}
	u, err := url.Parse(input)
	if err == nil && u.Scheme == "https" && u.Host == "onsen.nifty.com" && u.User == nil && u.RawQuery == "" && u.Fragment == "" {
		m := pathIDRE.FindStringSubmatch(u.Path)
		if len(m) == 2 && strings.HasSuffix(u.Path, "/"+m[1]+"/") {
			return m[1], nil
		}
	}
	return "", fmt.Errorf("invalid facility %q: use onsen012278 or its https://onsen.nifty.com/<area>/onsen012278/ URL", input)
}
func attr(n *html.Node, k string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.Attr {
		if a.Key == k {
			return a.Val
		}
	}
	return ""
}
func has(n *html.Node, c string) bool {
	for _, v := range strings.Fields(attr(n, "class")) {
		if v == c {
			return true
		}
	}
	return false
}
func walk(n *html.Node, f func(*html.Node)) {
	if n == nil {
		return
	}
	f(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, f)
	}
}
func first(n *html.Node, f func(*html.Node) bool) *html.Node {
	var out *html.Node
	walk(n, func(v *html.Node) {
		if out == nil && f(v) {
			out = v
		}
	})
	return out
}
func class(n *html.Node, c string) *html.Node {
	return first(n, func(v *html.Node) bool { return has(v, c) })
}
func text(n *html.Node) string {
	var b strings.Builder
	var add func(*html.Node)
	add = func(v *html.Node) {
		if v == nil {
			return
		}
		if v.Type == html.ElementNode && (v.Data == "script" || v.Data == "style" || v.Data == "noscript" || v.Data == "svg" || v.Data == "input") {
			return
		}
		if v.Type == html.TextNode {
			b.WriteString(v.Data)
		}
		if v.Data == "br" {
			b.WriteByte('\n')
		}
		for c := v.FirstChild; c != nil; c = c.NextSibling {
			add(c)
		}
		switch v.Data {
		case "p", "li", "div", "h3", "h4", "tr":
			b.WriteByte('\n')
		}
	}
	add(n)
	lines := []string{}
	for _, line := range strings.Split(b.String(), "\n") {
		line = strings.Join(strings.Fields(line), " ")
		if line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}
func scalar(s string) *float64 {
	v, e := strconv.ParseFloat(numberRE.FindString(s), 64)
	if e != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return nil
	}
	return &v
}
func integer(s string) *int {
	s = strings.ReplaceAll(s, ",", "")
	v, e := strconv.Atoi(numberRE.FindString(s))
	if e != nil {
		return nil
	}
	return &v
}
func minimum(s string) *int {
	m := priceRE.FindStringSubmatch(s)
	if len(m) != 2 {
		return nil
	}
	return integer(m[1])
}
func canonical(doc *html.Node, fallback string) string {
	n := first(doc, func(v *html.Node) bool { return v.Data == "link" && attr(v, "rel") == "canonical" })
	if n != nil {
		u, e := url.Parse(attr(n, "href"))
		if e == nil && u.Scheme == "https" && u.Host == "onsen.nifty.com" {
			return u.String()
		}
	}
	return fallback
}
func doc(b []byte) (*html.Node, error) { return html.Parse(strings.NewReader(string(b))) }
func absolute(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	u, e := url.Parse(s)
	if e != nil {
		return ""
	}
	base, _ := url.Parse(Origin)
	u = base.ResolveReference(u)
	if u.Scheme != "https" || u.Host != "onsen.nifty.com" || u.User != nil {
		return ""
	}
	return u.String()
}

// ParseSearch reads organic cards only. It never fetches individual details.
func ParseSearch(b []byte, sourceURL string, page int) (SearchData, error) {
	d, e := doc(b)
	if e != nil {
		return SearchData{}, e
	}
	out := SearchData{Items: []Facility{}, Page: page}
	seen := map[string]bool{}
	walk(d, func(n *html.Node) {
		if n.Data != "li" || !has(n, "shop") || has(n, "pr") {
			return
		}
		id := attr(n, "data-onsen-id")
		a := first(n, func(v *html.Node) bool { return v.Data == "a" && pathIDRE.MatchString(attr(v, "href")) })
		if a == nil {
			return
		}
		u := absolute(attr(a, "href"))
		if id == "" {
			if m := pathIDRE.FindStringSubmatch(u); len(m) == 2 {
				id = m[1]
			}
		}
		if !idRE.MatchString(id) || seen[id] {
			return
		}
		name := text(class(n, "name"))
		if name == "" {
			return
		}
		seen[id] = true
		f := Facility{ID: id, Name: name, URL: u, Area: strptr(text(class(n, "accessMain"))), Latitude: scalar(attr(n, "data-latitude")), Longitude: scalar(attr(n, "data-longitude")), Hours: strptr(text(class(n, "time"))), Admission: strptr(text(class(n, "price"))), PriceBasis: "source minimum hint; fee basis, extras and eligibility unknown", SourceLabels: []string{}}
		if f.Admission != nil {
			f.MinPriceJPY = minimum(*f.Admission)
		}
		point := class(n, "point")
		f.Rating = scalar(text(first(point, func(v *html.Node) bool { return v.Data == "strong" })))
		parts := strings.Split(text(point), "/")
		if len(parts) > 1 {
			f.ReviewCount = integer(parts[len(parts)-1])
		}
		if v := attr(n, "data-has-coupon"); v == "0" || v == "1" {
			f.CouponAvailable = ptr(v == "1")
		}
		walk(class(n, "types"), func(v *html.Node) {
			if v.Data == "li" && has(v, "badgeOn") {
				label := text(v)
				f.SourceLabels = append(f.SourceLabels, label)
				if has(v, "oneday") {
					f.DayUse = ptr(true)
				}
				if has(v, "stay") {
					f.Stay = ptr(true)
				}
			}
		})
		out.Items = append(out.Items, f)
	})
	if n := class(class(d, "newpagerDisp"), "max"); n != nil {
		out.Total = integer(text(n))
	}
	walk(d, func(n *html.Node) {
		if n.Data == "a" && strings.Contains(attr(n, "href"), fmt.Sprintf("/page-%d/", page+1)) {
			u := absolute(attr(n, "href"))
			if u != "" {
				out.NextURL = &u
			}
		}
	})
	if len(out.Items) == 0 {
		all := text(class(d, "listResultWrap"))
		if all == "" {
			all = text(first(d, func(n *html.Node) bool { return attr(n, "id") == "listResultWrap" }))
		}
		if !strings.Contains(all, "該当") && !strings.Contains(all, "見つか") && (out.Total == nil || *out.Total != 0) {
			return out, fmt.Errorf("Nifty search page has no recognized organic cards or explicit zero-result marker; source layout/access may have changed")
		}
	}
	return out, nil
}

func fields(d *html.Node) map[string]*html.Node {
	out := map[string]*html.Node{}
	walk(d, func(n *html.Node) {
		if n.Data != "h4" {
			return
		}
		p := n.Parent
		if p == nil {
			return
		}
		for v := p.NextSibling; v != nil; v = v.NextSibling {
			if v.Type == html.ElementNode {
				if v.Data == "div" && first(v, func(z *html.Node) bool { return z.Data == "h4" }) == nil {
					out[text(n)] = v
				}
				break
			}
		}
	})
	return out
}
func claimFromLabels(labels []string, allowed ...string) Claim {
	c := unknown()
	for _, s := range labels {
		for _, a := range allowed {
			if s == a {
				c.State = "source_claim"
				c.Evidence = append(c.Evidence, s)
			}
		}
	}
	return c
}

// ParseDetail uses facility-local semantic fields, excluding navigation, ads and reviews.
func ParseDetail(b []byte, sourceURL, id string) (Detail, error) {
	d, e := doc(b)
	if e != nil {
		return Detail{}, e
	}
	u := canonical(d, sourceURL)
	got, e := FacilityID(u)
	if e != nil || got != id {
		return Detail{}, fmt.Errorf("facility identity mismatch or removed listing for %s; verify its source URL", id)
	}
	h1 := first(d, func(n *html.Node) bool { return n.Data == "h1" })
	name := text(h1)
	if name == "" {
		return Detail{}, fmt.Errorf("facility %s has no recognized Japanese name; source layout/access may have changed", id)
	}
	fs := fields(d)
	get := func(k string) *string { return strptr(text(fs[k])) }
	out := Detail{Facility: Facility{ID: id, Name: name, URL: u, Hours: get("営業時間"), Admission: get("料金"), PriceBasis: "raw source admission; JPY, conditional fees and extras preserved; no payable quote", SourceLabels: []string{}}, ClosureDays: get("休業日"), Address: get("住所"), Access: get("交通アクセス"), Parking: get("駐車場"), Phone: get("電話"), Facilities: map[string]string{}, BathKind: []string{}, NaturalHotSpring: unknown(), PrivateRentableBath: unknown(), PrivateRoom: unknown(), PrivateBathEvidence: []string{}, Policies: map[string]Claim{"tattoo": unknown(), "children": unknown(), "accessibility": unknown()}}
	if len(fs) < 3 {
		return Detail{}, fmt.Errorf("facility %s missing semantic admission/access fields; source layout/access may have changed", id)
	}
	for _, k := range []string{"特徴", "泉質", "飲食施設", "付帯施設", "設備", "備付品", "温泉の特徴", "利用シーン", "カード利用", "電子決済"} {
		if v := text(fs[k]); v != "" {
			out.Facilities[k] = v
		}
	}
	for _, k := range []string{"特徴", "温泉の特徴"} {
		n := fs[k]
		walk(n, func(v *html.Node) {
			if v.Data == "a" || v.Data == "span" {
				s := text(v)
				if s != "" && !strings.Contains(s, "\n") {
					out.SourceLabels = append(out.SourceLabels, s)
				}
			}
		})
	}
	// Only active facility badges, never disabled badges or global source filters.
	walk(d, func(n *html.Node) {
		if has(n, "badge") && !has(n, "is-disabled") && strings.Contains(attr(n, "data-event-action"), "detailIntro") {
			s := text(n)
			if strings.Contains(attr(n, "data-event-category"), "kashikiriIcon") {
				s = "貸切風呂・個室風呂"
			}
			if s != "" {
				out.SourceLabels = append(out.SourceLabels, s)
			}
		}
	})
	// Header badges establish day-use/stay without consulting global navigation.
	header := h1.Parent
	for header != nil && header.Data != "header" {
		header = header.Parent
	}
	walk(header, func(n *html.Node) {
		if has(n, "badge") {
			s := text(n)
			switch s {
			case "日帰り", "宿泊", "クーポンあり", "銭湯", "スーパー銭湯":
				out.SourceLabels = append(out.SourceLabels, s)
			}
		}
	})
	if header != nil {
		walk(header, func(n *html.Node) {
			s := text(n)
			if n.Data == "div" && has(n, "is-xs") && (strings.Contains(s, "県 /") || strings.Contains(s, "都 /") || strings.Contains(s, "府 /") || strings.Contains(s, "北海道 /")) {
				out.Area = strptr(s)
			}
		})
	}
	for _, s := range out.SourceLabels {
		if s == "クーポンあり" {
			out.CouponAvailable = ptr(true)
		}
	}
	out.SourceLabels = unique(out.SourceLabels)
	out.NaturalHotSpring = claimFromLabels(out.SourceLabels, "天然温泉")
	out.PrivateRentableBath = claimFromLabels(out.SourceLabels, "日帰り貸切風呂", "貸切露天風呂", "貸切風呂")
	out.PrivateRoom = claimFromLabels(out.SourceLabels, "個室", "貸切個室", "個室休憩")
	for _, s := range out.SourceLabels {
		switch s {
		case "日帰り", "日帰り温泉":
			out.DayUse = ptr(true)
		case "宿泊":
			out.Stay = ptr(true)
		case "銭湯", "スーパー銭湯", "健康ランド", "サウナ":
			out.BathKind = append(out.BathKind, s)
		}
		if strings.Contains(s, "貸切") || strings.Contains(s, "個室") || strings.Contains(s, "家族風呂") {
			out.PrivateBathEvidence = append(out.PrivateBathEvidence, s)
		}
	}
	// Preserve explicit policy wording as evidence without generalizing permission.
	for _, k := range []string{"料金", "特徴", "付帯施設", "設備", "利用シーン"} {
		for _, line := range strings.Split(text(fs[k]), "\n") {
			for policy, terms := range map[string][]string{"tattoo": {"タトゥー", "刺青", "入れ墨", "いれずみ"}, "children": {"お子様", "子ども", "子供", "オムツ", "乳児", "歳以下", "小学生"}, "accessibility": {"バリアフリー", "車椅子", "車いす", "障害者用"}} {
				for _, term := range terms {
					if strings.Contains(line, term) {
						c := out.Policies[policy]
						c.State = "source_text"
						c.Evidence = append(c.Evidence, line)
						out.Policies[policy] = c
						break
					}
				}
			}
		}
	}
	for k, c := range out.Policies {
		c.Evidence = unique(c.Evidence)
		out.Policies[k] = c
	}
	if n := first(fs["公式HP"], func(v *html.Node) bool { return v.Data == "a" }); n != nil {
		link := attr(n, "href")
		v, e := url.Parse(link)
		if e == nil && (v.Scheme == "https" || v.Scheme == "http") && v.User == nil {
			out.OfficialURL = &link
		}
	}
	// Rating widget near the facility heading; never scrape individual reviews.
	parent := h1.Parent
	for depth := 0; parent != nil && depth < 4; depth++ {
		s := text(parent)
		m := regexp.MustCompile(`([0-5]\.[0-9]+)\s*点?\s*[/（(]?\s*([0-9,]+)\s*件`).FindStringSubmatch(s)
		if len(m) == 3 {
			out.Rating = scalar(m[1])
			out.ReviewCount = integer(m[2])
			break
		}
		parent = parent.Parent
	}
	return out, nil
}
func unique(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// ParseCoupons reports public terms only; issuance/redemption forms are never submitted.
func ParseCoupons(b []byte, sourceURL, id string) (CouponData, error) {
	d, e := doc(b)
	if e != nil {
		return CouponData{}, e
	}
	u := canonical(d, sourceURL)
	if !strings.Contains(u, "/"+id+"/coupon/") {
		return CouponData{}, fmt.Errorf("coupon identity mismatch for %s", id)
	}
	out := CouponData{FacilityID: id, URL: u, Coupons: []Coupon{}}
	seen := map[string]bool{}
	walk(d, func(n *html.Node) {
		if !has(n, "couponItem") {
			return
		}
		cid := strings.TrimSuffix(attr(n, "id"), "_cidx")
		if cid == "" || seen[cid] {
			return
		}
		seen[cid] = true
		full := text(n)
		c := Coupon{ID: cid, URL: u + "#" + attr(n, "id"), Title: strptr(text(class(n, "couponTtl"))), PriceText: strptr(text(class(n, "couponDiscount"))), ValidityText: strptr(text(class(n, "couponValid"))), Membership: "unknown", Conditions: []string{}, SourceTextComplete: true, Redeemability: "unknown; information only"}
		if c.Title == nil {
			walk(n, func(v *html.Node) {
				s := text(v)
				if c.Title == nil && v.Data == "p" && has(v, "is-weight-600") && strings.Contains(s, "円引き") {
					c.Title = &s
				}
			})
		}
		if c.PriceText == nil {
			walk(n, func(v *html.Node) {
				s := text(v)
				if c.PriceText == nil && v.Data == "p" && strings.Contains(s, "→") {
					c.PriceText = &s
				}
			})
		}
		if c.ValidityText == nil {
			for _, line := range strings.Split(full, "\n") {
				if dateRE.MatchString(line) {
					c.ValidityText = &line
					break
				}
			}
		}
		if c.ValidityText != nil {
			dates := dateRE.FindAllStringSubmatch(*c.ValidityText, -1)
			toDate := func(m []string) string {
				y, _ := strconv.Atoi(m[1])
				mo, _ := strconv.Atoi(m[2])
				day, _ := strconv.Atoi(m[3])
				return fmt.Sprintf("%04d-%02d-%02d", y, mo, day)
			}
			if len(dates) > 1 {
				c.ValidFrom = ptr(toDate(dates[0]))
			}
			if len(dates) > 0 {
				c.ValidUntil = ptr(toDate(dates[len(dates)-1]))
			}
		}
		app := has(n, "AppCouponItem") || strings.Contains(full, "アプリ限定")
		if app {
			c.AppOnly = ptr(true)
		}
		if strings.Contains(full, "おふろパス") && (strings.Contains(full, "登録") || strings.Contains(full, "会員")) {
			c.Membership = "おふろパス subscription required"
		}
		walk(n, func(v *html.Node) {
			if has(v, "couponDiscountItem") {
				s := text(v)
				if s != "" {
					c.Conditions = append(c.Conditions, s)
				}
			}
			if has(v, "couponDetailDescription") || has(v, "couponCaution") {
				s := text(v)
				if s != "" {
					c.Conditions = append(c.Conditions, s)
				}
			}
		})
		// All visible card text preserves special terms even when modern class names differ.
		c.SourceText = full
		for _, line := range strings.Split(full, "\n") {
			for _, term := range []string{"月額", "登録", "会員", "同時", "ペア", "受付", "提示", "できません", "不可", "のみ", "含ま", "別途", "特別料金", "対象", "利用可能", "利用条件"} {
				if strings.Contains(line, term) {
					c.Conditions = append(c.Conditions, line)
					break
				}
			}
		}
		c.Conditions = unique(c.Conditions)
		walk(n, func(v *html.Node) {
			if v.Data == "h3" && strings.Contains(text(v), "クーポン詳細") {
				for sib := v.NextSibling; sib != nil; sib = sib.NextSibling {
					if sib.Type == html.ElementNode {
						c.Details = strptr(text(sib))
						break
					}
				}
			}
		})
		if c.Details == nil {
			c.Details = strptr(full)
		} // Modern cards: preserve all visible count/cooldown/eligibility terms.
		if c.Title == nil {
			return
		}
		out.Coupons = append(out.Coupons, c)
	})
	if len(out.Coupons) == 0 {
		all := text(first(d, func(n *html.Node) bool { return n.Data == "main" }))
		if all == "" {
			all = text(d)
		}
		if !strings.Contains(all, "クーポンはありません") && !strings.Contains(all, "クーポンがありません") && !strings.Contains(all, "クーポンは登録されていません") {
			return out, fmt.Errorf("no recognized public coupon cards for %s; use bath show and the source coupon link", id)
		}
	}
	return out, nil
}

// ParseNearby rejects source semantic errors even when HTTP status is 200.
func ParseNearby(b []byte, lat, lon float64) ([]Facility, error) {
	var root struct {
		Result   json.RawMessage `json:"result"`
		Message  string          `json:"message"`
		Response *struct {
			List []map[string]json.RawMessage `json:"onsen_list"`
		} `json:"response"`
	}
	if err := json.Unmarshal(b, &root); err != nil {
		return nil, fmt.Errorf("Nifty map response is not JSON: %w", err)
	}
	if string(root.Result) != "1" && string(root.Result) != `"1"` {
		return nil, fmt.Errorf("Nifty map semantic error (result=%s); public map query unavailable, use bath search --region instead", root.Result)
	}
	if root.Response == nil || root.Response.List == nil {
		return nil, fmt.Errorf("Nifty map response lacks onsen_list; source layout changed")
	}
	out := []Facility{}
	seen := map[string]bool{}
	for _, r := range root.Response.List {
		s := func(k string) string {
			var x string
			if json.Unmarshal(r[k], &x) == nil {
				return x
			}
			var boolean bool
			if json.Unmarshal(r[k], &boolean) == nil {
				if boolean {
					return "1"
				}
				return "0"
			}
			var n json.Number
			if json.Unmarshal(r[k], &n) == nil {
				return n.String()
			}
			return ""
		}
		id := s("onsen_id")
		if !idRE.MatchString(id) || seen[id] {
			continue
		}
		seen[id] = true
		link := absolute(s("onsen_detail_url"))
		if link == "" {
			link = absolute(s("onsen_url"))
		}
		if link == "" {
			link = Origin + "/cs/catalog/onsen_onsen-detail/catalog_" + id + "_1.htm"
		}
		f := Facility{ID: id, Name: s("onsen_name"), URL: link, Area: strptr(s("area_text")), Latitude: scalar(s("onsen_lat")), Longitude: scalar(s("onsen_lon")), Admission: strptr(s("entrance_fee")), Hours: strptr(s("business_hours")), Rating: scalar(s("kuchikomi_score")), ReviewCount: integer(s("kuchikomi_count")), PriceBasis: "source minimum hint; fee basis, extras and eligibility unknown", SourceLabels: []string{}}
		if f.Admission != nil {
			f.MinPriceJPY = minimum(*f.Admission)
		}
		if f.Name == "" || f.Latitude == nil || f.Longitude == nil {
			continue
		}
		distance := DistanceKM(lat, lon, *f.Latitude, *f.Longitude)
		f.DistanceKM = &distance
		if x := s("coupon_flg"); x == "0" || x == "1" {
			f.CouponAvailable = ptr(x == "1")
		}
		for key, label := range map[string]string{"tennen_flg": "天然温泉", "gensen_flg": "源泉かけ流し", "roten_flg": "露天風呂", "kashikiri_flg": "貸切風呂、個室風呂", "ganban_flg": "岩盤浴", "shokuji_flg": "食事", "kyukei_flg": "休憩", "sauna_flg": "サウナ", "chushajo_flg": "駐車"} {
			if s(key) == "1" {
				f.SourceLabels = append(f.SourceLabels, label)
			}
		}
		sort.Strings(f.SourceLabels)
		if x := s("day_flg"); x == "0" || x == "1" {
			f.DayUse = ptr(x == "1")
		}
		if x := s("stay_flg"); x == "0" || x == "1" {
			f.Stay = ptr(x == "1")
		}
		if f.ReviewCount == nil || *f.ReviewCount == 0 {
			f.Rating = nil
		}

		out = append(out, f)
	}
	if len(root.Response.List) > 0 && len(out) == 0 {
		return nil, fmt.Errorf("Nifty map items have no recognized source IDs, names or coordinates")
	}
	sort.SliceStable(out, func(i, j int) bool { return *out[i].DistanceKM < *out[j].DistanceKM })
	return out, nil
}

// DistanceKM computes the great-circle distance, not a walking route.
func DistanceKM(lat1, lon1, lat2, lon2 float64) float64 {
	rad := math.Pi / 180
	dlat := (lat2 - lat1) * rad
	dlon := (lon2 - lon1) * rad
	a := math.Sin(dlat/2)*math.Sin(dlat/2) + math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dlon/2)*math.Sin(dlon/2)
	if a > 1 {
		a = 1
	}
	return math.Round(6371*2*math.Atan2(math.Sqrt(a), math.Sqrt(1-a))*1000) / 1000
}
