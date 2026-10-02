package mcp

// Keep public planning advice separate from generic storage scaffold prose.
func applySmartEXContext(ctx map[string]any) {
	ctx["query_tips"] = []string{
		"Resolve English or Japanese names with stations; Osaka is not Shin-Osaka.",
		"Use limit/detail flags where the command exposes them; reference pages do not expose cursor pagination.",
		"Fare returns dated adult basic fares; children, unverified discounts and live seats remain unknown.",
		"Timetable examples are incomplete and do not prove operation on the requested date; offline freshness remains unchecked.",
		"Window checks JST calendars, class and party restrictions; bilingual source conflicts stay explicit.",
		"Use handoff for the canonical official booking URL and travel checklist.",
	}
	ctx["local_storage"] = "This planning CLI has no sync or search and does not store planning output. The SQL scaffold has no smartEX trip data."
}
