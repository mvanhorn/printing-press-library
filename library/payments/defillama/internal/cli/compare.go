package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/defillama/internal/format"
	"github.com/mvanhorn/printing-press-library/library/payments/defillama/internal/store"
)

func newCompareCmd() *cobra.Command {
	var metrics, period string
	cmd := &cobra.Command{
		Use:   "compare <protocol> <protocol> [...]",
		Short: "Side-by-side comparison of multiple protocols",
		Args:  cobra.MinimumNArgs(2),
		RunE: withCtx(func(cmd *cobra.Command, args []string, cx *Ctx) error {
			defaultMs := []string{"tvl", "fees", "revenue"}
			ms := defaultMs
			if metrics != "" {
				ms = splitCSV(metrics)
				// validate
				known := map[string]bool{
					"tvl": true, "mcap": true, "fees": true, "revenue": true,
					"volume": true, "change_1d": true, "change_7d": true,
				}
				for _, m := range ms {
					if !known[m] {
						return fmt.Errorf("unknown metric %q (tvl|mcap|fees|revenue|volume|change_1d|change_7d)", m)
					}
				}
			}
			needFees := containsAnySlice(ms, "fees", "revenue")
			needVol := containsAnySlice(ms, "volume")

			deps := []string{"protocols"}
			if needFees {
				deps = append(deps, "fees")
			}
			if needVol {
				deps = append(deps, "dexs")
			}
			if err := ensureFresh(context.Background(), cx, deps); err != nil {
				return err
			}

			headers := []string{"METRIC"}
			slugs := make([]string, 0, len(args))
			for _, a := range args {
				slug, name, err := resolveProtocol(cx.S, a)
				if err != nil {
					return err
				}
				slugs = append(slugs, slug)
				headers = append(headers, strings.ToUpper(name))
			}

			// Historical mode: --period switches to a time-series table.
			if period != "" {
				return runCompareHistory(cx, slugs, headers, ms, period)
			}
			right := map[int]bool{}
			for i := 1; i <= len(slugs); i++ {
				right[i] = true
			}

			rec := format.NewRecorder(headers)

			addRow := func(label string, vals []format.Cell) {
				row := make([]format.Cell, 0, 1+len(vals))
				row = append(row, format.Str(label))
				row = append(row, vals...)
				rec.Append(row)
			}

			for _, m := range ms {
				switch m {
				case "tvl":
					vals := make([]format.Cell, len(slugs))
					for i, s := range slugs {
						var v sql.NullFloat64
						_ = cx.S.QueryRow(`SELECT tvl FROM protocols WHERE slug = ?`, s).Scan(&v)
						vals[i] = format.USD(v.Float64)
					}
					addRow("TVL", vals)
				case "change_1d":
					vals := make([]format.Cell, len(slugs))
					for i, s := range slugs {
						var v sql.NullFloat64
						_ = cx.S.QueryRow(`SELECT change_1d FROM protocols WHERE slug = ?`, s).Scan(&v)
						vals[i] = format.Pct(v.Float64)
					}
					addRow("CHANGE_1D", vals)
				case "change_7d":
					vals := make([]format.Cell, len(slugs))
					for i, s := range slugs {
						var v sql.NullFloat64
						_ = cx.S.QueryRow(`SELECT change_7d FROM protocols WHERE slug = ?`, s).Scan(&v)
						vals[i] = format.Pct(v.Float64)
					}
					addRow("CHANGE_7D", vals)
				case "mcap":
					vals := make([]format.Cell, len(slugs))
					for i, s := range slugs {
						var v sql.NullFloat64
						_ = cx.S.QueryRow(`SELECT mcap FROM protocols WHERE slug = ?`, s).Scan(&v)
						vals[i] = format.USD(v.Float64)
					}
					addRow("MCAP", vals)
				case "fees":
					vals24 := make([]format.Cell, len(slugs))
					vals7 := make([]format.Cell, len(slugs))
					for i, s := range slugs {
						var v24, v7 sql.NullFloat64
						_ = cx.S.QueryRow(`SELECT total_24h_fees, total_7d_fees FROM fees_overview WHERE protocol = ?`, s).Scan(&v24, &v7)
						vals24[i] = format.USD(v24.Float64)
						vals7[i] = format.USD(v7.Float64)
					}
					addRow("FEES_24H", vals24)
					addRow("FEES_7D", vals7)
				case "revenue":
					vals24 := make([]format.Cell, len(slugs))
					vals7 := make([]format.Cell, len(slugs))
					for i, s := range slugs {
						var v24, v7 sql.NullFloat64
						_ = cx.S.QueryRow(`SELECT total_24h_rev, total_7d_rev FROM fees_overview WHERE protocol = ?`, s).Scan(&v24, &v7)
						vals24[i] = format.USD(v24.Float64)
						vals7[i] = format.USD(v7.Float64)
					}
					addRow("REV_24H", vals24)
					addRow("REV_7D", vals7)
				case "volume":
					vals := make([]format.Cell, len(slugs))
					for i, s := range slugs {
						var v sql.NullFloat64
						_ = cx.S.QueryRow(`SELECT total_24h FROM dex_overview WHERE protocol = ?`, s).Scan(&v)
						vals[i] = format.USD(v.Float64)
					}
					addRow("VOL_24H", vals)
				}
			}
			return format.RenderRec(os.Stdout, mode(), rec, format.Options{NoHeader: G.NoHeader, RightAlign: right})
		}),
	}
	cmd.Flags().StringVar(&metrics, "metrics", "", "metrics: comma list of tvl,fees,revenue,volume,mcap,change_1d,change_7d (default tvl,fees,revenue)")
	cmd.Flags().StringVar(&period, "period", "", "time-series mode: 7d|30d|90d|180d|1y|all")
	return cmd
}

// runCompareHistory renders a date x protocol time-series table.
// One row per date, one column per protocol per metric (TVL only for now).
func runCompareHistory(cx *Ctx, slugs []string, baseHeaders []string, metrics []string, period string) error {
	days, err := parsePeriod(period)
	if err != nil {
		return err
	}
	// Ensure each protocol has fresh history.
	for _, s := range slugs {
		if err := ensureProtocolHistory(cx, s); err != nil {
			return err
		}
	}

	// Pick metrics: tvl, fees, revenue, volume. Build column headers as
	// {METRIC}:{PROTOCOL}.
	keepMetrics := []string{}
	for _, m := range metrics {
		switch m {
		case "tvl", "fees", "revenue", "volume":
			keepMetrics = append(keepMetrics, m)
		}
	}
	if len(keepMetrics) == 0 {
		keepMetrics = []string{"tvl"}
	}

	headers := []string{"DATE"}
	for _, m := range keepMetrics {
		for _, s := range slugs {
			headers = append(headers, strings.ToUpper(m+":"+s))
		}
	}
	right := []int{}
	for i := 1; i < len(headers); i++ {
		right = append(right, i)
	}

	// Collect all dates within the period across all selected slugs.
	cutoff := ""
	if days > 0 {
		cutoff = time.Now().AddDate(0, 0, -days).UTC().Format("2006-01-02")
	}
	dateSet := map[string]struct{}{}
	collectDates := func(table, col, slug string) {
		q := fmt.Sprintf("SELECT DISTINCT date FROM %s WHERE %s = ?", table, col)
		args := []any{slug}
		if cutoff != "" {
			q += " AND date >= ?"
			args = append(args, cutoff)
		}
		rows, err := cx.S.Query(q, args...)
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			var d string
			if err := rows.Scan(&d); err == nil {
				dateSet[d] = struct{}{}
			}
		}
	}
	for _, s := range slugs {
		for _, m := range keepMetrics {
			switch m {
			case "tvl":
				collectDates("protocol_tvl_hist", "protocol_slug", s)
			case "fees", "revenue":
				collectDates("fees_hist", "protocol", s)
			case "volume":
				collectDates("dex_hist", "protocol", s)
			}
		}
	}
	dates := make([]string, 0, len(dateSet))
	for d := range dateSet {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	rec := format.NewRecorder(headers)
	for _, d := range dates {
		row := []format.Cell{format.Str(d)}
		for _, m := range keepMetrics {
			for _, s := range slugs {
				v := lookupHistorical(cx, m, s, d)
				if m == "tvl" || m == "fees" || m == "revenue" || m == "volume" {
					row = append(row, format.USD(v))
				} else {
					row = append(row, format.Raw(v))
				}
			}
		}
		rec.Append(row)
	}
	return format.RenderRec(os.Stdout, mode(), rec, format.Options{NoHeader: G.NoHeader, RightAlign: rightSet(right)})
}

func lookupHistorical(cx *Ctx, metric, slug, date string) float64 {
	var v sql.NullFloat64
	switch metric {
	case "tvl":
		_ = cx.S.QueryRow(`SELECT tvl FROM protocol_tvl_hist WHERE protocol_slug = ? AND date = ? AND chain = '' LIMIT 1`, slug, date).Scan(&v)
	case "fees":
		_ = cx.S.QueryRow(`SELECT fees FROM fees_hist WHERE protocol = ? AND date = ?`, slug, date).Scan(&v)
	case "revenue":
		_ = cx.S.QueryRow(`SELECT revenue FROM fees_hist WHERE protocol = ? AND date = ?`, slug, date).Scan(&v)
	case "volume":
		_ = cx.S.QueryRow(`SELECT volume FROM dex_hist WHERE protocol = ? AND date = ?`, slug, date).Scan(&v)
	}
	return v.Float64
}

// ensureProtocolHistory triggers sync --protocol if the local history is stale
// or missing. Respects --no-sync.
func ensureProtocolHistory(cx *Ctx, slug string) error {
	if G.NoSync {
		return nil
	}
	stale, err := cx.S.StaleBefore("protocol_tvl_hist:"+slug, cx.Cfg.StaleHistoricalDur())
	if err != nil {
		return err
	}
	if !stale {
		return nil
	}
	fmt.Fprintf(os.Stderr, "syncing historical for %s...\n", slug)
	cx.Eng.Report = func(d, msg string) { fmt.Fprintf(os.Stderr, "  [%s] %s\n", d, msg) }
	return cx.Eng.SyncProtocolDetail(context.Background(), slug, false)
}

// protocolAliases bridges retired protocol names to their current slugs.
// DefiLlama scrubs old names from the API after a rebrand, so we keep a small
// table here. Add entries when a rebrand breaks existing skill prompts.
var protocolAliases = map[string]string{
	"eigenlayer": "eigencloud",
}

// resolveProtocol fuzzy-matches user input to a protocol slug.
// Resolution order:
//  1. exact slug match (case-insensitive)
//  2. known rebrand alias
//  3. exact name match
//  4. fuzzy LIKE on slug and name; highest-TVL match wins (with stderr note)
func resolveProtocol(s *store.Store, q string) (slug, name string, err error) {
	q = strings.TrimSpace(q)
	lower := strings.ToLower(q)
	// Pass 1: exact slug
	if err := s.QueryRow(`SELECT slug, name FROM protocols WHERE LOWER(slug) = ?`, lower).Scan(&slug, &name); err == nil {
		return slug, name, nil
	}
	// Pass 2: known rebrand alias
	if alias, ok := protocolAliases[lower]; ok {
		if err := s.QueryRow(`SELECT slug, name FROM protocols WHERE LOWER(slug) = ?`, alias).Scan(&slug, &name); err == nil {
			return slug, name, nil
		}
	}
	// Pass 2: fuzzy LIKE on slug + name (substring, case-insensitive). Order by TVL.
	rows, qerr := s.Query(`SELECT slug, name, COALESCE(tvl,0) FROM protocols
		WHERE LOWER(slug) LIKE ? OR LOWER(name) LIKE ?
		ORDER BY tvl DESC LIMIT 10`, "%"+lower+"%", "%"+lower+"%")
	if qerr != nil {
		return "", "", qerr
	}
	defer rows.Close()
	type match struct {
		slug, name string
		tvl        float64
	}
	matches := []match{}
	for rows.Next() {
		var m match
		if err := rows.Scan(&m.slug, &m.name, &m.tvl); err != nil {
			return "", "", err
		}
		matches = append(matches, m)
	}
	if len(matches) == 0 {
		return "", "", fmt.Errorf("protocol %q not found (try `sync` first?)", q)
	}
	// Check for exact name match in pass 2 results first.
	for _, m := range matches {
		if strings.ToLower(m.name) == lower {
			return m.slug, m.name, nil
		}
	}
	// Single unambiguous fuzzy match: use it silently.
	if len(matches) == 1 {
		return matches[0].slug, matches[0].name, nil
	}
	// Multiple matches: prefer the highest-TVL one but warn.
	primary := matches[0]
	others := make([]string, 0, len(matches)-1)
	for _, m := range matches[1:] {
		others = append(others, m.slug)
	}
	fmt.Fprintf(os.Stderr, "note: %q matched %s (also: %s)\n", q, primary.slug, strings.Join(others, ", "))
	return primary.slug, primary.name, nil
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(strings.ToLower(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func containsAnySlice(haystack []string, needles ...string) bool {
	for _, h := range haystack {
		for _, n := range needles {
			if h == n {
				return true
			}
		}
	}
	return false
}

