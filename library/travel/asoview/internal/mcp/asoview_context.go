package mcp

// asoviewContext grounds generated agent advice in the actual public command contract.
func asoviewContext(ctx map[string]any) map[string]any {
	if rows, ok := ctx["resources"].([]map[string]any); ok {
		for _, row := range rows {
			row["syncable"] = false
			row["searchable"] = false
		}
	}
	ctx["query_tips"] = []string{
		"Use discover with --region/--category/--date/--adults/--children; --query is a bounded local substring, not full-text catalog search.",
		"Discovery defaults to ten matches from one page. Continue using coverage.next_cursor as --cursor; maximum --pages is five.",
		"Fetch product details lazily for shortlisted IDs. Advertised minimums may be child bands, and unknown units remain null.",
		"Use availability for public dated stock and options for dated fee bands. Stock is a snapshot; no reserved slot or checkout quote is created.",
		"Inventory refresh is explicit with --refresh-inventory. Canonical handoff URLs are derived without purchasing or booking.",
	}
	return ctx
}
