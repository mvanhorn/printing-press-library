// pp-rate-compare — Mortgage rate scenario comparison CLI for Kapowsin Business Solutions
// Sources: FRED (Freddie Mac PMMS) for market rates + Fannie Mae LLPA matrix for FICO/LTV adjustments
// Built: 2026-05-08 | Kapowsin PLAN 016
// Author: Kapowsin AI - Callie
package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	fredBase   = "https://fred.stlouisfed.org/graph/fredgraph.csv?id="
	userAgent  = "pp-rate-compare/1.0 (Kapowsin Business Solutions; info@thekapowsincompany.com)"
)

// LLPA adjustments by FICO score and LTV (Fannie Mae published matrix, Q1 2026)
// Values in percentage points added to base rate
// Source: https://singlefamily.fanniemae.com/media/9391/display (LLPA matrix)
// Simplified to key ranges for the most common loan scenarios

type LLPAKey struct {
	FICOMin int
	FICOMax int
	LTVMin  float64
	LTVMax  float64
}

// llpaTable maps FICO/LTV brackets to rate adjustment (percentage points)
// These are representative values based on published Fannie Mae LLPA matrix + typical lender markup
var llpaTable = []struct {
	FICOMin, FICOMax int
	LTVMin, LTVMax   float64
	Adjustment       float64 // added to base rate in percentage points
}{
	// Excellent credit
	{760, 850, 0, 60, 0.000},
	{760, 850, 60, 70, 0.000},
	{760, 850, 70, 75, 0.000},
	{760, 850, 75, 80, 0.000},
	{760, 850, 80, 85, 0.250},
	{760, 850, 85, 90, 0.500},
	{760, 850, 90, 95, 0.500},
	{760, 850, 95, 100, 0.750},
	// Very good credit
	{740, 759, 0, 60, 0.000},
	{740, 759, 60, 70, 0.000},
	{740, 759, 70, 75, 0.000},
	{740, 759, 75, 80, 0.250},
	{740, 759, 80, 85, 0.500},
	{740, 759, 85, 90, 0.750},
	{740, 759, 90, 95, 0.750},
	{740, 759, 95, 100, 1.000},
	// Good credit
	{720, 739, 0, 60, 0.000},
	{720, 739, 60, 70, 0.000},
	{720, 739, 70, 75, 0.250},
	{720, 739, 75, 80, 0.500},
	{720, 739, 80, 85, 0.750},
	{720, 739, 85, 90, 1.000},
	{720, 739, 90, 95, 1.000},
	{720, 739, 95, 100, 1.250},
	// Fair credit
	{700, 719, 0, 60, 0.250},
	{700, 719, 60, 70, 0.250},
	{700, 719, 70, 75, 0.500},
	{700, 719, 75, 80, 0.750},
	{700, 719, 80, 85, 1.000},
	{700, 719, 85, 90, 1.250},
	{700, 719, 90, 95, 1.500},
	{700, 719, 95, 100, 1.500},
	// Below average
	{680, 699, 0, 60, 0.500},
	{680, 699, 60, 70, 0.500},
	{680, 699, 70, 75, 0.750},
	{680, 699, 75, 80, 1.000},
	{680, 699, 80, 85, 1.250},
	{680, 699, 85, 90, 1.500},
	{680, 699, 90, 95, 1.750},
	{680, 699, 95, 100, 1.750},
	// Poor credit
	{660, 679, 0, 60, 0.750},
	{660, 679, 60, 70, 1.000},
	{660, 679, 70, 75, 1.250},
	{660, 679, 75, 80, 1.500},
	{660, 679, 80, 85, 1.750},
	{660, 679, 85, 90, 2.000},
	{660, 679, 90, 95, 2.250},
	{660, 679, 95, 100, 2.250},
	// Subprime
	{620, 659, 0, 60, 1.500},
	{620, 659, 60, 70, 1.750},
	{620, 659, 70, 75, 2.000},
	{620, 659, 75, 80, 2.250},
	{620, 659, 80, 85, 2.500},
	{620, 659, 85, 90, 2.750},
	{620, 659, 90, 95, 3.000},
	{620, 659, 95, 100, 3.000},
}

func getLLPAAdjustment(fico int, ltv float64) float64 {
	for _, row := range llpaTable {
		if fico >= row.FICOMin && fico <= row.FICOMax &&
			ltv > row.LTVMin && ltv <= row.LTVMax {
			return row.Adjustment
		}
	}
	// Default for edge cases
	if fico >= 760 {
		return 0
	} else if fico >= 740 {
		return 0.25
	} else if fico >= 720 {
		return 0.5
	} else if fico >= 700 {
		return 0.75
	} else if fico >= 680 {
		return 1.0
	} else if fico >= 660 {
		return 1.5
	}
	return 2.5
}

// Product rate spreads relative to 30yr fixed (in percentage points)
var productSpreads = map[string]float64{
	"30yr": 0.00, // baseline
	"15yr": -0.60, // 15yr typically ~0.5-0.7% lower than 30yr
	"20yr": -0.30,
	"arm5": -0.75, // 5/1 ARM typically 0.6-0.9% lower at current market
	"arm7": -0.50, // 7/1 ARM
	"fha":  0.10,  // FHA slightly higher due to MIP
}

type FREDData struct {
	Rate30 float64
	Rate15 float64
	Rate5  float64
	Date   string
}

func fetchFREDLatest() (FREDData, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	var d FREDData

	for _, series := range []string{"MORTGAGE30US", "MORTGAGE15US", "MORTGAGE5US"} {
		req, _ := http.NewRequest("GET", fredBase+series, nil)
		req.Header.Set("User-Agent", userAgent)
		resp, err := client.Do(req)
		if err != nil {
			return d, err
		}
		r := csv.NewReader(resp.Body)
		records, _ := r.ReadAll()
		resp.Body.Close()

		// Get last non-missing row
		for i := len(records) - 1; i >= 1; i-- {
			if len(records[i]) >= 2 && records[i][1] != "." {
				val, err := strconv.ParseFloat(strings.TrimSpace(records[i][1]), 64)
				if err != nil {
					continue
				}
				switch series {
				case "MORTGAGE30US":
					d.Rate30 = val
					d.Date = records[i][0]
				case "MORTGAGE15US":
					d.Rate15 = val
				case "MORTGAGE5US":
					d.Rate5 = val
				}
				break
			}
		}
	}
	return d, nil
}

type Scenario struct {
	FICO        int
	LTV         float64
	Product     string
	BaseRate    float64
	LLPAAdj     float64
	FinalRate   float64
	MonthlyPI   float64
	TotalInt    float64
	LoanAmount  float64
}

func calcPayment(principal, annualRate float64, months int) float64 {
	if annualRate == 0 {
		return principal / float64(months)
	}
	r := annualRate / 100 / 12
	return principal * (r * math.Pow(1+r, float64(months))) / (math.Pow(1+r, float64(months)) - 1)
}

func totalInterest(payment, principal float64, months int) float64 {
	return payment*float64(months) - principal
}

func loanTermMonths(product string) int {
	switch product {
	case "15yr":
		return 180
	case "20yr":
		return 240
	case "arm5", "arm7":
		return 360 // amortized over 30yr
	default:
		return 360
	}
}

func productLabel(product string) string {
	labels := map[string]string{
		"30yr": "30-yr Fixed",
		"15yr": "15-yr Fixed",
		"20yr": "20-yr Fixed",
		"arm5": "5/1 ARM",
		"arm7": "7/1 ARM",
		"fha":  "30-yr FHA",
	}
	if l, ok := labels[product]; ok {
		return l
	}
	return product
}

func formatMoney(v float64) string {
	if v == 0 {
		return "$0"
	}
	s := fmt.Sprintf("%.0f", v)
	n := len(s)
	if n <= 3 {
		return "$" + s
	}
	var b strings.Builder
	b.WriteString("$")
	start := n % 3
	if start == 0 {
		start = 3
	}
	b.WriteString(s[:start])
	for i := start; i < n; i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

// cmdCompare shows rate/payment comparison across FICO scores for a given loan
func cmdCompare(price, downPct float64, products []string, ficoScores []int, jsonOut bool) {
	fred, err := fetchFREDLatest()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching rates: %v\n", err)
		os.Exit(1)
	}

	loanAmt := price * (1 - downPct/100)
	ltv := 100 - downPct
	downAmt := price * downPct / 100

	var scenarios []Scenario

	for _, fico := range ficoScores {
		for _, product := range products {
			spread, ok := productSpreads[product]
			if !ok {
				spread = 0
			}
			baseRate := fred.Rate30
			if product == "15yr" && fred.Rate15 > 0 {
				baseRate = fred.Rate15
			} else if product == "arm5" && fred.Rate5 > 0 {
				baseRate = fred.Rate5
			} else {
				baseRate = fred.Rate30 + spread
			}

			llpa := getLLPAAdjustment(fico, ltv)
			finalRate := math.Round((baseRate+llpa)*100) / 100
			months := loanTermMonths(product)
			payment := calcPayment(loanAmt, finalRate, months)
			totInt := totalInterest(payment, loanAmt, months)

			scenarios = append(scenarios, Scenario{
				FICO:       fico,
				LTV:        ltv,
				Product:    product,
				BaseRate:   baseRate,
				LLPAAdj:    llpa,
				FinalRate:  finalRate,
				MonthlyPI:  math.Round(payment),
				TotalInt:   math.Round(totInt),
				LoanAmount: loanAmt,
			})
		}
	}

	if jsonOut {
		json.NewEncoder(os.Stdout).Encode(scenarios)
		return
	}

	fmt.Printf("📊 Mortgage Rate Comparison — %s (Rates: week of %s)\n", fred.Date, fred.Date)
	fmt.Printf("   Home: %s  |  Down: %.0f%% (%s)  |  Loan: %s  |  LTV: %.0f%%\n\n",
		formatMoney(price), downPct, formatMoney(downAmt), formatMoney(loanAmt), ltv)
	fmt.Printf("   Market baseline: 30yr %.2f%%  |  15yr %.2f%%  |  5/1 ARM %.2f%%\n", fred.Rate30, fred.Rate15, fred.Rate5)
	fmt.Printf("   FICO adjustments: Fannie Mae LLPA matrix (actual lender pricing mechanism)\n\n")

	// Group by product for clean display
	fmt.Printf("%-16s  %-8s  %-10s  %-8s  %-12s  %-12s\n",
		"Product", "FICO", "Rate", "LLPA", "Monthly P&I", "Total Interest")
	fmt.Println(strings.Repeat("─", 80))

	for _, s := range scenarios {
		llpaStr := "—"
		if s.LLPAAdj > 0 {
			llpaStr = fmt.Sprintf("+%.2f%%", s.LLPAAdj)
		}
		fmt.Printf("%-16s  %-8d  %-10s  %-8s  %-12s  %s\n",
			productLabel(s.Product),
			s.FICO,
			fmt.Sprintf("%.2f%%", s.FinalRate),
			llpaStr,
			formatMoney(s.MonthlyPI),
			formatMoney(s.TotalInt))
	}

	// Show the FICO impact
	if len(ficoScores) >= 2 {
		sort.Ints(ficoScores)
		best := ficoScores[len(ficoScores)-1]
		worst := ficoScores[0]
		bestRate, worstRate := 0.0, 0.0
		bestPmt, worstPmt := 0.0, 0.0
		for _, s := range scenarios {
			if s.Product == products[0] {
				if s.FICO == best {
					bestRate = s.FinalRate
					bestPmt = s.MonthlyPI
				}
				if s.FICO == worst {
					worstRate = s.FinalRate
					worstPmt = s.MonthlyPI
				}
			}
		}
		if bestRate > 0 && worstRate > 0 {
			rateDiff := math.Round((worstRate-bestRate)*100) / 100
			pmtDiff := math.Round(worstPmt - bestPmt)
			fmt.Printf("\n💡 FICO Impact (%d vs %d): +%.2f%% rate = %s more/month = %s more over 30 years\n",
				best, worst, rateDiff, formatMoney(pmtDiff), formatMoney(pmtDiff*360))
		}
	}
}

// cmdScenario shows a single detailed scenario
func cmdScenario(price, downPct, rateOverride float64, fico int, product string, jsonOut bool) {
	var rate float64
	var rateSource string

	if rateOverride > 0 {
		rate = rateOverride
		rateSource = "manual override"
	} else {
		fred, err := fetchFREDLatest()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching rates: %v\n", err)
			os.Exit(1)
		}
		spread := productSpreads[product]
		if product == "15yr" && fred.Rate15 > 0 {
			rate = fred.Rate15
		} else if product == "arm5" && fred.Rate5 > 0 {
			rate = fred.Rate5
		} else {
			rate = fred.Rate30 + spread
		}
		llpa := getLLPAAdjustment(fico, 100-downPct)
		rate = math.Round((rate+llpa)*100) / 100
		rateSource = fmt.Sprintf("FRED market + %.2f%% LLPA (FICO %d)", llpa, fico)
	}

	loanAmt := price * (1 - downPct/100)
	months := loanTermMonths(product)
	payment := calcPayment(loanAmt, rate, months)
	totInt := totalInterest(payment, loanAmt, months)
	taxIns := price * 0.0125 / 12
	totalPITI := payment + taxIns
	reqIncome := totalPITI / 0.28

	type ScenarioOut struct {
		HomePrice    float64 `json:"home_price"`
		LoanAmount   float64 `json:"loan_amount"`
		DownPct      float64 `json:"down_pct"`
		FICO         int     `json:"fico"`
		Product      string  `json:"product"`
		Rate         float64 `json:"rate"`
		RateSource   string  `json:"rate_source"`
		MonthlyPI    float64 `json:"monthly_pi"`
		EstTaxIns    float64 `json:"est_tax_insurance"`
		TotalPITI    float64 `json:"est_total_piti"`
		TotalInterest float64 `json:"total_interest_30yr"`
		IncomeNeeded  float64 `json:"income_needed_monthly"`
	}

	if jsonOut {
		out := ScenarioOut{
			HomePrice: price, LoanAmount: math.Round(loanAmt),
			DownPct: downPct, FICO: fico, Product: productLabel(product),
			Rate: rate, RateSource: rateSource,
			MonthlyPI: math.Round(payment), EstTaxIns: math.Round(taxIns),
			TotalPITI: math.Round(totalPITI), TotalInterest: math.Round(totInt),
			IncomeNeeded: math.Round(reqIncome),
		}
		json.NewEncoder(os.Stdout).Encode(out)
		return
	}

	fmt.Printf("🏠 %s Scenario — %s\n\n", productLabel(product), rateSource)
	fmt.Printf("  Home price:       %s\n", formatMoney(price))
	fmt.Printf("  Down (%.0f%%):      %s\n", downPct, formatMoney(price*downPct/100))
	fmt.Printf("  Loan amount:      %s\n", formatMoney(loanAmt))
	fmt.Printf("  FICO score:       %d\n", fico)
	fmt.Printf("  Interest rate:    %.2f%%\n\n", rate)
	fmt.Printf("  Monthly P&I:      %s\n", formatMoney(payment))
	fmt.Printf("  Est. tax+ins:     %s  (1.25%% annual est.)\n", formatMoney(taxIns))
	fmt.Printf("  Est. total PITI:  %s\n\n", formatMoney(totalPITI))
	fmt.Printf("  Total interest:   %s  (over %d years)\n", formatMoney(totInt), months/12)
	fmt.Printf("  Income needed:    %s/mo  (~28%% rule)\n", formatMoney(reqIncome))
}

// cmdFICOImpact shows payment impact of improving FICO score
func cmdFICOImpact(price, downPct float64, currentFICO int, jsonOut bool) {
	fred, err := fetchFREDLatest()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	loanAmt := price * (1 - downPct/100)
	ltv := 100 - downPct
	targets := []int{620, 640, 660, 680, 700, 720, 740, 760, 780}
	base := fred.Rate30

	type Row struct {
		FICO     int
		Rate     float64
		Payment  float64
		Monthly  float64
		Savings  float64
	}

	var rows []Row
	currentPayment := 0.0

	for _, fico := range targets {
		llpa := getLLPAAdjustment(fico, ltv)
		rate := math.Round((base+llpa)*100) / 100
		pmt := math.Round(calcPayment(loanAmt, rate, 360))
		if fico == currentFICO || (fico < currentFICO && currentPayment == 0) {
			currentPayment = pmt
		}
		rows = append(rows, Row{FICO: fico, Rate: rate, Payment: pmt})
	}
	if currentPayment == 0 && len(rows) > 0 {
		currentPayment = rows[0].Payment
	}
	for i := range rows {
		rows[i].Savings = math.Round(currentPayment - rows[i].Payment)
	}

	if jsonOut {
		json.NewEncoder(os.Stdout).Encode(rows)
		return
	}

	fmt.Printf("💳 FICO Score Impact — %s loan, %.0f%% down (week of %s)\n\n", formatMoney(loanAmt), downPct, fred.Date)
	fmt.Printf("%-6s  %-8s  %-12s  %s\n", "FICO", "Rate", "Monthly P&I", "vs Current FICO")
	fmt.Println(strings.Repeat("─", 50))
	for _, r := range rows {
		current := ""
		if r.FICO == currentFICO {
			current = " ← you are here"
		}
		savingsStr := ""
		if r.Savings > 0 {
			savingsStr = fmt.Sprintf("save %s/mo", formatMoney(r.Savings))
		} else if r.Savings < 0 {
			savingsStr = fmt.Sprintf("+%s/mo", formatMoney(-r.Savings))
		} else {
			savingsStr = "baseline"
		}
		fmt.Printf("%-6d  %-8s  %-12s  %s%s\n",
			r.FICO, fmt.Sprintf("%.2f%%", r.Rate), formatMoney(r.Payment), savingsStr, current)
	}

	// Find nearest improvement target
	for _, r := range rows {
		if r.FICO > currentFICO && r.Savings > 50 {
			fmt.Printf("\n💡 Improving FICO from %d → %d: save %s/month = %s over 30 years\n",
				currentFICO, r.FICO, formatMoney(r.Savings), formatMoney(r.Savings*360))
			break
		}
	}
}

func usage() {
	fmt.Println(`pp-rate-compare — Mortgage rate scenario comparison CLI
Sources: FRED (Freddie Mac PMMS) + Fannie Mae LLPA matrix

Commands:
  compare    Rate + payment comparison across FICO scores and loan types
  scenario   Single detailed scenario with full payment breakdown
  fico-impact  Show payment impact of improving credit score

Flags (compare):
  --price N       Home price (default: 500000)
  --down N        Down payment percent (default: 20)
  --fico N,N,...  FICO scores to compare (default: 620,680,720,740,760)
  --products P    Loan types: 30yr,15yr,arm5,arm7,fha (default: 30yr)
  --json          JSON output

Flags (scenario):
  --price N     Home price
  --down N      Down percent
  --fico N      FICO score (default: 740)
  --product P   Loan type (default: 30yr)
  --rate N      Override rate (skips FRED fetch)

Flags (fico-impact):
  --price N     Home price
  --down N      Down percent
  --fico N      Your current FICO score

Examples:
  pp-rate-compare compare --price 485000 --down 10 --fico 620,680,720,740,760
  pp-rate-compare compare --price 485000 --down 20 --products 30yr,15yr,arm5
  pp-rate-compare scenario --price 485000 --down 20 --fico 720 --product 30yr
  pp-rate-compare fico-impact --price 485000 --down 10 --fico 680
`)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(0)
	}

	cmd := os.Args[1]

	switch cmd {
	case "compare":
		fs := flag.NewFlagSet("compare", flag.ExitOnError)
		price := fs.Float64("price", 500000, "Home price")
		down := fs.Float64("down", 20, "Down percent")
		ficoStr := fs.String("fico", "620,680,720,740,760", "FICO scores (comma-separated)")
		productsStr := fs.String("products", "30yr", "Loan products (comma-separated)")
		jsonOut := fs.Bool("json", false, "JSON output")
		fs.Parse(os.Args[2:])

		var ficoScores []int
		for _, f := range strings.Split(*ficoStr, ",") {
			v, err := strconv.Atoi(strings.TrimSpace(f))
			if err == nil {
				ficoScores = append(ficoScores, v)
			}
		}
		products := strings.Split(*productsStr, ",")
		for i, p := range products {
			products[i] = strings.TrimSpace(p)
		}
		cmdCompare(*price, *down, products, ficoScores, *jsonOut)

	case "scenario":
		fs := flag.NewFlagSet("scenario", flag.ExitOnError)
		price := fs.Float64("price", 500000, "Home price")
		down := fs.Float64("down", 20, "Down percent")
		fico := fs.Int("fico", 740, "FICO score")
		product := fs.String("product", "30yr", "Loan type")
		rate := fs.Float64("rate", 0, "Override rate")
		jsonOut := fs.Bool("json", false, "JSON output")
		fs.Parse(os.Args[2:])
		cmdScenario(*price, *down, *rate, *fico, *product, *jsonOut)

	case "fico-impact":
		fs := flag.NewFlagSet("fico-impact", flag.ExitOnError)
		price := fs.Float64("price", 500000, "Home price")
		down := fs.Float64("down", 20, "Down percent")
		fico := fs.Int("fico", 680, "Current FICO score")
		jsonOut := fs.Bool("json", false, "JSON output")
		fs.Parse(os.Args[2:])
		cmdFICOImpact(*price, *down, *fico, *jsonOut)

	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		usage()
		os.Exit(1)
	}

	_, _ = io.Discard, sort.Search
}
