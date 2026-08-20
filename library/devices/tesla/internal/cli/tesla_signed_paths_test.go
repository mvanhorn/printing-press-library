// Table-driven tests for tesla_signed_paths.go. Covers every cell of the
// decision matrix described in the 2026-05-22-001 plan, plus the explicit
// --via override errors.
package cli

import (
	"strings"
	"testing"
)

func TestSignedPaths_PickPath(t *testing.T) {
	type tc struct {
		name          string
		cmdClass      CommandClass
		vehicleClass  string
		fleetReady    bool
		hermesRunning bool
		via           string
		wantPath      string
		wantErr       bool
		// wantErrSub matches err.Error() when wantErr is true.
		wantErrSub string
	}
	cases := []tc{
		// REST-friendly cars: legacy REST wins for every command class on auto.
		{
			name:         "rest_friendly + owner-api + auto = rest",
			cmdClass:     ClassOwnerAPI,
			vehicleClass: VehicleClassRESTFriendly,
			via:          "auto",
			wantPath:     PathREST,
		},
		{
			name:         "rest_friendly + vcsec + auto = rest (legacy REST still works for old cars)",
			cmdClass:     ClassVCSEC,
			vehicleClass: VehicleClassRESTFriendly,
			via:          "auto",
			wantPath:     PathREST,
		},
		{
			name:         "rest_friendly + wake + auto = rest",
			cmdClass:     ClassWake,
			vehicleClass: VehicleClassRESTFriendly,
			via:          "auto",
			wantPath:     PathREST,
		},
		// Signed-cmd cars: KD6 defaults.
		{
			name:          "signed_cmd + owner-api + Fleet + relay running -> hermes (KD6 cost-honest default)",
			cmdClass:      ClassOwnerAPI,
			vehicleClass:  VehicleClassSignedCmd,
			fleetReady:    true,
			hermesRunning: true,
			via:           "auto",
			wantPath:      PathHermes,
		},
		{
			name:          "signed_cmd + owner-api + Fleet + relay NOT running -> fleet",
			cmdClass:      ClassOwnerAPI,
			vehicleClass:  VehicleClassSignedCmd,
			fleetReady:    true,
			hermesRunning: false,
			via:           "auto",
			wantPath:      PathFleet,
		},
		{
			name:          "signed_cmd + owner-api + only BLE (no Fleet, no relay) -> ble",
			cmdClass:      ClassOwnerAPI,
			vehicleClass:  VehicleClassSignedCmd,
			fleetReady:    false,
			hermesRunning: false,
			via:           "auto",
			wantPath:      PathBLE,
		},
		{
			name:          "signed_cmd + vcsec + Fleet + relay running -> fleet (Hermes cannot VCSEC)",
			cmdClass:      ClassVCSEC,
			vehicleClass:  VehicleClassSignedCmd,
			fleetReady:    true,
			hermesRunning: true,
			via:           "auto",
			wantPath:      PathFleet,
		},
		{
			name:          "signed_cmd + vcsec + no Fleet -> ble (Hermes cannot VCSEC)",
			cmdClass:      ClassVCSEC,
			vehicleClass:  VehicleClassSignedCmd,
			fleetReady:    false,
			hermesRunning: true,
			via:           "auto",
			wantPath:      PathBLE,
		},
		{
			name:          "signed_cmd + wake + Fleet -> fleet (Hermes wake_up is buggy)",
			cmdClass:      ClassWake,
			vehicleClass:  VehicleClassSignedCmd,
			fleetReady:    true,
			hermesRunning: true,
			via:           "auto",
			wantPath:      PathFleet,
		},
		{
			name:          "signed_cmd + wake + no Fleet -> ble",
			cmdClass:      ClassWake,
			vehicleClass:  VehicleClassSignedCmd,
			fleetReady:    false,
			hermesRunning: true,
			via:           "auto",
			wantPath:      PathBLE,
		},
		// Explicit overrides.
		{
			name:         "--via=fleet without Fleet creds -> error",
			cmdClass:     ClassOwnerAPI,
			vehicleClass: VehicleClassSignedCmd,
			fleetReady:   false,
			via:          "fleet",
			wantErr:      true,
			wantErrSub:   "Fleet API not configured",
		},
		{
			name:          "--via=fleet with Fleet creds -> fleet",
			cmdClass:      ClassOwnerAPI,
			vehicleClass:  VehicleClassSignedCmd,
			fleetReady:    true,
			hermesRunning: true, // would prefer hermes on auto, but we forced fleet
			via:           "fleet",
			wantPath:      PathFleet,
		},
		{
			name:         "--via=hermes for VCSEC -> error",
			cmdClass:     ClassVCSEC,
			vehicleClass: VehicleClassSignedCmd,
			fleetReady:   true,
			via:          "hermes",
			wantErr:      true,
			wantErrSub:   "Hermes does not support lock/unlock/trunk",
		},
		{
			name:          "--via=hermes without relay -> error",
			cmdClass:      ClassOwnerAPI,
			vehicleClass:  VehicleClassSignedCmd,
			fleetReady:    true,
			hermesRunning: false,
			via:           "hermes",
			wantErr:       true,
			wantErrSub:    "Hermes relay not running",
		},
		{
			name:          "--via=hermes with relay running -> hermes",
			cmdClass:      ClassOwnerAPI,
			vehicleClass:  VehicleClassSignedCmd,
			hermesRunning: true,
			via:           "hermes",
			wantPath:      PathHermes,
		},
		{
			name:         "--via=ble always works (the CLI just prints the recipe)",
			cmdClass:     ClassVCSEC,
			vehicleClass: VehicleClassSignedCmd,
			fleetReady:   false,
			via:          "ble",
			wantPath:     PathBLE,
		},
		{
			name:         "--via=bogus -> error",
			cmdClass:     ClassOwnerAPI,
			vehicleClass: VehicleClassSignedCmd,
			via:          "rocket",
			wantErr:      true,
			wantErrSub:   "must be auto|fleet|hermes|ble",
		},
		{
			name:         "--via=hermes for wake -> error (Hermes wake_up bug)",
			cmdClass:     ClassWake,
			vehicleClass: VehicleClassSignedCmd,
			via:          "hermes",
			wantErr:      true,
			wantErrSub:   "wake_up bug",
		},
		// REST-friendly overrides.
		{
			name:         "rest_friendly + --via=ble -> ble",
			cmdClass:     ClassOwnerAPI,
			vehicleClass: VehicleClassRESTFriendly,
			via:          "ble",
			wantPath:     PathBLE,
		},
		{
			name:         "rest_friendly + --via=hermes without relay -> error",
			cmdClass:     ClassOwnerAPI,
			vehicleClass: VehicleClassRESTFriendly,
			via:          "hermes",
			wantErr:      true,
			wantErrSub:   "Hermes relay not running",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := PickPath(c.cmdClass, c.vehicleClass, c.fleetReady, c.hermesRunning, c.via)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil; choice=%+v", c.wantErrSub, got)
				}
				if c.wantErrSub != "" && !strings.Contains(err.Error(), c.wantErrSub) {
					t.Fatalf("expected error containing %q, got: %v", c.wantErrSub, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Path != c.wantPath {
				t.Errorf("path: got %q want %q (reason=%q)", got.Path, c.wantPath, got.Reason)
			}
			if got.Reason == "" {
				t.Errorf("reason must be non-empty for user-facing surface")
			}
		})
	}
}

func TestSignedPaths_ClassifyCommand(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want CommandClass
	}{
		{"unlock is VCSEC", "unlock", ClassVCSEC},
		{"door_unlock is VCSEC", "door_unlock", ClassVCSEC},
		{"lock is VCSEC", "lock", ClassVCSEC},
		{"actuate_trunk is VCSEC", "actuate_trunk", ClassVCSEC},
		{"sentry_mode is VCSEC", "sentry_mode", ClassVCSEC},
		{"wake_up is Wake", "wake_up", ClassWake},
		{"wake is Wake", "wake", ClassWake},
		{"honk_horn is OwnerAPI", "honk_horn", ClassOwnerAPI},
		{"set_charge_limit is OwnerAPI", "set_charge_limit", ClassOwnerAPI},
		{"climate_on is OwnerAPI", "climate_on", ClassOwnerAPI},
		{"flash_lights is OwnerAPI", "flash_lights", ClassOwnerAPI},
		// Unknown defaults to OwnerAPI.
		{"unknown defaults to OwnerAPI", "do_a_barrel_roll", ClassOwnerAPI},
		// Case + slash normalization.
		{"uppercase normalizes", "UNLOCK", ClassVCSEC},
		{"slash-prefixed normalizes", "/unlock", ClassVCSEC},
		{"whitespace trims", "  unlock  ", ClassVCSEC},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := classifyCommand(c.in)
			if got != c.want {
				t.Errorf("classifyCommand(%q): got %s want %s", c.in, got, c.want)
			}
		})
	}
}

func TestSignedPaths_PickPathReasonNamesPath(t *testing.T) {
	// Belt-and-suspenders: the reason string the user sees on a default-print
	// invocation should at least name the path so the user can tell what was
	// chosen even if --json is off.
	choice, err := PickPath(ClassVCSEC, VehicleClassSignedCmd, true, false, "auto")
	if err != nil {
		t.Fatalf("PickPath: %v", err)
	}
	if !strings.Contains(strings.ToLower(choice.Reason), "fleet") {
		t.Errorf("reason should name Fleet, got %q", choice.Reason)
	}
}
