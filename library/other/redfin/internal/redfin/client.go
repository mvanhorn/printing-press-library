// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package redfin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// StripStingrayPrefix strips the literal `{}&&` CSRF prevention prefix from a
// Stingray JSON response if present. The shared HTTP client also sanitizes
// this prefix; this helper is kept so direct-fed bytes (tests, fixtures,
// alternate transports) parse correctly without leaning on the client.
func StripStingrayPrefix(data []byte) []byte {
	if len(data) >= 4 && bytes.HasPrefix(data, []byte("{}&&")) {
		return data[4:]
	}
	return data
}

// stingrayEnvelope is the shared shape every Stingray API response wraps its
// payload in. We unmarshal into this once, then descend into Payload per
// endpoint.
type stingrayEnvelope struct {
	Version      int             `json:"version"`
	ErrorMessage string          `json:"errorMessage"`
	ResultCode   int             `json:"resultCode"`
	Payload      json.RawMessage `json:"payload"`
}

// BuildSearchParams projects SearchOptions into the query-param map the
// `/stingray/api/gis` endpoint expects. Zero-valued fields are dropped.
func BuildSearchParams(opts SearchOptions) map[string]string {
	p := map[string]string{
		"al": "1",
		"v":  "8",
	}
	if opts.RegionID != 0 {
		p["region_id"] = strconv.FormatInt(opts.RegionID, 10)
	}
	if opts.RegionType != 0 {
		p["region_type"] = strconv.Itoa(opts.RegionType)
	}
	if opts.Status != 0 {
		p["status"] = strconv.Itoa(opts.Status)
	}
	if opts.SoldFlags != "" {
		p["sf"] = opts.SoldFlags
	}
	if len(opts.UIPropertyTypes) > 0 {
		parts := make([]string, len(opts.UIPropertyTypes))
		for i, t := range opts.UIPropertyTypes {
			parts[i] = strconv.Itoa(t)
		}
		p["uipt"] = strings.Join(parts, ",")
	}
	if opts.BedsMin > 0 {
		p["min_beds"] = formatFloat(opts.BedsMin)
	}
	if opts.BathsMin > 0 {
		p["min_baths"] = formatFloat(opts.BathsMin)
	}
	if opts.PriceMin > 0 {
		p["min_price"] = strconv.Itoa(opts.PriceMin)
	}
	if opts.PriceMax > 0 {
		p["max_price"] = strconv.Itoa(opts.PriceMax)
	}
	if opts.SqftMin > 0 {
		p["min_sqft"] = strconv.Itoa(opts.SqftMin)
	}
	if opts.SqftMax > 0 {
		p["max_sqft"] = strconv.Itoa(opts.SqftMax)
	}
	if opts.YearMin > 0 {
		p["min_year_built"] = strconv.Itoa(opts.YearMin)
	}
	if opts.YearMax > 0 {
		p["max_year_built"] = strconv.Itoa(opts.YearMax)
	}
	if opts.LotMin > 0 {
		p["min_lot_size"] = strconv.Itoa(opts.LotMin)
	}
	if opts.SchoolsMin > 0 {
		p["min_school_rating"] = strconv.Itoa(opts.SchoolsMin)
	}
	if opts.Polygon != "" {
		p["poly"] = opts.Polygon
	}
	num := opts.NumHomes
	if num <= 0 {
		num = 50
	}
	if num > 350 {
		num = 350
	}
	p["num_homes"] = strconv.Itoa(num)
	page := opts.PageNumber
	if page <= 0 {
		page = 1
	}
	p["page_number"] = strconv.Itoa(page)
	if opts.Sort != "" {
		p["sort"] = opts.Sort
	}
	return p
}

func formatFloat(f float64) string {
	if f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// gisHome is the slice of /api/gis payload.homes[] we map onto Listing.
// Stingray's gis response uses Map/Value boxes for typed scalars (e.g.
// `{"price": {"value": 499000, "level": 1}}`); we tolerate both raw scalars
// and the wrapper shape via valueOrInt / valueOrFloat / valueOrString.
type gisHome struct {
	URL        json.RawMessage `json:"url"`
	PropertyID int64           `json:"propertyId"`
	ListingID  int64           `json:"listingId"`
	MLS        json.RawMessage `json:"mlsId"`
	MLSStatus  json.RawMessage `json:"mlsStatus"`
	Price      json.RawMessage `json:"price"`
	Beds       json.RawMessage `json:"beds"`
	Baths      json.RawMessage `json:"baths"`
	Sqft       json.RawMessage `json:"sqFt"`
	YearBuilt  json.RawMessage `json:"yearBuilt"`
	HOA        json.RawMessage `json:"hoa"`
	DOM        json.RawMessage `json:"dom"`
	ListedAt   json.RawMessage `json:"timeOnRedfin"`
	Photos     json.RawMessage `json:"photos"`
	StreetLine json.RawMessage `json:"streetLine"`
	City       json.RawMessage `json:"city"`
	State      json.RawMessage `json:"state"`
	PostalCode json.RawMessage `json:"postalCode"`
	Latitude   json.RawMessage `json:"latitude"`
	Longitude  json.RawMessage `json:"longitude"`
	Latlong    json.RawMessage `json:"latLong"`
}

// ParseSearchResponse decodes a /stingray/api/gis response (after prefix
// strip) into a slice of Listings. Each home in payload.homes[] becomes a
// Listing populated with the gis-endpoint fields (URL, PropertyID, MLS,
// Status, Address, Price, Beds, Baths, Sqft, YearBuilt, HOA, DOM, ListedAt,
// Photos[0:1]). Missing fields stay zero-valued; the caller can compose the
// detail endpoints to fill them in.
func ParseSearchResponse(data []byte) ([]Listing, error) {
	data = StripStingrayPrefix(data)
	var env stingrayEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("decoding stingray envelope: %w", err)
	}
	if env.ErrorMessage != "" && env.ErrorMessage != "Success" && env.ResultCode != 0 {
		// Some Redfin paths carry "Success" in errorMessage; anything else with a
		// non-zero resultCode is an actual error.
		if env.ResultCode != 200 {
			return nil, fmt.Errorf("stingray error: %s (code %d)", env.ErrorMessage, env.ResultCode)
		}
	}
	if len(env.Payload) == 0 {
		return nil, nil
	}
	var payload struct {
		Homes []gisHome `json:"homes"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return nil, fmt.Errorf("decoding gis payload: %w", err)
	}
	out := make([]Listing, 0, len(payload.Homes))
	for _, h := range payload.Homes {
		l := Listing{
			URL:        valueOrString(h.URL),
			PropertyID: h.PropertyID,
			ListingID:  h.ListingID,
			MLS:        valueOrString(h.MLS),
			Status:     normalizeStatus(valueOrString(h.MLSStatus)),
			Price:      valueOrInt(h.Price),
			Beds:       valueOrFloat(h.Beds),
			Baths:      valueOrFloat(h.Baths),
			Sqft:       valueOrInt(h.Sqft),
			YearBuilt:  valueOrInt(h.YearBuilt),
			HOA:        valueOrInt(h.HOA),
			DOM:        valueOrInt(h.DOM),
			ListedAt:   valueOrString(h.ListedAt),
		}
		l.Address = Address{
			Street:     valueOrString(h.StreetLine),
			City:       valueOrString(h.City),
			State:      valueOrString(h.State),
			PostalCode: valueOrString(h.PostalCode),
			Latitude:   valueOrFloat(h.Latitude),
			Longitude:  valueOrFloat(h.Longitude),
		}
		// gis sometimes wraps lat/long in latLong:{value:{latitude,longitude}}
		if l.Address.Latitude == 0 && len(h.Latlong) > 0 {
			lat, lng := extractLatLong(h.Latlong)
			l.Address.Latitude = lat
			l.Address.Longitude = lng
		}
		// Photos: gis returns either a flat string list or an object with
		// photoUrls. We only emit the first photo from search results.
		if photo := firstPhoto(h.Photos); photo != "" {
			l.Photos = []string{photo}
		}
		out = append(out, l)
	}
	return out, nil
}

// initialInfoPayload is the slice of /home/details/initialInfo payload we use.
type initialInfoPayload struct {
	PropertyID int64  `json:"propertyId"`
	ListingID  int64  `json:"listingId"`
	URL        string `json:"url"`
	Address    struct {
		StreetLine string  `json:"streetLine"`
		City       string  `json:"city"`
		State      string  `json:"state"`
		PostalCode string  `json:"postalCode"`
		Latitude   float64 `json:"latitude"`
		Longitude  float64 `json:"longitude"`
	} `json:"streetAddress"`
}

// aboveTheFoldPayload pulls headline price + photos.
type aboveTheFoldPayload struct {
	MainHouseInfo struct {
		Price          json.RawMessage `json:"price"`
		Beds           json.RawMessage `json:"beds"`
		Baths          json.RawMessage `json:"baths"`
		Sqft           json.RawMessage `json:"sqFt"`
		PropertyType   string          `json:"propertyType"`
		MLSDisplayName string          `json:"mlsDisplayName"`
		ListingID      int64           `json:"listingId"`
		PropertyID     int64           `json:"propertyId"`
		Status         string          `json:"mlsStatus"`
		ListedAt       string          `json:"listingAddedDate"`
		LastSaleData   struct {
			LastSoldDate string `json:"lastSoldDate"`
		} `json:"lastSaleData"`
		HOA json.RawMessage `json:"hoaDues"`
		DOM json.RawMessage `json:"dom"`
	} `json:"mainHouseInfo"`
	Media struct {
		Photos []struct {
			PhotoURLs struct {
				FullScreenPhotoURL string `json:"fullScreenPhotoUrl"`
				NonFullScreen      string `json:"nonFullScreenPhotoUrl"`
			} `json:"photoUrls"`
		} `json:"photos"`
	} `json:"mediaBrowserInfoBySourceId"`
}

// belowTheFoldPayload pulls amenities, history, schools, AVM, lot, year.
type belowTheFoldPayload struct {
	PublicRecordsInfo struct {
		BasicInfo struct {
			YearBuilt        int    `json:"yearBuilt"`
			LotSquareFootage int    `json:"lotSquareFootage"`
			PropertyType     string `json:"propertyType"`
		} `json:"basicInfo"`
	} `json:"publicRecordsInfo"`
	PropertyHistoryInfo struct {
		Events []struct {
			EventDate        string          `json:"eventDate"`
			EventDescription string          `json:"eventDescription"`
			Price            json.RawMessage `json:"price"`
			Source           string          `json:"source"`
		} `json:"events"`
	} `json:"propertyHistoryInfo"`
	SchoolsAndDistrictsInfo struct {
		Schools []struct {
			Name       string  `json:"name"`
			GradeRange string  `json:"gradeRange"`
			Rating     float64 `json:"greatSchoolsRating"`
		} `json:"servingThisHomeSchools"`
	} `json:"schoolsAndDistrictsInfo"`
	AVMInfo struct {
		PredictedValue json.RawMessage `json:"predictedValue"`
	} `json:"avmInfo"`
}

// ParseListingDetail merges initialInfo + aboveTheFold + belowTheFold (each
// already prefix-stripped) into a Listing. Any of the three may be empty —
// we populate as much as we can. URL is taken from initialInfo if present,
// otherwise the caller is expected to provide it.
func ParseListingDetail(initial, above, below []byte) (Listing, error) {
	var l Listing

	if len(initial) > 0 {
		var env stingrayEnvelope
		if err := json.Unmarshal(StripStingrayPrefix(initial), &env); err == nil && len(env.Payload) > 0 {
			var p initialInfoPayload
			if err := json.Unmarshal(env.Payload, &p); err == nil {
				l.PropertyID = p.PropertyID
				l.ListingID = p.ListingID
				l.URL = p.URL
				l.Address = Address{
					Street:     p.Address.StreetLine,
					City:       p.Address.City,
					State:      p.Address.State,
					PostalCode: p.Address.PostalCode,
					Latitude:   p.Address.Latitude,
					Longitude:  p.Address.Longitude,
				}
			}
		}
	}

	if len(above) > 0 {
		var env stingrayEnvelope
		if err := json.Unmarshal(StripStingrayPrefix(above), &env); err == nil && len(env.Payload) > 0 {
			var p aboveTheFoldPayload
			if err := json.Unmarshal(env.Payload, &p); err == nil {
				if p.MainHouseInfo.PropertyID != 0 {
					l.PropertyID = p.MainHouseInfo.PropertyID
				}
				if p.MainHouseInfo.ListingID != 0 {
					l.ListingID = p.MainHouseInfo.ListingID
				}
				if p.MainHouseInfo.MLSDisplayName != "" {
					l.MLS = p.MainHouseInfo.MLSDisplayName
				}
				if v := normalizeStatus(p.MainHouseInfo.Status); v != "" {
					l.Status = v
				}
				if v := valueOrInt(p.MainHouseInfo.Price); v != 0 {
					l.Price = v
				}
				if v := valueOrFloat(p.MainHouseInfo.Beds); v != 0 {
					l.Beds = v
				}
				if v := valueOrFloat(p.MainHouseInfo.Baths); v != 0 {
					l.Baths = v
				}
				if v := valueOrInt(p.MainHouseInfo.Sqft); v != 0 {
					l.Sqft = v
				}
				if v := valueOrInt(p.MainHouseInfo.HOA); v != 0 {
					l.HOA = v
				}
				if v := valueOrInt(p.MainHouseInfo.DOM); v != 0 {
					l.DOM = v
				}
				if p.MainHouseInfo.PropertyType != "" {
					l.PropertyType = p.MainHouseInfo.PropertyType
				}
				if p.MainHouseInfo.ListedAt != "" {
					l.ListedAt = p.MainHouseInfo.ListedAt
				}
				if p.MainHouseInfo.LastSaleData.LastSoldDate != "" {
					l.SoldAt = p.MainHouseInfo.LastSaleData.LastSoldDate
				}
				for _, ph := range p.Media.Photos {
					if u := ph.PhotoURLs.FullScreenPhotoURL; u != "" {
						l.Photos = append(l.Photos, u)
					} else if u := ph.PhotoURLs.NonFullScreen; u != "" {
						l.Photos = append(l.Photos, u)
					}
				}
			}
		}
	}

	if len(below) > 0 {
		var env stingrayEnvelope
		if err := json.Unmarshal(StripStingrayPrefix(below), &env); err == nil && len(env.Payload) > 0 {
			var p belowTheFoldPayload
			if err := json.Unmarshal(env.Payload, &p); err == nil {
				if l.YearBuilt == 0 {
					l.YearBuilt = p.PublicRecordsInfo.BasicInfo.YearBuilt
				}
				if l.LotSize == 0 {
					l.LotSize = p.PublicRecordsInfo.BasicInfo.LotSquareFootage
				}
				if l.PropertyType == "" && p.PublicRecordsInfo.BasicInfo.PropertyType != "" {
					l.PropertyType = p.PublicRecordsInfo.BasicInfo.PropertyType
				}
				for _, ev := range p.PropertyHistoryInfo.Events {
					l.PriceHistory = append(l.PriceHistory, PriceHistoryEvent{
						Date:   ev.EventDate,
						Event:  ev.EventDescription,
						Price:  valueOrInt(ev.Price),
						Source: ev.Source,
					})
				}
				for _, s := range p.SchoolsAndDistrictsInfo.Schools {
					l.Schools = append(l.Schools, School{
						Name:   s.Name,
						Grades: s.GradeRange,
						Rating: s.Rating,
					})
				}
				if v := valueOrInt(p.AVMInfo.PredictedValue); v != 0 {
					l.Estimate = v
				}
			}
		}
	}

	return l, nil
}

// trendsPayload models the actual flat aggregate-trends response. Stingray
// returns a single snapshot of the configured period, not a per-month array.
// Money/percent fields arrive as strings ("$500K", "-2.3%") that we coerce
// to numbers via parseMoneyOrPercent. Counts arrive as numeric strings.
type trendsPayload struct {
	MedianListPrice    string `json:"medianListPrice"`
	MedianSalePrice    string `json:"medianSalePrice"`
	MedianSalePerSqFt  string `json:"medianSalePerSqFt"`
	MedianListPerSqFt  string `json:"medianListPerSqFt"`
	AvgDaysOnMarket    string `json:"avgDaysOnMarket"`
	MedianDom          string `json:"medianDom"`
	NumHomesOnMarket   string `json:"numHomesOnMarket"`
	AvgNumOffers       string `json:"avgNumOffers"`
	YoySalePerSqft     string `json:"yoySalePerSqft"`
	YoyMedianSalePrice string `json:"yoyMedianSalePrice"`
	HomeCountByType    []struct {
		Type  int    `json:"type"`
		Value string `json:"value"`
	} `json:"homeCountByPropertyType"`
}

// parseMoneyOrPercent coerces "$500K", "$1.2M", "-2.3%", "44", "1,250" to a
// float64. Returns 0 for unparseable input so empty fields do not pollute the
// output. Suffix multipliers: K=1e3, M=1e6, B=1e9.
func parseMoneyOrPercent(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimLeft(s, "-+$")
	s = strings.ReplaceAll(s, ",", "")
	mul := 1.0
	switch {
	case strings.HasSuffix(s, "%"):
		s = strings.TrimSuffix(s, "%")
	case strings.HasSuffix(s, "K"), strings.HasSuffix(s, "k"):
		s = s[:len(s)-1]
		mul = 1e3
	case strings.HasSuffix(s, "M"), strings.HasSuffix(s, "m"):
		s = s[:len(s)-1]
		mul = 1e6
	case strings.HasSuffix(s, "B"), strings.HasSuffix(s, "b"):
		s = s[:len(s)-1]
		mul = 1e9
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	if neg {
		n = -n
	}
	return n * mul
}

// ParseTrendsResponse decodes an aggregate-trends response into long-format
// rows. Stingray returns a single snapshot for the configured period; we
// emit one row per non-zero metric with Month set to "current" (the snapshot
// period is captured by the request URL, not the payload).
func ParseTrendsResponse(data []byte, regionLabel string, regionID int64) ([]RegionTrendPoint, error) {
	data = StripStingrayPrefix(data)
	var env stingrayEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("decoding stingray envelope: %w", err)
	}
	if len(env.Payload) == 0 {
		return nil, nil
	}
	var p trendsPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return nil, fmt.Errorf("decoding trends payload: %w", err)
	}
	var out []RegionTrendPoint
	emit := func(metric string, v float64) {
		if v == 0 {
			return
		}
		out = append(out, RegionTrendPoint{
			Region:   regionLabel,
			RegionID: regionID,
			Month:    "current",
			Metric:   metric,
			Value:    v,
		})
	}
	emit("median_list", parseMoneyOrPercent(p.MedianListPrice))
	emit("median_sale", parseMoneyOrPercent(p.MedianSalePrice))
	emit("median_sale_per_sqft", parseMoneyOrPercent(p.MedianSalePerSqFt))
	emit("median_list_per_sqft", parseMoneyOrPercent(p.MedianListPerSqFt))
	emit("avg_days_on_market", parseMoneyOrPercent(p.AvgDaysOnMarket))
	emit("median_dom", parseMoneyOrPercent(p.MedianDom))
	emit("active_count", parseMoneyOrPercent(p.NumHomesOnMarket))
	emit("avg_num_offers", parseMoneyOrPercent(p.AvgNumOffers))
	emit("yoy_sale_per_sqft_pct", parseMoneyOrPercent(p.YoySalePerSqft))
	emit("yoy_median_sale_pct", parseMoneyOrPercent(p.YoyMedianSalePrice))
	return out, nil
}

// valueOrInt unmarshals either a raw number or a Stingray Map box {"value": N}
// into an int.
func valueOrInt(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	// raw scalar
	var n float64
	if err := json.Unmarshal(raw, &n); err == nil {
		return int(n)
	}
	// boxed: {"value": N, "level": L}
	var box struct {
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(raw, &box); err == nil && len(box.Value) > 0 {
		if err := json.Unmarshal(box.Value, &n); err == nil {
			return int(n)
		}
	}
	return 0
}

func valueOrFloat(raw json.RawMessage) float64 {
	if len(raw) == 0 {
		return 0
	}
	var n float64
	if err := json.Unmarshal(raw, &n); err == nil {
		return n
	}
	var box struct {
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(raw, &box); err == nil && len(box.Value) > 0 {
		if err := json.Unmarshal(box.Value, &n); err == nil {
			return n
		}
	}
	return 0
}

func valueOrString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var box struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(raw, &box); err == nil {
		return box.Value
	}
	return ""
}

// extractLatLong pulls latitude/longitude from a latLong wrapper of either
// shape: {"value": {"latitude": ..., "longitude": ...}} or
// {"latitude": ..., "longitude": ...}.
func extractLatLong(raw json.RawMessage) (float64, float64) {
	if len(raw) == 0 {
		return 0, 0
	}
	var direct struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	}
	if err := json.Unmarshal(raw, &direct); err == nil && (direct.Latitude != 0 || direct.Longitude != 0) {
		return direct.Latitude, direct.Longitude
	}
	var box struct {
		Value struct {
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"value"`
	}
	if err := json.Unmarshal(raw, &box); err == nil {
		return box.Value.Latitude, box.Value.Longitude
	}
	return 0, 0
}

// firstPhoto returns the first usable photo URL from gis's photo block. The
// shape varies across Stingray rollouts, so we tolerate the common ones.
func firstPhoto(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// String list
	var list []string
	if err := json.Unmarshal(raw, &list); err == nil && len(list) > 0 {
		return list[0]
	}
	// Object with items[].photoUrls.fullScreenPhotoUrl
	var obj struct {
		Items []struct {
			PhotoURLs struct {
				FullScreenPhotoURL string `json:"fullScreenPhotoUrl"`
				NonFullScreen      string `json:"nonFullScreenPhotoUrl"`
			} `json:"photoUrls"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil && len(obj.Items) > 0 {
		if u := obj.Items[0].PhotoURLs.FullScreenPhotoURL; u != "" {
			return u
		}
		if u := obj.Items[0].PhotoURLs.NonFullScreen; u != "" {
			return u
		}
	}
	return ""
}

// normalizeStatus translates Redfin's mlsStatus codes into the human labels we
// surface in Listing.Status. Falls back to the raw value when unknown.
func normalizeStatus(s string) string {
	switch strings.ToLower(s) {
	case "active":
		return "Active"
	case "sold":
		return "Sold"
	case "pending":
		return "Pending"
	case "coming soon", "coming-soon":
		return "Coming Soon"
	case "contingent":
		return "Contingent"
	}
	return s
}
