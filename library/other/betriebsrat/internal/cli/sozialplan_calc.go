package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

type sozialplanInput struct {
	MonthlySalary float64
	YearsService  float64
	Age           int
	Disabled      bool
	Children      int
	Factor        float64
}

type sozialplanResult struct {
	Input       sozialplanInput `json:"input"`
	FormulaUsed string          `json:"formula_used"`
	BaseAmount  float64         `json:"base_amount_eur"`
	Adjustments []string        `json:"adjustments,omitempty"`
	FinalAmount float64         `json:"final_amount_eur"`
	MaxCap      float64         `json:"max_cap_eur,omitempty"`
	Note        string          `json:"note"`
	LegalBasis  string          `json:"legal_basis"`
}

type sozialplanBatchRow struct {
	Name   string           `json:"name"`
	Result sozialplanResult `json:"result"`
}

func calcSozialplan(input sozialplanInput, maxCap float64) sozialplanResult {
	base := math.Round(input.YearsService*input.MonthlySalary*input.Factor*100) / 100
	var adjustments []string
	adjusted := base

	if input.Disabled {
		bonus := math.Round(base*0.25*100) / 100
		adjusted += bonus
		adjustments = append(adjustments, fmt.Sprintf("+25%% Schwerbehinderung (GdB ≥ 50): +%.2f €", bonus))
	}
	if input.Children > 0 {
		cap := input.Children
		if cap > 3 {
			cap = 3
		}
		bonus := math.Round(base*float64(cap)*0.10*100) / 100
		adjusted += bonus
		adjustments = append(adjustments, fmt.Sprintf("+10%% je unterhaltspflichtiges Kind (%d, max. 3): +%.2f €", cap, bonus))
	}
	if input.Age >= 55 {
		bonus := math.Round(base*0.05*100) / 100
		adjusted += bonus
		adjustments = append(adjustments, fmt.Sprintf("+5%% Altersgruppe ≥55 Jahre: +%.2f €", bonus))
	}

	adjusted = math.Round(adjusted*100) / 100
	cappedAt := 0.0
	if maxCap > 0 && adjusted > maxCap {
		cappedAt = maxCap
		adjusted = maxCap
	}

	r := sozialplanResult{
		Input:       input,
		FormulaUsed: fmt.Sprintf("%.2f Jahre × %.2f € × Faktor %.2f", input.YearsService, input.MonthlySalary, input.Factor),
		BaseAmount:  base,
		Adjustments: adjustments,
		FinalAmount: adjusted,
		LegalBasis:  "§ 112 BetrVG; Münchner Formel (BAG-Rechtsprechung)",
		Note: "Schätzung. Tatsächliche Sozialplanabfindung hängt von den Verhandlungsergebnissen ab. " +
			"Der Sozialplan ist erzwingbar (§ 112 Abs. 4 BetrVG) — bei Scheitern entscheidet die Einigungsstelle.",
	}
	if cappedAt > 0 {
		r.MaxCap = cappedAt
		r.Note += fmt.Sprintf(" Kappungsgrenze: %.2f €.", cappedAt)
	}
	return r
}

func newSozialplanCalcCmd(flags *rootFlags) *cobra.Command {
	var salary float64
	var years float64
	var age int
	var disabled bool
	var children int
	var factor float64
	var maxCap float64
	var csvFile string

	cmd := &cobra.Command{
		Use:   "sozialplan-calc",
		Short: "Calculate Sozialplan entitlement using the Munich formula",
		Long: `Estimates the individual Sozialplan entitlement (Abfindung) for an employee
affected by a Betriebsänderung (§§ 111–112 BetrVG).

Uses the standard Munich formula:
  Abfindung = Betriebszugehörigkeit × Monatsgehalt × Faktor

Common factors:
  0.5  — basic minimum (BAG recommended floor)
  0.75 — standard in many Sozialplänen
  1.0  — typical for restructurings with good negotiating position
  1.5  — above average, often in large company Sozialplänen

Adjustments applied (based on BAG-Rechtsprechung and common BV practice):
  +25% if employee is severely disabled (GdB ≥ 50)
  +10% per dependent child (up to 3 children)
  +5%  if employee is ≥55 years old (near retirement)

Batch mode (--csv): CSV file with one employee per line:
  name,salary,years,age,disabled(true/false),children[,factor[,max_cap]]
  Lines starting with # are treated as comments and ignored.
  --factor and --max-cap flags set defaults for rows that omit those columns.`,
		Example: strings.Trim(`
  betriebsrat sozialplan-calc --salary 4500 --years 8 --age 42 --factor 0.75
  betriebsrat sozialplan-calc --salary 6000 --years 15 --age 58 --disabled --children 2 --factor 1.0
  betriebsrat sozialplan-calc --salary 3800 --years 3 --age 35 --max-cap 50000 --agent
  betriebsrat sozialplan-calc --csv employees.csv --factor 0.75 --max-cap 80000 --agent`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if factor <= 0 {
				factor = 0.5
			}

			// Batch CSV mode
			if csvFile != "" {
				return runSozialplanCSV(cmd, flags, csvFile, factor, maxCap)
			}

			// Single employee mode
			if salary <= 0 {
				return fmt.Errorf("--salary muss größer als 0 sein")
			}
			if years < 0 {
				return fmt.Errorf("--years darf nicht negativ sein")
			}

			r := calcSozialplan(sozialplanInput{
				MonthlySalary: salary,
				YearsService:  years,
				Age:           age,
				Disabled:      disabled,
				Children:      children,
				Factor:        factor,
			}, maxCap)

			if flags.asJSON || flags.agent {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(r)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Sozialplan-Abfindungsrechner (Münchner Formel)\n")
			fmt.Fprintf(w, "══════════════════════════════════════════════\n\n")
			fmt.Fprintf(w, "Eingaben:\n")
			fmt.Fprintf(w, "  Monatsgehalt:          %.2f €\n", salary)
			fmt.Fprintf(w, "  Betriebszugehörigkeit: %.1f Jahre\n", years)
			fmt.Fprintf(w, "  Alter:                 %d Jahre\n", age)
			fmt.Fprintf(w, "  Faktor:                %.2f\n\n", factor)
			fmt.Fprintf(w, "Berechnung:\n")
			fmt.Fprintf(w, "  Grundbetrag: %s = %.2f €\n", r.FormulaUsed, r.BaseAmount)
			if len(r.Adjustments) > 0 {
				fmt.Fprintln(w, "\n  Zuschläge:")
				for _, a := range r.Adjustments {
					fmt.Fprintf(w, "    %s\n", a)
				}
			}
			if r.MaxCap > 0 {
				fmt.Fprintf(w, "\n  Kappungsgrenze angewendet: %.2f €\n", r.MaxCap)
			}
			fmt.Fprintf(w, "\n  ► Abfindung (geschätzt): %.2f €\n", r.FinalAmount)
			fmt.Fprintf(w, "\n  Rechtsgrundlage: %s\n", r.LegalBasis)
			fmt.Fprintf(w, "  Hinweis: %s\n", r.Note)
			return nil
		},
	}

	cmd.Flags().Float64Var(&salary, "salary", 0, "Bruttomonatsgehalt in Euro")
	cmd.Flags().Float64Var(&years, "years", 0, "Betriebszugehörigkeit in Jahren (Dezimalzahl erlaubt, z.B. 8.5)")
	cmd.Flags().IntVar(&age, "age", 0, "Alter des Arbeitnehmers")
	cmd.Flags().BoolVar(&disabled, "disabled", false, "Schwerbehindert (GdB ≥ 50) — Zuschlag +25%%")
	cmd.Flags().IntVar(&children, "children", 0, "Unterhaltspflichtige Kinder — Zuschlag +10%% je Kind (max. 3)")
	cmd.Flags().Float64Var(&factor, "factor", 0.5, "Sozialplanfaktor (Standard: 0.5; üblich: 0.5–1.5)")
	cmd.Flags().Float64Var(&maxCap, "max-cap", 0, "Kappungsgrenze in Euro (0 = kein Cap)")
	cmd.Flags().StringVar(&csvFile, "csv", "", "CSV-Datei mit Mitarbeiterdaten für Batch-Berechnung")

	return cmd
}

func runSozialplanCSV(cmd *cobra.Command, flags *rootFlags, csvFile string, defaultFactor, defaultMaxCap float64) error {
	f, err := os.Open(csvFile)
	if err != nil {
		return fmt.Errorf("CSV-Datei konnte nicht geöffnet werden: %w", err)
	}
	defer f.Close()

	var rows []sozialplanBatchRow
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 6 {
			return fmt.Errorf("Zeile %d: mindestens 6 Spalten erforderlich (name,salary,years,age,disabled,children): %q", lineNum, line)
		}
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}

		sal, err := strconv.ParseFloat(parts[1], 64)
		if err != nil || sal <= 0 {
			return fmt.Errorf("Zeile %d: ungültiges Gehalt %q", lineNum, parts[1])
		}
		yrs, err := strconv.ParseFloat(parts[2], 64)
		if err != nil || yrs < 0 {
			return fmt.Errorf("Zeile %d: ungültige Betriebszugehörigkeit %q", lineNum, parts[2])
		}
		ag, err := strconv.Atoi(parts[3])
		if err != nil {
			return fmt.Errorf("Zeile %d: ungültiges Alter %q", lineNum, parts[3])
		}
		dis := strings.EqualFold(parts[4], "true") || parts[4] == "1"
		ch, err := strconv.Atoi(parts[5])
		if err != nil {
			return fmt.Errorf("Zeile %d: ungültige Kinderanzahl %q", lineNum, parts[5])
		}

		rowFactor := defaultFactor
		if len(parts) >= 7 && parts[6] != "" {
			rowFactor, _ = strconv.ParseFloat(parts[6], 64)
			if rowFactor <= 0 {
				rowFactor = defaultFactor
			}
		}
		rowMaxCap := defaultMaxCap
		if len(parts) >= 8 && parts[7] != "" {
			rowMaxCap, _ = strconv.ParseFloat(parts[7], 64)
		}

		result := calcSozialplan(sozialplanInput{
			MonthlySalary: sal,
			YearsService:  yrs,
			Age:           ag,
			Disabled:      dis,
			Children:      ch,
			Factor:        rowFactor,
		}, rowMaxCap)

		rows = append(rows, sozialplanBatchRow{Name: parts[0], Result: result})
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("Fehler beim Lesen der CSV-Datei: %w", err)
	}
	if len(rows) == 0 {
		return fmt.Errorf("keine Daten in der CSV-Datei gefunden")
	}

	if flags.asJSON || flags.agent {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	}

	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Sozialplan Batch-Berechnung (%d Mitarbeiter)\n", len(rows))
	fmt.Fprintf(w, "%-30s %8s %6s %5s %8s %12s\n", "Name", "Gehalt", "Jahre", "Alter", "Faktor", "Abfindung €")
	fmt.Fprintln(w, strings.Repeat("─", 75))
	total := 0.0
	for _, row := range rows {
		inp := row.Result.Input
		fmt.Fprintf(w, "%-30s %8.0f %6.1f %5d %8.2f %12.2f\n",
			row.Name, inp.MonthlySalary, inp.YearsService, inp.Age, inp.Factor, row.Result.FinalAmount)
		total += row.Result.FinalAmount
	}
	fmt.Fprintln(w, strings.Repeat("─", 75))
	fmt.Fprintf(w, "%-30s %34s %12.2f\n", "GESAMT", "", total)
	fmt.Fprintf(w, "\nRechtsgrundlage: § 112 BetrVG; Münchner Formel\n")
	return nil
}
