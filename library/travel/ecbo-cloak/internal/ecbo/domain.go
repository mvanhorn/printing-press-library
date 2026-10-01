package ecbo

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var idPattern = regexp.MustCompile(`^(?:[A-Za-z0-9_-]{8,32}|[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12})$`)
var uuidPattern = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)
var nextData = regexp.MustCompile(`(?s)<script id="__NEXT_DATA__"[^>]*>(.*?)</script>`)

func ID(s string) (string, error) {
	if strings.HasPrefix(s, "https://") {
		u, e := url.Parse(s)
		if e != nil || u.Host != "cloak.ecbo.io" {
			return "", &Error{2, "id", "use ecbo source ID or canonical cloak.ecbo.io URL"}
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) < 2 || (parts[len(parts)-2] != "space" && parts[len(parts)-2] != "spaces") {
			return "", &Error{2, "id", "URL must identify an ecbo facility"}
		}
		s = parts[len(parts)-1]
	}
	if !idPattern.MatchString(s) {
		return "", &Error{2, "id", "expected an 8..32-character source ID or facility UUID"}
	}
	return s, nil
}
func (c *Client) Detail(ctx context.Context, id string) (map[string]any, error) {
	id, e := ID(id)
	if e != nil {
		return nil, e
	}
	if !uuidPattern.MatchString(id) {
		resolved, e := c.resolve(ctx, id)
		if e != nil {
			return nil, e
		}
		id = resolved
	}
	d, e := c.Request(ctx, "GET", "https://api.ecbo.io/api/web/spaces/"+id, nil, time.Hour)
	if e != nil {
		return nil, e
	}
	if d["space_id"] != id || str(d["name"]) == "" {
		return nil, &Error{5, "source_shape", "detail identity/name missing or mismatched"}
	}
	// Japanese identity is a lazy second detail call when another locale is requested.
	var ja any = d["name"]
	if c.Locale != "ja" {
		saved := c.Locale
		c.Locale = "ja"
		j, e := c.Request(ctx, "GET", "https://api.ecbo.io/api/web/spaces/"+id, nil, time.Hour)
		c.Locale = saved
		if e != nil {
			return nil, e
		}
		if j["space_id"] != id || str(j["name"]) == "" {
			return nil, &Error{5, "source_shape", "Japanese facility identity/name missing or mismatched"}
		}
		ja = j["name"]
	}
	return map[string]any{"id": id, "source_id": d["encrypted_id"], "name": d["name"], "name_ja": ja, "address": d["localed_address"], "latitude": d["latitude"], "longitude": d["longitude"], "facility_type": d["space_type"], "booking_url": "https://cloak.ecbo.io/" + websiteLocale(c.Locale) + "/spaces/" + id, "booking_locale": websiteLocale(c.Locale), "requested_locale": c.Locale, "listed": true, "confirmed_available_capacity": nil, "business_hours": d["business_hours"], "holiday_business_hours": d["holiday_business_hours"], "timezone": "Asia/Tokyo", "acceptance_cutoff": nil, "pickup_cutoff": nil, "cutoff_note": "Source exposes business hours; separate acceptance/pickup cutoffs are not published in structured data. Read restrictions and exact source validation.", "overnight_allowed": d["can_reserve_multiple_days"], "same_day_allowed": d["can_reserve_same_day"], "holiday_booking_allowed": d["can_reserve_on_holiday"], "listed_maximum_items": d["capacity"], "daily_prices": prices(d["prices"]), "bag_dimensions": d["sizes"], "bag_dimensions_unit": nil, "dimension_note": "Source does not declare units for structured size dimensions; bag category policy uses cm", "features": d["features"], "nearby_stations": d["nearby_stations"], "restrictions": map[string]any{"extra_info": d["extra_info"], "introduction": d["introduction"], "jr_east": d["jr_east_space_detail"], "jr_west": d["jr_west_space_detail"]}, "capacity_observation": map[string]any{"raw_ratios": d["availabilities"], "source_updated_at": d["fully_status_updated_at"], "meaning": "Uninterpreted source ratio; not remaining item count or guaranteed capacity"}, "policy": Policy()}, nil
}
func prices(v any) []any {
	out := []any{}
	for _, x := range array(v) {
		p := object(x)
		out = append(out, map[string]any{"size": p["size"], "amount": p["price"], "currency": "JPY", "basis": "per_item_per_storage_day", "day_definition": "source quote determines interval total; business/calendar-day wording differs across provider pages"})
	}
	return out
}
func Policy() map[string]any {
	return map[string]any{"small": "bag size; generally longest side under 45 cm", "large": "suitcase size; generally longest side 45 cm or over", "exact_45cm": "First-party language variants disagree; confirm category at facility booking page", "max_weight_kg": 20, "count_rule": "Each separate piece counts; facility-specific rules may require multiple slots for one long item", "price_basis": "per item per storage day, varies by facility; quote is authoritative for requested interval", "overnight_rule": "General stores count beyond closing as another storage day; 24-hour stores count crossing midnight as another day; source quote/validation governs exceptions", "restricted_items": "Valuables, hazards, perishables, plants/animals, liquids that leak, strong odors, medical items, waste and illegal items; facility can impose further restrictions", "sources": []string{"https://cloak.ecbo.io/en", "https://help.ecbo.io/en/articles/2018741-what-items-am-i-allowed-to-store", "https://help.ecbo.io/ja/articles/2823650", "https://help.ecbo.io/en/articles/2018680-what-is-the-pricing-for-ecbo-cloak"}}
}
func ParseTimes(from, to string) (time.Time, time.Time, error) {
	loc := time.FixedZone("Asia/Tokyo", 9*3600)
	a, e := time.ParseInLocation("2006-01-02T15:04", from, loc)
	if e != nil {
		return a, a, &Error{2, "from", "use YYYY-MM-DDTHH:MM in Asia/Tokyo"}
	}
	b, e := time.ParseInLocation("2006-01-02T15:04", to, loc)
	if e != nil {
		return a, b, &Error{2, "to", "use YYYY-MM-DDTHH:MM in Asia/Tokyo"}
	}
	if !b.After(a) || b.Sub(a) > 30*24*time.Hour {
		return a, b, &Error{2, "interval", "pickup must follow deposit within 30 days"}
	}
	return a, b, nil
}
func ValidateCounts(small, large int) error {
	if small < 0 || large < 0 || small+large < 1 || small+large > 50 {
		return &Error{2, "bags", "small/large counts must be nonnegative with total 1..50"}
	}
	return nil
}
func (c *Client) Offer(ctx context.Context, id, from, to string, small, large int) (map[string]any, error) {
	a, b, e := ParseTimes(from, to)
	if e != nil {
		return nil, e
	}
	if e = ValidateCounts(small, large); e != nil {
		return nil, e
	}
	d, e := c.Detail(ctx, id)
	if e != nil {
		return nil, e
	}
	payload := map[string]any{"space_id": d["id"], "from": a.Format("2006-01-02 15:04"), "to": b.Format("2006-01-02 15:04"), "reservation_items": map[string]int{"small": small, "large": large}}
	q, e := c.Request(ctx, "POST", "https://api.ecbo.io/api/web/reservations/price", payload, 0)
	if e != nil {
		return nil, e
	}
	if _, ok := q["price"].(float64); !ok || str(q["currency"]) == "" {
		return nil, &Error{5, "source_shape", "quote missing numeric price or currency"}
	}
	v, e := c.Request(ctx, "POST", "https://api.ecbo.io/api/web/reservations/validate", payload, 0)
	if e != nil {
		return nil, e
	}
	valid, ok := v["valid"].(bool)
	if !ok {
		return nil, &Error{5, "source_shape", "validation missing boolean valid"}
	}
	status := "source_rejected"
	if valid {
		status = "source_validated_at_observation"
	}
	return map[string]any{"facility": d, "request": payload, "timezone": "Asia/Tokyo", "quote": q, "validation": v, "availability": status, "confirmed_available_capacity": nil, "capacity_note": "Validation is a live observation for the requested interval/counts; no slots held, remaining count undisclosed, recheck on booking page", "booking_url": d["booking_url"]}, nil
}

type NearOptions struct {
	Lat, Lon, Radius            float64
	Query, From, To             string
	Small, Large, Limit, Offset int
}

func (o NearOptions) Validate() error {
	if math.IsNaN(o.Lat) || math.IsNaN(o.Lon) || math.IsInf(o.Lat, 0) || math.IsInf(o.Lon, 0) || o.Lat < 20 || o.Lat > 46 || o.Lon < 122 || o.Lon > 154 {
		return &Error{2, "coordinates", "supply finite Japan --lat 20..46 and --lon 122..154"}
	}
	if math.IsNaN(o.Radius) || math.IsInf(o.Radius, 0) || o.Radius <= 0 || o.Radius > 100 || o.Limit < 1 || o.Limit > 50 || o.Offset < 0 || o.Offset > 50 {
		return &Error{2, "bounds", "radius-km 0..100, limit 1..50, offset 0..50"}
	}
	if (o.From == "") != (o.To == "") {
		return &Error{2, "interval", "--from and --to must be supplied together"}
	}
	if o.From != "" {
		if _, _, e := ParseTimes(o.From, o.To); e != nil {
			return e
		}
		return ValidateCounts(o.Small, o.Large)
	}
	if o.Small != 0 || o.Large != 0 {
		return &Error{2, "bags", "bag counts require --from and --to"}
	}
	return nil
}
func (c *Client) Near(ctx context.Context, o NearOptions) (map[string]any, error) {
	if e := o.Validate(); e != nil {
		return nil, e
	}
	v := url.Values{"latitude": {fmt.Sprint(o.Lat)}, "longitude": {fmt.Sprint(o.Lon)}, "locale": {c.Locale}, "time_zone": {"Asia/Tokyo"}}
	if o.From != "" {
		a, b, _ := ParseTimes(o.From, o.To)
		v.Set("booked_from", a.Format(time.RFC3339))
		v.Set("booked_to", b.Format(time.RFC3339))
		v.Set("small_locker_num", fmt.Sprint(o.Small))
		v.Set("large_locker_num", fmt.Sprint(o.Large))
	}
	d, e := c.Request(ctx, "GET", "https://search.ecbo.io/api/v1/spaces?"+v.Encode(), nil, 5*time.Minute)
	if e != nil {
		return nil, e
	}
	hits, ok := d["hits"].(map[string]any)
	if !ok {
		return nil, &Error{5, "source_shape", "search hits envelope missing"}
	}
	raw, ok := hits["hits"].([]any)
	if !ok {
		return nil, &Error{5, "source_shape", "search hit list missing"}
	}
	if d["timed_out"] == true || number(object(d["_shards"])["failed"]) > 0 {
		return nil, &Error{5, "partial_source", "search timed out or source shards failed"}
	}
	total, totalOK := hits["total"].(float64)
	if !totalOK || total < 0 || math.IsNaN(total) || math.IsInf(total, 0) || total != math.Floor(total) || total < float64(len(raw)) || len(raw) > 50 {
		return nil, &Error{5, "source_shape", "invalid search total or oversized source window"}
	}
	all := []any{}
	for _, h := range raw {
		r := object(h)
		s := object(r["_source"])
		if str(s["name"]) == "" {
			return nil, &Error{5, "source_shape", "search hit missing source Japanese identity"}
		}
		id := str(r["_id"])
		if _, e := ID(id); e != nil {
			return nil, &Error{5, "source_shape", "search hit identity invalid"}
		}
		var lat, lon float64
		found := false
		for _, p := range array(object(r["fields"])["location"]) {
			p := object(p)
			var latOK, lonOK bool
			lat, latOK = p["lat"].(float64)
			lon, lonOK = p["lon"].(float64)
			if !latOK || !lonOK || lat < 20 || lat > 46 || lon < 122 || lon > 154 {
				return nil, &Error{5, "source_shape", "invalid source search coordinates"}
			}
			found = true
			break
		}
		if !found {
			return nil, &Error{5, "source_shape", "search coordinates missing"}
		}
		dist := distance(o.Lat, o.Lon, lat, lon)
		if dist > o.Radius {
			continue
		}
		if o.Query != "" && !strings.Contains(strings.ToLower(str(s["name"])+" "+str(s["en_name"])), strings.ToLower(o.Query)) {
			continue
		}
		name := s["name"]
		if c.Locale == "en" && str(s["en_name"]) != "" {
			name = s["en_name"]
		}
		all = append(all, map[string]any{"id": id, "name": name, "name_ja": s["name"], "latitude": lat, "longitude": lon, "distance_km": math.Round(dist*1000) / 1000, "booking_url": "https://cloak.ecbo.io/" + websiteLocale(c.Locale) + "/space/" + id, "canonical_uuid": nil, "url_note": "First-party legacy booking URL redirects to canonical UUID page; fetch facilities get to resolve lazily", "listed": true, "availability": availability(o.From), "confirmed_available_capacity": nil, "source_item_counts": map[string]any{"small": s["small_locker_num"], "large": s["large_locker_num"], "meaning": "raw search values; not confirmed remaining capacity"}, "daily_prices": []any{map[string]any{"size": "small", "amount": s["bag_size"], "currency": "JPY", "basis": "per_item_per_storage_day"}, map[string]any{"size": "large", "amount": s["suitcase_size"], "currency": "JPY", "basis": "per_item_per_storage_day"}}, "listed_hours": map[string]any{"from": clock(s["available_from"]), "to": clock(s["available_to"]), "is_24_hours": s["available_24_hour"]}, "acceptance_cutoff": nil, "pickup_cutoff": nil})
	}
	sort.SliceStable(all, func(i, j int) bool {
		return number(object(all[i])["distance_km"]) < number(object(all[j])["distance_km"])
	})
	return page(all, o.Limit, o.Offset, map[string]any{"source_total": hits["total"], "source_window_size": len(raw), "source_window_limit": 50, "query_scope": "local name filter within source nearest window; not nationwide text search", "origin": map[string]any{"latitude": o.Lat, "longitude": o.Lon}, "radius_km": o.Radius, "requested_from": nullable(o.From), "requested_to": nullable(o.To)}), nil
}
func availability(from string) string {
	if from != "" {
		return "source_filtered_match_unvalidated"
	}
	return "listed_only"
}
func page(all []any, limit, offset int, meta map[string]any) map[string]any {
	start := offset
	if start > len(all) {
		start = len(all)
	}
	end := start + limit
	if end > len(all) {
		end = len(all)
	}
	var next any
	if end < len(all) {
		next = end
	}
	meta["matched_in_window"] = len(all)
	meta["offset"] = offset
	meta["limit"] = limit
	meta["next_offset"] = next
	return map[string]any{"results": all[start:end], "pagination": meta}
}
func distance(a, b, c, d float64) float64 {
	const r = math.Pi / 180
	x := math.Sin((c - a) * r / 2)
	y := math.Sin((d - b) * r / 2)
	z := x*x + math.Cos(a*r)*math.Cos(c*r)*y*y
	if z > 1 {
		z = 1
	}
	return 6371 * 2 * math.Atan2(math.Sqrt(z), math.Sqrt(1-z))
}
func clock(v any) any {
	s := str(v)
	if len(s) >= 16 {
		return s[11:16]
	}
	return nil
}
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func str(v any) string            { s, _ := v.(string); return s }
func number(v any) float64        { f, _ := v.(float64); return f }
func array(v any) []any           { a, _ := v.([]any); return a }
func object(v any) map[string]any { m, _ := v.(map[string]any); return m }
func (c *Client) SaveInventory(v map[string]any) error {
	p := filepath.Join(c.CacheDir, "inventory.json")
	b, e := json.Marshal(map[string]any{"data": v, "refreshed_at": time.Now().UTC().Format(time.RFC3339), "observations": c.Meta.Observations})
	if e != nil {
		return e
	}
	if e := writeCache(p, b); e != nil {
		return &Error{10, "inventory", e.Error()}
	}
	return nil
}
func (c *Client) Inventory(query string, limit, offset int) (map[string]any, error) {
	if limit < 1 || limit > 50 || offset < 0 || offset > 50 {
		return nil, &Error{2, "bounds", "limit 1..50 and offset 0..50"}
	}
	b, e := readLimited(filepath.Join(c.CacheDir, "inventory.json"))
	if os.IsNotExist(e) {
		return map[string]any{"results": []any{}, "refreshed_at": nil, "coverage": "No local snapshot; run inventory refresh with coordinates"}, nil
	}
	if e != nil {
		return nil, &Error{10, "inventory", e.Error()}
	}
	if len(b) > maxBody {
		return nil, &Error{10, "inventory", "snapshot exceeds 2 MiB"}
	}
	var d map[string]any
	if json.Unmarshal(b, &d) != nil {
		return nil, &Error{10, "inventory", "snapshot is invalid JSON"}
	}
	data, ok := d["data"].(map[string]any)
	if !ok {
		return nil, &Error{10, "inventory", "snapshot data object missing"}
	}
	rows, ok := data["results"].([]any)
	if !ok || len(rows) > 50 {
		return nil, &Error{10, "inventory", "snapshot results must be an array of at most 50 facilities"}
	}
	if _, e := time.Parse(time.RFC3339, str(d["refreshed_at"])); e != nil {
		return nil, &Error{10, "inventory", "snapshot refreshed_at missing or invalid"}
	}
	for _, item := range rows {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, &Error{10, "inventory", "snapshot facility is not an object"}
		}
		if _, e := ID(str(m["id"])); e != nil || str(m["name"]) == "" || str(m["name_ja"]) == "" {
			return nil, &Error{10, "inventory", "snapshot facility identity missing or invalid"}
		}
	}

	all := []any{}
	for _, x := range array(data["results"]) {
		m := object(x)
		if query == "" || strings.Contains(strings.ToLower(str(m["name"])+" "+str(m["name_ja"])), strings.ToLower(query)) {
			all = append(all, x)
		}
	}
	out := page(all, limit, offset, map[string]any{"scope": object(data["pagination"])})
	out["refreshed_at"] = d["refreshed_at"]
	out["observations"] = d["observations"]
	out["freshness_note"] = "Explicit snapshot; never auto-refreshed, never current capacity"
	return out, nil
}

// WindowPage returns the requested display slice without changing the saved full inventory.
func WindowPage(v map[string]any, limit, offset int) map[string]any {
	return page(array(v["results"]), limit, offset, object(v["pagination"]))
}

// The first-party web frontend redirects zh-CN pages to Japanese; API language headers remain selectable.
func websiteLocale(locale string) string {
	if locale == "zh-CN" {
		return "ja"
	}
	return locale
}
