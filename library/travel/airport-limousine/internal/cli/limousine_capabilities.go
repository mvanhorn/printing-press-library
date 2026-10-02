// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

// Curated discovery commands extend the generated hero-capability index without template edits.
func init() {
	whichIndex = append(whichIndex,
		whichEntry{Command: "routes", Description: "Find airport route IDs and area names in the provider route catalog.", Group: "Route and stop discovery", WhyItMatters: "Resolve the exact route and both timetable directions before planning."},
		whichEntry{Command: "stops find", Description: "Public stop search for exact terminal IDs, English names and Japanese names.", Group: "Route and stop discovery", WhyItMatters: "Keep station, bus terminal and airport terminals distinct."},
		whichEntry{Command: "stops get", Description: "Inspect a stop boarding location, address, maps and connected routes.", Group: "Route and stop discovery", WhyItMatters: "Get official terminal boarding details after resolving an exact stop ID."},
		whichEntry{Command: "handoff", Description: "Resolve a dated canonical booking handoff link for a provider route.", Group: "Route and stop discovery", WhyItMatters: "Read the official timetable and let the user follow reservation links."},
	)
}
