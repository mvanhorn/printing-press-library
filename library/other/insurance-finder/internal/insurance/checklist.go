package insurance

// GenerateChecklist returns the per-provider list of manual actions the tool
// must NOT perform on the user's behalf. These are the human-only steps:
// CAPTCHAs, account/password creation, government IDs, payment, and the
// explicit two-gate submit approval. It is a pure function.
func GenerateChecklist(prov Provider) ProviderChecklist {
	cl := ProviderChecklist{
		ProviderID:   prov.ID,
		ProviderName: prov.Name,
		QuoteURL:     prov.StartURL(),
	}
	cl.Items = []ChecklistItem{
		{
			Action:   "Complete any CAPTCHA / \"I'm not a robot\" challenge",
			Detail:   "Solve it yourself - the tool will not and cannot do this.",
			Required: false,
		},
		{
			Action:   "Create any account & enter passwords yourself",
			Detail:   "If the form requires an account, you create it. Never let the tool type a password.",
			Required: false,
		},
		{
			Action:   "Enter EIN / SSN / government ID yourself",
			Detail:   "The profile intentionally does NOT store these. Type them in directly only when the form genuinely needs them.",
			Required: true,
		},
		{
			Action:   "Enter payment / card / bank info yourself",
			Detail:   "Only at bind time. Quotes are free - do not enter payment during a quote run.",
			Required: false,
		},
		{
			Action:   "Approve each submission explicitly (two-gate)",
			Detail:   "Gate 1: review the filled values shown by the tool. Gate 2: you click submit. The tool never submits for you.",
			Required: true,
		},
		{
			Action:   "Decline optional marketing / SMS consents",
			Detail:   "Uncheck anything optional. The unavoidable TCPA lead-capture disclosure is the cost of an online quote.",
			Required: true,
		},
	}

	if prov.SubmitNote != "" {
		cl.Items = append(cl.Items, ChecklistItem{
			Action:   "Know where the REAL submit happens",
			Detail:   prov.SubmitNote,
			Required: true,
		})
	}
	if prov.ManualNote != "" {
		cl.Items = append(cl.Items, ChecklistItem{
			Action:   "Provider-specific note",
			Detail:   prov.ManualNote,
			Required: true,
		})
	}
	return cl
}
