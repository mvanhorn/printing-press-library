package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/costco/internal/cliutil"

	"github.com/spf13/cobra"
)

// newExportCmd exports receipts (or their line items) to a file or stdout, for
// import into spreadsheets and budgeting tools. Writing a user-visible file is a
// side effect, so the command is not mcp:read-only and short-circuits under the
// verifier.
func newExportCmd(flags *rootFlags) *cobra.Command {
	var since, until, output, format string
	var years int
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export receipts or line items to a file (jsonl or csv) for budgeting tools",
		Long: strings.Trim(`
Export receipts for a date range to a file (or stdout with -), for import into
spreadsheets and budgeting tools.

--format jsonl writes one receipt per line; --format csv writes a flat
line-item table (one row per item). Use --output <path> to write a file, or
'-' for stdout.`, "\n"),
		Example:     "  costco-pp-cli export --years 2 --format csv --output costco.csv",
		Annotations: map[string]string{"mcp:write-positionals": "0"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would export receipts (%s) to %s\n", orDefault(format, "jsonl"), orDefault(output, "stdout"))
				return nil
			}
			format = strings.ToLower(orDefault(format, "jsonl"))
			if format != "jsonl" && format != "csv" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--format must be jsonl or csv (got %q)", format))
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would export receipts (%s) to %s\n", format, orDefault(output, "stdout"))
				return nil
			}
			start, end, err := resolveRange(since, until, years)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			receipts, err := fetchReceipts(ctx, flags, start, end)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			w := cmd.OutOrStdout()
			var closeFn func() error
			if output != "" && output != "-" {
				f, err := os.Create(output)
				if err != nil {
					return fmt.Errorf("creating output file: %w", err)
				}
				w = f
				closeFn = f.Close
			}

			switch format {
			case "jsonl":
				enc := json.NewEncoder(w)
				for _, r := range receipts {
					if err := enc.Encode(r); err != nil {
						if closeFn != nil {
							_ = closeFn()
						}
						return err
					}
				}
			case "csv":
				cw := csv.NewWriter(w)
				_ = cw.Write([]string{"transactionDate", "channel", "warehouseName", "itemNumber", "upc", "description", "unitPrice", "quantity", "amount"})
				for _, r := range receipts {
					for _, it := range r.ItemArray {
						_ = cw.Write([]string{
							r.TransactionDate, r.channel(), r.WarehouseName,
							it.ItemNumber.String(), it.UPC.String(), strings.TrimSpace(it.Description + " " + it.Description2),
							strconv.FormatFloat(it.UnitPriceAmount.float(), 'f', -1, 64),
							strconv.FormatFloat(it.Unit.float(), 'f', -1, 64),
							strconv.FormatFloat(it.Amount.float(), 'f', -1, 64),
						})
					}
				}
				cw.Flush()
				if err := cw.Error(); err != nil {
					if closeFn != nil {
						_ = closeFn()
					}
					return err
				}
			}
			if closeFn != nil {
				if err := closeFn(); err != nil {
					return err
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "exported %d receipts to %s\n", len(receipts), output)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Start of range (YYYY-MM-DD or duration)")
	cmd.Flags().StringVar(&until, "until", "", "End of range (YYYY-MM-DD; default today)")
	cmd.Flags().IntVar(&years, "years", 2, "Lookback in years when --since is not set")
	cmd.Flags().StringVar(&format, "format", "jsonl", "Export format: jsonl or csv")
	cmd.Flags().StringVar(&output, "output", "", "Output file path (default stdout; '-' for stdout)")
	return cmd
}
