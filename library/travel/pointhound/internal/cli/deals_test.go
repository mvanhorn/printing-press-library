package cli

import "testing"

func TestComputeProgramCostsUsesTotalRatioAndSortsByEffectiveCost(t *testing.T) {
	costs := computeProgramCosts(35000, []offerTransferOption{
		{
			TotalTransferRatio: "1",
			TransferTime:       "up_to_24",
			EarnProgram:        offerEarnProgram{ID: "pep_amex", Name: "Amex Membership Rewards", Identifier: "amex-membership-rewards"},
		},
		{
			TotalTransferRatio: "1.25",
			TransferTime:       "instant",
			BonusTransferRatio: "0.25",
			EarnProgram:        offerEarnProgram{ID: "pep_rove", Name: "Rove Miles", Identifier: "rove-miles"},
		},
		{
			TransferRatio: "0.333",
			TransferTime:  "up_to_48",
			EarnProgram:   offerEarnProgram{ID: "pep_marriott", Name: "Marriott Bonvoy", Identifier: "marriott-bonvoy"},
		},
		{
			TotalTransferRatio: "1",
			TransferTime:       "instant",
			EarnProgram:        offerEarnProgram{ID: "pep_chase", Name: "Chase Ultimate Rewards", Identifier: "chase-ultimate-rewards"},
		},
		{
			TotalTransferRatio: "0",
			EarnProgram:        offerEarnProgram{ID: "pep_invalid", Name: "Invalid"},
		},
	})

	if len(costs) != 4 {
		t.Fatalf("expected 4 program costs, got %d", len(costs))
	}
	wantIDs := []string{"pep_rove", "pep_chase", "pep_amex", "pep_marriott"}
	wantPoints := []int{28000, 35000, 35000, 105106}
	for i := range wantIDs {
		if costs[i].ProgramID != wantIDs[i] {
			t.Errorf("costs[%d].ProgramID: want %s, got %s", i, wantIDs[i], costs[i].ProgramID)
		}
		if costs[i].EffectivePoints != wantPoints[i] {
			t.Errorf("costs[%d].EffectivePoints: want %d, got %d", i, wantPoints[i], costs[i].EffectivePoints)
		}
	}
	if !costs[0].HasBonus {
		t.Errorf("expected first cost to be marked as bonus")
	}
}

func TestFormatProgramCostsAbbreviatesAndLimits(t *testing.T) {
	costs := []ProgramCost{
		{ProgramName: "Rove Miles", EffectivePoints: 37440, TransferTime: "instant", HasBonus: true},
		{ProgramName: "Chase Ultimate Rewards", EffectivePoints: 46800, TransferTime: "instant"},
		{ProgramName: "Amex Membership Rewards", EffectivePoints: 46800, TransferTime: "instant"},
		{ProgramName: "Capital One Rewards", EffectivePoints: 46800, TransferTime: "instant"},
		{ProgramName: "Bilt Rewards", EffectivePoints: 46800, TransferTime: "instant"},
		{ProgramName: "Marriott Bonvoy", EffectivePoints: 140541, TransferTime: "up_to_24"},
		{ProgramName: "World of Hyatt Loyalty Program", EffectivePoints: 200000, TransferTime: "up_to_48"},
	}

	got := formatProgramCosts(costs)
	want := "Rove 37,440*  ·  Chase UR 46,800  ·  Amex MR 46,800  ·  Capital One 46,800  ·  Bilt 46,800  ·  Marriott Bonvoy 140,541 (up_to_24)  +1 more"
	if got != want {
		t.Errorf("formatted costs:\nwant %q\n got %q", want, got)
	}
}

func TestShortenProgramNameFallsBackToSuffixTrim(t *testing.T) {
	if got := shortenProgramName("Example Rewards"); got != "Example" {
		t.Errorf("suffix trim: want Example, got %s", got)
	}
	if got := shortenProgramName("British Airways Avios"); got != "BA Avios" {
		t.Errorf("fixed abbreviation: want BA Avios, got %s", got)
	}
}
