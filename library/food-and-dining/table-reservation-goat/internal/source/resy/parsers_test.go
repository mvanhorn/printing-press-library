// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package resy

import (
	"strings"
	"testing"
)

func TestParseSearchResponsePrefersNonEmptyEnvelope(t *testing.T) {
	// Resy has been observed returning either {search:{hits:[...]}} or
	// top-level {hits:[...]}. A payload carrying both with the nested
	// one empty MUST fall back to the top-level hits, not silently
	// drop everything.
	body := []byte(`{
		"search": {"hits": []},
		"hits": [{"objectID":"1387","name":"Le Bernardin"}]
	}`)
	venues, err := ParseSearchResponse(body)
	if err != nil {
		t.Fatalf("ParseSearchResponse: %v", err)
	}
	if len(venues) != 1 {
		t.Fatalf("got %d venues; want 1 (top-level fallback)", len(venues))
	}
	if venues[0].ID != "1387" || venues[0].Name != "Le Bernardin" {
		t.Errorf("venues[0] = %+v; want id=1387 Le Bernardin", venues[0])
	}
}

func TestParseSearchResponsePrefersGeoloc(t *testing.T) {
	body := []byte(`{
		"search": {
			"hits": [
				{"objectID":"1387","name":"Le Bernardin","url_slug":"le-bernardin","location":{"code":"ny","name":"New York"},"_geoloc":{"lat":40.7614,"lng":-73.9821}},
				{"objectID":"4567","name":"Atomix","url_slug":"atomix","_geoloc":{"lat":40.7457,"lng":-73.9826}},
				{"objectID":"9999","name":"Top-Level Coords","latitude":34.0522,"longitude":-118.2437}
			]
		}
	}`)
	venues, err := ParseSearchResponse(body)
	if err != nil {
		t.Fatalf("ParseSearchResponse: %v", err)
	}
	if len(venues) != 3 {
		t.Fatalf("got %d venues, want 3", len(venues))
	}
	if venues[0].Latitude != 40.7614 || venues[0].Longitude != -73.9821 {
		t.Errorf("venues[0] lat/lng = (%v, %v); want (40.7614, -73.9821)", venues[0].Latitude, venues[0].Longitude)
	}
	if venues[1].Latitude != 40.7457 {
		t.Errorf("venues[1] lat = %v; want 40.7457", venues[1].Latitude)
	}
	// Top-level latitude/longitude is the fallback when _geoloc is absent.
	if venues[2].Latitude != 34.0522 || venues[2].Longitude != -118.2437 {
		t.Errorf("venues[2] lat/lng = (%v, %v); want (34.0522, -118.2437)", venues[2].Latitude, venues[2].Longitude)
	}
}

func TestParseSearchResponseHandlesBothIDShapes(t *testing.T) {
	body := []byte(`{
		"search": {
			"hits": [
				{"id":{"resy":1387}, "name":"Le Bernardin", "url_slug":"le-bernardin", "location":{"code":"ny","name":"New York"}, "region":"NY"},
				{"objectID":"8033", "name":"Carbone", "url_slug":"carbone", "location":{"code":"ny","name":"New York"}},
				{"name":"missing-id","url_slug":"x"},
				{"id":{"resy":"4567"},"name":"Atomix","url_slug":"atomix","location":{"code":"ny","name":"New York"}}
			]
		}
	}`)
	venues, err := ParseSearchResponse(body)
	if err != nil {
		t.Fatalf("ParseSearchResponse: %v", err)
	}
	if len(venues) != 3 {
		t.Fatalf("got %d venues, want 3 (drop id-less row)", len(venues))
	}
	if venues[0].ID != "1387" || venues[0].Name != "Le Bernardin" {
		t.Errorf("venues[0] = %+v", venues[0])
	}
	if venues[0].URL != "https://resy.com/cities/ny/le-bernardin" {
		t.Errorf("venues[0].URL = %q", venues[0].URL)
	}
	if venues[1].ID != "8033" {
		t.Errorf("venues[1].ID = %q, want 8033", venues[1].ID)
	}
	if venues[2].ID != "4567" {
		t.Errorf("venues[2].ID = %q, want 4567 (string variant)", venues[2].ID)
	}
}

func TestParseAvailabilityResponseExtractsSlotsAndTime(t *testing.T) {
	body := []byte(`{
		"results": {
			"venues": [
				{
					"venue": {"id":{"resy":1387}, "name":"Le Bernardin"},
					"slots": [
						{"date":{"start":"2026-05-15 19:00:00"},"config":{"id":42,"token":"tok-a","type":"Dining Room"},"size":{"min":1,"max":4}},
						{"date":{"start":"2026-05-15 21:30:00"},"config":{"id":43,"token":"tok-b","type":"Bar"}},
						{"date":{"start":"2026-05-15 22:00"},"config":{"token":""}},
						{"config":{"token":"x"}}
					]
				}
			]
		}
	}`)
	slots, err := ParseAvailabilityResponse(body)
	if err != nil {
		t.Fatalf("ParseAvailabilityResponse: %v", err)
	}
	if len(slots) != 2 {
		t.Fatalf("got %d slots, want 2 (skip token-less + date-less rows)", len(slots))
	}
	if slots[0].Token != "tok-a" || slots[0].Time != "19:00" || slots[0].Type != "Dining Room" {
		t.Errorf("slots[0] = %+v", slots[0])
	}
	if slots[0].PartySize != 4 {
		t.Errorf("slots[0].PartySize = %d, want 4", slots[0].PartySize)
	}
	if slots[1].Token != "tok-b" || slots[1].Time != "21:30" {
		t.Errorf("slots[1] = %+v", slots[1])
	}
}

func TestParseResyTime(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"2026-05-15 19:00:00", "19:00"},
		{"2026-05-15 21:30", "21:30"},
		{"19:00:00", "19:00"},
		{"19:00", "19:00"},
		{"bogus", "bogus"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := ParseResyTime(tc.in); got != tc.want {
			t.Errorf("ParseResyTime(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestExtractVenueNameFromShare(t *testing.T) {
	cases := []struct {
		name  string
		share *resyShare
		want  string
	}{
		{
			name:  "nil share",
			share: nil,
			want:  "",
		},
		{
			name:  "RSVP phrasing",
			share: &resyShare{GenericMessage: "Please RSVP for Nishino on January 18 at 5:30PM"},
			want:  "Nishino",
		},
		{
			name:  "Reservation at phrasing prefers end anchor",
			share: &resyShare{Message: []resyShareMessage{{Title: "RSVP for our Reservation at Atomix"}}},
			want:  "Atomix",
		},
		{
			name: "Reservation at beats RSVP when both present",
			share: &resyShare{
				GenericMessage: "Please RSVP for Other on Jan 18 at 7:00PM",
				Message:        []resyShareMessage{{Title: "Reservation at Carbone"}},
			},
			want: "Carbone",
		},
		{
			name:  "no match returns empty",
			share: &resyShare{GenericMessage: "no patterns here"},
			want:  "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExtractVenueNameFromShare(tc.share); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseReservationsResponseModernAndLegacyShapes(t *testing.T) {
	body := []byte(`{
		"reservations": [
			{
				"resy_token": "rt-modern",
				"day": "2026-06-01",
				"num_seats": 2,
				"venue": {"id": 1387},
				"time_slot": "19:30:00",
				"share": {"generic_message": "Please RSVP for Le Bernardin on June 1 at 7:30PM"},
				"status": {"finished": 0}
			},
			{
				"resy_token": "rt-legacy",
				"num_seats": 4,
				"venue": {"id": 8033, "name": "Carbone"},
				"time_slot": {"date": "2026-06-02 20:00:00"},
				"status": "Completed"
			},
			{
				"reservation_id": 999,
				"day": "2026-06-03",
				"num_seats": 6,
				"time_slot": "21:00:00",
				"share": {"message":[{"title":"Reservation at Atomix"}]}
			},
			{"foo":"bar"}
		]
	}`)
	rows, err := ParseReservationsResponse(body)
	if err != nil {
		t.Fatalf("ParseReservationsResponse: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3 (drop id-less row)", len(rows))
	}
	if rows[0].ID != "rt-modern" || rows[0].VenueName != "Le Bernardin" || rows[0].Time != "19:30" {
		t.Errorf("rows[0] = %+v", rows[0])
	}
	if rows[1].VenueName != "Carbone" || rows[1].Date != "2026-06-02" || rows[1].Time != "20:00" {
		t.Errorf("rows[1] = %+v", rows[1])
	}
	if rows[1].Status != "Completed" {
		t.Errorf("rows[1].Status = %q, want Completed", rows[1].Status)
	}
	if rows[2].ID != "999" || rows[2].VenueName != "Atomix" {
		t.Errorf("rows[2] = %+v", rows[2])
	}
}

func TestParseReservationsResponseReadsLegacyDateStart(t *testing.T) {
	// Legacy reservation rows omit `day` and `time_slot` and carry
	// the datetime as `date.start`. Parser must read it and produce a
	// non-empty Date/Time so the idempotency preflight in bookOnResy
	// can prevent double-booking.
	body := []byte(`{
		"reservations": [
			{
				"resy_token": "rt-legacy-datestart",
				"num_seats": 2,
				"venue": {"id": 1387, "name": "Le Bernardin"},
				"date": {"start": "2026-07-04 20:00:00"}
			}
		]
	}`)
	rows, err := ParseReservationsResponse(body)
	if err != nil {
		t.Fatalf("ParseReservationsResponse: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows; want 1", len(rows))
	}
	r := rows[0]
	if r.Date != "2026-07-04" {
		t.Errorf("Date = %q; want 2026-07-04", r.Date)
	}
	if r.Time != "20:00" {
		t.Errorf("Time = %q; want 20:00", r.Time)
	}
}

func TestFilterUpcoming(t *testing.T) {
	rs := []UpcomingReservation{
		{ID: "a", Date: "2026-04-01"},
		{ID: "b", Date: "2026-05-15"},
		{ID: "c", Date: "2026-06-01"},
	}
	got := FilterUpcoming(rs, "2026-05-01")
	if len(got) != 2 || got[0].ID != "b" || got[1].ID != "c" {
		t.Errorf("FilterUpcoming = %+v", got)
	}
}

func TestPickPaymentMethodPrefersDefault(t *testing.T) {
	user := &struct {
		PaymentMethods []paymentMethod `json:"payment_methods"`
	}{
		PaymentMethods: []paymentMethod{
			{ID: []byte(`100`), IsDefault: false},
			{ID: []byte(`200`), IsDefault: true},
		},
	}
	got, err := pickPaymentMethod(user, "")
	if err != nil {
		t.Fatalf("pickPaymentMethod: %v", err)
	}
	if got != "200" {
		t.Errorf("got %q, want 200", got)
	}
}

func TestPickPaymentMethodFallsBackToFirst(t *testing.T) {
	user := &struct {
		PaymentMethods []paymentMethod `json:"payment_methods"`
	}{
		PaymentMethods: []paymentMethod{
			{ID: []byte(`100`)},
			{ID: []byte(`200`)},
		},
	}
	got, _ := pickPaymentMethod(user, "")
	if got != "100" {
		t.Errorf("got %q, want 100", got)
	}
}

func TestPickPaymentMethodHonorsOverride(t *testing.T) {
	got, err := pickPaymentMethod(nil, "explicit")
	if err != nil || got != "explicit" {
		t.Errorf("got (%q,%v); want (explicit,nil)", got, err)
	}
}

func TestPickPaymentMethodErrorsWithoutMethods(t *testing.T) {
	_, err := pickPaymentMethod(nil, "")
	if err != ErrNoPaymentMethod {
		t.Errorf("got %v, want ErrNoPaymentMethod", err)
	}
}

func TestIsSlotTakenMessage(t *testing.T) {
	hits := []string{
		"Slot is no longer available",
		"slot no longer available",
		"INVALID BOOK TOKEN",
		"invalid configuration id for venue 9999",
	}
	for _, msg := range hits {
		if !isSlotTakenMessage(msg) {
			t.Errorf("expected slot-taken classification for %q", msg)
		}
	}
	misses := []string{
		"no payment method on file",
		"please log in",
		"",
		"venue closed for renovation",
	}
	for _, msg := range misses {
		if isSlotTakenMessage(msg) {
			t.Errorf("did not expect slot-taken classification for %q", msg)
		}
	}
}

func TestParseAvailabilityResponseHandlesEmptyEnvelope(t *testing.T) {
	cases := [][]byte{
		[]byte(`{}`),
		[]byte(`{"results":null}`),
		[]byte(`{"results":{"venues":[]}}`),
	}
	for _, body := range cases {
		slots, err := ParseAvailabilityResponse(body)
		if err != nil {
			t.Errorf("ParseAvailabilityResponse(%s): %v", string(body), err)
		}
		if len(slots) != 0 {
			t.Errorf("ParseAvailabilityResponse(%s) = %+v; want []", string(body), slots)
		}
	}
}

func TestUnquoteJSONStripsString(t *testing.T) {
	got := unquoteJSON([]byte(`"hello"`))
	if got != "hello" {
		t.Errorf("got %q", got)
	}
}

func TestUnquoteJSONLeavesNumberAlone(t *testing.T) {
	got := unquoteJSON([]byte(`12345`))
	if got != "12345" {
		t.Errorf("got %q", got)
	}
}

func TestTruncateBodyBoundary(t *testing.T) {
	long := strings.Repeat("a", 250)
	got := truncateBody([]byte(long))
	if !strings.HasSuffix(got, "...") {
		t.Errorf("expected ...-suffix, got %q", got[len(got)-5:])
	}
	if len(got) != 203 {
		t.Errorf("len = %d, want 203", len(got))
	}
}
