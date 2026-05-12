// Copyright 2026 kjuju600. Licensed under Apache-2.0. See LICENSE.

package risk

import (
	"math"
	"testing"
)

func almost(a, b, eps float64) bool { return math.Abs(a-b) <= eps }

func TestKelly(t *testing.T) {
	cases := []struct {
		w, r, want float64
		wantErr    bool
	}{
		{0.5, 2, 0.25, false},   // fair coin, 2:1 payoff -> 0.25 (the essay's worked example)
		{0.4, 3, 0.2, false},    // 0.4 - 0.6/3 = 0.2
		{0.6, 1, 0.2, false},    // 0.6 - 0.4 = 0.2
		{0.5, 1, 0.0, false},    // even-money fair coin: no edge
		{0.3, 2, -0.05, false},  // negative edge -> negative Kelly
		{-0.1, 2, 0, true},      // bad win rate
		{0.5, 0, 0, true},       // bad payoff
		{1.5, 2, 0, true},       // bad win rate
	}
	for _, c := range cases {
		got, err := Kelly(c.w, c.r)
		if c.wantErr {
			if err == nil {
				t.Errorf("Kelly(%g,%g): want error, got %g", c.w, c.r, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("Kelly(%g,%g): unexpected error %v", c.w, c.r, err)
			continue
		}
		if !almost(got, c.want, 1e-9) {
			t.Errorf("Kelly(%g,%g) = %g; want %g", c.w, c.r, got, c.want)
		}
	}
}

func TestKellyReport(t *testing.T) {
	r, err := KellyReport(0.5, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !almost(r.KellyFraction, 0.25, 1e-9) || !almost(r.KellyPct, 25, 1e-9) || !almost(r.HalfKellyPct, 12.5, 1e-9) {
		t.Errorf("KellyReport(0.5,2) = %+v", r)
	}
	if !r.HasEdge {
		t.Errorf("expected positive edge")
	}
	r2, _ := KellyReport(0.3, 2)
	if r2.HasEdge {
		t.Errorf("expected no edge for W=0.3 R=2 (Kelly=%g)", r2.KellyFraction)
	}
}

func TestHeatPerTrade(t *testing.T) {
	hr, err := HeatPerTrade(100000, 1, 50, 45)
	if err != nil {
		t.Fatal(err)
	}
	if hr.RiskPerShare != 5 {
		t.Errorf("risk per share = %g; want 5", hr.RiskPerShare)
	}
	if hr.Shares != 200 {
		t.Errorf("shares = %d; want 200", hr.Shares)
	}
	if !almost(hr.DollarsAtRisk, 1000, 1e-9) || !almost(hr.HeatPct, 1, 1e-9) {
		t.Errorf("dollars at risk = %g, heat = %g; want 1000, 1", hr.DollarsAtRisk, hr.HeatPct)
	}
	if !almost(hr.PositionValue, 10000, 1e-9) {
		t.Errorf("position value = %g; want 10000", hr.PositionValue)
	}
	// flooring: 100000 * 0.5% = 500 target, risk/share = 3 -> 166 shares (not 166.67)
	hr2, _ := HeatPerTrade(100000, 0.5, 100, 97)
	if hr2.Shares != 166 {
		t.Errorf("shares = %d; want 166 (floored)", hr2.Shares)
	}
	if _, err := HeatPerTrade(100000, 1, 50, 50); err == nil {
		t.Errorf("expected error when entry == stop")
	}
	if _, err := HeatPerTrade(0, 1, 50, 45); err == nil {
		t.Errorf("expected error for zero equity")
	}
}

func TestPortfolioHeat(t *testing.T) {
	ps, err := ParsePositions("AAPL:200:185:1,MSFT:400:370:0.5,GLD:185:178:0.75")
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 3 {
		t.Fatalf("parsed %d positions; want 3", len(ps))
	}
	res, err := PortfolioHeat(100000, ps)
	if err != nil {
		t.Fatal(err)
	}
	// each position floors slightly below its target; total should be a bit
	// under 1 + 0.5 + 0.75 = 2.25% and above ~2.2%.
	if res.TotalHeatPct > 2.25 || res.TotalHeatPct < 2.0 {
		t.Errorf("total heat = %g; want roughly 2.2-2.25", res.TotalHeatPct)
	}
	if len(res.Positions) != 3 || len(res.Names) != 3 {
		t.Errorf("expected 3 per-position rows")
	}
	if _, err := ParsePositions("BAD:1:2"); err == nil {
		t.Errorf("expected parse error for too few fields")
	}
}

func TestUnclePoint(t *testing.T) {
	r, err := UnclePoint(100000, 30)
	if err != nil {
		t.Fatal(err)
	}
	if !almost(r.UnclePoint, 70000, 1e-6) || !almost(r.RoomToLose, 30000, 1e-6) {
		t.Errorf("UnclePoint(100000,30) = %+v", r)
	}
	if _, err := UnclePoint(100000, 0); err == nil {
		t.Errorf("expected error for 0 drawdown")
	}
	if _, err := UnclePoint(100000, 100); err == nil {
		t.Errorf("expected error for 100 drawdown")
	}
}

func TestCoinTossDeterministic(t *testing.T) {
	o := CoinTossOpts{WinRate: 0.5, Payoff: 2, BetFraction: 0.25, Trials: 50, Runs: 3000, Seed: 42}
	a, err := CoinToss(o)
	if err != nil {
		t.Fatal(err)
	}
	b, err := CoinToss(o)
	if err != nil {
		t.Fatal(err)
	}
	if a.MedianTerminal != b.MedianTerminal || a.RuinProbability != b.RuinProbability {
		t.Errorf("CoinToss not deterministic for the same seed: %v vs %v", a, b)
	}
	if a.OptimalFraction != 0.25 {
		t.Errorf("optimal fraction = %g; want 0.25", a.OptimalFraction)
	}
	if a.RuinProbability < 0 || a.RuinProbability > 1 {
		t.Errorf("ruin probability out of range: %g", a.RuinProbability)
	}
	if a.MeanMaxDrawdown < 0 || a.MeanMaxDrawdown > 100 {
		t.Errorf("mean max drawdown out of range: %g", a.MeanMaxDrawdown)
	}
	// a different seed should (almost surely) shift the sampled mean — the
	// median of terminal equity is a function of the median win-count and so
	// is seed-stable, but the mean depends on the tails.
	c, _ := CoinToss(CoinTossOpts{WinRate: 0.5, Payoff: 2, BetFraction: 0.25, Trials: 50, Runs: 3000, Seed: 99})
	if c.MeanTerminal == a.MeanTerminal {
		t.Errorf("different seeds gave an identical sampled mean (suspicious)")
	}
	if _, err := CoinToss(CoinTossOpts{WinRate: 0.5, Payoff: 2, BetFraction: 1.5, Trials: 10, Runs: 10}); err == nil {
		t.Errorf("expected error for bet fraction >= 1")
	}
}

func TestLakeRatio(t *testing.T) {
	// monotonically increasing -> lake ratio 0, max DD 0
	r, err := LakeRatio([]float64{100, 110, 120, 130})
	if err != nil {
		t.Fatal(err)
	}
	if r.LakeRatio != 0 || r.MaxDDpct != 0 {
		t.Errorf("monotone curve: lake ratio = %g, maxDD = %g; want 0,0", r.LakeRatio, r.MaxDDpct)
	}
	// dip then recover: peaks 100,105,105,110,110,120; values 100,105,98,110,102,120
	// lake = (105-98) + (110-102) = 7 + 8 = 15 ; earth = 100+105+98+110+102+120 = 635
	r2, _ := LakeRatio([]float64{100, 105, 98, 110, 102, 120})
	if !almost(r2.LakeArea, 15, 1e-9) || !almost(r2.EarthArea, 635, 1e-9) {
		t.Errorf("lake/earth = %g/%g; want 15/635", r2.LakeArea, r2.EarthArea)
	}
	if !almost(r2.LakeRatio, 15.0/635.0, 1e-12) {
		t.Errorf("lake ratio = %g; want %g", r2.LakeRatio, 15.0/635.0)
	}
	if _, err := LakeRatio([]float64{100}); err == nil {
		t.Errorf("expected error for <2 points")
	}
	if _, err := LakeRatio([]float64{100, -5}); err == nil {
		t.Errorf("expected error for non-positive point")
	}
}

func TestParseEquityCurve(t *testing.T) {
	got, err := ParseEquityCurve("# header\n100\n105\n\n98\n110\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 || got[0] != 100 || got[3] != 110 {
		t.Errorf("got %v", got)
	}
	// CSV with a date column -> last numeric column is taken
	got2, err := ParseEquityCurve("date,equity\n2024-01-01,1000\n2024-01-02,1010.5\n2024-01-03,995\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(got2) != 3 || got2[1] != 1010.5 {
		t.Errorf("got %v", got2)
	}
	if _, err := ParseEquityCurve("only one number: 42\n"); err == nil {
		t.Errorf("expected error for <2 parseable points")
	}
}
