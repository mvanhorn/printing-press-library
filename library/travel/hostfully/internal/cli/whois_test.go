package cli

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/hostfully/internal/store"
)

func TestPhoneDigitsMatch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		a    string
		b    string
		want bool
	}{
		{name: "country code and punctuation", a: "15550100199", b: "5550100199", want: true},
		{name: "same formatted number", a: "5550100199", b: "5550100199", want: true},
		{name: "different number", a: "5550100199", b: "5550109999", want: false},
		{name: "empty", a: "", b: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := phoneDigitsMatch(tt.a, tt.b); got != tt.want {
				t.Fatalf("phoneDigitsMatch(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestWhoisReservationMatchesAlternatePhone(t *testing.T) {
	t.Parallel()
	r := reservation{Phone: "+1 425 555 0100", CellPhone: "+1 555 010 0177"}
	if !whoisReservationMatches(r, "555-010-0177") {
		t.Fatal("alternate cell phone should identify the guest")
	}
}

func TestParseReservationDerivesPaidAndBalanceDueStates(t *testing.T) {
	t.Parallel()
	paid, err := parseReservation([]byte(`{
		"uid":"paid","order":{
			"currency":"USD","totalAmount":"550.05","amountPaid":"550.05",
			"balance":"0.00","transactions":[{"status":"SUCCESS"}]
		},"pinCode":{"pinCode":"1234"}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if paid.Paid == nil || !*paid.Paid || paid.PaymentStatus != "paid" {
		t.Fatalf("paid reservation = %#v", paid)
	}
	if paid.TotalAmount != "550.05" || paid.AmountPaid != "550.05" ||
		paid.Balance != "0.00" || paid.Currency != "USD" {
		t.Fatalf("payment amounts = %#v", paid)
	}
	if len(paid.PaymentTransactionStatuses) != 1 ||
		paid.PaymentTransactionStatuses[0] != "SUCCESS" {
		t.Fatalf("transaction statuses = %#v", paid.PaymentTransactionStatuses)
	}
	if paid.DoorPIN != "1234" {
		t.Fatalf("paid door PIN = %q", paid.DoorPIN)
	}

	due, err := parseReservation([]byte(`{
		"uid":"due","order":{
			"currency":"USD","totalAmount":"500.00","amountPaid":"100.00",
			"balance":"400.00","transactions":[{"status":"SUCCESS"}]
		},"pinCode":{"pinCode":"5678"}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if due.Paid == nil || *due.Paid || due.PaymentStatus != "balance_due" {
		t.Fatalf("balance-due reservation = %#v", due)
	}
	if due.DoorPIN != "5678" {
		t.Fatalf("balance-due Hostfully door PIN = %q", due.DoorPIN)
	}

	unverified, err := parseReservation([]byte(`{"uid":"unknown","pinCode":{"pinCode":"9012"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if unverified.Paid != nil || unverified.PaymentStatus != "unverified" {
		t.Fatalf("unverified reservation = %#v", unverified)
	}
	if unverified.DoorPIN != "9012" {
		t.Fatalf("unverified Hostfully door PIN = %q", unverified.DoorPIN)
	}
}

func TestMergeReservationPaymentPreservesRESTContext(t *testing.T) {
	t.Parallel()
	paid := true
	target := reservation{UID: "lead-1", Channel: "AIRBNB", GuestLast: "Guest"}
	indexed := reservation{
		UID: "lead-1", Paid: &paid, PaymentStatus: "paid", TotalAmount: "200.00",
		AmountPaid: "200.00", Balance: "0.00", Currency: "USD",
		PaymentTransactionStatuses: []string{"SUCCESS"}, DoorPIN: "1234",
	}
	mergeReservationPayment(&target, indexed)
	if target.Channel != "AIRBNB" || target.GuestLast != "Guest" {
		t.Fatalf("REST context was overwritten: %#v", target)
	}
	if target.Paid == nil || !*target.Paid || target.PaymentStatus != "paid" ||
		target.Balance != "0.00" || target.DoorPIN != "1234" {
		t.Fatalf("payment context was not merged: %#v", target)
	}
}

func TestFindWhoisStaysReturnsFullHistoryNewestFirst(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	reservations := []reservation{
		{GuestFirst: "Jamie", GuestLast: "Guest", Phone: "+1 (555) 010-0199", Unit: 2, CheckIn: now.Add(-48 * time.Hour), CheckOut: now.Add(-24 * time.Hour), Status: "CLOSED", Channel: "DIRECT"},
		{GuestFirst: "Jamie", GuestLast: "Guest", Phone: "555-010-0199", Unit: 3, CheckIn: now.Add(-time.Hour), CheckOut: now.Add(24 * time.Hour), Status: "BOOKED", Channel: "AIRBNB", BookingType: "BOOKING", Adults: 2, Children: 1, Pets: 1, Confirmation: "CONF-1", ArrivalNote: "about 4 PM", PaymentStatus: "paid", DoorPIN: "1234"},
		{GuestFirst: "Other", GuestLast: "Person", Phone: "425-555-0000", Unit: 4, CheckIn: now, CheckOut: now.Add(24 * time.Hour), Status: "BOOKED"},
	}

	got := findWhoisStays(reservations, "5550100199", now)
	if len(got) != 2 {
		t.Fatalf("len(stays) = %d, want 2", len(got))
	}
	if got[0].Stay != "current" || got[0].Unit != 3 {
		t.Fatalf("first stay = %#v, want current Unit 3", got[0])
	}
	if got[0].BookingType != "BOOKING" || got[0].Adults != 2 || got[0].Children != 1 || got[0].Pets != 1 || got[0].Confirmation != "CONF-1" || got[0].ArrivalNote != "about 4 PM" {
		t.Fatalf("context fields not preserved: %#v", got[0])
	}
	if got[0].PaymentStatus != "paid" || got[0].DoorPIN != "1234" {
		t.Fatalf("payment status not preserved: %#v", got[0])
	}
	if got[1].Stay != "past" || got[1].Status != "CLOSED" {
		t.Fatalf("historical stay = %#v, want past CLOSED", got[1])
	}
}

func TestFindWhoisStaysByName(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	reservations := []reservation{
		{GuestFirst: "Jamie", GuestLast: "Guestman", CheckIn: now.Add(24 * time.Hour), CheckOut: now.Add(48 * time.Hour)},
		{GuestFirst: "Other", GuestLast: "Person", CheckIn: now, CheckOut: now.Add(time.Hour)},
	}
	got := findWhoisStays(reservations, "guestm", now)
	if len(got) != 1 || got[0].Name != "Jamie Guestman" || got[0].Stay != "upcoming" {
		t.Fatalf("stays = %#v", got)
	}
}

func TestLoadWhoisReservationsIncludesClosedAndCancelled(t *testing.T) {
	db, err := store.OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, tc := range []struct {
		id     string
		status string
	}{
		{id: "closed", status: "CLOSED"},
		{id: "cancelled", status: "CANCELLED"},
	} {
		data, err := json.Marshal(map[string]any{
			"uid": tc.id, "status": tc.status,
			"checkInLocalDateTime":  "2026-01-01T15:00:00",
			"checkOutLocalDateTime": "2026-01-02T11:00:00",
			"guestInformation":      map[string]any{"firstName": "Past", "lastName": "Guest", "phoneNumber": "5550100199"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Upsert("leads", tc.id, data); err != nil {
			t.Fatal(err)
		}
	}

	reservations, err := loadWhoisReservations(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(reservations) != 2 {
		t.Fatalf("len(reservations) = %d, want 2", len(reservations))
	}
}

func TestFetchWhoisReservationsAcrossUnitsRunsPropertiesConcurrently(t *testing.T) {
	t.Parallel()
	properties := map[int]string{
		1: testUnitUIDs[1],
		2: testUnitUIDs[2],
		3: testUnitUIDs[3],
		4: testUnitUIDs[4],
	}
	var mu sync.Mutex
	active, maxActive := 0, 0
	graphql := &fakeHostfullyLeadAPI{}
	graphql.post = func(_ string, body any) (json.RawMessage, int, error) {
		mu.Lock()
		active++
		if active > maxActive {
			maxActive = active
		}
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
		mu.Lock()
		active--
		mu.Unlock()
		encoded, _ := json.Marshal(body)
		query := string(encoded)
		propertyUID := ""
		for _, uid := range properties {
			if strings.Contains(query, uid) {
				propertyUID = uid
				break
			}
		}
		lead, _ := json.Marshal(map[string]any{
			"uid":                   propertyUID,
			"propertyUid":           propertyUID,
			"checkInLocalDateTime":  "2026-08-30T15:00:00",
			"checkOutLocalDateTime": "2026-08-31T11:00:00",
			"guestInformation":      map[string]any{"firstName": "Test", "lastName": "Guest"},
		})
		page, _ := json.Marshal(map[string]any{
			"data":       map[string]any{"leads": []json.RawMessage{lead}},
			"extensions": map[string]any{},
		})
		return page, 200, nil
	}

	got, err := fetchWhoisReservationsAcrossUnits(
		graphql, properties, 4,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(properties) {
		t.Fatalf("reservations = %d, want %d", len(got), len(properties))
	}
	if maxActive < 2 {
		t.Fatalf("max concurrent requests = %d, want at least 2", maxActive)
	}
}

func TestFetchWhoisReservationsAcrossUnitsRejectsPartialHistory(t *testing.T) {
	t.Parallel()
	graphql := &fakeHostfullyLeadAPI{}
	graphql.post = func(_ string, body any) (json.RawMessage, int, error) {
		encoded, _ := json.Marshal(body)
		if strings.Contains(string(encoded), testUnitUIDs[2]) {
			return nil, 0, errors.New("upstream unavailable")
		}
		return json.RawMessage(`{"data":{"leads":[]},"extensions":{}}`), 200, nil
	}

	got, err := fetchWhoisReservationsAcrossUnits(
		graphql, map[int]string{1: testUnitUIDs[1], 2: testUnitUIDs[2]}, 2,
	)
	if err == nil || !strings.Contains(err.Error(), "unit 2") {
		t.Fatalf("error = %v, want unit 2 failure", err)
	}
	if got != nil {
		t.Fatalf("partial reservations returned: %#v", got)
	}
}

func TestFetchWhoisLeadsByUIDMaterializesRichRESTContext(t *testing.T) {
	t.Parallel()
	rest := &fakeHostfullyLeadAPI{}
	rest.get = func(path string, params map[string]string) (json.RawMessage, error) {
		if path != "/leads" || params["agencyUid"] != "agency" || params["leadUids"] != "lead-1" {
			t.Fatalf("request = %s %#v", path, params)
		}
		return json.RawMessage(`{"leads":[{
			"uid":"lead-1",
			"propertyUid":"` + testUnitUIDs[7] + `",
			"checkInLocalDateTime":"2026-09-05T15:00:00",
			"checkOutLocalDateTime":"2026-09-06T11:00:00",
			"status":"BOOKED",
			"channel":"WHIMSTAY",
			"type":"BOOKING",
			"externalBookingId":"CONF-1",
			"notes":"Time of arrival: 4 PM",
			"guestInformation":{
				"firstName":"Dara","lastName":"Guest",
				"phoneNumber":"4255550100","cellPhoneNumber":"5550100177",
				"adultCount":4,"childrenCount":1,"petCount":1
			}
		}]}`), nil
	}

	got, err := fetchWhoisLeadsByUID(rest, "agency", []string{"lead-1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("reservations = %d, want 1", len(got))
	}
	r := got[0]
	if r.Channel != "WHIMSTAY" || r.BookingType != "BOOKING" ||
		r.CellPhone != "5550100177" || r.Confirmation != "CONF-1" ||
		r.ArrivalNote != "4 PM" || r.Adults != 4 || r.Children != 1 || r.Pets != 1 {
		t.Fatalf("rich context was not preserved: %#v", r)
	}
}

func TestGraphQLWhoisIndexStaysLightweightAndPaymentQueryIsTargeted(t *testing.T) {
	t.Parallel()
	for _, field := range []string{
		"type", "source", "externalBookingId", "notes", "extraNotes",
		"phoneNumber", "cellPhoneNumber", "adultCount", "childrenCount", "petCount",
	} {
		if !strings.Contains(hostfullyGraphQLLeadFields, field) {
			t.Errorf("GraphQL lead index is missing %q", field)
		}
	}
	if strings.Contains(hostfullyGraphQLLeadFields, "order") {
		t.Error("GraphQL lead index must not load payment orders for complete history")
	}
	for _, field := range []string{
		"order", "currency", "totalAmount", "amountPaid", "balance", "transactions", "status",
	} {
		if !strings.Contains(hostfullyGraphQLSecureFields, field) {
			t.Errorf("targeted GraphQL payment fields are missing %q", field)
		}
	}
	if !strings.Contains(hostfullyGraphQLSecureFields, "pinCode { pinCode }") {
		t.Error("targeted GraphQL secure fields are missing the Hostfully door PIN")
	}
}

func TestFetchWhoisSecureDetailsByUIDsReturnsPaymentAndHostfullyPIN(t *testing.T) {
	t.Parallel()
	graphql := &fakeHostfullyLeadAPI{}
	graphql.post = func(path string, body any) (json.RawMessage, int, error) {
		if path != "/graphql" {
			t.Fatalf("path = %q", path)
		}
		encoded, _ := json.Marshal(body)
		query := string(encoded)
		for _, want := range []string{`lead0: lead(uid:\"lead-1\")`, `lead1: lead(uid:\"lead-2\")`} {
			if !strings.Contains(query, want) {
				t.Fatalf("query %s does not contain %s", query, want)
			}
		}
		return json.RawMessage(`{"data":{
			"lead0":{"uid":"lead-1","pinCode":{"pinCode":"1234"},"order":{"currency":"USD","totalAmount":"200.00","amountPaid":"200.00","balance":"0.00","transactions":[{"status":"SUCCESS"}]}},
			"lead1":{"uid":"lead-2","pinCode":{"pinCode":"5678"},"order":{"currency":"USD","totalAmount":"300.00","amountPaid":"0.00","balance":"300.00","transactions":[]}}
		}}`), 200, nil
	}

	got, err := fetchWhoisSecureDetailsByUIDs(graphql, []string{"lead-1", "lead-2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Paid == nil || !*got[0].Paid || got[0].PaymentStatus != "paid" || got[0].DoorPIN != "1234" {
		t.Fatalf("paid result = %#v", got)
	}
	if got[1].Paid == nil || *got[1].Paid || got[1].PaymentStatus != "balance_due" || got[1].DoorPIN != "5678" {
		t.Fatalf("balance-due result = %#v", got)
	}
}

func TestFetchWhoisSecureDetailsByUIDsRejectsPartialResponse(t *testing.T) {
	t.Parallel()
	graphql := &fakeHostfullyLeadAPI{}
	graphql.post = func(_ string, _ any) (json.RawMessage, int, error) {
		return json.RawMessage(`{"data":{"lead0":{"uid":"lead-1","order":null}}}`), 200, nil
	}

	got, err := fetchWhoisSecureDetailsByUIDs(graphql, []string{"lead-1", "lead-2"})
	if err == nil || !strings.Contains(err.Error(), "lead-2") {
		t.Fatalf("error = %v, want omitted lead-2", err)
	}
	if got != nil {
		t.Fatalf("partial payments returned: %#v", got)
	}
}

func TestValidWhoisQuery(t *testing.T) {
	for _, ok := range []string{"+1 (555) 010-0199", "Jamie", "O'Neil-Smith Jr."} {
		if !validWhoisQuery(ok) {
			t.Fatalf("expected %q valid", ok)
		}
	}
	for _, bad := range []string{"", "  ", "__x__", "a;b", "x@y"} {
		if validWhoisQuery(bad) {
			t.Fatalf("expected %q invalid", bad)
		}
	}
}
