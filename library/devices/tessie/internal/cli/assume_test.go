// Copyright 2026 Keith Herrington and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/devices/tessie/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/tessie/internal/cliutil/testenv"
)

func TestMatchVehicles_ExactVsSubstring(t *testing.T) {
	rows := []vehicleRow{
		{DisplayName: "Car", VIN: "VCK5YJ3E1EA000001"},
		{DisplayName: "Carpenter", VIN: "VCK5YJ3E1EA000003"},
	}
	// Exact/preferred match must not widen to a substring-named vehicle.
	if got := matchVehicles(rows, "Car"); len(got) != 1 || got[0].DisplayName != "Car" {
		t.Fatalf("exact 'Car' = %+v", got)
	}
	if got := matchVehicles(rows, "car"); len(got) != 1 || got[0].DisplayName != "Car" {
		t.Fatalf("exact-preferred lowercase 'car' = %+v", got)
	}
	// VIN suffix matching.
	if got := matchVehicles(rows, "000001"); len(got) != 1 || got[0].VIN != "VCK5YJ3E1EA000001" {
		t.Fatalf("vin-suffix = %+v", got)
	}
	// Substring fallback only applies when no exact match.
	if got := matchVehicles(rows, "carpen"); len(got) != 1 || got[0].DisplayName != "Carpenter" {
		t.Fatalf("substring 'carpen' = %+v", got)
	}
	// No match.
	if got := matchVehicles(rows, "nonexistent"); len(got) != 0 {
		t.Fatalf("nonexistent = %+v", got)
	}
}

func TestMaskVIN(t *testing.T) {
	if got := maskVIN("VCK5YJ3E1EA000001"); !strings.HasSuffix(got, "0001") {
		t.Fatalf("maskVIN = %q", got)
	}
	if strings.Contains(maskVIN("VCK5YJ3E1EA000001"), "VCK") {
		t.Fatal("maskVIN leaked VIN prefix")
	}
	if got := maskVIN("ABCDE"); got != "***" {
		t.Fatalf("short vin mask = %q, want ***", got)
	}
}

func TestIsFullVIN(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"VCK5YJ3E1EA000001", true},
		{"7SAYGDED3TF664951", true},
		{"Car", false},
		{"5YJ3E1EA0", false},          // 10 chars
		{"VCK5YJ3E1EA0000012", false}, // 18 chars
		{"VCK-5YJ3E1EA000001", false}, // contains dash
		{"VCK5YJ3E1EA00000I", false},  // contains I
	}
	for _, c := range cases {
		if got := isFullVIN(c.in); got != c.want {
			t.Fatalf("isFullVIN(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestAssumePathFollowsResolvedConfigDir(t *testing.T) {
	restore, err := cliutil.SetHomeOverride("")
	if err != nil {
		t.Fatalf("reset home override: %v", err)
	}
	t.Cleanup(restore)
	home := testenv.Isolate(t, cliutil.ConfigDir)

	t.Run("explicit --config", func(t *testing.T) {
		cfg := filepath.Join(t.TempDir(), "custom", "config.toml")
		flags := &rootFlags{configPath: cfg}
		want := filepath.Join(filepath.Dir(cfg), assumeFileName)
		if got := flags.assumePath(); got != want {
			t.Fatalf("assumePath() = %q, want %q", got, want)
		}
	})

	t.Run("TESSIE_CONFIG", func(t *testing.T) {
		cfg := filepath.Join(t.TempDir(), "env-config", "config.toml")
		t.Setenv("TESSIE_CONFIG", cfg)
		flags := &rootFlags{}
		want := filepath.Join(filepath.Dir(cfg), assumeFileName)
		if got := flags.assumePath(); got != want {
			t.Fatalf("assumePath() = %q, want %q", got, want)
		}
	})

	t.Run("TESSIE_HOME", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "relocated-home")
		t.Setenv("TESSIE_HOME", root)
		flags := &rootFlags{}
		want := filepath.Join(root, "config", assumeFileName)
		if got := flags.assumePath(); got != want {
			t.Fatalf("assumePath() = %q, want %q", got, want)
		}
	})

	t.Run("--home", func(t *testing.T) {
		root := t.TempDir()
		restoreHome, err := cliutil.SetHomeOverride(root)
		if err != nil {
			t.Fatalf("SetHomeOverride(%q): %v", root, err)
		}
		t.Cleanup(restoreHome)
		flags := &rootFlags{homePath: root}
		want := filepath.Join(root, "config", assumeFileName)
		if got := flags.assumePath(); got != want {
			t.Fatalf("assumePath() = %q, want %q", got, want)
		}
	})

	t.Run("platform default", func(t *testing.T) {
		flags := &rootFlags{}
		want := filepath.Join(home, ".config", "tessie-pp-cli", assumeFileName)
		if got := flags.assumePath(); got != want {
			t.Fatalf("assumePath() = %q, want %q", got, want)
		}
	})
}

func TestSaveAssumedUsesUniqueTempFile(t *testing.T) {
	restore, err := cliutil.SetHomeOverride("")
	if err != nil {
		t.Fatalf("reset home override: %v", err)
	}
	t.Cleanup(restore)
	_ = testenv.Isolate(t, cliutil.ConfigDir)

	dir := t.TempDir()
	flags := &rootFlags{configPath: filepath.Join(dir, "config.toml")}
	store := assumeStore{AssumedVIN: "VCK5YJ3E1EA000001", AssumedName: "Car"}
	if err := flags.saveAssumed(store); err != nil {
		t.Fatalf("saveAssumed() error = %v", err)
	}

	sharedTmp := filepath.Join(dir, assumeFileName+".tmp")
	if _, err := os.Stat(sharedTmp); !os.IsNotExist(err) {
		t.Fatalf("shared %s leftover: %v", sharedTmp, err)
	}
	matches, err := filepath.Glob(filepath.Join(dir, "."+assumeFileName+".*.tmp"))
	if err != nil {
		t.Fatalf("glob leftover temps: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("leftover unique temps = %v", matches)
	}

	got, err := flags.loadAssumed()
	if err != nil {
		t.Fatalf("loadAssumed() error = %v", err)
	}
	if got != store {
		t.Fatalf("loadAssumed() = %+v, want %+v", got, store)
	}

	raw, err := os.ReadFile(flags.assumePath())
	if err != nil {
		t.Fatalf("read assumed.json: %v", err)
	}
	var decoded assumeStore
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode assumed.json: %v", err)
	}
	if decoded != store {
		t.Fatalf("assumed.json = %+v, want %+v", decoded, store)
	}
}

func TestDisplayNameOrFallback(t *testing.T) {
	if got := displayNameOr(vehicleRow{DisplayName: "Car", VIN: "X"}); got != "Car" {
		t.Fatalf("display = %q", got)
	}
	if got := displayNameOr(vehicleRow{Plate: "AB12", VIN: "X"}); got != "AB12" {
		t.Fatalf("plate = %q", got)
	}
	if got := displayNameOr(vehicleRow{VIN: "VCK5YJ3E1EA000001"}); !strings.HasSuffix(got, "0001") {
		t.Fatalf("vin fallback = %q", got)
	}
}
