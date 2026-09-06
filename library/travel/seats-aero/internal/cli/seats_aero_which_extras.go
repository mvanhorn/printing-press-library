// Hand-authored extension: appends endpoint capabilities to the which index (generated which.go seeds novel features only).
package cli

func init() {
	appendSeatsAeroEndpointCapabilities()
}

func appendSeatsAeroEndpointCapabilities() {
	existing := make(map[string]struct{}, len(whichIndex))
	for _, entry := range whichIndex {
		existing[entry.Command] = struct{}{}
	}
	for _, entry := range seatsAeroEndpointCapabilities {
		if _, ok := existing[entry.Command]; ok {
			continue
		}
		whichIndex = append(whichIndex, entry)
		existing[entry.Command] = struct{}{}
	}
}

// seatsAeroEndpointCapabilities lists the generated endpoint commands agents
// most often ask for by capability. Descriptions use the same vocabulary as
// --help and SKILL.md so `which` ranks them for natural-language queries.
var seatsAeroEndpointCapabilities = []whichEntry{
	{Command: "awards", Description: "Search award seat availability between airports across every mileage program: cabins, direct-only, carriers, date window, lowest mileage first (cached search).", Group: "Live Partner API", WhyItMatters: "Start here for any find-an-award question; one call spans all programs."},
	{Command: "availability", Description: "Bulk award availability calendar for one mileage program by cabin, date window and regions.", Group: "Live Partner API", WhyItMatters: "Use for a whole program's calendar; use awards for a specific route."},
	{Command: "trips", Description: "Flight-level trip details (segments, taxes, booking links) for an availability row.", Group: "Live Partner API", WhyItMatters: "The bookable itinerary behind a search result."},
	{Command: "routes", Description: "List every route a mileage program tracks (origin, destination, region, distance).", Group: "Live Partner API", WhyItMatters: "Discover coverage before searching availability."},
	{Command: "destinations", Description: "Nonstop destinations reachable from one airport with the lowest mileage per cabin across all programs.", Group: "Live Partner API", WhyItMatters: "Where-can-I-go discovery from a single origin."},
	{Command: "refresh", Description: "Queue a refresh of stale cached availability rows (Pro keys only; credit-metered).", Group: "Live Partner API", WhyItMatters: "Re-verify cached data before booking; prefer recheck for a quota-guarded plan."},
}
