// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package mufap

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/mufap/internal/cliutil"
)

// MUFAP's monthly net-sales pages.
//
// WHY ParseTable CANNOT BE USED HERE. Both net-sales pages put a spanning
// TITLE row inside <thead> ("May 2024 (in PKR millions)"), and the investor
// page adds a second spanning row naming the nine investor classes.
// ParseTable flattens every <th> in the <thead> into one header list, which
// yields 7 headers for the monthly page and 32 for the investor page, then
// matches data rows on that count. The data rows carry 6 and 21 cells, so
// nothing matches and ParseTable returns ZERO ROWS AND NO ERROR. Measured
// 2026-09-08 across five monthly and three investor fixtures: 0 rows on all
// eight. A silent empty result is the worst possible failure shape for a flow
// series, so these pages get their own header resolution: pick the <thead> row
// whose cell count equals the data rows' cell count, and fail loudly when no
// row matches.
//
// Kept in a separate hand-authored file so `generate --force` preserves it.

// netSalesPension holds the two sector labels under which MUFAP renders the
// SAME voluntary-pension figures. Summing both double-counts pension flow.
// Measured: 4,680 rows before de-duplication versus 4,072 after, so the
// duplication is about 13% of all rows.
var netSalesPension = map[string]bool{
	"Pension Funds (Open-End Funds)": true,
	"Employer Pension Funds":         true,
}

// NetSalesRow is one (sector, category) flow row, in PKR millions.
//
// Sales, Redemptions and NetSales are POINTERS because a dash or an empty cell
// on this page means the category did not report -- it does NOT mean zero.
// That is the opposite of CDC's convention, where a dash is a real zero, so
// the distinction is carried in the type rather than left to a comment.
type NetSalesRow struct {
	SectorID    int      `json:"sector_id"`
	Sector      string   `json:"sector"`
	Category    string   `json:"category"`
	Sales       *float64 `json:"sales"`
	Redemptions *float64 `json:"redemptions"`
	NetSales    *float64 `json:"net_sales"`
}

// NetSalesTotal is the sheet's own Total row (SectorId 100).
//
// It is the INVARIANT CHECK, never a data row. Including it in any sum
// double-counts the entire month.
type NetSalesTotal struct {
	Sales       *float64 `json:"sales"`
	Redemptions *float64 `json:"redemptions"`
	NetSales    *float64 `json:"net_sales"`
}

// NetSalesInvariant reconciles the parsed rows against the sheet's Total row.
//
// The tolerance is ROUNDING-AWARE and this matters. MUFAP renders whole PKR
// millions, so each of n contributing rows carries up to +/-0.5 of rounding
// error and so does the total; the residual is therefore bounded by
// 0.5*(n+1) and a fixed tolerance is wrong. Measured over the 25 months that
// carry a Total row: every residual is <= 2.0 while the bound ranges 3.0-7.5,
// so all 25 months reconcile. An earlier fixed 1.5 tolerance failed three of
// them (2025-12, 2026-01, 2026-04) as false alarms -- and 2026-04 failed on
// the REDEMPTION residual while its sales residual was exactly 0, which is why
// both sides are checked and reported separately.
type NetSalesInvariant struct {
	Checked          bool    `json:"checked"`
	OK               bool    `json:"ok"`
	ContributingRows int     `json:"contributing_rows"`
	SumSales         float64 `json:"sum_sales"`
	SumRedemptions   float64 `json:"sum_redemptions"`
	ResidSales       float64 `json:"resid_sales"`
	ResidRedemptions float64 `json:"resid_redemptions"`
	RoundingBound    float64 `json:"rounding_bound"`
	Note             string  `json:"note"`
}

// NetSales is one month of industry flow.
type NetSales struct {
	Period string         `json:"period"`
	Title  string         `json:"title"`
	Rows   []NetSalesRow  `json:"rows"`
	Total  *NetSalesTotal `json:"total,omitempty"`
	// HasTotalRow distinguishes "the sheet had no Total row" from "the Total
	// row was present but every figure was a dash". Both mean there is nothing
	// to reconcile against, but they are different observations: measured
	// 2026-09-08, the empty months (e.g. 2020-06) DO render a Total row as
	// ['100','Total','Total','-','-','-']. Collapsing the two would record a
	// published-but-empty month as a missing page.
	HasTotalRow          bool              `json:"has_total_row"`
	Reporting            int               `json:"reporting"`
	VPSDuplicatesDropped int               `json:"vps_duplicates_dropped"`
	Invariant            NetSalesInvariant `json:"invariant"`
	Note                 string            `json:"note,omitempty"`
}

// NetSalesInvestorRow is one (sector, category, investor class) pair of flows.
//
// There is NO net column on this page. Net must be derived as
// Sales-Redemptions, and only when BOTH are present.
type NetSalesInvestorRow struct {
	SectorID      int      `json:"sector_id"`
	Sector        string   `json:"sector"`
	Category      string   `json:"category"`
	InvestorClass string   `json:"investor_class"`
	Sales         *float64 `json:"sales"`
	Redemptions   *float64 `json:"redemptions"`
}

// NetSalesInvestor is one month of flow split across investor classes.
//
// THIS FEED IS A PARTIAL SLICE. Measured across 25 months, its class totals
// failed to reconcile against the headline month total in 19 of them, so it
// MUST NOT be used to apportion the industry total. It is usable as a
// coverage-matched comparison, which is how it refuted the mutual-fund tax
// channel on that channel's own predicted incidence.
type NetSalesInvestor struct {
	Period    string                `json:"period"`
	Title     string                `json:"title"`
	Classes   []string              `json:"investor_classes"`
	Rows      []NetSalesInvestorRow `json:"rows"`
	Total     []NetSalesInvestorRow `json:"total,omitempty"`
	Reporting int                   `json:"reporting"`
	Note      string                `json:"note,omitempty"`
}

// ErrNetSalesChallenge reports that the response was a Cloudflare interstitial
// rather than the page.
//
// MUFAP challenges these paths INTERMITTENTLY: measured 2026-09-08, 3 of 8
// sequential requests from a plain Go client returned HTTP 403 with a
// "Just a moment..." body, and a retry with backoff cleared it on the third
// attempt. A challenge body has no <thead>, so without this check it would
// parse to zero rows and read as "this month has no data".
var ErrNetSalesChallenge = fmt.Errorf("response was a Cloudflare challenge page, not the net-sales table")

// IsChallengePage reports whether a response body is a Cloudflare interstitial.
func IsChallengePage(doc string) bool {
	return strings.Contains(doc, "_cf_chl_opt") ||
		strings.Contains(doc, "Just a moment") ||
		strings.Contains(doc, "cf-mitigated")
}

// netSalesCells returns the cell text of every <tr> in the document, together
// with whether that row's cells were <td> (a data row) or <th> (a header row).
type netSalesTR struct {
	cells  []string
	isData bool
}

func netSalesRows(doc string) []netSalesTR {
	var out []netSalesTR
	for _, tr := range reTr.FindAllStringSubmatch(doc, -1) {
		if td := reTd.FindAllStringSubmatch(tr[1], -1); len(td) > 0 {
			cells := make([]string, 0, len(td))
			for _, c := range td {
				cells = append(cells, foldSpaces(cliutil.CleanText(stripTags(c[1]))))
			}
			out = append(out, netSalesTR{cells: cells, isData: true})
			continue
		}
		if th := reTh.FindAllStringSubmatch(tr[1], -1); len(th) > 0 {
			cells := make([]string, 0, len(th))
			for _, c := range th {
				cells = append(cells, foldSpaces(cliutil.CleanText(stripTags(c[1]))))
			}
			out = append(out, netSalesTR{cells: cells, isData: false})
		}
	}
	return out
}

// netSalesTitle returns the spanning single-cell header row, which carries the
// month the page actually rendered. Callers compare it against the month they
// asked for: MUFAP serves a page for every (Month, Year) whether or not it has
// data, so the title is the only in-band confirmation of what came back.
func netSalesTitle(rows []netSalesTR) string {
	for _, r := range rows {
		if !r.isData && len(r.cells) == 1 && r.cells[0] != "" {
			return r.cells[0]
		}
	}
	return ""
}

// netSalesDataRows returns the data rows whose first cell is a sector id and
// whose cell count is want.
func netSalesDataRows(rows []netSalesTR, want int) [][]string {
	var out [][]string
	for _, r := range rows {
		if !r.isData || len(r.cells) != want {
			continue
		}
		if _, err := strconv.Atoi(strings.TrimSpace(r.cells[0])); err != nil {
			continue
		}
		out = append(out, r.cells)
	}
	return out
}

// netSalesHeader finds the <thead> row whose cell count is want, so the
// spanning title and class-group rows cannot be mistaken for the header.
func netSalesHeader(rows []netSalesTR, want int) ([]string, bool) {
	for _, r := range rows {
		if !r.isData && len(r.cells) == want {
			return r.cells, true
		}
	}
	return nil, false
}

func optNumber(s string) *float64 {
	v, ok := ParseNumber(s)
	if !ok {
		return nil
	}
	return &v
}

// ParseNetSales parses the monthly net-sales page for one period.
//
// period is the "YYYY-MM" the caller requested; it is recorded on the result
// so a stored row can never be attributed to the wrong month.
func ParseNetSales(doc, period string) (*NetSales, error) {
	if IsChallengePage(doc) {
		return nil, ErrNetSalesChallenge
	}
	all := netSalesRows(doc)
	if len(all) == 0 {
		return nil, fmt.Errorf("no table rows found: page shape changed")
	}
	const wantCells = 6
	if _, ok := netSalesHeader(all, wantCells); !ok {
		return nil, fmt.Errorf("no %d-cell header row found in <thead>: page shape changed", wantCells)
	}

	out := &NetSales{Period: period, Title: netSalesTitle(all), Rows: []NetSalesRow{}}

	var dedup [][]string
	seen := map[string]bool{}
	for _, c := range netSalesDataRows(all, wantCells) {
		sector := c[1]
		if sector == "Total" {
			out.HasTotalRow = true
			out.Total = &NetSalesTotal{
				Sales:       optNumber(c[3]),
				Redemptions: optNumber(c[4]),
				NetSales:    optNumber(c[5]),
			}
			continue
		}
		// For the duplicated pension labels the identity of a row is its
		// category AND its three figures, so the same figures rendered under
		// the second label collapse onto the first. For every other sector the
		// identity is (sector, category), which is the page's real key.
		key := sector + "\x00" + c[2]
		if netSalesPension[sector] {
			key = "PENSION\x00" + c[2] + "\x00" + c[3] + "\x00" + c[4] + "\x00" + c[5]
		}
		if seen[key] {
			out.VPSDuplicatesDropped++
			continue
		}
		seen[key] = true
		dedup = append(dedup, c)
	}

	for _, c := range dedup {
		id, _ := strconv.Atoi(strings.TrimSpace(c[0]))
		row := NetSalesRow{
			SectorID:    id,
			Sector:      c[1],
			Category:    c[2],
			Sales:       optNumber(c[3]),
			Redemptions: optNumber(c[4]),
			NetSales:    optNumber(c[5]),
		}
		if row.NetSales != nil {
			out.Reporting++
		}
		out.Rows = append(out.Rows, row)
	}

	out.Invariant = reconcileNetSales(out.Rows, out.Total)
	switch {
	case !out.HasTotalRow:
		out.Note = "no Total row on this page at all: the page shape may have changed. MUFAP normally renders a Total row even for months it published no data for, so treat this as suspicious rather than as an empty month."
	case out.Total.Sales == nil && out.Total.Redemptions == nil && out.Total.NetSales == nil:
		out.Note = "Total row present but every figure is a dash: MUFAP serves a rendered page for every (Month, Year) whether or not it published data, so this is a month with NO DATA rather than a fetch failure. Measured: 79 of the 104 months from 2018-01 to 2026-08 are empty this way."
	}
	return out, nil
}

// reconcileNetSales sums the data rows and compares them against the Total row
// under a rounding-aware bound. See NetSalesInvariant for the measurement.
func reconcileNetSales(rows []NetSalesRow, total *NetSalesTotal) NetSalesInvariant {
	inv := NetSalesInvariant{}
	var n int
	for _, r := range rows {
		if r.Sales != nil {
			inv.SumSales += *r.Sales
			n++
		}
		if r.Redemptions != nil {
			inv.SumRedemptions += *r.Redemptions
		}
	}
	inv.ContributingRows = n
	if total == nil || total.Sales == nil || total.Redemptions == nil {
		inv.Note = "not checked: this month has no Total row to reconcile against"
		return inv
	}
	inv.Checked = true
	inv.ResidSales = inv.SumSales - *total.Sales
	inv.ResidRedemptions = inv.SumRedemptions - *total.Redemptions
	inv.RoundingBound = 0.5 * float64(n+1)
	inv.OK = abs(inv.ResidSales) <= inv.RoundingBound && abs(inv.ResidRedemptions) <= inv.RoundingBound
	if inv.OK {
		inv.Note = fmt.Sprintf("reconciles: sales residual %.1f and redemption residual %.1f are both within the +/-%.1f rounding bound for %d whole-million rows",
			inv.ResidSales, inv.ResidRedemptions, inv.RoundingBound, n)
	} else {
		inv.Note = fmt.Sprintf("DOES NOT reconcile: sales residual %.1f, redemption residual %.1f, rounding bound +/-%.1f for %d rows. Do not sum these rows as the month total; use the Total row and report the gap.",
			inv.ResidSales, inv.ResidRedemptions, inv.RoundingBound, n)
	}
	return inv
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

// ParseNetSalesInvestor parses the investor-class net-sales page for one period.
//
// The page is 3 key columns followed by one (Sales, Redemptions) pair per
// investor class, so the column count is 3+2*len(classes) and the class names
// come from the spanning group row rather than from the repeated
// "Sales"/"Redemptions" sub-header. Deriving the class list from the group row
// means a class added or removed upstream changes the output rather than
// silently shifting every figure one column left.
func ParseNetSalesInvestor(doc, period string) (*NetSalesInvestor, error) {
	if IsChallengePage(doc) {
		return nil, ErrNetSalesChallenge
	}
	all := netSalesRows(doc)
	if len(all) == 0 {
		return nil, fmt.Errorf("no table rows found: page shape changed")
	}

	classes, err := netSalesInvestorClasses(all)
	if err != nil {
		return nil, err
	}
	wantCells := 3 + 2*len(classes)
	if _, ok := netSalesHeader(all, wantCells); !ok {
		return nil, fmt.Errorf("group row named %d investor classes so the header should carry %d cells, but no <thead> row has that many: page shape changed",
			len(classes), wantCells)
	}

	out := &NetSalesInvestor{
		Period:  period,
		Title:   netSalesTitle(all),
		Classes: classes,
		Rows:    []NetSalesInvestorRow{},
	}
	for _, c := range netSalesDataRows(all, wantCells) {
		id, _ := strconv.Atoi(strings.TrimSpace(c[0]))
		isTotal := c[1] == "Total"
		for i, class := range classes {
			row := NetSalesInvestorRow{
				SectorID:      id,
				Sector:        c[1],
				Category:      c[2],
				InvestorClass: class,
				Sales:         optNumber(c[3+2*i]),
				Redemptions:   optNumber(c[4+2*i]),
			}
			if isTotal {
				out.Total = append(out.Total, row)
				continue
			}
			if row.Sales != nil || row.Redemptions != nil {
				out.Reporting++
			}
			out.Rows = append(out.Rows, row)
		}
	}
	out.Note = "PARTIAL SLICE: this feed carries no net column (derive net as sales-redemptions, and only where both are present), and its class totals failed to reconcile against the headline month total in 19 of 25 measured months. Do NOT apportion the industry total with it."
	return out, nil
}

// netSalesInvestorClasses reads the investor-class names from the spanning
// group row: the header row that is neither the single-cell title nor the
// repeated Sales/Redemptions sub-header.
func netSalesInvestorClasses(rows []netSalesTR) ([]string, error) {
	for _, r := range rows {
		if r.isData || len(r.cells) < 3 {
			continue
		}
		// The group row leads with an empty spanning cell for the three key
		// columns, then names each class once.
		if strings.TrimSpace(r.cells[0]) != "" {
			continue
		}
		var classes []string
		for _, c := range r.cells[1:] {
			c = strings.TrimSpace(c)
			if c == "" || strings.EqualFold(c, "Sales") || strings.EqualFold(c, "Redemptions") {
				continue
			}
			classes = append(classes, c)
		}
		if len(classes) > 0 {
			return classes, nil
		}
	}
	return nil, fmt.Errorf("no investor-class group row found in <thead>: page shape changed")
}

// NetSalesPeriod formats a month and year as the "YYYY-MM" period key.
func NetSalesPeriod(month, year int) string {
	return fmt.Sprintf("%04d-%02d", year, month)
}
