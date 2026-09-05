package cli

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// Currency scoping for flow metrics.
//
// NCCPL publishes both a rupee value and a USD value on the same flow row
// (`net_value` alongside `net_value_USD`), so an unfiltered panel mixes two
// units in one `value` column. --currency keeps one of them.
var nccplUSDMetrics = map[string]bool{"net_value_USD": true, "USD": true}

func nccplMetricMatchesCurrency(metric, currency string) bool {
	switch strings.ToLower(strings.TrimSpace(currency)) {
	case "", "any":
		return true
	case "usd":
		return nccplUSDMetrics[metric]
	case "pkr", "rs", "rupee", "rupees":
		return !nccplUSDMetrics[metric]
	default:
		return true
	}
}

func nccplValidCurrency(currency string) bool {
	switch strings.ToLower(strings.TrimSpace(currency)) {
	case "", "any", "usd", "pkr", "rs", "rupee", "rupees":
		return true
	}
	return false
}

// nccplSortPanel orders rows. "abs-value" is the incumbent dashboards' default
// view: biggest flow first regardless of direction.
func nccplSortPanel(rows []nccplPanelRow, sortBy string) error {
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "", "date":
		sort.SliceStable(rows, func(i, j int) bool {
			if rows[i].Date != rows[j].Date {
				return rows[i].Date < rows[j].Date
			}
			return rows[i].Key < rows[j].Key
		})
	case "value":
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].Value > rows[j].Value })
	case "abs-value", "abs":
		sort.SliceStable(rows, func(i, j int) bool {
			return math.Abs(rows[i].Value) > math.Abs(rows[j].Value)
		})
	case "key":
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].Key < rows[j].Key })
	case "metric":
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].Metric < rows[j].Metric })
	default:
		return fmt.Errorf("unknown --sort %q; valid: date, value, abs-value, key, metric", sortBy)
	}
	return nil
}

// nccplPivotCell is one sector x investor-class cell.
type nccplPivotCell struct {
	Sector string             `json:"sector"`
	By     map[string]float64 `json:"by_investor"`
	Total  float64            `json:"total"`
}

// nccplPivotView is the sector x investor-class matrix the public dashboards render.
type nccplPivotView struct {
	Metric    string           `json:"metric"`
	Investors []string         `json:"investor_classes"`
	Sectors   []nccplPivotCell `json:"sectors"`
	RowsUsed  int              `json:"rows_used"`
	Note      string           `json:"note,omitempty"`
}

// nccplPivotPanel folds sector-wise rows into a sector x investor-class matrix.
//
// The row key for the sector resources is "SEC_CODE|SECTOR_NAME|CLIENT_TYPE", so the
// sector is everything before the final segment and the investor class is the last.
func nccplPivotPanel(rows []nccplPanelRow, metric string) nccplPivotView {
	out := nccplPivotView{Metric: metric, Investors: make([]string, 0), Sectors: make([]nccplPivotCell, 0)}
	bySector := map[string]map[string]float64{}
	investors := map[string]bool{}
	for _, r := range rows {
		if metric != "" && r.Metric != metric {
			continue
		}
		parts := strings.Split(r.Key, "|")
		if len(parts) < 2 {
			continue
		}
		investor := parts[len(parts)-1]
		sector := strings.Join(parts[:len(parts)-1], "|")
		if bySector[sector] == nil {
			bySector[sector] = map[string]float64{}
		}
		bySector[sector][investor] += r.Value
		investors[investor] = true
		out.RowsUsed++
	}
	for i := range investors {
		out.Investors = append(out.Investors, i)
	}
	sort.Strings(out.Investors)
	names := make([]string, 0, len(bySector))
	for s := range bySector {
		names = append(names, s)
	}
	sort.Strings(names)
	for _, s := range names {
		cell := nccplPivotCell{Sector: s, By: bySector[s]}
		for _, v := range bySector[s] {
			cell.Total += v
		}
		out.Sectors = append(out.Sectors, cell)
	}
	if out.RowsUsed == 0 {
		out.Note = "no rows matched; --pivot expects a sector-wise resource (fipi-sector or lipi-sector) and a single --metrics field"
	}
	return out
}
