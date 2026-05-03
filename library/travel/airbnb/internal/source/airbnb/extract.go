package airbnb

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/cliutil"
)

var (
	moneyRe  = regexp.MustCompile(`[$£€]\s*([0-9][0-9,]*(?:\.[0-9]{2})?)`)
	roomIDRe = regexp.MustCompile(`/rooms/([0-9A-Za-z_-]+)`)
)

func firstNiobeData(root any) any {
	arr, ok := asMap(root)["niobeClientData"].([]any)
	if !ok || len(arr) == 0 {
		return nil
	}
	for _, item := range arr {
		inner, ok := item.([]any)
		if ok && len(inner) > 1 {
			if data := asMap(inner[1])["data"]; data != nil {
				return data
			}
		}
	}
	return nil
}

func listingFromSearch(lmap, priceQuote map[string]any) Listing {
	l := Listing{
		ID:                 listingIDFromSearch(lmap),
		Name:               clean(str(lmap["name"])),
		Title:              clean(str(lmap["title"])),
		AvgRatingLocalized: clean(str(lmap["avgRatingLocalized"])),
		ReviewsCount:       int(num(lmap["reviewsCount"])),
		Coordinate:         coordFromMap(asMap(lmap["coordinate"])),
		PrimaryLine:        stringList(firstByKey(lmap, "primaryLine")),
		SecondaryLine:      stringList(firstByKey(lmap, "secondaryLine")),
	}
	if l.Title == "" {
		l.Title = l.Name
	}
	for _, b := range asSlice(lmap["formattedBadges"]) {
		if text := clean(str(asMap(b)["text"])); text != "" {
			l.Badges = append(l.Badges, text)
		}
	}
	price := asMap(firstByKey(priceQuote, "structuredStayDisplayPrice"))
	l.PrimaryPrice = priceLine(asMap(price["primaryLine"]))
	l.SecondaryPrice = priceLine(asMap(price["secondaryLine"]))
	if l.SecondaryPrice != nil || l.PrimaryPrice != nil {
		l.PriceBreakdown = &PriceBreakdown{Currency: "USD", Fees: map[string]float64{}}
		if l.SecondaryPrice != nil {
			l.PriceBreakdown.Total = l.SecondaryPrice.Amount
			l.PriceTotal = l.SecondaryPrice.Amount
		}
		if l.PrimaryPrice != nil {
			l.PriceBreakdown.PerNight = l.PrimaryPrice.Amount
			l.PerNightPrice = l.PrimaryPrice.Amount
		}
	}
	enrichCounts(&l, lmap)
	return l
}

func listingFromPDPSections(root any, listingID string) *Listing {
	l := &Listing{ID: listingID, URL: airbnbBase + "/rooms/" + listingID, RawSections: map[string]any{}}
	sections := asSlice(pathValue(root, "presentation", "stayProductDetailPage", "sections", "sections"))
	for _, item := range sections {
		sm := asMap(item)
		sectionID := str(sm["sectionId"])
		if sectionID == "" {
			sectionID = str(sm["id"])
		}
		if sectionID == "" {
			continue
		}
		l.RawSections[strings.ToLower(sectionID)] = sm
		sec := asMap(sm["section"])
		if len(sec) == 0 {
			sec = sm
		}
		if inner := asMap(sec["section"]); len(inner) > 0 {
			sec = inner
		}
		switch sectionID {
		case "TITLE_DEFAULT":
			if title := clean(str(sec["title"])); title != "" {
				l.Title = title
				l.Name = title
			}
		case "MEET_YOUR_HOST":
			card := asMap(sec["cardData"])
			l.HostName = clean(str(card["name"]))
			l.HostBio = clean(str(sec["about"]))
			if l.PropertyManagementName == "" {
				l.PropertyManagementName = hostBrandFromSection(l.Title, card, sec)
			}
		case "LOCATION_DEFAULT":
			lat, lng := num(sec["lat"]), num(sec["lng"])
			if lat != 0 || lng != 0 {
				l.Coordinate = &Coord{Latitude: lat, Longitude: lng}
			}
			if subtitle := clean(str(sec["subtitle"])); subtitle != "" {
				parts := strings.SplitN(subtitle, ",", 3)
				l.City = strings.TrimSpace(parts[0])
				if len(parts) >= 2 {
					l.Region = strings.TrimSpace(parts[1])
				}
			}
		case "AMENITIES_DEFAULT":
			l.Amenities = collectTexts(sec["seeAllAmenitiesGroups"])
		case "POLICIES_DEFAULT":
			l.HouseRules = stringList(sec["houseRules"])
			l.Policies = collectTexts(sec)
		case "DESCRIPTION_DEFAULT":
			htmlDesc := asMap(sec["htmlDescription"])
			l.Description = clean(str(htmlDesc["htmlText"]))
		case "HIGHLIGHTS_DEFAULT":
			l.Highlights = collectTexts(sec["highlights"])
		case "BOOK_IT_SIDEBAR":
			if maxGuests := int(num(sec["maxGuestCapacity"])); maxGuests > 0 {
				l.SleepsMax = maxGuests
			}
			if pb := airbnbPriceBreakdown(sec["structuredDisplayPrice"]); pb != nil {
				applyPriceBreakdown(l, pb)
				price := asMap(sec["structuredDisplayPrice"])
				l.PrimaryPrice = priceLine(asMap(price["primaryLine"]))
				l.SecondaryPrice = priceLine(asMap(price["secondaryLine"]))
			}
		}
	}
	return l
}

func applyPriceBreakdown(l *Listing, pb *PriceBreakdown) {
	if l == nil || pb == nil {
		return
	}
	l.PriceBreakdown = pb
	l.PriceTotal = pb.Total
	l.PerNightPrice = pb.PerNight
}

func pathValue(v any, keys ...string) any {
	cur := v
	for _, key := range keys {
		m := asMap(cur)
		if len(m) == 0 {
			return nil
		}
		cur = m[key]
	}
	return cur
}

func hostBrandFromSection(title string, card, section map[string]any) string {
	if name := clean(str(card["name"])); name != "" {
		return name
	}
	for _, source := range []string{title, str(card["titleText"]), str(section["about"]), str(section["superhostTitleText"])} {
		if brand := titleBrand(source); brand != "" {
			return brand
		}
	}
	return ""
}

func titleBrand(s string) string {
	re := regexp.MustCompile(`\bby\s+([A-Z][\w' &-]+(?: Vacation Rentals| Getaways| Cabins| Rentals| Properties| Group| LLC| Hosts| Vacations)?)`)
	if m := re.FindStringSubmatch(s); len(m) > 1 {
		return clean(m[1])
	}
	if before, _, ok := strings.Cut(s, " is a Superhost"); ok {
		return clean(before)
	}
	return ""
}

func airbnbPriceBreakdown(v any) *PriceBreakdown {
	m := asMap(v)
	if len(m) == 0 {
		return nil
	}
	pb := &PriceBreakdown{Currency: "USD", Fees: map[string]float64{}, Raw: v}
	if primary := priceLine(asMap(m["primaryLine"])); primary != nil {
		pb.PerNight = primary.Amount
	}
	if secondary := priceLine(asMap(m["secondaryLine"])); secondary != nil {
		pb.Total = secondary.Amount
	}
	for _, item := range findObjects(m["explanationData"], []string{"price"}) {
		amount := amountFromText(firstLocalString(item, "price", "formattedAmount", "localizedPrice"))
		label := strings.ToLower(firstLocalString(item, "title", "label", "description"))
		switch {
		case amount == 0:
		case strings.Contains(label, "clean"):
			pb.Fees["cleaning"] += amount
		case strings.Contains(label, "service"):
			pb.Fees["service"] += amount
		case strings.Contains(label, "tax"):
			pb.Fees["tax"] += amount
		case strings.Contains(label, "total") && pb.Total == 0:
			pb.Total = amount
		case strings.Contains(label, "night") && pb.PerNight == 0:
			pb.PerNight = amount
		}
	}
	for _, text := range collectTexts(v) {
		amount := amountFromText(text)
		lower := strings.ToLower(text)
		switch {
		case amount == 0:
		case strings.Contains(lower, "clean"):
			pb.Fees["cleaning"] += amount
		case strings.Contains(lower, "service"):
			pb.Fees["service"] += amount
		case strings.Contains(lower, "tax"):
			pb.Fees["tax"] += amount
		case strings.Contains(lower, "total") && pb.Total == 0:
			pb.Total = amount
		}
	}
	if pb.PerNight == 0 && pb.Total == 0 && len(pb.Fees) == 0 {
		return nil
	}
	return pb
}

func firstLocalString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if s := clean(str(m[key])); s != "" {
			return s
		}
	}
	return ""
}

func listingIDFromSearch(lmap map[string]any) string {
	for _, key := range []string{"id", "listingId", "roomId", "encodedId"} {
		if id := normalizeListingID(str(lmap[key])); id != "" {
			return id
		}
	}
	for _, key := range []string{"listingUrl", "pdpUrl"} {
		if id := listingIDFromURL(str(firstByKey(lmap, key))); id != "" {
			return id
		}
	}
	if demandStayListing := asMap(lmap["demandStayListing"]); len(demandStayListing) > 0 {
		for _, key := range []string{"id", "listingId", "roomId", "encodedId"} {
			if id := normalizeListingID(str(demandStayListing[key])); id != "" {
				return id
			}
		}
	}
	return ""
}

func normalizeListingID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	if decoded, err := base64.StdEncoding.DecodeString(id); err == nil {
		if _, value, ok := strings.Cut(string(decoded), ":"); ok && value != "" {
			return value
		}
	}
	return id
}

func listingIDFromURL(raw string) string {
	if raw == "" {
		return ""
	}
	match := roomIDRe.FindStringSubmatch(raw)
	if len(match) < 2 {
		return ""
	}
	return normalizeListingID(match[1])
}

func priceLine(m map[string]any) *PriceLine {
	if len(m) == 0 {
		return nil
	}
	p := &PriceLine{
		Price:           clean(str(m["price"])),
		DiscountedPrice: clean(str(m["discountedPrice"])),
		OriginalPrice:   clean(str(m["originalPrice"])),
		Qualifier:       clean(str(m["qualifier"])),
	}
	source := p.Price
	if source == "" {
		source = p.DiscountedPrice
	}
	p.Amount = amountFromText(source)
	return p
}

func amountFromText(s string) float64 {
	match := moneyRe.FindStringSubmatch(s)
	if len(match) < 2 {
		return 0
	}
	v, _ := strconv.ParseFloat(strings.ReplaceAll(match[1], ",", ""), 64)
	return v
}

func coordFromMap(m map[string]any) *Coord {
	if len(m) == 0 {
		return nil
	}
	return &Coord{Latitude: num(m["latitude"]), Longitude: num(m["longitude"])}
}

func firstByKey(v any, key string) any {
	switch x := v.(type) {
	case map[string]any:
		if val, ok := x[key]; ok {
			return val
		}
		for _, val := range x {
			if found := firstByKey(val, key); found != nil {
				return found
			}
		}
	case []any:
		for _, val := range x {
			if found := firstByKey(val, key); found != nil {
				return found
			}
		}
	}
	return nil
}

func firstStringByKeys(v any, keys ...string) string {
	for _, key := range keys {
		if s := clean(str(firstByKey(v, key))); s != "" {
			return s
		}
	}
	return ""
}

func collectTexts(v any) []string {
	seen := map[string]bool{}
	var out []string
	var walk func(any)
	walk = func(x any) {
		switch t := x.(type) {
		case map[string]any:
			for _, key := range []string{"title", "subtitle", "body", "text", "htmlText", "description"} {
				if s := clean(str(t[key])); s != "" && len(s) < 400 && !seen[s] {
					seen[s] = true
					out = append(out, s)
				}
			}
			for _, v := range t {
				walk(v)
			}
		case []any:
			for _, v := range t {
				walk(v)
			}
		}
	}
	walk(v)
	return out
}

func collectURLs(v any) []string {
	seen := map[string]bool{}
	var out []string
	var walk func(any)
	walk = func(x any) {
		switch t := x.(type) {
		case map[string]any:
			for _, key := range []string{"url", "picture", "baseUrl"} {
				if s := str(t[key]); strings.HasPrefix(s, "http") && !seen[s] {
					seen[s] = true
					out = append(out, s)
				}
			}
			for _, v := range t {
				walk(v)
			}
		case []any:
			for _, v := range t {
				walk(v)
			}
		}
	}
	walk(v)
	return out
}

func enrichCounts(l *Listing, root any) {
	if l.City == "" {
		l.City = firstStringByKeys(root, "city", "localizedCity", "locationTitle")
	}
	if l.Beds == 0 {
		l.Beds = int(num(firstByKey(root, "beds")))
	}
	if l.Baths == 0 {
		l.Baths = num(firstByKey(root, "bathrooms"))
	}
	if l.SleepsMax == 0 {
		l.SleepsMax = int(num(firstByKey(root, "personCapacity")))
	}
}

func robotsAllows(body, path string) bool {
	applies := false
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(strings.Split(line, "#")[0])
		if line == "" {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		k, v = strings.ToLower(strings.TrimSpace(k)), strings.TrimSpace(v)
		switch k {
		case "user-agent":
			applies = v == "*"
		case "disallow":
			if applies && v != "" && strings.HasPrefix(path, v) {
				return false
			}
		case "allow":
			if applies && v != "" && strings.HasPrefix(path, v) {
				return true
			}
		}
	}
	return true
}

func RelayListingID(id string) string {
	return base64.StdEncoding.EncodeToString([]byte("DemandStayListing:" + id))
}

func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func asSlice(v any) []any {
	if s, ok := v.([]any); ok {
		return s
	}
	return nil
}

func str(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case fmt.Stringer:
		return x.String()
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", x)
	}
}

func clean(s string) string { return cliutil.CleanText(s) }

func num(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(strings.Trim(x, "$,")), 64)
		return f
	default:
		return 0
	}
}

func stringList(v any) []string {
	var out []string
	for _, item := range asSlice(v) {
		if s := stringListItem(item); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func stringListItem(item any) string {
	if m := asMap(item); len(m) > 0 {
		for _, key := range []string{"body", "text"} {
			if s := clean(str(m[key])); s != "" {
				return s
			}
		}
		return ""
	}
	return clean(str(item))
}
