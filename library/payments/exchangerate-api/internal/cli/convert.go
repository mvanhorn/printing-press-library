// Novel command: convert. Ergonomic wrapper for currency conversion that
// matches the TimothyYe/exchangerate UX ("er USD 12 CNY,JPY") and adds
// multi-target conversion from a single /latest call.
package cli

// PATCH exchangerate-novel-convert: ergonomic convert (single/multi-target via /pair or /latest).

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/exchangerate-api/internal/store"
	"github.com/spf13/cobra"
)

func newConvertCmd(flags *rootFlags) *cobra.Command {
	var (
		fromFlag, toFlag string
		amountFlag       string
		dbPath           string
		noLog            bool
	)
	cmd := &cobra.Command{
		Use:   "convert <amount> <from> <to[,to,...]>",
		Short: "Convert an amount from one currency to one or more targets",
		Long: `Convert <amount> from <from> to one or more <to> currencies. If multiple targets
are given (comma-separated), a single /latest call is issued and every target is
converted locally — saving N-1 quota ticks.

Each conversion is logged to the local conversions_log so 'log show' can recall
recent work.`,
		Example: "  exchangerate-api-pp-cli convert 250 USD EUR\n  exchangerate-api-pp-cli convert 100 USD EUR,GBP,JPY --json\n  exchangerate-api-pp-cli convert 50 GBP JPY --json --select results.0.result\n  exchangerate-api-pp-cli convert --amount 1000 --from USD --to EUR,GBP",
		// Not mcp:read-only: writes to local conversions_log on every call.
		// Agents that want the read-only form should pass --no-log.
		Annotations: map[string]string{},
		RunE: func(cmd *cobra.Command, args []string) error {
			amountStr, from, to := amountFlag, fromFlag, toFlag
			switch len(args) {
			case 3:
				amountStr, from, to = args[0], args[1], args[2]
			case 0:
				if amountStr == "" || from == "" || to == "" {
					// In dry-run mode the verify probe invokes
					// "<cmd> --dry-run" with no args; surface the
					// planned shape instead of bailing to help so the
					// probe sees a non-empty stdout.
					if dryRunOK(flags) {
						fmt.Fprintln(cmd.OutOrStdout(), "DRY-RUN convert <amount> <from> <to[,to,...]> (1 /pair or /latest call)")
						return nil
					}
					return cmd.Help()
				}
			default:
				if dryRunOK(flags) {
					fmt.Fprintln(cmd.OutOrStdout(), "DRY-RUN convert <amount> <from> <to[,to,...]> (1 /pair or /latest call)")
					return nil
				}
				return usageErr(fmt.Errorf("expected: convert <amount> <from> <to[,to,...]> (got %d positional args)", len(args)))
			}
			amount, err := strconv.ParseFloat(strings.TrimSpace(amountStr), 64)
			if err != nil {
				if dryRunOK(flags) {
					fmt.Fprintf(cmd.OutOrStdout(), "DRY-RUN convert <amount=%s> <from=%s> <to=%s> (1 /pair or /latest call)\n", amountStr, from, to)
					return nil
				}
				return usageErr(fmt.Errorf("amount %q is not a number", amountStr))
			}
			from = strings.ToUpper(strings.TrimSpace(from))
			targets := splitAndUpper(to)
			if len(targets) == 0 {
				if dryRunOK(flags) {
					fmt.Fprintf(cmd.OutOrStdout(), "DRY-RUN convert <amount=%g> <from=%s> (1 /pair or /latest call)\n", amount, from)
					return nil
				}
				return usageErr(fmt.Errorf("at least one target currency is required"))
			}
			if dryRunOK(flags) {
				if len(targets) == 1 {
					fmt.Fprintf(cmd.OutOrStdout(), "DRY-RUN GET /v6/<key>/pair/%s/%s/%g\n", from, targets[0], amount)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "DRY-RUN GET /v6/<key>/latest/%s (locally convert %g to %s)\n", from, amount, strings.Join(targets, ","))
				}
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if c.Config == nil || c.Config.ExchangerateApiKey == "" {
				return usageErr(fmt.Errorf("EXCHANGERATE_API_KEY is required; export it or run 'auth set-token <key>'"))
			}

			results := make([]map[string]any, 0, len(targets))
			source := "api:/latest"

			if len(targets) == 1 {
				// Single target: use /pair/<from>/<to>/<amount> so the API
				// returns conversion_result directly. Use the parsed amount
				// rather than the raw user string so the wire value matches
				// what we validated (avoids round-tripping unvalidated input
				// into the URL path).
				source = "api:/pair"
				amountWire := strconv.FormatFloat(amount, 'f', -1, 64)
				path := fmt.Sprintf("/v6/%s/pair/%s/%s/%s", c.Config.ExchangerateApiKey, from, targets[0], amountWire)
				body, err := c.Get(path, nil)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var pr struct {
					Result           string  `json:"result"`
					ErrorType        string  `json:"error-type"`
					BaseCode         string  `json:"base_code"`
					TargetCode       string  `json:"target_code"`
					ConversionRate   float64 `json:"conversion_rate"`
					ConversionResult float64 `json:"conversion_result"`
				}
				if err := json.Unmarshal(body, &pr); err != nil {
					return fmt.Errorf("parsing /pair response: %w", err)
				}
				if pr.Result != "success" {
					return fmt.Errorf("API error: %s", pr.ErrorType)
				}
				results = append(results, map[string]any{
					"from":   pr.BaseCode,
					"to":     pr.TargetCode,
					"amount": amount,
					"rate":   pr.ConversionRate,
					"result": pr.ConversionResult,
				})
			} else {
				// Multi-target: one /latest call, compute locally.
				path := fmt.Sprintf("/v6/%s/latest/%s", c.Config.ExchangerateApiKey, from)
				body, err := c.Get(path, nil)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var lr struct {
					Result          string             `json:"result"`
					ErrorType       string             `json:"error-type"`
					BaseCode        string             `json:"base_code"`
					ConversionRates map[string]float64 `json:"conversion_rates"`
				}
				if err := json.Unmarshal(body, &lr); err != nil {
					return fmt.Errorf("parsing /latest response: %w", err)
				}
				if lr.Result != "success" {
					return fmt.Errorf("API error: %s", lr.ErrorType)
				}
				for _, t := range targets {
					rate, ok := lr.ConversionRates[t]
					if !ok {
						return fmt.Errorf("target %s not supported by API (base %s)", t, lr.BaseCode)
					}
					results = append(results, map[string]any{
						"from":   lr.BaseCode,
						"to":     t,
						"amount": amount,
						"rate":   rate,
						"result": amount * rate,
					})
				}
			}

			// Log to local conversions_log (best-effort; failure does not break the result).
			if !noLog {
				if dbPath == "" {
					dbPath = defaultDBPath("exchangerate-api-pp-cli")
				}
				if s, sErr := store.OpenWithContext(cmd.Context(), dbPath); sErr == nil {
					for _, r := range results {
						_ = s.InsertConversionLog(cmd.Context(),
							r["from"].(string), r["to"].(string),
							r["amount"].(float64), r["result"].(float64), r["rate"].(float64),
							source)
					}
					_ = s.Close()
				}
			}

			payload := map[string]any{
				"amount":    amount,
				"from":      from,
				"targets":   targets,
				"results":   results,
				"api_calls": 1,
				"source":    source,
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), payload, flags)
			}
			out := cmd.OutOrStdout()
			for _, r := range results {
				fmt.Fprintf(out, "%.4f %s = %.4f %s  (rate %.6f)\n",
					r["amount"].(float64), r["from"].(string),
					r["result"].(float64), r["to"].(string),
					r["rate"].(float64))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&fromFlag, "from", "", "Source currency (alternative to positional)")
	cmd.Flags().StringVar(&toFlag, "to", "", "Target currency or comma-separated list (alternative to positional)")
	cmd.Flags().StringVar(&amountFlag, "amount", "", "Amount to convert (alternative to positional)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override local SQLite path (default ~/.local/share/exchangerate-api-pp-cli/data.db)")
	cmd.Flags().BoolVar(&noLog, "no-log", false, "Skip writing the conversion to conversions_log")
	return cmd
}
