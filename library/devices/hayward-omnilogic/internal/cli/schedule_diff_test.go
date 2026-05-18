package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/devices/hayward-omnilogic/internal/omnilogic"
)

// TestDiffMspConfigs covers the schedule-diff equipment paths beyond heaters.
// Each sub-test mutates a single field/list between two MSP snapshots and
// asserts the corresponding change-kind appears in the diff.
//
// Regression for Greptile #3216324918: the original diffMspConfigs only
// compared heaters, so pump/light/relay/chlorinator/backyard-relay edits
// silently produced "no changes" even though service-tech edits to those
// surfaces are exactly what the command's Long help promises to catch.
func TestDiffMspConfigs(t *testing.T) {
	base := func() *omnilogic.MspConfig {
		return &omnilogic.MspConfig{
			Relays: []omnilogic.Equipment{
				{SystemID: "20", Name: "Landscape", Type: "RLY_HIGH_VOLTAGE", Function: "RLY_LIGHTS"},
			},
			BodiesOfWater: []omnilogic.BodyOfWater{
				{
					SystemID: "1", Name: "Pool",
					Pumps: []omnilogic.Equipment{
						{SystemID: "3", Name: "Filter Pump", Type: "FMT_VARIABLE_SPEED_PUMP", MinSpeed: "18", MaxSpeed: "100"},
					},
					Lights: []omnilogic.Equipment{
						{SystemID: "8", Name: "pool", Type: "COLOR_LOGIC_UCL", V2Active: "no"},
					},
					Relays: []omnilogic.Equipment{
						{SystemID: "30", Name: "Cleaner", Type: "RLY_LOW_VOLTAGE", Function: "RLY_CLEANER"},
					},
					Chlorinator: &omnilogic.Equipment{SystemID: "9", Name: "Salt", Type: "CHLOR", CellType: "3"},
					Heaters: []omnilogic.Heater{
						{SystemID: "4", Name: "Gas", CurrentSetPoint: "75", Enabled: "yes"},
					},
				},
			},
		}
	}

	type tc struct {
		name      string
		mutate    func(after *omnilogic.MspConfig)
		wantKinds []string
	}
	cases := []tc{
		{
			name:      "no-op produces no changes",
			mutate:    func(after *omnilogic.MspConfig) {},
			wantKinds: nil,
		},
		{
			name: "heater setpoint change",
			mutate: func(after *omnilogic.MspConfig) {
				after.BodiesOfWater[0].Heaters[0].CurrentSetPoint = "84"
			},
			wantKinds: []string{"heater-setpoint-changed"},
		},
		{
			name: "pump speed range change",
			mutate: func(after *omnilogic.MspConfig) {
				after.BodiesOfWater[0].Pumps[0].MaxSpeed = "75"
			},
			wantKinds: []string{"pump-speed-range-changed"},
		},
		{
			name: "pump renamed",
			mutate: func(after *omnilogic.MspConfig) {
				after.BodiesOfWater[0].Pumps[0].Name = "Main Pump"
			},
			wantKinds: []string{"pump-renamed"},
		},
		{
			name: "pump removed (also bumps count)",
			mutate: func(after *omnilogic.MspConfig) {
				after.BodiesOfWater[0].Pumps = nil
			},
			wantKinds: []string{"pump-count-changed", "pump-removed"},
		},
		{
			name: "light v2 toggled on",
			mutate: func(after *omnilogic.MspConfig) {
				after.BodiesOfWater[0].Lights[0].V2Active = "yes"
			},
			wantKinds: []string{"light-v2-active-changed"},
		},
		{
			name: "relay function changed (service-tech repurposed)",
			mutate: func(after *omnilogic.MspConfig) {
				after.BodiesOfWater[0].Relays[0].Function = "RLY_WATERFALL"
			},
			wantKinds: []string{"relay-function-changed"},
		},
		{
			name: "chlorinator cell type changed (cell replaced)",
			mutate: func(after *omnilogic.MspConfig) {
				after.BodiesOfWater[0].Chlorinator.CellType = "4"
			},
			wantKinds: []string{"chlorinator-cell-type-changed"},
		},
		{
			name: "chlorinator removed",
			mutate: func(after *omnilogic.MspConfig) {
				after.BodiesOfWater[0].Chlorinator = nil
			},
			wantKinds: []string{"chlorinator-removed"},
		},
		{
			name: "backyard-level relay renamed",
			mutate: func(after *omnilogic.MspConfig) {
				after.Relays[0].Name = "Path Lights"
			},
			wantKinds: []string{"backyard-relay-renamed"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := base()
			after := base()
			tc.mutate(after)
			changes := diffMspConfigs(before, after)
			if tc.wantKinds == nil {
				if len(changes) != 0 {
					t.Errorf("expected no changes, got %d: %+v", len(changes), changes)
				}
				return
			}
			gotKinds := map[string]int{}
			for _, c := range changes {
				k, _ := c["kind"].(string)
				gotKinds[k]++
			}
			for _, want := range tc.wantKinds {
				if gotKinds[want] == 0 {
					t.Errorf("expected change kind %q in output, got: %+v", want, changes)
				}
			}
		})
	}
}
