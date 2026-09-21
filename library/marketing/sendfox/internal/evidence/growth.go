package evidence

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const defaultGrowthWindow = 30 * 24 * time.Hour

// GrowthReport derives honest growth measurements from one or two explicitly
// supplied snapshots. A single snapshot can only observe timestamped additions
// and suppressions; exact joins, leaves and retention require a complete prior
// membership snapshot.
func GrowthReport(s Snapshot, now time.Time, maxAge, window time.Duration) Report {
	r := NewReport("growth-report", s)
	r.Require(s, now, maxAge, "contacts", "unsubscribed", "lists", "memberships")
	if window <= 0 {
		window = defaultGrowthWindow
	}

	end, endOK := snapshotObservedAt(s, "contacts")
	if !endOK {
		end, endOK = snapshotObservedAt(s, "unsubscribed", "memberships")
	}
	if !endOK {
		end = now.UTC()
		r.Unknowns = append(r.Unknowns, "growth window: observation timestamp missing; --window anchored to current time")
	}
	start := end.Add(-window)
	windowLabel := start.Format(time.RFC3339) + "/" + end.Format(time.RFC3339)

	newContacts, missingCreated := timestampedCount(s.Resources["contacts"].Items, start, end, "created_at")
	unsubscribed, missingUnsubscribed := timestampedCount(s.Resources["unsubscribed"].Items, start, end, "unsubscribed_at", "created_at")
	if len(s.Resources["unsubscribed"].Items) == 0 {
		// A complete empty suppression resource is an observed zero. An absent or
		// incomplete resource is handled by Require and remains unknown.
		missingUnsubscribed = 0
	}

	newComplete := resourceComplete(s.Resources["contacts"]) && missingCreated == 0
	unsubComplete := resourceComplete(s.Resources["unsubscribed"]) && missingUnsubscribed == 0
	newWarnings := missingTimestampWarnings("contacts", "created_at", missingCreated)
	unsubWarnings := missingTimestampWarnings("unsubscribed", "unsubscribed_at/created_at", missingUnsubscribed)
	if !newComplete {
		r.Unknowns = append(r.Unknowns, "contacts: complete created_at evidence unavailable for the growth window")
	}
	if !unsubComplete {
		r.Unknowns = append(r.Unknowns, "unsubscribed: complete transition timestamps unavailable for the growth window")
	}
	r.Data["window"] = Row{"start": start.Format(time.RFC3339), "end": end.Format(time.RFC3339), "duration": window.String(), "anchored_to_observation": endOK}
	r.Data["new_contacts"] = countMetric("count(contacts.created_at within window)", windowLabel, newContacts, "derived_timestamped", newComplete, newWarnings)
	r.Data["unsubscribes"] = countMetric("count(unsubscribed.unsubscribed_at or created_at within window)", windowLabel, unsubscribed, "derived_timestamped", unsubComplete, unsubWarnings)

	netComplete := newComplete && unsubComplete
	netWarnings := append([]string{}, newWarnings...)
	netWarnings = append(netWarnings, unsubWarnings...)
	netWarnings = append(netWarnings, "Observed net growth excludes deletions, bounces and contacts whose transition timestamps are unavailable.")
	observedNet := newContacts - unsubscribed
	var observedNetValue any = observedNet
	if !netComplete {
		observedNetValue = nil
	}
	observedNetMetric := metric(
		"timestamped new contacts - timestamped unsubscribes",
		windowLabel,
		observedNetValue,
		nil,
		"derived_partial",
		netComplete,
		netWarnings,
	)
	if !netComplete {
		observedNetMetric["observed_value"] = observedNet
	}
	r.Data["observed_net_growth"] = observedNetMetric

	if s.Previous == nil {
		r.Data["snapshot_comparison"] = Row{
			"available": false,
			"metrics":   []Row{},
			"warnings":  []string{"Supply --previous to calculate exact observed joins, leaves, retention and net list growth between snapshots."},
		}
		r.Finish()
		return r
	}

	previous := *s.Previous
	previousReport := NewReport("previous-growth-snapshot", previous)
	previousReport.Require(previous, now, 0, "contacts", "lists", "memberships")
	for _, unknown := range previousReport.Unknowns {
		r.Unknowns = append(r.Unknowns, "Previous "+unknown)
	}
	comparisonWindow, windowWarnings := comparisonWindow(previous, s)
	membershipComplete := resourceComplete(previous.Resources["memberships"]) && previous.Resources["memberships"].ScopesComplete &&
		resourceComplete(s.Resources["memberships"]) && s.Resources["memberships"].ScopesComplete
	complete := membershipComplete && len(windowWarnings) == 0
	if !membershipComplete {
		windowWarnings = append(windowWarnings, "Membership comparison is incomplete or child scopes are not proven complete; missing rows are unknown, not zero.")
		r.Unknowns = append(r.Unknowns, "membership comparison: both snapshots need complete, uncapped child scopes")
	}
	if len(windowWarnings) > 0 && membershipComplete {
		r.Unknowns = append(r.Unknowns, "snapshot comparison: previous observation must have a valid timestamp earlier than current")
	}

	listNames := map[string]string{}
	for _, list := range previous.Resources["lists"].Items {
		listNames[Text(list["id"])] = Text(list["name"])
	}
	for _, list := range s.Resources["lists"].Items {
		listNames[Text(list["id"])] = Text(list["name"])
	}
	previousMembers, previousInvalid := membershipsByList(previous.Resources["memberships"].Items)
	currentMembers, currentInvalid := membershipsByList(s.Resources["memberships"].Items)
	if previousInvalid+currentInvalid > 0 {
		complete = false
		windowWarnings = append(windowWarnings, fmt.Sprintf("Membership comparison excluded %d row(s) without both list_id and contact_id; results are unknown, not zero.", previousInvalid+currentInvalid))
		r.Unknowns = append(r.Unknowns, "membership comparison: membership identity fields are incomplete")
	}
	listIDs := map[string]bool{}
	for id := range previousMembers {
		listIDs[id] = true
	}
	for id := range currentMembers {
		listIDs[id] = true
	}
	orderedIDs := make([]string, 0, len(listIDs))
	for id := range listIDs {
		orderedIDs = append(orderedIDs, id)
	}
	sort.Strings(orderedIDs)

	lists := make([]Row, 0, len(orderedIDs))
	for _, listID := range orderedIDs {
		before, after := previousMembers[listID], currentMembers[listID]
		joins, leaves, retained := setDelta(before, after)
		opening, closing := len(before), len(after)
		warnings := append([]string{}, windowWarnings...)
		retentionWarnings := append([]string{}, warnings...)
		growthWarnings := append([]string{}, warnings...)
		if opening == 0 {
			retentionWarnings = append(retentionWarnings, "Retention is undefined because the opening membership was zero.")
			growthWarnings = append(growthWarnings, "Growth rate is undefined because the opening membership was zero.")
		}
		var netValue any = closing - opening
		if !complete {
			netValue = nil
		}
		lists = append(lists, Row{
			"list_id":         listID,
			"list_name":       listNames[listID],
			"opening_members": countMetric("count(previous membership identities)", comparisonWindow, opening, "snapshot_comparison", complete, warnings),
			"closing_members": countMetric("count(current membership identities)", comparisonWindow, closing, "snapshot_comparison", complete, warnings),
			"joins":           countMetric("count(current identities - previous identities)", comparisonWindow, joins, "snapshot_comparison", complete, warnings),
			"leaves":          countMetric("count(previous identities - current identities)", comparisonWindow, leaves, "snapshot_comparison", complete, warnings),
			"net_growth":      metric("closing members - opening members", comparisonWindow, netValue, nil, "snapshot_comparison", complete, warnings),
			"retention_rate":  ratioMetric("retained members / opening members * 100", comparisonWindow, retained, opening, "snapshot_comparison", complete && opening > 0, retentionWarnings),
			"growth_rate":     ratioMetric("(closing members - opening members) / opening members * 100", comparisonWindow, closing-opening, opening, "snapshot_comparison", complete && opening > 0, growthWarnings),
		})
	}
	r.Data["snapshot_comparison"] = Row{"available": true, "window": comparisonWindow, "complete": complete, "lists": lists, "warnings": windowWarnings}
	r.Finish()
	return r
}

func metric(formula, window string, numerator, denominator any, grade string, complete bool, warnings []string) Row {
	if warnings == nil {
		warnings = []string{}
	}
	return Row{
		"formula":        formula,
		"window":         window,
		"value":          numerator,
		"numerator":      numerator,
		"denominator":    denominator,
		"evidence_grade": grade,
		"complete":       complete,
		"warnings":       warnings,
	}
}

func ratioMetric(formula, window string, numerator, denominator int, grade string, complete bool, warnings []string) Row {
	var value any
	var observedNumerator any = numerator
	var observedDenominator any = denominator
	if complete && denominator != 0 {
		value = percent(numerator, denominator)
	} else {
		observedNumerator = nil
		observedDenominator = nil
	}
	result := metric(formula, window, observedNumerator, observedDenominator, grade, complete && denominator != 0, warnings)
	result["value"] = value
	return result
}

func countMetric(formula, window string, value int, grade string, complete bool, warnings []string) Row {
	var numerator any = value
	if !complete {
		numerator = nil
	}
	result := metric(formula, window, numerator, nil, grade, complete, warnings)
	if !complete {
		result["observed_numerator"] = value
	}
	return result
}

func percent(numerator, denominator int) float64 {
	return float64(numerator) / float64(denominator) * 100
}

func resourceComplete(resource Resource) bool {
	return resource.Complete != nil && *resource.Complete && !resource.Capped && (resource.Total == nil || *resource.Total == len(resource.Items))
}

func timestampedCount(rows []Row, start, end time.Time, keys ...string) (int, int) {
	count, missing := 0, 0
	for _, row := range rows {
		when, ok := rowTime(row, keys...)
		if !ok {
			missing++
			continue
		}
		if !when.Before(start) && !when.After(end) {
			count++
		}
	}
	return count, missing
}

func missingTimestampWarnings(resource, fields string, missing int) []string {
	if missing == 0 {
		return []string{}
	}
	return []string{fmt.Sprintf("%s: %d row(s) lack parseable %s timestamps; value is unknown rather than zero", resource, missing, fields)}
}

func rowTime(row Row, keys ...string) (time.Time, bool) {
	for _, key := range keys {
		text := strings.TrimSpace(Text(row[key]))
		if text == "" {
			continue
		}
		if parsed, err := time.Parse(time.RFC3339, text); err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}

func snapshotObservedAt(s Snapshot, resources ...string) (time.Time, bool) {
	var latest time.Time
	found := false
	for _, name := range resources {
		when, err := time.Parse(time.RFC3339, s.Resources[name].ObservedAt)
		if err == nil && (!found || when.After(latest)) {
			latest, found = when.UTC(), true
		}
	}
	return latest, found
}

func comparisonWindow(previous, current Snapshot) (string, []string) {
	start, startOK := snapshotObservedAt(previous, "memberships")
	if !startOK {
		start, startOK = snapshotObservedAt(previous, "contacts")
	}
	end, endOK := snapshotObservedAt(current, "memberships")
	if !endOK {
		end, endOK = snapshotObservedAt(current, "contacts")
	}
	warnings := []string{}
	if !startOK || !endOK {
		warnings = append(warnings, "Snapshot comparison window is unknown because an observation timestamp is missing.")
		return "unknown", warnings
	}
	if !end.After(start) {
		warnings = append(warnings, "Current observation is not later than the previous observation.")
	}
	return start.Format(time.RFC3339) + "/" + end.Format(time.RFC3339), warnings
}

func membershipsByList(rows []Row) (map[string]map[string]bool, int) {
	out := map[string]map[string]bool{}
	invalid := 0
	for _, row := range rows {
		listID, contactID := Text(row["list_id"]), Text(row["contact_id"])
		if listID == "" || contactID == "" {
			invalid++
			continue
		}
		if out[listID] == nil {
			out[listID] = map[string]bool{}
		}
		out[listID][contactID] = true
	}
	return out, invalid
}

func setDelta(before, after map[string]bool) (joins, leaves, retained int) {
	for id := range after {
		if before[id] {
			retained++
		} else {
			joins++
		}
	}
	for id := range before {
		if !after[id] {
			leaves++
		}
	}
	return joins, leaves, retained
}
