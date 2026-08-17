package nsdl

// ParseAUC, ParseTrades, and ParseRegistry all extract the largest data
// table on their respective NSDL report pages into column-name -> value
// maps. Unlike the net-investment series (a single stable layout worth a
// bespoke positional parser), these report families vary in column count
// and grouping, so composite-name extraction generalizes better than a
// hand-tuned struct per report.

// ParseAUC parses an AUC (assets under custody) country-wise or
// category-wise report page (ReportDetail.aspx?RepID=14|18).
func ParseAUC(body []byte) ([]map[string]string, error) {
	return ParseGenericRecords(body)
}

// ParseTrades parses a trade-wise equity or debt report page.
func ParseTrades(body []byte) ([]map[string]string, error) {
	return ParseGenericRecords(body)
}

// ParseRegistry parses the registered-FPI list, category-wise registration
// counts, or DDP pendency report pages.
func ParseRegistry(body []byte) ([]map[string]string, error) {
	return ParseGenericRecords(body)
}
