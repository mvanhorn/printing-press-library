// Hand-authored (Slice 2): orders unshipped. Reads the local store only.
// pp:data-source local

package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		if p := s2FindChild(root, "orders"); p != nil {
			addNovelCommandIfAbsent(p, newS2OrdersUnshippedCmd(flags))
		}
	})
}

// s2Now is the clock for age calculations (overridden in tests).
var s2Now = time.Now

func newS2OrdersUnshippedCmd(flags *rootFlags) *cobra.Command {
	var full bool
	var limit int
	var dbPath string
	cmd := &cobra.Command{
		Use:   "unshipped",
		Short: "List paid orders that have not shipped yet, oldest first (buyer details hidden unless --full).",
		Long: strings.Trim(`
Reads synced receipts and returns those that are paid but not shipped (canceled
and fully refunded orders are skipped), oldest first, with their age in days. Output is PII-minimized:
receipt id, dates, order total, country, item count and whether the buyer left a
message. Pass --full to include the buyer name, email, buyer_user_id, address fields and message text.
No API calls; run 'etsy-pp-cli sync --resources receipts' first.
`, "\n"),
		Example: strings.Trim(`
  etsy-pp-cli orders unshipped
  etsy-pp-cli orders unshipped --limit 10 --json
  etsy-pp-cli orders unshipped --full
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if full {
				if err := etsyRefuseBuyerDataOverMCP("orders unshipped --full"); err != nil {
					return err
				}
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "orders unshipped")
			}
			if dbPath == "" {
				dbPath = defaultDBPath("etsy-pp-cli")
			}
			rows := make([]map[string]any, 0)
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				return s2PrintOrders(cmd, flags, rows, []string{shopxNoteNoStore})
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()
			notes := shopxCommandNotes(db, "receipts")
			rows, err = s2UnshippedRows(db, full, s2Now())
			if err != nil {
				return err
			}
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			return s2PrintOrders(cmd, flags, rows, notes)
		},
	}
	cmd.Flags().BoolVar(&full, "full", false, "Include buyer name, email, buyer_user_id, address fields and message text")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum orders to return (0 = all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

func s2PrintOrders(cmd *cobra.Command, flags *rootFlags, rows []map[string]any, notes []string) error {
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		shopxPrintNotes(cmd, notes)
		if len(rows) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "no unshipped paid orders")
			return nil
		}
		tw := newTabWriter(cmd.OutOrStdout())
		fmt.Fprintln(tw, "RECEIPT\tAGE(d)\tTOTAL\tCOUNTRY\tITEMS")
		for _, r := range rows {
			fmt.Fprintf(tw, "%v\t%v\t%v %v\t%v\t%v\n", r["receipt_id"], r["age_days"], r["grand_total"], r["currency"], r["country_iso"], r["item_count"])
		}
		return tw.Flush()
	}
	return shopxEmit(cmd, flags, rows, notes)
}

func s2FirstTrue(m map[string]any, keys ...string) (val, present bool) {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			return s2Bool(v), true
		}
	}
	return false, false
}

func s2Int(v any) int64 {
	if n, ok := v.(interface{ Int64() (int64, error) }); ok {
		i, _ := n.Int64()
		return i
	}
	return 0
}

func s2UnshippedRows(db *store.Store, full bool, now time.Time) ([]map[string]any, error) {
	rs, err := db.DB().Query(`SELECT data FROM receipts`)
	if err != nil {
		if syncHintMissingTable(err) {
			return make([]map[string]any, 0), nil
		}
		return nil, err
	}
	defer rs.Close()
	type rec struct {
		ts  int64
		row map[string]any
	}
	recs := make([]rec, 0)
	for rs.Next() {
		var data []byte
		if err := rs.Scan(&data); err != nil {
			return nil, err
		}
		m, err := s2DecodeObject(data)
		if err != nil {
			continue
		}
		if !shopxReceiptNeedsShipping(m) {
			continue
		}
		ts := s2Int(m["created_timestamp"])
		if ts == 0 {
			ts = s2Int(m["create_timestamp"])
		}
		row := map[string]any{
			"receipt_id":  m["receipt_id"],
			"country_iso": m["country_iso"],
			"status":      m["status"],
		}
		if ts > 0 {
			t := time.Unix(ts, 0).UTC()
			row["created"] = t.Format(time.RFC3339)
			row["age_days"] = int(now.Sub(t).Hours() / 24)
		}
		if gt, ok := s2MoneyFloat(m["grandtotal"]); ok {
			row["grand_total"] = gt
			if mm, ok := m["grandtotal"].(map[string]any); ok {
				row["currency"] = mm["currency_code"]
			}
		}
		if tx, ok := m["transactions"].([]any); ok {
			row["item_count"] = len(tx)
		}
		msg := s2Str(m["message_from_buyer"])
		row["has_buyer_message"] = strings.TrimSpace(msg) != ""
		if full {
			for _, k := range []string{"name", "buyer_email", "buyer_user_id", "first_line", "second_line", "city", "state", "zip", "formatted_address", "message_from_buyer"} {
				if v, ok := m[k]; ok {
					row[k] = v
				}
			}
		}
		recs = append(recs, rec{ts, row})
	}
	if err := rs.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(recs, func(i, j int) bool { return recs[i].ts < recs[j].ts })
	out := make([]map[string]any, 0, len(recs))
	for _, r := range recs {
		out = append(out, r.row)
	}
	return out, nil
}

// etsyCLINonBuyerTypes mirrors the MCP allowlist without an import cycle.
var etsyCLINonBuyerTypes = map[string]bool{
	"app":                                 true,
	"buyer_taxonomy":                      true,
	"holiday_preferences":                 true,
	"ledger_entries":                      true,
	"listing_files":                       true,
	"listing_images":                      true,
	"listing_inventory":                   true,
	"listing_translations":                true,
	"listing_variation_images":            true,
	"listing_videos":                      true,
	"listings":                            true,
	"listings-active":                     true,
	"listings-batch":                      true,
	"listings-batch-shipping":             true,
	"listings-featured":                   true,
	"listings_active":                     true,
	"listings_batch":                      true,
	"listings_batch_shipping":             true,
	"listings_featured":                   true,
	"listings_inventory":                  true,
	"marketplace_listings":                true,
	"processing_profiles":                 true,
	"production_partners":                 true,
	"return_policies":                     true,
	"seller_taxonomy":                     true,
	"shipping_profiles":                   true,
	"shipping_profiles-shipping-carriers": true,
	"shipping_profiles_destinations":      true,
	"shipping_profiles_shipping_carriers": true,
	"shop_sections":                       true,
	"shops":                               true,
}

// EtsyNonBuyerResourceTypes returns the sorted CLI mirror of the MCP allowlist.
func EtsyNonBuyerResourceTypes() []string {
	types := make([]string, 0, len(etsyCLINonBuyerTypes))
	for typ := range etsyCLINonBuyerTypes {
		types = append(types, typ)
	}
	sort.Strings(types)
	return types
}

func etsyMCPBuyerPIIOptIn() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("ETSY_MCP_ALLOW_BUYER_PII"))) {
	case "1", "true", "yes":
		return true
	}
	return false
}

func etsyRefuseBuyerDataOverMCP(what string) error {
	if cliutil.IsMCPChildEnv() && !etsyMCPBuyerPIIOptIn() {
		return usageErr(fmt.Errorf("%s exposes buyer data over MCP; explicit opt-in requires ETSY_MCP_ALLOW_BUYER_PII=1", what))
	}
	return nil
}

func etsyBuyerBearingType(t string) bool {
	t = strings.ToLower(strings.TrimSpace(t))
	if t == "" {
		return false
	}
	return !etsyCLINonBuyerTypes[localCanonicalResourceType(t)]
}
