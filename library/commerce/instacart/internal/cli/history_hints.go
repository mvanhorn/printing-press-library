package cli

// backfillHint returns the canonical one-line nudge shown to users whose
// local Instacart history store is empty. Surfaced by `doctor` and by
// `add --json` when live-search fallthrough happens on an empty store.
//
// Keep the message short. It is meant to be a pointer, not a tutorial.
// Full walkthrough lives in the pp-instacart skill (backfill intent) and
// in docs/patterns/authenticated-session-scraping.md.
func backfillHint() string {
	return "no local order history. run `/pp-instacart backfill my orders` to populate so `add` can resolve from your real purchases."
}

// historyIsEmpty reports whether the local purchase-history store has any
// orders or purchased_items rows. Used to gate first-run discovery hints.
// Errors from the store are treated as non-empty so the hint does not fire
// on a corrupt or locked database.
func historyIsEmpty(app *AppContext) bool {
	orderCount, err := app.Store.CountOrders()
	if err != nil || orderCount > 0 {
		return false
	}
	itemCount, _, err := app.Store.CountPurchasedItems()
	if err != nil {
		return false
	}
	return itemCount == 0
}
