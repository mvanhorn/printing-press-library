package consumer

import (
	_ "embed"
	"encoding/json"
	"testing"
)

//go:embed testdata/reservations_list_live_shape.json
var liveReservationsFixture []byte

func TestParseLiveShapedReservationsList(t *testing.T) {
	list, err := parseReservationsPayload(liveReservationsFixture)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("len=%d", len(list))
	}
	r := list[0]
	if r.ID != "100000001" {
		t.Fatalf("id=%q", r.ID)
	}
	if r.RentalID != 100000001 {
		t.Fatalf("rental_id=%d", r.RentalID)
	}
	if r.PriceCents != 1908 {
		t.Fatalf("price_cents=%d", r.PriceCents)
	}
	if r.Price != "$19.08" {
		t.Fatalf("price=%q", r.Price)
	}
	if !r.Cancellable {
		t.Fatal("expected cancellable")
	}
	if r.FacilityTitle != "Example Garage - Covered Valet" {
		t.Fatalf("facility_title=%q", r.FacilityTitle)
	}
	if r.Status != "success" {
		t.Fatalf("status=%q", r.Status)
	}
}

func TestParseReservationPriceAsNumber(t *testing.T) {
	raw := json.RawMessage(`{"rental_id":1,"price":1908,"status":"success"}`)
	list, err := parseReservationsPayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	if list[0].PriceCents != 1908 {
		t.Fatalf("got %d", list[0].PriceCents)
	}
}
