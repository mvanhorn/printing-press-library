package store

// AnalyticsResourceTypes returns a fresh set of types backed by typed tables.
func AnalyticsResourceTypes() map[string]bool {
	types := make(map[string]bool, len(typedListTableByResource))
	for rt := range typedListTableByResource {
		types[rt] = true
	}
	return types
}
