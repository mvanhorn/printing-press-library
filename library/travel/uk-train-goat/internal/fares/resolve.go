package fares

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

// ResolvedFare is a single fare returned by Resolve, enriched with ticket and
// restriction metadata.
type ResolvedFare struct {
	TicketCode      string
	TicketName      string
	Route           string // same ticket may be filed at multiple routes with different prices
	Pence           int
	RestrictionCode string
	RestrictionDesc string
	Single          bool // true when ticket_type == "S"
}

// Resolve returns the walk-up fares from fromCRS to toCRS valid on date
// (YYYYMMDD), sorted by pence ascending. Returns an empty slice (not nil) when
// no fare matches. Never returns a fare absent from the store or a
// date-expired fare.
func Resolve(db *sql.DB, fromCRS, toCRS, date string) ([]ResolvedFare, error) {
	// Step 1: CRS -> NLC (not date-filtered).
	fromNLC, ok, err := crsToNLC(db, fromCRS)
	if err != nil {
		return nil, fmt.Errorf("fares: Resolve: CRS lookup %q: %w", fromCRS, err)
	}
	if !ok {
		return []ResolvedFare{}, nil
	}

	toNLC, ok, err := crsToNLC(db, toCRS)
	if err != nil {
		return nil, fmt.Errorf("fares: Resolve: CRS lookup %q: %w", toCRS, err)
	}
	if !ok {
		return []ResolvedFare{}, nil
	}

	// Step 2: Build fare-location sets for origin and destination.
	oSet, err := fareLocationSet(db, fromNLC)
	if err != nil {
		return nil, fmt.Errorf("fares: Resolve: origin location set: %w", err)
	}
	dSet, err := fareLocationSet(db, toNLC)
	if err != nil {
		return nil, fmt.Errorf("fares: Resolve: dest location set: %w", err)
	}

	// Step 3: Match flows date-active on date, forward or reversible.
	matchedFlows, err := matchFlows(db, oSet, dSet, date)
	if err != nil {
		return nil, fmt.Errorf("fares: Resolve: match flows: %w", err)
	}

	if len(matchedFlows) == 0 {
		return []ResolvedFare{}, nil
	}

	// Steps 4-6: Fetch fares + ticket names + restriction descriptions.
	type dedupKey struct {
		ticketCode string
		route      string
	}
	cheapest := make(map[dedupKey]ResolvedFare)

	flowIDs := make([]string, len(matchedFlows))
	routeByFlow := make(map[string]string, len(matchedFlows))
	for i, fm := range matchedFlows {
		flowIDs[i] = fm.flowID
		routeByFlow[fm.flowID] = fm.route
	}

	// Build IN-clause placeholders.
	placeholders := make([]string, len(flowIDs))
	args := make([]interface{}, len(flowIDs))
	for i, id := range flowIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	inClause := strings.Join(placeholders, ",")

	q := fmt.Sprintf(`
		SELECT f.flow_id, f.ticket_code, f.pence, f.restriction_code,
		       tt.description, tt.ticket_type,
		       COALESCE(r.description, '') AS restriction_desc
		FROM rjf_fares f
		JOIN rjf_ticket_types tt ON tt.code = f.ticket_code
		LEFT JOIN rjf_restrictions r ON r.code = f.restriction_code AND f.restriction_code != ''
		WHERE f.flow_id IN (%s)`, inClause)

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("fares: Resolve: query fares: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var flowID, ticketCode, restrictionCode, ticketName, ticketTypeVal, restrictionDesc string
		var pence int
		if err := rows.Scan(&flowID, &ticketCode, &pence, &restrictionCode,
			&ticketName, &ticketTypeVal, &restrictionDesc); err != nil {
			return nil, fmt.Errorf("fares: Resolve: scan fare row: %w", err)
		}
		route := routeByFlow[flowID]
		key := dedupKey{ticketCode: ticketCode, route: route}
		existing, exists := cheapest[key]
		if !exists || pence < existing.Pence {
			cheapest[key] = ResolvedFare{
				TicketCode:      ticketCode,
				TicketName:      ticketName,
				Route:           route,
				Pence:           pence,
				RestrictionCode: restrictionCode,
				RestrictionDesc: restrictionDesc,
				Single:          ticketTypeVal == "S",
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fares: Resolve: iterate fares: %w", err)
	}

	// Step 7: NFO direct overrides (rjf_ndf).
	ndfRows, err := db.Query(`
		SELECT n.route, n.ticket_code, n.pence, n.restriction_code,
		       tt.description, tt.ticket_type,
		       COALESCE(r.description, '') AS restriction_desc
		FROM rjf_ndf n
		JOIN rjf_ticket_types tt ON tt.code = n.ticket_code
		LEFT JOIN rjf_restrictions r ON r.code = n.restriction_code AND n.restriction_code != ''
		WHERE n.origin_nlc = ? AND n.dest_nlc = ?
		  AND n.start_date <= ? AND n.end_date >= ?`,
		fromNLC, toNLC, date, date)
	if err != nil {
		return nil, fmt.Errorf("fares: Resolve: query ndf: %w", err)
	}
	defer ndfRows.Close()

	for ndfRows.Next() {
		var route, ticketCode, restrictionCode, ticketName, ticketTypeVal, restrictionDesc string
		var pence int
		if err := ndfRows.Scan(&route, &ticketCode, &pence, &restrictionCode,
			&ticketName, &ticketTypeVal, &restrictionDesc); err != nil {
			return nil, fmt.Errorf("fares: Resolve: scan ndf row: %w", err)
		}
		key := dedupKey{ticketCode: ticketCode, route: route}
		existing, exists := cheapest[key]
		if !exists || pence < existing.Pence {
			cheapest[key] = ResolvedFare{
				TicketCode:      ticketCode,
				TicketName:      ticketName,
				Route:           route,
				Pence:           pence,
				RestrictionCode: restrictionCode,
				RestrictionDesc: restrictionDesc,
				Single:          ticketTypeVal == "S",
			}
		}
	}
	if err := ndfRows.Err(); err != nil {
		return nil, fmt.Errorf("fares: Resolve: iterate ndf: %w", err)
	}

	// Step 8: Sort by pence ascending; tie-break by ticket_code then route.
	result := make([]ResolvedFare, 0, len(cheapest))
	for _, rf := range cheapest {
		result = append(result, rf)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Pence != result[j].Pence {
			return result[i].Pence < result[j].Pence
		}
		if result[i].TicketCode != result[j].TicketCode {
			return result[i].TicketCode < result[j].TicketCode
		}
		return result[i].Route < result[j].Route
	})
	return result, nil
}

// crsToNLC resolves a CRS code to an NLC. Not date-filtered.
// Returns ("", false, nil) when the CRS is not in the store.
func crsToNLC(db *sql.DB, crs string) (string, bool, error) {
	var nlc string
	err := db.QueryRow(`SELECT nlc FROM rjf_locations WHERE crs = ? LIMIT 1`, crs).Scan(&nlc)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return nlc, true, nil
}

// fareLocationSet builds the set {nlc} ∪ clusters ∪ group memberships for nlc.
func fareLocationSet(db *sql.DB, nlc string) (map[string]struct{}, error) {
	set := map[string]struct{}{nlc: {}}

	// Clusters: any cluster_id that nlc is a member of.
	rows, err := db.Query(`SELECT cluster_id FROM rjf_clusters WHERE member_nlc = ?`, nlc)
	if err != nil {
		return nil, fmt.Errorf("query clusters: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var clusterID string
		if err := rows.Scan(&clusterID); err != nil {
			return nil, fmt.Errorf("scan cluster: %w", err)
		}
		set[clusterID] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate clusters: %w", err)
	}

	// Group memberships: any group_nlc that nlc belongs to.
	gRows, err := db.Query(`SELECT group_nlc FROM rjf_group_members WHERE member_nlc = ?`, nlc)
	if err != nil {
		return nil, fmt.Errorf("query group_members: %w", err)
	}
	defer gRows.Close()
	for gRows.Next() {
		var groupNLC string
		if err := gRows.Scan(&groupNLC); err != nil {
			return nil, fmt.Errorf("scan group_member: %w", err)
		}
		set[groupNLC] = struct{}{}
	}
	if err := gRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate group_members: %w", err)
	}

	return set, nil
}

type flowMatch struct {
	flowID string
	route  string
}

// matchFlows returns all flow_id/route pairs that are date-active on date and
// whose origin/dest are covered by oSet/dSet (forward) or by dSet/oSet with
// direction='R' (reverse reuse).
func matchFlows(db *sql.DB, oSet, dSet map[string]struct{}, date string) ([]flowMatch, error) {
	oList := setToSlice(oSet)
	dList := setToSlice(dSet)

	// Build four placeholder groups: oList, dList, dList, oList (forward + reverse).
	oPlaceholders := makePlaceholders(len(oList))
	dPlaceholders := makePlaceholders(len(dList))

	q := fmt.Sprintf(`
		SELECT flow_id, route FROM rjf_flows
		WHERE start_date <= ? AND end_date >= ?
		  AND (
		    (origin_nlc IN (%s) AND dest_nlc IN (%s))
		    OR
		    (direction = 'R' AND origin_nlc IN (%s) AND dest_nlc IN (%s))
		  )`,
		oPlaceholders, dPlaceholders, dPlaceholders, oPlaceholders)

	// Args order: date, date, oList..., dList..., dList..., oList...
	args := make([]interface{}, 0, 2+2*len(oList)+2*len(dList))
	args = append(args, date, date)
	for _, v := range oList {
		args = append(args, v)
	}
	for _, v := range dList {
		args = append(args, v)
	}
	for _, v := range dList {
		args = append(args, v)
	}
	for _, v := range oList {
		args = append(args, v)
	}

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("query flows: %w", err)
	}
	defer rows.Close()

	var matches []flowMatch
	for rows.Next() {
		var fm flowMatch
		if err := rows.Scan(&fm.flowID, &fm.route); err != nil {
			return nil, fmt.Errorf("scan flow: %w", err)
		}
		matches = append(matches, fm)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate flows: %w", err)
	}
	return matches, nil
}

func setToSlice(s map[string]struct{}) []string {
	out := make([]string, 0, len(s))
	for k := range s {
		out = append(out, k)
	}
	sort.Strings(out) // deterministic query args
	return out
}

func makePlaceholders(n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = "?"
	}
	return strings.Join(parts, ",")
}
