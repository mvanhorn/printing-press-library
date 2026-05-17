// Novel command: convert-batch. Reads amounts from stdin (one per line) and
// converts each from --from to --to using ONE /pair fetch. The rate is then
// applied locally to every amount, saving N-1 quota ticks vs invoking
// 'convert' per amount.
package cli

// PATCH exchangerate-novel-convert: convert-batch (stdin-driven, 1 /pair fetch for N amounts).

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/exchangerate-api/internal/store"
	"github.com/spf13/cobra"
)

func newConvertBatchCmd(flags *rootFlags) *cobra.Command {
	var (
		from, to string
		input    string
		dbPath   string
		noLog    bool
	)
	cmd := &cobra.Command{
		Use:   "convert-batch",
		Short: "Convert many amounts from one source-target rate fetch (stdin or --input)",
		Long: `Reads amounts one per line from stdin (or --input <file>) and converts each
from --from to --to using a single /pair call. The fetched rate is applied
locally to every amount, so N amounts cost 1 quota tick.`,
		Example: "  printf '10\\n25\\n100' | exchangerate-api-pp-cli convert-batch --from USD --to EUR\n  exchangerate-api-pp-cli convert-batch --from GBP --to JPY --input - --json   # read amounts from stdin",
		// Not mcp:read-only: writes one row per amount to local conversions_log.
		// Pass --no-log for the read-only form.
		Annotations: map[string]string{},
		RunE: func(cmd *cobra.Command, args []string) error {
			// In dry-run mode, surface the planned request even when no
			// flags were supplied so the verify probe (which invokes
			// "<cmd> --dry-run" with no args) can confirm the command
			// is reachable without erroring on missing flags.
			if dryRunOK(flags) {
				fromShown := from
				if fromShown == "" {
					fromShown = "<from>"
				}
				toShown := to
				if toShown == "" {
					toShown = "<to>"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "DRY-RUN convert-batch %s -> %s (reads amounts from %s; 1 /pair call)\n",
					strings.ToUpper(fromShown), strings.ToUpper(toShown),
					func() string {
						if input == "" {
							return "stdin"
						}
						return input
					}())
				return nil
			}
			if from == "" || to == "" {
				return usageErr(fmt.Errorf("--from and --to are required"))
			}
			from = strings.ToUpper(from)
			to = strings.ToUpper(to)
			var reader io.Reader
			switch input {
			case "", "-":
				reader = os.Stdin
			default:
				f, err := os.Open(input)
				if err != nil {
					return fmt.Errorf("opening --input %s: %w", input, err)
				}
				defer f.Close()
				reader = f
			}
			amounts := []float64{}
			sc := bufio.NewScanner(reader)
			for sc.Scan() {
				line := strings.TrimSpace(sc.Text())
				if line == "" {
					continue
				}
				v, err := strconv.ParseFloat(line, 64)
				if err != nil {
					return usageErr(fmt.Errorf("line %q is not a number", line))
				}
				amounts = append(amounts, v)
			}
			if err := sc.Err(); err != nil {
				return fmt.Errorf("reading input: %w", err)
			}
			if len(amounts) == 0 {
				// Empty stdin or empty file is a successful no-op (matches
				// agent-friendly convention: don't fail on "nothing to do").
				payload := map[string]any{
					"from": from, "to": to, "count": 0, "api_calls": 0, "results": []any{}, "note": "no amounts on input",
				}
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					return printJSONFiltered(cmd.OutOrStdout(), payload, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "no amounts provided on stdin/input; nothing to convert")
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if c.Config == nil || c.Config.ExchangerateApiKey == "" {
				return usageErr(fmt.Errorf("EXCHANGERATE_API_KEY is required; export it or run 'auth set-token <key>'"))
			}
			path := fmt.Sprintf("/v6/%s/pair/%s/%s", c.Config.ExchangerateApiKey, from, to)
			body, err := c.Get(path, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var pr struct {
				Result         string  `json:"result"`
				ErrorType      string  `json:"error-type"`
				BaseCode       string  `json:"base_code"`
				TargetCode     string  `json:"target_code"`
				ConversionRate float64 `json:"conversion_rate"`
			}
			if err := json.Unmarshal(body, &pr); err != nil {
				return fmt.Errorf("parsing /pair: %w", err)
			}
			if pr.Result != "success" {
				return fmt.Errorf("API error: %s", pr.ErrorType)
			}

			type row struct {
				Amount float64 `json:"amount"`
				Result float64 `json:"result"`
			}
			results := make([]row, 0, len(amounts))
			for _, a := range amounts {
				results = append(results, row{Amount: a, Result: a * pr.ConversionRate})
			}
			if !noLog {
				if dbPath == "" {
					dbPath = defaultDBPath("exchangerate-api-pp-cli")
				}
				if s, sErr := store.OpenWithContext(cmd.Context(), dbPath); sErr == nil {
					for _, r := range results {
						_ = s.InsertConversionLog(cmd.Context(), pr.BaseCode, pr.TargetCode, r.Amount, r.Result, pr.ConversionRate, "api:/pair (batch)")
					}
					_ = s.Close()
				}
			}
			payload := map[string]any{
				"from":      pr.BaseCode,
				"to":        pr.TargetCode,
				"rate":      pr.ConversionRate,
				"count":     len(results),
				"api_calls": 1,
				"results":   results,
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), payload, flags)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Rate %s -> %s: %.6f (1 API call, %d amounts)\n", pr.BaseCode, pr.TargetCode, pr.ConversionRate, len(results))
			for _, r := range results {
				fmt.Fprintf(out, "  %.4f %s = %.4f %s\n", r.Amount, pr.BaseCode, r.Result, pr.TargetCode)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Source currency")
	cmd.Flags().StringVar(&to, "to", "", "Target currency")
	cmd.Flags().StringVar(&input, "input", "", "Read amounts from this file instead of stdin")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override local SQLite path")
	cmd.Flags().BoolVar(&noLog, "no-log", false, "Skip writing to conversions_log")
	return cmd
}
