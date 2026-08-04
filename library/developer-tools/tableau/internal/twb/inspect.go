package twb

// Summary is a structured overview of a workbook.
type Summary struct {
	Path            string   `json:"path"`
	SheetCount      int      `json:"sheet_count"`
	Sheets          []string `json:"sheets"`
	DashboardCount  int      `json:"dashboard_count"`
	Dashboards      []string `json:"dashboards"`
	ZoneCount       int      `json:"zone_count"`
	DatasourceCount int      `json:"datasource_count"`
	Datasources     []string `json:"datasources"`
	CalcCount       int      `json:"calc_count"`
}

// Inspect returns a structural summary of the workbook.
func (w *Workbook) Inspect() Summary {
	s := Summary{Path: w.path}

	for _, name := range w.ListSheets() {
		s.Sheets = append(s.Sheets, name)
	}
	s.SheetCount = len(s.Sheets)

	for _, d := range w.ListDashboards() {
		s.Dashboards = append(s.Dashboards, d.Name)
		s.ZoneCount += d.ZoneCount
	}
	s.DashboardCount = len(s.Dashboards)

	dsParent := w.datasourcesParent()
	if dsParent != nil {
		for _, ds := range dsParent.SelectElements("datasource") {
			label := ds.SelectAttrValue("caption", "")
			if label == "" {
				label = ds.SelectAttrValue("name", "")
			}
			s.Datasources = append(s.Datasources, label)
		}
	}
	s.DatasourceCount = len(s.Datasources)
	s.CalcCount = len(w.ListCalcs())
	return s
}
