// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.

package cli

// nepraScopeLimit is one thing this CLI deliberately does NOT do, with the
// measured reason it does not.
//
// The absorb manifest's parity target was bijlicheck, which openly refuses
// K-Electric, time-of-use and unaccountable readings — but only in its README.
// Declaring refusals in the MACHINE-READABLE surface is the point: an agent
// that can read why a thing is impossible stops asking for it, and stops
// treating a refusal as a bug.
type nepraScopeLimit struct {
	Subject string `json:"subject"`
	// Verdict is unavailable | excluded_by_source | unverifiable | out_of_scope.
	Verdict string `json:"verdict"`
	Reason  string `json:"reason"`
	// Instead names the command that DOES answer the nearest useful question.
	Instead string `json:"instead,omitempty"`
}

// nepraScopeLimits is the declared scope. Every entry is a measured finding,
// not a policy preference.
var nepraScopeLimits = []nepraScopeLimit{
	{
		// Added 11 Sep 2026. `licence`'s own refusal message points the caller
		// at `doctor --scope`, and an output review found that promise
		// dangling: the scope list had 12 subjects and none of them mentioned
		// --ticker. A refusal that cites a declaration which does not exist is
		// worse than no citation.
		Subject: "licence register lookup by PSX ticker (licence --ticker)",
		Verdict: "unverifiable",
		Reason: "The embedded plant crosswalk carries generation-WORKBOOK spellings while the licence register " +
			"publishes LEGAL names, and the fold deliberately does not treat Ltd and Limited as the same token. " +
			"MEASURED against all 335 in-scope register headers: only 28 resolve. `--ticker HUBC` would return " +
			"three entities and silently MISS 'Hub Power Company Limited' (1,292 MW) and 'Hub Power Generation " +
			"Company (Pvt.) Limited Narowal', while KAPCO, NPL and NCPL each reach zero. A flag that answers for " +
			"8% of the register while looking complete is worse than no flag, so it exits 2. Closing this is a " +
			"curated-alias DATA task, not a code change.",
		Instead: "licence --search \"<company>\" against the register's own names, or fleet --parent <symbol> " +
			"for the generation panel, where the crosswalk is the right instrument",
	},
	{
		Subject: "consumer tariff rate schedules (per-category Rs/kWh)",
		Verdict: "unverifiable",
		Reason: "The only machine-readable consumer schedule on the site is frozen at 'Notified Tariff 01-01-2019'. " +
			"The current S.R.O.s are image-only PDFs: S.R.O. 41(1)2026 is 100 pages carrying 100 embedded images and " +
			"yields 14,900 characters of text, every one a repeated registrar stamp, and it is one of 13 near-identical " +
			"SROs dated 13.01.2026 with no machine-determinable DISCO attribution. Emitting a rate would mean inventing one.",
		Instead: "events --disco <name>, which dates the determination and links the PDF",
	},
	{
		Subject: "circular debt and national electricity sales",
		Verdict: "unverifiable",
		Reason: "These live only inside the State of Industry Report PDFs — 610 MB across ten reports at 68-164 KB/s, " +
			"with SIR 2025 alone at 316 MB. " +
			"Per-character kerning makes exact matching return false absences, so a figure could be reported missing when it is present.",
		Instead: "sources, which catalogues the reports and their reachability",
	},
	{
		Subject: "TESCO reliability metrics (SAIFI, SAIDI, losses)",
		Verdict: "excluded_by_source",
		Reason: "NEPRA excludes TESCO from the Performance Evaluation Report on the record, stating that supply to a large " +
			"number of consumers remains un-metered and its recording systems are unreliable. Ten entities are evaluated, not eleven. " +
			"This CLI returns the exclusion and NEPRA's own wording rather than an empty eleventh row or a zero.",
		Instead: "disco --metric <m>, which returns the ten evaluated entities and TESCO's stated exclusion",
	},
	{
		Subject: "FY2023-24 Performance Evaluation Report",
		Verdict: "unavailable",
		Reason: "The only published path for it 404s on NEPRA's own index. This is a real hole in the source, recorded as " +
			"UNAVAILABLE, which is a different fact from a report that exists and reports nothing.",
		Instead: "sources --kind per, which lists which report-years are reachable",
	},
	{
		Subject: "SAIDI for FY2010-11 through FY2013-14",
		Verdict: "unverifiable",
		Reason: "These exist only as TRUNCATED Excel chart data labels such as '19,535.' — the trailing digits are absent from " +
			"the PDF text layer, not merely hard to read. They are recorded UNVERIFIED and never parsed as numbers. One further " +
			"year in the surveyed range was never captured at all and is recorded as a known gap rather than interpolated.",
		Instead: "conflicts, which lists the recorded unverified figures",
	},
	{
		Subject: "the Power BI dashboard",
		Verdict: "unavailable",
		Reason: "Viewer-only. Probed twice with resource-key, Origin and Referer set: /public/reports/{rk} answers curl 56 " +
			"(empty reply) and /explore/.../modelsAndExploration answers 403. There is no plain-HTTP path to its data.",
		Instead: "gen --fy <year>, which reads the published workbook the dashboard is built from",
	},
	{
		Subject: "fuel price adjustment after June 2022",
		Verdict: "unverifiable",
		Reason: "The post-June-2022 source is OCR-corrupted and fails SILENTLY: one entity appears under five spellings " +
			"(XWDISCOs 41x, XWDlSCOs 4x, XWDJSCOs 1x, XWDISCQs 1x), amounts read as 'Rs.l.2000/kWh' with a letter l for the digit 1, " +
			"and 'Kl' for KE. Parsing it would produce plausible wrong numbers rather than an error.",
		Instead: "fca, whose coverage is hard-bounded Jul-2018 to Jun-2022, and events for later determinations",
	},
	{
		Subject: "consumer bill audit and recomputation",
		Verdict: "out_of_scope",
		Reason: "Requires per-consumer bill intake. This is a regulator-data CLI; a billing tool is a different product with a " +
			"different input surface.",
	},
	{
		Subject: "net-billing and prosumer simulation",
		Verdict: "out_of_scope",
		Reason:  "Simulation over assumed inputs rather than published regulator data.",
	},
	{
		Subject: "load-shedding schedules",
		Verdict: "out_of_scope",
		Reason:  "The available implementations take hand-downloaded XLSX files as input, which is not a NEPRA surface.",
	},
	{
		Subject: "cross-checking NEPRA's generation totals against the IEA series",
		Verdict: "out_of_scope",
		Reason: "IEA's own published policy forbids the fetch, measured 11 Sep 2026: " +
			"api.iea.org/robots.txt is \"User-agent: *\" / \"Disallow: /\" — a blanket disallow for every " +
			"agent across the entire API host, unlike the named-bot block on nepra.org.pk that this CLI " +
			"was cleared against with an honest product UA. IEA's Terms of Use separately exclude " +
			"standalone datasets and databases from their CC BY 4.0 Open Use Terms, making the series " +
			"Non-CC Material. Either one forbids it alone. The comparison would ALSO be invalid with the " +
			"data in hand: NEPRA publishes Jul-Jun fiscal years, IEA publishes calendar years ending " +
			"2023, so no two periods align, and NEPRA's dataset excludes K-Electric's own fleet while " +
			"IEA's series is national.",
		Instead: "verify --crosscheck iea, which reports the measured NEPRA side and every reason the two cannot be subtracted",
	},
	{
		Subject: "any JSON API",
		Verdict: "unavailable",
		Reason: "NEPRA publishes no API. Proven twice with two independent tools: a 124-request browser capture and the owner's " +
			"own DevTools Fetch/XHR filter (3 of 280 requests) both found ZERO non-telemetry XHR. The only Fetch/XHR on the site is " +
			"POST /cdn-cgi/rum returning 204, which is Cloudflare telemetry. The data pages carry 0 forms and 0 inputs, and the site " +
			"search is an outsourced Google CSE. Everything here is parsed from published HTML and PDF.",
	},
}
