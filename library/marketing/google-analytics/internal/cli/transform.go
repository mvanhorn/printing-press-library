package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-analytics/internal/ga4"
)

func flattenRows(raw ga4.ReportResponse) []map[string]any {
	out := []map[string]any{}
	for _, rv := range raw.Rows {
		row := map[string]any{}
		for i, h := range raw.DimensionHeaders {
			if i < len(rv.DimensionValues) {
				row[h.Name] = rv.DimensionValues[i].Value
			}
		}
		for i, h := range raw.MetricHeaders {
			if i < len(rv.MetricValues) {
				row[h.Name] = parseNum(rv.MetricValues[i].Value)
			}
		}
		out = append(out, row)
	}
	return out
}
func flattenTotals(raw ga4.ReportResponse) []map[string]any {
	r := raw
	r.Rows = raw.Totals
	r.DimensionHeaders = nil
	return flattenRows(r)
}
func parseNum(s string) any {
	if strings.Contains(s, ".") {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	return s
}
func enrich(rows []map[string]any) []map[string]any {
	for _, r := range rows {
		conv := toFloat(r["conversions"])
		sess := toFloat(r["sessions"])
		rev := toFloat(r["totalRevenue"]) + toFloat(r["purchaseRevenue"])
		trans := toFloat(r["transactions"])
		if sess > 0 && conv >= 0 {
			r["conversion_rate"] = conv / sess
		}
		if trans > 0 && rev > 0 {
			r["aov"] = rev / trans
		}
	}
	return rows
}
func rowKey(r map[string]any, dims []string) string {
	parts := []string{}
	for _, d := range dims {
		parts = append(parts, fmt.Sprint(r[d]))
	}
	return strings.Join(parts, "|")
}
func compareRows(a, b []map[string]any, dims, metrics []string) map[string]any {
	bm := map[string]map[string]any{}
	for _, r := range b {
		bm[rowKey(r, dims)] = r
	}
	rows := []map[string]any{}
	for _, ar := range a {
		key := rowKey(ar, dims)
		br := bm[key]
		out := map[string]any{"key": key}
		for _, d := range dims {
			out[d] = ar[d]
		}
		maxPct := 0.0
		for _, m := range metrics {
			av := toFloat(ar[m])
			bv := toFloat(br[m])
			delta := av - bv
			pct := 0.0
			if bv != 0 {
				pct = delta / bv
			}
			out[m] = map[string]float64{"current": av, "previous": bv, "delta": delta, "pct_change": pct}
			if abs(pct) > abs(maxPct) {
				maxPct = pct
			}
		}
		out["largest_pct_change"] = maxPct
		rows = append(rows, out)
	}
	return map[string]any{"rows": rows, "row_count": len(rows)}
}
func trend(rows []map[string]any, metric string) map[string]any {
	if len(rows) == 0 {
		return map[string]any{}
	}
	first := toFloat(rows[0][metric])
	last := toFloat(rows[len(rows)-1][metric])
	delta := last - first
	pct := 0.0
	if first != 0 {
		pct = delta / first
	}
	return map[string]any{"first": first, "last": last, "delta": delta, "pct_change": pct}
}
func inferPrevious(start, end, period string) (string, string) {
	if ps, pe, ok := inferPreviousAbsolute(start, end); ok {
		return ps, pe
	}
	switch period {
	case "wow":
		return "14daysAgo", "8daysAgo"
	case "mom":
		return "60daysAgo", "31daysAgo"
	default:
		return "14daysAgo", "8daysAgo"
	}
}

func inferPreviousAbsolute(start, end string) (string, string, bool) {
	startDate, err := time.Parse("2006-01-02", start)
	if err != nil {
		return "", "", false
	}
	endDate, err := time.Parse("2006-01-02", end)
	if err != nil || endDate.Before(startDate) {
		return "", "", false
	}
	days := int(endDate.Sub(startDate).Hours()/24) + 1
	previousEnd := startDate.AddDate(0, 0, -1)
	previousStart := previousEnd.AddDate(0, 0, -(days - 1))
	return previousStart.Format("2006-01-02"), previousEnd.Format("2006-01-02"), true
}

func toFloat(v any) float64 {
	switch x := v.(type) {
	case nil:
		return 0
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	case map[string]any:
		return toFloat(x["pct_change"])
	}
	return 0
}
func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
func uniqNonEmpty(xs []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, x := range xs {
		x = cleanProperty(x)
		if x != "" && !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}
func visibleProperties(s ga4.AccountSummariesResponse) []string {
	props := []string{}
	for _, sum := range s.AccountSummaries {
		for _, p := range sum.PropertySummaries {
			if p.Property != "" {
				props = append(props, cleanProperty(p.Property))
			}
		}
	}
	sort.Strings(props)
	return props
}
