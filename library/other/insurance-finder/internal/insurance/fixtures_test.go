package insurance

import "strings"

// containsFold reports whether substr is within s, case-insensitively.
func containsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// importerProfile models any importer / private-label / manufacturer (the
// "deemed manufacturer" class). This is the profile that MUST be routed to
// specialty markets, regardless of industry.
func importerProfile() Profile {
	return Profile{
		LegalName:                  "Acme Imports LLC",
		EntityStructure:            "LLC",
		FormationState:             "Delaware",
		BusinessAddress:            "100 Commerce Way, Springfield, IL 62701",
		ContactName:                "Jane Doe",
		ContactEmail:               "owner@example.com",
		ContactPhone:               "(312) 555-0100",
		YearStarted:                2023,
		RevenueBand:                "$100k-$175k",
		EmployeesW2:                0,
		Contractors1099:            1,
		IndustryClass:              "Consumer-goods importer/distributor",
		Importer:                   true,
		PrivateLabel:               true,
		Manufacturer:               true,
		Products:                   "Houseware and accessory products",
		CountriesOfOrigin:          []string{"China"},
		PriorClaims5yr:             false,
		CurrentCoverage:            "None - first-time buyer",
		GLPerOccurrence:            "$1,000,000",
		GLAggregate:                "$2,000,000",
		ProductsCompletedOps:       true,
		TradeShowAdditionalInsured: true,
		EffectiveDate:              "2026-06-29",
		BudgetAnnualUSD:            2500,
	}
}

// retailProfile models any low-hazard retailer/service business that does NOT
// import, private-label, or manufacture. This profile should be routed to
// mainstream instant-quote carriers.
func retailProfile() Profile {
	return Profile{
		LegalName:            "Main Street Books LLC",
		EntityStructure:      "LLC",
		FormationState:       "Delaware",
		BusinessAddress:      "55 Market St, Springfield, IL 62701",
		ContactName:          "John Roe",
		ContactEmail:         "shop@example.com",
		ContactPhone:         "(312) 555-0199",
		YearStarted:          2020,
		RevenueBand:          "$100k-$175k",
		IndustryClass:        "Retail store",
		Importer:             false,
		PrivateLabel:         false,
		Manufacturer:         false,
		GLPerOccurrence:      "$1,000,000",
		GLAggregate:          "$2,000,000",
		ProductsCompletedOps: true,
		EffectiveDate:        "2026-06-29",
	}
}

// manufacturerOnlyProfile models a business that manufactures (or has products
// made for it) but does NOT import or private-label - e.g. a domestic contract
// manufacturer. IsImporterClass is true, but only the manufacturer appetite
// should apply.
func manufacturerOnlyProfile() Profile {
	p := importerProfile()
	p.LegalName = "Domestic Goods Mfg LLC"
	p.IndustryClass = "Contract manufacturer"
	p.Importer = false
	p.PrivateLabel = false
	p.Manufacturer = true
	p.CountriesOfOrigin = nil
	return p
}

// privateLabelOnlyProfile models a private-label brand that neither imports nor
// manufactures (e.g. it buys finished goods from a domestic supplier and sells
// under its own brand). Only the private-label appetite should apply.
func privateLabelOnlyProfile() Profile {
	p := importerProfile()
	p.LegalName = "Housebrand Co LLC"
	p.IndustryClass = "Private-label reseller"
	p.Importer = false
	p.PrivateLabel = true
	p.Manufacturer = false
	p.CountriesOfOrigin = nil
	return p
}

func mustRegistry(t interface{ Fatalf(string, ...any) }) Registry {
	reg, err := EmbeddedRegistry()
	if err != nil {
		t.Fatalf("EmbeddedRegistry: %v", err)
	}
	return reg
}

// recByID indexes recommendations by provider id.
func recByID(recs []Recommendation) map[string]Recommendation {
	m := make(map[string]Recommendation, len(recs))
	for _, r := range recs {
		m[r.Provider.ID] = r
	}
	return m
}
