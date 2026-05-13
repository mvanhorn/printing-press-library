// Package blsdata holds curated static BLS reference data that the live API
// does not expose: the most-popular series catalog, the release calendar,
// footnote codes, the seasonal-adjustment decoder, and the macro snapshot
// indicator list. Marked `// pp:novel-static-reference`.
// PATCH: hand-authored novel-feature file. See .printing-press-patches.json patch id "novel-blsdata-package".
package blsdata

// pp:novel-static-reference

// CatalogEntry is one row in the curated BLS series catalog.
type CatalogEntry struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Survey string `json:"survey"`
	Area   string `json:"area,omitempty"`
	Item   string `json:"item,omitempty"`
	Units  string `json:"units,omitempty"`
	Adjust string `json:"adjust,omitempty"` // seasonal | nsa
}

// Catalog returns the curated catalog of well-known BLS series. This is a
// hand-picked subset of the most useful indicators, covering CPI components
// and metro areas, headline labor-market indicators, JOLTS, PPI, ECI, and
// productivity. Series IDs are valid against the live BLS API.
//
// The list is deliberately small (~120 entries) and is used to seed the
// local FTS5 search index so `series search` works out of the box without
// the user running a multi-megabyte flat-file import. Power users who need
// the full catalog can layer `printing-press` flat-file ingestion on top.
func Catalog() []CatalogEntry {
	return []CatalogEntry{
		// CPI-U headline and components (CUUR = NSA, CUSR = SA)
		{ID: "CUUR0000SA0", Title: "CPI-U all items, U.S. city average, NSA", Survey: "CU", Area: "U.S. city average", Item: "All items", Units: "Index 1982-84=100", Adjust: "nsa"},
		{ID: "CUSR0000SA0", Title: "CPI-U all items, U.S. city average, SA", Survey: "CU", Area: "U.S. city average", Item: "All items", Units: "Index 1982-84=100", Adjust: "seasonal"},
		{ID: "CUUR0000SA0L1E", Title: "CPI-U all items less food and energy (core CPI), NSA", Survey: "CU", Area: "U.S. city average", Item: "All items less food and energy", Units: "Index 1982-84=100", Adjust: "nsa"},
		{ID: "CUSR0000SA0L1E", Title: "CPI-U core CPI (all items less food and energy), SA", Survey: "CU", Area: "U.S. city average", Item: "All items less food and energy", Units: "Index 1982-84=100", Adjust: "seasonal"},
		{ID: "CUUR0000SAF1", Title: "CPI-U food, NSA", Survey: "CU", Area: "U.S. city average", Item: "Food", Units: "Index 1982-84=100", Adjust: "nsa"},
		{ID: "CUSR0000SAF1", Title: "CPI-U food, SA", Survey: "CU", Area: "U.S. city average", Item: "Food", Units: "Index 1982-84=100", Adjust: "seasonal"},
		{ID: "CUUR0000SAF11", Title: "CPI-U food at home, NSA", Survey: "CU", Area: "U.S. city average", Item: "Food at home", Adjust: "nsa"},
		{ID: "CUUR0000SEFV", Title: "CPI-U food away from home, NSA", Survey: "CU", Area: "U.S. city average", Item: "Food away from home", Adjust: "nsa"},
		{ID: "CUUR0000SA0E", Title: "CPI-U energy, NSA", Survey: "CU", Area: "U.S. city average", Item: "Energy", Adjust: "nsa"},
		{ID: "CUSR0000SA0E", Title: "CPI-U energy, SA", Survey: "CU", Area: "U.S. city average", Item: "Energy", Adjust: "seasonal"},
		{ID: "CUUR0000SETA01", Title: "CPI-U gasoline (all types), NSA", Survey: "CU", Area: "U.S. city average", Item: "Gasoline, all types", Adjust: "nsa"},
		{ID: "CUUR0000SAH", Title: "CPI-U housing, NSA", Survey: "CU", Area: "U.S. city average", Item: "Housing", Adjust: "nsa"},
		{ID: "CUUR0000SAH1", Title: "CPI-U shelter, NSA", Survey: "CU", Area: "U.S. city average", Item: "Shelter", Adjust: "nsa"},
		{ID: "CUSR0000SAH1", Title: "CPI-U shelter, SA", Survey: "CU", Area: "U.S. city average", Item: "Shelter", Adjust: "seasonal"},
		{ID: "CUUR0000SEHA", Title: "CPI-U rent of primary residence, NSA", Survey: "CU", Area: "U.S. city average", Item: "Rent of primary residence", Adjust: "nsa"},
		{ID: "CUUR0000SEHC", Title: "CPI-U owners' equivalent rent of residences, NSA", Survey: "CU", Area: "U.S. city average", Item: "Owners' equivalent rent of residences", Adjust: "nsa"},
		{ID: "CUUR0000SAM", Title: "CPI-U medical care, NSA", Survey: "CU", Area: "U.S. city average", Item: "Medical care", Adjust: "nsa"},
		{ID: "CUSR0000SAM", Title: "CPI-U medical care, SA", Survey: "CU", Area: "U.S. city average", Item: "Medical care", Adjust: "seasonal"},
		{ID: "CUUR0000SETB01", Title: "CPI-U airline fares, NSA", Survey: "CU", Area: "U.S. city average", Item: "Airline fares", Adjust: "nsa"},
		{ID: "CUUR0000SETA02", Title: "CPI-U new vehicles, NSA", Survey: "CU", Area: "U.S. city average", Item: "New vehicles", Adjust: "nsa"},
		{ID: "CUUR0000SETA03", Title: "CPI-U used cars and trucks, NSA", Survey: "CU", Area: "U.S. city average", Item: "Used cars and trucks", Adjust: "nsa"},

		// CPI-U metro areas (selected major MSAs)
		{ID: "CUURA101SA0", Title: "CPI-U all items, Northeast, NSA", Survey: "CU", Area: "Northeast", Item: "All items", Adjust: "nsa"},
		{ID: "CUURA102SA0", Title: "CPI-U all items, Midwest, NSA", Survey: "CU", Area: "Midwest", Item: "All items", Adjust: "nsa"},
		{ID: "CUURA103SA0", Title: "CPI-U all items, South, NSA", Survey: "CU", Area: "South", Item: "All items", Adjust: "nsa"},
		{ID: "CUURA104SA0", Title: "CPI-U all items, West, NSA", Survey: "CU", Area: "West", Item: "All items", Adjust: "nsa"},
		{ID: "CUURA421SA0", Title: "CPI-U all items, Los Angeles-Long Beach-Anaheim, NSA", Survey: "CU", Area: "Los Angeles-Long Beach-Anaheim, CA", Item: "All items", Adjust: "nsa"},
		{ID: "CUURA101SAH1", Title: "CPI-U shelter, Northeast, NSA", Survey: "CU", Area: "Northeast", Item: "Shelter", Adjust: "nsa"},
		{ID: "CUURS35ASA0", Title: "CPI-U all items, San Francisco-Oakland-Hayward, NSA", Survey: "CU", Area: "San Francisco-Oakland-Hayward, CA", Item: "All items", Adjust: "nsa"},
		{ID: "CUURS49ASA0", Title: "CPI-U all items, Seattle-Tacoma-Bellevue, NSA", Survey: "CU", Area: "Seattle-Tacoma-Bellevue, WA", Item: "All items", Adjust: "nsa"},
		{ID: "CUURS12ASA0", Title: "CPI-U all items, New York-Newark-Jersey City, NSA", Survey: "CU", Area: "New York-Newark-Jersey City, NY-NJ-PA", Item: "All items", Adjust: "nsa"},
		{ID: "CUURS23ASA0", Title: "CPI-U all items, Chicago-Naperville-Elgin, NSA", Survey: "CU", Area: "Chicago-Naperville-Elgin, IL-IN-WI", Item: "All items", Adjust: "nsa"},
		{ID: "CUURS37ASA0", Title: "CPI-U all items, Atlanta-Sandy Springs-Roswell, NSA", Survey: "CU", Area: "Atlanta-Sandy Springs-Roswell, GA", Item: "All items", Adjust: "nsa"},
		{ID: "CUURS11ASA0", Title: "CPI-U all items, Boston-Cambridge-Newton, NSA", Survey: "CU", Area: "Boston-Cambridge-Newton, MA-NH", Item: "All items", Adjust: "nsa"},
		{ID: "CUURS24ASA0", Title: "CPI-U all items, Detroit-Warren-Dearborn, NSA", Survey: "CU", Area: "Detroit-Warren-Dearborn, MI", Item: "All items", Adjust: "nsa"},
		{ID: "CUURS48BSA0", Title: "CPI-U all items, Phoenix-Mesa-Scottsdale, NSA", Survey: "CU", Area: "Phoenix-Mesa-Scottsdale, AZ", Item: "All items", Adjust: "nsa"},

		// CPI-W (urban wage earners)
		{ID: "CWUR0000SA0", Title: "CPI-W all items, NSA", Survey: "CW", Area: "U.S. city average", Item: "All items", Adjust: "nsa"},
		{ID: "CWUR0000SA0L1E", Title: "CPI-W all items less food and energy, NSA", Survey: "CW", Area: "U.S. city average", Item: "Core (less food and energy)", Adjust: "nsa"},

		// Employment situation - national payrolls (CES)
		{ID: "CES0000000001", Title: "Total nonfarm payroll employment, SA", Survey: "CE", Area: "U.S.", Item: "Total nonfarm employment", Units: "Thousands of persons", Adjust: "seasonal"},
		{ID: "CES0500000001", Title: "Total private payroll employment, SA", Survey: "CE", Area: "U.S.", Item: "Total private employment", Adjust: "seasonal"},
		{ID: "CES0500000002", Title: "Average weekly hours of all employees, total private, SA", Survey: "CE", Area: "U.S.", Item: "Avg weekly hours, total private", Adjust: "seasonal"},
		{ID: "CES0500000003", Title: "Average hourly earnings of all employees, total private, SA", Survey: "CE", Area: "U.S.", Item: "Avg hourly earnings, total private", Units: "Dollars per hour", Adjust: "seasonal"},
		{ID: "CES0500000008", Title: "Average hourly earnings of production and nonsupervisory employees, total private, SA", Survey: "CE", Area: "U.S.", Item: "Avg hourly earnings, production/nonsupervisory", Adjust: "seasonal"},
		{ID: "CES1000000001", Title: "Mining and logging payroll employment, SA", Survey: "CE", Area: "U.S.", Item: "Mining and logging", Adjust: "seasonal"},
		{ID: "CES2000000001", Title: "Construction payroll employment, SA", Survey: "CE", Area: "U.S.", Item: "Construction", Adjust: "seasonal"},
		{ID: "CES3000000001", Title: "Manufacturing payroll employment, SA", Survey: "CE", Area: "U.S.", Item: "Manufacturing", Adjust: "seasonal"},
		{ID: "CES4000000001", Title: "Trade, transportation, and utilities employment, SA", Survey: "CE", Area: "U.S.", Item: "Trade, transportation, utilities", Adjust: "seasonal"},
		{ID: "CES5000000001", Title: "Information sector payroll employment, SA", Survey: "CE", Area: "U.S.", Item: "Information", Adjust: "seasonal"},
		{ID: "CES5500000001", Title: "Financial activities payroll employment, SA", Survey: "CE", Area: "U.S.", Item: "Financial activities", Adjust: "seasonal"},
		{ID: "CES6000000001", Title: "Professional and business services payroll employment, SA", Survey: "CE", Area: "U.S.", Item: "Professional and business services", Adjust: "seasonal"},
		{ID: "CES6500000001", Title: "Education and health services payroll employment, SA", Survey: "CE", Area: "U.S.", Item: "Education and health services", Adjust: "seasonal"},
		{ID: "CES7000000001", Title: "Leisure and hospitality payroll employment, SA", Survey: "CE", Area: "U.S.", Item: "Leisure and hospitality", Adjust: "seasonal"},
		{ID: "CES8000000001", Title: "Other services payroll employment, SA", Survey: "CE", Area: "U.S.", Item: "Other services", Adjust: "seasonal"},
		{ID: "CES9000000001", Title: "Government payroll employment, SA", Survey: "CE", Area: "U.S.", Item: "Government", Adjust: "seasonal"},

		// Unemployment / labor force (CPS, LN prefix)
		{ID: "LNS14000000", Title: "Unemployment rate (U-3), SA", Survey: "LN", Area: "U.S.", Item: "Unemployment rate (U-3)", Units: "Percent", Adjust: "seasonal"},
		{ID: "LNS14000003", Title: "Unemployment rate, White, SA", Survey: "LN", Area: "U.S.", Item: "Unemployment rate, White", Adjust: "seasonal"},
		{ID: "LNS14000006", Title: "Unemployment rate, Black or African American, SA", Survey: "LN", Area: "U.S.", Item: "Unemployment rate, Black", Adjust: "seasonal"},
		{ID: "LNS14000009", Title: "Unemployment rate, Hispanic or Latino, SA", Survey: "LN", Area: "U.S.", Item: "Unemployment rate, Hispanic", Adjust: "seasonal"},
		{ID: "LNS14000012", Title: "Unemployment rate, 16-19 years, SA", Survey: "LN", Area: "U.S.", Item: "Unemployment rate, 16-19 years", Adjust: "seasonal"},
		{ID: "LNS14000024", Title: "Unemployment rate, 25-54 years, SA", Survey: "LN", Area: "U.S.", Item: "Unemployment rate, 25-54 years", Adjust: "seasonal"},
		{ID: "LNS12000000", Title: "Civilian employment level, SA", Survey: "LN", Area: "U.S.", Item: "Civilian employment level", Units: "Thousands", Adjust: "seasonal"},
		{ID: "LNS13000000", Title: "Civilian unemployment level, SA", Survey: "LN", Area: "U.S.", Item: "Civilian unemployment level", Adjust: "seasonal"},
		{ID: "LNS11000000", Title: "Civilian labor force level, SA", Survey: "LN", Area: "U.S.", Item: "Civilian labor force level", Adjust: "seasonal"},
		{ID: "LNS11300000", Title: "Civilian labor force participation rate, SA", Survey: "LN", Area: "U.S.", Item: "Labor force participation rate", Units: "Percent", Adjust: "seasonal"},
		{ID: "LNS12300000", Title: "Employment-population ratio, SA", Survey: "LN", Area: "U.S.", Item: "Employment-population ratio", Units: "Percent", Adjust: "seasonal"},
		{ID: "LNS13327709", Title: "Unemployment rate (U-6, broadest), SA", Survey: "LN", Area: "U.S.", Item: "Unemployment rate, U-6", Adjust: "seasonal"},

		// LAUS state-level unemployment (selected states; full set lives in flat files)
		{ID: "LASST060000000000003", Title: "California unemployment rate, SA", Survey: "LA", Area: "California", Item: "Unemployment rate", Adjust: "seasonal"},
		{ID: "LASST360000000000003", Title: "New York unemployment rate, SA", Survey: "LA", Area: "New York", Item: "Unemployment rate", Adjust: "seasonal"},
		{ID: "LASST480000000000003", Title: "Texas unemployment rate, SA", Survey: "LA", Area: "Texas", Item: "Unemployment rate", Adjust: "seasonal"},
		{ID: "LASST120000000000003", Title: "Florida unemployment rate, SA", Survey: "LA", Area: "Florida", Item: "Unemployment rate", Adjust: "seasonal"},
		{ID: "LASST170000000000003", Title: "Illinois unemployment rate, SA", Survey: "LA", Area: "Illinois", Item: "Unemployment rate", Adjust: "seasonal"},
		{ID: "LASST420000000000003", Title: "Pennsylvania unemployment rate, SA", Survey: "LA", Area: "Pennsylvania", Item: "Unemployment rate", Adjust: "seasonal"},
		{ID: "LASST390000000000003", Title: "Ohio unemployment rate, SA", Survey: "LA", Area: "Ohio", Item: "Unemployment rate", Adjust: "seasonal"},
		{ID: "LASST530000000000003", Title: "Washington unemployment rate, SA", Survey: "LA", Area: "Washington", Item: "Unemployment rate", Adjust: "seasonal"},

		// JOLTS - openings, hires, separations
		{ID: "JTS000000000000000JOL", Title: "Job openings, total nonfarm, SA", Survey: "JT", Area: "U.S.", Item: "Job openings", Units: "Thousands", Adjust: "seasonal"},
		{ID: "JTS000000000000000HIL", Title: "Hires, total nonfarm, SA", Survey: "JT", Area: "U.S.", Item: "Hires", Adjust: "seasonal"},
		{ID: "JTS000000000000000TSL", Title: "Total separations, total nonfarm, SA", Survey: "JT", Area: "U.S.", Item: "Total separations", Adjust: "seasonal"},
		{ID: "JTS000000000000000QUL", Title: "Quits, total nonfarm, SA", Survey: "JT", Area: "U.S.", Item: "Quits", Adjust: "seasonal"},
		{ID: "JTS000000000000000LDL", Title: "Layoffs and discharges, total nonfarm, SA", Survey: "JT", Area: "U.S.", Item: "Layoffs and discharges", Adjust: "seasonal"},
		{ID: "JTS000000000000000JOR", Title: "Job openings rate, total nonfarm, SA", Survey: "JT", Area: "U.S.", Item: "Job openings rate", Units: "Rate", Adjust: "seasonal"},
		{ID: "JTS000000000000000QUR", Title: "Quits rate, total nonfarm, SA", Survey: "JT", Area: "U.S.", Item: "Quits rate", Adjust: "seasonal"},

		// PPI - finished goods, commodities
		{ID: "WPSFD4", Title: "PPI final demand, NSA", Survey: "WP", Area: "U.S.", Item: "Final demand", Units: "Index Nov 2009=100", Adjust: "nsa"},
		{ID: "WPSFD49207", Title: "PPI final demand less foods and energy (core PPI), NSA", Survey: "WP", Area: "U.S.", Item: "Final demand less foods and energy", Adjust: "nsa"},
		{ID: "PCU3361113361111", Title: "PPI industry: automobile manufacturing, NSA", Survey: "PC", Area: "U.S.", Item: "Automobile manufacturing", Adjust: "nsa"},

		// ECI - Employment Cost Index
		{ID: "CIU1010000000000A", Title: "ECI total compensation, all civilian workers, NSA", Survey: "CI", Area: "U.S.", Item: "Total compensation, all civilian", Adjust: "nsa"},
		{ID: "CIU2010000000000A", Title: "ECI wages and salaries, all civilian workers, NSA", Survey: "CI", Area: "U.S.", Item: "Wages and salaries, all civilian", Adjust: "nsa"},
		{ID: "CIS1010000000000I", Title: "ECI total compensation, all civilian, SA index", Survey: "CI", Area: "U.S.", Item: "Total compensation, SA index", Adjust: "seasonal"},

		// Productivity (PR)
		{ID: "PRS85006092", Title: "Nonfarm business labor productivity, % change from previous quarter, SA", Survey: "PR", Area: "U.S.", Item: "Nonfarm business labor productivity (% chg)", Adjust: "seasonal"},
		{ID: "PRS85006112", Title: "Nonfarm business unit labor costs, % change from previous quarter, SA", Survey: "PR", Area: "U.S.", Item: "Nonfarm business unit labor costs (% chg)", Adjust: "seasonal"},

		// Average prices (AP) - gasoline, milk, eggs
		{ID: "APU000074714", Title: "Average price: gasoline, unleaded regular, per gallon, NSA", Survey: "AP", Area: "U.S. city average", Item: "Gasoline, unleaded regular", Units: "USD/gallon", Adjust: "nsa"},
		{ID: "APU0000709112", Title: "Average price: milk, fresh, whole, fortified, per gallon, NSA", Survey: "AP", Area: "U.S. city average", Item: "Milk, whole, gallon", Adjust: "nsa"},
		{ID: "APU0000708111", Title: "Average price: eggs, grade A, large, per dozen, NSA", Survey: "AP", Area: "U.S. city average", Item: "Eggs, grade A, large", Adjust: "nsa"},
		{ID: "APU0000702111", Title: "Average price: bread, white, per pound, NSA", Survey: "AP", Area: "U.S. city average", Item: "Bread, white, lb", Adjust: "nsa"},
		{ID: "APU0000711211", Title: "Average price: ground beef, 100% beef, per pound, NSA", Survey: "AP", Area: "U.S. city average", Item: "Ground beef, lb", Adjust: "nsa"},
	}
}

// FindByID returns the catalog entry for a series ID, or nil if not found.
func FindByID(id string) *CatalogEntry {
	for i, e := range Catalog() {
		if e.ID == id {
			return &Catalog()[i]
		}
	}
	return nil
}
