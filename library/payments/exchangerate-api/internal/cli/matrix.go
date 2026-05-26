// Novel command: matrix. Computes an N×N cross-rate matrix for any list of
// currencies from a single /latest call, by deriving every cross-rate from
// the API's base→target rates. One API request, N² output rates.
package cli

// PATCH exchangerate-novel-matrix: NxN cross-rate matrix derived locally from a single /latest call.

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newMatrixCmd(flags *rootFlags) *cobra.Command {
	var (
		base    string
		asCSV   bool
		decimal int
	)
	cmd := &cobra.Command{
		Use:   "matrix <currencies>",
		Short: "Cross-rate matrix for N currencies from one API call",
		Long: `Show a full N×N cross-rate matrix for the given comma-separated currency codes.
Issues one /latest call against --base (default USD) and derives every cross-rate locally,
saving N-1 API calls per pair vs hitting /pair for each one.`,
		Example: "  exchangerate-api-pp-cli matrix USD,EUR,GBP,JPY\n  exchangerate-api-pp-cli matrix USD,EUR,GBP --base USD --json\n  exchangerate-api-pp-cli matrix USD,EUR,GBP,JPY,CHF,CAD --csv",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				// In dry-run mode the verify probe invokes
				// "<cmd> --dry-run" with no args; surface the planned
				// shape instead of bailing to help so the probe sees a
				// non-empty stdout.
				if dryRunOK(flags) {
					fmt.Fprintln(cmd.OutOrStdout(), "DRY-RUN matrix <currencies> (1 /latest call -> N^2 cross-rates)")
					return nil
				}
				return cmd.Help()
			}
			codes := splitAndUpper(args[0])
			if len(codes) < 2 {
				if dryRunOK(flags) {
					fmt.Fprintf(cmd.OutOrStdout(), "DRY-RUN matrix <currencies=%s> (1 /latest call -> N^2 cross-rates)\n", args[0])
					return nil
				}
				return usageErr(fmt.Errorf("at least two currency codes required (comma-separated)"))
			}
			if base == "" {
				base = codes[0]
			}
			base = strings.ToUpper(base)
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "DRY-RUN GET /v6/<key>/latest/%s (derive %dx%d cross-rates locally)\n", base, len(codes), len(codes))
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if c.Config == nil || c.Config.ExchangerateApiKey == "" {
				return usageErr(fmt.Errorf("EXCHANGERATE_API_KEY is required; export it or run 'auth set-token <key>'"))
			}
			path := fmt.Sprintf("/v6/%s/latest/%s", c.Config.ExchangerateApiKey, base)
			body, err := c.Get(path, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var resp struct {
				Result          string             `json:"result"`
				BaseCode        string             `json:"base_code"`
				ConversionRates map[string]float64 `json:"conversion_rates"`
				ErrorType       string             `json:"error-type"`
			}
			if err := json.Unmarshal(body, &resp); err != nil {
				return fmt.Errorf("parsing /latest response: %w", err)
			}
			if resp.Result != "success" {
				return fmt.Errorf("API error: %s", resp.ErrorType)
			}
			// Copy before mutating. c.Get may cache the unmarshaled view in
			// a future refactor; writing through the original map would
			// corrupt that cache for subsequent callers.
			rates := make(map[string]float64, len(resp.ConversionRates)+1)
			for k, v := range resp.ConversionRates {
				rates[k] = v
			}
			rates[resp.BaseCode] = 1.0
			missing := []string{}
			for _, code := range codes {
				if _, ok := rates[code]; !ok {
					missing = append(missing, code)
				}
			}
			if len(missing) > 0 {
				return fmt.Errorf("unsupported codes for base %s: %s", base, strings.Join(missing, ","))
			}
			matrix := make(map[string]map[string]float64, len(codes))
			for _, from := range codes {
				row := make(map[string]float64, len(codes))
				for _, to := range codes {
					row[to] = rates[to] / rates[from]
				}
				matrix[from] = row
			}
			if asCSV || flags.csv {
				w := csv.NewWriter(cmd.OutOrStdout())
				header := append([]string{""}, codes...)
				_ = w.Write(header)
				for _, from := range codes {
					row := []string{from}
					for _, to := range codes {
						row = append(row, formatRate(matrix[from][to], decimal))
					}
					_ = w.Write(row)
				}
				w.Flush()
				return nil
			}
			payload := map[string]any{
				"base":         base,
				"derived_base": resp.BaseCode,
				"codes":        codes,
				"matrix":       matrix,
				"api_calls":    1,
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), payload, flags)
			}
			// Human-friendly text table.
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Cross-rate matrix (base=%s, 1 API call → %d² rates):\n", resp.BaseCode, len(codes))
			fmt.Fprintf(out, "%-6s", "")
			for _, to := range codes {
				fmt.Fprintf(out, "%14s", to)
			}
			fmt.Fprintln(out)
			for _, from := range codes {
				fmt.Fprintf(out, "%-6s", from)
				for _, to := range codes {
					fmt.Fprintf(out, "%14s", formatRate(matrix[from][to], decimal))
				}
				fmt.Fprintln(out)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&base, "base", "", "Base currency for the API call (defaults to first code in <currencies>)")
	cmd.Flags().BoolVar(&asCSV, "matrix-csv", false, "Force CSV output (same as --csv)")
	cmd.Flags().IntVar(&decimal, "decimal", 6, "Decimal places for displayed rates")
	return cmd
}

func splitAndUpper(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(strings.ToUpper(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func formatRate(v float64, decimal int) string {
	return fmt.Sprintf("%.*f", decimal, v)
}
