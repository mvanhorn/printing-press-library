// Copyright 2026 kjuju600. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/seykota/internal/crawl"
	"github.com/mvanhorn/printing-press-library/library/productivity/seykota/internal/risk"
)

func newRiskCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "risk",
		Short: "Ed Seykota's risk math — read the essay and run its calculators (Kelly, heat, Uncle Point, Coin-Toss, Lake Ratio)",
		Long: `Ed Seykota's "Risk Management" essay (seykota.com/tribe/risk/) is the
canonical primary source on trend-following risk control. 'risk show'
prints it from the local archive. The other subcommands turn its math into
runnable commands:

  kelly         the optimal fixed bet fraction  K = W − (1 − W)/R
  heat          per-trade and portfolio "heat" (fixed-fraction sizing)
  uncle-point   the equity level you must stay above
  coin-toss     Monte-Carlo the essay's fixed-fraction Coin-Toss model
  lake-ratio    the Lake Ratio over your own equity curve
  explain       the essay passage that defines a metric + the command that computes it

These are operationalizations of the essay's formulas, not trading advice.`,
	}
	cmd.AddCommand(newRiskShowCmd(flags))
	cmd.AddCommand(newRiskKellyCmd(flags))
	cmd.AddCommand(newRiskHeatCmd(flags))
	cmd.AddCommand(newRiskUnclePointCmd(flags))
	cmd.AddCommand(newRiskCoinTossCmd(flags))
	cmd.AddCommand(newRiskLakeRatioCmd(flags))
	cmd.AddCommand(newRiskExplainCmd(flags))
	return cmd
}

func newRiskShowCmd(flags *rootFlags) *cobra.Command {
	var dbPath, section string
	var listSections bool
	var maxChars int
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Print Ed Seykota's 'Risk Management' essay from the local archive (or one section)",
		Example: strings.Trim(`
  seykota-pp-cli risk show
  seykota-pp-cli risk show --list
  seykota-pp-cli risk show --section "The Kelly Formula"
  seykota-pp-cli risk show --section "Lake Ratio" --max 3000
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if listSections {
				hs := crawl.RiskHeadings()
				if wantsJSON(cmd, flags) {
					return emitJSON(cmd, flags, map[string]any{"sections": hs})
				}
				for _, h := range hs {
					fmt.Fprintln(cmd.OutOrStdout(), h)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "\nJump to one:  seykota-pp-cli risk show --section \"<name>\"")
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}
			s, err := openCorpus(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			d, err := s.RiskDoc()
			if err != nil {
				if strings.Contains(err.Error(), "no rows") {
					return notFoundErr(fmt.Errorf("the risk essay is not in the local archive — run 'seykota-pp-cli index build'"))
				}
				return err
			}
			body := d.Body
			sect := strings.TrimSpace(section)
			if sect != "" {
				win, ok := crawl.RiskSectionWindow(d.Body, sect)
				if !ok {
					return notFoundErr(fmt.Errorf("section %q not found in the risk essay — try 'seykota-pp-cli risk show --list'", sect))
				}
				body = win
			}
			if maxChars > 0 && len(body) > maxChars {
				body = body[:maxChars] + "\n…(truncated; use --max 0 for the full text)…"
			}
			if wantsJSON(cmd, flags) {
				return emitJSON(cmd, flags, map[string]any{"title": d.Title, "url": d.URL, "section": sect, "body": body})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Ed Seykota — \"Risk Management\"\n%s\n\n%s\n", d.URL, body)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Archive DB path (default: the standard data dir)")
	cmd.Flags().StringVar(&section, "section", "", "Print only this section (case-insensitive; see --list)")
	cmd.Flags().BoolVar(&listSections, "list", false, "List the essay's section headings instead of printing it")
	cmd.Flags().IntVar(&maxChars, "max", 0, "Truncate output to this many characters (0 = full text)")
	return cmd
}

func newRiskKellyCmd(flags *rootFlags) *cobra.Command {
	var winRate, payoff float64
	cmd := &cobra.Command{
		Use:   "kelly",
		Short: "The Kelly fraction K = W − (1 − W)/R: the optimal fixed bet fraction for win-rate W and payoff R",
		Long: `Computes the Kelly criterion exactly as the essay states it:

  K = W − (1 − W) / R

W is the probability of a win (0..1), R is the win/loss payoff ratio. For a
fair coin paying 2:1, K = 0.5 − 0.5/2 = 0.25 — bet a quarter of the stake.
Also reports half-Kelly (a common conservative choice) and the expected
return per unit bet.`,
		Example: strings.Trim(`
  seykota-pp-cli risk kelly --win-rate 0.5 --payoff 2
  seykota-pp-cli risk kelly --win-rate 0.4 --payoff 3 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := risk.KellyReport(winRate, payoff)
			if err != nil {
				return usageErr(err)
			}
			if wantsJSON(cmd, flags) {
				return emitJSON(cmd, flags, r)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Win rate W = %.4g   Payoff R = %.4g\n", r.WinRate, r.Payoff)
			fmt.Fprintf(cmd.OutOrStdout(), "Edge per bet (W·R − (1−W)) = %+.4g\n", r.EdgePerBet)
			fmt.Fprintf(cmd.OutOrStdout(), "Kelly fraction K = W − (1−W)/R = %.4f  (%.2f%% of equity per trade)\n", r.KellyFraction, r.KellyPct)
			fmt.Fprintf(cmd.OutOrStdout(), "Half-Kelly = %.2f%%\n", r.HalfKellyPct)
			if !r.HasEdge {
				fmt.Fprintln(cmd.OutOrStdout(), "No positive edge — the optimal bet is 0. Any positive bet is overbetting.")
			}
			return nil
		},
	}
	cmd.Flags().Float64Var(&winRate, "win-rate", 0, "Probability of a win, 0..1 (required)")
	cmd.Flags().Float64Var(&payoff, "payoff", 0, "Win/loss payoff ratio R, > 0 (required)")
	return cmd
}

func newRiskHeatCmd(flags *rootFlags) *cobra.Command {
	var equity, riskPct, entry, stop float64
	var positions string
	cmd := &cobra.Command{
		Use:   "heat",
		Short: "Fixed-fraction position sizing and 'heat' — per trade, or summed across positions",
		Long: `Sizes one position to risk a fixed fraction of equity between entry and
stop (shares floored to a whole number, so realized heat is at or just
below the target):

  dollars at risk = equity × risk-pct/100
  shares          = floor(dollars at risk ÷ |entry − stop|)
  heat            = (shares × |entry − stop|) ÷ equity

Pass --positions name:entry:stop:riskPct,… (same --equity for all) to size
a book and sum the total "heat" — Seykota's term for total risk exposure.
A five-position book risking 2% on each carries ~10% total heat; the
essay's simulations put the optimal *total* heat near 140%, well past which
losses dominate.`,
		Example: strings.Trim(`
  seykota-pp-cli risk heat --equity 100000 --risk-pct 1 --entry 50 --stop 45
  seykota-pp-cli risk heat --equity 250000 --risk-pct 0.5 --entry 412.30 --stop 398.00 --json --select shares,dollars_at_risk,heat_pct
  seykota-pp-cli risk heat --equity 100000 --positions "AAPL:200:185:1,MSFT:400:370:0.5,GLD:185:178:0.75"
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(positions) != "" {
				ps, err := risk.ParsePositions(positions)
				if err != nil {
					return usageErr(err)
				}
				res, err := risk.PortfolioHeat(equity, ps)
				if err != nil {
					return usageErr(err)
				}
				if wantsJSON(cmd, flags) {
					return emitJSON(cmd, flags, res)
				}
				rows := make([][]string, 0, len(res.Positions))
				for i, hr := range res.Positions {
					rows = append(rows, []string{
						res.Names[i],
						fmt.Sprintf("%.2f", hr.Entry), fmt.Sprintf("%.2f", hr.Stop),
						fmt.Sprintf("%d", hr.Shares), fmt.Sprintf("$%.2f", hr.DollarsAtRisk),
						fmt.Sprintf("%.3f%%", hr.HeatPct), fmt.Sprintf("$%.0f", hr.PositionValue),
					})
				}
				_ = printRows(cmd, []string{"NAME", "ENTRY", "STOP", "SHARES", "AT-RISK", "HEAT", "POS-VALUE"}, rows)
				fmt.Fprintf(cmd.OutOrStdout(), "\nEquity: $%.0f   Total portfolio heat: %.3f%%\n", res.Equity, res.TotalHeatPct)
				return nil
			}
			hr, err := risk.HeatPerTrade(equity, riskPct, entry, stop)
			if err != nil {
				return usageErr(err)
			}
			if wantsJSON(cmd, flags) {
				return emitJSON(cmd, flags, hr)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Equity $%.2f   risk %.4g%%   entry %.4g   stop %.4g\n", hr.Equity, hr.RiskPct, hr.Entry, hr.Stop)
			fmt.Fprintf(cmd.OutOrStdout(), "Risk per share = |entry − stop| = %.4g\n", hr.RiskPerShare)
			fmt.Fprintf(cmd.OutOrStdout(), "Target dollars at risk = %.2f%% × $%.2f = $%.2f\n", hr.RiskPct, hr.Equity, hr.TargetDollars)
			fmt.Fprintf(cmd.OutOrStdout(), "→ Shares = %d   (dollars at risk = $%.2f, heat = %.3f%%)\n", hr.Shares, hr.DollarsAtRisk, hr.HeatPct)
			fmt.Fprintf(cmd.OutOrStdout(), "  Position value = $%.2f  (%.2f%% of equity)\n", hr.PositionValue, hr.PositionPctEq)
			return nil
		},
	}
	cmd.Flags().Float64Var(&equity, "equity", 0, "Account equity (required)")
	cmd.Flags().Float64Var(&riskPct, "risk-pct", 0, "Percent of equity to risk on the trade, e.g. 1 for 1% (required unless --positions)")
	cmd.Flags().Float64Var(&entry, "entry", 0, "Entry price (required unless --positions)")
	cmd.Flags().Float64Var(&stop, "stop", 0, "Stop price (required unless --positions)")
	cmd.Flags().StringVar(&positions, "positions", "", "Size a book and sum total heat: name:entry:stop:riskPct,…")
	return cmd
}

func newRiskUnclePointCmd(flags *rootFlags) *cobra.Command {
	var equity, drawdownPct float64
	cmd := &cobra.Command{
		Use:   "uncle-point",
		Short: "The Uncle Point — the equity level you'd quit at = equity × (1 − max drawdown you'll tolerate)",
		Long: `The "Uncle Point" is the equity level at which a trader cries uncle and
abandons the system. A fixed-fraction system must stay above it; naming it
makes the rest of the math (bet fraction, heat) about keeping you off it.

  uncle point = equity × (1 − max-drawdown-pct/100)`,
		Example: strings.Trim(`
  seykota-pp-cli risk uncle-point --equity 100000 --drawdown-pct 30
  seykota-pp-cli risk uncle-point --equity 250000 --drawdown-pct 20 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := risk.UnclePoint(equity, drawdownPct)
			if err != nil {
				return usageErr(err)
			}
			if wantsJSON(cmd, flags) {
				return emitJSON(cmd, flags, r)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Equity $%.2f   max drawdown you'll tolerate: %.4g%%\n", r.Equity, r.MaxDrawdownPct)
			fmt.Fprintf(cmd.OutOrStdout(), "→ Uncle Point = $%.2f   (you can lose $%.2f before you hit it)\n", r.UnclePoint, r.RoomToLose)
			return nil
		},
	}
	cmd.Flags().Float64Var(&equity, "equity", 0, "Account equity (required)")
	cmd.Flags().Float64Var(&drawdownPct, "drawdown-pct", 0, "Max drawdown you'd tolerate before quitting, e.g. 30 (required, 0 < d < 100)")
	return cmd
}

func newRiskCoinTossCmd(flags *rootFlags) *cobra.Command {
	var winRate, payoff, betFraction, ruinFrac float64
	var trials, runs int
	var seed int64
	cmd := &cobra.Command{
		Use:   "coin-toss",
		Short: "Monte-Carlo the essay's fixed-fraction Coin-Toss model: median equity, ruin probability, max drawdown, vs optimal-f",
		Long: `Simulates the essay's pedagogical engine: a long series of fixed-fraction
bets on a biased coin with a payoff ratio. Each bet multiplies equity by
(1 + f·R) on a win or (1 − f) on a loss. Runs many independent series and
reports the distribution of terminal equity, the probability of ruin (a run
that falls to --ruin-fraction of the stake), the mean maximum drawdown, and
how the chosen bet fraction compares to the optimal (Kelly) fraction.

Deterministic for a given --seed.`,
		Example: strings.Trim(`
  seykota-pp-cli risk coin-toss --win-rate 0.5 --payoff 2 --bet-fraction 0.25 --trials 100 --runs 10000 --seed 1
  seykota-pp-cli risk coin-toss --win-rate 0.45 --payoff 2.5 --bet-fraction 0.1 --trials 200 --runs 50000 --seed 7 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := risk.CoinToss(risk.CoinTossOpts{
				WinRate: winRate, Payoff: payoff, BetFraction: betFraction,
				Trials: trials, Runs: runs, Seed: seed, RuinFrac: ruinFrac,
			})
			if err != nil {
				return usageErr(err)
			}
			if wantsJSON(cmd, flags) {
				return emitJSON(cmd, flags, res)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Coin-Toss: W=%.4g  R=%.4g  bet-fraction=%.4g  trials=%d  runs=%d  seed=%d  (ruin = equity ≤ %.2g× start)\n",
				res.WinRate, res.Payoff, res.BetFraction, res.Trials, res.Runs, res.Seed, res.RuinFraction)
			fmt.Fprintf(cmd.OutOrStdout(), "Terminal equity (× start):  median %.4g   mean %.4g   P10 %.4g   P90 %.4g   worst %.4g   best %.4g\n",
				res.MedianTerminal, res.MeanTerminal, res.P10Terminal, res.P90Terminal, res.WorstTerminal, res.BestTerminal)
			fmt.Fprintf(cmd.OutOrStdout(), "Probability of ruin: %.2f%%   Mean max drawdown: %.1f%%\n", res.RuinProbability*100, res.MeanMaxDrawdown)
			fmt.Fprintf(cmd.OutOrStdout(), "Optimal (Kelly) fraction: %.4f  →  %s\n", res.OptimalFraction, res.BetVsOptimal)
			return nil
		},
	}
	cmd.Flags().Float64Var(&winRate, "win-rate", 0, "Probability of a win, 0..1 (required)")
	cmd.Flags().Float64Var(&payoff, "payoff", 0, "Win/loss payoff ratio R, > 0 (required)")
	cmd.Flags().Float64Var(&betFraction, "bet-fraction", 0, "Fraction of equity risked per bet, 0..1 (required)")
	cmd.Flags().IntVar(&trials, "trials", 100, "Bets per run")
	cmd.Flags().IntVar(&runs, "runs", 10000, "Independent runs to simulate")
	cmd.Flags().Int64Var(&seed, "seed", 1, "RNG seed (same seed → same result)")
	cmd.Flags().Float64Var(&ruinFrac, "ruin-fraction", 0.10, "A run is 'ruined' if equity ever falls to this fraction of the starting stake")
	return cmd
}

func newRiskLakeRatioCmd(flags *rootFlags) *cobra.Command {
	var path, values string
	cmd := &cobra.Command{
		Use:   "lake-ratio",
		Short: "Compute Seykota's Lake Ratio over an equity curve — Σ(peak−value) ÷ Σ value",
		Long: `Picture an equity curve as terrain: when it dips, water (drawdown) pools in
the valley. The Lake Ratio is the ratio of that water's area to the area
under the curve:

  lake ratio = Σ(running-peak_i − value_i) ÷ Σ value_i

0 for a curve that never drops; larger the more time and depth it spends
underwater. Provide the curve inline with --values 100,105,98,110,…, or
from a file/stdin with --equity-curve <file|-> (one number per line, or a
CSV whose last numeric column on each line is the equity; blank and #
comment lines are ignored).`,
		Example: strings.Trim(`
  seykota-pp-cli risk lake-ratio --values 100,105,98,110,102,120
  seykota-pp-cli risk lake-ratio --equity-curve equity.csv --json
  printf '100\n105\n98\n110\n102\n120\n' | seykota-pp-cli risk lake-ratio --equity-curve -
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			vals := strings.TrimSpace(values)
			p := strings.TrimSpace(path)
			if vals == "" && p == "" {
				return usageErr(fmt.Errorf("provide the equity series with --values 100,105,98,… or --equity-curve <file|->"))
			}
			if vals != "" && p != "" {
				return usageErr(fmt.Errorf("use either --values or --equity-curve, not both"))
			}
			var raw string
			if vals != "" {
				raw = strings.ReplaceAll(vals, ",", "\n")
			} else if p == "-" {
				b, err := io.ReadAll(cmd.InOrStdin())
				if err != nil {
					return fmt.Errorf("reading equity curve from stdin: %w", err)
				}
				raw = string(b)
			} else {
				b, err := os.ReadFile(p)
				if err != nil {
					return fmt.Errorf("reading equity curve: %w", err)
				}
				raw = string(b)
			}
			curve, err := risk.ParseEquityCurve(raw)
			if err != nil {
				return usageErr(err)
			}
			res, err := risk.LakeRatio(curve)
			if err != nil {
				return usageErr(err)
			}
			if wantsJSON(cmd, flags) {
				return emitJSON(cmd, flags, res)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Equity curve: %d points   start %.4g → end %.4g\n", res.Points, res.StartVal, res.EndVal)
			fmt.Fprintf(cmd.OutOrStdout(), "Lake area (Σ peak−value) = %.4g   Earth area (Σ value) = %.4g\n", res.LakeArea, res.EarthArea)
			fmt.Fprintf(cmd.OutOrStdout(), "→ Lake Ratio = %.4f   (max drawdown along the way: %.1f%%)\n", res.LakeRatio, res.MaxDDpct)
			return nil
		},
	}
	cmd.Flags().StringVar(&values, "values", "", "Equity series inline, comma-separated (e.g. 100,105,98,110)")
	cmd.Flags().StringVar(&path, "equity-curve", "", "Path to a file with the equity series (or - for stdin)")
	return cmd
}

func newRiskExplainCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "explain [metric]",
		Short: "For a named risk metric, print the essay passage that defines it + the formula + the command that computes it",
		Long: `Bridges the prose and the math. With no argument, lists the metric
vocabulary. With a metric (heat, kelly, uncle-point, lake-ratio, coin-toss,
timid-bold), prints which section of the risk essay defines it, the formula,
the matching 'seykota risk …' subcommand, and a one-line gloss.`,
		Example: strings.Trim(`
  seykota-pp-cli risk explain
  seykota-pp-cli risk explain heat
  seykota-pp-cli risk explain uncle-point --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				ms := risk.AllMetrics()
				if wantsJSON(cmd, flags) {
					return emitJSON(cmd, flags, map[string]any{"count": len(ms), "metrics": ms})
				}
				rows := make([][]string, 0, len(ms))
				for _, m := range ms {
					rows = append(rows, []string{m.Key, m.Name, m.Command})
				}
				_ = printRows(cmd, []string{"METRIC", "NAME", "COMMAND"}, rows)
				fmt.Fprintln(cmd.OutOrStdout(), "\nExplain one:  seykota-pp-cli risk explain <metric>")
				return nil
			}
			m, ok := risk.LookupMetric(args[0])
			if !ok {
				return notFoundErr(fmt.Errorf("unknown metric %q — run 'seykota-pp-cli risk explain' for the list", args[0]))
			}
			if wantsJSON(cmd, flags) {
				return emitJSON(cmd, flags, m)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n%s\n\n", m.Name, m.Blurb)
			if m.Formula != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Formula:        %s\n", m.Formula)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Essay section:  %s\n  (read it: seykota-pp-cli risk show --section %q)\n", m.EssaySection, m.EssaySection)
			if m.Command != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Compute it:     %s\n", m.Command)
			}
			return nil
		},
	}
	return cmd
}
