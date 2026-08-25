// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/travelclick/internal/types"
)

func TestNovelRatesCheapestNightHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"rates", "cheapest-night", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("rates cheapest-night --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "cheapest-night"} {
		if !strings.Contains(help, want) {
			t.Fatalf("rates cheapest-night --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestComputeCheapestHotelNight(t *testing.T) {
	dates := []types.MultiRoomDate{
		{
			Date:        "2026-09-15",
			IsAvailable: true,
			Rate: types.Rate{
				MinRate: 150.0,
			},
		},
		{
			Date:        "2026-09-16",
			IsAvailable: true,
			Rate: types.Rate{
				MinRate: 140.0,
			},
		},
		{
			Date:        "2026-09-17",
			IsAvailable: false,
			Rate: types.Rate{
				MinRate: 110.0, // unavailable, should be ignored
			},
		},
	}

	best, hasRate := computeCheapestHotelNight(dates, "102306", "made-nyc")
	if !hasRate {
		t.Fatalf("expected to find a rate")
	}

	if best.Rate != 140.0 {
		t.Errorf("expected cheapest rate to be 140.0, got %f", best.Rate)
	}
	if best.Date != "2026-09-16" {
		t.Errorf("expected cheapest date to be 2026-09-16, got %s", best.Date)
	}
}
