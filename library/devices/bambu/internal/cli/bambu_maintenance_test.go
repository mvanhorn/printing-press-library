package cli

import "testing"

func TestScopedMaintenanceBaselinesIgnoreLegacyRows(t *testing.T) {
	items := []map[string]any{{"task": "nozzle-clean", "legacy_unscoped": true}, {"task": "carbon-rods", "legacy_unscoped": false}}
	completed, ignored := scopedMaintenanceBaselines(items)
	if _, ok := completed["nozzle-clean"]; ok {
		t.Fatal("legacy row became an authoritative baseline")
	}
	if _, ok := completed["carbon-rods"]; !ok || len(ignored) != 1 || ignored[0] != "nozzle-clean" {
		t.Fatalf("completed=%#v ignored=%#v", completed, ignored)
	}
}
