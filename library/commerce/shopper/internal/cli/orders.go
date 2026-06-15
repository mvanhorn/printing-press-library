// Copyright 2026 educrvz and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel command: purchase history + month-over-month spend.
//
// PATCH: orders-history. The web "Histórico de compras" page reads
// GET /orders/orders?size=N, an endpoint the generated spec never captured, so
// the CLI had no way to see past orders or actual spend. This command adds it
// and, because order totals are what the customer was charged, rolls them up
// into a month-over-month spend table across one or both storefronts.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/shopper/internal/client"
)

// order is one row of GET /orders/orders. Dates are "dd/mm/yyyy".
type order struct {
	ID           int    `json:"id"`
	DeliveryDate string `json:"deliveryDate"`
	OrderDate    string `json:"orderDate"`
	OrderHour    string `json:"orderHour"`
	Amount       string `json:"amount"`
	Status       string `json:"status"`
}

type ordersResponse struct {
	Last   bool    `json:"last"`
	Orders []order `json:"orders"`
}

func newOrdersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "orders",
		Short: "Purchase history and month-over-month spend",
		Long: `Reads your past orders from GET /orders/orders (the data behind the web
"Histórico de compras" page) and rolls order totals into actual spend.

Store-scoped: pass --store programada|fresh to pick a storefront. 'orders spend'
defaults to reporting both side by side.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newOrdersHistoryCmd(flags))
	cmd.AddCommand(newOrdersSpendCmd(flags))
	return cmd
}

// fetchOrders pulls the order history for whatever store the client's headers
// currently target. size caps the page; the API returns newest-first.
func fetchOrders(cmd *cobra.Command, c *client.Client, size int, flags *rootFlags) (ordersResponse, error) {
	var out ordersResponse
	params := map[string]string{"size": strconv.Itoa(size)}
	data, err := c.Get(cmd.Context(), "/orders/orders", params)
	if err != nil {
		return out, classifyAPIError(err, flags)
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return out, fmt.Errorf("decode orders response: %w", err)
	}
	return out, nil
}

func newOrdersHistoryCmd(flags *rootFlags) *cobra.Command {
	var size int
	cmd := &cobra.Command{
		Use:     "history",
		Aliases: []string{"list", "ls"},
		Short:   "List past orders for a store (date, delivery, amount, status)",
		Example: "  shopper-pp-cli orders history --store fresh --size 20\n  shopper-pp-cli orders history --store programada --json",
		Annotations: map[string]string{
			"mcp:read-only":          "true",
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), `{"dry_run":true,"would":"GET /orders/orders?size=%d","store":%q}`+"\n", size, storeLabel(flags.store))
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, err := fetchOrders(cmd, c, size, flags)
			if err != nil {
				return err
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), resp, flags)
			}
			headers := []string{"ORDER DATE", "DELIVERY", "AMOUNT", "STATUS", "ID"}
			rows := make([][]string, 0, len(resp.Orders))
			for _, o := range resp.Orders {
				rows = append(rows, []string{o.OrderDate, o.DeliveryDate, strings.TrimSpace(stripNBSP(o.Amount)), o.Status, strconv.Itoa(o.ID)})
			}
			if err := flags.printTable(cmd, headers, rows); err != nil {
				return err
			}
			if !resp.Last {
				fmt.Fprintf(cmd.ErrOrStderr(), "\nMore orders exist beyond --size %d; raise it to see older history.\n", size)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&size, "size", 50, "Maximum number of orders to fetch (newest first)")
	return cmd
}

// monthSpend accumulates one calendar month across storefronts.
type monthSpend struct {
	Month      string             `json:"month"` // "2026-04"
	Label      string             `json:"label"` // "Apr/26"
	ByStore    map[string]float64 `json:"by_store"`
	Total      float64            `json:"total"`
	OrderCount int                `json:"order_count"`
}

func newOrdersSpendCmd(flags *rootFlags) *cobra.Command {
	var months int
	var size int
	cmd := &cobra.Command{
		Use:   "spend",
		Short: "Month-over-month actual spend, by store",
		Long: `Sums order totals by the month the order was placed and prints a
month-over-month table. With no --store it queries all four Shopper storefronts
(programada, fresh, pet, unica); stores with no spend in the window are hidden
from the table (and named on stderr) but always present in --json. Pass --store
to scope to one. Spend is bucketed by order date (when you were charged),
matching the web purchase history.`,
		Example: "  shopper-pp-cli orders spend --months 12\n  shopper-pp-cli orders spend --store fresh --months 6 --json",
		Annotations: map[string]string{
			"mcp:read-only":          "true",
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if months <= 0 {
				return usageErr(fmt.Errorf("--months must be positive"))
			}

			// Which storefronts to report. An explicit --store scopes to one;
			// otherwise query all four Shopper storefronts so the report is
			// complete for any account, regardless of which stores they use.
			var storeNames []string
			if flags.store != "" {
				if _, ok := client.ResolveStore(flags.store); !ok {
					return usageErr(fmt.Errorf("unknown --store %q: choose one of %s, or a store id", flags.store, strings.Join(client.StoreNames(), ", ")))
				}
				storeNames = []string{strings.ToLower(flags.store)}
			} else {
				storeNames = client.SpendStoreNames()
			}

			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), `{"dry_run":true,"would":"GET /orders/orders?size=%d per store","stores":%q,"months":%d}`+"\n", size, storeNames, months)
				return nil
			}

			// Build the ordered list of YYYY-MM month buckets ending this month.
			now := time.Now()
			buckets := lastNMonths(now, months)
			idx := map[string]int{}
			spend := make([]monthSpend, len(buckets))
			for i, key := range buckets {
				idx[key] = i
				spend[i] = monthSpend{Month: key, Label: monthLabel(key), ByStore: map[string]float64{}}
			}

			for _, name := range storeNames {
				st, ok := client.ResolveStore(name)
				if !ok {
					return usageErr(fmt.Errorf("unknown store %q", name))
				}
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				client.SetStoreHeaders(c, st)
				resp, err := fetchOrders(cmd, c, size, flags)
				if err != nil {
					return err
				}
				for _, o := range resp.Orders {
					key, ok := monthKeyFromBR(o.OrderDate)
					if !ok {
						continue
					}
					i, in := idx[key]
					if !in {
						continue
					}
					spend[i].ByStore[name] += parseBRL(o.Amount)
					spend[i].Total += parseBRL(o.Amount)
					spend[i].OrderCount++
				}
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), ordersSpendView(storeNames, spend), flags)
			}

			// Per-store totals across the whole window (used to drop empty columns).
			storeTotals := map[string]float64{}
			for _, m := range spend {
				for _, n := range storeNames {
					storeTotals[n] += m.ByStore[n]
				}
			}

			// Human table: show only stores with spend in the window, so an
			// account that uses 2 of the 4 stores still gets a clean 2-column
			// table. The full four-store breakdown is always in --json.
			shown := make([]string, 0, len(storeNames))
			hidden := make([]string, 0, len(storeNames))
			for _, n := range storeNames {
				if storeTotals[n] != 0 {
					shown = append(shown, n)
				} else {
					hidden = append(hidden, n)
				}
			}
			if len(shown) == 0 { // every store empty: keep columns so the table isn't blank
				shown = storeNames
			}

			headers := []string{"MONTH"}
			for _, n := range shown {
				headers = append(headers, strings.ToUpper(n))
			}
			headers = append(headers, "TOTAL", "ORDERS")
			rows := make([][]string, 0, len(spend)+1)
			var grand float64
			for _, m := range spend {
				row := []string{m.Label}
				for _, n := range shown {
					row = append(row, formatBRL(m.ByStore[n]))
				}
				row = append(row, formatBRL(m.Total), strconv.Itoa(m.OrderCount))
				grand += m.Total
				rows = append(rows, row)
			}
			totalRow := []string{"TOTAL"}
			for _, n := range shown {
				totalRow = append(totalRow, formatBRL(storeTotals[n]))
			}
			totalRow = append(totalRow, formatBRL(grand), "")
			rows = append(rows, totalRow)
			if err := flags.printTable(cmd, headers, rows); err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "\nAvg/month over %d months: %s\n", months, formatBRL(grand/float64(months)))
			if len(hidden) > 0 {
				// No silent truncation: name the stores we queried but omitted.
				fmt.Fprintf(cmd.ErrOrStderr(), "No spend over this window for: %s (queried, hidden from table; see --json for all 4).\n", strings.Join(hidden, ", "))
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&months, "months", 12, "Number of months to report, ending this month")
	cmd.Flags().IntVar(&size, "size", 500, "Max orders to fetch per store before bucketing")
	return cmd
}

// ordersSpendView shapes the JSON payload for --json consumers.
func ordersSpendView(stores []string, spend []monthSpend) map[string]any {
	storeTotals := map[string]float64{}
	var grand float64
	for _, m := range spend {
		for _, n := range stores {
			storeTotals[n] += m.ByStore[n]
		}
		grand += m.Total
	}
	avg := map[string]float64{}
	for n, t := range storeTotals {
		avg[n] = t / float64(len(spend))
	}
	return map[string]any{
		"stores":          stores,
		"months":          spend,
		"store_totals":    storeTotals,
		"total":           grand,
		"avg_per_month":   avg,
		"combined_avg":    grand / float64(len(spend)),
		"months_reported": len(spend),
	}
}

// --- small parsing/formatting helpers (BRL + dd/mm/yyyy) ---

func stripNBSP(s string) string { return strings.ReplaceAll(s, " ", " ") }

// parseBRL turns "R$ 2.124,65" into 2124.65. Tolerant of NBSP and missing R$.
func parseBRL(s string) float64 {
	s = stripNBSP(s)
	s = strings.ReplaceAll(s, "R$", "")
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ".", "") // thousands sep
	s = strings.ReplaceAll(s, ",", ".")
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

// formatBRL renders 2124.65 as "2.124,65" (no currency symbol, table-friendly).
func formatBRL(v float64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	s := strconv.FormatFloat(v, 'f', 2, 64)
	intPart, frac := s, "00"
	if i := strings.IndexByte(s, '.'); i >= 0 {
		intPart, frac = s[:i], s[i+1:]
	}
	// group thousands with '.'
	var b strings.Builder
	n := len(intPart)
	for i, ch := range intPart {
		if i > 0 && (n-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(ch)
	}
	out := b.String() + "," + frac
	if neg {
		out = "-" + out
	}
	return out
}

// monthKeyFromBR maps "23/04/2026" -> "2026-04".
func monthKeyFromBR(d string) (string, bool) {
	parts := strings.Split(strings.TrimSpace(d), "/")
	if len(parts) != 3 {
		return "", false
	}
	mm, yyyy := parts[1], parts[2]
	if len(mm) != 2 || len(yyyy) != 4 {
		return "", false
	}
	return yyyy + "-" + mm, true
}

// lastNMonths returns YYYY-MM keys for the n months ending in now's month,
// oldest first.
func lastNMonths(now time.Time, n int) []string {
	keys := make([]string, 0, n)
	y, m := now.Year(), int(now.Month())
	for i := 0; i < n; i++ {
		keys = append(keys, fmt.Sprintf("%04d-%02d", y, m))
		m--
		if m == 0 {
			m = 12
			y--
		}
	}
	sort.Strings(keys)
	return keys
}

var monthAbbr = [...]string{"", "Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

// monthLabel maps "2026-04" -> "Apr/26".
func monthLabel(key string) string {
	parts := strings.Split(key, "-")
	if len(parts) != 2 {
		return key
	}
	m, _ := strconv.Atoi(parts[1])
	if m < 1 || m > 12 {
		return key
	}
	return fmt.Sprintf("%s/%s", monthAbbr[m], parts[0][2:])
}

// storeLabel normalizes an empty --store to its effective default for messages.
func storeLabel(s string) string {
	if s == "" {
		return "programada (default)"
	}
	return s
}
