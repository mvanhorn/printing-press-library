// Copyright 2026 matt-van-horn. Licensed under Apache-2.0.
// `expense from-line` — parse a bank/card statement line into an Expensify expense.

package cli

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newExpenseFromLineCmd(flags *rootFlags) *cobra.Command {
	var category, policyID, currency string
	var previewOnly bool
	cmd := &cobra.Command{
		Use:   "from-line <bank-line>",
		Short: "Parse a bank/card statement line into an expense",
		Long: `Regex-parses a line like "2026-04-18 DOORDASH*JOES $14.25" or "04/18 AMEX*SHAKE SHACK $24.00"
into date / merchant / amount. Card prefixes (AMEX*, SQ*, TST*, PP*) are stripped.`,
		Example: `  expensify-pp-cli expense from-line "2026-04-18 DOORDASH*JOES $14.25"
  expensify-pp-cli expense from-line "04/18 SQ*BLUE BOTTLE 6.50" --category Meals`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			line := strings.Join(args, " ")
			date, merchant, amount, err := parseBankLine(line)
			if err != nil {
				return usageErr(err)
			}
			if currency == "" {
				currency = "USD"
			}
			body := map[string]any{
				"amount":   amount,
				"merchant": merchant,
				"created":  date,
				"currency": currency,
			}
			if category != "" {
				body["category"] = category
			}
			if policyID != "" {
				body["policyID"] = policyID
			}

			if previewOnly || flags.dryRun {
				w := cmd.OutOrStdout()
				fmt.Fprintf(w, "DRY RUN: would create expense from line: %s\n", line)
				fmt.Fprintf(w, "  merchant: %s\n", merchant)
				fmt.Fprintf(w, "  amount:   %d cents (%s %.2f)\n", amount, currency, float64(amount)/100)
				fmt.Fprintf(w, "  date:     %s\n", date)
				if category != "" {
					fmt.Fprintf(w, "  category: %s\n", category)
				}
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, status, err := c.Post(cmd.Context(), "/RequestMoney", body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			id := extractTransactionID(data)
			w := cmd.OutOrStdout()
			if id != "" {
				fmt.Fprintf(w, "Created expense %s from line: %s (HTTP %d)\n", id, line, status)
			} else {
				fmt.Fprintf(w, "Created expense from line: %s (HTTP %d)\n", line, status)
			}
			fmt.Fprintf(w, "  merchant: %s\n  amount:   %d cents (%s %.2f)\n  date:     %s\n",
				merchant, amount, currency, float64(amount)/100, date)
			if category != "" {
				fmt.Fprintf(w, "  category: %s\n", category)
			}
			if flags.asJSON {
				fmt.Fprintln(w, string(data))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&category, "category", "", "Category")
	cmd.Flags().StringVar(&policyID, "policy", "", "Policy/workspace ID")
	cmd.Flags().StringVar(&currency, "currency", "", "Currency (default USD)")
	cmd.Flags().BoolVar(&previewOnly, "dry-run", false, "Preview only")
	return cmd
}

var (
	reISODate     = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})\s+`)
	reSlashDate   = regexp.MustCompile(`^(\d{1,2})[/-](\d{1,2})(?:[/-](\d{2,4}))?\s+`)
	reAmountDecl  = regexp.MustCompile(`\$?(\d+(?:,\d{3})*(?:\.\d{2}))\s*$`)
	reCardPrefix  = regexp.MustCompile(`(?i)^(AMEX|SQ|TST|PP|DD|GPA|PAYPAL|APL)\*+`)
	reMultiSpaces = regexp.MustCompile(`\s{2,}`)
)

// parseBankLine returns YYYY-MM-DD date, cleaned merchant, cents amount.
func parseBankLine(line string) (string, string, int, error) {
	line = strings.TrimSpace(line)
	date := ""
	rest := line
	if m := reISODate.FindStringSubmatch(line); m != nil {
		date = m[1]
		rest = strings.TrimPrefix(line, m[0])
	} else if m := reSlashDate.FindStringSubmatch(line); m != nil {
		month, _ := strconv.Atoi(m[1])
		day, _ := strconv.Atoi(m[2])
		year := time.Now().Year()
		if m[3] != "" {
			y, _ := strconv.Atoi(m[3])
			if y < 100 {
				y += 2000
			}
			year = y
		}
		date = fmt.Sprintf("%04d-%02d-%02d", year, month, day)
		rest = strings.TrimPrefix(line, m[0])
	} else {
		date = time.Now().Format("2006-01-02")
	}

	// Amount from the end
	am := reAmountDecl.FindStringSubmatch(rest)
	if am == nil {
		return "", "", 0, fmt.Errorf("could not parse amount from %q", line)
	}
	amount, err := strconv.ParseFloat(strings.ReplaceAll(am[1], ",", ""), 64)
	if err != nil {
		return "", "", 0, fmt.Errorf("bad amount %q: %w", am[1], err)
	}
	merchant := strings.TrimSpace(reAmountDecl.ReplaceAllString(rest, ""))
	merchant = strings.TrimRight(merchant, " -")
	merchant = reCardPrefix.ReplaceAllString(merchant, "")
	merchant = reMultiSpaces.ReplaceAllString(merchant, " ")
	merchant = strings.TrimSpace(merchant)
	if merchant == "" {
		return "", "", 0, fmt.Errorf("could not parse merchant from %q", line)
	}
	return date, merchant, int(amount*100 + 0.5), nil
}
