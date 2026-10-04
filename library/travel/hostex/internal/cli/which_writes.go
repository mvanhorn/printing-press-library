// Copyright 2026 bust011r and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH(write-commands-document-body-shape-in-cli): the listing calendar write
// commands are advertised to `which` so "update prices" resolves to them, and
// they only rank for queries that carry a write verb.

func init() {
	whichIndex = append(whichIndex,
		whichEntry{Command: "listings update-prices", Description: "Update the prices of a channel listing for date ranges (live price change; dry-run first).", Group: "Listing calendar writes", WhyItMatters: "Use to change what guests pay on a channel for specific dates; always preview with --dry-run, prices are integers in the listing currency."},
		whichEntry{Command: "listings update-restrictions", Description: "Update the booking restrictions of a channel listing for date ranges: min/max stay, closed on arrival or departure.", Group: "Listing calendar writes", WhyItMatters: "Use to set minimum nights or close check-in on a channel for specific dates; preview with --dry-run."},
		whichEntry{Command: "listings update-inventories", Description: "Update the per-channel inventory of a listing for date ranges; does not change the property calendar.", Group: "Listing calendar writes", WhyItMatters: "Use to open or close one channel for specific dates; to block the property itself use availabilities update."},
	)
}

// whichCalendarWriteVerbs are the words that signal a query wants to change
// listing calendar data; they extend the generic whichWriteVerbs set.
var whichCalendarWriteVerbs = map[string]bool{
	"update": true, "change": true, "set": true, "edit": true, "modify": true,
	"adjust": true, "push": true, "raise": true, "lower": true, "increase": true,
	"decrease": true, "open": true, "close": true, "block": true, "reprice": true,
}

func whichHasWriteIntent(qTokens []string) bool {
	for _, qt := range qTokens {
		if whichWriteVerbs[qt] || whichCalendarWriteVerbs[qt] {
			return true
		}
	}
	return false
}
