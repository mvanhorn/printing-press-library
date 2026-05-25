package client

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestRewriteFleetSubsetPath(t *testing.T) {
	cases := []struct {
		name, in        string
		includeLocation bool
		wantPath        string
		wantKeys        []string
	}{
		{"charge_state by vin", "/api/1/vehicles/LRWYGCEK3NC240575/data_request/charge_state", false, "/api/1/vehicles/LRWYGCEK3NC240575/vehicle_data?endpoints=charge_state", []string{"charge_state"}},
		{"climate_state by id", "/api/1/vehicles/1689140615856945/data_request/climate_state", false, "/api/1/vehicles/1689140615856945/vehicle_data?endpoints=climate_state", []string{"climate_state"}},
		{"drive_state without location scope stays single endpoint", "/api/1/vehicles/LRW123/data_request/drive_state", false, "/api/1/vehicles/LRW123/vehicle_data?endpoints=drive_state", []string{"drive_state"}},
		{"drive_state with location scope also requests location_data", "/api/1/vehicles/LRW123/data_request/drive_state", true, "/api/1/vehicles/LRW123/vehicle_data?endpoints=drive_state%3Blocation_data", []string{"drive_state", "location_data"}},
		{"location flag does not affect non-drive subsets", "/api/1/vehicles/LRW123/data_request/charge_state", true, "/api/1/vehicles/LRW123/vehicle_data?endpoints=charge_state", []string{"charge_state"}},
		{"full vehicle_data untouched", "/api/1/vehicles/LRW123/vehicle_data", false, "/api/1/vehicles/LRW123/vehicle_data", nil},
		{"products untouched", "/api/1/products", false, "/api/1/products", nil},
		{"command untouched", "/api/1/vehicles/LRW123/command/door_lock", false, "/api/1/vehicles/LRW123/command/door_lock", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotPath, gotKeys := rewriteFleetSubsetPath(tc.in, tc.includeLocation)
			if gotPath != tc.wantPath || !reflect.DeepEqual(gotKeys, tc.wantKeys) {
				t.Errorf("rewriteFleetSubsetPath(%q, %v) = (%q, %v), want (%q, %v)", tc.in, tc.includeLocation, gotPath, gotKeys, tc.wantPath, tc.wantKeys)
			}
		})
	}
}

func TestUnwrapFleetSubset(t *testing.T) {
	t.Run("single key unwrapped to owner-api shape", func(t *testing.T) {
		in := []byte(`{"response":{"charge_state":{"battery_level":48,"charging_state":"Disconnected"},"vehicle_id":123}}`)
		got := string(unwrapFleetSubset(in, []string{"charge_state"}))
		want := `{"response":{"battery_level":48,"charging_state":"Disconnected"}}`
		if got != want {
			t.Errorf("unwrapFleetSubset = %s, want %s", got, want)
		}
	})
	t.Run("drive_state merges location_data fields", func(t *testing.T) {
		in := []byte(`{"response":{"drive_state":{"shift_state":"P","speed":null},"location_data":{"latitude":50.1,"longitude":14.4}}}`)
		got := unwrapFleetSubset(in, []string{"drive_state", "location_data"})
		var env struct {
			Response map[string]json.RawMessage `json:"response"`
		}
		if err := json.Unmarshal(got, &env); err != nil {
			t.Fatalf("result not valid JSON: %v (%s)", err, got)
		}
		for _, k := range []string{"shift_state", "speed", "latitude", "longitude"} {
			if _, ok := env.Response[k]; !ok {
				t.Errorf("merged response missing %q: %s", k, got)
			}
		}
	})
	t.Run("missing key returns input untouched", func(t *testing.T) {
		in := []byte(`{"response":{"climate_state":{}}}`)
		if got := string(unwrapFleetSubset(in, []string{"charge_state"})); got != string(in) {
			t.Errorf("expected untouched, got %s", got)
		}
	})
	t.Run("malformed body returns input untouched", func(t *testing.T) {
		in := []byte(`not json`)
		if got := string(unwrapFleetSubset(in, []string{"charge_state"})); got != string(in) {
			t.Errorf("expected untouched, got %s", got)
		}
	})
}
