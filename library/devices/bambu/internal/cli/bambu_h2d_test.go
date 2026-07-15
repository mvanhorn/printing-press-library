package cli

import "testing"

func TestH2DTemperatureViewUsesActivePackedExtruder(t *testing.T) {
	raw := map[string]any{
		"nozzle_temper":        float64(39),
		"nozzle_target_temper": float64(0),
		"device": map[string]any{"extruder": map[string]any{
			"state": float64(33042),
			"info": []any{
				map[string]any{"id": float64(0), "temp": float64(39)},
				map[string]any{"id": float64(1), "temp": float64(14418140)},
			},
		}},
	}
	view := temperatureView(raw)
	if view["active_nozzle_id"] != 1 || view["nozzle_temper"] != 220 || view["nozzle_target_temper"] != 220 {
		t.Fatalf("temperature view = %#v", view)
	}
	nozzles, ok := view["nozzles"].([]map[string]any)
	if !ok || len(nozzles) != 2 || nozzles[1]["active"] != true {
		t.Fatalf("nozzles = %#v", view["nozzles"])
	}
}

func TestActiveTrayUsesSingleH2DMaterialMapping(t *testing.T) {
	raw := map[string]any{
		"mapping": []any{float64(258)},
		"ams": map[string]any{
			"tray_now": "2",
			"ams": []any{
				map[string]any{"tray": []any{map[string]any{}, map[string]any{}, map[string]any{"tray_type": "PC"}, map[string]any{}}},
				map[string]any{"tray": []any{map[string]any{}, map[string]any{}, map[string]any{"tray_type": "PLA", "tray_sub_brands": "PLA Basic"}, map[string]any{}}},
			},
		},
	}
	tray := activeTray(raw)
	if tray == nil || tray["global_id"] != 6 || tray["unit"] != 1 || tray["slot"] != 2 || tray["tray_type"] != "PLA" {
		t.Fatalf("active tray = %#v", tray)
	}
}
