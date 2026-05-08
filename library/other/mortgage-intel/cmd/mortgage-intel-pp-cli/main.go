// pp-mortgage-intel — Mortgage rate intelligence CLI for Kapowsin Business Solutions
// Sources: FRED (Federal Reserve Bank of St. Louis) — free, no API key required
// Built: 2026-05-08 | Phase 3 Part 2 of PLAN 016
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
	fredBase    = "https://fred.stlouisfed.org/graph/fredgraph.csv?id="
	series30yr  = "MORTGAGE30US"
	series15yr  = "MORTGAGE15US"
	series5arm  = "MORTGAGE5US"
	userAgent   = "pp-mortgage-intel/1.0 (Kapowsin Business Solutions; info@thekapowsincompany.com)"
)

type RatePoint struct {
	Date string  `json:"date"`
	Rate float64 `json:"rate"`
}

type RateSeries struct {
	Name   string      `json:"name"`
	Points []RatePoint `json:"points"`
}

func fetchFRED(seriesID string, weeks int) ([]RatePoint, error) {
	url := fredBase + seriesID
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("FRED fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("FRED returned HTTP %d", resp.StatusCode)
	}

	r := csv.NewReader(resp.Body)
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("CSV parse failed: %w", err)
	}

	var points []RatePoint
	for _, rec := range records[1:] { // skip header
		if len(rec) < 2 || rec[1] == "." {
			continue // FRED uses "." for missing data
		}
		rate, err := strconv.ParseFloat(strings.TrimSpace(rec[1]), 64)
		if err != nil {
			continue
		}
		points = append(points, RatePoint{Date: rec[0], Rate: rate})
	}

	// Sort descending (most recent first) and trim to weeks
	sort.Slice(points, func(i, j int) bool { return points[i].Date > points[j].Date })
	if weeks > 0 && len(points) > weeks {
		points = points[:weeks]
	}
	return points, nil
}

func monthlyPayment(price, downPct, annualRate float64) float64 {
	loan := price * (1 - downPct/100)
	if annualRate == 0 {
		return loan / 360
	}
	r := annualRate / 100 / 12
	n := 360.0 // 30-year
	return loan * (r * math.Pow(1+r, n)) / (math.Pow(1+r, n) - 1)
}

func cmdCurrent(jsonOut bool) {
	type CurrentRates struct {
		AsOf    string  `json:"as_of"`
		Rate30  float64 `json:"rate_30yr_fixed"`
		Rate15  float64 `json:"rate_15yr_fixed"`
		Rate5   float64 `json:"rate_5_1_arm"`
		Chg30   float64 `json:"weekly_change_30yr"`
		Chg15   float64 `json:"weekly_change_15yr"`
		Source  string  `json:"source"`
	}

	pts30, err := fetchFRED(series30yr, 2)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	pts15, _ := fetchFRED(series15yr, 2)
	pts5, _ := fetchFRED(series5arm, 2)

	cur30, chg30 := pts30[0].Rate, 0.0
	if len(pts30) >= 2 {
		chg30 = pts30[0].Rate - pts30[1].Rate
	}
	cur15, chg15 := 0.0, 0.0
	if len(pts15) > 0 {
		cur15 = pts15[0].Rate
		if len(pts15) >= 2 {
			chg15 = pts15[0].Rate - pts15[1].Rate
		}
	}
	cur5 := 0.0
	if len(pts5) > 0 {
		cur5 = pts5[0].Rate
	}

	if jsonOut {
		out := CurrentRates{
			AsOf: pts30[0].Date, Rate30: cur30, Rate15: cur15, Rate5: cur5,
			Chg30: math.Round(chg30*100) / 100,
			Chg15: math.Round(chg15*100) / 100,
			Source: "FRED / Freddie Mac PMMS",
		}
		json.NewEncoder(os.Stdout).Encode(out)
		return
	}

	chg30Str := fmt.Sprintf("%.2f", chg30)
	if chg30 > 0 {
		chg30Str = "▲ +" + chg30Str
	} else if chg30 < 0 {
		chg30Str = "▼ " + chg30Str
	} else {
		chg30Str = "→ " + chg30Str
	}

	fmt.Printf("📊 Mortgage Rates — Week of %s\n", pts30[0].Date)
	fmt.Printf("Source: Freddie Mac PMMS via FRED (free, no API key)\n\n")
	fmt.Printf("• 30-yr Fixed: %.2f%%  %s pts vs last week\n", cur30, chg30Str)
	if cur15 > 0 {
		chg15Str := fmt.Sprintf("%.2f", chg15)
		if chg15 > 0 {
			chg15Str = "▲ +" + chg15Str
		} else if chg15 < 0 {
			chg15Str = "▼ " + chg15Str
		} else {
			chg15Str = "→ " + chg15Str
		}
		fmt.Printf("• 15-yr Fixed: %.2f%%  %s pts vs last week\n", cur15, chg15Str)
	}
	if cur5 > 0 {
		fmt.Printf("• 5/1 ARM:     %.2f%%\n", cur5)
	}
}

func cmdHistory(weeks int, jsonOut bool) {
	pts30, err := fetchFRED(series30yr, weeks)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	pts15, _ := fetchFRED(series15yr, weeks)

	// Build a map for 15yr by date
	map15 := make(map[string]float64)
	for _, p := range pts15 {
		map15[p.Date] = p.Rate
	}

	if jsonOut {
		type HistRow struct {
			Date    string  `json:"date"`
			Rate30  float64 `json:"rate_30yr"`
			Rate15  float64 `json:"rate_15yr,omitempty"`
		}
		var rows []HistRow
		for _, p := range pts30 {
			rows = append(rows, HistRow{Date: p.Date, Rate30: p.Rate, Rate15: map15[p.Date]})
		}
		json.NewEncoder(os.Stdout).Encode(rows)
		return
	}

	fmt.Printf("📈 Mortgage Rate History — Last %d Weeks (30-yr Fixed)\n\n", weeks)
	for _, p := range pts30 {
		r15 := map15[p.Date]
		if r15 > 0 {
			fmt.Printf("  %s   30yr: %.2f%%   15yr: %.2f%%\n", p.Date, p.Rate, r15)
		} else {
			fmt.Printf("  %s   30yr: %.2f%%\n", p.Date, p.Rate)
		}
	}
}

func cmdTrend(jsonOut bool) {
	pts, err := fetchFRED(series30yr, 8)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if len(pts) < 4 {
		fmt.Println("Not enough data for trend analysis")
		return
	}

	latest := pts[0].Rate
	fourWkAgo := pts[3].Rate
	eightWkAgo := 0.0
	if len(pts) >= 8 {
		eightWkAgo = pts[7].Rate
	}

	chg4wk := latest - fourWkAgo
	direction := "→ FLAT"
	if chg4wk > 0.1 {
		direction = "▲ RISING"
	} else if chg4wk < -0.1 {
		direction = "▼ FALLING"
	}

	if jsonOut {
		type TrendOut struct {
			Direction     string  `json:"direction"`
			CurrentRate   float64 `json:"current_rate_30yr"`
			Change4Weeks  float64 `json:"change_4_weeks"`
			Change8Weeks  float64 `json:"change_8_weeks,omitempty"`
		}
		chg8 := 0.0
		if eightWkAgo > 0 {
			chg8 = math.Round((latest-eightWkAgo)*100) / 100
		}
		out := TrendOut{
			Direction: strings.TrimSpace(direction),
			CurrentRate: latest,
			Change4Weeks: math.Round(chg4wk*100) / 100,
			Change8Weeks: chg8,
		}
		json.NewEncoder(os.Stdout).Encode(out)
		return
	}

	fmt.Printf("📉 30-Year Rate Trend\n\n")
	fmt.Printf("  %s  (%.2f pts over 4 weeks)\n", direction, chg4wk)
	fmt.Printf("  Current:   %.2f%%  (%s)\n", latest, pts[0].Date)
	fmt.Printf("  4 wks ago: %.2f%%  (%s)\n", fourWkAgo, pts[3].Date)
	if eightWkAgo > 0 {
		fmt.Printf("  8 wks ago: %.2f%%  (%s)\n", eightWkAgo, pts[7].Date)
	}
	fmt.Printf("\n")
	if chg4wk > 0.25 {
		fmt.Println("  ⚠️  Rates climbing fast — lock-in urgency high")
	} else if chg4wk < -0.25 {
		fmt.Println("  💡 Rates dropping — watch for lock/float decision")
	} else {
		fmt.Println("  Rates stable — standard lock/float considerations apply")
	}
}

func cmdAfford(price, downPct, rate float64, jsonOut bool) {
	if rate == 0 {
		// Fetch current 30yr rate
		pts, err := fetchFRED(series30yr, 1)
		if err != nil || len(pts) == 0 {
			rate = 7.0 // fallback
		} else {
			rate = pts[0].Rate
		}
	}

	payment := monthlyPayment(price, downPct, rate)
	loanAmt := price * (1 - downPct/100)
	downAmt := price * downPct / 100

	// Estimate taxes + insurance (~1.25% of home value annually)
	taxIns := price * 0.0125 / 12
	totalPITI := payment + taxIns

	if jsonOut {
		type AffordOut struct {
			HomePrice     float64 `json:"home_price"`
			DownPct       float64 `json:"down_pct"`
			DownAmount    float64 `json:"down_amount"`
			LoanAmount    float64 `json:"loan_amount"`
			Rate          float64 `json:"rate_30yr"`
			PrincipalInt  float64 `json:"monthly_pi"`
			TaxIns        float64 `json:"est_tax_insurance"`
			TotalPITI     float64 `json:"est_total_piti"`
		}
		out := AffordOut{
			HomePrice: price, DownPct: downPct, DownAmount: math.Round(downAmt),
			LoanAmount: math.Round(loanAmt), Rate: rate,
			PrincipalInt: math.Round(payment), TaxIns: math.Round(taxIns),
			TotalPITI: math.Round(totalPITI),
		}
		json.NewEncoder(os.Stdout).Encode(out)
		return
	}

	fmt.Printf("🏠 Affordability Estimate\n\n")
	fmt.Printf("  Home price:         $%s\n", formatMoney(price))
	fmt.Printf("  Down (%.0f%%):         $%s\n", downPct, formatMoney(downAmt))
	fmt.Printf("  Loan amount:        $%s\n", formatMoney(loanAmt))
	fmt.Printf("  Rate (30yr):        %.2f%%\n\n", rate)
	fmt.Printf("  Monthly P&I:        $%s\n", formatMoney(payment))
	fmt.Printf("  Est. tax+ins:       $%s\n", formatMoney(taxIns))
	fmt.Printf("  Est. total (PITI):  $%s\n\n", formatMoney(totalPITI))

	// Rule of thumb: PITI should be <28% of gross monthly income
	reqIncome := totalPITI / 0.28
	fmt.Printf("  Income needed (~28%% rule): $%s/mo  ($%s/yr)\n",
		formatMoney(reqIncome), formatMoney(reqIncome*12))
}

func formatMoney(v float64) string {
	s := fmt.Sprintf("%.0f", v)
	// Insert commas
	n := len(s)
	if n <= 3 {
		return s
	}
	var b strings.Builder
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

func usage() {
	fmt.Println(`pp-mortgage-intel — Mortgage Rate Intelligence CLI
Source: FRED (Federal Reserve / Freddie Mac PMMS) — free, no API key

Commands:
  current              Latest 30yr, 15yr, 5/1 ARM rates with weekly change
  history [--weeks N]  Rate history (default: 12 weeks)
  trend                4-week trend: RISING / FALLING / FLAT
  afford               Monthly payment calculator
    --price N          Home price (default: 500000)
    --down N           Down payment percent (default: 20)
    --rate N           Override rate (default: current 30yr)

Flags:
  --json               Output structured JSON

Examples:
  pp-mortgage-intel current
  pp-mortgage-intel history --weeks 24
  pp-mortgage-intel trend --json
  pp-mortgage-intel afford --price 485000 --down 10
`)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(0)
	}

	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "JSON output")
	weeks := fs.Int("weeks", 12, "Weeks of history")
	price := fs.Float64("price", 500000, "Home price")
	down := fs.Float64("down", 20, "Down payment percent")
	rate := fs.Float64("rate", 0, "Override rate (default: current 30yr)")

	switch cmd {
	case "current":
		fs.Parse(os.Args[2:])
		cmdCurrent(*jsonOut)
	case "history":
		fs.Parse(os.Args[2:])
		cmdHistory(*weeks, *jsonOut)
	case "trend":
		fs.Parse(os.Args[2:])
		cmdTrend(*jsonOut)
	case "afford":
		fs.Parse(os.Args[2:])
		cmdAfford(*price, *down, *rate, *jsonOut)
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		usage()
		os.Exit(1)
	}

	_ = io.Discard // satisfy import
}
