// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseVehicleArgPreservesMultiwordModel(t *testing.T) {
	got, err := parseVehicleArg("2021 LandRover Range Rover")
	if err != nil {
		t.Fatal(err)
	}
	if got.Year != 2021 || got.Make != "LandRover" || got.Model != "Range Rover" {
		t.Fatalf("unexpected vehicle: %#v", got)
	}
}

func TestComponentCountsOrdersByFrequency(t *testing.T) {
	got := componentCounts([]map[string]any{{"components": "AIR BAGS"}, {"components": "BRAKES"}, {"components": "AIR BAGS"}})
	if len(got) != 2 || got[0]["component"] != "AIR BAGS" || got[0]["count"] != 2 {
		t.Fatalf("unexpected counts: %#v", got)
	}
}

func TestReadCommunicationFileFiltersExactVehicle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "communications.tsv")
	data := "123\t\t20260701\tTSB-1\t20260630\t\tService Bulletin / Repair Instructions\tHONDA\tCIVIC\t2020\tAIR BAGS\tRestraints\tInflator\tInspect the inflator\n" +
		"124\t\t20260701\tTSB-2\t20260630\t\tOther\tHONDA\tACCORD\t2020\tENGINE\tEngine\t\tOther model\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readCommunicationFile(path, vehicleQuery{Year: 2020, Make: "Honda", Model: "Civic"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].DocumentID != "TSB-1" {
		t.Fatalf("unexpected matches: %#v", got)
	}
}
