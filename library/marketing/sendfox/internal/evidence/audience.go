package evidence

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/marketing/sendfox/internal/contract"
	"sort"
	"strings"
	"time"
)

// AudienceHealth audits observed relationships without inferring consent or deliverability.
func AudienceHealth(s Snapshot, now time.Time, maxAge time.Duration) Report {
	r := NewReport("audience-health", s)
	r.Require(s, now, maxAge, "contacts", "lists", "memberships", "contact_tags", "tags", "contact_fields", "unsubscribed", "activity")
	contacts := s.Resources["contacts"].Items
	byID := Index(contacts, "id")
	lists := Index(s.Resources["lists"].Items, "id")
	tags := Index(s.Resources["tags"].Items, "id")
	emails := map[string][]string{}
	members := map[string]int{}
	suppressed := Suppressed(s)
	for _, c := range contacts {
		email := Email(c["email"])
		id := Text(c["id"])
		emails[email] = append(emails[email], id)
		if !ValidEmail(email) {
			r.Findings = append(r.Findings, Row{"kind": "invalid_email", "contact_id": id, "email": email})
		}
		if suppressed[email] {
			r.Findings = append(r.Findings, Row{"kind": "suppressed", "contact_id": id, "email": email})
		}
	}
	keys := []string{}
	for email := range emails {
		keys = append(keys, email)
	}
	sort.Strings(keys)
	for _, email := range keys {
		ids := emails[email]
		if len(ids) > 1 {
			r.Findings = append(r.Findings, Row{"kind": "identity_collision", "email": email, "contact_ids": ids})
		}
	}
	for _, link := range s.Resources["memberships"].Items {
		cid, lid := Text(link["contact_id"]), Text(link["list_id"])
		members[cid]++
		if byID[cid] == nil || lists[lid] == nil {
			r.Findings = append(r.Findings, Row{"kind": "orphan_membership", "contact_id": cid, "list_id": lid})
		}
	}
	for _, link := range s.Resources["contact_tags"].Items {
		cid, tid := Text(link["contact_id"]), Text(link["tag_id"])
		if byID[cid] == nil || tags[tid] == nil {
			r.Findings = append(r.Findings, Row{"kind": "orphan_tag", "contact_id": cid, "tag_id": tid})
		}
	}
	if v := s.Resources["memberships"]; v.Complete != nil && *v.Complete && !v.Capped && v.ScopesComplete {
		for _, c := range contacts {
			if members[Text(c["id"])] == 0 {
				r.Findings = append(r.Findings, Row{"kind": "unassigned", "contact_id": c["id"]})
			}
		}
	} else {
		r.Unknowns = append(r.Unknowns, "memberships: child scopes not proven complete; unassigned counts unknown")
	}
	defs := Index(s.Resources["contact_fields"].Items, "id")
	for _, c := range contacts {
		for _, field := range Rows(c["contact_fields"]) {
			id := Text(field["id"])
			def := defs[id]
			if def == nil {
				r.Findings = append(r.Findings, Row{"kind": "unknown_custom_field", "contact_id": c["id"], "field_id": id})
				continue
			}
			typ := Text(def["type"])
			schema := map[string]any{"type": "string"}
			switch typ {
			case "number":
				schema["type"] = "number"
			case "date":
				schema["format"] = "date"
			}
			if err := contract.Validate(schema, field["value"]); err != nil {
				r.Findings = append(r.Findings, Row{"kind": "custom_field_type", "contact_id": c["id"], "field_id": id, "expected": typ})
			}
		}
	}
	cohorts := engagementCohorts(s, now)
	if cohorts["complete"] != true {
		r.Unknowns = append(r.Unknowns, "engagement cohorts: activity history or timestamps are incomplete; never-engaged count is unknown")
	}
	r.Data = Row{
		"contact_count":             len(contacts),
		"suppressed_identity_count": len(suppressed),
		"relationship_counts":       members,
		"engagement_cohorts":        cohorts,
		"note":                      "Structural checks do not establish consent or deliverability. Engagement cohorts use observed open/click timestamps only.",
	}
	r.Finish()
	return r
}

func engagementCohorts(s Snapshot, now time.Time) Row {
	contacts := s.Resources["contacts"]
	activity := s.Resources["activity"]
	asOf, ok := snapshotObservedAt(s, "activity", "contacts")
	if !ok {
		asOf = now.UTC()
	}
	complete := resourceComplete(contacts) && resourceComplete(activity) && activity.ScopesComplete
	latest := map[string]time.Time{}
	contactLatest := map[string]time.Time{}
	unparseable := map[string]bool{}
	for _, event := range activity.Items {
		kind := strings.ToLower(Text(event["type"]))
		if kind != "open" && kind != "opened" && kind != "click" && kind != "clicked" {
			continue
		}
		id := Text(event["contact_id"])
		when, parsed := rowTime(event, "occurred_at", "event_at", "created_at", "timestamp", "opened_at", "clicked_at")
		if id == "" || !parsed || when.After(asOf.Add(5*time.Minute)) {
			if id != "" {
				unparseable[id] = true
			}
			continue
		}
		if when.After(latest[id]) {
			latest[id] = when
		}
	}
	for _, contact := range contacts.Items {
		id := Text(contact["id"])
		for _, field := range []string{"last_engaged_at", "last_clicked_at", "last_opened_at"} {
			value := strings.TrimSpace(Text(contact[field]))
			if value == "" {
				continue
			}
			parsed, err := time.Parse(time.RFC3339, value)
			if err != nil || parsed.After(asOf.Add(5*time.Minute)) {
				unparseable[id] = true
				continue
			}
			if parsed.After(contactLatest[id]) {
				contactLatest[id] = parsed.UTC()
			}
		}
	}
	if len(unparseable) > 0 {
		complete = false
	}

	counts := map[string]int{"active_30d": 0, "warm_31_90d": 0, "cooling_91_180d": 0, "dormant_181d_plus": 0, "never_engaged": 0, "unknown": 0}
	for _, contact := range contacts.Items {
		id := Text(contact["id"])
		when := latest[id]
		if contactWhen := contactLatest[id]; contactWhen.After(when) {
			when = contactWhen
		}
		if !when.IsZero() {
			age := asOf.Sub(when)
			switch {
			case age <= 30*24*time.Hour:
				counts["active_30d"]++
			case age <= 90*24*time.Hour:
				counts["warm_31_90d"]++
			case age <= 180*24*time.Hour:
				counts["cooling_91_180d"]++
			default:
				counts["dormant_181d_plus"]++
			}
			continue
		}
		if complete && !unparseable[id] {
			counts["never_engaged"]++
		} else {
			counts["unknown"]++
		}
	}

	rows := make([]Row, 0, 6)
	for _, name := range []string{"active_30d", "warm_31_90d", "cooling_91_180d", "dormant_181d_plus", "never_engaged", "unknown"} {
		warnings := []string{}
		cohortComplete := complete
		var value any = counts[name]
		if name == "unknown" {
			cohortComplete = true
		} else if !complete {
			value = nil
			warnings = append(warnings, "Activity history or contact scope is incomplete; observed_count is a lower bound and missing engagement is unknown, not never.")
		}
		rows = append(rows, Row{"cohort": name, "count": value, "observed_count": counts[name], "complete": cohortComplete, "warnings": warnings})
	}
	return Row{
		"as_of":         asOf.Format(time.RFC3339),
		"complete":      complete,
		"cohorts":       rows,
		"source_fields": []string{"activity.type=open/click with event timestamp", "contacts.last_engaged_at", "contacts.last_clicked_at", "contacts.last_opened_at"},
		"definitions":   "active <=30d; warm 31-90d; cooling 91-180d; dormant >180d; never requires complete activity evidence; otherwise unknown",
	}
}

// ContactDossier resolves one explicit identity; duplicate normalized emails are never silently selected.
func ContactDossier(s Snapshot, id, email string, now time.Time, maxAge time.Duration) Report {
	r := NewReport("contact-dossier", s)
	r.Require(s, now, maxAge, "contacts", "lists", "memberships", "tags", "contact_tags", "contact_fields", "activity", "unsubscribed")
	matches := []Row{}
	for _, c := range s.Resources["contacts"].Items {
		if id != "" && Text(c["id"]) == id || id == "" && email != "" && Email(c["email"]) == Email(email) {
			matches = append(matches, c)
		}
	}
	if len(matches) != 1 {
		r.Blockers = append(r.Blockers, fmt.Sprintf("identity matched %d contacts; supply an unambiguous --id or --email", len(matches)))
		r.Data["matches"] = matches
		r.Finish()
		return r
	}
	c := matches[0]
	cid := Text(c["id"])
	members, tags, activity := []Row{}, []Row{}, []Row{}
	listIndex := Index(s.Resources["lists"].Items, "id")
	tagIndex := Index(s.Resources["tags"].Items, "id")
	for _, v := range s.Resources["memberships"].Items {
		if Text(v["contact_id"]) == cid {
			members = append(members, Row{"membership": v, "list": listIndex[Text(v["list_id"])]})
		}
	}
	for _, v := range s.Resources["contact_tags"].Items {
		if Text(v["contact_id"]) == cid {
			tags = append(tags, Row{"membership": v, "tag": tagIndex[Text(v["tag_id"])]})
		}
	}
	for _, v := range s.Resources["activity"].Items {
		if Text(v["contact_id"]) == cid {
			activity = append(activity, v)
		}
	}
	for _, name := range []string{"memberships", "contact_tags", "activity"} {
		if !s.Resources[name].ScopesComplete {
			r.Unknowns = append(r.Unknowns, name+": child scopes not proven complete")
		}
	}
	r.Data = Row{"contact": c, "lists": members, "tags": tags, "activity": activity, "suppressed": Suppressed(s)[Email(c["email"])], "custom_field_definitions": s.Resources["contact_fields"].Items}
	r.Finish()
	return r
}
