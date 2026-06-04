package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type normalizeUnconverted struct {
	CurrencyCode string  `json:"currency_code"`
	Original     float64 `json:"original"`
}

type normalizeRow struct {
	CurrencyCode string  `json:"currency_code"`
	Original     float64 `json:"original"`
	Rate         float64 `json:"rate"`
	Converted    float64 `json:"converted"`
}

type normalizeMetric struct {
	Rows        []normalizeRow         `json:"rows"`
	TotalBase   float64                `json:"total_base"`
	Unconverted []normalizeUnconverted `json:"unconverted"`
}

type normalizeResult struct {
	Base  string          `json:"base"`
	Net   normalizeMetric `json:"net"`
	Spend normalizeMetric `json:"spend"`
}

func parseNormalizeRates(raw []string) (map[string]float64, error) {
	rates := make(map[string]float64)
	for _, item := range raw {
		parts := strings.SplitN(strings.TrimSpace(item), "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid --rate %q: expected CUR=FACTOR", item)
		}
		cc := strings.ToUpper(strings.TrimSpace(parts[0]))
		if cc == "" {
			return nil, fmt.Errorf("invalid --rate %q: currency code is required", item)
		}
		factor, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid --rate %q: factor must be numeric", item)
		}
		if factor <= 0 {
			return nil, fmt.Errorf("invalid --rate %q: factor must be > 0", item)
		}
		rates[cc] = factor
	}
	return rates, nil
}

func loadNormalizeRatesFile(path string) (map[string]float64, error) {
	if strings.TrimSpace(path) == "" {
		return map[string]float64{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read --rates-file: %w", err)
	}
	var raw map[string]float64
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse --rates-file JSON object: %w", err)
	}
	rates := make(map[string]float64, len(raw))
	for k, v := range raw {
		cc := strings.ToUpper(strings.TrimSpace(k))
		if cc == "" {
			return nil, fmt.Errorf("--rates-file contains empty currency code")
		}
		if v <= 0 {
			return nil, fmt.Errorf("--rates-file contains non-positive factor for %s", cc)
		}
		rates[cc] = v
	}
	return rates, nil
}

func makeNormalizeMetric(byCurrency map[string]float64, rates map[string]float64, base string) normalizeMetric {
	metric := normalizeMetric{
		Rows:        make([]normalizeRow, 0),
		Unconverted: make([]normalizeUnconverted, 0),
	}
	currencies := make([]string, 0, len(byCurrency))
	for cc := range byCurrency {
		currencies = append(currencies, cc)
	}
	sort.Strings(currencies)

	for _, cc := range currencies {
		original := round2(byCurrency[cc])
		rate := 0.0
		if cc == base {
			rate = 1.0
		} else {
			rate = rates[cc]
		}
		if rate <= 0 {
			metric.Unconverted = append(metric.Unconverted, normalizeUnconverted{CurrencyCode: cc, Original: original})
			continue
		}
		converted := round2(original * rate)
		metric.Rows = append(metric.Rows, normalizeRow{
			CurrencyCode: cc,
			Original:     original,
			// Keep the full-precision rate actually used in the conversion so a
			// reader can reproduce original*rate≈converted; rounding it for display
			// disagreed with the conversion math (Greptile #970).
			Rate:      rate,
			Converted: converted,
		})
		metric.TotalBase = round2(metric.TotalBase + converted)
	}

	return metric
}

func computeNormalize(friends []Friend, expenses []Expense, youID int, rates map[string]float64, base string) normalizeResult {
	netByCurrency := make(map[string]float64)
	for _, f := range friends {
		for _, b := range f.Balance {
			cc := strings.ToUpper(strings.TrimSpace(b.CurrencyCode))
			if cc == "" {
				continue
			}
			netByCurrency[cc] += parseAmount(b.Amount)
		}
	}

	spendByCurrency := make(map[string]float64)
	for _, e := range expenses {
		if e.Payment || expenseDeleted(e.DeletedAt) {
			continue
		}
		cc := strings.ToUpper(strings.TrimSpace(e.CurrencyCode))
		if cc == "" {
			continue
		}
		for _, u := range e.Users {
			if u.UserID == youID {
				spendByCurrency[cc] += parseAmount(u.OwedShare)
				break
			}
		}
	}

	return normalizeResult{
		Base:  base,
		Net:   makeNormalizeMetric(netByCurrency, rates, base),
		Spend: makeNormalizeMetric(spendByCurrency, rates, base),
	}
}

// pp:data-source local
func newNormalizeCmd(flags *rootFlags) *cobra.Command {
	base := "USD"
	rateFlags := make([]string, 0)
	ratesFile := ""

	cmd := &cobra.Command{
		Use:         "normalize",
		Short:       "Normalize multi-currency net and spend into one base currency using user-supplied FX rates",
		Example:     "  splitwise-pp-cli normalize --base USD --rate EUR=1.08 --rate GBP=1.27 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "would normalize multi-currency balances and spend")
				return nil
			}

			baseCode := strings.ToUpper(strings.TrimSpace(base))
			if baseCode == "" {
				return usageErr(fmt.Errorf("--base must not be empty"))
			}

			mergedRates, err := loadNormalizeRatesFile(ratesFile)
			if err != nil {
				return usageErr(err)
			}
			inlineRates, err := parseNormalizeRates(rateFlags)
			if err != nil {
				return usageErr(err)
			}
			for cc, rate := range inlineRates {
				mergedRates[cc] = rate
			}
			mergedRates[baseCode] = 1.0

			// Historical/automatic FX lookup is intentionally out of scope; normalize is deterministic and offline-only.
			db, err := openSplitwiseStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			hintIfUnsynced(cmd, db, "get-friends")
			hintIfUnsynced(cmd, db, "get-expenses")
			hintIfStale(cmd, db, "get-friends", flags.maxAge)
			hintIfStale(cmd, db, "get-expenses", flags.maxAge)

			friends, err := loadFriends(db)
			if err != nil {
				return err
			}
			expenses, err := loadExpenses(db)
			if err != nil {
				return err
			}
			youID := loadCurrentUserID(db)
			if youID == 0 {
				// Without a synced current user the Spend per-user owed-share loop
				// never matches, so Spend totals are silently 0.00. Warn on stderr in
				// every output mode (mirrors balances --by-group) so the zero reads as
				// an unknown-identity artifact, not a real total.
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "note: current user not synced; run sync to populate get-current-user")
			}

			res := computeNormalize(friends, expenses, youID, mergedRates, baseCode)
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, res)
			}

			out := cmd.OutOrStdout()
			_, _ = fmt.Fprintln(out, "Net position")
			tw := tabwriter.NewWriter(out, 2, 4, 2, ' ', 0)
			_, _ = fmt.Fprintf(tw, "CURRENCY\tORIGINAL\tRATE\tIN %s\n", res.Base)
			for _, row := range res.Net.Rows {
				_, _ = fmt.Fprintf(tw, "%s\t%.2f\t%.4f\t%.2f\n", row.CurrencyCode, row.Original, row.Rate, row.Converted)
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(out, "Total ≈ %.2f %s\n", res.Net.TotalBase, res.Base)

			_, _ = fmt.Fprintln(out)
			_, _ = fmt.Fprintln(out, "Spend")
			tw2 := tabwriter.NewWriter(out, 2, 4, 2, ' ', 0)
			_, _ = fmt.Fprintf(tw2, "CURRENCY\tORIGINAL\tRATE\tIN %s\n", res.Base)
			for _, row := range res.Spend.Rows {
				_, _ = fmt.Fprintf(tw2, "%s\t%.2f\t%.4f\t%.2f\n", row.CurrencyCode, row.Original, row.Rate, row.Converted)
			}
			if err := tw2.Flush(); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(out, "Total ≈ %.2f %s\n", res.Spend.TotalBase, res.Base)

			if len(res.Net.Unconverted) > 0 || len(res.Spend.Unconverted) > 0 {
				_, _ = fmt.Fprintln(out)
				_, _ = fmt.Fprintln(out, "Unconverted currencies (add --rate CUR=FACTOR):")
				for _, row := range res.Net.Unconverted {
					_, _ = fmt.Fprintf(out, "net: %s %.2f\n", row.CurrencyCode, row.Original)
				}
				for _, row := range res.Spend.Unconverted {
					_, _ = fmt.Fprintf(out, "spend: %s %.2f\n", row.CurrencyCode, row.Original)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&base, "base", "USD", "Base currency code used for normalization")
	cmd.Flags().StringArrayVar(&rateFlags, "rate", nil, "FX rate CUR=FACTOR where 1 CUR equals FACTOR in base currency (repeatable)")
	cmd.Flags().StringVar(&ratesFile, "rates-file", "", "JSON file with currency-rate object, e.g. {\"EUR\":1.08}; merged before --rate overrides")
	cmd.Long = "Normalize your multi-currency Splitwise net position and spend into one base currency using only user-supplied rates from --rate and/or --rates-file. Historical or automatic FX lookup is intentionally out of scope."

	return cmd
}
