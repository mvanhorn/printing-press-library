package cli

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/rakuten-travel/internal/cliutil/testenv"
)

type cliRoomFixture struct {
	Plan, Room string
	Amount     int64
}

func fixtureRequestDate(request *http.Request, suffix string) time.Time {
	values := request.URL.Query()
	year, _ := strconv.Atoi(values.Get("f_nen" + suffix))
	month, _ := strconv.Atoi(values.Get("f_tuki" + suffix))
	day, _ := strconv.Atoi(values.Get("f_hi" + suffix))
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

// This synthetic page uses the independently observed source structure, with
// conditions and reservation form values copied from the actual wire request.
// The command tests exercise the real domain client and parser through HTTP.
func offerCLIHTML(request *http.Request, rooms []cliRoomFixture, next bool) string {
	hotel := request.URL.Path[strings.LastIndex(request.URL.Path, "/")+1:]
	values := request.URL.Query()
	conditions := map[string]any{"isDated": true, "isHotelFixed": true, "isDayuse": false, "f_no": []string{hotel}}
	for key, value := range values {
		conditions[key] = value
	}
	plans := map[string]any{}
	for _, room := range rooms {
		if plans[room.Plan] == nil {
			plans[room.Plan] = map[string]any{"rooms": map[string]any{}}
		}
		planRooms := plans[room.Plan].(map[string]any)["rooms"].(map[string]any)
		planRooms[room.Room] = map[string]any{"sumTotalChargeTaxInclusive": room.Amount, "sumTotalChargeTaxExclusive": room.Amount - 1000}
	}
	quotes := map[string]any{}
	if len(rooms) > 0 {
		quotes[hotel] = map[string]any{"plans": plans}
	}
	conditionJSON, _ := json.Marshal(conditions)
	quoteJSON, _ := json.Marshal(quotes)
	var body strings.Builder
	fmt.Fprintf(&body, `<html><title>Rakuten dated plans</title><body><script>hinfo.conditions=%s;hinfo.hotels=%s;</script><span class="plan-pagination__page--current">%s</span><span class="plan-number__total-count">%d</span>`, conditionJSON, quoteJSON, values.Get("f_page_no"), len(plans))
	nights := int(fixtureRequestDate(request, "2").Sub(fixtureRequestDate(request, "1")) / (24 * time.Hour))
	for i, room := range rooms {
		fmt.Fprintf(&body, `<div class="planThumb" id="%s"><h3>Whole stay plan %s</h3><div data-locate="plan-description">Full plan description and booking conditions.</div><div class="rm-type-wrapper" id="%s-%s"><h4>Room %s</h4><div data-locate="roomType-Remark">Detailed room facilities.</div><div data-locate="roomType-option-meal">食事 朝食なし 夕食なし</div><div data-locate="plan-price-detail">%d泊 大人%s人 税込</div><div class="ndPrice">合計 %d円</div><div class="cmn_rbAndNvrWrap">1人あたり %d円</div><form name="book%d" method="POST" action="https://aps1.travel.rakuten.co.jp/portal/my/ry_kensaku.k4">`, room.Plan, room.Plan, room.Plan, room.Room, room.Room, nights, values.Get("f_otona_su"), room.Amount, room.Amount/2, i)
		form := map[string]string{"f_no": hotel, "f_camp_id": room.Plan, "f_syu": room.Room, "f_hi1": fixtureRequestDate(request, "1").Format("2006-01-02"), "f_hi2": fixtureRequestDate(request, "2").Format("2006-01-02")}
		for _, key := range []string{"f_heya_su", "f_otona_su", "f_s1", "f_s2", "f_y1", "f_y2", "f_y3", "f_y4"} {
			form[key] = values.Get(key)
		}
		for key, value := range form {
			fmt.Fprintf(&body, `<input name="%s" value="%s">`, key, html.EscapeString(value))
		}
		fmt.Fprintf(&body, `</form><a data-locate="plan-reservation-button" href="javascript:document['book%d'].submit()">予約</a></div></div>`, i)
	}
	if len(rooms) == 0 {
		body.WriteString("<div class=\"planList\"><p>ご希望の日程に該当する空室が見つかりません</p></div>")
	}
	if next {
		nextURL := *request.URL
		nextValues := nextURL.Query()
		page, _ := strconv.Atoi(nextValues.Get("f_page_no"))
		nextValues.Set("f_page_no", strconv.Itoa(page+1))
		nextURL.RawQuery = nextValues.Encode()
		fmt.Fprintf(&body, `<a href="%s">next</a>`, html.EscapeString(nextURL.String()))
	}
	body.WriteString("</body></html>")
	return body.String()
}

func TestPublicTravelOfferSearchPriceSelectionAndPaging(t *testing.T) {
	testenv.Isolate(t)
	calls := 0
	fixtures := []cliRoomFixture{{"3951989", "s-double-", 20720}, {"3951989", "twin-a-", 23940}, {"3951989", "twin-b-", 24940}}
	injectTravelHTTP(t, func(request *http.Request) (*http.Response, error) {
		calls++
		if request.URL.Query().Get("f_heya_su") != "2" || request.URL.Query().Get("f_y4") != "1" {
			t.Fatalf("per-room query lost: %s", request.URL)
		}
		rows := fixtures
		if request.URL.Query().Get("f_page_no") == "2" {
			rows = []cliRoomFixture{{"3951990", "page-two-", 25940}}
		}
		return travelHTTPResponse(request, 200, offerCLIHTML(request, rows, request.URL.Query().Get("f_page_no") == "1")), nil
	})
	args := replaceTravelFlag(offerCLIArgs("search"), "--rooms", "2")
	args = append(args, "--infant-none", "1", "--limit", "1", "--agent")
	out, diagnostics, err := runPublicTravelCLI(args...)
	if err != nil || diagnostics != "" {
		t.Fatalf("offer err=%v stderr=%s", err, diagnostics)
	}
	payload := decodeTravelOutput(t, out)
	row := travelObject(t, travelRows(t, payload["results"])[0])
	price := travelObject(t, row["price"])
	if price["currency"] != "JPY" || price["per_room_whole_stay_jpy"] != float64(20720) || price["per_person_whole_stay_jpy"] != float64(10360) || price["consumption_tax"] != "included" || price["accommodation_tax"] != "unknown" || price["optional_fees"] != "unknown" {
		t.Fatalf("price units/tax evidence lost under agent compact: %s", out)
	}
	if _, exists := row["description"]; exists {
		t.Fatalf("search retained long descriptions: %s", out)
	}
	if row["room_id"] != "s-double-" || row["plan_cancellation_policy"] != nil || row["room_anchor"] != "3951989-s-double-" {
		t.Fatalf("identity/policy wrong: %s", out)
	}
	query := travelObject(t, row["query"])
	if query["rooms"] != float64(2) || travelObject(t, query["children_per_room"])["infant_none"] != float64(1) {
		t.Fatalf("party echo wrong: %s", out)
	}
	page := travelObject(t, travelObject(t, payload["meta"])["page"])
	if page["source_unit"] != "plans" || page["source_items_seen"] != float64(1) || page["rows_seen"] != float64(3) || page["next_page"] != float64(1) || page["next_offset"] != float64(1) {
		t.Fatalf("plan/room continuation lost: %s", out)
	}
	out, _, err = runPublicTravelCLI(append(args, "--offset", "2")...)
	if err != nil {
		t.Fatal(err)
	}
	payload = decodeTravelOutput(t, out)
	page = travelObject(t, travelObject(t, payload["meta"])["page"])
	if travelObject(t, travelRows(t, payload["results"])[0])["room_id"] != "twin-b-" || page["next_page"] != float64(2) || page["next_offset"] != float64(0) {
		t.Fatalf("source page continuation incorrect: %s", out)
	}
	out, _, err = runPublicTravelCLI(append(args, "--page", "2", "--select", "results.hotel_id,results.room_id,results.price.per_room_whole_stay_jpy")...)
	if err != nil {
		t.Fatal(err)
	}
	payload = decodeTravelOutput(t, out)
	row = travelObject(t, travelRows(t, payload["results"])[0])
	if len(payload) != 1 || len(row) != 3 || row["room_id"] != "page-two-" || len(travelObject(t, row["price"])) != 1 {
		t.Fatalf("price selection did not survive agent mode: %s", out)
	}
	if calls != 3 {
		t.Fatalf("fresh uncached inventory must fetch once per invocation: calls=%d", calls)
	}
}

func TestPublicTravelOfferShowExactTupleAndOnePageSnapshot(t *testing.T) {
	testenv.Isolate(t)
	rooms := make([]cliRoomFixture, 105)
	for i := range rooms {
		rooms[i] = cliRoomFixture{"3951989", fmt.Sprintf("r%d-", i), 20720 + int64(i)}
	}
	calls := 0
	injectTravelHTTP(t, func(request *http.Request) (*http.Response, error) {
		calls++
		switch {
		case strings.Contains(request.URL.Path, "/hotelinfo/plan/"):
			return travelHTTPResponse(request, 200, offerCLIHTML(request, rooms, true)), nil
		case strings.HasSuffix(request.URL.Path, "_std.html"):
			return travelHTTPResponse(request, 200, hotelDetailsCLIHTML), nil
		default:
			return travelHTTPResponse(request, 200, hotelBasicCLIHTML), nil
		}
	})
	args := append(offerCLIArgs("show"), "--plan", "3951989", "--room", "r104-", "--agent", "--no-cache")
	out, diagnostics, err := runPublicTravelCLI(args...)
	if err != nil || diagnostics != "" {
		t.Fatalf("show err=%v stderr=%s", err, diagnostics)
	}
	payload := decodeTravelOutput(t, out)
	inspection := travelObject(t, payload["results"])
	offer := travelObject(t, inspection["offer"])
	if offer["hotel_id"] != "51870" || offer["plan_id"] != "3951989" || offer["room_id"] != "r104-" || offer["description"] == nil || offer["room_description"] == nil {
		t.Fatalf("exact offer/full show content lost: %s", out)
	}
	if offer["plan_cancellation_policy"] != nil || travelObject(t, inspection["property_cancellation_policy"])["level"] != "property" || !strings.Contains(fmt.Sprint(inspection["property_fee_notes"]), "宿泊税") {
		t.Fatalf("property vs plan policy/fee scope lost: %s", out)
	}
	if !strings.HasSuffix(offer["booking_url"].(string), "#3951989-r104-") || !strings.Contains(offer["source_url"].(string), "f_flg=PLAN") {
		t.Fatalf("dated room handoff incorrect: %s", out)
	}
	meta := travelObject(t, payload["meta"])
	source := travelObject(t, meta["source_info"])
	if source["cache_state"] != "invocation_snapshot" || travelObject(t, meta["requests"])["requests"] != float64(3) || calls != 3 {
		t.Fatalf("same-page scan refetched source: calls=%d output=%s", calls, out)
	}
}

func TestPublicTravelOfferShowNotFoundAndBudgetStayInspectable(t *testing.T) {
	testenv.Isolate(t)
	calls := 0
	injectTravelHTTP(t, func(request *http.Request) (*http.Response, error) {
		calls++
		return travelHTTPResponse(request, 200, offerCLIHTML(request, []cliRoomFixture{{"3951989", "s-double-", 20720}}, true)), nil
	})
	args := append(offerCLIArgs("show"), "--plan", "3951989", "--room", "s-double", "--max-scan-pages", "1")
	out, diagnostics, err := runPublicTravelCLI(args...)
	if ExitCode(err) != 3 || !strings.Contains(diagnostics, "inspected source pages") {
		t.Fatalf("not-found boundary lost: err=%v stderr=%s", err, diagnostics)
	}
	payload := decodeTravelOutput(t, out)
	meta := travelObject(t, payload["meta"])
	if meta["status"] != "not_found_within_inspected_pages" || !strings.Contains(fmt.Sprint(meta["note"]), "--max-scan-pages") || !strings.Contains(fmt.Sprint(meta["note"]), "--page") || !strings.Contains(fmt.Sprint(meta["note"]), "does not prove") || travelObject(t, payload["results"])["offer"] != nil || travelObject(t, meta["page"])["next_page"] != float64(2) || calls != 1 {
		t.Fatalf("show substituted a similar tuple or hid coverage: %s", out)
	}
	out, diagnostics, err = runPublicTravelCLI(append(replaceTravelFlag(args, "--room", "s-double-"), "--max-requests", "1")...)
	if ExitCode(err) != 5 || !strings.Contains(diagnostics, "request_budget") {
		t.Fatalf("budget exhaustion not explicit: err=%v stderr=%s", err, diagnostics)
	}
	payload = decodeTravelOutput(t, out)
	meta = travelObject(t, payload["meta"])
	if meta["status"] != "property_inspection_error" || travelObject(t, payload["results"])["offer"] == nil || travelObject(t, meta["error"])["kind"] != "request_budget" {
		t.Fatalf("budget failure discarded found offer: %s", out)
	}
}

func TestPublicTravelCompareRetainsEmptyAndFailedCells(t *testing.T) {
	testenv.Isolate(t)
	calls := 0
	injectTravelHTTP(t, func(request *http.Request) (*http.Response, error) {
		calls++
		if strings.HasSuffix(request.URL.Path, "/33333") {
			return travelHTTPResponse(request, 503, "<html>Unavailable</html>"), nil
		}
		rooms := []cliRoomFixture{{"3951989", "s-double-", 20720}, {"3951989", "twin-", 30000}}
		if strings.HasSuffix(request.URL.Path, "/72056") {
			rooms = nil
		}
		return travelHTTPResponse(request, 200, offerCLIHTML(request, rooms, false)), nil
	})
	in, _ := futureTravelDates()
	out, diagnostics, err := runPublicTravelCLI("compare", "--hotels", "51870,72056,33333", "--checkins", in, "--nights", "2", "--rooms", "2", "--adults-per-room", "2", "--infant-none", "1", "--agent")
	if err != nil || !strings.Contains(diagnostics, "1 of 3 cells failed") {
		t.Fatalf("partial compare err=%v stderr=%s", err, diagnostics)
	}
	payload := decodeTravelOutput(t, out)
	result := travelObject(t, payload["results"])
	cells := travelRows(t, result["cells"])
	failures := travelRows(t, result["fetch_failures"])
	if len(cells) != 3 || len(failures) != 1 {
		t.Fatalf("matrix dropped cells: %s", out)
	}
	wantStatuses := []string{"ok", "no_availability", "error"}
	for i, raw := range cells {
		cell := travelObject(t, raw)
		if cell["status"] != wantStatuses[i] {
			t.Fatalf("cell status mismatch: %s", out)
		}
		query := travelObject(t, cell["query"])
		if query["rooms"] != float64(2) || query["adults_per_room"] != float64(2) || travelObject(t, query["children_per_room"])["infant_none"] != float64(1) {
			t.Fatalf("comparison party changed: %s", out)
		}
		if i > 0 && cell["bounded_source_page_minimum"] != nil {
			t.Fatalf("empty/error cell got phantom price: %s", out)
		}
	}
	minimum := travelObject(t, travelObject(t, cells[0])["bounded_source_page_minimum"])
	if travelObject(t, minimum["price"])["per_room_whole_stay_jpy"] != float64(20720) || travelObject(t, failures[0])["kind"] != "upstream_error" {
		t.Fatalf("minimum/failure evidence incorrect: %s", out)
	}
	meta := travelObject(t, payload["meta"])
	if meta["source"] != "computed" || meta["transport"] != "live" || meta["price_comparison_scope"] != "bounded_source_page_only" || travelObject(t, meta["requests"])["requests"] != float64(5) || calls != 5 {
		t.Fatalf("computed/live/request metadata wrong: %s", out)
	}
	if strings.Contains(out, `"cheapest"`) {
		t.Fatalf("unqualified cheapest claim: %s", out)
	}
}

func TestPublicTravelInventoryCacheAndNoCacheContract(t *testing.T) {
	testenv.Isolate(t)
	calls := 0
	injectTravelHTTP(t, func(request *http.Request) (*http.Response, error) {
		calls++
		return travelHTTPResponse(request, 200, offerCLIHTML(request, []cliRoomFixture{{"3951989", "s-double-", 20720}}, false)), nil
	})
	args := append(offerCLIArgs("search"), "--inventory-cache-seconds", "60")
	out, _, err := runPublicTravelCLI(args...)
	if err != nil {
		t.Fatal(err)
	}
	first := travelObject(t, travelObject(t, decodeTravelOutput(t, out)["meta"])["source_info"])["observed_at"]
	out, _, err = runPublicTravelCLI(args...)
	if err != nil {
		t.Fatal(err)
	}
	meta := travelObject(t, decodeTravelOutput(t, out)["meta"])
	if meta["source"] != "local" || travelObject(t, meta["source_info"])["observed_at"] != first || calls != 1 {
		t.Fatalf("cached inventory freshness lost: %s calls=%d", out, calls)
	}
	out, _, err = runPublicTravelCLI(append(args, "--no-cache")...)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || travelObject(t, travelObject(t, decodeTravelOutput(t, out)["meta"])["source_info"])["cache_state"] != "disabled" {
		t.Fatalf("no-cache did not bypass source cache: %s calls=%d", out, calls)
	}
	out, _, err = runPublicTravelCLI(args...)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || travelObject(t, travelObject(t, decodeTravelOutput(t, out)["meta"])["source_info"])["observed_at"] != first {
		t.Fatalf("no-cache overwrote saved cache: %s calls=%d", out, calls)
	}
}

func TestPublicTravelEmptyOffersAndCompareBudget(t *testing.T) {
	testenv.Isolate(t)
	injectTravelHTTP(t, func(request *http.Request) (*http.Response, error) {
		return travelHTTPResponse(request, 200, offerCLIHTML(request, nil, false)), nil
	})
	out, _, err := runPublicTravelCLI(offerCLIArgs("search")...)
	if err != nil {
		t.Fatal(err)
	}
	payload := decodeTravelOutput(t, out)
	if len(travelRows(t, payload["results"])) != 0 || travelObject(t, payload["meta"])["status"] != "no_availability" {
		t.Fatalf("no availability became failure/null rows: %s", out)
	}
	in, _ := futureTravelDates()
	out, diagnostics, err := runPublicTravelCLI("compare", "--hotels", "51870,72056", "--checkins", in, "--nights", "2", "--rooms", "1", "--adults-per-room", "2", "--max-requests", "1")
	if err != nil || !strings.Contains(diagnostics, "1 of 2 cells failed") {
		t.Fatalf("partial budget compare err=%v stderr=%s", err, diagnostics)
	}
	payload = decodeTravelOutput(t, out)
	result := travelObject(t, payload["results"])
	cells := travelRows(t, result["cells"])
	if len(cells) != 2 || travelObject(t, cells[0])["status"] != "no_availability" || travelObject(t, cells[1])["status"] != "error" || travelObject(t, travelRows(t, result["fetch_failures"])[0])["kind"] != "request_budget" {
		t.Fatalf("budget failure misclassified as empty inventory: %s", out)
	}
}
