// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package mcp

func providerContextResources() []map[string]any {
	return []map[string]any{
		{"name": "routes", "description": "Original course IDs, names and canonical URLs", "commands": []string{"routes list", "bus route"}, "syncable": false},
		{"name": "services", "description": "Dated JST inventory, source availability and overnight timestamps", "commands": []string{"bus services"}, "syncable": false},
		{"name": "stop-pair fare evidence", "description": "Source stop IDs, Adult/Child JPY fares, party capacity and cancellation fees", "commands": []string{"bus quote"}, "syncable": false},
		{"name": "operator conditions", "description": "Source baggage, boarding and route/operator policy text", "commands": []string{"bus conditions"}, "syncable": false},
	}
}

func providerQueryTips() []string {
	return []string{
		"Use routes list --query for a local substring filter over one public catalog response. Route and service lists use offset/limit, default 20, maximum 100; no cursor or after parameter.",
		"Use bus route for direction IDs and published schedules; schedules do not establish bookable inventory.",
		"Use bus services with an explicit YYYY-MM-DD JST service day. Omitted dates default to seven days after today in JST. Source date substitution, not on sale, sold out and unknown availability remain distinct.",
		"Use bus quote --include-stops to discover stop IDs scoped to route, direction, service and fare plan, then quote the selected pair and Adult/Child counts. Prices are one-way JPY arithmetic; numeric positive capacity is a conservative lower bound, transaction limits and seat/gender feasibility are separate.",
		"Use bus conditions for route/operator baggage and boarding text. Hand the canonical booking URL to the traveler; bookings, payments and account operations are outside this CLI.",
		"Provider data is live-only with no inventory cache. Use --timeout and --rate-limit to bound requests; --data-source local is rejected by provider commands. Local learning records are separate from provider inventory.",
	}
}
