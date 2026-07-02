package insurance

// Warnings returns the underwriting landmines relevant to the profile, ordered
// most-severe first. These encode the lessons from the live multi-carrier quote
// run. It is a pure function.
func Warnings(p Profile) []Warning {
	var ws []Warning

	if p.IsImporterClass() {
		// The #1 importer landmine.
		ws = append(ws, Warning{
			Title:    "Foreign-products exclusion",
			Severity: SeverityCritical,
			Detail:   "For an importer, the #1 landmine is a foreign-products / imported-goods exclusion that quietly guts your product liability. Confirm IN WRITING that the policy has NO such exclusion before you bind.",
		})
		ws = append(ws, Warning{
			Title:    "Route to specialty markets, not mainstream instant-quote carriers",
			Severity: SeverityImportant,
			Detail:   "As an importer / private-label / manufacturer you are a \"deemed manufacturer\". Mainstream instant-quote carriers (e.g. The Hartford, biBerk, Next) decline this class. Go to specialty markets (Insurance Canopy / Veracity, XINSURANCE) or a marketplace that routes there (Tivly).",
		})
		ws = append(ws, Warning{
			Title:    "Products-completed operations must cover imported goods",
			Severity: SeverityImportant,
			Detail:   "Make sure products-completed operations is included AND that it applies to your imported private-label products specifically.",
		})
	}

	if p.IsImporterClass() || p.DBA != "" {
		ws = append(ws, Warning{
			Title:    "GL Coverage B does not protect your brand IP",
			Severity: SeverityImportant,
			Detail:   "GL Coverage B (Personal & Advertising Injury) excludes patent and most trademark infringement. Real IP protection for your private-label brand needs a separate IP / media policy - do not assume GL covers it.",
		})
	}

	// Process lessons that apply to any online quote run.
	ws = append(ws, Warning{
		Title:    "Multi-step wizards capture the lead at the CONTACT step",
		Severity: SeverityInfo,
		Detail:   "On many broker wizards, entering your phone number and clicking Next IS the submit (plus a marketing consent). Know where the real submission happens before you fill the contact step.",
	})
	ws = append(ws, Warning{
		Title:    "Use the next business working day as the start date",
		Severity: SeverityInfo,
		Detail:   "Default the policy effective date to the next business working day unless you have a reason to pick another.",
	})
	ws = append(ws, Warning{
		Title:    "Decline optional marketing / SMS consents",
		Severity: SeverityInfo,
		Detail:   "Uncheck optional marketing and SMS consents. The unavoidable TCPA disclosure on lead capture is the cost of getting an online quote.",
	})

	return ws
}
