package client

import "testing"

func TestRewriteFleetSubsetPath(t *testing.T) {
	cases := []struct {
		name, in, wantPath, wantKey string
	}{
		{"charge_state by vin", "/api/1/vehicles/LRWYGCEK3NC240575/data_request/charge_state", "/api/1/vehicles/LRWYGCEK3NC240575/vehicle_data?endpoints=charge_state", "charge_state"},
		{"climate_state by id", "/api/1/vehicles/1689140615856945/data_request/climate_state", "/api/1/vehicles/1689140615856945/vehicle_data?endpoints=climate_state", "climate_state"},
		{"drive_state stays single endpoint (no location_data; needs vehicle_location scope)", "/api/1/vehicles/LRW123/data_request/drive_state", "/api/1/vehicles/LRW123/vehicle_data?endpoints=drive_state", "drive_state"},
		{"full vehicle_data untouched", "/api/1/vehicles/LRW123/vehicle_data", "/api/1/vehicles/LRW123/vehicle_data", ""},
		{"products untouched", "/api/1/products", "/api/1/products", ""},
		{"command untouched", "/api/1/vehicles/LRW123/command/door_lock", "/api/1/vehicles/LRW123/command/door_lock", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotPath, gotKey := rewriteFleetSubsetPath(tc.in)
			if gotPath != tc.wantPath || gotKey != tc.wantKey {
				t.Errorf("rewriteFleetSubsetPath(%q) = (%q, %q), want (%q, %q)", tc.in, gotPath, gotKey, tc.wantPath, tc.wantKey)
			}
		})
	}
}

func TestUnwrapFleetSubset(t *testing.T) {
	t.Run("unwraps to owner-api shape", func(t *testing.T) {
		in := []byte(`{"response":{"charge_state":{"battery_level":48,"charging_state":"Disconnected"},"vehicle_id":123}}`)
		got := string(unwrapFleetSubset(in, "charge_state"))
		want := `{"response":{"battery_level":48,"charging_state":"Disconnected"}}`
		if got != want {
			t.Errorf("unwrapFleetSubset = %s, want %s", got, want)
		}
	})
	t.Run("missing key returns input untouched", func(t *testing.T) {
		in := []byte(`{"response":{"climate_state":{}}}`)
		if got := string(unwrapFleetSubset(in, "charge_state")); got != string(in) {
			t.Errorf("expected untouched, got %s", got)
		}
	})
	t.Run("malformed body returns input untouched", func(t *testing.T) {
		in := []byte(`not json`)
		if got := string(unwrapFleetSubset(in, "charge_state")); got != string(in) {
			t.Errorf("expected untouched, got %s", got)
		}
	})
}
