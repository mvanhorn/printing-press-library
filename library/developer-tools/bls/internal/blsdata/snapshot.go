package blsdata

// pp:novel-static-reference
//
// MacroSnapshot returns the curated 15-series list that powers
// `bls-pp-cli snapshot macro`. The set was hand-picked to cover the major
// dimensions of the U.S. macro economy a strategist, journalist, or agent
// typically reaches for on release mornings: prices, labor market, jobs
// turnover, compensation, productivity, and unit labor costs.

// SnapshotEntry is one series in the macro snapshot.
type SnapshotEntry struct {
	ID    string
	Label string
}

// MacroSnapshot returns the 15 series IDs that power `snapshot macro`. Order
// matters — the snapshot prints in this order so the headline indicators
// (CPI, U3, payrolls) land at the top.
func MacroSnapshot() []SnapshotEntry {
	return []SnapshotEntry{
		{ID: "CUSR0000SA0", Label: "CPI-U, all items (SA)"},
		{ID: "CUSR0000SA0L1E", Label: "Core CPI (all items less food and energy, SA)"},
		{ID: "CUSR0000SAF1", Label: "CPI-U, food (SA)"},
		{ID: "CUSR0000SA0E", Label: "CPI-U, energy (SA)"},
		{ID: "CUSR0000SAH1", Label: "CPI-U, shelter (SA)"},
		{ID: "LNS14000000", Label: "Unemployment rate (U-3, SA)"},
		{ID: "LNS11300000", Label: "Labor force participation rate (SA)"},
		{ID: "LNS12300000", Label: "Employment-population ratio (SA)"},
		{ID: "CES0000000001", Label: "Total nonfarm payroll employment (SA)"},
		{ID: "CES0500000003", Label: "Avg hourly earnings, total private (SA)"},
		{ID: "JTS000000000000000JOL", Label: "Job openings, total nonfarm (SA)"},
		{ID: "JTS000000000000000QUR", Label: "Quits rate, total nonfarm (SA)"},
		{ID: "WPSFD4", Label: "PPI, final demand (NSA)"},
		{ID: "CIU1010000000000A", Label: "ECI, total compensation, all civilian (NSA)"},
		{ID: "PRS85006092", Label: "Nonfarm business labor productivity, % chg (SA)"},
	}
}
