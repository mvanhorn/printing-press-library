// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

// export_fhir.go implements the `export-fhir` command: it maps one trial's
// ClinicalTrials.gov registry data to a minimal FHIR R4 ResearchStudy resource.
//
// This is a LIGHTWEIGHT METADATA EXPORT, not a full FHIR-conformant clinical
// dataset. Only a small, high-confidence subset of ResearchStudy fields is
// populated (id, identifier, title, status, phase, condition, sponsor, and a
// documented enrollment extension). It is not validated against a FHIR profile
// and omits the many fields a conformant ResearchStudy may carry.
//
// FHIR R4 ResearchStudy reference:
// https://hl7.org/fhir/R4/researchstudy.html
package cli

import (
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// FHIR system/code-system URIs used in the mapping.
const (
	// Identifier.system for ClinicalTrials.gov NCT ids.
	fhirNCTSystem = "http://clinicaltrials.gov"
	// ResearchStudy.phase coding system (FHIR R4 research-study-phase).
	fhirPhaseSystem = "http://terminology.hl7.org/CodeSystem/research-study-phase"
	// R4 ResearchStudy has no scalar enrollment-count field (ResearchStudy.enrollment
	// is Reference(Group)). We carry the posted target as a documented extension.
	fhirEnrollmentExtURL = "https://clinicaltrials.gov/fhir/StructureDefinition/enrollment-target"
)

// --- Minimal FHIR R4 resource structs (only the fields this export populates) ---

// fhirIdentifier maps to ResearchStudy.identifier (Identifier datatype).
type fhirIdentifier struct {
	System string `json:"system,omitempty"`
	Value  string `json:"value,omitempty"`
}

// fhirCoding maps to the Coding datatype (a member of CodeableConcept.coding).
type fhirCoding struct {
	System  string `json:"system,omitempty"`
	Code    string `json:"code,omitempty"`
	Display string `json:"display,omitempty"`
}

// fhirCodeableConcept maps to the CodeableConcept datatype (ResearchStudy.phase,
// ResearchStudy.condition).
type fhirCodeableConcept struct {
	Coding []fhirCoding `json:"coding,omitempty"`
	Text   string       `json:"text,omitempty"`
}

// fhirReference maps to the Reference datatype (ResearchStudy.sponsor →
// Reference(Organization)). Only display is populated — CT.gov gives a name,
// not a resolvable Organization resource.
type fhirReference struct {
	Display string `json:"display,omitempty"`
}

// fhirExtension maps to the Extension datatype, used here to carry the posted
// enrollment target (no scalar field exists on R4 ResearchStudy).
type fhirExtension struct {
	URL          string `json:"url"`
	ValueInteger int    `json:"valueInteger"`
}

// fhirResearchStudy is the minimal FHIR R4 ResearchStudy resource this command
// emits. Field names and JSON keys follow the R4 element names exactly.
type fhirResearchStudy struct {
	ResourceType string                `json:"resourceType"` // always "ResearchStudy"
	ID           string                `json:"id,omitempty"`
	Identifier   []fhirIdentifier      `json:"identifier,omitempty"`
	Title        string                `json:"title,omitempty"`
	Status       string                `json:"status"` // required (1..1)
	Phase        *fhirCodeableConcept  `json:"phase,omitempty"`
	Condition    []fhirCodeableConcept `json:"condition,omitempty"`
	Sponsor      *fhirReference        `json:"sponsor,omitempty"`
	Extension    []fhirExtension       `json:"extension,omitempty"`
}

// fhirStatusFromCTGov maps a ClinicalTrials.gov OverallStatus to the closest
// FHIR R4 ResearchStudy.status code. status is required (1..1), so unmapped
// values fall back to "active" (the common ongoing state) — documented so a
// reviewer knows the fallback is deliberate, not a coding gap.
func fhirStatusFromCTGov(s string) string {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "COMPLETED":
		return "completed"
	case "TERMINATED":
		// FHIR: "Study terminated early and will not resume."
		return "administratively-completed"
	case "WITHDRAWN":
		// FHIR: "Study terminated before enrolling first participant."
		return "withdrawn"
	case "SUSPENDED":
		return "temporarily-closed-to-accrual"
	case "ACTIVE_NOT_RECRUITING":
		return "closed-to-accrual"
	case "NOT_YET_RECRUITING":
		return "approved"
	case "RECRUITING", "ENROLLING_BY_INVITATION", "AVAILABLE":
		return "active"
	default:
		// Unmapped CT.gov state — status is required, default to active.
		return "active"
	}
}

// fhirPhaseFromCTGov maps CT.gov phase enums to a FHIR R4 research-study-phase
// CodeableConcept. Returns nil when no phase is posted. Unmapped combinations
// are carried as text only rather than emitting an invalid code.
func fhirPhaseFromCTGov(phases []string) *fhirCodeableConcept {
	if len(phases) == 0 {
		return nil
	}
	key := strings.ToUpper(strings.Join(phases, "/"))
	var code, display string
	switch key {
	case "EARLY_PHASE1":
		code, display = "early-phase-1", "Early Phase 1"
	case "PHASE1":
		code, display = "phase-1", "Phase 1"
	case "PHASE1/PHASE2":
		code, display = "phase-1-phase-2", "Phase 1/Phase 2"
	case "PHASE2":
		code, display = "phase-2", "Phase 2"
	case "PHASE2/PHASE3":
		code, display = "phase-2-phase-3", "Phase 2/Phase 3"
	case "PHASE3":
		code, display = "phase-3", "Phase 3"
	case "PHASE4":
		code, display = "phase-4", "Phase 4"
	case "NA", "N/A":
		code, display = "n-a", "N/A"
	default:
		// No matching FHIR code — preserve the raw value as text.
		return &fhirCodeableConcept{Text: strings.Join(phases, "/")}
	}
	return &fhirCodeableConcept{
		Coding: []fhirCoding{{System: fhirPhaseSystem, Code: code, Display: display}},
		Text:   display,
	}
}

// buildResearchStudy maps a normalized Trial to the minimal FHIR ResearchStudy.
func buildResearchStudy(t Trial) fhirResearchStudy {
	rs := fhirResearchStudy{
		ResourceType: "ResearchStudy",
		ID:           t.NCTID,
		Title:        t.Title,
		Status:       fhirStatusFromCTGov(t.Status),
		Phase:        fhirPhaseFromCTGov(t.Phases),
	}
	if t.NCTID != "" {
		rs.Identifier = []fhirIdentifier{{System: fhirNCTSystem, Value: t.NCTID}}
	}
	for _, c := range t.Conditions {
		if strings.TrimSpace(c) != "" {
			rs.Condition = append(rs.Condition, fhirCodeableConcept{Text: c})
		}
	}
	if t.Sponsor != "" {
		rs.Sponsor = &fhirReference{Display: t.Sponsor}
	}
	if t.Enrollment > 0 {
		rs.Extension = []fhirExtension{{URL: fhirEnrollmentExtURL, ValueInteger: t.Enrollment}}
	}
	return rs
}

// pp:data-source live
func newNovelExportFHIRCmd(flags *rootFlags) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "export-fhir <nct-id>",
		Short: "Export a trial's registry metadata as a minimal FHIR R4 ResearchStudy resource (JSON) or a flat CSV row.",
		Long: "Maps one trial's ClinicalTrials.gov registry data to a minimal FHIR R4\n" +
			"ResearchStudy resource: id, identifier, title, status, phase, condition,\n" +
			"sponsor, and a documented enrollment-target extension.\n\n" +
			"This is a LIGHTWEIGHT METADATA EXPORT, not a full FHIR-conformant clinical\n" +
			"dataset. Only a small subset of ResearchStudy fields is populated, and the\n" +
			"output is not validated against a FHIR profile.\n\n" +
			"--format fhir (default) emits the ResearchStudy JSON; --format csv emits a\n" +
			"single flat row (id, title, status, phase, condition, sponsor, enrollment).",
		Example: "  clinical-trials-pp-cli export-fhir NCT04280705\n" +
			"  clinical-trials-pp-cli export-fhir NCT04280705 --format csv",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			switch format {
			case "fhir", "csv":
				// valid
			default:
				return usageErr(fmt.Errorf("invalid --format value %q: must be fhir or csv", format))
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would export a trial as a FHIR R4 ResearchStudy resource")
				return nil
			}
			nctID := strings.TrimSpace(args[0])
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			t, ok, err := ctgovFetchOne(ctx, c, nctID)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if !ok {
				return usageErr(fmt.Errorf("no trial found for %q (check the NCT id)", nctID))
			}

			rs := buildResearchStudy(t)
			if format == "csv" {
				return renderFHIRCSV(cmd, t, rs)
			}
			// Default: FHIR JSON. The resource itself is the output shape;
			// route through printJSONFiltered to honor house output conventions.
			return printJSONFiltered(cmd.OutOrStdout(), rs, flags)
		},
	}
	cmd.Flags().StringVar(&format, "format", "fhir", "Output format: fhir (FHIR R4 ResearchStudy JSON) or csv (flat row)")
	return cmd
}

// renderFHIRCSV writes a single flat CSV row of the mapped fields.
func renderFHIRCSV(cmd *cobra.Command, t Trial, rs fhirResearchStudy) error {
	w := csv.NewWriter(cmd.OutOrStdout())
	header := []string{"id", "title", "status", "phase", "condition", "sponsor", "enrollment"}
	if err := w.Write(header); err != nil {
		return err
	}

	phase := ""
	if rs.Phase != nil {
		phase = rs.Phase.Text
	}
	enrollment := ""
	if t.Enrollment > 0 {
		enrollment = fmt.Sprintf("%d", t.Enrollment)
	}
	row := []string{
		rs.ID,
		rs.Title,
		rs.Status,
		phase,
		strings.Join(t.Conditions, "; "),
		t.Sponsor,
		enrollment,
	}
	if err := w.Write(row); err != nil {
		return err
	}
	w.Flush()
	return w.Error()
}
