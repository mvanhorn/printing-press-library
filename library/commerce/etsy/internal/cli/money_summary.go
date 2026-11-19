// Hand-authored (Slice 2): money summary. Reads the local ledger only.
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
		if p := s2FindChild(root, "money"); p != nil {
			addNovelCommandIfAbsent(p, newS2MoneySummaryCmd(flags))
		}
	})
}

// Ledger entry categories. Etsy ledger amounts are signed minor units: credits
// (sales) are positive, debits (fees, refunds, payouts) negative.
const (
	s2CatSale   = "sale"
	s2CatFee    = "fee"
	s2CatRefund = "refund"
	s2CatPayout = "payout"
	s2CatOther  = "other"
)

// s2LedgerCategory classifies an entry from its ledger_type, reference_type and
// description. Order matters: payouts, refunds and fees win over the generic
// "payment" sale match, so "bill payment" and "transaction fee" are fees.
func s2LedgerCategory(ledgerType, referenceType, description string) string {
	s := strings.ToLower(ledgerType + " " + referenceType + " " + description)
	switch {
	case strings.Contains(s, "disburs"):
		return s2CatPayout
	case strings.Contains(s, "refund"):
		return s2CatRefund
	case strings.Contains(s, "fee"), strings.Contains(s, "listing"), strings.Contains(s, "bill"), strings.Contains(s, "renew"):
		return s2CatFee
	case strings.Contains(s, "payment"), strings.Contains(s, "sale"), strings.Contains(s, "transaction"):
		return s2CatSale
	}
	return s2CatOther
}

type s2MoneyTotals struct {
	Period       string `json:"period,omitempty"`
	Currency     string `json:"currency"`
	Divisor      int    `json:"minor_unit_divisor"`
	GrossMinor   int64  `json:"gross_minor"`
	FeesMinor    int64  `json:"fees_minor"`
	RefundsMinor int64  `json:"refunds_minor"`
	NetMinor     int64  `json:"net_minor"`
	PayoutsMinor int64  `json:"payouts_minor"`
	OtherMinor   int64  `json:"other_minor"`
	Entries      int    `json:"entries"`
}

type s2MoneySummary struct {
	Since          string          `json:"since"`
	Until          string          `json:"until"`
	Entries        int             `json:"entries"`
	Totals         []s2MoneyTotals `json:"totals"`
	Periods        []s2MoneyTotals `json:"periods"`
	Classification string          `json:"classification"`
}

var s2ZeroDecimal = map[string]bool{"JPY": true, "KRW": true, "VND": true, "CLP": true, "ISK": true}

func s2Divisor(cur string) int {
	if s2ZeroDecimal[strings.ToUpper(cur)] {
		return 1
	}
	return 100
}

func (t *s2MoneyTotals) add(cat string, amount int64) {
	t.Entries++
	switch cat {
	case s2CatSale:
		t.GrossMinor += amount
	case s2CatFee:
		t.FeesMinor -= amount
	case s2CatRefund:
		t.RefundsMinor -= amount
	case s2CatPayout:
		t.PayoutsMinor -= amount
	default:
		t.OtherMinor += amount
	}
	t.NetMinor = t.GrossMinor - t.FeesMinor - t.RefundsMinor
}

func s2PeriodLabel(t time.Time, by string) string {
	switch by {
	case "day":
		return t.Format("2006-01-02")
	case "week":
		y, w := t.ISOWeek()
		return fmt.Sprintf("%d-W%02d", y, w)
	case "none":
		return ""
	}
	return t.Format("2006-01")
}

func newS2MoneySummaryCmd(flags *rootFlags) *cobra.Command {
	var since, period, dbPath string
	cmd := &cobra.Command{
		Use:   "summary",
		Short: "Period totals from the local ledger: gross, fees, refunds, net and payouts.",
		Long: strings.Trim(`
Totals the synced payment-account ledger over a window. Gross is sales
credited; fees, refunds and payouts are shown as positive amounts taken out;
net is gross minus fees minus refunds (payouts only move money to your bank,
so they are listed separately). Entries are classified by ledger_type,
reference_type and description; anything unrecognized is reported as other
rather than dropped. Amounts are minor units (cents) with the divisor given.
No API calls; run 'etsy-pp-cli sync --resources ledger_entries' first. For a
per-fee-type breakdown use 'money fees'.
`, "\n"),
		Example: strings.Trim(`
  etsy-pp-cli money summary --since 30d
  etsy-pp-cli money summary --since 90d --period week --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "money summary")
			}
			d, err := cliutil.ParseDurationLoose(since)
			if err != nil || d <= 0 {
				return usageErr(fmt.Errorf("--since %q is not a duration like 30d, 12w or 720h", since))
			}
			switch period {
			case "month", "week", "day", "none":
			default:
				return usageErr(fmt.Errorf("--period %q must be month, week, day or none", period))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("etsy-pp-cli")
			}
			now := s2Now().UTC()
			out := s2MoneySummary{Since: now.Add(-d).Format(time.RFC3339), Until: now.Format(time.RFC3339),
				Totals: make([]s2MoneyTotals, 0), Periods: make([]s2MoneyTotals, 0),
				Classification: "sale=payment/sale/transaction credits; fee=fee/listing/bill/renew; refund=refund; payout=disbursement; else other"}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				return shopxEmit(cmd, flags, out, []string{shopxNoteNoStore})
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()
			notes := shopxCommandNotes(db, "ledger_entries")
			if err := s2FillMoneySummary(db, &out, now.Add(-d), now, period); err != nil {
				return err
			}
			if out.Entries == 0 {
				notes = append(notes, "no ledger entries in this window; widen --since or sync ledger_entries")
			}
			return shopxEmit(cmd, flags, out, notes)
		},
	}
	cmd.Flags().StringVar(&since, "since", "30d", "Window to total, e.g. 30d, 12w, 720h")
	cmd.Flags().StringVar(&period, "period", "month", "Group totals by month, week, day or none")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

func s2FillMoneySummary(db *store.Store, out *s2MoneySummary, from, to time.Time, period string) error {
	rs, err := db.DB().Query(`SELECT COALESCE(amount,0), COALESCE(currency,''), COALESCE(ledger_type,''), COALESCE(reference_type,''),
		COALESCE(description,''), COALESCE(create_date, created_timestamp, 0)
		FROM ledger_entries WHERE COALESCE(create_date, created_timestamp, 0) >= ? AND COALESCE(create_date, created_timestamp, 0) <= ?`,
		from.Unix(), to.Unix())
	if err != nil {
		if syncHintMissingTable(err) {
			return nil
		}
		return err
	}
	defer rs.Close()
	byCur := map[string]*s2MoneyTotals{}
	byPer := map[string]*s2MoneyTotals{}
	for rs.Next() {
		var amount, ts int64
		var cur, lt, rt, desc string
		if err := rs.Scan(&amount, &cur, &lt, &rt, &desc, &ts); err != nil {
			return err
		}
		cat := s2LedgerCategory(lt, rt, desc)
		out.Entries++
		t := byCur[cur]
		if t == nil {
			t = &s2MoneyTotals{Currency: cur, Divisor: s2Divisor(cur)}
			byCur[cur] = t
		}
		t.add(cat, amount)
		if period != "none" {
			label := s2PeriodLabel(time.Unix(ts, 0).UTC(), period)
			pk := label + "|" + cur
			p := byPer[pk]
			if p == nil {
				p = &s2MoneyTotals{Period: label, Currency: cur, Divisor: s2Divisor(cur)}
				byPer[pk] = p
			}
			p.add(cat, amount)
		}
	}
	if err := rs.Err(); err != nil {
		return err
	}
	for _, t := range byCur {
		out.Totals = append(out.Totals, *t)
	}
	for _, p := range byPer {
		out.Periods = append(out.Periods, *p)
	}
	sort.Slice(out.Totals, func(i, j int) bool { return out.Totals[i].Currency < out.Totals[j].Currency })
	sort.Slice(out.Periods, func(i, j int) bool {
		if out.Periods[i].Period != out.Periods[j].Period {
			return out.Periods[i].Period < out.Periods[j].Period
		}
		return out.Periods[i].Currency < out.Periods[j].Currency
	})
	return nil
}
