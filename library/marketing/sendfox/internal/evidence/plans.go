package evidence

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/marketing/sendfox/internal/contract"
	"sort"
	"time"
)

// AutomationPlan compiles a JSON/YAML definition to ordered, inactive requests.
func AutomationPlan(s Snapshot, now time.Time, maxAge time.Duration) Report {
	r := NewReport("automation-plan", s)
	r.Require(s, now, maxAge, "lists", "campaigns")
	op, _ := contract.Match("POST", "/automations")
	body := Row{"active": false}
	for _, key := range []string{"title", "trigger_type", "trigger_list_id", "trigger_campaign_id"} {
		if v, ok := s.Automation[key]; ok {
			body[key] = v
		}
	}
	if s.Automation["active"] == true {
		r.Blockers = append(r.Blockers, "active=true requires separate human approval; emitted plan forces inactive")
	}
	if err := contract.Validate(op.BodySchema, body); err != nil {
		r.Blockers = append(r.Blockers, err.Error())
	}
	trigger := Text(body["trigger_type"])
	ref, resource := "trigger_campaign_id", "campaigns"
	if trigger == "apply_list" {
		ref, resource = "trigger_list_id", "lists"
	}
	if trigger == "" {
		r.Blockers = append(r.Blockers, "trigger_type is required for a useful automation plan")
	} else if Index(s.Resources[resource].Items, "id")[Text(body[ref])] == nil {
		r.Blockers = append(r.Blockers, ref+" must refer to an existing "+resource+" record")
	}
	r.ProposedActions = append(r.ProposedActions, Row{"step": "automation", "method": "POST", "path": "/automations", "body": body, "capture": "automation", "execute": false})
	emails := Rows(s.Automation["emails"])
	if len(emails) == 0 {
		r.Blockers = append(r.Blockers, "automation emails must contain at least one definition")
	}
	emailOp, _ := contract.Match("POST", "/automations/1/emails")
	for i, email := range emails {
		copyEmail := Row{}
		for k, v := range email {
			copyEmail[k] = v
		}
		if _, ok := copyEmail["delay_hours"]; !ok {
			if i == 0 {
				copyEmail["delay_hours"] = 0
			} else {
				copyEmail["delay_hours"] = 24
			}
		}
		if err := contract.Validate(emailOp.BodySchema, copyEmail); err != nil {
			r.Blockers = append(r.Blockers, fmt.Sprintf("email %d: %s", i+1, err))
		}
		r.ProposedActions = append(r.ProposedActions, Row{"step": fmt.Sprintf("email_%d", i+1), "depends_on": "automation", "method": "POST", "path": "/automations/${automation.id}/emails", "body": copyEmail, "execute": false})
	}
	r.Data = Row{"email_count": len(emails), "active": false, "symbolic_references": true, "limitations": []string{"Enrollment/progress transfer is absent from the public API.", "Activation may reschedule stale deliverables; activation is excluded from this plan."}}
	r.Finish()
	return r
}

var InventoryResources = []string{"contacts", "unsubscribed", "lists", "memberships", "tags", "contact_tags", "contact_fields", "campaigns", "campaign_stats", "engagement", "activity", "forms", "domains", "automations", "automation_emails"}

// ExportBundle packages supplied evidence, preserving missing scopes and hashes; not a point-in-time backup guarantee.
func ExportBundle(s Snapshot, now time.Time, maxAge time.Duration) Report {
	r := NewReport("export-bundle", s)
	r.Require(s, now, maxAge, InventoryResources...)
	counts := map[string]int{}
	manifest := []Row{}
	names := []string{}
	for name := range s.Resources {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		v := s.Resources[name]
		counts[name] = len(v.Items)
		manifest = append(manifest, Row{"resource": name, "count": len(v.Items), "sha256": Hash(v.Items), "source": v.Source, "observed_at": v.ObservedAt, "complete": v.Complete, "capped": v.Capped, "pages": v.Pages, "total": v.Total, "scopes_complete": v.ScopesComplete})
	}
	r.Data = Row{"counts": counts, "manifest": manifest, "snapshot": s, "snapshot_sha256": Hash(s), "consistency": "Independent observations; no account-wide point-in-time or tombstone guarantee."}
	r.Finish()
	return r
}

// APIGaps is a curated static comparison against Kit, never an account response.
var APIGaps = []string{"webhooks", "purchases_orders", "snippets_templates", "public_posts_feed", "growth_aggregate_analytics", "richer_behavioral_filters", "automation_enrollment_sequence_progress"}

// MigrationReadiness compares supplied exports and produces evidence, not an executable migration.
func MigrationReadiness(s Snapshot, now time.Time, maxAge time.Duration) Report {
	r := NewReport("migration-readiness", s)
	r.Data["api_gaps"] = APIGaps
	r.Require(s, now, maxAge, "contacts", "unsubscribed", "lists", "tags", "contact_fields", "forms", "automations", "domains")
	if s.Kit == nil {
		r.Blockers = append(r.Blockers, "Kit snapshot missing; supply --kit or input.kit")
		r.Finish()
		return r
	}
	kit := *s.Kit
	sourceReport := NewReport("source", kit)
	sourceReport.Require(kit, now, maxAge, "contacts", "unsubscribed", "lists", "tags", "contact_fields", "forms", "automations")
	for _, u := range sourceReport.Unknowns {
		r.Unknowns = append(r.Unknowns, "Kit "+u)
	}
	source := kit.Resources["contacts"].Items
	target := s.Resources["contacts"].Items
	// Reconciliation must honor both providers' suppression evidence, including
	// source suppressions stored separately from contact records.
	union := Snapshot{Resources: map[string]Resource{"contacts": s.Resources["contacts"]}}
	suppressions := append([]Row{}, s.Resources["unsubscribed"].Items...)
	suppressions = append(suppressions, kit.Resources["unsubscribed"].Items...)
	union.Resources["unsubscribed"] = Resource{Items: suppressions}
	recon := ReconcileCSV(source, union)
	r.Data["reconciliation"] = recon.Data
	sourceSupp, targetSupp := Suppressed(kit), Suppressed(s)
	conflicts := []Row{}
	byEmail := map[string]Row{}
	for _, c := range target {
		byEmail[Email(c["email"])] = c
	}
	for email := range sourceSupp {
		if byEmail[email] != nil && !targetSupp[email] {
			conflicts = append(conflicts, Row{"email": email, "target_id": byEmail[email]["id"], "reason": "source_suppressed_target_not_suppressed"})
		}
	}
	sort.Slice(conflicts, func(i, j int) bool { return Text(conflicts[i]["email"]) < Text(conflicts[j]["email"]) })
	if len(conflicts) > 0 {
		r.Blockers = append(r.Blockers, "suppression conflicts must be resolved before any pilot or cutover")
	}
	r.Data["suppression_audit"] = Row{"source_count": len(sourceSupp), "target_count": len(targetSupp), "conflicts": conflicts}
	mappings := []Row{}
	approved := map[string]string{}
	for _, m := range s.Mappings {
		key := m.Kind + ":" + m.Source
		why := ""
		if !m.Approved {
			why = "not approved"
		} else if old, ok := approved[key]; ok && old != m.Target {
			why = "conflicting mapping"
		} else if Index(kit.Resources[m.Kind].Items, "id")[m.Source] == nil || Index(s.Resources[m.Kind].Items, "id")[m.Target] == nil {
			why = "source or target reference missing"
		} else {
			approved[key] = m.Target
			if m.Kind == "contact_fields" {
				a := Index(kit.Resources[m.Kind].Items, "id")[m.Source]
				b := Index(s.Resources[m.Kind].Items, "id")[m.Target]
				if Text(a["type"]) != Text(b["type"]) {
					why = "immutable field type mismatch"
				}
			}
		}
		mappings = append(mappings, Row{"kind": m.Kind, "source": m.Source, "target": m.Target, "approved": m.Approved, "issue": why})
		if why != "" {
			r.Blockers = append(r.Blockers, key+": "+why)
		}
	}
	suggestions := []Row{}
	for _, kind := range []string{"lists", "tags", "contact_fields", "forms", "automations"} {
		for _, src := range kit.Resources[kind].Items {
			key := kind + ":" + Text(src["id"])
			if approved[key] != "" {
				continue
			}
			r.Blockers = append(r.Blockers, "unmapped source "+key)
			for _, dst := range s.Resources[kind].Items {
				name := Text(src["name"])
				if name == "" {
					name = Text(src["title"])
				}
				targetName := Text(dst["name"])
				if targetName == "" {
					targetName = Text(dst["title"])
				}
				if name != "" && name == targetName {
					suggestions = append(suggestions, Row{"kind": kind, "source": src["id"], "target": dst["id"], "approved": false, "reason": "name-match suggestion only"})
				}
			}
		}
	}
	r.Data["mapping_validation"] = mappings
	r.Data["mapping_suggestions"] = suggestions
	for _, req := range s.Requirements {
		for _, gap := range APIGaps {
			if req == gap {
				r.Blockers = append(r.Blockers, "required parity absent: "+gap)
			}
		}
	}
	delta := Row{"available": false, "added": []Row{}, "changed": []Row{}, "missing_now": []Row{}, "newly_suppressed": []string{}}
	if s.Previous != nil {
		previous := *s.Previous
		previousReport := NewReport("previous", previous)
		previousReport.Require(previous, now, 0, "contacts", "unsubscribed")
		for _, u := range previousReport.Unknowns {
			r.Unknowns = append(r.Unknowns, "Previous Kit "+u)
		}
		old := map[string]Row{}
		current := map[string]Row{}
		for _, c := range previous.Resources["contacts"].Items {
			old[Email(c["email"])] = c
		}
		for _, c := range source {
			current[Email(c["email"])] = c
		}
		added, changed, missing := []Row{}, []Row{}, []Row{}
		for _, c := range source {
			email := Email(c["email"])
			if old[email] == nil {
				added = append(added, c)
			} else if Hash(old[email]) != Hash(c) {
				changed = append(changed, c)
			}
		}
		for _, c := range previous.Resources["contacts"].Items {
			if current[Email(c["email"])] == nil {
				missing = append(missing, c)
			}
		}
		newSupp := []string{}
		oldSupp := Suppressed(previous)
		for email := range sourceSupp {
			if !oldSupp[email] {
				newSupp = append(newSupp, email)
			}
		}
		sort.Strings(newSupp)
		delta = Row{"available": true, "complete": len(previousReport.Unknowns) == 0 && len(sourceReport.Unknowns) == 0, "added": added, "changed": changed, "missing_now": missing, "newly_suppressed": newSupp, "note": "Missing rows are observations, never deletion instructions."}
	} else {
		r.Unknowns = append(r.Unknowns, "previous Kit snapshot missing; late-change delta unavailable")
	}
	r.Data["delta"] = delta
	samples := source
	if len(samples) > 10 {
		samples = samples[:10]
	}
	r.Data["pilot_validation"] = Row{"source_count": len(source), "target_count": len(target), "sample": samples, "actual_sends_verified": false, "domain_evidence": s.Resources["domains"].Items}
	r.Unknowns = append(r.Unknowns, "Live links, current sending-domain readiness, pilot delivery and sequence progress remain unverified")
	r.Data["cutover_checklist"] = []string{"Approve mappings and all declared API gaps", "Verify unsubscribe/suppression parity and refresh late-change delta", "Validate pilot counts, samples, exclusions and links", "Verify sending domain, timezone and account entitlement", "Human approval before form/DNS changes, sends or automation activation", "Retain original provider until acceptance and rollback prerequisites are signed off"}
	r.Data["rollback_artifact"] = Row{"source_snapshot_sha256": Hash(kit), "target_snapshot_sha256": Hash(s.Resources), "source_ids": contactIDs(source), "target_ids": contactIDs(target), "automatic_rollback": false, "limits": []string{"Cannot unsend messages", "Cannot restore unsupported sequence progress", "No tombstone/change-feed guarantee"}, "manual_prerequisites": []string{"Keep original exports and provider configuration", "Record IDs of any subsequently authorized creations", "Preserve suppression updates on both providers"}}
	r.ProposedActions = append(r.ProposedActions, Row{"type": "migration_plan", "execute": false, "steps": []string{"inventory", "approve mappings", "suppression audit", "offline pilot validation", "late-change delta", "human cutover review"}})
	r.Finish()
	return r
}
func contactIDs(rows []Row) []string {
	ids := []string{}
	for _, r := range rows {
		ids = append(ids, Text(r["id"]))
	}
	sort.Strings(ids)
	return ids
}
