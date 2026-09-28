package cli

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/jalan/internal/jalan"
)

func comparisonSelectDates() (string, string) {
	first := futureStayDate()
	date, _ := time.Parse("2006-01-02", first)
	return first, date.AddDate(0, 0, 1).Format("2006-01-02")
}

func comparisonSelectionFixture(first, second string) jalan.Response {
	group := func(index int, date string, items []any) map[string]any {
		return map[string]any{
			"alternative_index": index, "check_in": date, "query": map[string]any{"check_in": date, "adults_per_room": 2},
			"status": "ok", "pagination": map[string]any{"returned_count": len(items), "has_more": false},
			"source_url":              "https://www.jalan.net/yad385995/plan/?check_in=" + date,
			"observations":            []map[string]any{{"observed_at": "2026-09-27T15:00:00Z", "cache_status": "live"}},
			"comparable_price_groups": []any{}, "results": items,
		}
	}
	offer := func(plan, room string, amount any) map[string]any {
		return map[string]any{
			"property_id": "385995", "plan_id": plan, "room_id": room, "plan_name": "和室",
			"price": map[string]any{"amount": amount, "basis": "whole_stay", "currency": "JPY", "extra_fees": "入湯税別"},
		}
	}
	return jalan.Response{
		Meta: map[string]any{"source": "Jalan public HTML", "observed_at": "2026-09-27T15:00:00Z", "timezone": "Asia/Tokyo", "query": map[string]any{"dates": []string{first, second}}, "coverage": "bounded observed alternatives; not exhaustive", "upstream_requests": 2},
		Results: []any{
			group(0, first, []any{offer("03912759", "0576806", int64(46200)), offer("03806855", "0546600", nil)}),
			group(1, second, []any{offer("04111111", "0555555", int64(39000))}),
		},
		Pagination:    map[string]any{"requested_alternatives": 2, "returned_count": 2, "failed_count": 0, "has_more": false},
		FetchFailures: []map[string]any{},
	}
}

func TestStayCompareSelectDatesKeepsCellContextAndNestedOfferOrder(t *testing.T) {
	first, second := comparisonSelectDates()
	fake := &fakeStayService{response: comparisonSelectionFixture(first, second)}
	installStayFake(t, fake)
	out, diagnostic, err := executeStayDelivery(t, "compare", "385995", "--dates", first+","+second, "--agent", "--select", "check_in,results.property_id,results.plan_id,results.room_id,results.price.amount,results.price.basis")
	if err != nil || diagnostic != "" {
		t.Fatalf("compare selection failed: %v stderr=%s", err, diagnostic)
	}
	selected := decodeStay(t, out)
	if len(selected) != 4 || len(selected["results"].([]any)) != 2 {
		t.Fatalf("envelope/group count changed: %s", out)
	}
	for key := range fake.response.Meta {
		if !reflect.DeepEqual(selected["meta"].(map[string]any)[key], jsonValue(t, fake.response.Meta[key])) {
			t.Fatalf("meta.%s changed: %s", key, out)
		}
	}
	cells := selected["results"].([]any)
	expectedCounts := []int{2, 1}
	expectedIDs := [][]string{{"03912759", "03806855"}, {"04111111"}}
	for i, cell := range cells {
		group := cell.(map[string]any)
		for _, key := range []string{"alternative_index", "check_in", "query", "status", "pagination", "source_url", "observations"} {
			if group[key] == nil {
				t.Fatalf("cell %d lost %s: %s", i, key, out)
			}
		}
		entries := group["results"].([]any)
		if len(entries) != expectedCounts[i] {
			t.Fatalf("cell %d offer count changed: %s", i, out)
		}
		for j, entry := range entries {
			item := entry.(map[string]any)
			if item["plan_id"] != expectedIDs[i][j] || item["property_id"] != "385995" || item["plan_name"] != nil {
				t.Fatalf("cell %d offer %d order/projection changed: %s", i, j, out)
			}
			price := item["price"].(map[string]any)
			if len(price) != 2 || price["basis"] != "whole_stay" {
				t.Fatalf("cell %d offer %d price fields wrong: %s", i, j, out)
			}
		}
	}
	if cells[0].(map[string]any)["results"].([]any)[1].(map[string]any)["price"].(map[string]any)["amount"] != nil {
		t.Fatalf("unknown amount turned into a value: %s", out)
	}
	if !reflect.DeepEqual(selected["pagination"], jsonValue(t, fake.response.Pagination)) || len(selected["fetch_failures"].([]any)) != 0 {
		t.Fatalf("coverage metadata changed: %s", out)
	}
}

func TestStayCompareSelectPlansKeepsPairIdentityAndPartialFailures(t *testing.T) {
	first, second := comparisonSelectDates()
	response := comparisonSelectionFixture(first, second)
	group := response.Results[0].(map[string]any)
	group["plan_id"] = "03912759"
	group["room_id"] = "0576806"
	group["results"] = []any{map[string]any{"property_id": "385995", "plan_id": "03912759", "room_id": "0576806", "price": map[string]any{"amount": nil, "basis": "unknown", "final_payable": nil, "extra_fees": "不明"}, "room_description": "露天風呂なし"}}
	group["pagination"] = map[string]any{"returned_count": 1, "has_more": false}
	response.Results = response.Results[:1]
	response.Meta["status"] = "partial"
	response.FetchFailures = []map[string]any{{"alternative_index": 1, "check_in": second, "plan_id": "03806855", "room_id": "0546600", "code": "parse_failure", "message": "source changed"}}
	response.Pagination["returned_count"] = 1
	response.Pagination["failed_count"] = 1
	fake := &fakeStayService{response: response, err: &jalan.PartialError{Failures: response.FetchFailures, Cause: &jalan.Error{Code: "parse_failure", Message: "source changed"}}}
	installStayFake(t, fake)
	out, diagnostic, err := executeStayDelivery(t, "compare", "385995", "--check-in", first, "--plans", "03912759:0576806,03806855:0546600", "--select", "results.property_id,results.plan_id,results.room_id,results.price")
	if err == nil || ExitCode(err) != 8 || !json.Valid([]byte(diagnostic)) {
		t.Fatalf("partial selection lost exit/diagnostic: err=%v stderr=%s", err, diagnostic)
	}
	selected := decodeStay(t, out)
	if len(selected["fetch_failures"].([]any)) != 1 || selected["meta"].(map[string]any)["status"] != "partial" {
		t.Fatalf("partial metadata/failures lost: %s", out)
	}
	cells := selected["results"].([]any)
	if len(cells) != 1 {
		t.Fatalf("successful comparison cell lost: %s", out)
	}
	cell := cells[0].(map[string]any)
	for _, key := range []string{"alternative_index", "check_in", "query", "status", "pagination", "source_url", "observations", "plan_id", "room_id"} {
		if cell[key] == nil {
			t.Fatalf("plan cell lost %s: %s", key, out)
		}
	}
	price := cell["results"].([]any)[0].(map[string]any)["price"].(map[string]any)
	if len(price) != 4 || price["amount"] != nil || price["final_payable"] != nil || price["basis"] != "unknown" {
		t.Fatalf("whole price/unknowns changed: %s", out)
	}
	if cell["results"].([]any)[0].(map[string]any)["room_description"] != nil {
		t.Fatalf("unselected plan detail leaked: %s", out)
	}
}

func TestStayCompareSelectRejectsWrongPricePathsWithoutPrintingData(t *testing.T) {
	first, second := comparisonSelectDates()
	fake := &fakeStayService{response: comparisonSelectionFixture(first, second)}
	installStayFake(t, fake)
	for _, selection := range []string{"price.amount", "check_in,price.amount", "results.price.unit", "check_in,results.price.unit"} {
		out, diagnostic, err := executeStayDelivery(t, "compare", "385995", "--dates", first+","+second, "--select", selection)
		if out != "" || err == nil || ExitCode(err) != 2 || !json.Valid([]byte(diagnostic)) {
			t.Fatalf("selection %q should be a usage error: stdout=%q stderr=%q err=%v", selection, out, diagnostic, err)
		}
		failure := decodeStay(t, diagnostic)["error"].(map[string]any)
		if !strings.Contains(failure["hint"].(string), "results.price.") {
			t.Fatalf("selection %q has no actionable comparison hint: %s", selection, diagnostic)
		}
	}
}

func TestStayDirectItemSelectRetainsOptionalResultsPrefix(t *testing.T) {
	fake := &fakeStayService{response: stayFixture()}
	installStayFake(t, fake)
	out, diagnostic, err := executeStayDelivery(t, "property", "385995", "--select", "results.id,price.amount,baths.in_room")
	if err != nil || diagnostic != "" {
		t.Fatalf("direct item selection failed: %v stderr=%s", err, diagnostic)
	}
	selected := decodeStay(t, out)
	result := selected["results"].([]any)[0].(map[string]any)
	if len(result) != 3 || result["id"] != "385995" || result["price"].(map[string]any)["amount"] != float64(46200) || result["baths"].(map[string]any)["in_room"] != nil {
		t.Fatalf("direct item projection/unknown changed: %s", out)
	}
}

func jsonValue(t *testing.T, value any) any {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

func TestStayCompareSelectSourceRefAndOverlappingPricePaths(t *testing.T) {
	first, second := comparisonSelectDates()
	fixture := comparisonSelectionFixture(first, second)
	offer := fixture.Results[0].(map[string]any)["results"].([]any)[0].(map[string]any)
	price := offer["price"].(map[string]any)
	sourceURL := "https://www.jalan.net/yad385995/plan/?check_in=" + first
	price["evidence"] = []map[string]any{{"field": "price.amount", "text": "46,200円", "url": sourceURL}}
	fake := &fakeStayService{response: fixture}
	installStayFake(t, fake)
	args := []string{"compare", "385995", "--dates", first + "," + second, "--select"}
	out, diagnostic, err := executeStayDelivery(t, append(args, "results.price.evidence.source_ref")...)
	if err != nil || diagnostic != "" {
		t.Fatalf("evidence selection failed: %v stderr=%s", err, diagnostic)
	}
	selected := decodeStay(t, out)
	refs := selected["meta"].(map[string]any)["sources"].(map[string]any)
	if refs["s1"] != sourceURL {
		t.Fatalf("source map lost URL: %s", out)
	}
	cells := selected["results"].([]any)
	if len(cells) != 2 || len(cells[0].(map[string]any)["results"].([]any)) != 2 {
		t.Fatalf("nested order/cardinality changed: %s", out)
	}
	evidence := cells[0].(map[string]any)["results"].([]any)[0].(map[string]any)["price"].(map[string]any)["evidence"].([]any)[0].(map[string]any)
	if len(evidence) != 1 || evidence["source_ref"] != "s1" {
		t.Fatalf("exposed source_ref projection wrong: %s", out)
	}
	secondOffer := cells[0].(map[string]any)["results"].([]any)[1].(map[string]any)
	if len(secondOffer) != 0 {
		t.Fatalf("offer lacking evidence changed position or gained data: %s", out)
	}
	var whole any
	for i, selection := range []string{"results.price", "results.price,results.price.amount", "results.price.amount,results.price"} {
		out, diagnostic, err = executeStayDelivery(t, append(args, selection)...)
		if err != nil || diagnostic != "" {
			t.Fatalf("overlap %q failed: %v stderr=%s", selection, err, diagnostic)
		}
		got := decodeStay(t, out)["results"]
		if i == 0 {
			whole = got
		} else if !reflect.DeepEqual(got, whole) {
			t.Fatalf("overlap %q changed whole price or nested cardinality: %s", selection, out)
		}
	}
	priceOut := whole.([]any)[0].(map[string]any)["results"].([]any)[0].(map[string]any)["price"].(map[string]any)
	for _, key := range []string{"amount", "basis", "currency", "extra_fees", "evidence"} {
		if _, ok := priceOut[key]; !ok {
			t.Fatalf("overlapping projection dropped %s", key)
		}
	}
}

func TestStayCompareSelectEmptyInventoryRetainsArraysAndContext(t *testing.T) {
	first, second := comparisonSelectDates()
	fixture := comparisonSelectionFixture(first, second)
	for _, group := range fixture.Results {
		cell := group.(map[string]any)
		cell["results"] = []any{}
		cell["pagination"] = map[string]any{"returned_count": 0, "has_more": false}
		cell["status"] = "no_matches"
	}
	installStayFake(t, &fakeStayService{response: fixture})
	out, diagnostic, err := executeStayDelivery(t, "compare", "385995", "--dates", first+","+second, "--select", "results.price.amount")
	if err != nil || diagnostic != "" {
		t.Fatalf("empty inventory projection failed: %v stderr=%s", err, diagnostic)
	}
	for _, row := range decodeStay(t, out)["results"].([]any) {
		cell := row.(map[string]any)
		if len(cell["results"].([]any)) != 0 || cell["status"] != "no_matches" || cell["check_in"] == nil || cell["pagination"] == nil || cell["observations"] == nil {
			t.Fatalf("empty alternative lost coverage or produced placeholder objects: %s", out)
		}
	}
}
