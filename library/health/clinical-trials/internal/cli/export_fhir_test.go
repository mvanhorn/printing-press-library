// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"io"
	"testing"

	"github.com/spf13/cobra"
)

// TestNovelExportFHIRHelpWires smoke-tests that the export-fhir command
// resolves at runtime and renders --help without error.
func TestNovelExportFHIRHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"export-fhir", "--help"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("export-fhir --help error = %v (novel command not wired correctly?)", err)
	}
}

func TestFHIRStatusFromCTGov(t *testing.T) {
	cases := map[string]string{
		"COMPLETED":               "completed",
		"TERMINATED":              "administratively-completed",
		"WITHDRAWN":               "withdrawn",
		"SUSPENDED":               "temporarily-closed-to-accrual",
		"ACTIVE_NOT_RECRUITING":   "closed-to-accrual",
		"NOT_YET_RECRUITING":      "approved",
		"RECRUITING":              "active",
		"ENROLLING_BY_INVITATION": "active",
		"SOMETHING_UNKNOWN":       "active", // documented fallback
		"":                        "active",
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			if got := fhirStatusFromCTGov(in); got != want {
				t.Errorf("fhirStatusFromCTGov(%q) = %q, want %q", in, got, want)
			}
		})
	}
}

func TestFHIRPhaseFromCTGov(t *testing.T) {
	t.Run("no phase returns nil", func(t *testing.T) {
		if got := fhirPhaseFromCTGov(nil); got != nil {
			t.Errorf("got %+v, want nil", got)
		}
	})

	t.Run("mapped phase carries system coding", func(t *testing.T) {
		cc := fhirPhaseFromCTGov([]string{"PHASE3"})
		if cc == nil || len(cc.Coding) != 1 {
			t.Fatalf("got %+v, want one coding", cc)
		}
		if cc.Coding[0].System != fhirPhaseSystem {
			t.Errorf("System = %q, want %q", cc.Coding[0].System, fhirPhaseSystem)
		}
		if cc.Coding[0].Code != "phase-3" {
			t.Errorf("Code = %q, want phase-3", cc.Coding[0].Code)
		}
	})

	t.Run("combination phase maps to compound code", func(t *testing.T) {
		cc := fhirPhaseFromCTGov([]string{"PHASE1", "PHASE2"})
		if cc == nil || len(cc.Coding) != 1 || cc.Coding[0].Code != "phase-1-phase-2" {
			t.Fatalf("got %+v, want phase-1-phase-2", cc)
		}
	})

	t.Run("unmapped phase carried as text only", func(t *testing.T) {
		cc := fhirPhaseFromCTGov([]string{"WEIRD_PHASE"})
		if cc == nil {
			t.Fatal("got nil, want text-only concept")
		}
		if len(cc.Coding) != 0 {
			t.Errorf("Coding = %+v, want none for unmapped phase", cc.Coding)
		}
		if cc.Text == "" {
			t.Error("Text should carry the raw value")
		}
	})
}

func TestBuildResearchStudy(t *testing.T) {
	t.Run("full trial maps every populated field", func(t *testing.T) {
		trial := Trial{
			NCTID:      "NCT04280705",
			Title:      "A Study",
			Status:     "RECRUITING",
			Phases:     []string{"PHASE3"},
			Conditions: []string{"Cancer", "Melanoma"},
			Sponsor:    "Acme Labs",
			Enrollment: 300,
		}
		rs := buildResearchStudy(trial)

		if rs.ResourceType != "ResearchStudy" {
			t.Errorf("ResourceType = %q, want ResearchStudy", rs.ResourceType)
		}
		if rs.ID != "NCT04280705" {
			t.Errorf("ID = %q", rs.ID)
		}
		if len(rs.Identifier) != 1 || rs.Identifier[0].System != fhirNCTSystem || rs.Identifier[0].Value != "NCT04280705" {
			t.Errorf("Identifier = %+v", rs.Identifier)
		}
		if rs.Status != "active" {
			t.Errorf("Status = %q, want active", rs.Status)
		}
		if len(rs.Condition) != 2 {
			t.Errorf("Condition len = %d, want 2", len(rs.Condition))
		}
		if rs.Sponsor == nil || rs.Sponsor.Display != "Acme Labs" {
			t.Errorf("Sponsor = %+v", rs.Sponsor)
		}
		if len(rs.Extension) != 1 || rs.Extension[0].URL != fhirEnrollmentExtURL || rs.Extension[0].ValueInteger != 300 {
			t.Errorf("Extension = %+v", rs.Extension)
		}
	})

	t.Run("missing fields are omitted, not invented", func(t *testing.T) {
		trial := Trial{NCTID: "NCT1", Status: "COMPLETED"}
		rs := buildResearchStudy(trial)
		if rs.Phase != nil {
			t.Errorf("Phase = %+v, want nil", rs.Phase)
		}
		if rs.Condition != nil {
			t.Errorf("Condition = %+v, want nil", rs.Condition)
		}
		if rs.Sponsor != nil {
			t.Errorf("Sponsor = %+v, want nil", rs.Sponsor)
		}
		if rs.Extension != nil {
			t.Errorf("Extension = %+v, want nil (zero enrollment)", rs.Extension)
		}
	})

	t.Run("JSON serialization uses R4 element names", func(t *testing.T) {
		trial := Trial{NCTID: "NCT1", Status: "COMPLETED", Enrollment: 50}
		rs := buildResearchStudy(trial)
		b, err := json.Marshal(rs)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}
		var obj map[string]any
		if err := json.Unmarshal(b, &obj); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if obj["resourceType"] != "ResearchStudy" {
			t.Errorf("resourceType = %v", obj["resourceType"])
		}
		if _, ok := obj["status"]; !ok {
			t.Error("status must always be present (R4 requires 1..1)")
		}
		// Empty condition/phase/sponsor must be omitted from JSON.
		for _, k := range []string{"phase", "condition", "sponsor"} {
			if _, ok := obj[k]; ok {
				t.Errorf("key %q should be omitted when empty", k)
			}
		}
		ext, ok := obj["extension"].([]any)
		if !ok || len(ext) != 1 {
			t.Fatalf("extension = %v, want one entry", obj["extension"])
		}
		e := ext[0].(map[string]any)
		if e["url"] != fhirEnrollmentExtURL {
			t.Errorf("extension url = %v", e["url"])
		}
		if e["valueInteger"].(float64) != 50 {
			t.Errorf("extension valueInteger = %v, want 50", e["valueInteger"])
		}
	})
}

func TestRenderFHIRCSV(t *testing.T) {
	trial := Trial{
		NCTID:      "NCT1",
		Title:      "A Study",
		Status:     "RECRUITING",
		Phases:     []string{"PHASE3"},
		Conditions: []string{"Cancer", "Melanoma"},
		Sponsor:    "Acme Labs",
		Enrollment: 300,
	}
	rs := buildResearchStudy(trial)

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	if err := renderFHIRCSV(cmd, trial, rs); err != nil {
		t.Fatalf("renderFHIRCSV error: %v", err)
	}

	rows, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatalf("csv parse error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2 (header + data)", len(rows))
	}
	wantHeader := []string{"id", "title", "status", "phase", "condition", "sponsor", "enrollment"}
	for i, h := range wantHeader {
		if rows[0][i] != h {
			t.Errorf("header[%d] = %q, want %q", i, rows[0][i], h)
		}
	}
	data := rows[1]
	if data[0] != "NCT1" || data[2] != "active" || data[4] != "Cancer; Melanoma" || data[6] != "300" {
		t.Errorf("data row = %v", data)
	}
}
