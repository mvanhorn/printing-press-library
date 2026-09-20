// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
// Patch record to be written at .printing-press-patches/nepra-verify-fetch-gate.json;
// it is not in this change because the writable set for this task is source only.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraparse"
)

// verifyManifestEntry is one surface as the manifest print renders it.
//
// The internals are pointers so that "never measured" is an absent key rather
// than a zero. Four of the seven reachable generation years have no measured
// internals at all, and printing 0 rows for them would be worse than printing
// nothing.
type verifyManifestEntry struct {
	Surface string `json:"surface"`
	Kind    string `json:"kind"`
	Label   string `json:"label"`
	Path    string `json:"path"`
	// FY and ExpectedBandLabel are null for a sheet surface rather than
	// absent. Not one key is omitted anywhere in this struct: --compact
	// projects a list of objects through a keys-present-in-80%-of-rows rule,
	// and fy appears on only 7 of the 11 surfaces, so an omitted key would
	// be dropped from the whole listing while an explicit null survives.
	FY                *string `json:"fy"`
	ExpectedBandLabel *string `json:"expected_band_label"`
	ByteFloor         int     `json:"byte_floor"`
	ByteFloorAsOf     string  `json:"byte_floor_measured_on"`
	RowFloor          *int    `json:"row_floor"`
	Asserted          bool    `json:"internals_asserted"`
	InternalsAsOf     *string `json:"internals_measured_on"`

	// A nil pointer here means NEVER MEASURED and serialises as null. It is
	// never a zero, because a zero row count is a measurement.
	TableRows     *int `json:"table_rows"`
	RawCells      *int `json:"raw_cells"`
	Plants        *int `json:"plants"`
	PhysicalWidth *int `json:"physical_width"`
	NBSPBytes     *int `json:"nbsp_0xa0_bytes"`
	GridRows      *int `json:"grid_rows"`
	GridRawCells  *int `json:"grid_raw_cells"`

	SumPassed   *int     `json:"sum_gwh_passed"`
	SumEligible *int     `json:"sum_gwh_eligible"`
	Ineligible  *int     `json:"ineligible_rows"`
	SigmaSumGWh *float64 `json:"sigma_sum_gwh"`

	// NotAssertedReason is set exactly when Asserted is false.
	NotAssertedReason *string `json:"not_asserted_reason"`
}

const verifyNotAssertedReason = "No committed fixture and no second observation of this year's " +
	"internals. Re-measuring the four unfixtured years live showed their internals genuinely " +
	"differ from the three fixtured ones (FY2018-19 and FY2019-20 are 32 physical columns wide; " +
	"FY2021-22 fails the GWh sum identity on 9 of 125 rows), so they cannot inherit another " +
	"year's expectations. verify computes and reports their internals and cannot fail on them."

func intPtr(v int) *int           { return &v }
func floatPtr(v float64) *float64 { return &v }
func strPtr(v string) *string     { return &v }

// verifyManifestPayload is what --manifest prints. It makes no request.
type verifyManifestPayload struct {
	AsOf string `json:"manifest_as_of"`
	// Fingerprint is the 32-column canonical header, joined with "|", and
	// its SHA-256. This is what a schema-drift refusal compares against.
	Fingerprint       string `json:"column_fingerprint"`
	FingerprintSHA256 string `json:"column_fingerprint_sha256"`
	LogicalColumns    int    `json:"logical_columns"`

	Surfaces          []verifyManifestEntry   `json:"surfaces"`
	Decoys            []verifyDecoy           `json:"decoys"`
	UnpublishedYears  []verifyUnpublishedYear `json:"unpublished_years"`
	StaleAfterDefault int                     `json:"stale_after_default_days"`

	NewestPublishedFY   string `json:"newest_published_fy"`
	NewestFYCoverageEnd string `json:"newest_fy_coverage_end"`

	DeclaredGaps []verifyGap `json:"declared_gaps"`
	Note         string      `json:"note"`
}

// verifyDeclaredGaps are the questions this command does not answer. They are
// printed with the manifest so the gate's own limits are as discoverable as
// its expectations.
func verifyDeclaredGaps() []verifyGap {
	return []verifyGap{
		{
			Gap: "there is no artifact store to verify",
			Reason: "The scaffolded description said \"assert every cached artifact\". This CLI has " +
				"no sync command and no artifact store; the local SQLite database holds only the " +
				"teach loop's rows and the HTTP cache is opaque with no enumeration accessor. " +
				"verify is a FETCH gate. root.go's highlights block and SKILL.md still carry the " +
				"old sentence and are generated files.",
		},
		{
			Gap:    "row, cell and plant counts for FY2018-19, FY2019-20, FY2021-22 and FY2022-23",
			Reason: verifyNotAssertedReason,
		},
		{
			Gap: "the IEA side of --crosscheck iea",
			Reason: "It needs a request to api.iea.org, a third-party host outside this CLI's " +
				"governance review, and no fixture of the series exists here to test the shape " +
				"assertion against. The NEPRA side and the four reasons the two cannot be " +
				"subtracted are reported instead.",
		},
		{
			Gap: "the PER PDF surface",
			Reason: "Deliberately outside the gated set. Only one of roughly thirteen PER paths has " +
				"ever been observed at 200, two research documents disagree on the base path, and " +
				"a PDF returns wrapped in a base64 envelope so len(body) would measure the " +
				"envelope rather than the document. A byte floor over that would be a floor on " +
				"the wrong number.",
		},
		{
			Gap: "exit code 6 as a stable contract",
			Reason: "6 is this command's proposal for a gate refusal, chosen because 2/3/4/5/7/10 " +
				"are taken and a cron needs \"NEPRA is down\", \"NEPRA never published this\" and " +
				"\"NEPRA served something that failed the gate\" to be three different codes. It " +
				"is not yet an established convention in this CLI.",
		},
	}
}

func verifyBuildManifestPayload() verifyManifestPayload {
	fp := nepraparse.HeaderFingerprint{Columns: nepraparse.CanonicalColumns()}.Fingerprint()
	p := verifyManifestPayload{
		AsOf:                verifyManifestAsOf,
		Fingerprint:         fp,
		FingerprintSHA256:   verifyFingerprintSHA256(fp),
		LogicalColumns:      nepraparse.LogicalColumns,
		Decoys:              verifyDecoys,
		UnpublishedYears:    verifyUnpublishedYears,
		StaleAfterDefault:   verifyStaleAfterDefault,
		NewestPublishedFY:   verifyNewestPublishedFY,
		NewestFYCoverageEnd: verifyNewestFYCoverageEnd,
		DeclaredGaps:        verifyDeclaredGaps(),
		Note: "Byte floors are FLOORS measured on the dates shown, not equalities. Internals are " +
			"asserted only where internals_asserted is true. No request was made to produce this.",
	}
	for _, s := range verifySurfaces {
		e := verifyManifestEntry{
			Surface: s.ID, Kind: s.Kind, Label: s.Label, Path: s.Path,
			ByteFloor: s.ByteFloor, ByteFloorAsOf: s.ByteFloorAsOf,
			Asserted: s.Asserted,
		}
		if s.FY != "" {
			e.FY = strPtr(s.FY)
			e.ExpectedBandLabel = strPtr(s.ExpectedBandLabel)
		}
		if s.Kind == verifyKindSheet {
			e.RowFloor = intPtr(s.RowFloor)
		}
		if in := s.Internals; in != nil {
			e.InternalsAsOf = strPtr(in.InternalsAsOf)
			if s.Kind == verifyKindWorkbook {
				e.TableRows = intPtr(in.TableRows)
				e.RawCells = intPtr(in.RawCells)
				e.Plants = intPtr(in.Plants)
				e.PhysicalWidth = intPtr(in.PhysicalWidth)
				e.NBSPBytes = intPtr(in.NBSPBytes)
				e.SumPassed = intPtr(in.SumPassed)
				e.SumEligible = intPtr(in.SumEligible)
				e.Ineligible = intPtr(in.Ineligible)
				e.SigmaSumGWh = floatPtr(in.SigmaSumGWh)
			} else {
				e.GridRows = intPtr(in.GridRows)
				e.GridRawCells = intPtr(in.GridRawCells)
			}
		} else {
			e.NotAssertedReason = strPtr(verifyNotAssertedReason)
		}
		p.Surfaces = append(p.Surfaces, e)
	}
	return p
}

// verifyPrintManifest prints the frozen expectations and makes NO request.
func verifyPrintManifest(cmd *cobra.Command, flags *rootFlags) error {
	p := verifyBuildManifestPayload()
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		w := cmd.ErrOrStderr()
		fmt.Fprintf(w, "Expectation manifest, measured %s. No request made.\n", p.AsOf)
		fmt.Fprintf(w, "%d logical columns, fingerprint sha256 %s\n", p.LogicalColumns, p.FingerprintSHA256)
		for _, e := range p.Surfaces {
			asserted := "report-only"
			if e.Asserted {
				asserted = "asserted"
			}
			fmt.Fprintf(w, "  %-14s %-20s floor %8d B (%s)  %s\n",
				e.Surface, e.Kind, e.ByteFloor, e.ByteFloorAsOf, asserted)
		}
		for _, d := range p.Decoys {
			fmt.Fprintf(w, "  decoy %-15s %6d B  HTTP %d\n", d.ID, d.Bytes, d.HTTPStatus)
		}
		for _, u := range p.UnpublishedYears {
			fmt.Fprintf(w, "  unpublished FY%-8s %s\n", u.FY, u.Evidence)
		}
		for _, g := range p.DeclaredGaps {
			fmt.Fprintf(w, "  DECLARED GAP: %s\n", g.Gap)
		}
		fmt.Fprintf(w, "Pass --fy <year>, --surface <id> or --all to gate a live fetch.\n")
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return printOutputWithFlagsMeta(cmd.OutOrStdout(), raw, flags,
		map[string]any{"source": "manifest", "manifest_as_of": p.AsOf})
}
