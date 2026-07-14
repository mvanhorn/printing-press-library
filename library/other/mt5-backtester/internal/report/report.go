package report

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
)

// Stats holds all key metrics from a backtest report.
type Stats struct {
	// Identity
	Expert   string
	Symbol   string
	Period   string
	FromDate string
	ToDate   string
	Model    string
	Deposit  float64
	Currency string

	// Core results
	NetProfit      float64
	GrossProfit    float64
	GrossLoss      float64
	ProfitFactor   float64
	ExpectedPayoff float64
	SharpeRatio    float64

	// Drawdown
	AbsoluteDrawdown float64
	MaxDrawdown      float64
	MaxDrawdownPct   float64
	RelativeDrawdown float64

	// Trade stats
	TotalTrades   int
	ShortTrades   int
	LongTrades    int
	ProfitTrades  int
	LossTrades    int
	LargestWin    float64
	LargestLoss   float64
	AvgWin        float64
	AvgLoss       float64
	MaxConsecWins int
	MaxConsecLoss int
	WinRate       float64

	// Balance
	InitialDeposit float64
	FinalBalance   float64
	ReturnPct      float64

	// Quality
	Bars         int
	Ticks        int
	ModelQuality string
	ParsedAt     time.Time
}

var (
	// Patterns to extract values from MT5 HTML reports
	reNetProfit      = regexp.MustCompile(`(?i)net profit[^<]*<[^>]+>([^<]+)`)
	reGrossProfit    = regexp.MustCompile(`(?i)gross profit[^<]*<[^>]+>([^<]+)`)
	reGrossLoss      = regexp.MustCompile(`(?i)gross loss[^<]*<[^>]+>([^<]+)`)
	reProfitFactor   = regexp.MustCompile(`(?i)profit factor[^<]*<[^>]+>([^<]+)`)
	reSharpe         = regexp.MustCompile(`(?i)sharpe ratio[^<]*<[^>]+>([^<]+)`)
	reExpectedPayoff = regexp.MustCompile(`(?i)expected payoff[^<]*<[^>]+>([^<]+)`)
	reMaxDrawdown    = regexp.MustCompile(`(?i)maximal drawdown[^<]*<[^>]+>([\d\s.,]+)\s*\(([\d.,]+)%\)`)
	reRelDrawdown    = regexp.MustCompile(`(?i)relative drawdown[^<]*<[^>]+>([\d.,]+)%`)
	reTotalTrades    = regexp.MustCompile(`(?i)total trades[^<]*<[^>]+>(\d+)`)
	reShortTrades    = regexp.MustCompile(`(?i)short positions[^<]*<[^>]+>(\d+)`)
	reLongTrades     = regexp.MustCompile(`(?i)long positions[^<]*<[^>]+>(\d+)`)
	reProfitTrades   = regexp.MustCompile(`(?i)profit trades[^<]*<[^>]+>(\d+)`)
	reLossTrades     = regexp.MustCompile(`(?i)loss trades[^<]*<[^>]+>(\d+)`)
	reLargestWin     = regexp.MustCompile(`(?i)largest profit trade[^<]*<[^>]+>([^<]+)`)
	reLargestLoss    = regexp.MustCompile(`(?i)largest loss trade[^<]*<[^>]+>([^<]+)`)
	reAvgWin         = regexp.MustCompile(`(?i)average profit trade[^<]*<[^>]+>([^<]+)`)
	reAvgLoss        = regexp.MustCompile(`(?i)average loss trade[^<]*<[^>]+>([^<]+)`)
	reMaxConsecWins  = regexp.MustCompile(`(?i)maximum consecutive wins[^<]*<[^>]+>(\d+)`)
	reMaxConsecLoss  = regexp.MustCompile(`(?i)maximum consecutive losses[^<]*<[^>]+>(\d+)`)
	reInitDeposit    = regexp.MustCompile(`(?i)initial deposit[^<]*<[^>]+>([^<]+)`)
	reFinalBalance   = regexp.MustCompile(`(?i)final balance[^<]*<[^>]+>([^<]+)`)
	reExpert         = regexp.MustCompile(`(?i)expert:\s*<[^>]+>([^<]+)`)
	reSymbol         = regexp.MustCompile(`(?i)symbol:\s*<[^>]+>([^<]+)`)
	rePeriod         = regexp.MustCompile(`(?i)period:\s*<[^>]+>([^<]+)`)
	reBars           = regexp.MustCompile(`(?i)bars in test[^<]*<[^>]+>(\d+)`)
	reTicks          = regexp.MustCompile(`(?i)ticks modelled[^<]*<[^>]+>(\d+)`)
	reQuality        = regexp.MustCompile(`(?i)modelling quality[^<]*<[^>]+>([^<]+)`)
)

// ParseFile reads an MT5 HTML report and extracts key statistics.
func ParseFile(path string) (*Stats, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open report: %w", err)
	}
	defer f.Close()

	var sb strings.Builder
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		sb.WriteString(scanner.Text())
		sb.WriteString("\n")
	}
	content := sb.String()

	stats := &Stats{ParsedAt: time.Now()}

	extractFloat := func(re *regexp.Regexp) float64 {
		if m := re.FindStringSubmatch(content); len(m) > 1 {
			v := cleanNumber(m[1])
			f, _ := strconv.ParseFloat(v, 64)
			return f
		}
		return 0
	}
	extractInt := func(re *regexp.Regexp) int {
		if m := re.FindStringSubmatch(content); len(m) > 1 {
			v := strings.TrimSpace(m[1])
			i, _ := strconv.Atoi(v)
			return i
		}
		return 0
	}
	extractStr := func(re *regexp.Regexp) string {
		if m := re.FindStringSubmatch(content); len(m) > 1 {
			return strings.TrimSpace(stripTags(m[1]))
		}
		return ""
	}

	stats.Expert = extractStr(reExpert)
	stats.Symbol = extractStr(reSymbol)
	stats.Period = extractStr(rePeriod)
	stats.NetProfit = extractFloat(reNetProfit)
	stats.GrossProfit = extractFloat(reGrossProfit)
	stats.GrossLoss = extractFloat(reGrossLoss)
	stats.ProfitFactor = extractFloat(reProfitFactor)
	stats.SharpeRatio = extractFloat(reSharpe)
	stats.ExpectedPayoff = extractFloat(reExpectedPayoff)
	stats.LargestWin = extractFloat(reLargestWin)
	stats.LargestLoss = extractFloat(reLargestLoss)
	stats.AvgWin = extractFloat(reAvgWin)
	stats.AvgLoss = extractFloat(reAvgLoss)
	stats.InitialDeposit = extractFloat(reInitDeposit)
	stats.FinalBalance = extractFloat(reFinalBalance)
	stats.TotalTrades = extractInt(reTotalTrades)
	stats.ShortTrades = extractInt(reShortTrades)
	stats.LongTrades = extractInt(reLongTrades)
	stats.ProfitTrades = extractInt(reProfitTrades)
	stats.LossTrades = extractInt(reLossTrades)
	stats.MaxConsecWins = extractInt(reMaxConsecWins)
	stats.MaxConsecLoss = extractInt(reMaxConsecLoss)
	stats.Bars = extractInt(reBars)
	stats.Ticks = extractInt(reTicks)
	stats.ModelQuality = extractStr(reQuality)

	// Drawdown (has pct in parens)
	if m := reMaxDrawdown.FindStringSubmatch(content); len(m) > 2 {
		stats.MaxDrawdown, _ = strconv.ParseFloat(cleanNumber(m[1]), 64)
		stats.MaxDrawdownPct, _ = strconv.ParseFloat(cleanNumber(m[2]), 64)
	}
	if m := reRelDrawdown.FindStringSubmatch(content); len(m) > 1 {
		stats.RelativeDrawdown, _ = strconv.ParseFloat(cleanNumber(m[1]), 64)
	}

	// Derived
	if stats.TotalTrades > 0 {
		stats.WinRate = float64(stats.ProfitTrades) / float64(stats.TotalTrades) * 100
	}
	if stats.InitialDeposit > 0 {
		stats.ReturnPct = (stats.FinalBalance - stats.InitialDeposit) / stats.InitialDeposit * 100
	}

	return stats, nil
}

// Print displays a formatted summary of backtest results.
func Print(s *Stats, verbose bool) {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════╗")
	fmt.Printf("║  MT5 Backtest Report                                 ║\n")
	fmt.Println("╚══════════════════════════════════════════════════════╝")

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	if s.Expert != "" {
		fmt.Fprintf(w, "Expert:\t%s\n", s.Expert)
	}
	if s.Symbol != "" {
		fmt.Fprintf(w, "Symbol:\t%s  %s\n", s.Symbol, s.Period)
	}
	fmt.Fprintf(w, "Bars / Ticks:\t%s / %s\n", fmtInt(s.Bars), fmtInt(s.Ticks))
	if s.ModelQuality != "" {
		fmt.Fprintf(w, "Model Quality:\t%s\n", s.ModelQuality)
	}
	w.Flush()

	fmt.Println("\n── P&L ─────────────────────────────────────────────────")
	w = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Net Profit:\t%s\n", fmtMoney(s.NetProfit))
	fmt.Fprintf(w, "Gross Profit:\t%s\n", fmtMoney(s.GrossProfit))
	fmt.Fprintf(w, "Gross Loss:\t%s\n", fmtMoney(s.GrossLoss))
	fmt.Fprintf(w, "Profit Factor:\t%.2f\n", s.ProfitFactor)
	fmt.Fprintf(w, "Sharpe Ratio:\t%.2f\n", s.SharpeRatio)
	fmt.Fprintf(w, "Expected Payoff:\t%s\n", fmtMoney(s.ExpectedPayoff))
	if s.InitialDeposit > 0 {
		fmt.Fprintf(w, "Return:\t%.2f%% (%.2f → %.2f)\n", s.ReturnPct, s.InitialDeposit, s.FinalBalance)
	}
	w.Flush()

	fmt.Println("\n── Drawdown ─────────────────────────────────────────────")
	w = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Max Drawdown:\t%s (%.2f%%)\n", fmtMoney(s.MaxDrawdown), s.MaxDrawdownPct)
	fmt.Fprintf(w, "Relative Drawdown:\t%.2f%%\n", s.RelativeDrawdown)
	w.Flush()

	fmt.Println("\n── Trades ───────────────────────────────────────────────")
	w = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Total Trades:\t%d\n", s.TotalTrades)
	fmt.Fprintf(w, "Long / Short:\t%d / %d\n", s.LongTrades, s.ShortTrades)
	fmt.Fprintf(w, "Profit / Loss:\t%d / %d  (%.1f%% win rate)\n", s.ProfitTrades, s.LossTrades, s.WinRate)
	fmt.Fprintf(w, "Largest Win:\t%s\n", fmtMoney(s.LargestWin))
	fmt.Fprintf(w, "Largest Loss:\t%s\n", fmtMoney(s.LargestLoss))
	fmt.Fprintf(w, "Avg Win:\t%s\n", fmtMoney(s.AvgWin))
	fmt.Fprintf(w, "Avg Loss:\t%s\n", fmtMoney(s.AvgLoss))
	fmt.Fprintf(w, "Max Consec Wins:\t%d\n", s.MaxConsecWins)
	fmt.Fprintf(w, "Max Consec Losses:\t%d\n", s.MaxConsecLoss)
	w.Flush()
	fmt.Println()
}

// PrintJSON outputs stats as JSON (for agent-native usage).
func PrintJSON(s *Stats) {
	fmt.Printf(`{
  "expert": %q,
  "symbol": %q,
  "period": %q,
  "net_profit": %.2f,
  "profit_factor": %.2f,
  "sharpe_ratio": %.2f,
  "max_drawdown_pct": %.2f,
  "relative_drawdown_pct": %.2f,
  "total_trades": %d,
  "win_rate_pct": %.2f,
  "expected_payoff": %.2f,
  "return_pct": %.2f,
  "initial_deposit": %.2f,
  "final_balance": %.2f,
  "bars": %d,
  "ticks": %d,
  "model_quality": %q
}
`,
		s.Expert, s.Symbol, s.Period,
		s.NetProfit, s.ProfitFactor, s.SharpeRatio,
		s.MaxDrawdownPct, s.RelativeDrawdown,
		s.TotalTrades, s.WinRate, s.ExpectedPayoff,
		s.ReturnPct, s.InitialDeposit, s.FinalBalance,
		s.Bars, s.Ticks, s.ModelQuality,
	)
}

// PrintCSV outputs a single CSV row (for batch comparison).
func PrintCSVHeader() {
	fmt.Println("expert,symbol,period,net_profit,profit_factor,sharpe,max_dd_pct,win_rate,trades,return_pct")
}

func PrintCSVRow(s *Stats) {
	fmt.Printf("%q,%s,%s,%.2f,%.2f,%.2f,%.2f,%.1f,%d,%.2f\n",
		s.Expert, s.Symbol, s.Period,
		s.NetProfit, s.ProfitFactor, s.SharpeRatio,
		s.MaxDrawdownPct, s.WinRate, s.TotalTrades, s.ReturnPct,
	)
}

// helpers

// cleanNumber normalizes an MT5 report numeric cell for strconv parsing.
// Reports can be locale-formatted: "1,234.56", "1 234.56", or "1 234,56"
// (decimal comma). Spaces/NBSPs are always thousands separators; when both
// '.' and ',' appear, the rightmost is the decimal separator; a lone comma is
// a decimal separator unless it is followed by exactly three digits (the
// English thousands pattern "1,234").
func cleanNumber(s string) string {
	s = stripTags(s)
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\u00a0", "")
	s = strings.ReplaceAll(s, " ", "")
	lastDot := strings.LastIndexByte(s, '.')
	lastComma := strings.LastIndexByte(s, ',')
	switch {
	case lastComma == -1:
		// No commas: nothing to disambiguate.
	case lastDot > lastComma:
		// "1,234.56" — commas are thousands separators.
		s = strings.ReplaceAll(s, ",", "")
	case lastDot != -1:
		// "1.234,56" — dots are thousands separators, comma is decimal.
		s = strings.ReplaceAll(s, ".", "")
		s = strings.Replace(s, ",", ".", 1)
	case strings.Count(s, ",") == 1 && len(s)-lastComma-1 != 3:
		// "1234,56" — decimal comma.
		s = strings.Replace(s, ",", ".", 1)
	default:
		// "1,234" / "1,234,567" — thousands commas.
		s = strings.ReplaceAll(s, ",", "")
	}
	return s
}

func stripTags(s string) string {
	re := regexp.MustCompile(`<[^>]+>`)
	return re.ReplaceAllString(s, "")
}

func fmtMoney(v float64) string {
	if v >= 0 {
		return fmt.Sprintf("+%.2f", v)
	}
	return fmt.Sprintf("%.2f", v)
}

func fmtInt(v int) string {
	if v == 0 {
		return "-"
	}
	s := strconv.Itoa(v)
	// Insert thousands separators
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}
