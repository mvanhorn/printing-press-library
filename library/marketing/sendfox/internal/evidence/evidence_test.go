package evidence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T) Snapshot {
	t.Helper()
	s, e := Load("../../examples/snapshot.json")
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestWorkflowBehavior(t *testing.T) {
	s := fixture(t)
	now := time.Now()
	for _, tc := range []struct {
		name  string
		run   func() Report
		check func(Report) bool
	}{
		{"preflight", func() Report { return CampaignPreflight(s, now, 0) }, func(r Report) bool {
			return r.Data["eligible_observed_count"] == 1 && len(r.ProposedActions) == 1 && r.ProposedActions[0]["draft"] == true
		}},
		{"review", func() Report { return CampaignReview(s, "42", "non_openers", now, 0) }, func(r Report) bool {
			return r.Data["eligible_observed_count"] == 1 && len(r.Data["exclusions"].([]Row)) == 1
		}},
		{"health", func() Report { return AudienceHealth(s, now, 0) }, func(r Report) bool { return r.Data["contact_count"] == 2 && r.Data["suppressed_identity_count"] == 1 }},
		{"dossier", func() Report { return ContactDossier(s, "1", "", now, 0) }, func(r Report) bool {
			return r.Data["contact"].(map[string]any)["email"] == "reader@example.com" && len(r.Data["lists"].([]Row)) == 1
		}},
		{"automation", func() Report { return AutomationPlan(s, now, 0) }, func(r Report) bool {
			return len(r.ProposedActions) == 3 && r.ProposedActions[0]["body"].(map[string]any)["active"] == false && r.ProposedActions[2]["body"].(map[string]any)["delay_hours"] == 24
		}},
		{"export", func() Report { return ExportBundle(s, now, 0) }, func(r Report) bool {
			return r.Data["counts"].(map[string]int)["contacts"] == 2 && len(r.Data["manifest"].([]Row)) == 15
		}},
		{"migration", func() Report { return MigrationReadiness(s, now, 0) }, func(r Report) bool {
			return r.Data["delta"].(map[string]any)["available"] == true && r.Data["rollback_artifact"].(map[string]any)["automatic_rollback"] == false
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := tc.run()
			if !tc.check(r) {
				b, _ := json.Marshal(r)
				t.Fatal(string(b))
			}
			if r.Findings == nil || r.Blockers == nil || r.Unknowns == nil {
				t.Fatal("nil result arrays")
			}
		})
	}
}
func TestIncompleteEvidenceNeverReady(t *testing.T) {
	s := Snapshot{SchemaVersion: 1, Resources: map[string]Resource{}}
	for _, r := range []Report{CampaignPreflight(s, time.Now(), time.Hour), CampaignReview(s, "42", "non_openers", time.Now(), time.Hour), AudienceHealth(s, time.Now(), time.Hour), ContactDossier(s, "1", "", time.Now(), time.Hour), AutomationPlan(s, time.Now(), time.Hour), ExportBundle(s, time.Now(), time.Hour), GrowthReport(s, time.Now(), time.Hour, 30*24*time.Hour), MigrationReadiness(s, time.Now(), time.Hour)} {
		if r.Ready || len(r.Unknowns)+len(r.Blockers) == 0 {
			t.Fatal(r.Workflow)
		}
	}
}
func TestAdversarialEvidence(t *testing.T) {
	t.Run("ambiguity", func(t *testing.T) {
		s := fixture(t)
		v := s.Resources["contacts"]
		v.Items = append(v.Items, Row{"id": 3, "email": "READER@example.com"})
		s.Resources["contacts"] = v
		r := ContactDossier(s, "", "reader@example.com", time.Now(), 0)
		if r.Ready || len(r.Blockers) == 0 {
			t.Fatal("picked first identity")
		}
	})
	t.Run("exclusion wins", func(t *testing.T) {
		s := fixture(t)
		s.Campaign["excluded_contact_tags"] = []any{3}
		r := CampaignPreflight(s, time.Now(), 0)
		if r.Data["eligible_observed_count"] != 0 {
			t.Fatal(r.Data)
		}
	})
	t.Run("schedule removed", func(t *testing.T) {
		s := fixture(t)
		s.Campaign["scheduled_at"] = "2027-01-01T00:00:00Z"
		r := CampaignPreflight(s, time.Now(), 0)
		if r.Ready {
			t.Fatal("scheduled")
		}
		if _, ok := r.Data["draft"].(map[string]any)["scheduled_at"]; ok {
			t.Fatal("unsafe draft")
		}
	})
	t.Run("stale", func(t *testing.T) {
		s := fixture(t)
		r := AudienceHealth(s, time.Now().Add(48*time.Hour), time.Hour)
		if r.Ready || len(r.Unknowns) == 0 {
			t.Fatal("stale ready")
		}
	})
	t.Run("wrong cohort", func(t *testing.T) {
		s := fixture(t)
		r := CampaignReview(s, "42", "clickers", time.Now(), 0)
		if r.Data["eligible_observed_count"] != 0 {
			t.Fatal("irrelevant cohort")
		}
	})
	t.Run("no links", func(t *testing.T) {
		s := fixture(t)
		delete(s.Resources["campaign_stats"].Items[0], "link_stats")
		r := CampaignReview(s, "42", "non_openers", time.Now(), 0)
		if len(r.Unknowns) == 0 {
			t.Fatal("invented zero")
		}
	})
	t.Run("bad delay", func(t *testing.T) {
		s := fixture(t)
		s.Automation["emails"].([]any)[0].(map[string]any)["delay_hours"] = 5001
		r := AutomationPlan(s, time.Now(), 0)
		if len(r.Blockers) == 0 {
			t.Fatal("invalid delay")
		}
	})
	t.Run("no delta fabricated", func(t *testing.T) {
		s := fixture(t)
		r := MigrationReadiness(s, time.Now(), 0)
		d := r.Data["delta"].(map[string]any)
		if len(d["changed"].([]Row)) != 0 || len(d["added"].([]Row)) != 0 {
			t.Fatal(d)
		}
	})
	t.Run("suppression conflict", func(t *testing.T) {
		s := fixture(t)
		v := s.Resources["unsubscribed"]
		v.Items = []Row{}
		s.Resources["unsubscribed"] = v
		delete(s.Resources["contacts"].Items[1], "unsubscribed_at")
		r := MigrationReadiness(s, time.Now(), 0)
		if len(r.Data["suppression_audit"].(map[string]any)["conflicts"].([]Row)) != 1 {
			t.Fatal(r.Data)
		}
	})
}
func TestDecodeFormatsAndCSV(t *testing.T) {
	for _, text := range []string{`{"schema_version":1,"resources":{}}`, "schema_version: 1\nresources: {}", "---\nschema_version: 1\nresources: {}\n---\n# Note"} {
		s, e := Decode([]byte(text))
		if e != nil || s.SchemaVersion != 1 {
			t.Fatal(e)
		}
	}
	for _, text := range []string{"schema_version: 2", "schema_version: 1\nunknown: true", "schema_version: 1\n---\nschema_version: 1"} {
		if _, e := Decode([]byte(text)); e == nil {
			t.Fatal(text)
		}
	}
	file := filepath.Join(t.TempDir(), "contacts.csv")
	if e := os.WriteFile(file, []byte("email,first,last\nnew@example.com,New,Reader\nNEW@example.com,Duplicate,Reader\ninvalid,No,Email\n"), 0600); e != nil {
		t.Fatal(e)
	}
	rows, e := ReadCSV(file)
	if e != nil {
		t.Fatal(e)
	}
	r := ReconcileCSV(rows, fixture(t))
	if r.Data["to_create_count"] != 1 || r.Data["skip_count"] != 2 {
		t.Fatal(r)
	}
	if rows[0]["first_name"] != "New" {
		t.Fatal(rows)
	}
}
func TestStableHashAndHelpers(t *testing.T) {
	if Hash(Row{"b": 2, "a": 1}) != Hash(Row{"a": 1, "b": 2}) {
		t.Fatal("unstable hash")
	}
	for _, tc := range []struct {
		email string
		valid bool
	}{{"reader@example.com", true}, {"Reader <reader@example.com>", false}, {"invalid", false}} {
		if ValidEmail(tc.email) != tc.valid {
			t.Fatal(tc)
		}
	}
	if Email(" READER@example.com ") != "reader@example.com" || Text(nil) != "" {
		t.Fatal("normalization")
	}
	if len(IDs([]any{1, "2"})) != 2 || len(Rows([]any{Row{"id": 1}})) != 1 || Index([]Row{{"id": 1}}, "id")["1"] == nil {
		t.Fatal("helpers")
	}
	r := NewReport("check", fixture(t))
	r.Finish()
	if !r.Ready {
		t.Fatal(r)
	}
	if !strings.Contains(APIGaps[len(APIGaps)-1], "progress") {
		t.Fatal(APIGaps)
	}
}

func TestMigrationSeparateSourceSuppressionsAndIncompleteDelta(t *testing.T) {
	s := fixture(t)
	c := s.Resources["contacts"]
	c.Items = []Row{}
	s.Resources["contacts"] = c
	u := s.Resources["unsubscribed"]
	u.Items = []Row{}
	s.Resources["unsubscribed"] = u
	kc := s.Kit.Resources["contacts"]
	kc.Items = []Row{{"id": 88, "email": "suppressed@example.com"}}
	s.Kit.Resources["contacts"] = kc
	ku := s.Kit.Resources["unsubscribed"]
	ku.Items = []Row{{"email": "suppressed@example.com"}}
	s.Kit.Resources["unsubscribed"] = ku
	prev := s.Previous.Resources["contacts"]
	prev.Complete = nil
	s.Previous.Resources["contacts"] = prev
	r := MigrationReadiness(s, time.Now(), 0)
	recon := r.Data["reconciliation"].(map[string]any)
	if recon["to_create_count"] != 0 {
		t.Fatal("source suppression included in create plan", recon)
	}
	if r.Data["delta"].(map[string]any)["complete"] != false {
		t.Fatal("incomplete delta marked complete")
	}
}
func TestOversizedEvidenceReadIsBounded(t *testing.T) {
	p := filepath.Join(t.TempDir(), "large.json")
	f, e := os.Create(p)
	if e != nil {
		t.Fatal(e)
	}
	if e = f.Truncate(65 << 20); e != nil {
		t.Fatal(e)
	}
	if e = f.Close(); e != nil {
		t.Fatal(e)
	}
	if _, e = Load(p); e == nil || !strings.Contains(e.Error(), "64 MiB") {
		t.Fatal(e)
	}
}
