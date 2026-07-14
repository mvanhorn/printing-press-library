package batch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/mt5-backtester/internal/backtest"
	"github.com/mvanhorn/printing-press-library/library/other/mt5-backtester/internal/config"
	"github.com/mvanhorn/printing-press-library/library/other/mt5-backtester/internal/report"
	"github.com/mvanhorn/printing-press-library/library/other/mt5-backtester/internal/setfile"
)

// ── Job definition ────────────────────────────────────────────────────────────

// Job defines one backtest item. Every field except EA is optional with sensible defaults.
type Job struct {
	EA       string            `json:"ea"`
	Symbol   string            `json:"symbol,omitempty"`
	Period   string            `json:"period,omitempty"`
	From     string            `json:"from,omitempty"`
	To       string            `json:"to,omitempty"`
	Model    *int              `json:"model,omitempty"`
	Deposit  float64           `json:"deposit,omitempty"`
	Currency string            `json:"currency,omitempty"`
	Leverage int               `json:"leverage,omitempty"`
	Profile  string            `json:"profile,omitempty"`
	Inputs   map[string]string `json:"inputs,omitempty"`
	SetFile  string            `json:"set_file,omitempty"`
	Label    string            `json:"label,omitempty"`
}

// Defaults holds batch-level defaults applied to every job.
type Defaults struct {
	Symbol   string            `json:"symbol"`
	Period   string            `json:"period"`
	From     string            `json:"from"`
	To       string            `json:"to"`
	Model    int               `json:"model"`
	Deposit  float64           `json:"deposit"`
	Currency string            `json:"currency"`
	Leverage int               `json:"leverage"`
	Profile  string            `json:"profile"`
	Inputs   map[string]string `json:"inputs,omitempty"`
	SetFile  string            `json:"set_file,omitempty"`
}

// BatchFile is the top-level JSON format.
type BatchFile struct {
	Defaults Defaults `json:"defaults"`
	Jobs     []Job    `json:"jobs"`
}

// Resolve merges job fields with defaults.
func (j Job) Resolve(d Defaults) Job {
	if j.Symbol == "" {
		j.Symbol = d.Symbol
	}
	if j.Period == "" {
		j.Period = d.Period
	}
	if j.From == "" {
		j.From = d.From
	}
	if j.To == "" {
		j.To = d.To
	}
	if j.Model == nil {
		m := d.Model
		j.Model = &m
	}
	if j.Deposit == 0 {
		j.Deposit = d.Deposit
	}
	if j.Currency == "" {
		j.Currency = d.Currency
	}
	if j.Leverage == 0 {
		j.Leverage = d.Leverage
	}
	if j.Profile == "" {
		j.Profile = d.Profile
	}
	if j.SetFile == "" {
		j.SetFile = d.SetFile
	}

	// Merge inputs: defaults first, job inputs override
	if len(d.Inputs) > 0 {
		merged := make(map[string]string)
		for k, v := range d.Inputs {
			merged[k] = v
		}
		for k, v := range j.Inputs {
			merged[k] = v
		}
		j.Inputs = merged
	}

	// Hardcoded fallbacks
	if j.Symbol == "" {
		j.Symbol = "EURUSD"
	}
	if j.Period == "" {
		j.Period = "H1"
	}
	if j.Model == nil {
		m := 1
		j.Model = &m
	}
	if j.Deposit == 0 {
		j.Deposit = 10000
	}
	if j.Currency == "" {
		j.Currency = "USD"
	}
	if j.Leverage == 0 {
		j.Leverage = 100
	}
	if j.From == "" {
		j.From = time.Now().AddDate(-1, 0, 0).Format("2006.01.02")
	}
	if j.To == "" {
		j.To = time.Now().Format("2006.01.02")
	}
	return j
}

func (j Job) displayLabel() string {
	if j.Label != "" {
		return j.Label
	}
	return fmt.Sprintf("%s | %s | %s", j.EA, j.Symbol, j.Period)
}

// ── Result ────────────────────────────────────────────────────────────────────

type JobResult struct {
	Job        Job
	ReportPath string
	Stats      *report.Stats
	Duration   time.Duration
	Error      error
}

// ── Runner ────────────────────────────────────────────────────────────────────

// TerminalResolver maps profile name to terminal path + portable flag.
type TerminalResolver func(profile string) (terminalPath string, portable bool, err error)

// RunBatch executes all jobs sequentially.
func RunBatch(jobs []Job, defaults Defaults, workDir string, timeout time.Duration, resolver TerminalResolver, verbose bool) []JobResult {
	results := make([]JobResult, 0, len(jobs))

	fmt.Printf("\n%d jobs queued\n", len(jobs))
	printJobTable(jobs, defaults)
	fmt.Println()

	for i, rawJob := range jobs {
		job := rawJob.Resolve(defaults)
		fmt.Printf("[%d/%d] %s\n", i+1, len(jobs), job.displayLabel())
		fmt.Printf("       %s → %s  model=%d  deposit=%.0f %s\n",
			job.From, job.To, *job.Model, job.Deposit, job.Currency)

		start := time.Now()

		termPath, portable, err := resolver(job.Profile)
		if err != nil {
			results = append(results, JobResult{Job: job, Error: err, Duration: time.Since(start)})
			fmt.Printf("       ✗ terminal error: %v\n", err)
			continue
		}

		model := 1
		if job.Model != nil {
			model = *job.Model
		}

		var setPath string
		if job.SetFile != "" {
			resolved, rerr := setfile.ResolvePath(job.SetFile, termPath, portable)
			if rerr != nil {
				results = append(results, JobResult{Job: job, Error: rerr, Duration: time.Since(start)})
				fmt.Printf("       ✗ set file: %v\n", rerr)
				continue
			}
			setPath = resolved
			if verbose {
				fmt.Printf("       set: %s\n", setPath)
			}
		}

		params := &config.BacktestParams{
			Expert:       config.ResolveExpertPath(job.EA),
			Symbol:       strings.ToUpper(job.Symbol),
			Period:       strings.ToUpper(job.Period),
			FromDate:     job.From,
			ToDate:       job.To,
			Model:        model,
			Deposit:      job.Deposit,
			Currency:     job.Currency,
			Leverage:     job.Leverage,
			Optimization: 0,
			ShutdownMode: 1,
			Inputs:       job.Inputs,
			SetFile:      setPath,
		}

		iniPath, err := config.WriteINI(params, workDir)
		if err != nil {
			results = append(results, JobResult{Job: job, Error: err, Duration: time.Since(start)})
			fmt.Printf("       ✗ ini error: %v\n", err)
			continue
		}
		if verbose {
			fmt.Printf("       ini: %s\n", iniPath)
		}

		runResult := backtest.Run(backtest.RunOptions{
			TerminalPath: termPath,
			INIPath:      iniPath,
			ReportDir:    workDir,
			Timeout:      timeout,
			Portable:     portable,
			Verbose:      verbose,
		})

		jr := JobResult{Job: job, Duration: time.Since(start)}

		if runResult.Error != nil {
			jr.Error = runResult.Error
			fmt.Printf("       ✗ %v\n", runResult.Error)
		} else if runResult.ReportPath != "" {
			jr.ReportPath = runResult.ReportPath
			stats, parseErr := report.ParseFile(runResult.ReportPath)
			if parseErr == nil {
				jr.Stats = stats
				printInlineResult(stats)
			} else {
				jr.Error = parseErr
				fmt.Printf("       ~ report found but parse failed: %v\n", parseErr)
			}
		} else {
			fmt.Printf("       ~ no report found (exit=%d)\n", runResult.ExitCode)
		}

		results = append(results, jr)
	}
	return results
}

// ── Expand helpers ────────────────────────────────────────────────────────────

func ExpandSymbols(base Job, symbols []string) []Job {
	jobs := make([]Job, 0, len(symbols))
	for _, sym := range symbols {
		j := base
		j.Symbol = sym
		jobs = append(jobs, j)
	}
	return jobs
}

func ExpandPeriods(base Job, periods []string) []Job {
	jobs := make([]Job, 0, len(periods))
	for _, p := range periods {
		j := base
		j.Period = p
		jobs = append(jobs, j)
	}
	return jobs
}

func ExpandEAs(eas []string, symbol, period, from, to string) []Job {
	jobs := make([]Job, 0, len(eas))
	for _, ea := range eas {
		jobs = append(jobs, Job{EA: ea, Symbol: symbol, Period: period, From: from, To: to})
	}
	return jobs
}

// ── File I/O ──────────────────────────────────────────────────────────────────

func LoadBatchFile(path string) (*BatchFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read batch file: %w", err)
	}
	var bf BatchFile
	if err := json.Unmarshal(data, &bf); err != nil {
		return nil, fmt.Errorf("parse batch file: %w", err)
	}
	return &bf, nil
}

func WriteBatchTemplate(path string) error {
	model0 := 0
	_ = model0
	from := time.Now().AddDate(-1, 0, 0).Format("2006.01.02")
	to := time.Now().Format("2006.01.02")

	example := BatchFile{
		Defaults: Defaults{
			Symbol: "EURUSD", Period: "H1",
			From: from, To: to,
			Model: 1, Deposit: 10000, Currency: "USD", Leverage: 100,
			Profile: "default",
		},
		Jobs: []Job{
			// One EA across multiple symbols — symbol overrides default
			{EA: "MACD Sample", Symbol: "EURUSD", Label: "MACD - EUR/USD H1"},
			{EA: "MACD Sample", Symbol: "GBPUSD", Label: "MACD - GBP/USD H1"},
			{EA: "MACD Sample", Symbol: "USDJPY", Label: "MACD - USD/JPY H1"},
			{EA: "MACD Sample", Symbol: "AUDUSD", Label: "MACD - AUD/USD H1"},

			// Same EA across multiple timeframes — period overrides default
			{EA: "Moving Average", Period: "M15", Label: "MA - EUR/USD M15"},
			{EA: "Moving Average", Period: "H4", Label: "MA - EUR/USD H4"},
			{EA: "Moving Average", Period: "D1", Label: "MA - EUR/USD D1"},

			// Custom EA with specific inputs and every-tick model
			{
				EA: "MyScalper", Symbol: "GBPUSD", Period: "M5",
				Model: &model0, Deposit: 50000,
				Label: "MyScalper GBP/USD M5",
				Inputs: map[string]string{
					"StopLoss": "20", "TakeProfit": "40", "MagicNumber": "99001",
				},
			},

			// Different broker terminal via profile
			{EA: "MyEA", Symbol: "XAUUSD", Period: "H1", Profile: "broker2", Label: "MyEA Gold broker2"},

			// Set file drives the inputs; ranges and Y/N flags from the file are preserved.
			{EA: "MyEA", Symbol: "EURUSD", SetFile: "MyEA.set", Label: "MyEA EURUSD via set file"},

			// Same set file, different symbol, with one input overridden.
			// "inputs" wins over set file values — the rest of the set file is still used.
			{
				EA: "MyEA", Symbol: "GBPUSD", SetFile: "MyEA.set",
				Inputs: map[string]string{"MagicNumber": "99002"},
				Label:  "MyEA GBPUSD set file + magic override",
			},
		},
	}
	data, _ := json.MarshalIndent(example, "", "  ")
	return os.WriteFile(path, data, 0644)
}

func ExampleBatchFilePath(workDir string) string {
	return filepath.Join(workDir, "batch.json")
}

// ── Output ────────────────────────────────────────────────────────────────────

func PrintSummaryTable(results []JobResult) {
	fmt.Println("\n╔══════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║  Batch Results                                                       ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════╝")

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "#\tLabel\tNet P&L\tPF\tSharpe\tMaxDD%\tWin%\tTrades\tReturn%\tTime")
	fmt.Fprintln(w, "─\t─────\t───────\t──\t──────\t──────\t────\t──────\t───────\t────")
	for i, r := range results {
		num := fmt.Sprintf("%d", i+1)
		label := truncate(r.Job.displayLabel(), 35)
		if r.Error != nil {
			fmt.Fprintf(w, "%s\t%s\tERROR\t\t\t\t\t\t\t%s\n", num, label, r.Duration.Round(time.Second))
			continue
		}
		if r.Stats == nil {
			fmt.Fprintf(w, "%s\t%s\tno report\t\t\t\t\t\t\t%s\n", num, label, r.Duration.Round(time.Second))
			continue
		}
		s := r.Stats
		fmt.Fprintf(w, "%s\t%s\t%.2f\t%.2f\t%.2f\t%.1f\t%.1f\t%d\t%.1f%%\t%s\n",
			num, label, s.NetProfit, s.ProfitFactor, s.SharpeRatio,
			s.MaxDrawdownPct, s.WinRate, s.TotalTrades, s.ReturnPct,
			r.Duration.Round(time.Second))
	}
	w.Flush()

	passed, failed := 0, 0
	for _, r := range results {
		if r.Error != nil || r.Stats == nil {
			failed++
		} else {
			passed++
		}
	}
	fmt.Printf("\n%d completed, %d failed\n", passed, failed)
	if passed > 1 {
		printBestPerformers(results)
	}
}

func PrintSummaryJSON(results []JobResult) {
	type row struct {
		Label        string  `json:"label"`
		EA           string  `json:"ea"`
		Symbol       string  `json:"symbol"`
		Period       string  `json:"period"`
		NetProfit    float64 `json:"net_profit"`
		ProfitFactor float64 `json:"profit_factor"`
		Sharpe       float64 `json:"sharpe_ratio"`
		MaxDDPct     float64 `json:"max_dd_pct"`
		WinRate      float64 `json:"win_rate_pct"`
		Trades       int     `json:"trades"`
		ReturnPct    float64 `json:"return_pct"`
		DurSec       float64 `json:"duration_sec"`
		Error        string  `json:"error,omitempty"`
	}
	var rows []row
	for _, r := range results {
		if r.Error != nil {
			rows = append(rows, row{Label: r.Job.displayLabel(), EA: r.Job.EA,
				Symbol: r.Job.Symbol, Period: r.Job.Period, Error: r.Error.Error()})
			continue
		}
		if r.Stats != nil {
			s := r.Stats
			rows = append(rows, row{
				Label: r.Job.displayLabel(), EA: r.Job.EA,
				Symbol: r.Job.Symbol, Period: r.Job.Period,
				NetProfit: s.NetProfit, ProfitFactor: s.ProfitFactor,
				Sharpe: s.SharpeRatio, MaxDDPct: s.MaxDrawdownPct,
				WinRate: s.WinRate, Trades: s.TotalTrades, ReturnPct: s.ReturnPct,
				DurSec: r.Duration.Seconds(),
			})
		}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(rows)
}

func PrintSummaryCSV(results []JobResult) {
	report.PrintCSVHeader()
	for _, r := range results {
		if r.Stats != nil {
			report.PrintCSVRow(r.Stats)
		}
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func printJobTable(jobs []Job, defaults Defaults) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "#\tEA\tSymbol\tPeriod\tModel\tFrom → To\tProfile")
	for i, j := range jobs {
		r := j.Resolve(defaults)
		model := 1
		if r.Model != nil {
			model = *r.Model
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%d\t%s → %s\t%s\n",
			i+1, r.EA, r.Symbol, r.Period, model, r.From, r.To, r.Profile)
	}
	w.Flush()
}

func printInlineResult(s *report.Stats) {
	status := "PROFIT"
	if s.NetProfit < 0 {
		status = "LOSS"
	}
	fmt.Printf("       ✓ %-6s  P&L: %+.2f  PF: %.2f  Sharpe: %.2f  MaxDD: %.1f%%  Trades: %d  Win: %.1f%%\n",
		status, s.NetProfit, s.ProfitFactor, s.SharpeRatio,
		s.MaxDrawdownPct, s.TotalTrades, s.WinRate)
}

func printBestPerformers(results []JobResult) {
	type ranked struct {
		label string
		val   float64
	}
	bestPF := ranked{}
	bestReturn := ranked{}
	bestSharpe := ranked{}
	lowestDD := ranked{val: 1e9}

	for _, r := range results {
		if r.Stats == nil {
			continue
		}
		s := r.Stats
		lbl := r.Job.displayLabel()
		if s.ProfitFactor > bestPF.val {
			bestPF = ranked{lbl, s.ProfitFactor}
		}
		if s.ReturnPct > bestReturn.val {
			bestReturn = ranked{lbl, s.ReturnPct}
		}
		if s.SharpeRatio > bestSharpe.val {
			bestSharpe = ranked{lbl, s.SharpeRatio}
		}
		if s.MaxDrawdownPct < lowestDD.val {
			lowestDD = ranked{lbl, s.MaxDrawdownPct}
		}
	}

	fmt.Println("\n── Top performers ───────────────────────────────────────────────────")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if bestPF.label != "" {
		fmt.Fprintf(w, "Best Profit Factor:\t%s\t(%.2f)\n", bestPF.label, bestPF.val)
	}
	if bestReturn.label != "" {
		fmt.Fprintf(w, "Best Return:\t%s\t(%.1f%%)\n", bestReturn.label, bestReturn.val)
	}
	if bestSharpe.label != "" {
		fmt.Fprintf(w, "Best Sharpe:\t%s\t(%.2f)\n", bestSharpe.label, bestSharpe.val)
	}
	if lowestDD.label != "" && lowestDD.val < 1e9 {
		fmt.Fprintf(w, "Lowest Drawdown:\t%s\t(%.1f%%)\n", lowestDD.label, lowestDD.val)
	}
	w.Flush()
	fmt.Println()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
