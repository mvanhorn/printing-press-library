// Copyright 2026 kjuju600. Licensed under Apache-2.0. See LICENSE.

package risk

import (
	"sort"
	"strings"
)

// Metric describes one named risk concept from Ed Seykota's essay: which
// section of the essay defines it, the formula (where there is one), the
// `seykota risk ...` subcommand that computes it, and a one-line gloss.
type Metric struct {
	Key          string   `json:"key"`            // canonical key, e.g. "heat"
	Name         string   `json:"name"`           // display name
	Aliases      []string `json:"aliases,omitempty"`
	EssaySection string   `json:"essay_section"`  // heading text in /tribe/risk/ (use with `seykota risk show --section`)
	Formula      string   `json:"formula,omitempty"`
	Command      string   `json:"command,omitempty"` // `seykota ...` subcommand that computes it
	Blurb        string   `json:"blurb"`
}

var metrics = []Metric{
	{
		Key: "heat", Name: "Heat (per-trade and total)", Aliases: []string{"risk-per-trade", "portfolio-heat"},
		EssaySection: "The Uncle Point",
		Formula:      "per-trade heat = dollars at risk ÷ equity ;  total heat = Σ per-trade heat across open positions",
		Command:      "seykota risk heat --equity E --risk-pct p --entry x --stop s  (add --positions name:entry:stop:riskPct,… to sum)",
		Blurb:        "How much of your equity is exposed: a five-position book risking 2% on each carries 10% total heat. Seykota's simulations put optimal total heat near 140%; well past that, losses dominate. You control heat through position sizing, not by changing the market's luck or payoff.",
	},
	{
		Key: "kelly", Name: "The Kelly Formula", Aliases: []string{"kelly-formula", "optimal-f", "optimal-fraction", "k"},
		EssaySection: "The Kelly Formula",
		Formula:      "K = W − (1 − W) / R   (W = win probability, R = win/loss payoff ratio)",
		Command:      "seykota risk kelly --win-rate W --payoff R",
		Blurb:        "The fixed bet fraction that maximizes long-run growth. For a fair coin paying 2:1, K = 0.5 − 0.5/2 = 0.25 — bet a quarter of the stake. Betting more than K (the Bold Trader) grows faster but ruin risk rises sharply; betting less (the Timid Trader) is safer but slower.",
	},
	{
		Key: "uncle-point", Name: "The Uncle Point", Aliases: []string{"uncle", "unclepoint"},
		EssaySection: "The Uncle Point",
		Formula:      "uncle point = equity × (1 − max drawdown you'll tolerate)",
		Command:      "seykota risk uncle-point --equity E --drawdown-pct d",
		Blurb:        "The equity level at which a trader 'cries uncle' and quits the system. Every fixed-fraction system must stay above it; naming it makes the rest of the math (bet fraction, heat) about keeping you off it.",
	},
	{
		Key: "lake-ratio", Name: "The Lake Ratio", Aliases: []string{"lakeratio", "lake"},
		EssaySection: "Lake Ratio",
		Formula:      "lake ratio = area of drawdown 'water' ÷ area of equity 'earth'   ( Σ(peak−value) ÷ Σ value )",
		Command:      "seykota risk lake-ratio --equity-curve <file|->",
		Blurb:        "Picture the equity curve as terrain: when it dips, water (drawdown) pools in the valley. The Lake Ratio is the ratio of that water's area to the area under the curve — 0 for a curve that never drops, larger the more time and depth it spends underwater. A drawdown-aware companion to max drawdown.",
	},
	{
		Key: "coin-toss", Name: "The Coin-Toss model", Aliases: []string{"cointoss", "fixed-fraction", "simulation"},
		EssaySection: "The Coin Toss Example",
		Formula:      "each bet: equity ×= (1 + f·R) on a win, equity ×= (1 − f) on a loss   (f = bet fraction)",
		Command:      "seykota risk coin-toss --win-rate W --payoff R --bet-fraction f --trials N --runs M [--seed s]",
		Blurb:        "The essay's pedagogical engine: a long series of fixed-fraction bets on a biased coin with a payoff ratio. Run many such series and you see growth, drawdown, and probability of ruin trade off against the bet fraction — and why the optimal fraction is K, not 'as much as possible'.",
	},
	{
		Key: "timid-bold", Name: "The Timid Trader / Bold Trader rules", Aliases: []string{"timid", "bold", "timid-trader", "bold-trader"},
		EssaySection: "The Timid Trader Rule",
		Formula:      "timid: bet < K → less growth, less risk ;  bold: bet > K → more growth, sharply more ruin risk",
		Command:      "seykota risk kelly … (compare your fraction to K) ;  seykota risk coin-toss … (see the trade-off)",
		Blurb:        "Two heuristics from the essay. The Timid Trader rule: betting less than the optimal fraction yields slower growth. The Bold Trader rule: betting more than optimal yields faster growth right up until ruin overwhelms it. They bracket the fixed-fraction sweet spot.",
	},
}

// MetricKeys returns the canonical metric keys, sorted.
func MetricKeys() []string {
	out := make([]string, 0, len(metrics))
	for _, m := range metrics {
		out = append(out, m.Key)
	}
	sort.Strings(out)
	return out
}

// AllMetrics returns a copy of the metric list.
func AllMetrics() []Metric {
	out := make([]Metric, len(metrics))
	copy(out, metrics)
	return out
}

// LookupMetric resolves a metric by key or alias (case-insensitive,
// hyphen-insensitive).
func LookupMetric(name string) (Metric, bool) {
	norm := func(s string) string {
		s = strings.ToLower(strings.TrimSpace(s))
		return strings.ReplaceAll(strings.ReplaceAll(s, "_", ""), "-", "")
	}
	n := norm(name)
	for _, m := range metrics {
		if norm(m.Key) == n {
			return m, true
		}
		for _, a := range m.Aliases {
			if norm(a) == n {
				return m, true
			}
		}
		if norm(m.Name) == n {
			return m, true
		}
	}
	return Metric{}, false
}
