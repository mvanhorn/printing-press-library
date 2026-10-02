// Source RPCs delegate to Client.Post in http.go, which uses cliutil.AdaptiveLimiter
// and returns typed cliutil.RateLimitError failures; these workflows preserve them.
package traveloka

import (
	"context"
	"strings"
	"time"
)

// Location retains the source namespace and type; city and airport IDs are distinct.
type Location struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Namespace   string         `json:"namespace"`
	Name        string         `json:"name"`
	DisplayName string         `json:"display_name"`
	Country     string         `json:"country"`
	Coordinates map[string]any `json:"coordinates"`
	Attributes  map[string]any `json:"attributes"`
}
type Resolution struct {
	Query       string         `json:"query"`
	Kind        string         `json:"kind"`
	Shopper     Shopper        `json:"shopper"`
	RetrievedAt string         `json:"retrieved_at"`
	Status      string         `json:"status"`
	Ambiguous   bool           `json:"ambiguous"`
	Locations   []Location     `json:"locations"`
	Coverage    map[string]any `json:"coverage"`
}

// Resolve reads source-ranked query groups, never directory or popular suggestions.
func (c *Client) Resolve(ctx context.Context, query, kind string, shop Shopper, limit int) (*Resolution, error) {
	query = strings.TrimSpace(query)
	if err := ValidateShopper(shop); err != nil {
		return nil, err
	}
	if query == "" || len(query) > 200 || limit < 0 {
		return nil, apiError("INVALID_INPUT", "a nonempty bounded location query and nonnegative limit are required", 0, false)
	}
	if kind != "airport" && kind != "city" && kind != "property" && kind != "all" {
		return nil, apiError("INVALID_INPUT", "resolution kind must be airport, city, property or all", 0, false)
	}
	cap := boundedLimit(limit, 10, 50)
	r := &Resolution{Query: query, Kind: kind, Shopper: shop, RetrievedAt: time.Now().UTC().Format(time.RFC3339Nano), Locations: []Location{}, Coverage: map[string]any{"limit": cap, "ordering": "source_ranked_query_groups"}}
	seen := map[string]bool{}
	matched := 0
	add := func(l Location) {
		key := l.Namespace + ":" + l.Type + ":" + l.ID
		if l.ID == "" || seen[key] {
			return
		}
		seen[key] = true
		matched++
		if len(r.Locations) < cap {
			r.Locations = append(r.Locations, l)
		}
	}
	if kind == "airport" || kind == "city" || kind == "all" {
		d, err := c.TemplateData(airportSearchPath)
		if err != nil {
			return nil, err
		}
		d["query"] = query
		d["frequentAirport"] = []any{}
		d["originAirport"] = ""
		resp, err := c.Post(ctx, airportSearchPath, d, shop)
		if err != nil {
			return nil, err
		}
		data, err := responseData(resp)
		if err != nil {
			return nil, err
		}
		if _, ok := data["sections"].([]any); !ok {
			return nil, apiError("MALFORMED_RESPONSE", "airport resolver has no ranked sections", 200, false)
		}
		scanned, truncated := walkRankedAirportResults(data, query, kind, add)
		r.Coverage["airport_nodes_scanned"] = scanned
		r.Coverage["airport_traversal_truncated"] = truncated

	}
	if kind == "city" || kind == "property" || kind == "all" {
		d, err := c.TemplateData(hotelAutocompletePath)
		if err != nil {
			return nil, err
		}
		d["query"] = query
		d["experimentContext"] = map[string]any{"mapParamKeyToVariant": map[string]any{"varAutocompleteLogic": "control"}}
		resp, err := c.Post(ctx, hotelAutocompletePath, d, shop)
		if err != nil {
			return nil, err
		}
		data, err := responseData(resp)
		if err != nil {
			return nil, err
		}
		groups := []string{"autoCompleteContent", "geoRegionContent", "geoCityContent", "geoAreaContent", "landmarkContent", "hotelContent"}
		groupFound := false
		for _, group := range groups {
			obj := sourceObject(data[group])
			if _, ok := obj["rows"].([]any); ok {
				groupFound = true
			}
			for _, v := range sourceList(obj["rows"]) {
				o := sourceObject(v)
				t := sourceString(o["type"])
				if t == "" {
					continue
				}
				property := t == "HOTEL"
				if kind == "property" && !property {
					continue
				}
				if kind == "city" && t != "GEO_REGION" && t != "GEO_CITY" && t != "GEO_AREA" {
					continue
				}
				if !property && t != "GEO_REGION" && t != "GEO_CITY" && t != "GEO_AREA" && t != "LANDMARK" {
					continue
				}
				if !locationRelevant(query, o["id"], o["name"], o["displayName"], o["globalName"], o["additionalInfo"]) {
					continue
				}
				add(Location{ID: sourceString(o["id"]), Type: t, Namespace: "hotel", Name: sourceString(o["name"]), DisplayName: sourceString(o["displayName"]), Country: sourceString(o["country"]), Coordinates: sourceObject(o["geoLocation"]), Attributes: sourceFields(o, "geoId", "accommodationType", "additionalInfo", "globalName", "landmarkType", "localeDisplayType", "matchingScore", "numHotels", "numberOfHotel", "targetUrl")})
			}
		}
		if !groupFound {
			return nil, apiError("MALFORMED_RESPONSE", "hotel resolver has no categorized query rows", 200, false)
		}
	}
	r.Ambiguous = matched > 1
	r.Status = "success"
	if matched == 0 {
		r.Status = "no_inventory"
	}
	r.Coverage["matched"] = matched
	r.Coverage["returned"] = len(r.Locations)
	r.Coverage["truncated"] = matched > cap
	if r.Coverage["airport_traversal_truncated"] == true {
		r.Status = "incomplete"
		r.Coverage["truncated"] = true
		r.Coverage["matched_is_lower_bound"] = true
	}
	return r, nil
}

func locationRelevant(query string, values ...any) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return false
	}
	parts := []string{}
	for _, v := range values {
		if s := strings.ToLower(sourceString(v)); s != "" {
			if s == query || strings.Contains(s, query) {
				return true
			}
			parts = append(parts, s)
		}
	}
	all := strings.Join(parts, " ")
	for _, word := range strings.Fields(query) {
		if !strings.Contains(all, word) {
			return false
		}
	}
	return len(parts) > 0
}

// walkRankedAirportResults follows source child relationships in depth-first source
// order. Each child must independently match; its parent's relevance is not inherited.
func walkRankedAirportResults(data map[string]any, query, kind string, add func(Location)) (int, bool) {
	const maxNodes, maxDepth = 2000, 8
	scanned, truncated := 0, false
	var walk func(any, int, string, string)
	walk = func(raw any, depth int, parentID, parentType string) {
		if scanned >= maxNodes || depth > maxDepth {
			truncated = true
			return
		}
		o := sourceObject(raw)
		if o == nil {
			return
		}
		scanned++
		t := sourceString(o["type"])
		id := firstString(o["entityId"], o["code"])
		display := sourceObject(o["displayData"])
		allowed := t == "AIRPORT" || t == "CITY" || t == "REGION"
		if kind == "airport" {
			allowed = t == "AIRPORT"
		}
		if kind == "city" {
			allowed = t == "CITY" || t == "REGION"
		}
		children := sourceList(o["children"])
		if allowed && locationRelevant(query, o["code"], o["entityId"], o["iataCode"], o["location"], display["title"], display["description"]) {
			attributes := sourceFields(o, "code", "entityId", "areaCode", "iataCode", "icaoCode", "location", "displayData", "score", "airportType", "childrenSectionTitle")
			attributes["children_count"] = len(children)
			if parentID != "" {
				attributes["parent_id"] = parentID
				attributes["parent_type"] = parentType
			}
			add(Location{ID: id, Type: t, Namespace: "airport", Name: firstString(display["title"], o["location"]), DisplayName: sourceString(display["title"]), Country: sourceString(o["country"]), Coordinates: sourceObject(o["geoLocation"]), Attributes: attributes})
		}
		for _, child := range children {
			walk(child, depth+1, id, t)
			if scanned >= maxNodes {
				truncated = true
				break
			}
		}
	}
	for _, raw := range sourceList(data["sections"]) {
		section := sourceObject(raw)
		typ := strings.ToUpper(sourceString(section["type"]))
		if strings.Contains(typ, "POPULAR") || strings.Contains(typ, "RECENT") || strings.Contains(typ, "FREQUENT") {
			continue
		}
		for _, result := range sourceList(section["results"]) {
			walk(result, 0, "", "")
			if scanned >= maxNodes {
				truncated = true
				break
			}
		}
		if scanned >= maxNodes {
			break
		}
	}
	return scanned, truncated
}
