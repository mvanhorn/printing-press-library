// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/travelclick/internal/types"
)

func TestNovelRatesCompareHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"rates", "compare", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("rates compare --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "compare"} {
		if !strings.Contains(help, want) {
			t.Fatalf("rates compare --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestComputeLowestHotelRate(t *testing.T) {
	stays := []types.AvailRoomStay{
		{
			RoomTypes: []types.RoomType{
				{
					RoomTypeName: "Deluxe King",
					AverageRates: []types.RatePlanRate{
						{
							RatePlanCode: "BAR",
							Rate:         200.0,
						},
						{
							RatePlanCode: "MEM",
							Rate:         180.0,
						},
					},
					NightlyRates: []types.NightlyRate{
						{
							RatePlanCode:                "BAR",
							AmtTotal:                    200.0,
							TotalServiceChargeExclusive: 20.0,
						},
						{
							RatePlanCode:                "MEM",
							AmtTotal:                    180.0,
							TotalServiceChargeExclusive: 15.0,
						},
					},
				},
			},
		},
	}

	best, hasRate := computeLowestHotelRate(stays, "102306", "made-nyc", 2, "")
	if !hasRate {
		t.Fatalf("expected to find a rate")
	}

	// 180 + 15 = 195.0 total cost is cheaper than 200 + 20 = 220.0
	if best.LowestTotal != 195.0 {
		t.Errorf("expected lowest total to be 195.0, got %f", best.LowestTotal)
	}
	if best.RoomTypeName != "Deluxe King" {
		t.Errorf("expected room type Deluxe King, got %s", best.RoomTypeName)
	}
	if best.RatePlanCode != "MEM" {
		t.Errorf("expected rate plan MEM, got %s", best.RatePlanCode)
	}
	if best.Currency != "USD" {
		t.Errorf("expected default currency USD when payload has none, got %s", best.Currency)
	}
}

func TestComputeLowestHotelRateFallback(t *testing.T) {
	// Hotel-wide fallback: no matching nightly rows exist at all, so
	// compare still returns average*nights. This must not run when any
	// fee-inclusive nightly total is available (see mixed-plan test).
	stays := []types.AvailRoomStay{
		{
			RoomTypes: []types.RoomType{
				{
					RoomTypeName: "Suite",
					AverageRates: []types.RatePlanRate{
						{
							RatePlanCode: "BAR",
							Rate:         300.0,
						},
					},
				},
			},
		},
	}

	best, hasRate := computeLowestHotelRate(stays, "102306", "made-nyc", 3, "")
	if !hasRate {
		t.Fatalf("expected to find a rate")
	}

	// 300.0 * 3 nights = 900.0, used only because this hotel has zero nightly totals.
	if best.LowestTotal != 900.0 {
		t.Errorf("expected lowest total to be 900.0, got %f", best.LowestTotal)
	}
	if best.RatePlanCode != "BAR" {
		t.Errorf("expected rate plan BAR, got %s", best.RatePlanCode)
	}
	if best.FeeInclusive {
		t.Errorf("expected fallback-only hotel to be fee-exclusive")
	}
}

func TestComputeLowestHotelRatePrefersFeeInclusiveOverAverageFallback(t *testing.T) {
	// CHEAP has no nightly rows. Its room-only average*nights (100*3=300)
	// would beat BAR's fee-inclusive nightly total (400+50+50=500) if the
	// per-plan fallback were allowed to compete. BAR must win.
	stays := []types.AvailRoomStay{
		{
			RoomTypes: []types.RoomType{
				{
					RoomTypeName: "Deluxe King",
					AverageRates: []types.RatePlanRate{
						{
							RatePlanCode: "BAR",
							Rate:         400.0,
						},
						{
							RatePlanCode: "CHEAP",
							Rate:         100.0,
						},
					},
					NightlyRates: []types.NightlyRate{
						{
							RatePlanCode:                "BAR",
							AmtTotal:                    400.0,
							TotalServiceChargeExclusive: 50.0,
							TotalResortFeeExclusive:     50.0,
						},
					},
				},
			},
		},
	}

	best, hasRate := computeLowestHotelRate(stays, "102306", "made-nyc", 3, "EUR")
	if !hasRate {
		t.Fatalf("expected to find a rate")
	}
	if best.RatePlanCode != "BAR" {
		t.Errorf("expected fee-inclusive plan BAR to win, got %s", best.RatePlanCode)
	}
	if best.LowestTotal != 500.0 {
		t.Errorf("expected fee-inclusive total 500.0, got %f", best.LowestTotal)
	}
	if best.Currency != "EUR" {
		t.Errorf("expected payload currency EUR, got %s", best.Currency)
	}
	if !best.FeeInclusive {
		t.Errorf("expected fee-inclusive result")
	}
}

func feeInclusiveStay(total float64) []types.AvailRoomStay {
	return []types.AvailRoomStay{
		{
			RoomTypes: []types.RoomType{
				{
					RoomTypeName: "Deluxe King",
					AverageRates: []types.RatePlanRate{
						{RatePlanCode: "BAR", Rate: total},
					},
					NightlyRates: []types.NightlyRate{
						{
							RatePlanCode:                "BAR",
							AmtTotal:                    total,
							TotalServiceChargeExclusive: 0,
							TotalResortFeeExclusive:     0,
						},
					},
				},
			},
		},
	}
}

func fallbackOnlyStay(nightly float64) []types.AvailRoomStay {
	return []types.AvailRoomStay{
		{
			RoomTypes: []types.RoomType{
				{
					RoomTypeName: "Suite",
					AverageRates: []types.RatePlanRate{
						{RatePlanCode: "AVG", Rate: nightly},
					},
				},
			},
		},
	}
}

func TestRankHotelCompareResultsExcludesFeeExclusiveFallback(t *testing.T) {
	hotelA, okA := computeLowestHotelRate(feeInclusiveStay(500), "hotel-a", "", 1, "USD")
	if !okA {
		t.Fatal("expected hotel A rate")
	}
	hotelB, okB := computeLowestHotelRate(fallbackOnlyStay(300), "hotel-b", "", 1, "USD")
	if !okB {
		t.Fatal("expected hotel B fallback rate")
	}
	if hotelA.LowestTotal != 500 || !hotelA.FeeInclusive {
		t.Fatalf("hotel A: total=%f inclusive=%v", hotelA.LowestTotal, hotelA.FeeInclusive)
	}
	if hotelB.LowestTotal != 300 || hotelB.FeeInclusive {
		t.Fatalf("hotel B: total=%f inclusive=%v", hotelB.LowestTotal, hotelB.FeeInclusive)
	}

	ranked, incomparable := rankHotelCompareResults([]CompareHotelResult{hotelB, hotelA})
	if len(ranked) != 1 {
		t.Fatalf("expected 1 comparable hotel, got %d: %+v", len(ranked), ranked)
	}
	if ranked[0].HotelID != "hotel-a" {
		t.Fatalf("expected hotel A to win comparable ranking, got %s", ranked[0].HotelID)
	}
	if ranked[0].LowestTotal != 500 {
		t.Fatalf("expected hotel A total 500, got %f", ranked[0].LowestTotal)
	}
	if len(incomparable) != 1 || incomparable[0].HotelID != "hotel-b" {
		t.Fatalf("expected hotel B in incomparable list, got %+v", incomparable)
	}
}

func TestRankHotelCompareResultsFallbackOnlyHotelsRankTogether(t *testing.T) {
	cheap, okCheap := computeLowestHotelRate(fallbackOnlyStay(100), "cheap", "", 3, "EUR")
	if !okCheap {
		t.Fatal("expected cheap fallback rate")
	}
	pricey, okPricey := computeLowestHotelRate(fallbackOnlyStay(180), "pricey", "", 3, "EUR")
	if !okPricey {
		t.Fatal("expected pricey fallback rate")
	}
	if cheap.LowestTotal != 300 || pricey.LowestTotal != 540 {
		t.Fatalf("fallback totals cheap=%f pricey=%f", cheap.LowestTotal, pricey.LowestTotal)
	}

	ranked, incomparable := rankHotelCompareResults([]CompareHotelResult{pricey, cheap})
	if len(incomparable) != 0 {
		t.Fatalf("expected no incomparable hotels when all are fallback-only, got %+v", incomparable)
	}
	if len(ranked) != 2 {
		t.Fatalf("expected both fallback hotels ranked, got %d", len(ranked))
	}
	if ranked[0].HotelID != "cheap" || ranked[0].LowestTotal != 300 {
		t.Fatalf("expected cheap hotel first by average*nights, got %+v", ranked[0])
	}
	if ranked[1].HotelID != "pricey" {
		t.Fatalf("expected pricey hotel second, got %+v", ranked[1])
	}
}
