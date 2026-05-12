// Copyright 2026 kjuju600. Licensed under Apache-2.0. See LICENSE.

// Package risk implements the position-sizing math from Ed Seykota's
// "Risk Management" essay (seykota.com/tribe/risk/) and the Seykota & Druz
// paper "Determining Optimal Risk" (Stocks & Commodities, 1993): the Kelly
// fraction, fixed-fraction "heat" sizing, the Uncle Point, a Monte-Carlo of
// the essay's Coin-Toss model, and the Lake Ratio.
//
// These are operationalizations of formulas the essay describes, not
// trading advice. Where the essay is qualitative (the Lake Ratio's "lake"
// metaphor), the chosen definition is documented at the function.
package risk

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"
)

// Kelly returns the optimal fixed bet fraction K = W − (1 − W) / R for a
// win probability W (0..1) and a win/loss payoff ratio R (>0): the fraction
// of equity to risk per trade that maximizes long-run growth.
func Kelly(winRate, payoff float64) (float64, error) {
	if winRate < 0 || winRate > 1 {
		return 0, fmt.Errorf("win-rate must be between 0 and 1 (got %g)", winRate)
	}
	if payoff <= 0 {
		return 0, fmt.Errorf("payoff must be greater than 0 (got %g)", payoff)
	}
	return winRate - (1-winRate)/payoff, nil
}

// EdgePerBet returns the expected return per unit bet: W·R − (1 − W).
func EdgePerBet(winRate, payoff float64) float64 {
	return winRate*payoff - (1 - winRate)
}

// KellyResult bundles the Kelly output for display.
type KellyResult struct {
	WinRate       float64 `json:"win_rate"`
	Payoff        float64 `json:"payoff"`
	KellyFraction float64 `json:"kelly_fraction"`
	KellyPct      float64 `json:"kelly_pct"`
	HalfKellyPct  float64 `json:"half_kelly_pct"`
	EdgePerBet    float64 `json:"edge_per_bet"`
	HasEdge       bool    `json:"has_edge"`
}

// KellyReport computes a KellyResult.
func KellyReport(winRate, payoff float64) (KellyResult, error) {
	k, err := Kelly(winRate, payoff)
	if err != nil {
		return KellyResult{}, err
	}
	edge := EdgePerBet(winRate, payoff)
	return KellyResult{
		WinRate: winRate, Payoff: payoff,
		KellyFraction: k, KellyPct: k * 100, HalfKellyPct: k * 50,
		EdgePerBet: edge, HasEdge: k > 0 && edge > 0,
	}, nil
}

// HeatResult is the per-trade fixed-fraction sizing output.
type HeatResult struct {
	Equity         float64 `json:"equity"`
	RiskPct        float64 `json:"risk_pct"`
	Entry          float64 `json:"entry"`
	Stop           float64 `json:"stop"`
	RiskPerShare   float64 `json:"risk_per_share"`
	TargetDollars  float64 `json:"target_dollars_at_risk"`
	Shares         int64   `json:"shares"`
	DollarsAtRisk  float64 `json:"dollars_at_risk"`
	HeatPct        float64 `json:"heat_pct"`
	PositionValue  float64 `json:"position_value"`
	PositionPctEq  float64 `json:"position_pct_of_equity"`
}

// HeatPerTrade sizes one position to risk riskPct of equity between entry
// and stop. Shares are floored to a whole number, so the realized "heat"
// (dollars at risk ÷ equity) is at or just below the target.
func HeatPerTrade(equity, riskPct, entry, stop float64) (HeatResult, error) {
	if equity <= 0 {
		return HeatResult{}, fmt.Errorf("equity must be greater than 0")
	}
	if riskPct <= 0 {
		return HeatResult{}, fmt.Errorf("risk-pct must be greater than 0")
	}
	rps := math.Abs(entry - stop)
	if rps == 0 {
		return HeatResult{}, fmt.Errorf("entry and stop must differ (risk per share is zero)")
	}
	target := equity * riskPct / 100
	shares := int64(math.Floor(target / rps))
	if shares < 0 {
		shares = 0
	}
	dollarsAtRisk := float64(shares) * rps
	posVal := float64(shares) * entry
	return HeatResult{
		Equity: equity, RiskPct: riskPct, Entry: entry, Stop: stop,
		RiskPerShare: rps, TargetDollars: target, Shares: shares,
		DollarsAtRisk: dollarsAtRisk, HeatPct: dollarsAtRisk / equity * 100,
		PositionValue: posVal, PositionPctEq: posVal / equity * 100,
	}, nil
}

// Position is one open position for portfolio-heat aggregation.
type Position struct {
	Name    string  `json:"name"`
	Entry   float64 `json:"entry"`
	Stop    float64 `json:"stop"`
	RiskPct float64 `json:"risk_pct"`
}

// PortfolioHeatResult bundles per-position heat plus the total.
type PortfolioHeatResult struct {
	Equity       float64      `json:"equity"`
	Positions    []HeatResult `json:"positions"`
	Names        []string     `json:"names"`
	TotalHeatPct float64      `json:"total_heat_pct"`
}

// PortfolioHeat sizes each position to its own riskPct and sums the heat —
// Seykota's "total heat": a five-position book risking 2% each carries 10%
// total heat. The essay's simulations put the optimal *total* heat near
// 140%; well past that, losses dominate.
func PortfolioHeat(equity float64, positions []Position) (PortfolioHeatResult, error) {
	if equity <= 0 {
		return PortfolioHeatResult{}, fmt.Errorf("equity must be greater than 0")
	}
	if len(positions) == 0 {
		return PortfolioHeatResult{}, fmt.Errorf("no positions provided")
	}
	out := PortfolioHeatResult{Equity: equity}
	for _, p := range positions {
		hr, err := HeatPerTrade(equity, p.RiskPct, p.Entry, p.Stop)
		if err != nil {
			return PortfolioHeatResult{}, fmt.Errorf("position %q: %w", p.Name, err)
		}
		out.Positions = append(out.Positions, hr)
		out.Names = append(out.Names, p.Name)
		out.TotalHeatPct += hr.HeatPct
	}
	return out, nil
}

// ParsePositions parses a "name:entry:stop:riskPct" comma list.
func ParsePositions(spec string) ([]Position, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, fmt.Errorf("empty positions spec")
	}
	var out []Position
	for i, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		f := strings.Split(part, ":")
		if len(f) != 4 {
			return nil, fmt.Errorf("position %d (%q): expected name:entry:stop:riskPct", i+1, part)
		}
		entry, err := strconv.ParseFloat(strings.TrimSpace(f[1]), 64)
		if err != nil {
			return nil, fmt.Errorf("position %q: bad entry %q", f[0], f[1])
		}
		stop, err := strconv.ParseFloat(strings.TrimSpace(f[2]), 64)
		if err != nil {
			return nil, fmt.Errorf("position %q: bad stop %q", f[0], f[2])
		}
		rp, err := strconv.ParseFloat(strings.TrimSpace(f[3]), 64)
		if err != nil {
			return nil, fmt.Errorf("position %q: bad risk-pct %q", f[0], f[3])
		}
		out = append(out, Position{Name: strings.TrimSpace(f[0]), Entry: entry, Stop: stop, RiskPct: rp})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no positions parsed from %q", spec)
	}
	return out, nil
}

// UnclePointResult is the Uncle-Point output.
type UnclePointResult struct {
	Equity         float64 `json:"equity"`
	MaxDrawdownPct float64 `json:"max_drawdown_pct"`
	UnclePoint     float64 `json:"uncle_point"`
	RoomToLose     float64 `json:"room_to_lose"`
}

// UnclePoint returns the equity level at which a trader "cries uncle" and
// quits — the floor a fixed-fraction system must stay above. It is simply
// equity · (1 − maxDrawdownPct/100); the value of naming it is that the
// rest of the math (bet fraction, heat) is what keeps you off it.
func UnclePoint(equity, maxDrawdownPct float64) (UnclePointResult, error) {
	if equity <= 0 {
		return UnclePointResult{}, fmt.Errorf("equity must be greater than 0")
	}
	if maxDrawdownPct <= 0 || maxDrawdownPct >= 100 {
		return UnclePointResult{}, fmt.Errorf("max-drawdown-pct must be between 0 and 100 (exclusive)")
	}
	up := equity * (1 - maxDrawdownPct/100)
	return UnclePointResult{Equity: equity, MaxDrawdownPct: maxDrawdownPct, UnclePoint: up, RoomToLose: equity - up}, nil
}

// CoinTossOpts parameterizes the Monte-Carlo of the essay's Coin-Toss /
// fixed-fraction model.
type CoinTossOpts struct {
	WinRate     float64 // P(win), 0..1
	Payoff      float64 // win:loss ratio, >0
	BetFraction float64 // fixed fraction of equity risked per bet, 0..1
	Trials      int     // bets per run
	Runs        int     // independent runs
	Seed        int64   // RNG seed (0 -> 1)
	RuinFrac    float64 // a run is "ruined" if equity ever falls to this fraction of start (default 0.10)
}

// CoinTossResult is the simulation summary (all equities are multiples of
// the starting stake, which is 1.0).
type CoinTossResult struct {
	WinRate          float64 `json:"win_rate"`
	Payoff           float64 `json:"payoff"`
	BetFraction      float64 `json:"bet_fraction"`
	Trials           int     `json:"trials"`
	Runs             int     `json:"runs"`
	Seed             int64   `json:"seed"`
	RuinFraction     float64 `json:"ruin_fraction"`
	MedianTerminal   float64 `json:"median_terminal_equity"`
	MeanTerminal     float64 `json:"mean_terminal_equity"`
	P10Terminal      float64 `json:"p10_terminal_equity"`
	P90Terminal      float64 `json:"p90_terminal_equity"`
	WorstTerminal    float64 `json:"worst_terminal_equity"`
	BestTerminal     float64 `json:"best_terminal_equity"`
	RuinProbability  float64 `json:"ruin_probability"`
	MeanMaxDrawdown  float64 `json:"mean_max_drawdown_pct"`
	OptimalFraction  float64 `json:"optimal_fraction_kelly"`
	BetVsOptimal     string  `json:"bet_vs_optimal"`
}

// CoinToss runs the simulation.
func CoinToss(o CoinTossOpts) (CoinTossResult, error) {
	if o.WinRate < 0 || o.WinRate > 1 {
		return CoinTossResult{}, fmt.Errorf("win-rate must be between 0 and 1")
	}
	if o.Payoff <= 0 {
		return CoinTossResult{}, fmt.Errorf("payoff must be greater than 0")
	}
	if o.BetFraction <= 0 || o.BetFraction >= 1 {
		return CoinTossResult{}, fmt.Errorf("bet-fraction must be between 0 and 1 (exclusive)")
	}
	if o.Trials <= 0 {
		o.Trials = 100
	}
	if o.Runs <= 0 {
		o.Runs = 10000
	}
	if o.Runs > 5_000_000 {
		return CoinTossResult{}, fmt.Errorf("runs too large (max 5,000,000)")
	}
	if o.RuinFrac <= 0 {
		o.RuinFrac = 0.10
	}
	seed := o.Seed
	if seed == 0 {
		seed = 1
	}
	rng := rand.New(rand.NewSource(seed))

	terminals := make([]float64, o.Runs)
	var sumTerminal, sumMaxDD float64
	ruined := 0
	for r := 0; r < o.Runs; r++ {
		eq := 1.0
		peak := 1.0
		maxDD := 0.0
		isRuined := false
		for t := 0; t < o.Trials; t++ {
			if rng.Float64() < o.WinRate {
				eq *= 1 + o.BetFraction*o.Payoff
			} else {
				eq *= 1 - o.BetFraction
			}
			if eq > peak {
				peak = eq
			}
			dd := (peak - eq) / peak
			if dd > maxDD {
				maxDD = dd
			}
			if eq <= o.RuinFrac {
				isRuined = true
			}
		}
		terminals[r] = eq
		sumTerminal += eq
		sumMaxDD += maxDD
		if isRuined {
			ruined++
		}
	}
	sort.Float64s(terminals)
	kelly, _ := Kelly(o.WinRate, o.Payoff)
	res := CoinTossResult{
		WinRate: o.WinRate, Payoff: o.Payoff, BetFraction: o.BetFraction,
		Trials: o.Trials, Runs: o.Runs, Seed: seed, RuinFraction: o.RuinFrac,
		MedianTerminal:  percentile(terminals, 50),
		MeanTerminal:    sumTerminal / float64(o.Runs),
		P10Terminal:     percentile(terminals, 10),
		P90Terminal:     percentile(terminals, 90),
		WorstTerminal:   terminals[0],
		BestTerminal:    terminals[len(terminals)-1],
		RuinProbability: float64(ruined) / float64(o.Runs),
		MeanMaxDrawdown: sumMaxDD / float64(o.Runs) * 100,
		OptimalFraction: kelly,
	}
	switch {
	case kelly <= 0:
		res.BetVsOptimal = "no positive edge — optimal fraction is 0; any bet is overbetting"
	case o.BetFraction < kelly*0.95:
		res.BetVsOptimal = fmt.Sprintf("under-betting (%.1f%% vs optimal %.1f%%) — timid-trader territory: safer, slower growth", o.BetFraction*100, kelly*100)
	case o.BetFraction > kelly*1.05:
		res.BetVsOptimal = fmt.Sprintf("over-betting (%.1f%% vs optimal %.1f%%) — bold-trader territory: faster growth, sharply rising ruin risk", o.BetFraction*100, kelly*100)
	default:
		res.BetVsOptimal = fmt.Sprintf("near optimal (%.1f%% ≈ %.1f%%)", o.BetFraction*100, kelly*100)
	}
	return res, nil
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	idx := p / 100 * float64(len(sorted)-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return sorted[lo]
	}
	frac := idx - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}

// LakeRatioResult is the Lake-Ratio output.
type LakeRatioResult struct {
	Points    int     `json:"points"`
	LakeArea  float64 `json:"lake_area"`
	EarthArea float64 `json:"earth_area"`
	LakeRatio float64 `json:"lake_ratio"`
	MaxDDpct  float64 `json:"max_drawdown_pct"`
	StartVal  float64 `json:"start_value"`
	EndVal    float64 `json:"end_value"`
}

// LakeRatio computes Seykota's Lake Ratio over an equity curve. The essay
// describes it as the ratio of the area of "water" (the gaps between the
// running peak and the curve, where drawdowns pool) to the area of "earth"
// (the area under the curve). Here: lakeArea = Σ(peak_i − value_i),
// earthArea = Σ value_i, ratio = lakeArea / earthArea — 0 for a curve that
// never drops, larger the more time it spends underwater. Requires at least
// two strictly-positive points.
func LakeRatio(curve []float64) (LakeRatioResult, error) {
	if len(curve) < 2 {
		return LakeRatioResult{}, fmt.Errorf("need at least 2 equity points (got %d)", len(curve))
	}
	for i, v := range curve {
		if v <= 0 {
			return LakeRatioResult{}, fmt.Errorf("equity point %d is not positive (%g)", i+1, v)
		}
	}
	peak := curve[0]
	var lake, earth, maxDD float64
	for _, v := range curve {
		if v > peak {
			peak = v
		}
		gap := peak - v
		lake += gap
		earth += v
		if dd := gap / peak; dd > maxDD {
			maxDD = dd
		}
	}
	ratio := 0.0
	if earth > 0 {
		ratio = lake / earth
	}
	return LakeRatioResult{
		Points: len(curve), LakeArea: lake, EarthArea: earth, LakeRatio: ratio,
		MaxDDpct: maxDD * 100, StartVal: curve[0], EndVal: curve[len(curve)-1],
	}, nil
}

// ParseEquityCurve reads numeric equity values from raw text: one number
// per line, or comma/whitespace separated, ignoring blank lines, comment
// lines (#), and (optionally) a non-numeric header line. If a line has
// multiple comma-separated columns, the last numeric column is taken (so
// "date,equity" CSVs work).
func ParseEquityCurve(raw string) ([]float64, error) {
	var out []float64
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var fields []string
		if strings.Contains(line, ",") {
			fields = strings.Split(line, ",")
		} else {
			fields = strings.Fields(line)
		}
		var got bool
		for i := len(fields) - 1; i >= 0; i-- {
			f := strings.TrimSpace(strings.Trim(fields[i], `"'`))
			f = strings.ReplaceAll(f, "$", "")
			f = strings.ReplaceAll(f, "_", "")
			f = strings.ReplaceAll(f, ",", "")
			if v, err := strconv.ParseFloat(f, 64); err == nil {
				out = append(out, v)
				got = true
				break
			}
		}
		_ = got
	}
	if len(out) < 2 {
		return nil, fmt.Errorf("parsed %d numeric points; need at least 2 (expected one number per line, or a CSV with an equity column)", len(out))
	}
	return out, nil
}
