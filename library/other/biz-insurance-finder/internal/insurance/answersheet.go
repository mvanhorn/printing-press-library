package insurance

import (
	"fmt"
	"strings"
)

// GenerateAnswerSheet maps the stored profile to a paste-ready set of field
// values for one provider's quote form, then appends any provider-specific
// hints from the registry. It is a pure function.
func GenerateAnswerSheet(p Profile, prov Provider) AnswerSheet {
	sheet := AnswerSheet{
		ProviderID:   prov.ID,
		ProviderName: prov.Name,
		QuoteURL:     prov.StartURL(),
	}
	add := func(field, value, note string) {
		sheet.Fields = append(sheet.Fields, AnswerField{Field: field, Value: value, Note: note})
	}

	// --- Business identity ---
	add("Legal entity name", p.LegalName, "")
	if p.DBA != "" {
		add("DBA / brand", p.DBA, "")
	}
	add("Entity structure", p.EntityStructure, "")
	add("State of formation", p.FormationState, "")
	add("Business address", p.BusinessAddress, "")
	mailing := p.MailingAddress
	if mailing == "" {
		mailing = p.BusinessAddress
	}
	add("Mailing address", mailing, "")

	// --- Contact / named insured ---
	add("Contact / named insured", p.ContactName, "")
	add("Email", p.ContactEmail, "")
	add("Phone", p.ContactPhone, "")

	// --- Operations / class ---
	add("Year business started", yearStr(p.YearStarted), "Use operating history, not a rebrand date.")
	add("Annual revenue", p.RevenueBand, "")
	add("W-2 employees", fmt.Sprintf("%d", p.EmployeesW2), "")
	add("1099 contractors", fmt.Sprintf("%d", p.Contractors1099), "")
	if p.IndustryClass != "" {
		add("Industry / class", p.IndustryClass, "Let the form suggest the closest class code if unsure.")
	}

	// --- Importer / private-label / manufacturer ---
	add("Importer / private-label / manufacturer", importerStatus(p),
		importerStatusNote(p))
	if p.Products != "" {
		add("Products", p.Products, "")
	}
	if len(p.CountriesOfOrigin) > 0 {
		add("Country of origin", strings.Join(p.CountriesOfOrigin, ", "), "")
	}
	add("Sells on Amazon", yesNo(p.SellsOnAmazon), "")
	if p.RetailerLimitRequirement != "" {
		add("Retailer limit requirement", p.RetailerLimitRequirement, "")
	}

	// --- Loss history / current coverage ---
	add("Prior claims (5 yr)", priorClaims(p), "")
	if p.CurrentCoverage != "" {
		add("Current / prior coverage", p.CurrentCoverage, "")
	}

	// --- Coverage requested ---
	add("GL limits", glLimits(p), "Per occurrence / aggregate.")
	add("Products-completed operations", yesNo(p.ProductsCompletedOps),
		productsCompletedNote(p))
	add("Trade-show additional insured", yesNo(p.TradeShowAdditionalInsured),
		tradeShowNote(p))
	if p.WantCyber {
		add("Cyber / e-commerce liability", "Yes - quote if offered", "")
	}
	if p.WantBOP {
		add("BOP / business personal property", "Yes - quote if offered", "")
	}
	if p.EffectiveDate != "" {
		add("Desired effective date", p.EffectiveDate, "Default to the next business working day.")
	}
	if p.BudgetAnnualUSD > 0 {
		add("Budget (annual)", fmt.Sprintf("$%d/yr", p.BudgetAnnualUSD), "")
	}

	// --- Consents ---
	add("Marketing / SMS consent", "DECLINE optional marketing & SMS consents",
		"The unavoidable TCPA disclosure on lead capture is the cost of an online quote; decline anything optional.")

	// --- Provider-specific hints from the registry ---
	for _, h := range prov.FieldHints {
		add(h.Field, h.Value, h.Note)
	}

	return sheet
}

func yearStr(y int) string {
	if y == 0 {
		return "(not provided)"
	}
	return fmt.Sprintf("%d", y)
}

func yesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

func importerStatus(p Profile) string {
	var parts []string
	if p.Importer {
		parts = append(parts, "Importer of record")
	}
	if p.PrivateLabel {
		parts = append(parts, "Private-label own brand")
	}
	if p.Manufacturer {
		parts = append(parts, "Manufacturer")
	}
	if len(parts) == 0 {
		return "No - reseller / service only (not an importer or manufacturer)"
	}
	return strings.Join(parts, "; ")
}

func importerStatusNote(p Profile) string {
	if p.IsImporterClass() {
		return "For US product liability you are the \"deemed manufacturer\". Disclose this honestly - it routes you to the right (specialty) market."
	}
	return ""
}

func priorClaims(p Profile) string {
	if !p.PriorClaims5yr {
		return "None"
	}
	if p.PriorClaimsDetail != "" {
		return "Yes - " + p.PriorClaimsDetail
	}
	return "Yes"
}

func glLimits(p Profile) string {
	occ := p.GLPerOccurrence
	agg := p.GLAggregate
	if occ == "" && agg == "" {
		return "(not provided)"
	}
	if agg == "" {
		return occ + " per occurrence"
	}
	return fmt.Sprintf("%s per occurrence / %s aggregate", occ, agg)
}

func productsCompletedNote(p Profile) string {
	if p.IsImporterClass() {
		return "Must apply to your imported private-label goods - confirm it is included."
	}
	return ""
}

func tradeShowNote(p Profile) string {
	if p.TradeShowAdditionalInsured {
		return "Ask for a blanket / automatic additional-insured endorsement so each trade-show venue can be added."
	}
	return ""
}
