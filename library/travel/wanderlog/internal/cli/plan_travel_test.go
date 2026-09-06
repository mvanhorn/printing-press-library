package cli

import (
	"context"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlog/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlog/internal/config"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func travelTestBlock(id int, pid string) map[string]any {
	return map[string]any{"id": id, "type": "place", "place": map[string]any{"place_id": pid, "name": pid}}
}
func travelTestEstimate(from, to, mode string, seconds float64) map[string]any {
	return map[string]any{"fromPlaceId": from, "toPlaceId": to, "travelMode": mode, "route": map[string]any{"distance": map[string]any{"value": 100.0}, "duration": map[string]any{"value": seconds}}}
}

func TestTravelLegsPreservesDirectionAndSkipsNotes(t *testing.T) {
	a, b, c := travelTestBlock(1, "a"), travelTestBlock(2, "b"), travelTestBlock(3, "c")
	a["endTime"] = "10:00"
	b["startTime"] = "10:05"
	sec := resolvedSection{Blocks: []any{a, map[string]any{"type": "note"}, b, c}}
	trip := map[string]any{"_resources": map[string]any{"distancesBetweenPlaces": map[string]any{"forward": travelTestEstimate("a", "b", "walking", 600), "reverse": travelTestEstimate("c", "b", "walking", 100)}}}
	report := buildTravelLegs(trip, sec, []string{"driving", "walking"}, "walking", false)
	legs := report["legs"].([]map[string]any)
	if len(legs) != 2 {
		t.Fatalf("legs=%v", legs)
	}
	est := legs[0]["estimates"].([]map[string]any)
	if est[0]["available"] != false || est[1]["available"] != true {
		t.Fatal(est)
	}
	schedule := legs[0]["schedule"].(map[string]any)
	if schedule["status"] != "conflict" || schedule["slack_seconds"] != float64(-300) {
		t.Fatal(schedule)
	}
	if legs[1]["estimates"].([]map[string]any)[1]["available"] != false {
		t.Fatal("reversed route treated as forward")
	}
	unselected := buildTravelLegs(trip, sec, []string{"walking"}, "", false)
	if _, ok := unselected["legs"].([]map[string]any)[0]["schedule"]; ok {
		t.Fatal("inferred selected mode")
	}
}

func TestTravelSlackUnknownAndFits(t *testing.T) {
	from, to := map[string]any{"endTime": "10:00"}, map[string]any{"startTime": "10:20"}
	e := map[string]any{"available": true, "duration_seconds": 600.0}
	if s := travelScheduleSlack(from, to, e); s["status"] != "fits" || s["slack_seconds"] != 600.0 {
		t.Fatal(s)
	}
	for _, pair := range [][2]map[string]any{{{}, to}, {from, {"startTime": "09:00"}}} {
		if travelScheduleSlack(pair[0], pair[1], e)["status"] != "unknown" {
			t.Fatal("guessed overnight/missing time")
		}
	}
	if travelScheduleSlack(from, to, nil)["status"] != "unknown" {
		t.Fatal("guessed missing estimate")
	}
}

func TestTravelPlanningAndSuggestions(t *testing.T) {
	sec := resolvedSection{Raw: map[string]any{"id": 7}}
	trip := map[string]any{"itinerary": map[string]any{"sections": []any{map[string]any{"blocks": []any{travelTestBlock(1, "planned")}}}}, "_resources": map[string]any{
		"placeMetadata":          []any{map[string]any{"placeId": "planned", "minMinutesSpent": 30, "maxMinutesSpent": 60, "description": "must not leak"}},
		"sectionRecommendations": map[string]any{"7": []any{map[string]any{"placeId": "planned"}, map[string]any{"placeId": "new", "name": "Example"}, map[string]any{"placeId": "new"}, map[string]any{"placeId": "next"}}}}}
	items := travelSuggestions(trip, sec, 5)
	if len(items) != 2 || items[0]["place_id"] != "new" {
		t.Fatal(items)
	}
	if len(travelSuggestions(trip, sec, 1)) != 1 {
		t.Fatal("limit ignored")
	}
	p := travelPlanningFields(trip, map[string]any{"place_id": "planned", "business_status": "OPERATIONAL"})
	if p["minMinutesSpent"] != 30 {
		t.Fatal(p)
	}
	if _, ok := p["description"]; ok {
		t.Fatal("unrequested text leaked")
	}
}

func TestTravelModesAndMalformedEstimates(t *testing.T) {
	modes, err := travelModes(" WALKING,walking,driving ")
	if err != nil || len(modes) != 2 {
		t.Fatal(modes, err)
	}
	for _, raw := range []string{"", "hovercraft", "walking,"} {
		if _, err := travelModes(raw); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	e := travelTestEstimate("a", "b", "walking", -1)
	got := savedTravelEstimate(map[string]any{"x": e}, "a", "b", "walking")
	if got["available"] != false {
		t.Fatal(got)
	}
}

func TestBudgetCategoryCurrencyIsolation(t *testing.T) {
	expense := func(amount float64, currency, category string) any {
		return map[string]any{"amount": map[string]any{"amount": amount, "currencyCode": currency}, "category": category}
	}
	got := summarizeBudget(map[string]any{"expenses": []any{expense(100, "JPY", "food"), expense(20, "SGD", "food"), expense(7, "SGD", "transit")}})
	legacy := got["totals_by_category"].(map[string]float64)
	if _, ok := legacy["food"]; ok {
		t.Fatal("mixed category summed")
	}
	if legacy["transit"] != 7 {
		t.Fatal(legacy)
	}
	grouped := got["totals_by_category_currency"].(map[string]map[string]float64)
	if grouped["food"]["JPY"] != 100 || grouped["food"]["SGD"] != 20 {
		t.Fatal(grouped)
	}
	if got["mixed_currency_categories_omitted"] != true {
		t.Fatal("missing compatibility notice")
	}
}

func TestTravelSavedSnapshotShapeAndTotals(t *testing.T) {
	// Synthetic fixture preserves the live API's compound key and nested numeric units.
	var trip map[string]any
	err := json.Unmarshal([]byte(`{"itinerary":{"sections":[{"id":7,"mode":"dayPlan","date":"2030-01-02","blocks":[{"id":1,"place":{"place_id":"a","name":"Renamed stop"}},{"id":2,"placeId":"b","title":"Fallback title"},{"id":3,"place_id":"c","name":"Third"}]}]},"_resources":{"distancesBetweenPlaces":{"[\"a\",\"b\",\"driving\"]":{"fromPlaceId":"a","toPlaceId":"b","travelMode":"driving","route":{"distance":{"value":6115.2,"text":"3.8 mi"},"duration":{"value":696,"text":"12 mins"},"polyline":"not exported"}}}}}`), &trip)
	if err != nil {
		t.Fatal(err)
	}
	sec, err := resolveSection(trip, 1, -1, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := buildTravelLegs(trip, sec, []string{"driving", "walking"}, "", false)
	totals := got["totals_by_mode"].([]map[string]any)
	if totals[0]["complete"] != false || totals[0]["missing_leg_count"] != 1 || totals[0]["available_distance_meters"] != 6115.2 || totals[0]["available_duration_seconds"] != 696.0 {
		t.Fatal(totals)
	}
	if totals[1]["missing_leg_count"] != 2 {
		t.Fatal(totals)
	}
	stops := got["stops"].([]map[string]any)
	if stops[0]["name"] != "Renamed stop" || stops[1]["name"] != "Fallback title" {
		t.Fatal(stops)
	}
	encoded, _ := json.Marshal(got)
	var roundTrip map[string]any
	if json.Unmarshal(encoded, &roundTrip) != nil {
		t.Fatal("invalid output")
	}
}

func TestTravelRejectsStaleCoordinatesAndPreservesUnknownStops(t *testing.T) {
	a, b := travelTestBlock(1, "a"), travelTestBlock(2, "b")
	a["place"].(map[string]any)["geometry"] = map[string]any{"location": map[string]any{"lat": -20.0, "lng": 130.0}}
	b["place"].(map[string]any)["geometry"] = map[string]any{"location": map[string]any{"lat": -20.1, "lng": 130.1}}
	estimate := travelTestEstimate("a", "b", "driving", 300)
	estimate["fromLat"] = -20.0
	estimate["fromLng"] = 130.0
	estimate["toLat"] = -20.1
	estimate["toLng"] = 130.1
	saved := map[string]any{"route": estimate}
	if got := savedTravelEstimate(saved, "a", "b", "driving", a, b); got["available"] != true {
		t.Fatal(got)
	}
	estimate["toLat"] = -21.0
	if got := savedTravelEstimate(saved, "a", "b", "driving", a, b); got["available"] != false || got["reason"] != "saved_coordinates_changed" {
		t.Fatal(got)
	}
	unknown := map[string]any{"id": 3, "type": "place", "place": map[string]any{"name": "Hotel without Google id"}}
	report := buildTravelLegs(map[string]any{}, resolvedSection{Blocks: []any{a, unknown, b}}, []string{"driving"}, "", false)
	if len(report["stops"].([]map[string]any)) != 3 {
		t.Fatal("unlocated stop disappeared")
	}
	totals := report["totals_by_mode"].([]map[string]any)[0]
	if totals["missing_leg_count"] != 2 || totals["complete"] != false {
		t.Fatal(totals)
	}
	if travelStopID(map[string]any{"place": map[string]any{"placeId": "nested"}}) != "nested" {
		t.Fatal("nested placeId lost")
	}
}

func TestBudgetSummaryReportsIncompleteAmounts(t *testing.T) {
	expenses := []any{
		map[string]any{"category": "food", "amount": map[string]any{"amount": 10.0, "currencyCode": "sgd"}},
		map[string]any{"category": "food", "amount": map[string]any{"amount": -2.0, "currencyCode": "SGD"}},
		map[string]any{"category": "food", "amount": map[string]any{"currencyCode": "SGD"}},
		map[string]any{"category": "food", "amount": map[string]any{"amount": "unknown", "currencyCode": "SGD"}},
		map[string]any{"category": "food", "amount": map[string]any{"amount": 20.0}},
	}
	got := summarizeBudget(map[string]any{"expenses": expenses})
	if got["totals_complete"] != false || got["incomplete_expense_count"] != 3 {
		t.Fatal(got)
	}
	totals := got["totals_by_currency"].(map[string]float64)
	if len(totals) != 1 || totals["SGD"] != 8 {
		t.Fatal(totals)
	}
}

func TestTravelConsumersUseActualFetchPlanResourceEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tripPlans/abcdefghijklmnop" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"tripPlan":{"itinerary":{"sections":[{"id":7,"mode":"dayPlan","date":"2030-01-02","blocks":[{"id":1,"type":"place","endTime":"10:00","place":{"place_id":"a","name":"Museum","geometry":{"location":{"lat":1,"lng":2}}}},{"id":2,"type":"place","startTime":"10:20","place":{"place_id":"b","name":"Park","geometry":{"location":{"lat":3,"lng":4}}}}]}]}},"resources":{"distancesBetweenPlaces":{"route":{"fromPlaceId":"a","toPlaceId":"b","fromLat":1,"fromLng":2,"toLat":3,"toLng":4,"travelMode":"walking","route":{"distance":{"value":600},"duration":{"value":600}}}},"placeMetadata":[{"placeId":"a","minMinutesSpent":30,"maxMinutesSpent":90}],"sectionRecommendations":{"7":[{"placeId":"new","name":"Garden"}]}}}`))
	}))
	defer server.Close()
	c := client.New(&config.Config{BaseURL: server.URL}, time.Second, 0)
	trip, _, err := fetchPlan(context.Background(), c, "abcdefghijklmnop", 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := trip["_resources"].(map[string]json.RawMessage); !ok {
		t.Fatal("fixture must exercise actual raw-resource contract")
	}
	sec, err := resolveSection(trip, 1, -1, 0)
	if err != nil {
		t.Fatal(err)
	}
	report := buildTravelLegs(trip, sec, []string{"walking"}, "walking", true)
	legs := report["legs"].([]map[string]any)
	estimate := legs[0]["estimates"].([]map[string]any)[0]
	if estimate["available"] != true || estimate["duration_seconds"] != 600.0 {
		t.Fatalf("saved route lost: %#v", estimate)
	}
	if legs[0]["schedule"].(map[string]any)["status"] != "fits" {
		t.Fatal(legs)
	}
	suggestions := travelSuggestions(trip, sec, 5)
	if len(suggestions) != 1 || suggestions[0]["place_id"] != "new" {
		t.Fatalf("saved suggestions lost: %#v", suggestions)
	}
	_, block, _, err := resolveUniquePlanBlock(trip, 1)
	if err != nil {
		t.Fatal(err)
	}
	planning := readablePlanBlockWithPlanning(trip, block, false, false)["planning"].(map[string]any)
	if planning["minMinutesSpent"] != 30.0 || planning["maxMinutesSpent"] != 90.0 {
		t.Fatalf("saved planning metadata lost: %#v", planning)
	}
}
