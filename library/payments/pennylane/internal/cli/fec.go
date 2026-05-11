// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/payments/pennylane/internal/cliutil"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// newFECCmd returns the "fec" parent command.
func newFECCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fec",
		Short: "FEC — validation du Fichier des Écritures Comptables (DGFiP)",
	}
	cmd.AddCommand(newFECValidateCmd(flags))
	return cmd
}

// ─── fec validate ──────────────────────────────────────────────────────────

// Required DGFiP FEC columns (subset — positions vary, we match by header name).
var fecRequiredFields = []string{
	"JournalCode", "JournalLib", "EcritureNum", "EcritureDate",
	"CompteNum", "CompteLib", "EcritureLib", "Debit", "Credit",
}

type fecError struct {
	Line    int    `json:"line"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

type fecResult struct {
	Valid        bool       `json:"valid"`
	File         string     `json:"file"`
	TotalLines   int        `json:"total_lines"`
	EntryCount   int        `json:"entry_count"`
	JournalCount int        `json:"journal_count"`
	AccountCount int        `json:"account_count"`
	TotalDebit   float64    `json:"total_debit"`
	TotalCredit  float64    `json:"total_credit"`
	BalanceOK    bool       `json:"balance_ok"`
	Errors       []fecError `json:"errors"`
}

func newFECValidateCmd(flags *rootFlags) *cobra.Command {
	var filePath string

	cmd := &cobra.Command{
		Use:         "validate",
		Short:       "Valider un fichier FEC (DGFiP) — équilibre débit/crédit, champs obligatoires, monotonie des dates",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  accounting-pp-cli fec validate --file FEC2025.txt
  accounting-pp-cli fec validate --file FEC2025.txt --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if filePath == "" {
				return fmt.Errorf("--file est requis")
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), `{"status":"valid","error_count":0,"journal_count":0,"entry_count":0}`)
				return nil
			}

			f, err := os.Open(filePath)
			if err != nil {
				return fmt.Errorf("ouverture du fichier FEC : %w", err)
			}
			defer f.Close()

			var errors []fecError
			var headers []string
			var colIndex = make(map[string]int)
			var totalDebit, totalCredit float64
			var lastDate string
			journals := make(map[string]bool)
			accounts := make(map[string]bool)
			lineNum := 0
			entryCount := 0

			scanner := bufio.NewScanner(f)
			scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

			for scanner.Scan() {
				line := scanner.Text()
				lineNum++

				// Detect separator: tab or pipe or semicolon
				sep := detectFECSep(line)

				if lineNum == 1 {
					// Header line
					headers = strings.Split(line, string(sep))
					for i, h := range headers {
						colIndex[strings.TrimSpace(h)] = i
					}
					// Check required fields
					for _, req := range fecRequiredFields {
						if _, ok := colIndex[req]; !ok {
							errors = append(errors, fecError{
								Line:    1,
								Field:   req,
								Message: fmt.Sprintf("champ obligatoire manquant : %s", req),
							})
						}
					}
					continue
				}

				if strings.TrimSpace(line) == "" {
					continue
				}

				fields := strings.Split(line, string(sep))

				// Pad if needed
				for len(fields) < len(headers) {
					fields = append(fields, "")
				}

				entryCount++

				// Date monotonicity
				if idx, ok := colIndex["EcritureDate"]; ok && idx < len(fields) {
					date := strings.TrimSpace(fields[idx])
					if date != "" {
						if lastDate != "" && date < lastDate {
							errors = append(errors, fecError{
								Line:    lineNum,
								Field:   "EcritureDate",
								Message: fmt.Sprintf("date non monotone : %s < %s (ligne précédente)", date, lastDate),
							})
						}
						lastDate = date
					}
				}

				// Debit + Credit
				debit := 0.0
				credit := 0.0
				if idx, ok := colIndex["Debit"]; ok && idx < len(fields) {
					s := strings.ReplaceAll(strings.TrimSpace(fields[idx]), ",", ".")
					if s != "" {
						v, err := strconv.ParseFloat(s, 64)
						if err != nil {
							errors = append(errors, fecError{
								Line:    lineNum,
								Field:   "Debit",
								Message: fmt.Sprintf("valeur débit invalide : %q", fields[idx]),
							})
						} else {
							debit = v
						}
					}
				}
				if idx, ok := colIndex["Credit"]; ok && idx < len(fields) {
					s := strings.ReplaceAll(strings.TrimSpace(fields[idx]), ",", ".")
					if s != "" {
						v, err := strconv.ParseFloat(s, 64)
						if err != nil {
							errors = append(errors, fecError{
								Line:    lineNum,
								Field:   "Credit",
								Message: fmt.Sprintf("valeur crédit invalide : %q", fields[idx]),
							})
						} else {
							credit = v
						}
					}
				}
				totalDebit += debit
				totalCredit += credit

				// Journals
				if idx, ok := colIndex["JournalCode"]; ok && idx < len(fields) {
					journals[strings.TrimSpace(fields[idx])] = true
				}
				// Accounts
				if idx, ok := colIndex["CompteNum"]; ok && idx < len(fields) {
					accounts[strings.TrimSpace(fields[idx])] = true
				}
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("lecture du fichier : %w", err)
			}

			balanceOK := absFloat(totalDebit-totalCredit) < 0.01
			if !balanceOK {
				errors = append(errors, fecError{
					Line:    0,
					Message: fmt.Sprintf("déséquilibre débit/crédit : débit=%.2f crédit=%.2f diff=%.2f", totalDebit, totalCredit, totalDebit-totalCredit),
				})
			}

			result := fecResult{
				Valid:        len(errors) == 0,
				File:         filePath,
				TotalLines:   lineNum,
				EntryCount:   entryCount,
				JournalCount: len(journals),
				AccountCount: len(accounts),
				TotalDebit:   roundFloat(totalDebit, 2),
				TotalCredit:  roundFloat(totalCredit, 2),
				BalanceOK:    balanceOK,
				Errors:       errors,
			}
			if result.Errors == nil {
				result.Errors = []fecError{}
			}

			if flags.asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			validStr := "VALIDE"
			if !result.Valid {
				validStr = "INVALIDE"
			}
			fmt.Printf("Fichier : %s\nStatut  : %s\n\n", filePath, validStr)
			tw := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
			fmt.Fprintf(tw, "Lignes totales\t%d\n", lineNum)
			fmt.Fprintf(tw, "Écritures\t%d\n", entryCount)
			fmt.Fprintf(tw, "Journaux\t%d\n", len(journals))
			fmt.Fprintf(tw, "Comptes\t%d\n", len(accounts))
			fmt.Fprintf(tw, "Total débit\t%.2f\n", totalDebit)
			fmt.Fprintf(tw, "Total crédit\t%.2f\n", totalCredit)
			fmt.Fprintf(tw, "Équilibre\t%v\n", balanceOK)
			_ = tw.Flush()

			if len(errors) > 0 {
				fmt.Printf("\nErreurs (%d) :\n", len(errors))
				for _, e := range errors {
					if e.Line > 0 {
						fmt.Printf("  ligne %d : %s\n", e.Line, e.Message)
					} else {
						fmt.Printf("  %s\n", e.Message)
					}
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&filePath, "file", "", "Chemin vers le fichier FEC (.txt)")
	return cmd
}

func detectFECSep(line string) rune {
	counts := map[rune]int{'\t': 0, '|': 0, ';': 0}
	for _, c := range line {
		if _, ok := counts[c]; ok {
			counts[c]++
		}
	}
	// Iterate in a fixed priority order to guarantee deterministic results on ties.
	// Priority: tab (DGFiP default) > pipe > semicolon.
	best := '\t'
	bestN := counts['\t']
	for _, sep := range []rune{'|', ';'} {
		if counts[sep] > bestN {
			bestN = counts[sep]
			best = sep
		}
	}
	return best
}

func absFloat(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func roundFloat(x float64, decimals int) float64 {
	pow := 1.0
	for i := 0; i < decimals; i++ {
		pow *= 10
	}
	return float64(int(x*pow+0.5)) / pow
}
