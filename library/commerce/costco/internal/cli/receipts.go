package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// receiptSummary is the flat per-receipt row emitted by `receipts`. The full
// line-item detail lives behind `receipt get <barcode>` so list output stays
// CSV/--select friendly.
type receiptSummary struct {
	TransactionDate string  `json:"transactionDate"`
	Channel         string  `json:"channel"`
	WarehouseName   string  `json:"warehouseName"`
	WarehouseNumber string  `json:"warehouseNumber"`
	ItemCount       int     `json:"itemCount"`
	SubTotal        float64 `json:"subTotal"`
	Taxes           float64 `json:"taxes"`
	InstantSavings  float64 `json:"instantSavings"`
	Total           float64 `json:"total"`
	Barcode         string  `json:"transactionBarcode"`
}

func summarize(r costcoReceipt) receiptSummary {
	return receiptSummary{
		TransactionDate: r.TransactionDate,
		Channel:         r.channel(),
		WarehouseName:   r.WarehouseName,
		WarehouseNumber: r.WarehouseNumber.String(),
		ItemCount:       int(r.TotalItemCount.float()),
		SubTotal:        r.SubTotal.float(),
		Taxes:           r.Taxes.float(),
		InstantSavings:  r.InstantSavings.float(),
		Total:           r.Total.float(),
		Barcode:         r.TransactionBarcode,
	}
}

// matchesType reports whether a receipt belongs to the requested channel filter.
// typ is one of: all, warehouse, gas, carwash.
func matchesType(r costcoReceipt, typ string) bool {
	typ = strings.ToLower(strings.TrimSpace(typ))
	if typ == "" || typ == "all" {
		return true
	}
	ch := r.channel()
	switch typ {
	case "gas", "fuel":
		return strings.Contains(ch, "gas")
	case "carwash", "car-wash":
		return strings.Contains(ch, "carwash")
	case "warehouse", "in-warehouse", "inwarehouse":
		return ch == "warehouse"
	default:
		return ch == typ
	}
}

func newReceiptsCmd(flags *rootFlags) *cobra.Command {
	var since, until, typ string
	var years, limit int
	cmd := &cobra.Command{
		Use:   "receipts",
		Short: "List in-warehouse, gas, and carwash receipts for a date range",
		Long: strings.Trim(`
List Costco in-warehouse, gas, and carwash receipts for a date range.

The website caps its date picker at 2 years, but the API accepts any range.
Pass --since/--until (YYYY-MM-DD) or --since 6mo/1y, or widen with --years.
Use 'history-depth' to find how far back your account actually goes, and
'receipt get <barcode>' for full line-item detail on one receipt.`, "\n"),
		Example: strings.Trim(`
  costco-pp-cli receipts --since 2024-01-01
  costco-pp-cli receipts --years 3 --type gas --json
  costco-pp-cli receipts --since 6mo --agent --select transactionDate,warehouseName,total`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch receipts for the requested range")
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
			rows := make([]receiptSummary, 0, len(receipts))
			for _, r := range receipts {
				if !matchesType(r, typ) {
					continue
				}
				rows = append(rows, summarize(r))
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].TransactionDate > rows[j].TransactionDate })
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			b, err := json.Marshal(rows)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Start of range: YYYY-MM-DD or a duration like 30d/6mo/1y")
	cmd.Flags().StringVar(&until, "until", "", "End of range (YYYY-MM-DD; default today)")
	cmd.Flags().IntVar(&years, "years", 2, "Lookback in years when --since is not set")
	cmd.Flags().StringVar(&typ, "type", "all", "Filter by channel: all, warehouse, gas, carwash")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max receipts to return (0 = all)")
	return cmd
}

func newReceiptCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "receipt",
		Short:       "Inspect a single receipt",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newReceiptGetCmd(flags))
	return cmd
}

func newReceiptGetCmd(flags *rootFlags) *cobra.Command {
	var since, until string
	var years int
	cmd := &cobra.Command{
		Use:   "get <barcode>",
		Short: "Show full line-item detail for one receipt by transaction barcode",
		Long: strings.Trim(`
Show full detail (line items, tenders, coupons) for a single receipt.

The barcode comes from 'receipts' output (transactionBarcode). Because the API
returns receipts by date range, pass a range wide enough to include the receipt
(defaults to the last 2 years; widen with --years or --since).`, "\n"),
		Example:     "  costco-pp-cli receipt get 123456789012 --years 3 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch the receipt and print line-item detail")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a transaction barcode is required"))
			}
			barcode := args[0]
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
			for _, r := range receipts {
				if r.TransactionBarcode == barcode {
					b, err := json.Marshal(r)
					if err != nil {
						return err
					}
					return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
				}
			}
			return fmt.Errorf("no receipt with barcode %q in range %s..%s (widen with --years or --since)", barcode, start, end)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Start of search range (YYYY-MM-DD or duration)")
	cmd.Flags().StringVar(&until, "until", "", "End of search range (YYYY-MM-DD; default today)")
	cmd.Flags().IntVar(&years, "years", 2, "Lookback in years when --since is not set")
	return cmd
}

// channelCount is one row of the counts summary.
type channelCount struct {
	Channel string  `json:"channel"`
	Count   int     `json:"count"`
	Total   float64 `json:"total"`
}

func newCountsCmd(flags *rootFlags) *cobra.Command {
	var since, until string
	var years int
	cmd := &cobra.Command{
		Use:   "counts",
		Short: "Summarize receipt counts and spend by channel (warehouse/gas/carwash)",
		Long: strings.Trim(`
Summarize how many receipts and how much spend fall in each channel
(warehouse, gas, carwash) over a date range.`, "\n"),
		Example:     "  costco-pp-cli counts --years 3 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would summarize receipt counts by channel")
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
			agg := map[string]*channelCount{}
			for _, r := range receipts {
				ch := r.channel()
				if agg[ch] == nil {
					agg[ch] = &channelCount{Channel: ch}
				}
				agg[ch].Count++
				agg[ch].Total += r.Total.float()
			}
			rows := make([]channelCount, 0, len(agg))
			for _, v := range agg {
				rows = append(rows, *v)
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].Count > rows[j].Count })
			b, err := json.Marshal(rows)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Start of range (YYYY-MM-DD or duration)")
	cmd.Flags().StringVar(&until, "until", "", "End of range (YYYY-MM-DD; default today)")
	cmd.Flags().IntVar(&years, "years", 2, "Lookback in years when --since is not set")
	return cmd
}
