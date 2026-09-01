// Hand-authored transcendence command. generate --force preserves this file.
// pp:data-source live
package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/hostex/internal/cliutil"
)

func newNovelRevenueRollupCmd(flags *rootFlags) *cobra.Command {
	var flagBy string
	var flagMonth string
	var flagFrom string
	var flagTo string
	var flagMaxPages string

	cmd := &cobra.Command{
		Use:   "revenue-rollup",
		Short: "Net income minus expense by property or month over a date range, from the live ledger.",
		Long: "Queries the Hostex transactions ledger live over a date range and nets income\n" +
			"minus expense grouped by property or by month. The API returns raw ledger\n" +
			"entries (and requires an explicit start/end date); this paginates the range and\n" +
			"aggregates per group and currency in one command.\n\n" +
			"Live command (needs a valid token). Defaults to the last 365 days unless --month\n" +
			"or --from/--to is given. Scanning stops after --max-pages pages of 100 records;\n" +
			"when that cap cuts the ledger short the output sets truncated=true and a warning\n" +
			"goes to stderr, so partial totals are never presented as complete.",
		Example:     "  hostex-pp-cli revenue-rollup --by property --month 2026-06 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would query /transactions over the selected range and aggregate income-expense")
				return nil
			}
			if err := rejectLocalDataSource(flags); err != nil {
				return err
			}
			by := strings.ToLower(strings.TrimSpace(flagBy))
			if by == "" {
				by = "property"
			}
			if by != "property" && by != "month" {
				return usageErr(fmt.Errorf("--by must be 'property' or 'month'"))
			}
			maxPages, err := strconv.Atoi(strings.TrimSpace(flagMaxPages))
			if err != nil || maxPages <= 0 {
				return usageErr(fmt.Errorf("--max-pages must be a positive integer"))
			}
			if cliutil.IsDogfoodEnv() {
				maxPages = 1
			}

			// Resolve the [start,end] date range.
			var start, end string
			switch {
			case flagMonth != "":
				mt, err := time.Parse("2006-01", strings.TrimSpace(flagMonth))
				if err != nil {
					return usageErr(fmt.Errorf("--month must be YYYY-MM"))
				}
				start = mt.Format("2006-01-02")
				end = mt.AddDate(0, 1, -1).Format("2006-01-02")
			case flagFrom != "" || flagTo != "":
				if flagFrom == "" || flagTo == "" {
					return usageErr(fmt.Errorf("--from and --to must be supplied together (YYYY-MM-DD)"))
				}
				start, end = strings.TrimSpace(flagFrom), strings.TrimSpace(flagTo)
			default:
				now := nowUTC()
				start = now.AddDate(-1, 0, 0).Format("2006-01-02")
				end = now.Format("2006-01-02")
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type agg struct {
				Group    string  `json:"group"`
				Currency string  `json:"currency,omitempty"`
				Income   float64 `json:"income"`
				Expense  float64 `json:"expense"`
				Net      float64 `json:"net"`
				Count    int     `json:"count"`
			}
			const pageSize = 100
			groups := map[string]*agg{}
			scanned := 0
			reportedTotal := 0
			hitPageCap := false

			for page := 0; page < maxPages; page++ {
				params := map[string]string{
					"start_date": start,
					"end_date":   end,
					"offset":     strconv.Itoa(page * pageSize),
					"limit":      strconv.Itoa(pageSize),
				}
				raw, err := c.Get(ctx, "/transactions", params)
				if err != nil {
					return fmt.Errorf("querying transactions: %w", err)
				}
				var resp struct {
					Transactions []map[string]any `json:"transactions"`
					Total        int              `json:"total"`
				}
				if err := json.Unmarshal(novUnwrapData(raw), &resp); err != nil {
					return fmt.Errorf("decoding transactions page %d: %w", page, err)
				}
				if resp.Total > reportedTotal {
					reportedTotal = resp.Total
				}
				if len(resp.Transactions) == 0 {
					break
				}
				for _, t := range resp.Transactions {
					scanned++
					amount, ok := novNum(t, "amount")
					if !ok {
						continue
					}
					var group string
					if by == "property" {
						group = novStr(t, "property_title")
						if group == "" {
							if pid := novStr(t, "property_id"); pid != "" {
								group = "property_" + pid
							} else {
								group = "(operator-level)"
							}
						}
					} else {
						if mt, mok := novTime(t["action_at"]); mok {
							group = mt.Format("2006-01")
						} else {
							group = "(undated)"
						}
					}
					currency := novStr(t, "currency")
					key := group + "|" + currency
					a := groups[key]
					if a == nil {
						a = &agg{Group: group, Currency: currency}
						groups[key] = a
					}
					if strings.EqualFold(novStr(t, "direction"), "expense") {
						a.Expense += amount
					} else {
						a.Income += amount
					}
					a.Net = a.Income - a.Expense
					a.Count++
				}
				if len(resp.Transactions) < pageSize {
					break
				}
				// A full page on the last allowed iteration means the ledger
				// almost certainly continues past the cap.
				if page == maxPages-1 {
					hitPageCap = true
				}
			}

			truncated := revenueScanTruncated(scanned, reportedTotal, hitPageCap)
			if truncated {
				if reportedTotal > 0 {
					fmt.Fprintf(cmd.ErrOrStderr(),
						"warning: scanned %d of %d transactions before hitting the %d-page cap; income, expense and net are understated. Re-run with a larger --max-pages or a narrower date range.\n",
						scanned, reportedTotal, maxPages)
				} else {
					fmt.Fprintf(cmd.ErrOrStderr(),
						"warning: hit the %d-page cap after %d transactions and the ledger may continue; income, expense and net may be understated. Re-run with a larger --max-pages or a narrower date range.\n",
						maxPages, scanned)
				}
			}

			rows := make([]agg, 0, len(groups))
			for _, a := range groups {
				rows = append(rows, *a)
			}
			sort.SliceStable(rows, func(i, j int) bool {
				return rows[i].Net > rows[j].Net
			})

			// `truncated` is always emitted (no omitempty): an agent reading
			// this JSON must see an explicit false before trusting the totals.
			view := struct {
				By                  string `json:"by"`
				Range               string `json:"range"`
				ScannedTransactions int    `json:"scanned_transactions"`
				TotalTransactions   int    `json:"total_transactions,omitempty"`
				Truncated           bool   `json:"truncated"`
				Groups              []agg  `json:"groups"`
			}{
				By:                  by,
				Range:               start + ".." + end,
				ScannedTransactions: scanned,
				TotalTransactions:   reportedTotal,
				Truncated:           truncated,
				Groups:              rows,
			}
			return novEmit(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&flagBy, "by", "property", "Group by 'property' or 'month'")
	cmd.Flags().StringVar(&flagMonth, "month", "", "Single month, format YYYY-MM (e.g. 2026-06)")
	cmd.Flags().StringVar(&flagFrom, "from", "", "Range start date YYYY-MM-DD (use with --to)")
	cmd.Flags().StringVar(&flagTo, "to", "", "Range end date YYYY-MM-DD (use with --from)")
	cmd.Flags().StringVar(&flagMaxPages, "max-pages", "50", "Max 100-record pages to scan; output sets truncated=true when the cap is hit")
	return cmd
}

// revenueScanTruncated reports whether the ledger scan stopped short of the
// full result set, so the caller never presents partial sums as complete.
// reportedTotal is the API's `total` and is authoritative whenever it is
// present (> 0); Hostex does not always populate it, so hitPageCap — set when
// the last allowed page came back full — is the fallback signal.
func revenueScanTruncated(scanned, reportedTotal int, hitPageCap bool) bool {
	if reportedTotal > 0 {
		return scanned < reportedTotal
	}
	return hitPageCap
}
