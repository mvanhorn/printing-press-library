package booking

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type PropertyCard struct {
	Name              string  `json:"name"`
	Slug              string  `json:"slug"`
	Country           string  `json:"country"`
	URL               string  `json:"url"`
	Price             float64 `json:"price"`
	PriceText         string  `json:"price_text"`
	Currency          string  `json:"currency"`
	ReviewScore       float64 `json:"review_score"`
	ReviewLabel       string  `json:"review_label"`
	ReviewCount       int     `json:"review_count"`
	Stars             int     `json:"stars"`
	DistanceKM        float64 `json:"distance_km"`
	PhotoURL          string  `json:"photo_url"`
	FreeCancellation  bool    `json:"free_cancellation"`
	NoPrepayment      bool    `json:"no_prepayment"`
	BreakfastIncluded bool    `json:"breakfast_included"`
	Sustainability    bool    `json:"sustainability"`
	GeniusDiscount    bool    `json:"genius_discount"`
}

type Property struct {
	Name        string   `json:"name"`
	Slug        string   `json:"slug"`
	Country     string   `json:"country"`
	Address     string   `json:"address"`
	City        string   `json:"city"`
	PostalCode  string   `json:"postal_code"`
	Latitude    float64  `json:"latitude"`
	Longitude   float64  `json:"longitude"`
	ReviewScore float64  `json:"review_score"`
	ReviewCount int      `json:"review_count"`
	ReviewLabel string   `json:"review_label"`
	Stars       int      `json:"stars"`
	HotelType   string   `json:"hotel_type"`
	PriceRange  string   `json:"price_range"`
	Currency    string   `json:"currency"`
	Facilities  []string `json:"facilities"`
	Description string   `json:"description"`
	Photos      []string `json:"photos"`
	URL         string   `json:"url"`
}

type Review struct {
	Title           string  `json:"title"`
	Positive        string  `json:"positive"`
	Negative        string  `json:"negative"`
	Score           float64 `json:"score"`
	ReviewerName    string  `json:"reviewer_name"`
	ReviewerCountry string  `json:"reviewer_country"`
	TravelerType    string  `json:"traveler_type"`
	StayDate        string  `json:"stay_date"`
	ReviewDate      string  `json:"review_date"`
	Language        string  `json:"language"`
	HelpfulVotes    int     `json:"helpful_votes"`
}

type MapMarker struct {
	PropertyID  string  `json:"property_id"`
	Slug        string  `json:"slug"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Price       float64 `json:"price"`
	Currency    string  `json:"currency"`
	ReviewScore float64 `json:"review_score"`
}

type Trip struct {
	ConfirmationNumber string  `json:"confirmation_number"`
	PropertyName       string  `json:"property_name"`
	PropertySlug       string  `json:"property_slug"`
	Country            string  `json:"country"`
	City               string  `json:"city"`
	Checkin            string  `json:"checkin"`
	Checkout           string  `json:"checkout"`
	Nights             int     `json:"nights"`
	TotalPrice         float64 `json:"total_price"`
	Currency           string  `json:"currency"`
	State              string  `json:"state"`
	BookedOn           string  `json:"booked_on"`
}

type WishlistItem struct {
	PropertyName  string  `json:"property_name"`
	PropertySlug  string  `json:"property_slug"`
	Country       string  `json:"country"`
	City          string  `json:"city"`
	LastSeenPrice float64 `json:"last_seen_price"`
	Currency      string  `json:"currency"`
	AddedOn       string  `json:"added_on"`
	PhotoURL      string  `json:"photo_url"`
	URL           string  `json:"url"`
}

type Rewards struct {
	GeniusLevel          int      `json:"genius_level"`
	GeniusLabel          string   `json:"genius_label"`
	LifetimeStays        int      `json:"lifetime_stays"`
	CreditBalance        float64  `json:"credit_balance"`
	CreditCurrency       string   `json:"credit_currency"`
	PendingVouchers      int      `json:"pending_vouchers"`
	CategoriesDiscounted []string `json:"categories_discounted"`
}

type Profile struct {
	DisplayName       string `json:"display_name"`
	GeniusLevel       int    `json:"genius_level"`
	GeniusLabel       string `json:"genius_label"`
	PreferredLanguage string `json:"preferred_language"`
	PreferredCurrency string `json:"preferred_currency"`
	PreferredCountry  string `json:"preferred_country"`
}

type FlightOffer struct {
	OfferID         string   `json:"offer_id"`
	Carrier         string   `json:"carrier"`
	CarrierIATA     string   `json:"carrier_iata"`
	FlightNumbers   []string `json:"flight_numbers"`
	OriginIATA      string   `json:"origin_iata"`
	DestinationIATA string   `json:"destination_iata"`
	DepartTime      string   `json:"depart_time"`
	ArriveTime      string   `json:"arrive_time"`
	DurationMinutes int      `json:"duration_minutes"`
	Stops           int      `json:"stops"`
	LayoverAirports []string `json:"layover_airports"`
	Cabin           string   `json:"cabin"`
	Price           float64  `json:"price"`
	Currency        string   `json:"currency"`
	URL             string   `json:"url"`
}

type Attraction struct {
	Slug         string  `json:"slug"`
	Country      string  `json:"country"`
	Name         string  `json:"name"`
	Category     string  `json:"category"`
	DurationText string  `json:"duration_text"`
	ReviewScore  float64 `json:"review_score"`
	ReviewCount  int     `json:"review_count"`
	PriceFrom    float64 `json:"price_from"`
	Currency     string  `json:"currency"`
	PhotoURL     string  `json:"photo_url"`
	URL          string  `json:"url"`
}

type AttractionDetail struct {
	Slug               string   `json:"slug"`
	Country            string   `json:"country"`
	Name               string   `json:"name"`
	Description        string   `json:"description"`
	Category           string   `json:"category"`
	Inclusions         []string `json:"inclusions"`
	DurationOptions    []string `json:"duration_options"`
	MeetingPoint       string   `json:"meeting_point"`
	CancellationPolicy string   `json:"cancellation_policy"`
	Languages          []string `json:"languages"`
	ReviewScore        float64  `json:"review_score"`
	ReviewCount        int      `json:"review_count"`
	PriceFrom          float64  `json:"price_from"`
	Currency           string   `json:"currency"`
	Photos             []string `json:"photos"`
	URL                string   `json:"url"`
}

type CarsLanding struct {
	FeaturedDeals []string `json:"featured_deals"`
	Suppliers     []string `json:"suppliers"`
	SupplierPaths []string `json:"supplier_paths"`
	PickupCities  []string `json:"pickup_cities"`
	WebUIURL      string   `json:"web_ui_url"`
}

type Destination struct {
	DestID      string  `json:"dest_id"`
	DestType    string  `json:"dest_type"`
	Name        string  `json:"name"`
	Country     string  `json:"country"`
	CountryCode string  `json:"country_code"`
	URLName     string  `json:"urlname"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
}

var (
	numberRE          = regexp.MustCompile(`[-+]?\d[\d,]*(?:\.\d+)?`)
	positiveNumberRE  = regexp.MustCompile(`\d[\d,]*(?:\.\d+)?`)
	moneyRE           = regexp.MustCompile(`(?i)(US\$|CA\$|AU\$|NZ\$|HK\$|SG\$|[$€£¥₹])\s*(\d[\d,]*(?:\.\d+)?)|(\d[\d,]*(?:\.\d+)?)\s*(USD|EUR|GBP|CAD|AUD|NZD|HKD|SGD|JPY|CNY|INR)`)
	hotelURLRE        = regexp.MustCompile(`(?i)/hotel/([^/]+)/([^/?#]+?)\.html`)
	attractionURLRE   = regexp.MustCompile(`(?i)/attractions/([a-z]{2})/(pr[^/?#]+?)(?:\.html)?(?:[/?#]|$)`)
	timeRE            = regexp.MustCompile(`\b(?:[01]?\d|2[0-3]):[0-5]\d\b`)
	flightNumberRE    = regexp.MustCompile(`\b([A-Z0-9]{2})\s?(\d{1,4}[A-Z]?)\b`)
	iataRE            = regexp.MustCompile(`\b[A-Z]{3}\b`)
	reviewLabelValues = []string{"Exceptional", "Wonderful", "Superb", "Fabulous", "Excellent", "Very Good", "Good", "Pleasant", "Fair", "Poor", "Bad"}
	carSuppliers      = map[string]string{
		"alamo": "Alamo", "avis": "Avis", "budget": "Budget", "dollar": "Dollar", "enterprise": "Enterprise",
		"fox": "Fox", "hertz": "Hertz", "sixt": "Sixt", "thrifty": "Thrifty",
	}
)

func ParseSearchResults(html []byte) ([]byte, error) {
	doc, err := parseHTML(html, "searchresults")
	if err != nil {
		return nil, err
	}
	items := make([]PropertyCard, 0)
	doc.Find(`[data-testid="property-card"]`).Each(func(_ int, card *goquery.Selection) {
		href := firstAttr(card, "href", `a[href*="/hotel/"]`)
		country, slug := parseHotelPath(href)
		priceText := firstText(card, `[data-testid="price-and-discounted-price"]`)
		price, currency := parseMoney(priceText)
		reviewText := firstText(card, `[data-testid="review-score"]`)
		score, label, count := parseReviewSummary(reviewText)
		text := cleanText(card.Text())
		item := PropertyCard{
			Name:              firstText(card, `[data-testid="title"]`, `h3`, `h2`),
			Slug:              slug,
			Country:           country,
			URL:               absoluteBookingURL(href),
			Price:             price,
			PriceText:         priceText,
			Currency:          currency,
			ReviewScore:       score,
			ReviewLabel:       label,
			ReviewCount:       count,
			Stars:             parseStars(card),
			DistanceKM:        parseDistanceKM(text),
			PhotoURL:          firstAttr(card, "src", `img[src]`),
			FreeCancellation:  containsAny(text, "free cancellation"),
			NoPrepayment:      containsAny(text, "no prepayment", "no payment needed"),
			BreakfastIncluded: containsAny(text, "breakfast included", "includes breakfast"),
			Sustainability:    containsAny(text, "sustainable", "travel sustainable"),
			GeniusDiscount:    containsAny(text, "genius"),
		}
		items = append(items, item)
	})
	return marshal("searchresults", items)
}

func ParseHotelDetail(html []byte) ([]byte, error) {
	doc, err := parseHTML(html, "hotel detail")
	if err != nil {
		return nil, err
	}
	ld := findJSONLD(doc, "Hotel")
	canonical := firstAttr(doc.Selection, "href", `link[rel="canonical"]`, `meta[property="og:url"]`)
	country, slug := parseHotelPath(canonical)
	photos := uniqueAttrs(doc.Selection, "src", `img[src*="bstatic.com"]`, 6)
	facilities := collectHotelAmenities(doc, ld)
	rating := objectAt(ld, "aggregateRating")
	address := objectAt(ld, "address")
	geo := objectAt(ld, "geo")
	priceRange := stringValue(ld["priceRange"])
	_, currency := parseMoney(priceRange)
	prop := Property{
		Name:        firstNonEmpty(stringValue(ld["name"]), firstText(doc.Selection, `h1`), firstAttr(doc.Selection, "content", `meta[property="og:title"]`)),
		Slug:        slug,
		Country:     firstNonEmpty(country, stringValue(address["addressCountry"])),
		Address:     stringValue(address["streetAddress"]),
		City:        stringValue(address["addressLocality"]),
		PostalCode:  stringValue(address["postalCode"]),
		Latitude:    floatValue(geo["latitude"]),
		Longitude:   floatValue(geo["longitude"]),
		ReviewScore: floatValue(rating["ratingValue"]),
		ReviewCount: intValue(rating["reviewCount"]),
		ReviewLabel: findReviewLabel(cleanText(doc.Text())),
		Stars:       parseStars(doc.Selection),
		HotelType:   firstNonEmpty(stringValue(ld["@type"]), firstAttr(doc.Selection, "content", `meta[property="og:type"]`)),
		PriceRange:  priceRange,
		Currency:    currency,
		Facilities:  facilities,
		Description: firstNonEmpty(stringValue(ld["description"]), firstText(doc.Selection, `[data-testid="property-description"]`, `#property_description_content`, `[property-description]`), firstAttr(doc.Selection, "content", `meta[name="description"]`)),
		Photos:      photos,
		URL:         absoluteBookingURL(canonical),
	}
	return marshal("hotel detail", prop)
}

func ParseReviewList(html []byte) ([]byte, error) {
	doc, err := parseHTML(html, "reviewlist")
	if err != nil {
		return nil, err
	}
	items := make([]Review, 0)
	doc.Find(`[data-testid*="review-card"], .c-review-block`).Each(func(_ int, card *goquery.Selection) {
		text := cleanText(card.Text())
		review := Review{
			Title:           firstText(card, `[data-testid="review-title"]`, `.c-review-block__title`, `h3`, `h4`),
			Positive:        firstText(card, `[data-testid="review-positive-text"]`, `.c-review__row--positive`, `[class*="positive"]`),
			Negative:        firstText(card, `[data-testid="review-negative-text"]`, `.c-review__row--negative`, `[class*="negative"]`),
			Score:           parseFloatText(firstText(card, `[data-testid="review-score"]`, `.bui-review-score__badge`, `[class*="score"]`)),
			ReviewerName:    firstText(card, `[data-testid="reviewer-name"]`, `.bui-avatar-block__title`, `[class*="reviewer"] [class*="name"]`),
			ReviewerCountry: firstText(card, `[data-testid="reviewer-country"]`, `.bui-avatar-block__subtitle`, `[class*="country"]`),
			TravelerType:    labeledValue(text, "Traveler type", "Trip type"),
			StayDate:        labeledValue(text, "Stayed in", "Stay date"),
			ReviewDate:      labeledValue(text, "Reviewed", "Review date"),
			Language:        labeledValue(text, "Language"),
			HelpfulVotes:    parseHelpfulVotes(text),
		}
		if review.Score == 0 {
			review.Score = parseFloatText(text)
		}
		items = append(items, review)
	})
	return marshal("reviewlist", items)
}

func ParseFlightOffers(html []byte) ([]byte, error) {
	doc, err := parseHTML(html, "flight offers")
	if err != nil {
		return nil, err
	}
	items := make([]FlightOffer, 0)
	doc.Find(`[class*="FlightCard"], [data-testid*="flight-card"], article[class*="flight"]`).Each(func(_ int, card *goquery.Selection) {
		text := cleanText(card.Text())
		times := timeRE.FindAllString(text, -1)
		price, currency := parseMoney(text)
		flightNumbers, carrierIATA := parseFlightNumbers(text)
		airports := uniqueStrings(iataRE.FindAllString(text, -1), 0)
		offer := FlightOffer{
			OfferID:         firstNonEmpty(firstAttr(card, "data-offer-id", `[data-offer-id]`), firstAttr(card, "id", `[id]`)),
			Carrier:         firstNonEmpty(firstText(card, `[class*="Carrier"]`, `[data-testid*="carrier"]`), firstAttr(card, "alt", `img[alt]`)),
			CarrierIATA:     carrierIATA,
			FlightNumbers:   flightNumbers,
			OriginIATA:      "",
			DestinationIATA: "",
			DepartTime:      nthString(times, 0),
			ArriveTime:      nthString(times, 1),
			DurationMinutes: parseDurationMinutes(text),
			Stops:           parseStops(text),
			LayoverAirports: airports,
			Cabin:           parseCabin(text),
			Price:           price,
			Currency:        currency,
			URL:             absoluteBookingURL(firstAttr(card, "href", `a[href]`)),
		}
		items = append(items, offer)
	})
	return marshal("flight offers", items)
}

func ParseAttractionList(html []byte) ([]byte, error) {
	doc, err := parseHTML(html, "attraction list")
	if err != nil {
		return nil, err
	}
	items := make([]Attraction, 0)
	seen := map[string]bool{}
	doc.Find(`a[href*="/attractions/"]`).Each(func(_ int, link *goquery.Selection) {
		href, _ := link.Attr("href")
		country, slug := parseAttractionPath(href)
		if slug == "" || seen[country+"/"+slug] {
			return
		}
		seen[country+"/"+slug] = true
		card := surroundingCard(link)
		text := cleanText(card.Text())
		price, currency := parseMoney(text)
		score, _, count := parseReviewSummary(text)
		item := Attraction{
			Slug:         slug,
			Country:      country,
			Name:         firstNonEmpty(cleanText(link.Text()), firstText(card, `h3`, `h2`, `[data-testid*="title"]`)),
			Category:     firstText(card, `[data-testid*="category"]`, `[class*="category"]`),
			DurationText: firstDurationText(text),
			ReviewScore:  score,
			ReviewCount:  count,
			PriceFrom:    price,
			Currency:     currency,
			PhotoURL:     firstAttr(card, "src", `img[src]`),
			URL:          absoluteBookingURL(href),
		}
		items = append(items, item)
	})
	return marshal("attraction list", items)
}

func ParseAttractionDetail(html []byte) ([]byte, error) {
	doc, err := parseHTML(html, "attraction detail")
	if err != nil {
		return nil, err
	}
	canonical := firstAttr(doc.Selection, "href", `link[rel="canonical"]`, `meta[property="og:url"]`)
	country, slug := parseAttractionPath(canonical)
	text := cleanText(doc.Text())
	price, currency := parseMoney(text)
	score, _, count := parseReviewSummary(text)
	item := AttractionDetail{
		Slug:               slug,
		Country:            country,
		Name:               firstNonEmpty(firstText(doc.Selection, `h1`, `[data-testid*="title"]`), firstAttr(doc.Selection, "content", `meta[property="og:title"]`)),
		Description:        firstNonEmpty(firstText(doc.Selection, `[data-testid*="description"]`, `[class*="description"]`), longestParagraph(doc.Selection), firstAttr(doc.Selection, "content", `meta[name="description"]`)),
		Category:           firstText(doc.Selection, `[data-testid*="category"]`, `[class*="category"]`, `nav a`),
		Inclusions:         collectTexts(doc.Selection, `[data-testid*="included"] li, [class*="included"] li, [data-testid*="inclusion"] li`, 0),
		DurationOptions:    uniqueStrings(append(collectTexts(doc.Selection, `[data-testid*="duration"] li, [class*="duration"] li`, 0), firstDurationText(text)), 0),
		MeetingPoint:       labeledValue(text, "Meeting point", "Meeting location"),
		CancellationPolicy: labeledValue(text, "Cancellation policy", "Cancellation"),
		Languages:          collectTexts(doc.Selection, `[data-testid*="language"] li, [class*="language"] li`, 0),
		ReviewScore:        score,
		ReviewCount:        count,
		PriceFrom:          price,
		Currency:           currency,
		Photos:             uniqueAttrs(doc.Selection, "src", `img[src*="bstatic.com"], img[src]`, 12),
		URL:                absoluteBookingURL(canonical),
	}
	return marshal("attraction detail", item)
}

func ParseCarsLanding(html []byte) ([]byte, error) {
	doc, err := parseHTML(html, "cars landing")
	if err != nil {
		return nil, err
	}
	out := CarsLanding{
		FeaturedDeals: make([]string, 0),
		Suppliers:     make([]string, 0),
		SupplierPaths: make([]string, 0),
		PickupCities:  make([]string, 0),
		WebUIURL:      "https://www.booking.com/cars/index.html",
	}
	doc.Find("h1, h2, h3, [data-testid*=headline], [class*=headline]").Each(func(_ int, s *goquery.Selection) {
		text := cleanText(s.Text())
		if text != "" && len(text) < 180 {
			out.FeaturedDeals = append(out.FeaturedDeals, text)
		}
	})
	doc.Find(`a[href*="/cars/"]`).Each(func(_ int, a *goquery.Selection) {
		href, _ := a.Attr("href")
		path := pathOnly(href)
		if city := pickupCityPath(path); city != "" {
			out.PickupCities = append(out.PickupCities, city)
			return
		}
		if supplier, ok := supplierFromPath(path); ok {
			out.Suppliers = append(out.Suppliers, supplier)
			out.SupplierPaths = append(out.SupplierPaths, path)
		}
	})
	out.FeaturedDeals = uniqueStrings(out.FeaturedDeals, 0)
	out.Suppliers = uniqueStrings(out.Suppliers, 0)
	out.SupplierPaths = uniqueStrings(out.SupplierPaths, 0)
	out.PickupCities = uniqueStrings(out.PickupCities, 0)
	return marshal("cars landing", out)
}

func ParseTrips(html []byte) ([]byte, error) {
	doc, err := parseHTML(html, "trips")
	if err != nil {
		return nil, err
	}
	items := make([]Trip, 0)
	doc.Find(`[data-testid="trip-card"], .booking-card, .trip__upcoming-list-item`).Each(func(_ int, card *goquery.Selection) {
		text := cleanText(card.Text())
		href := firstAttr(card, "href", `a[href*="/hotel/"]`)
		country, slug := parseHotelPath(href)
		checkin, checkout := parseDatePair(text)
		price, currency := parseMoney(text)
		item := Trip{
			ConfirmationNumber: labeledValue(text, "Confirmation number", "Booking number"),
			PropertyName:       firstText(card, `[data-testid*="property-name"]`, `[class*="property"]`, `h3`, `h2`),
			PropertySlug:       slug,
			Country:            country,
			City:               labeledValue(text, "City"),
			Checkin:            checkin,
			Checkout:           checkout,
			Nights:             firstNonZeroInt(parseLabeledInt(text, "night"), nightsBetween(checkin, checkout)),
			TotalPrice:         price,
			Currency:           currency,
			State:              parseTripState(text),
			BookedOn:           labeledValue(text, "Booked on", "Booked"),
		}
		items = append(items, item)
	})
	return marshal("trips", items)
}

func ParseWishlist(html []byte) ([]byte, error) {
	doc, err := parseHTML(html, "wishlist")
	if err != nil {
		return nil, err
	}
	items := make([]WishlistItem, 0)
	doc.Find(`[data-testid*="wishlist-item"], [class*="wishlist"] li, [data-property-id]`).Each(func(_ int, card *goquery.Selection) {
		text := cleanText(card.Text())
		href := firstAttr(card, "href", `a[href*="/hotel/"]`)
		country, slug := parseHotelPath(href)
		price, currency := parseMoney(text)
		item := WishlistItem{
			PropertyName:  firstText(card, `[data-testid*="property-name"]`, `[data-testid*="title"]`, `h3`, `h2`),
			PropertySlug:  slug,
			Country:       country,
			City:          labeledValue(text, "City"),
			LastSeenPrice: price,
			Currency:      currency,
			AddedOn:       labeledValue(text, "Added on", "Saved on"),
			PhotoURL:      firstAttr(card, "src", `img[src]`),
			URL:           absoluteBookingURL(href),
		}
		items = append(items, item)
	})
	return marshal("wishlist", items)
}

func ParseRewards(html []byte) ([]byte, error) {
	doc, err := parseHTML(html, "rewards")
	if err != nil {
		return nil, err
	}
	text := cleanText(doc.Text())
	balance, currency := parseMoney(text)
	level := parseGeniusLevel(text)
	out := Rewards{
		GeniusLevel:          level,
		GeniusLabel:          geniusLabel(level, text),
		LifetimeStays:        parseLabeledInt(text, "lifetime stays", "stays"),
		CreditBalance:        balance,
		CreditCurrency:       currency,
		PendingVouchers:      parseLabeledInt(text, "pending vouchers", "vouchers"),
		CategoriesDiscounted: discountedCategories(text),
	}
	return marshal("rewards", out)
}

func ParseProfile(html []byte) ([]byte, error) {
	doc, err := parseHTML(html, "profile")
	if err != nil {
		return nil, err
	}
	text := cleanText(doc.Text())
	level := parseGeniusLevel(text)
	out := Profile{
		DisplayName:       firstNonEmpty(firstText(doc.Selection, `[data-testid*="display-name"]`, `[class*="profile"] h1`, `h1`), labeledValue(text, "Name", "Display name")),
		GeniusLevel:       level,
		GeniusLabel:       geniusLabel(level, text),
		PreferredLanguage: firstNonEmpty(settingValue(doc.Selection, "language"), labeledValue(text, "Preferred language", "Language")),
		PreferredCurrency: firstNonEmpty(settingValue(doc.Selection, "currency"), labeledValue(text, "Preferred currency", "Currency")),
		PreferredCountry:  firstNonEmpty(settingValue(doc.Selection, "country"), labeledValue(text, "Preferred country", "Country")),
	}
	return marshal("profile", out)
}

func ParseDestinationAutocomplete(html []byte) ([]byte, error) {
	doc, err := parseHTML(html, "destination autocomplete")
	if err != nil {
		return nil, err
	}
	page := string(html)
	canonical := firstAttr(doc.Selection, "href", `link[rel="canonical"]`, `meta[property="og:url"]`)
	values := queryValues(canonical)
	out := Destination{
		DestID:      firstNonEmpty(values.Get("dest_id"), regexValue(page, `(?i)["']?b?_?dest_id["']?\s*[:=]\s*["']?([^"',;&<\s]+)`)),
		DestType:    firstNonEmpty(values.Get("dest_type"), regexValue(page, `(?i)["']?b?_?dest_type["']?\s*[:=]\s*["']?([^"',;&<\s]+)`)),
		Name:        firstNonEmpty(regexValue(page, `(?i)["']?(?:ss|destination_name|name)["']?\s*[:=]\s*["']([^"']+)`), firstText(doc.Selection, `h1`)),
		Country:     regexValue(page, `(?i)["']?(?:country|b_country_name)["']?\s*[:=]\s*["']([^"']+)`),
		CountryCode: strings.ToUpper(regexValue(page, `(?i)["']?(?:country_code|b_country_code|cc1)["']?\s*[:=]\s*["']?([a-z]{2})`)),
		URLName:     firstNonEmpty(values.Get("dest_urlname"), values.Get("ssne"), regexValue(page, `(?i)["']?(?:urlname|dest_urlname)["']?\s*[:=]\s*["']([^"']+)`)),
		Latitude:    parseFloatText(firstNonEmpty(regexValue(page, `(?i)["']?(?:latitude|lat|b_latitude)["']?\s*[:=]\s*["']?([-+]?\d+(?:\.\d+)?)`), values.Get("latitude"))),
		Longitude:   parseFloatText(firstNonEmpty(regexValue(page, `(?i)["']?(?:longitude|lon|lng|b_longitude)["']?\s*[:=]\s*["']?([-+]?\d+(?:\.\d+)?)`), values.Get("longitude"))),
	}
	return marshal("destination autocomplete", out)
}

func ParseMapMarkers(data []byte) ([]byte, error) {
	items := make([]MapMarker, 0)
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return marshal("map markers", items)
	}
	results, ok := mapResults(root)
	if !ok {
		return marshal("map markers", items)
	}
	for _, raw := range results {
		obj, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		price, currency := markerPrice(obj)
		propertyID := recursiveString(obj, "property_id", "propertyId", "id", "hotelId")
		if propertyID == "" {
			if numericID := recursiveNumber(obj, "property_id", "propertyId", "id", "hotelId"); numericID != 0 {
				propertyID = strconv.FormatFloat(numericID, 'f', -1, 64)
			}
		}
		items = append(items, MapMarker{
			PropertyID:  propertyID,
			Slug:        recursiveString(obj, "slug", "pageName", "urlName"),
			Latitude:    recursiveNumber(obj, "latitude", "lat"),
			Longitude:   recursiveNumber(obj, "longitude", "lng", "lon"),
			Price:       price,
			Currency:    currency,
			ReviewScore: recursiveNumber(obj, "review_score", "reviewScore", "ratingValue", "score"),
		})
	}
	return marshal("map markers", items)
}

func parseHTML(html []byte, endpoint string) (*goquery.Document, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", endpoint, err)
	}
	return doc, nil
}

func marshal(endpoint string, v any) ([]byte, error) {
	out, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", endpoint, err)
	}
	return out, nil
}

func firstText(root *goquery.Selection, selectors ...string) string {
	for _, selector := range selectors {
		var text string
		if selector == "" {
			text = cleanText(root.Text())
		} else {
			text = cleanText(root.Find(selector).First().Text())
		}
		if text != "" {
			return text
		}
	}
	return ""
}

func firstAttr(root *goquery.Selection, attr string, selectors ...string) string {
	for _, selector := range selectors {
		selection := root
		if selector != "" {
			selection = root.Find(selector).First()
		}
		if v, ok := selection.Attr(attr); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
		if attr == "href" {
			if v, ok := selection.Attr("content"); ok && strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		}
	}
	return ""
}

func cleanText(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func parseMoney(text string) (float64, string) {
	m := moneyRE.FindStringSubmatch(text)
	if len(m) == 0 {
		return parsePositiveFloatText(text), ""
	}
	if m[2] != "" {
		return parsePositiveFloatText(m[2]), currencyFromSymbol(m[1])
	}
	return parsePositiveFloatText(m[3]), strings.ToUpper(m[4])
}

func parseFloatText(text string) float64 {
	m := numberRE.FindString(text)
	if m == "" {
		return 0
	}
	n, _ := strconv.ParseFloat(strings.ReplaceAll(m, ",", ""), 64)
	return n
}

// parsePositiveFloatText is parseFloatText that ignores a leading sign.
// Use for fields that are semantically non-negative (review scores, prices,
// counts) so that scraped promo text like "-20% off" does not poison the result.
func parsePositiveFloatText(text string) float64 {
	m := positiveNumberRE.FindString(text)
	if m == "" {
		return 0
	}
	n, _ := strconv.ParseFloat(strings.ReplaceAll(m, ",", ""), 64)
	return n
}

func intValue(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case json.Number:
		i, _ := x.Int64()
		return int(i)
	case string:
		return int(parseFloatText(x))
	default:
		return 0
	}
}

func floatValue(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case json.Number:
		f, _ := x.Float64()
		return f
	case string:
		return parseFloatText(x)
	default:
		return 0
	}
}

func stringValue(v any) string {
	switch x := v.(type) {
	case string:
		return cleanText(x)
	case fmt.Stringer:
		return cleanText(x.String())
	case nil:
		return ""
	default:
		return cleanText(fmt.Sprint(x))
	}
}

func currencyFromSymbol(symbol string) string {
	switch strings.ToUpper(symbol) {
	case "$", "US$":
		return "USD"
	case "CA$":
		return "CAD"
	case "AU$":
		return "AUD"
	case "NZ$":
		return "NZD"
	case "HK$":
		return "HKD"
	case "SG$":
		return "SGD"
	case "€":
		return "EUR"
	case "£":
		return "GBP"
	case "¥":
		return "JPY"
	case "₹":
		return "INR"
	default:
		return strings.TrimSpace(symbol)
	}
}

func parseReviewSummary(text string) (float64, string, int) {
	// Booking.com listing cards interleave promo discounts ("-20% off") with
	// review scores; use the non-negative parser so promo text cannot poison
	// the score. Real review scores are 0-10 anyway.
	score := parsePositiveFloatText(text)
	if score > 10 {
		score = 0
	}
	label := findReviewLabel(text)
	count := 0
	if m := regexp.MustCompile(`(?i)([\d,]+)\s+reviews?`).FindStringSubmatch(text); len(m) > 1 {
		count = int(parsePositiveFloatText(m[1]))
	}
	return score, label, count
}

func findReviewLabel(text string) string {
	lower := strings.ToLower(text)
	for _, label := range reviewLabelValues {
		if strings.Contains(lower, strings.ToLower(label)) {
			return label
		}
	}
	return ""
}

func parseHotelPath(href string) (string, string) {
	if m := hotelURLRE.FindStringSubmatch(href); len(m) > 2 {
		return strings.ToLower(m[1]), strings.TrimSuffix(m[2], ".html")
	}
	return "", ""
}

func parseAttractionPath(href string) (string, string) {
	if m := attractionURLRE.FindStringSubmatch(href); len(m) > 2 {
		return strings.ToLower(m[1]), strings.TrimSuffix(m[2], ".html")
	}
	return "", ""
}

func absoluteBookingURL(href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	if strings.HasPrefix(href, "//") {
		return "https:" + href
	}
	if strings.HasPrefix(href, "/") {
		return "https://www.booking.com" + href
	}
	return href
}

func parseStars(root *goquery.Selection) int {
	text := cleanText(root.Text())
	root.Find(`[aria-label]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if label, ok := s.Attr("aria-label"); ok {
			text += " " + label
		}
		return true
	})
	if m := regexp.MustCompile(`(?i)\b([1-5])\s*(?:out of\s*)?5?\s*stars?\b`).FindStringSubmatch(text); len(m) > 1 {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	if m := regexp.MustCompile(`(?i)\b([1-5])-star\b`).FindStringSubmatch(text); len(m) > 1 {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 0
}

func parseDistanceKM(text string) float64 {
	if m := regexp.MustCompile(`(?i)([\d.]+)\s*(km|kilometers?|miles?|mi)\b`).FindStringSubmatch(text); len(m) > 2 {
		value := parseFloatText(m[1])
		unit := strings.ToLower(m[2])
		if strings.HasPrefix(unit, "mi") {
			return value * 1.60934
		}
		return value
	}
	return 0
}

func containsAny(text string, needles ...string) bool {
	lower := strings.ToLower(text)
	for _, needle := range needles {
		if strings.Contains(lower, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if clean := cleanText(v); clean != "" {
			return clean
		}
	}
	return ""
}

func firstNonZeroInt(values ...int) int {
	for _, v := range values {
		if v != 0 {
			return v
		}
	}
	return 0
}

func nthString(values []string, idx int) string {
	if idx >= 0 && idx < len(values) {
		return values[idx]
	}
	return ""
}

// collectHotelAmenities returns just the actual amenity tiles for a hotel
// detail page. Booking.com renders amenities in a single bordered card whose
// container has data-testid="property-most-popular-facilities-wrapper". Earlier
// versions of this parser fanned out to any selector containing the substring
// "facilities", which dragged in the top nav, breadcrumbs, section tabs, and
// per-category review scores. JSON-LD amenityFeature is preferred when present,
// otherwise the narrow testid wrapper is used.
func collectHotelAmenities(doc *goquery.Document, ld map[string]any) []string {
	if features, ok := ld["amenityFeature"].([]any); ok {
		names := make([]string, 0, len(features))
		for _, f := range features {
			if obj, ok := f.(map[string]any); ok {
				if name := cleanText(stringValue(obj["name"])); name != "" {
					names = append(names, name)
				}
			} else if name := cleanText(stringValue(f)); name != "" {
				names = append(names, name)
			}
		}
		if len(names) > 0 {
			return uniqueStrings(names, 0)
		}
	}
	return collectTexts(doc.Selection, `[data-testid="property-most-popular-facilities-wrapper"] li`, 0)
}

func collectTexts(root *goquery.Selection, selector string, limit int) []string {
	items := make([]string, 0)
	root.Find(selector).EachWithBreak(func(_ int, s *goquery.Selection) bool {
		text := cleanText(s.Text())
		if text != "" {
			items = append(items, text)
		}
		return limit == 0 || len(items) < limit
	})
	return uniqueStrings(items, limit)
}

func uniqueAttrs(root *goquery.Selection, attr, selector string, limit int) []string {
	items := make([]string, 0)
	root.Find(selector).EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if value, ok := s.Attr(attr); ok && strings.TrimSpace(value) != "" {
			items = append(items, strings.TrimSpace(value))
		}
		return limit == 0 || len(items) < limit
	})
	return uniqueStrings(items, limit)
}

func uniqueStrings(values []string, limit int) []string {
	out := make([]string, 0)
	seen := map[string]bool{}
	for _, value := range values {
		clean := cleanText(value)
		key := strings.ToLower(clean)
		if clean == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, clean)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func findJSONLD(doc *goquery.Document, typeName string) map[string]any {
	var found map[string]any
	doc.Find(`script[type="application/ld+json"]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
		var raw any
		if err := json.Unmarshal([]byte(strings.TrimSpace(s.Text())), &raw); err != nil {
			return true
		}
		if obj := findTypedObject(raw, typeName); obj != nil {
			found = obj
			return false
		}
		return true
	})
	if found == nil {
		return map[string]any{}
	}
	return found
}

func findTypedObject(v any, typeName string) map[string]any {
	switch x := v.(type) {
	case map[string]any:
		if matchesType(x["@type"], typeName) {
			return x
		}
		for _, value := range x {
			if child := findTypedObject(value, typeName); child != nil {
				return child
			}
		}
	case []any:
		for _, value := range x {
			if child := findTypedObject(value, typeName); child != nil {
				return child
			}
		}
	}
	return nil
}

func matchesType(v any, typeName string) bool {
	switch x := v.(type) {
	case string:
		return strings.EqualFold(x, typeName)
	case []any:
		for _, value := range x {
			if matchesType(value, typeName) {
				return true
			}
		}
	}
	return false
}

func objectAt(obj map[string]any, key string) map[string]any {
	if child, ok := obj[key].(map[string]any); ok {
		return child
	}
	return map[string]any{}
}

func labeledValue(text string, labels ...string) string {
	for _, label := range labels {
		pattern := fmt.Sprintf(`(?i)%s\s*:?\s*([^|.]+)`, regexp.QuoteMeta(label))
		if m := regexp.MustCompile(pattern).FindStringSubmatch(text); len(m) > 1 {
			return cleanText(m[1])
		}
	}
	return ""
}

func parseHelpfulVotes(text string) int {
	if m := regexp.MustCompile(`(?i)(\d+)\s+helpful`).FindStringSubmatch(text); len(m) > 1 {
		return int(parseFloatText(m[1]))
	}
	return 0
}

func parseFlightNumbers(text string) ([]string, string) {
	matches := flightNumberRE.FindAllStringSubmatch(text, -1)
	out := make([]string, 0)
	carrier := ""
	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		if carrier == "" {
			carrier = m[1]
		}
		out = append(out, m[1]+m[2])
	}
	return uniqueStrings(out, 0), carrier
}

func parseDurationMinutes(text string) int {
	if m := regexp.MustCompile(`(?i)(\d+)\s*h(?:ours?)?\s*(\d+)?\s*m?`).FindStringSubmatch(text); len(m) > 1 {
		hours, _ := strconv.Atoi(m[1])
		minutes := 0
		if len(m) > 2 && m[2] != "" {
			minutes, _ = strconv.Atoi(m[2])
		}
		return hours*60 + minutes
	}
	if m := regexp.MustCompile(`(?i)(\d+)\s*m(?:in(?:utes?)?)?`).FindStringSubmatch(text); len(m) > 1 {
		minutes, _ := strconv.Atoi(m[1])
		return minutes
	}
	return 0
}

func parseStops(text string) int {
	lower := strings.ToLower(text)
	if strings.Contains(lower, "nonstop") || strings.Contains(lower, "direct") {
		return 0
	}
	if m := regexp.MustCompile(`(?i)(\d+)\s+stops?`).FindStringSubmatch(text); len(m) > 1 {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 0
}

func parseCabin(text string) string {
	for _, cabin := range []string{"economy", "premium economy", "business", "first"} {
		if strings.Contains(strings.ToLower(text), cabin) {
			return cabin
		}
	}
	return ""
}

func surroundingCard(link *goquery.Selection) *goquery.Selection {
	best := link.Parent()
	bestLen := len(cleanText(best.Text()))
	current := best
	for i := 0; i < 5 && current.Length() > 0; i++ {
		textLen := len(cleanText(current.Text()))
		if textLen > bestLen && textLen < 2500 {
			best = current
			bestLen = textLen
		}
		current = current.Parent()
	}
	return best
}

func firstDurationText(text string) string {
	if m := regexp.MustCompile(`(?i)(?:duration\s*)?(\d+(?:\.\d+)?\s*(?:hours?|hrs?|minutes?|mins?)(?:\s*-\s*\d+(?:\.\d+)?\s*(?:hours?|hrs?|minutes?|mins?))?)`).FindStringSubmatch(text); len(m) > 1 {
		return cleanText(m[1])
	}
	return ""
}

func longestParagraph(root *goquery.Selection) string {
	longest := ""
	root.Find("p").Each(func(_ int, p *goquery.Selection) {
		text := cleanText(p.Text())
		if len(text) > len(longest) {
			longest = text
		}
	})
	return longest
}

func pathOnly(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Path != "" {
		return u.Path
	}
	return raw
}

func supplierFromPath(path string) (string, bool) {
	if m := regexp.MustCompile(`(?i)^/cars/([^/]+)/index\.html`).FindStringSubmatch(path); len(m) > 1 {
		supplier, ok := carSuppliers[strings.ToLower(m[1])]
		return supplier, ok
	}
	return "", false
}

func pickupCityPath(path string) string {
	if regexp.MustCompile(`(?i)^/cars/city/[a-z]{2}/[^/]+\.html`).MatchString(path) {
		return path
	}
	return ""
}

func parseDatePair(text string) (string, string) {
	dates := regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}\b|(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Sept|Oct|Nov|Dec)[a-z]*\.?\s+\d{1,2},?\s+\d{4}`).FindAllString(text, -1)
	if len(dates) < 2 {
		return "", ""
	}
	return normalizeDate(dates[0]), normalizeDate(dates[1])
}

func normalizeDate(raw string) string {
	raw = strings.ReplaceAll(strings.TrimSpace(raw), ".", "")
	for _, layout := range []string{"2006-01-02", "Jan 2, 2006", "January 2, 2006", "Jan 2 2006", "January 2 2006"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return raw
}

func nightsBetween(checkin, checkout string) int {
	in, errIn := time.Parse("2006-01-02", checkin)
	out, errOut := time.Parse("2006-01-02", checkout)
	if errIn != nil || errOut != nil || !out.After(in) {
		return 0
	}
	return int(out.Sub(in).Hours() / 24)
}

func parseLabeledInt(text string, labels ...string) int {
	for _, label := range labels {
		if m := regexp.MustCompile(`(?i)(\d+)\s+` + regexp.QuoteMeta(label)).FindStringSubmatch(text); len(m) > 1 {
			n, _ := strconv.Atoi(m[1])
			return n
		}
		if m := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(label) + `\s*:?\s*(\d+)`).FindStringSubmatch(text); len(m) > 1 {
			n, _ := strconv.Atoi(m[1])
			return n
		}
	}
	return 0
}

func parseTripState(text string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "cancelled") || strings.Contains(lower, "canceled"):
		return "cancelled"
	case strings.Contains(lower, "past"):
		return "past"
	case strings.Contains(lower, "upcoming"):
		return "upcoming"
	default:
		return ""
	}
}

func parseGeniusLevel(text string) int {
	if m := regexp.MustCompile(`(?i)genius\s+level\s+([1-3])|level\s+([1-3])`).FindStringSubmatch(text); len(m) > 0 {
		for _, part := range m[1:] {
			if part != "" {
				n, _ := strconv.Atoi(part)
				return n
			}
		}
	}
	return 0
}

func geniusLabel(level int, text string) string {
	if level > 0 {
		return fmt.Sprintf("Level %d", level)
	}
	if m := regexp.MustCompile(`(?i)(Genius\s+Level\s+[1-3]|Level\s+[1-3])`).FindStringSubmatch(text); len(m) > 1 {
		return cleanText(m[1])
	}
	return ""
}

func discountedCategories(text string) []string {
	out := make([]string, 0)
	for _, cat := range []string{"stays", "car rentals", "cars", "attractions", "taxis", "flights"} {
		if containsAny(text, cat) && containsAny(text, "discount") {
			out = append(out, cat)
		}
	}
	return uniqueStrings(out, 0)
}

func settingValue(root *goquery.Selection, name string) string {
	var value string
	root.Find(`input, select, [data-testid], [class]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
		blob := strings.ToLower(strings.Join([]string{
			attrOrEmpty(s, "name"), attrOrEmpty(s, "id"), attrOrEmpty(s, "data-testid"), attrOrEmpty(s, "aria-label"), cleanText(s.Text()),
		}, " "))
		if !strings.Contains(blob, name) {
			return true
		}
		value = firstNonEmpty(attrOrEmpty(s, "value"), attrOrEmpty(s, "data-value"), cleanText(s.Text()))
		return value == ""
	})
	return value
}

func attrOrEmpty(s *goquery.Selection, attr string) string {
	value, _ := s.Attr(attr)
	return strings.TrimSpace(value)
}

func regexValue(text, pattern string) string {
	if m := regexp.MustCompile(pattern).FindStringSubmatch(text); len(m) > 1 {
		return cleanText(htmlUnescape(m[1]))
	}
	return ""
}

func htmlUnescape(s string) string {
	replacements := map[string]string{"\\u0026": "&", "&amp;": "&", `\"`: `"`, `\/`: "/"}
	for old, newValue := range replacements {
		s = strings.ReplaceAll(s, old, newValue)
	}
	return s
}

func queryValues(raw string) url.Values {
	u, err := url.Parse(raw)
	if err != nil {
		return url.Values{}
	}
	return u.Query()
}

func mapResults(root any) ([]any, bool) {
	current := root
	for _, key := range []string{"data", "searchQueries", "search", "results"} {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current = m[key]
	}
	results, ok := current.([]any)
	return results, ok
}

func recursiveString(v any, keys ...string) string {
	keySet := make(map[string]bool, len(keys))
	for _, key := range keys {
		keySet[strings.ToLower(key)] = true
	}
	var walk func(any) string
	walk = func(current any) string {
		switch x := current.(type) {
		case map[string]any:
			sorted := make([]string, 0, len(x))
			for key := range x {
				sorted = append(sorted, key)
			}
			sort.Strings(sorted)
			for _, key := range sorted {
				if keySet[strings.ToLower(key)] {
					if s := stringValue(x[key]); s != "" && s != "<nil>" {
						return s
					}
				}
			}
			for _, key := range sorted {
				if s := walk(x[key]); s != "" {
					return s
				}
			}
		case []any:
			for _, item := range x {
				if s := walk(item); s != "" {
					return s
				}
			}
		}
		return ""
	}
	return walk(v)
}

func recursiveNumber(v any, keys ...string) float64 {
	keySet := make(map[string]bool, len(keys))
	for _, key := range keys {
		keySet[strings.ToLower(key)] = true
	}
	var walk func(any) float64
	walk = func(current any) float64 {
		switch x := current.(type) {
		case map[string]any:
			sorted := make([]string, 0, len(x))
			for key := range x {
				sorted = append(sorted, key)
			}
			sort.Strings(sorted)
			for _, key := range sorted {
				if keySet[strings.ToLower(key)] {
					if n := floatValue(x[key]); n != 0 {
						return n
					}
				}
			}
			for _, key := range sorted {
				if n := walk(x[key]); n != 0 {
					return n
				}
			}
		case []any:
			for _, item := range x {
				if n := walk(item); n != 0 {
					return n
				}
			}
		}
		return 0
	}
	return walk(v)
}

func markerPrice(obj map[string]any) (float64, string) {
	priceText := recursiveString(obj, "price", "priceText", "displayPrice")
	price, currency := parseMoney(priceText)
	if price != 0 {
		return price, currency
	}
	price = recursiveNumber(obj, "price", "amount", "amountUnformatted", "value")
	currency = firstNonEmpty(recursiveString(obj, "currency", "currencyCode", "code"), currency)
	return price, currency
}
