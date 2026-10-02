package traveloka

import (
	"context"
	"net/http"
	"reflect"
	"strconv"
	"testing"
)

func TestSimulatedCityResolverExcludesForeignLandmarksAndProperties(t *testing.T) {
	for _, tt := range []struct {
		kind      string
		limit     int
		ids       []string
		matched   int
		truncated bool
	}{
		{"city", 10, []string{"region", "geo-region", "geo-city", "geo-area"}, 4, false},
		{"city", 2, []string{"region", "geo-region"}, 4, true},
		{"all", 10, []string{"region", "geo-region", "geo-city", "geo-area", "milan-restaurant", "turin-restaurant", "hotel"}, 7, false},
	} {
		t.Run(tt.kind+"_"+strconv.Itoa(tt.limit), func(t *testing.T) {
			c := simulatedCoreClient(t)
			c.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
				coreRequestData(t, r)
				if r.URL.Path == airportSearchPath {
					return coreResponse(t, r, map[string]any{"sections": []any{map[string]any{"type": "SEARCH", "results": []any{map[string]any{"entityId": "region", "type": "REGION", "location": "Singapore"}}}}}), nil
				}
				return coreResponse(t, r, map[string]any{
					"autoCompleteContent": map[string]any{"rows": []any{}},
					"geoRegionContent":    map[string]any{"rows": []any{map[string]any{"id": "geo-region", "type": "GEO_REGION", "name": "Singapore"}}},
					"geoCityContent":      map[string]any{"rows": []any{map[string]any{"id": "geo-city", "type": "GEO_CITY", "name": "Singapore"}}},
					"geoAreaContent":      map[string]any{"rows": []any{map[string]any{"id": "geo-area", "type": "GEO_AREA", "name": "Singapore area"}}},
					"landmarkContent":     map[string]any{"rows": []any{map[string]any{"id": "milan-restaurant", "type": "LANDMARK", "name": "Singapore", "displayName": "Singapore Restaurant, Milan"}, map[string]any{"id": "turin-restaurant", "type": "LANDMARK", "name": "Singapore", "displayName": "Singapore Restaurant, Turin"}}},
					"hotelContent":        map[string]any{"rows": []any{map[string]any{"id": "hotel", "type": "HOTEL", "name": "M Hotel Singapore"}}},
				}), nil
			}))
			result, err := c.Resolve(context.Background(), "Singapore", tt.kind, Shopper{"SG", "en-SG", "SGD"}, tt.limit)
			if err != nil {
				t.Fatal(err)
			}
			ids := []string{}
			for _, location := range result.Locations {
				ids = append(ids, location.ID)
			}
			if !reflect.DeepEqual(ids, tt.ids) || result.Coverage["matched"] != tt.matched || result.Coverage["truncated"] != tt.truncated {
				t.Fatalf("city source kinds/cap changed: ids=%v coverage=%v", ids, result.Coverage)
			}
			if result.Locations[0].Type != "REGION" || result.Locations[0].Namespace != "airport" {
				t.Fatal("source REGION identity changed")
			}
		})
	}
}
