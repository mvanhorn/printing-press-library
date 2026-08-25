// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import "testing"

func TestPlanReservationNativeBlockShapes(t *testing.T) {
	flight := newFlightReservationBlock(planReservationOptions{airline: "NH", flightNumber: "463", departureAirport: "HND", arrivalAirport: "OKA", startDate: "2026-09-01", startTime: "08:30", endDate: "2026-09-01", endTime: "11:15", confirmationNumber: "ABC123", travelerNames: []string{"Ada"}})
	if stringField(flight, "type") != "flight" || mapField(flight, "flightInfo") == nil || mapField(flight, "depart") == nil || mapField(flight, "arrive") == nil {
		t.Fatalf("flight block = %#v", flight)
	}
	if stringField(mapField(mapField(flight, "flightInfo"), "airline"), "iata") != "NH" {
		t.Fatalf("flightInfo = %#v", flight["flightInfo"])
	}

	transit := newTransitReservationBlock("train", testPlace("Shinjuku Station"), testPlace("Tokyo Station"), planReservationOptions{carrier: "JR", startDate: "2026-09-02", endDate: "2026-09-02"})
	if stringField(transit, "type") != "train" || stringField(transit, "carrier") != "JR" || mapField(transit, "depart") == nil || mapField(transit, "arrive") == nil {
		t.Fatalf("transit block = %#v", transit)
	}

	cruise := newCruiseReservationBlock(testPlace("Naha Port"), testPlace("Ishigaki Port"), planReservationOptions{cruiseLine: "Example Cruises", shipName: "Blue", voyageNumber: "V1"})
	if stringField(cruise, "type") != "cruise" || stringField(cruise, "cruiseLine") != "Example Cruises" || cruise["portsOfCall"] == nil {
		t.Fatalf("cruise block = %#v", cruise)
	}

	attachment := newStandaloneAttachmentBlock(planReservationOptions{title: "Tickets", url: "https://example.com/tickets.pdf", filename: "tickets.pdf"})
	if stringField(attachment, "type") != "attachment" || stringField(attachment, "title") != "Tickets" {
		t.Fatalf("attachment block = %#v", attachment)
	}
	attachments, _ := attachment["attachments"].([]any)
	if len(attachments) != 1 {
		t.Fatalf("attachments = %#v", attachment["attachments"])
	}
}

func TestPlanReservationPlaceBackedShapes(t *testing.T) {
	lodging := newLodgingReservationBlock(testPlace("Hotel Moon Beach"), planReservationOptions{startDate: "2026-09-01", endDate: "2026-09-03", confirmationNumber: "LODGE1", travelerNames: []string{"Ada", "Grace"}})
	if got := reservationKindForBlock(lodging); got != "lodging" {
		t.Fatalf("lodging kind = %q block %#v", got, lodging)
	}
	hotel := mapField(lodging, "hotel")
	if stringField(hotel, "checkIn") != "2026-09-01" || stringField(hotel, "confirmationNumber") != "LODGE1" {
		t.Fatalf("hotel = %#v", hotel)
	}

	offerBlock, err := newLodgingReservationBlockFromOffer(planReservationOptions{
		startDate:        "2026-09-01",
		endDate:          "2026-09-03",
		lodgingOfferJSON: `{"source":"airbnb","offerId":"123","lodging":{"id":{"type":"airbnb","listingId":"123"},"name":"Naha Base","location":{"latitude":26.2,"longitude":127.7},"images":[{"url":"https://example.com/image.jpg"}],"rating":{"source":"Airbnb","value":5},"ratingCount":40},"priceRate":{"site":"Airbnb","amount":200,"currencyCode":"SGD","total":{"amount":1400,"currencyCode":"SGD"},"bookingUrl":"https://example.com/book"}}`,
	})
	if err != nil {
		t.Fatalf("newLodgingReservationBlockFromOffer: %v", err)
	}
	if got := reservationKindForBlock(offerBlock); got != "lodging" {
		t.Fatalf("offer lodging kind = %q block %#v", got, offerBlock)
	}
	offerPlace := mapField(offerBlock, "place")
	if stringField(offerPlace, "name") != "Naha Base" || stringField(offerPlace, "place_id") != "lodging:airbnb:123" {
		t.Fatalf("offer place = %#v", offerPlace)
	}
	meta := mapField(offerBlock, "lodgingOffer")
	price := mapField(meta, "priceRate")
	if stringField(price, "bookingUrl") != "https://example.com/book" {
		t.Fatalf("offer meta = %#v", meta)
	}

	restaurant := newRestaurantReservationBlock(testPlace("Sushi Harbor"), planReservationOptions{startDate: "2026-09-02", startTime: "19:00", partySize: 4, nameForReservation: "Ada", confirmationNumber: "DINNER1"})
	if got := reservationKindForBlock(restaurant); got != "restaurant" {
		t.Fatalf("restaurant kind = %q block %#v", got, restaurant)
	}
	if intAny(restaurant["partySize"]) != 4 || stringField(restaurant, "nameForReservation") != "Ada" {
		t.Fatalf("restaurant = %#v", restaurant)
	}
}

func TestPlanReservationCollectReservationBlocks(t *testing.T) {
	trip := testPlanTrip("Reservation target")
	secs := sections(trip)
	sec, _ := secs[0].(map[string]any)
	blocks, _ := sec["blocks"].([]any)
	blocks = append(blocks,
		newFlightReservationBlock(planReservationOptions{airline: "NH"}),
		newStandaloneAttachmentBlock(planReservationOptions{title: "Passports"}),
		newLodgingReservationBlock(testPlace("Hotel"), planReservationOptions{}),
	)
	sec["blocks"] = blocks

	all, err := collectReservationBlocks(trip, planEditOptions{}, "")
	if err != nil {
		t.Fatalf("collect all: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("all reservations = %#v", all)
	}
	flights, err := collectReservationBlocks(trip, planEditOptions{}, "flight")
	if err != nil {
		t.Fatalf("collect flights: %v", err)
	}
	if len(flights) != 1 || flights[0]["kind"] != "flight" {
		t.Fatalf("flights = %#v", flights)
	}
}

func testPlace(name string) map[string]any {
	return map[string]any{
		"place_id":          "place-" + name,
		"name":              name,
		"formatted_address": name + " address",
		"geometry":          map[string]any{"location": map[string]any{"lat": 1.25, "lng": 2.5}},
	}
}

func testFourDayLodgingTrip() map[string]any {
	day := func(id int, date string, blocks []any) map[string]any {
		return map[string]any{"id": id, "mode": "dayPlan", "date": date, "blocks": blocks}
	}
	return map[string]any{
		"title":     "Four day lodging target",
		"startDate": "2026-09-01",
		"endDate":   "2026-09-04",
		"itinerary": map[string]any{
			"sections": []any{
				day(1101, "2026-09-01", []any{map[string]any{"id": 2101, "type": "note"}}),
				day(1102, "2026-09-02", []any{}),
				day(1103, "2026-09-03", []any{}),
				day(1104, "2026-09-04", []any{}),
				map[string]any{"id": 1199, "mode": "notes", "blocks": []any{}},
			},
		},
	}
}

func TestLodgingStayNightDatesHalfOpen(t *testing.T) {
	dates, err := lodgingStayNightDates("2026-09-01", "2026-09-03")
	if err != nil {
		t.Fatalf("lodgingStayNightDates: %v", err)
	}
	if len(dates) != 2 || dates[0] != "2026-09-01" || dates[1] != "2026-09-02" {
		t.Fatalf("dates = %#v", dates)
	}
}

func TestLodgingNightSectionsFourDayTrip(t *testing.T) {
	trip := testFourDayLodgingTrip()
	nights, err := lodgingNightSections(trip, "2026-09-01", "2026-09-03")
	if err != nil {
		t.Fatalf("lodgingNightSections: %v", err)
	}
	if len(nights) != 2 || nights[0].Index != 0 || nights[1].Index != 1 {
		t.Fatalf("nights = %#v", nights)
	}
	if nights[0].Report.Date != "2026-09-01" || nights[1].Report.Date != "2026-09-02" || nights[0].Report.Day != 1 || nights[1].Report.Day != 2 {
		t.Fatalf("night reports = %#v %#v", nights[0].Report, nights[1].Report)
	}
	if _, err := lodgingNightSections(trip, "2026-09-04", "2026-09-06"); err == nil {
		t.Fatal("expected missing 2026-09-05 to fail closed")
	}
}

func TestShouldSpanLodgingNightsDefault(t *testing.T) {
	if !shouldSpanLodgingNights(true, "2026-09-01", "2026-09-03") {
		t.Fatal("default span when end-date > start-date")
	}
	if shouldSpanLodgingNights(true, "2026-09-01", "2026-09-01") {
		t.Fatal("same-day stay should not span")
	}
	if shouldSpanLodgingNights(false, "2026-09-01", "2026-09-03") {
		t.Fatal("--span-nights=false must not span")
	}
}

func TestLodgingInsertOpsSpansNightsOneOpArray(t *testing.T) {
	trip := testFourDayLodgingTrip()
	place := testPlace("Wakasa 2-7-14")
	echo, err := finalizeLodgingPlace(place, planReservationOptions{
		planEditOptions:     planEditOptions{lat: 1.25, lng: 2.5},
		displayName:         "Hotel Moon Beach",
		expectNameSubstring: "Wakasa",
	})
	if err != nil {
		t.Fatalf("finalizeLodgingPlace: %v", err)
	}
	if echo["resolved_name"] != "Wakasa 2-7-14" || echo["resolved_address"] != "Wakasa 2-7-14 address" || echo["distance_m"] != 0 {
		t.Fatalf("echo = %#v", echo)
	}
	if stringField(place, "name") != "Hotel Moon Beach" {
		t.Fatalf("display name = %q", stringField(place, "name"))
	}
	block := newLodgingReservationBlock(place, planReservationOptions{startDate: "2026-09-01", endDate: "2026-09-03"})
	result, err := buildLodgingReservationInsert(trip, planReservationOptions{
		planEditOptions: planEditOptions{position: -1},
		spanNights:      true,
		startDate:       "2026-09-01",
		endDate:         "2026-09-03",
		displayName:     "Hotel Moon Beach",
	}, block, echo)
	if err != nil {
		t.Fatalf("buildLodgingReservationInsert: %v", err)
	}
	if len(result.Ops) != 2 {
		t.Fatalf("ops = %#v", result.Ops)
	}
	wantBlockIndex := []int{1, 0}
	ids := map[int]bool{}
	for i, op := range result.Ops {
		path, _ := op["p"].([]any)
		if len(path) != 5 || path[0] != "itinerary" || path[1] != "sections" || path[3] != "blocks" {
			t.Fatalf("op path = %#v", path)
		}
		if intAny(path[2]) != i || intAny(path[4]) != wantBlockIndex[i] {
			t.Fatalf("op path = %#v want sections.%d.blocks.%d", path, i, wantBlockIndex[i])
		}
		li, _ := op["li"].(map[string]any)
		if li == nil {
			t.Fatalf("missing li on op %d: %#v", i, op)
		}
		id := intAny(li["id"])
		if id == 0 || ids[id] {
			t.Fatalf("block ids not unique: %#v", result.Ops)
		}
		ids[id] = true
		hotel := mapField(li, "hotel")
		if stringField(hotel, "checkIn") != "2026-09-01" || stringField(hotel, "checkOut") != "2026-09-03" {
			t.Fatalf("hotel = %#v", hotel)
		}
		if stringField(mapField(li, "place"), "name") != "Hotel Moon Beach" {
			t.Fatalf("copied name = %#v", li["place"])
		}
	}
	nights, _ := result.Report.Block["nights"].([]map[string]any)
	if len(nights) != 2 {
		t.Fatalf("nights = %#v", result.Report.Block["nights"])
	}
	if nights[0]["date"] != "2026-09-01" || nights[1]["date"] != "2026-09-02" {
		t.Fatalf("night dates = %#v", nights)
	}
	if nights[0]["name"] != "Hotel Moon Beach" || nights[0]["block_id"] == nights[1]["block_id"] {
		t.Fatalf("night rows = %#v", nights)
	}
	if result.Report.Block["resolved_name"] != "Wakasa 2-7-14" || result.Report.Block["distance_m"] != 0 {
		t.Fatalf("report echo = %#v", result.Report.Block)
	}
}

func TestLodgingSpanDisabledUsesSelectedDay(t *testing.T) {
	trip := testFourDayLodgingTrip()
	block := newLodgingReservationBlock(testPlace("Hotel Moon Beach"), planReservationOptions{startDate: "2026-09-01", endDate: "2026-09-03"})
	result, err := buildLodgingReservationInsert(trip, planReservationOptions{
		planEditOptions: planEditOptions{day: 3, sectionIndex: -1, position: -1},
		spanNights:      false,
		startDate:       "2026-09-01",
		endDate:         "2026-09-03",
	}, block, nil)
	if err != nil {
		t.Fatalf("buildLodgingReservationInsert: %v", err)
	}
	if len(result.Ops) != 1 {
		t.Fatalf("ops = %#v", result.Ops)
	}
	path, _ := result.Ops[0]["p"].([]any)
	if intAny(path[2]) != 2 {
		t.Fatalf("expected day 3 section index 2, path = %#v", path)
	}
}

func TestExpectNameSubstringFailsClosed(t *testing.T) {
	if err := expectNameSubstring("Hotel Moon Beach", "moon"); err != nil {
		t.Fatalf("expected match: %v", err)
	}
	if err := expectNameSubstring("Wakasa 2-7-14", "Moon"); err == nil {
		t.Fatal("geocoded street name must fail --expect-name-substring Moon")
	}
	if err := expectNameSubstring("", "Moon"); err == nil {
		t.Fatal("empty resolved name must fail closed")
	}
}

func TestLodgingPlaceResolveEchoDistance(t *testing.T) {
	place := testPlace("Hotel")
	echo := lodgingPlaceResolveEcho(place, 1.25, 2.5)
	if echo["resolved_name"] != "Hotel" || echo["resolved_address"] != "Hotel address" || echo["distance_m"] != 0 {
		t.Fatalf("same-point echo = %#v", echo)
	}
	moved := lodgingPlaceResolveEcho(place, 1.26, 2.5)
	meters, _ := moved["distance_m"].(int)
	if meters < 900 || meters > 1300 {
		t.Fatalf("1.25 to 1.26 deg distance_m = %d", meters)
	}
}
