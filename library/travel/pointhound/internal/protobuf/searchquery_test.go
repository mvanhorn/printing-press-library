package protobuf

import (
	"encoding/base64"
	"testing"
)

func TestEncode_RoundTripsExpectedShape(t *testing.T) {
	// Captured from a real Pointhound search: SFO->LIS on 2026-06-15, economy, 1 traveler.
	// q = CiEKA1NGTxIaU2FuIEZyYW5jaXNjbyBJbnRsIEFpcnBvcnQSOAoDTElTEjFIdW1iZXJ0byBEZWxnYWRvIEFpcnBvcnQgKExpc2JvbiBQb3J0ZWxhIEFpcnBvcnQpGgoyMDI2LTA2LTE1KAEwAQ
	sq := SearchQuery{
		OriginCode:      "SFO",
		OriginName:      "San Francisco Intl Airport",
		DestinationCode: "LIS",
		DestinationName: "Humberto Delgado Airport (Lisbon Portela Airport)",
		Date:            "2026-06-15",
		Cabin:           CabinEconomy,
		Passengers:      1,
	}
	got, err := sq.Encode()
	if err != nil {
		t.Fatal(err)
	}
	want := "CiEKA1NGTxIaU2FuIEZyYW5jaXNjbyBJbnRsIEFpcnBvcnQSOAoDTElTEjFIdW1iZXJ0byBEZWxnYWRvIEFpcnBvcnQgKExpc2JvbiBQb3J0ZWxhIEFpcnBvcnQpGgoyMDI2LTA2LTE1KAEwAQ"
	if got != want {
		gotBytes, _ := base64.RawURLEncoding.DecodeString(got)
		wantBytes, _ := base64.RawURLEncoding.DecodeString(want)
		t.Errorf("encoded q mismatch\n got: %q (%d bytes)\n  raw: % x\nwant: %q (%d bytes)\n  raw: % x",
			got, len(gotBytes), gotBytes, want, len(wantBytes), wantBytes)
	}
}

func TestEncode_RejectsInvalid(t *testing.T) {
	cases := []struct {
		name string
		sq   SearchQuery
	}{
		{"missing origin", SearchQuery{DestinationCode: "TPE", Date: "2026-05-16", Cabin: 1, Passengers: 1}},
		{"missing dest", SearchQuery{OriginCode: "YVR", Date: "2026-05-16", Cabin: 1, Passengers: 1}},
		{"missing date", SearchQuery{OriginCode: "YVR", DestinationCode: "TPE", Cabin: 1, Passengers: 1}},
		{"bad cabin", SearchQuery{OriginCode: "YVR", DestinationCode: "TPE", Date: "2026-05-16", Cabin: 9, Passengers: 1}},
		{"passengers 0", SearchQuery{OriginCode: "YVR", DestinationCode: "TPE", Date: "2026-05-16", Cabin: 1, Passengers: 0}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := c.sq.Encode(); err == nil {
				t.Errorf("expected error for %s, got nil", c.name)
			}
		})
	}
}
