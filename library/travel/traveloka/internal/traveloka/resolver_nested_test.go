package traveloka

// Simulated regression for the delivered source's REGION -> AIRPORT children shape.
import (
	"context"
	"net/http"
	"reflect"
	"strconv"
	"testing"
)

func coreNestedSingaporeResolver() map[string]any {
	child := func(id, name, location string) any {
		return map[string]any{"entityId": id, "code": id, "iataCode": id, "type": "AIRPORT", "location": location, "country": "Singapore", "displayData": map[string]any{"title": name}, "children": []any{}}
	}
	return map[string]any{"sections": []any{map[string]any{"type": "RECOMMENDED", "results": []any{map[string]any{"entityId": "107493", "code": "SINA", "type": "REGION", "location": "Singapore", "country": "Singapore", "displayData": map[string]any{"title": "Singapore", "description": "Singapore"}, "children": []any{child("SIN", "Changi International Airport", "Singapore"), child("XSP", "Seletar Airport", "Singapore"), child("DPS", "Ngurah Rai Airport", "Bali")}}}}, map[string]any{"type": "POPULAR", "results": []any{child("BAD", "Singapore popular suggestion", "Singapore")}}}, "directory": []any{child("DIR", "Singapore directory", "Singapore")}}
}
func TestSimulatedCoreResolverNestedRegionAirportsAndTypes(t *testing.T) {
	for _, tt := range []struct {
		kind       string
		limit      int
		ids, types []string
		matched    int
		truncated  bool
	}{{"airport", 10, []string{"SIN", "XSP"}, []string{"AIRPORT", "AIRPORT"}, 2, false}, {"airport", 1, []string{"SIN"}, []string{"AIRPORT"}, 2, true}, {"city", 10, []string{"107493"}, []string{"REGION"}, 1, false}, {"all", 10, []string{"107493", "SIN", "XSP"}, []string{"REGION", "AIRPORT", "AIRPORT"}, 3, false}, {"all", 2, []string{"107493", "SIN"}, []string{"REGION", "AIRPORT"}, 3, true}} {
		t.Run(tt.kind+"_limit_"+strconv.Itoa(tt.limit), func(t *testing.T) {
			c := simulatedCoreClient(t)
			c.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
				d := coreRequestData(t, r)
				if d["query"] != "Singapore" {
					t.Fatal("source query differs")
				}
				if r.URL.Path == airportSearchPath {
					return coreResponse(t, r, coreNestedSingaporeResolver()), nil
				}
				if r.URL.Path == hotelAutocompletePath {
					return coreResponse(t, r, map[string]any{"autoCompleteContent": map[string]any{"rows": []any{}}}), nil
				}
				t.Fatal("unexpected resolver route")
				return nil, nil
			}))
			r, e := c.Resolve(context.Background(), "Singapore", tt.kind, Shopper{"SG", "en-SG", "SGD"}, tt.limit)
			if e != nil {
				t.Fatal(e)
			}
			ids, types := []string{}, []string{}
			for _, l := range r.Locations {
				ids = append(ids, l.ID)
				types = append(types, l.Type)
				if l.Namespace != "airport" {
					t.Fatal("source namespace changed")
				}
				if l.Type == "AIRPORT" && (l.Attributes["parent_id"] != "107493" || l.Attributes["parent_type"] != "REGION") {
					t.Fatal("source parent relationship lost")
				}
				if _, ok := l.Attributes["children"]; ok {
					t.Fatal("unbounded raw children escaped output cap")
				}
			}
			if !reflect.DeepEqual(ids, tt.ids) || !reflect.DeepEqual(types, tt.types) || r.Status != "success" || r.Coverage["matched"] != tt.matched || r.Coverage["truncated"] != tt.truncated || r.Ambiguous != (tt.matched > 1) {
				t.Fatalf("nested source type/order/cap mismatch: ids=%v types=%v coverage=%v", ids, types, r.Coverage)
			}
		})
	}
}
func TestSimulatedCoreResolverRecursiveTraversalIsDefensivelyBounded(t *testing.T) {
	root := map[string]any{"entityId": "root", "type": "REGION", "location": "Singapore", "displayData": map[string]any{"title": "Singapore"}}
	current := root
	for i := 0; i < 12; i++ {
		next := map[string]any{"entityId": "nested", "type": "REGION", "location": "Singapore"}
		current["children"] = []any{next}
		current = next
	}
	current["children"] = []any{map[string]any{"entityId": "too-deep", "type": "AIRPORT", "location": "Singapore"}}
	data := map[string]any{"sections": []any{map[string]any{"type": "RECOMMENDED", "results": []any{root}}}}
	locations := []Location{}
	scanned, truncated := walkRankedAirportResults(data, "Singapore", "all", func(l Location) { locations = append(locations, l) })
	if !truncated || scanned != 9 || len(locations) != 9 {
		t.Fatal("source nested depth was not bounded")
	}
	for _, l := range locations {
		if l.ID == "too-deep" {
			t.Fatal("over-depth source airport returned")
		}
	}
	c := simulatedCoreClient(t)
	c.SetHTTPTransport(simulatedTransport(func(r *http.Request) (*http.Response, error) {
		coreRequestData(t, r)
		return coreResponse(t, r, data), nil
	}))
	resolution, err := c.Resolve(context.Background(), "Singapore", "airport", Shopper{"SG", "en-SG", "SGD"}, 10)
	if err != nil || resolution.Status != "incomplete" || resolution.Coverage["airport_traversal_truncated"] != true || resolution.Coverage["matched_is_lower_bound"] != true {
		t.Fatal("bounded traversal was presented as completed empty inventory")
	}

	children := make([]any, 2100)
	for i := range children {
		children[i] = map[string]any{"type": "AIRPORT", "entityId": "source-airport", "location": "Singapore"}
	}
	root["children"] = children
	scanned, truncated = walkRankedAirportResults(data, "Singapore", "airport", func(Location) {})
	if !truncated || scanned != 2000 {
		t.Fatal("source traversal node budget ignored")
	}
}
