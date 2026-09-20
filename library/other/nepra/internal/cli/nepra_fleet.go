// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
// The registered constructor lives in fleet.go and delegates here.
//
// pp:data-source local
// Computed entirely from the embedded crosswalk: no request, no store read.
// `--data-source live` is refused (exit 2) rather than silently ignored.
// Repeated from fleet.go so the annotation sits in the file that declares
// the cobra.Command, which is where the surface scanner resolves it.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraparse"
	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraxwalk"
)

// nepraFleetPublishedFYs are the fiscal years whose generation workbooks are
// REACHABLE on nepra.org.pk, measured during the survey and recorded in
// NEPRA_HANDOFF.md "SETTLED FACTS / GENERATION URL": seven years, FY2017-18
// through FY2023-24, with FY2016-17, FY2024-25 and FY2025-26 all returning 404.
//
// It is used ONLY to classify a fiscal year this crosswalk never indexed:
// "published but not indexed here" is a different fact from "NEPRA never
// published it", and collapsing the two would let the command imply the site
// has data it does not have, or that it lacks data it does have. Nothing is
// fetched.
var nepraFleetPublishedFYs = []string{
	"2017-18", "2018-19", "2019-20", "2020-21", "2021-22", "2022-23", "2023-24",
}

// nepraFleetSNoNote is the S.No warning. The numbers are the crosswalk's own
// curated `sno_is_not_a_key` prose, which no exported accessor reaches
// (internal/nepraxwalk/resolve.go:22-32 keeps the table struct unexported), and
// they are independently asserted in internal/nepraxwalk/fixture_test.go:110-155.
// The panel ALSO recomputes an S.No-stability figure from the embedded rows at
// runtime and reports it as an assertion, so this prose is never the only claim.
const nepraFleetSNoNote = "S.No is an in-year display ordinal, NOT a cross-year key: of the 107 plant names common to " +
	"the FY2017-18 and FY2023-24 workbooks, ZERO keep the same S.No (AES Lalpir 1->36, Allai Khwar 56->8, " +
	"Kot Addu 9->33, Kohinoor 12->35, Narowal 11->58). This panel is keyed on name plus a curated alias table."

// nepraFleetHeldOutNote explains the held-out fiscal year. The out-of-sample
// figures are asserted in internal/nepraxwalk/coverage_test.go:217-248 and the
// megawatt figure was re-summed from
// internal/nepraxwalk/testdata/heldout_2020-21.json (105 numeric rows totalling
// 36,902.00 MW, plus 3 "Export to K.Electric" cells, 108 rows in all). That
// fixture is NOT embedded and has no accessor, so the command cannot read it at
// runtime; it is quoted here to prove the unmeasured column is not a zero.
const nepraFleetHeldOutNote = "FY2020-21 was withheld from the crosswalk on purpose so it can serve as an out-of-sample " +
	"test of the alias table: 107 of its 108 published names resolve (99.1%), the single decline being " +
	"\"(NPPCL) - Balloki\". Its workbook publishes 36,902.00 MW over 105 numeric rows (plus 3 " +
	"\"Export to K.Electric\" cells), so an unmeasured column here is a limit of this index, not a finding of zero."

// nepraFleetPublicationIntervalNote is attached to every plant, because
// valid_to_fy is empty on all 133 rows including the 12 DELICENSED and 1
// DECOMMISSIONED FY2023-24 plants (internal/nepraxwalk/model.go:173-184).
const nepraFleetPublicationIntervalNote = "valid_to_fy is a PUBLICATION interval, never an operating claim. It is empty for " +
	"ALL 133 crosswalk rows, delicensed plants included. Operating status comes only from the fiscal year's block_status."

// nepraFleetValidFromNote fires on a plant first observed in the latest
// observed fiscal year. Only two of the seven published workbooks were fetched,
// so a first observation is not a commissioning date.
const nepraFleetValidFromNote = "valid_from_fy is the first fetched workbook this identity appears in, NOT a commissioning date: " +
	"only FY2017-18 and FY2023-24 were ever fetched, so this means \"absent from the FY2017-18 workbook\"."

const (
	fleetCoverageObserved  = "observed"
	fleetCoverageHeldOut   = "held_out"
	fleetCoverageNotIndexd = "published_not_indexed"
	fleetCoverageNotPub    = "not_published"
)

const (
	fleetParentKindAuto = "auto"
	fleetParentKindPSX  = "psx-symbol"
)

// fleetMaxFYSpan caps --fy-range so a typo cannot enumerate centuries of
// columns that are all unmeasured anyway.
const fleetMaxFYSpan = 20

// ---------------------------------------------------------------------------
// Wire shapes
// ---------------------------------------------------------------------------

type fleetEnvelope struct {
	Meta    fleetMeta    `json:"meta"`
	Results fleetResults `json:"results"`
}

type fleetMeta struct {
	// Source is "computed": every figure comes from the embedded crosswalk,
	// with no request and no local store read.
	Source    string             `json:"source"`
	Basis     string             `json:"basis"`
	Crosswalk fleetCrosswalkMeta `json:"crosswalk"`
}

type fleetCrosswalkMeta struct {
	SchemaValid      bool     `json:"schema_valid"`
	SchemaError      string   `json:"schema_error,omitempty"`
	Rows             int      `json:"rows"`
	Observations     int      `json:"observations"`
	ObservedFYs      []string `json:"observed_fys"`
	HeldOutFYs       []string `json:"held_out_fys"`
	LatestObservedFY string   `json:"latest_observed_fy"`
	PublishedFYs     []string `json:"published_fys"`
	SNoNote          string   `json:"sno_note"`
}

type fleetResults struct {
	Selector     fleetSelector     `json:"selector"`
	Resolution   fleetResolution   `json:"resolution"`
	AsOf         fleetAsOf         `json:"as_of"`
	Plants       []fleetPlant      `json:"plants"`
	FiscalYears  []fleetFiscalYear `json:"fiscal_years"`
	Assertions   []fleetAssertion  `json:"assertions"`
	DeclaredGaps []fleetGap        `json:"declared_gaps"`
}

type fleetSelector struct {
	Kind       string `json:"kind"`
	Value      string `json:"value"`
	ParentKind string `json:"parent_kind,omitempty"`
	FYRange    string `json:"fy_range"`
}

type fleetResolution struct {
	Token           string   `json:"token,omitempty"`
	TokenKind       string   `json:"token_kind,omitempty"`
	ParentName      string   `json:"parent_name,omitempty"`
	PSXTicker       string   `json:"psx_ticker,omitempty"`
	ListedStatus    string   `json:"listed_status,omitempty"`
	Status          string   `json:"status"`
	PlantsReached   int      `json:"plants_reached"`
	MatchKind       string   `json:"match_kind,omitempty"`
	MatchedName     string   `json:"matched_name,omitempty"`
	NormalizedQuery string   `json:"normalized_query,omitempty"`
	ScopeNote       string   `json:"scope_note,omitempty"`
	AbsenceNote     string   `json:"absence_note,omitempty"`
	ProbeTokens     []string `json:"probe_tokens,omitempty"`
	Caveat          string   `json:"caveat,omitempty"`
}

type fleetAsOf struct {
	AsOfFY              string `json:"as_of_fy"`
	Basis               string `json:"basis"`
	OperatingPlants     int    `json:"operating_plants"`
	NonOperatingPlants  int    `json:"non_operating_plants"`
	StatusUnknownPlants int    `json:"status_unknown_plants"`
	NotObservedPlants   int    `json:"not_observed_plants"`
	// The two sums are reported separately and are never added. Each is an
	// nepraxwalk.MWSum, whose MarshalJSON omits `mw` entirely when the sum is
	// not a measurement, so an unobserved fleet cannot be read as 0 MW.
	CapacityOperating    nepraxwalk.MWSum `json:"capacity_operating"`
	CapacityNonOperating nepraxwalk.MWSum `json:"capacity_non_operating"`
	DoNotAdd             string           `json:"do_not_add"`
}

type fleetPlant struct {
	CanonicalName           string         `json:"canonical_name"`
	Aliases                 []string       `json:"aliases,omitempty"`
	ParentName              string         `json:"parent_name,omitempty"`
	PSXTicker               string         `json:"psx_ticker,omitempty"`
	ListedStatus            string         `json:"listed_status"`
	Confidence              string         `json:"confidence"`
	ValidFromFY             string         `json:"valid_from_fy"`
	ValidToFY               string         `json:"valid_to_fy"`
	PublicationIntervalNote string         `json:"publication_interval_note"`
	ValidFromNote           string         `json:"valid_from_note,omitempty"`
	Evidence                string         `json:"evidence"`
	ByFY                    []fleetPlantFY `json:"by_fy"`
}

// fleetPlantFY is one plant-year cell. Every field below `Reason` is a pointer
// or omitempty so an UNOBSERVED year carries no status, no operating flag and
// NO capacity key at all: absence from a workbook must not render as a false,
// a zero, or an empty string that reads like data.
type fleetPlantFY struct {
	FY                string            `json:"fy"`
	Observed          bool              `json:"observed"`
	Reason            string            `json:"reason,omitempty"`
	SNo               *int              `json:"sno,omitempty"`
	PublishedName     string            `json:"published_name,omitempty"`
	Technology        string            `json:"technology,omitempty"`
	Fuel              string            `json:"fuel,omitempty"`
	Status            string            `json:"status,omitempty"`
	Operating         *bool             `json:"operating,omitempty"`
	StatusKnown       *bool             `json:"status_known,omitempty"`
	InstalledCapacity *nepraparse.Value `json:"installed_capacity,omitempty"`
}

type fleetFiscalYear struct {
	FY                    string           `json:"fy"`
	FYObserved            bool             `json:"fy_observed"`
	CoverageClass         string           `json:"coverage_class"`
	PlantsInFleet         int              `json:"plants_in_fleet"`
	ObservedInFY          int              `json:"observed_in_fy"`
	NoObservationInFY     int              `json:"no_observation_in_fy"`
	CapacityReported      nepraxwalk.MWSum `json:"capacity_reported"`
	CapacityOperating     nepraxwalk.MWSum `json:"capacity_operating"`
	CapacityNonOperating  nepraxwalk.MWSum `json:"capacity_non_operating"`
	CapacityStatusUnknown nepraxwalk.MWSum `json:"capacity_status_unknown"`
	CapacityNotReported   int              `json:"capacity_not_reported"`
	CapacityStatusCell    int              `json:"capacity_status_cell"`
	CapacityUnknownText   int              `json:"capacity_unknown_text"`
	CapacityBasis         string           `json:"capacity_basis"`
	ClassNote             string           `json:"class_note,omitempty"`
	// OverstatementPct is how much bigger capacity_reported is than
	// capacity_operating, as a percentage. It is present only when BOTH sums
	// are measurements and operating capacity is above zero. It is the one
	// number that quantifies the error this panel exists to prevent, and it is
	// a ratio of two sums the crosswalk computed — the CLI mints no MW total.
	OverstatementPct *float64 `json:"published_vs_operating_overstatement_pct,omitempty"`
}

type fleetAssertion struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

// fleetGap is a promise this command does NOT keep, with the reason. It ships
// in the payload so an agent reading the JSON learns the limit from the command
// rather than from a README.
type fleetGap struct {
	Question string `json:"question"`
	Verdict  string `json:"verdict"`
	Reason   string `json:"reason"`
	Instead  string `json:"instead,omitempty"`
}

type fleetRefusal struct {
	Error fleetRefusalBody `json:"error"`
}

type fleetRefusalBody struct {
	Kind               string                      `json:"kind"`
	Token              string                      `json:"token,omitempty"`
	Query              string                      `json:"query,omitempty"`
	Reason             string                      `json:"reason"`
	Hazard             string                      `json:"hazard,omitempty"`
	Candidates         []nepraxwalk.TokenCandidate `json:"candidates,omitempty"`
	Recovery           []string                    `json:"recovery,omitempty"`
	DeclinedName       string                      `json:"declined_name,omitempty"`
	FuzzyMatching      string                      `json:"fuzzy_matching,omitempty"`
	AbsentParentHint   string                      `json:"absent_parent_hint,omitempty"`
	CounterFact        string                      `json:"counter_fact,omitempty"`
	NearDuplicateGroup []string                    `json:"near_duplicate_group,omitempty"`
	KnownSymbols       []string                    `json:"known_symbols,omitempty"`
}

// ---------------------------------------------------------------------------
// Command
// ---------------------------------------------------------------------------

// newNepraFleetCmd builds the operator/plant fleet panel.
//
// Everything it prints comes from the embedded crosswalk
// (internal/nepraxwalk/crosswalk.json, 133 rows / 241 observations). There is
// no request, no store read and no parser run, so `--data-source live` is
// refused rather than faked.
//
// The three things it is built to refuse are worth stating up front, because
// each of them is a wrong answer that looks right:
//
//   - capacity_operating and capacity_non_operating are NEVER added. HUBC's
//     FY2023-24 fleet is 2,289.00 MW generating over 5 plants plus 1,292.00 MW
//     of DELICENSED Hub Power that generates nothing.
//   - the bare token KEL is refused, not picked. It is K-Electric's PSX symbol
//     AND NEPRA's acronym for Kohinoor Energy Limited (PSX KOHE).
//   - a fiscal year the crosswalk never indexed comes back UNMEASURED, with the
//     `mw` key absent, never as 0 MW.
func newNepraFleetCmd(flags *rootFlags) *cobra.Command {
	var (
		parent     string
		parentKind string
		plant      string
		fyRange    string
		strict     bool
	)

	cmd := &cobra.Command{
		Use:   "fleet",
		Short: "Follow one plant or a listed operator's whole fleet across every published fiscal year",
		Long: "One plant, or one PSX-listed operator's whole fleet, tracked across NEPRA's re-sorted and\n" +
			"renamed generation workbooks.\n\n" +
			"Delicensed megawatts are named and held OUT of live capacity; operating and non-operating\n" +
			"capacity are reported separately and must never be added. A fiscal year this crosswalk never\n" +
			"indexed comes back unmeasured — the `mw` key is absent — never as 0 MW. A token that reads as\n" +
			"both a PSX symbol and a NEPRA plant acronym (KEL, AGL, APL, SPL, AEL, HEPL) is refused with both\n" +
			"candidates rather than guessed.\n\n" +
			"Computed entirely from the embedded crosswalk: no request is made. With no selector it prints\n" +
			"the catalogue.",
		Example: "  nepra-pp-cli fleet\n" +
			"  nepra-pp-cli fleet --parent HUBC\n" +
			"  nepra-pp-cli fleet --parent HUBC --agent | jq '.results.as_of.capacity_operating'\n" +
			"  nepra-pp-cli fleet --parent KEL --parent-kind psx-symbol\n" +
			"  nepra-pp-cli fleet --plant \"Kotri Power Station\" --fy-range 2017-18:2023-24",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "--parent=HUBC",
			"pp:typed-exit-codes": "true",
			"pp:novel-hand-coded": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// (a) Help-only branch: with NO flags of its own the catalogue is
			// useful and makes no request. A flag set without a selector is
			// NOT help-only — it is a usage error, and falls through to (d).
			noSelector := strings.TrimSpace(parent) == "" && strings.TrimSpace(plant) == ""
			var modifierSet bool
			for _, f := range []string{"parent-kind", "fy-range", "strict"} {
				if cmd.Flags().Changed(f) {
					modifierSet = true
				}
			}
			if noSelector && !modifierSet {
				return fleetPrintCatalogue(cmd, flags)
			}
			// (b) Dry run.
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "fleet")
			}
			// (c) This command has no live leg at all; say so rather than
			// pretending --data-source live changed anything.
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return usageErr(err)
			}
			// (d) Input validation.
			sel, err := fleetValidateSelector(parent, parentKind, plant, cmd.Flags().Changed("parent-kind"))
			if err != nil {
				return usageErr(err)
			}
			fys, err := fleetFiscalYearLabels(fyRange)
			if err != nil {
				return usageErr(err)
			}
			sel.FYRange = strings.Join(fys, ",")

			// (e) Resolve, (f) build, (g) render.
			return fleetRun(cmd, flags, sel, fys, strict)
		},
	}

	cmd.Flags().StringVar(&parent, "parent", "",
		"PSX symbol ("+strings.Join(nepraxwalk.Tickers(), ", ")+") or an UNAMBIGUOUS NEPRA plant acronym (TEL, CPHGCL, HCPC...). Mutually exclusive with --plant")
	cmd.Flags().StringVar(&parentKind, "parent-kind", fleetParentKindAuto,
		"How to read --parent: auto (refuses a token ambiguous across the PSX-symbol and NEPRA-acronym namespaces) or psx-symbol (PSX namespace only; the only route to an operator absent from these workbooks)")
	cmd.Flags().StringVar(&plant, "plant", "",
		"Published plant name, verbatim, canonical or any recorded alias. NBSP padding, Excel's newline+indent, case and trailing periods are tolerated; there is NO fuzzy matching. Mutually exclusive with --parent")
	cmd.Flags().StringVar(&fyRange, "fy-range", "",
		"Inclusive fiscal-year window, e.g. 2017-18:2023-24. Default: the crosswalk's observed years only. A year it never observed returns fy_observed=false and UNMEASURED capacity, never 0 MW")
	cmd.Flags().BoolVar(&strict, "strict", false,
		"Exit non-zero when an assertion fails: crosswalk schema, the as-of cross-check, an observed year's population, or no requested fiscal year observed at all")
	return cmd
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

func fleetValidateSelector(parent, parentKind, plant string, parentKindSet bool) (fleetSelector, error) {
	p := strings.TrimSpace(parent)
	pl := strings.TrimSpace(plant)
	kind := strings.TrimSpace(parentKind)
	if kind == "" {
		kind = fleetParentKindAuto
	}
	if p != "" && pl != "" {
		return fleetSelector{}, fmt.Errorf("--parent and --plant are mutually exclusive; --parent takes an operator token, --plant one published plant name")
	}
	switch kind {
	case fleetParentKindAuto, fleetParentKindPSX:
	default:
		return fleetSelector{}, fmt.Errorf("--parent-kind %q is not valid; allowed values are %q and %q. "+
			"There is deliberately no nepra-acronym value: the acronym index is unexported, so an acronym intent cannot bypass the curated refusal — "+
			"use --plant \"<full published name>\" or the real PSX symbol instead", parentKind, fleetParentKindAuto, fleetParentKindPSX)
	}
	if parentKindSet && p == "" {
		return fleetSelector{}, fmt.Errorf("--parent-kind only applies to --parent, and --parent was not set")
	}
	if p == "" && pl == "" {
		return fleetSelector{}, fmt.Errorf("no selector: pass --parent <SYMBOL> or --plant \"<published name>\"; run `fleet` with no flags for the catalogue")
	}
	if p != "" {
		return fleetSelector{Kind: "parent", Value: p, ParentKind: kind}, nil
	}
	return fleetSelector{Kind: "plant", Value: pl}, nil
}

// fleetFiscalYearLabels turns --fy-range into an inclusive list of labels.
// An empty range defaults to the crosswalk's observed years, so the default
// panel makes no claim about a year nobody indexed.
func fleetFiscalYearLabels(fyRange string) ([]string, error) {
	r := strings.TrimSpace(fyRange)
	if r == "" {
		out := nepraxwalk.ObservedFYs()
		if len(out) == 0 {
			return nil, fmt.Errorf("the embedded crosswalk records no observed fiscal years")
		}
		sort.Strings(out)
		return out, nil
	}
	parts := strings.Split(r, ":")
	if len(parts) != 2 {
		return nil, fmt.Errorf("--fy-range %q must be exactly <FY>:<FY>, e.g. 2017-18:2023-24", fyRange)
	}
	start, err := nepraparse.ParseFiscalYear(parts[0])
	if err != nil {
		return nil, fmt.Errorf("--fy-range start: %w", err)
	}
	end, err := nepraparse.ParseFiscalYear(parts[1])
	if err != nil {
		return nil, fmt.Errorf("--fy-range end: %w", err)
	}
	if start.Start > end.Start {
		return nil, fmt.Errorf("--fy-range %q is inverted: %s starts after %s", fyRange, start.Label(), end.Label())
	}
	if span := end.Start - start.Start + 1; span > fleetMaxFYSpan {
		return nil, fmt.Errorf("--fy-range %q spans %d fiscal years; the maximum is %d", fyRange, span, fleetMaxFYSpan)
	}
	var out []string
	for y := start.Start; y <= end.Start; y++ {
		out = append(out, nepraparse.FiscalYear{Start: y, End: y + 1}.Label())
	}
	return out, nil
}

func fleetClassifyFY(fy string) string {
	for _, o := range nepraxwalk.ObservedFYs() {
		if o == fy {
			return fleetCoverageObserved
		}
	}
	for _, h := range nepraxwalk.HeldOutFYs() {
		if h == fy {
			return fleetCoverageHeldOut
		}
	}
	for _, p := range nepraFleetPublishedFYs {
		if p == fy {
			return fleetCoverageNotIndexd
		}
	}
	return fleetCoverageNotPub
}

func fleetClassNote(fy, class string) string {
	switch class {
	case fleetCoverageHeldOut:
		return nepraFleetHeldOutNote
	case fleetCoverageNotIndexd:
		return fmt.Sprintf("NEPRA publishes an FY%s generation workbook — it is one of the seven reachable years "+
			"%v — but this crosswalk indexed only %v, so nothing here is measured for FY%s. That is an index limit, not a zero.",
			fy, nepraFleetPublishedFYs, nepraxwalk.ObservedFYs(), fy)
	case fleetCoverageNotPub:
		return fmt.Sprintf("FY%s is outside the seven generation workbooks reachable on nepra.org.pk %v "+
			"(FY2016-17, FY2024-25 and FY2025-26 all return 404), so no publication exists to measure.",
			fy, nepraFleetPublishedFYs)
	default:
		return ""
	}
}

func fleetUnobservedReason(fy, class string, plantObserved bool) string {
	if class == fleetCoverageObserved && !plantObserved {
		return fmt.Sprintf("not in the FY%s workbook", fy)
	}
	switch class {
	case fleetCoverageHeldOut:
		return fmt.Sprintf("FY%s was deliberately held out of this crosswalk; no row carries an observation for it", fy)
	case fleetCoverageNotIndexd:
		return fmt.Sprintf("FY%s is published by NEPRA but was never indexed into this crosswalk", fy)
	default:
		return fmt.Sprintf("FY%s is not one of the seven generation workbooks NEPRA publishes", fy)
	}
}

// ---------------------------------------------------------------------------
// Run
// ---------------------------------------------------------------------------

func fleetRun(cmd *cobra.Command, flags *rootFlags, sel fleetSelector, fys []string, strict bool) error {
	res := fleetResults{Selector: sel, DeclaredGaps: fleetDeclaredGaps()}

	var (
		names    []string
		pp       nepraxwalk.ParentPlants
		havePP   bool
		absentAP *nepraxwalk.AbsentParent
	)

	switch {
	case sel.Kind == "parent" && sel.ParentKind == fleetParentKindPSX:
		p, ok := nepraxwalk.PlantsForParent(sel.Value)
		if !ok {
			return fleetUnknownToken(cmd, flags, sel.Value)
		}
		pp, havePP = p, true
		res.Resolution = fleetResolution{
			Token:      strings.ToUpper(strings.TrimSpace(sel.Value)),
			TokenKind:  string(nepraxwalk.TokenPSXTicker),
			ParentName: p.ParentName,
			PSXTicker:  p.PSXTicker,
			Status:     string(p.Status),
			Caveat:     p.Caveat,
		}
		if p.Status == nepraxwalk.StatusAbsentFromDataset {
			res.Resolution.ListedStatus = string(nepraxwalk.StatusListed)
			res.Resolution.AbsenceNote = p.AbsenceNote
			absentAP = fleetAbsentParentBySymbol(p.PSXTicker)
			if absentAP != nil {
				res.Resolution.ProbeTokens = absentAP.ProbeTokens
			}
			return fleetRenderAbsent(cmd, flags, res, p)
		}
		for _, pr := range p.Plants {
			names = append(names, pr.CanonicalName)
			if res.Resolution.ListedStatus == "" {
				if row, ok := nepraxwalk.RowByCanonicalName(pr.CanonicalName); ok {
					res.Resolution.ListedStatus = string(row.ListedStatus)
				}
			}
		}

	case sel.Kind == "parent":
		tr, err := nepraxwalk.ResolveTicker(sel.Value)
		if err != nil {
			var amb *nepraxwalk.AmbiguousTokenError
			if errors.As(err, &amb) {
				return fleetRefuseAmbiguous(cmd, flags, amb)
			}
			if errors.Is(err, nepraxwalk.ErrUnknownToken) {
				return fleetUnknownToken(cmd, flags, sel.Value)
			}
			return err
		}
		res.Resolution = fleetResolution{
			Token:        tr.Token,
			TokenKind:    string(tr.Kind),
			ParentName:   tr.ParentName,
			PSXTicker:    tr.PSXTicker,
			ListedStatus: string(tr.ListedStatus),
			Status:       string(tr.Status),
		}
		if tr.Status == nepraxwalk.StatusAbsentFromDataset {
			// Unreachable today (ticker.go:111 refuses the only absent symbol
			// before ticker.go:133 can return it) but handled rather than
			// assumed away.
			res.Resolution.AbsenceNote = tr.AbsenceNote
			if ap := fleetAbsentParentBySymbol(tr.PSXTicker); ap != nil {
				res.Resolution.ProbeTokens = ap.ProbeTokens
				res.Resolution.Caveat = ap.Caveat
			}
			p, _ := nepraxwalk.PlantsForParent(tr.PSXTicker)
			return fleetRenderAbsent(cmd, flags, res, p)
		}
		names = append(names, tr.Plants...)
		if tr.Kind == nepraxwalk.TokenPSXTicker {
			if p, ok := nepraxwalk.PlantsForParent(tr.PSXTicker); ok {
				pp, havePP = p, true
			}
		} else if tr.PSXTicker != "" {
			// An acronym names the plants it names, never the parent's fleet.
			if p, ok := nepraxwalk.PlantsForParent(tr.PSXTicker); ok && len(p.Plants) > len(names) {
				res.Resolution.ScopeNote = fmt.Sprintf(
					"the NEPRA acronym %q reaches %d plant(s); its parent %s (PSX %s) has %d in this crosswalk. "+
						"Run `nepra-pp-cli fleet --parent %s` for the whole fleet.",
					tr.Token, len(names), tr.ParentName, tr.PSXTicker, len(p.Plants), tr.PSXTicker)
			}
		}

	default: // plant
		m, ok := nepraxwalk.Resolve(sel.Value)
		if !ok {
			return fleetDeclinePlant(cmd, flags, sel.Value)
		}
		names = append(names, m.Row.CanonicalName)
		res.Resolution = fleetResolution{
			ParentName:      m.Row.ParentName,
			PSXTicker:       m.Row.PSXTicker,
			ListedStatus:    string(m.Row.ListedStatus),
			Status:          string(nepraxwalk.StatusHasPlants),
			MatchKind:       string(m.Kind),
			MatchedName:     m.MatchedName,
			NormalizedQuery: m.NormalizedQuery,
		}
		if m.Row.PSXTicker != "" {
			if p, ok := nepraxwalk.PlantsForParent(m.Row.PSXTicker); ok && len(p.Plants) > 1 {
				res.Resolution.ScopeNote = fmt.Sprintf(
					"this is 1 plant; its parent %s (PSX %s) has %d in this crosswalk. Run `nepra-pp-cli fleet --parent %s` for the whole fleet.",
					m.Row.ParentName, m.Row.PSXTicker, len(p.Plants), m.Row.PSXTicker)
			}
		}
	}

	sort.Strings(names)
	res.Resolution.PlantsReached = len(names)

	latest := nepraxwalk.LatestObservedFY()
	res.AsOf = fleetBuildAsOf(names, latest, pp, havePP)
	res.Plants = fleetBuildPlants(names, fys)
	res.FiscalYears = fleetBuildFiscalYears(names, fys)
	res.Assertions = fleetAssertions(names, fys, latest, pp, havePP, res)

	return fleetRender(cmd, flags, res, strict)
}

func fleetAbsentParentBySymbol(symbol string) *nepraxwalk.AbsentParent {
	want := strings.ToUpper(strings.TrimSpace(symbol))
	for _, ap := range nepraxwalk.AbsentParents() {
		if strings.ToUpper(strings.TrimSpace(ap.PSXTicker)) == want {
			out := ap
			return &out
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Build: as-of block
// ---------------------------------------------------------------------------

// fleetCountPartition counts the fleet's plants by operating status in fy,
// straight off the rows. It counts PLANTS, never megawatts: every MW figure in
// this command is an nepraxwalk.MWSum handed back by Coverage or
// PlantsForParent, because MWSum has no exported constructor and a hand-rolled
// float is exactly how a fabricated zero gets shipped.
func fleetCountPartition(names []string, fy string) (operating, nonOperating, statusUnknown, notObserved int) {
	for _, n := range names {
		row, ok := nepraxwalk.RowByCanonicalName(n)
		if !ok {
			notObserved++
			continue
		}
		o, found := row.ObservedIn(fy)
		if !found {
			notObserved++
			continue
		}
		isOp, known := o.Operating()
		switch {
		case !known:
			statusUnknown++
		case isOp:
			operating++
		default:
			nonOperating++
		}
	}
	return operating, nonOperating, statusUnknown, notObserved
}

func fleetBuildAsOf(names []string, latest string, pp nepraxwalk.ParentPlants, havePP bool) fleetAsOf {
	out := fleetAsOf{AsOfFY: latest}
	if havePP {
		out.OperatingPlants = pp.OperatingPlants
		out.NonOperatingPlants = pp.NonOperatingPlants
		out.StatusUnknownPlants = pp.StatusUnknownPlants
		out.NotObservedPlants = pp.NotObservedPlants
		out.CapacityOperating = pp.CapacityOperating
		out.CapacityNonOperating = pp.CapacityNonOperating
		out.Basis = fmt.Sprintf("nepraxwalk.PlantsForParent(%q), as of FY%s", pp.PSXTicker, latest)
	} else {
		op, nonOp, unknown, notObs := fleetCountPartition(names, latest)
		rep := nepraxwalk.Coverage(latest, names)
		out.OperatingPlants, out.NonOperatingPlants = op, nonOp
		out.StatusUnknownPlants, out.NotObservedPlants = unknown, notObs
		out.CapacityOperating = rep.CapacityOperating
		out.CapacityNonOperating = rep.CapacityNonOperating
		out.Basis = fmt.Sprintf("nepraxwalk.Coverage(%q, <%d resolved plant(s)>), as of FY%s", latest, len(names), latest)
	}
	out.DoNotAdd = fleetDoNotAdd(names, latest, out)
	return out
}

// fleetDoNotAdd states, in the payload, why the two sums are separate — and
// names the plants that make the difference, so the warning is specific to
// this fleet rather than boilerplate. It deliberately does NOT print a
// combined total: there is no honest single "fleet capacity" scalar.
func fleetDoNotAdd(names []string, latest string, as fleetAsOf) string {
	base := "capacity_operating and capacity_non_operating are reported separately BECAUSE adding them is the error " +
		"this panel exists to prevent. There is deliberately no single fleet-capacity figure."
	if !as.CapacityNonOperating.Measured() {
		return base + " Non-operating capacity is unmeasured for this selection, which is not a finding of zero."
	}
	mw, _ := as.CapacityNonOperating.Float64()
	if as.CapacityNonOperating.Plants() == 0 {
		return base + fmt.Sprintf(" As of FY%s this selection has no non-operating published capacity — a measured 0.00 MW, not an absence of measurement.", latest)
	}
	var offenders []string
	for _, n := range names {
		row, ok := nepraxwalk.RowByCanonicalName(n)
		if !ok {
			continue
		}
		o, found := row.ObservedIn(latest)
		if !found {
			continue
		}
		if isOp, known := o.Operating(); known && !isOp {
			offenders = append(offenders, fmt.Sprintf("%s (%s)", row.CanonicalName, o.BlockStatusName))
		}
	}
	return base + fmt.Sprintf(" As of FY%s, %.2f MW over %d plant(s) of this selection is published, real and generating nothing: %s.",
		latest, mw, as.CapacityNonOperating.Plants(), strings.Join(offenders, "; "))
}

// ---------------------------------------------------------------------------
// Build: per-plant rows
// ---------------------------------------------------------------------------

func fleetBuildPlants(names, fys []string) []fleetPlant {
	out := make([]fleetPlant, 0, len(names))
	latest := nepraxwalk.LatestObservedFY()
	for _, n := range names {
		row, ok := nepraxwalk.RowByCanonicalName(n)
		if !ok {
			// A membership name that no longer resolves is a real
			// inconsistency; report it rather than dropping the plant.
			out = append(out, fleetPlant{
				CanonicalName:           n,
				Evidence:                "NO CROSSWALK ROW: this canonical name was reached through a parent index but RowByCanonicalName declined it. Reported rather than dropped.",
				PublicationIntervalNote: nepraFleetPublicationIntervalNote,
			})
			continue
		}
		p := fleetPlant{
			CanonicalName:           row.CanonicalName,
			Aliases:                 row.Aliases,
			ParentName:              row.ParentName,
			PSXTicker:               row.PSXTicker,
			ListedStatus:            string(row.ListedStatus),
			Confidence:              string(row.Confidence),
			ValidFromFY:             row.ValidFromFY,
			ValidToFY:               row.ValidToFY,
			PublicationIntervalNote: nepraFleetPublicationIntervalNote,
			Evidence:                row.Evidence,
		}
		if row.ValidFromFY == latest && latest != "" {
			p.ValidFromNote = nepraFleetValidFromNote
		}
		for _, fy := range fys {
			class := fleetClassifyFY(fy)
			o, found := row.ObservedIn(fy)
			if !found {
				p.ByFY = append(p.ByFY, fleetPlantFY{
					FY:       fy,
					Observed: false,
					Reason:   fleetUnobservedReason(fy, class, false),
				})
				continue
			}
			st, _ := o.BlockStatus()
			isOp, known := o.Operating()
			sno, capacity := o.SNo, o.Capacity()
			opCopy, knownCopy := isOp, known
			p.ByFY = append(p.ByFY, fleetPlantFY{
				FY:                fy,
				Observed:          true,
				SNo:               &sno,
				PublishedName:     o.PublishedName,
				Technology:        o.Technology,
				Fuel:              o.Fuel,
				Status:            st.String(),
				Operating:         &opCopy,
				StatusKnown:       &knownCopy,
				InstalledCapacity: &capacity,
			})
		}
		out = append(out, p)
	}
	return out
}

// ---------------------------------------------------------------------------
// Build: fiscal-year columns
// ---------------------------------------------------------------------------

func fleetBuildFiscalYears(names, fys []string) []fleetFiscalYear {
	out := make([]fleetFiscalYear, 0, len(fys))
	for _, fy := range fys {
		class := fleetClassifyFY(fy)
		rep := nepraxwalk.Coverage(fy, names)
		col := fleetFiscalYear{
			FY:                    fy,
			FYObserved:            rep.FYObserved,
			CoverageClass:         class,
			PlantsInFleet:         rep.DistinctPlants,
			ObservedInFY:          rep.ObservedInFY,
			NoObservationInFY:     rep.NoObservationInFY,
			CapacityReported:      rep.CapacityReported,
			CapacityOperating:     rep.CapacityOperating,
			CapacityNonOperating:  rep.CapacityNonOperating,
			CapacityStatusUnknown: rep.CapacityStatusUnknown,
			CapacityNotReported:   rep.CapacityNotReported,
			CapacityStatusCell:    rep.CapacityStatusCell,
			CapacityUnknownText:   rep.CapacityUnknownText,
			CapacityBasis:         rep.CapacityBasis,
			ClassNote:             fleetClassNote(fy, class),
		}
		col.OverstatementPct = fleetOverstatement(rep.CapacityReported, rep.CapacityOperating)
		out = append(out, col)
	}
	return out
}

// fleetOverstatement is (reported/operating - 1) * 100, present only when both
// sums are real measurements and operating capacity is positive. Nil otherwise:
// a ratio against an unmeasured or zero denominator is not a number.
func fleetOverstatement(reported, operating nepraxwalk.MWSum) *float64 {
	rep, okR := reported.Float64()
	op, okO := operating.Float64()
	if !okR || !okO || op <= 0 {
		return nil
	}
	pct := (rep/op - 1) * 100
	return &pct
}

// ---------------------------------------------------------------------------
// Assertions
// ---------------------------------------------------------------------------

func fleetAssertions(names, fys []string, latest string, pp nepraxwalk.ParentPlants, havePP bool, res fleetResults) []fleetAssertion {
	var out []fleetAssertion

	// 1. Schema.
	rows := nepraxwalk.Rows()
	obs := 0
	for _, r := range rows {
		obs += len(r.Observed)
	}
	if err := nepraxwalk.Validate(); err != nil {
		out = append(out, fleetAssertion{Name: "crosswalk_schema", OK: false,
			Detail: fmt.Sprintf("nepraxwalk.Validate() failed over %d rows / %d observations: %v", len(rows), obs, err)})
	} else {
		out = append(out, fleetAssertion{Name: "crosswalk_schema", OK: true,
			Detail: fmt.Sprintf("nepraxwalk.Validate() clean over %d rows / %d observations", len(rows), obs)})
	}

	// 2. As-of cross-check. Two independent computations of the same fleet.
	op, nonOp, unknown, notObs := fleetCountPartition(names, latest)
	rep := nepraxwalk.Coverage(latest, names)
	if havePP {
		okCounts := op == pp.OperatingPlants && nonOp == pp.NonOperatingPlants &&
			unknown == pp.StatusUnknownPlants && notObs == pp.NotObservedPlants
		okSums := fleetSameSum(pp.CapacityOperating, rep.CapacityOperating) &&
			fleetSameSum(pp.CapacityNonOperating, rep.CapacityNonOperating)
		out = append(out, fleetAssertion{
			Name: "as_of_cross_check", OK: okCounts && okSums,
			Detail: fmt.Sprintf("PlantsForParent(%q) operating %s / non-operating %s and partition %d/%d/%d/%d vs "+
				"Coverage(%q, fleet) operating %s / non-operating %s and recount %d/%d/%d/%d",
				pp.PSXTicker, pp.CapacityOperating, pp.CapacityNonOperating,
				pp.OperatingPlants, pp.NonOperatingPlants, pp.StatusUnknownPlants, pp.NotObservedPlants,
				latest, rep.CapacityOperating, rep.CapacityNonOperating, op, nonOp, unknown, notObs),
		})
	} else {
		okCounts := op+nonOp+unknown == rep.ObservedInFY && notObs == rep.NoObservationInFY
		out = append(out, fleetAssertion{
			Name: "as_of_cross_check", OK: okCounts,
			Detail: fmt.Sprintf("row recount as of FY%s gives %d observed (%d operating / %d non-operating / %d status-unknown) "+
				"and %d unobserved vs Coverage's %d observed and %d unobserved",
				latest, op+nonOp+unknown, op, nonOp, unknown, notObs, rep.ObservedInFY, rep.NoObservationInFY),
		})
	}

	// 3. Per-observed-FY population, recomputed off the rows against Coverage.
	var mismatches []string
	var checked []string
	for _, col := range res.FiscalYears {
		if col.CoverageClass != fleetCoverageObserved {
			continue
		}
		o, n, u, _ := fleetCountPartition(names, col.FY)
		got := o + n + u
		checked = append(checked, fmt.Sprintf("FY%s %d", col.FY, col.ObservedInFY))
		if got != col.ObservedInFY {
			mismatches = append(mismatches, fmt.Sprintf("FY%s reported %d observed, rows give %d", col.FY, col.ObservedInFY, got))
		}
	}
	switch {
	case len(checked) == 0:
		out = append(out, fleetAssertion{Name: "fy_population_floor", OK: true,
			Detail: "no requested fiscal year is one the crosswalk observed, so there is no population to check"})
	case len(mismatches) > 0:
		out = append(out, fleetAssertion{Name: "fy_population_floor", OK: false, Detail: strings.Join(mismatches, "; ")})
	default:
		out = append(out, fleetAssertion{Name: "fy_population_floor", OK: true,
			Detail: "observation counts recomputed off the rows agree with Coverage for " + strings.Join(checked, ", ")})
	}

	// 4. Was anything observable at all?
	var observedFYs, unobserved []string
	for _, col := range res.FiscalYears {
		if col.CoverageClass == fleetCoverageObserved {
			observedFYs = append(observedFYs, col.FY)
		} else {
			unobserved = append(unobserved, fmt.Sprintf("FY%s (%s)", col.FY, col.CoverageClass))
		}
	}
	if len(observedFYs) == 0 {
		out = append(out, fleetAssertion{Name: "some_fy_observed", OK: false,
			Detail: fmt.Sprintf("NONE of the %d requested fiscal years was observed by this crosswalk: %s. "+
				"Every capacity figure in this panel is unmeasured — that is honest, but it is not an answer.",
				len(res.FiscalYears), strings.Join(unobserved, ", "))})
	} else {
		out = append(out, fleetAssertion{Name: "some_fy_observed", OK: true,
			Detail: fmt.Sprintf("%d of %d requested fiscal years are observed (%s)",
				len(observedFYs), len(res.FiscalYears), strings.Join(observedFYs, ", "))})
	}

	// 5. S.No stability, recomputed at runtime over the rows this panel used.
	out = append(out, fleetSNoAssertion(names))
	return out
}

func fleetSameSum(a, b nepraxwalk.MWSum) bool {
	av, aok := a.Float64()
	bv, bok := b.Float64()
	return aok == bok && av == bv && a.Plants() == b.Plants()
}

// fleetSNoAssertion recomputes the S.No claim from the embedded rows instead of
// only quoting the curated note: for every plant in this selection observed in
// both of the crosswalk's observed years, did its S.No move?
func fleetSNoAssertion(names []string) fleetAssertion {
	fys := nepraxwalk.ObservedFYs()
	if len(fys) < 2 {
		return fleetAssertion{Name: "sno_is_not_a_key", OK: true,
			Detail: "fewer than two observed fiscal years, so no cross-year S.No comparison is possible"}
	}
	sort.Strings(fys)
	first, last := fys[0], fys[len(fys)-1]
	var moves []string
	var both, stable int
	for _, n := range names {
		row, ok := nepraxwalk.RowByCanonicalName(n)
		if !ok {
			continue
		}
		a, okA := row.ObservedIn(first)
		b, okB := row.ObservedIn(last)
		if !okA || !okB {
			continue
		}
		both++
		if a.SNo == b.SNo {
			stable++
			continue
		}
		moves = append(moves, fmt.Sprintf("%s %d->%d", row.CanonicalName, a.SNo, b.SNo))
	}
	if both == 0 {
		return fleetAssertion{Name: "sno_is_not_a_key", OK: true,
			Detail: fmt.Sprintf("no plant in this selection is observed in both FY%s and FY%s, so S.No could not be compared", first, last)}
	}
	return fleetAssertion{
		Name: "sno_is_not_a_key", OK: stable == 0,
		Detail: fmt.Sprintf("of %d plant(s) in this selection observed in both FY%s and FY%s, %d keep their S.No; measured moves: %s",
			both, first, last, stable, strings.Join(moves, ", ")),
	}
}

// ---------------------------------------------------------------------------
// Declared gaps
// ---------------------------------------------------------------------------

// fleetDeclaredGaps are the questions this command is asked and cannot answer
// from what ships today. They are in the payload, not just the docs, because an
// agent that reads only JSON must still learn the limit.
func fleetDeclaredGaps() []fleetGap {
	return []fleetGap{
		{
			Question: "capacity for all seven published fiscal years",
			Verdict:  "partially_unavailable",
			Reason: fmt.Sprintf("NEPRA publishes seven generation workbooks %v, but this crosswalk indexed only %v "+
				"(FY2020-21 is held out on purpose as an out-of-sample test; FY2018-19, FY2019-20, FY2021-22 and FY2022-23 were never fetched). "+
				"The other five years return unmeasured columns rather than invented numbers.",
				nepraFleetPublishedFYs, nepraxwalk.ObservedFYs()),
			Instead: "growing them needs a live leg (fetch + nepraparse.ParseWorkbook + Resolve per year), which nothing in this CLI calls today",
		},
		{
			Question: "load factor, GWh generated, utilisation",
			Verdict:  "out_of_scope",
			Reason:   "the crosswalk's Observation carries only fy, sno, published_name, technology, fuel, installed capacity and its state, and block_status. No monthly generation is indexed, so no load factor is computable.",
			Instead:  "the monthly generation panel needs nepraparse.ParseWorkbook over the fetched workbook",
		},
		{
			Question: "dependable capacity",
			Verdict:  "unavailable",
			Reason:   "NEPRA publishes a Dependable Capacity (MW) column and nepraparse models it, but it was not curated into the crosswalk's 241 observations, so this command has no value to report — as opposed to a value of zero.",
		},
		{
			Question: "equity share of a fleet's capacity",
			Verdict:  "unverifiable",
			Reason:   "ownership percentages exist only as prose inside each row's `evidence`. A capacity sum here treats a joint venture as wholly owned; read `confidence` and `evidence` before attributing megawatts to a listed company.",
		},
	}
}

// ---------------------------------------------------------------------------
// Refusals
// ---------------------------------------------------------------------------

// fleetRefuseAmbiguous surfaces BOTH readings of a colliding token and picks
// neither. Exit 2.
func fleetRefuseAmbiguous(cmd *cobra.Command, flags *rootFlags, amb *nepraxwalk.AmbiguousTokenError) error {
	body := fleetRefusalBody{
		Kind:       "ambiguous_token",
		Token:      amb.Token,
		Reason:     amb.Reason,
		Hazard:     amb.Hazard,
		Candidates: amb.Candidates,
		Recovery:   fleetRecoveryFor(amb),
	}
	if err := fleetWriteRefusal(cmd, flags, fleetRefusal{Error: body}); err != nil {
		return err
	}
	return usageErr(fmt.Errorf("token %q is ambiguous and will not be guessed (%d candidates); see the refusal envelope for both readings and the recovery commands",
		amb.Token, len(amb.Candidates)))
}

func fleetRecoveryFor(amb *nepraxwalk.AmbiguousTokenError) []string {
	var out []string
	seen := map[string]bool{}
	add := func(s string) {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, c := range amb.Candidates {
		if c.Kind == nepraxwalk.TokenNEPRAAcronym {
			if c.PSXTicker != "" && !strings.EqualFold(c.PSXTicker, amb.Token) {
				add("nepra-pp-cli fleet --parent " + c.PSXTicker)
			}
			if c.CanonicalName != "" {
				add(fmt.Sprintf("nepra-pp-cli fleet --plant %q", c.CanonicalName))
			}
		}
	}
	for _, c := range amb.Candidates {
		if c.Kind != nepraxwalk.TokenPSXTicker || c.PSXTicker == "" {
			continue
		}
		if fleetAbsentParentBySymbol(c.PSXTicker) != nil {
			add(fmt.Sprintf("nepra-pp-cli fleet --parent %s --parent-kind %s", c.PSXTicker, fleetParentKindPSX))
			continue
		}
		if _, ok := nepraxwalk.PlantsForParent(c.PSXTicker); ok {
			add(fmt.Sprintf("nepra-pp-cli fleet --parent %s --parent-kind %s", c.PSXTicker, fleetParentKindPSX))
		}
	}
	add("nepra-pp-cli fleet   # the catalogue, including all refused tokens")
	return out
}

// fleetUnknownToken declines a token in neither namespace. Exit 3.
func fleetUnknownToken(cmd *cobra.Command, flags *rootFlags, token string) error {
	tickers := nepraxwalk.Tickers()
	body := fleetRefusalBody{
		Kind:  "unknown_token",
		Token: strings.ToUpper(strings.TrimSpace(token)),
		Reason: fmt.Sprintf("%q is neither a PSX symbol nor an unambiguous NEPRA plant acronym in this crosswalk. "+
			"That is not a claim the token is invalid on the PSX — only that no plant here is attributed to it.",
			strings.TrimSpace(token)),
		FuzzyMatching: "disabled",
		KnownSymbols:  tickers,
	}
	if err := fleetWriteRefusal(cmd, flags, fleetRefusal{Error: body}); err != nil {
		return err
	}
	return notFoundErr(fmt.Errorf("no plants attributed to %q; the %d PSX symbols in this crosswalk are %s",
		strings.TrimSpace(token), len(tickers), strings.Join(tickers, ", ")))
}

// fleetDeclinePlant declines a published name. There is NO fuzzy suggestion:
// "(NPPCL) - Balloki" shares the exact prefix "(NPPCL) - " with a different
// 1,230 MW plant, so any prefix rule hands one plant's history to another.
func fleetDeclinePlant(cmd *cobra.Command, flags *rootFlags, query string) error {
	norm := nepraxwalk.Normalize(query)
	body := fleetRefusalBody{
		Kind:         "unknown_plant_name",
		Query:        query,
		DeclinedName: norm,
		Reason: "no canonical name or recorded alias matches this string, and the one-trailing-parenthetical fallback " +
			"did not leave exactly one plant. Declining is the answer: there is no fuzzy fallback of any kind.",
		FuzzyMatching: "disabled",
	}
	if ap, hit := fleetAbsentParentHint(norm); hit {
		body.AbsentParentHint = fmt.Sprintf("this query names %s, whose OWN generation fleet is absent from these workbooks. %s "+
			"Reach the absence record with `nepra-pp-cli fleet --parent %s --parent-kind %s`.",
			ap.ParentName, ap.AbsenceNote, ap.PSXTicker, fleetParentKindPSX)
		body.CounterFact = "Do NOT read that as \"nothing at Bin Qasim\": Lucky Electric Power Company Limited (LEPCL), " +
			"the 660 MW supercritical plant at Bin Qasim, IS in this dataset (FY2023-24 S.No 70, PSX LUCK)."
	}
	if grp, hit := fleetNearDuplicateGroup(norm); hit {
		body.NearDuplicateGroup = grp
	}
	if err := fleetWriteRefusal(cmd, flags, fleetRefusal{Error: body}); err != nil {
		return err
	}
	return notFoundErr(fmt.Errorf("no plant matches %q (normalised %q); fuzzy matching is disabled by design", query, norm))
}

// fleetAbsentParentHint fires when the query contains one of an absent
// parent's probe tokens on word boundaries.
func fleetAbsentParentHint(norm string) (nepraxwalk.AbsentParent, bool) {
	up := strings.ToUpper(norm)
	for _, ap := range nepraxwalk.AbsentParents() {
		for _, tok := range ap.ProbeTokens {
			if fleetContainsWord(up, strings.ToUpper(tok)) {
				return ap, true
			}
		}
	}
	return nepraxwalk.AbsentParent{}, false
}

// fleetContainsWord reports whether needle appears in haystack delimited by
// non-alphanumeric characters, so "SITE" does not fire inside "SITES".
func fleetContainsWord(haystack, needle string) bool {
	if needle == "" {
		return false
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] != needle {
			continue
		}
		if i > 0 && fleetIsWordByte(haystack[i-1]) {
			continue
		}
		if j := i + len(needle); j < len(haystack) && fleetIsWordByte(haystack[j]) {
			continue
		}
		return true
	}
	return false
}

func fleetIsWordByte(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z'
}

// fleetNearDuplicateGroup names a reviewed near-duplicate group when the
// declined query is a prefix of two or more of its names.
//
// This APPROXIMATES the refusal reason rather than reporting it: nepraxwalk's
// Resolve returns a bare bool with no reason code, and foldKey and
// stripTrailingParen are unexported, so the CLI cannot know whether a decline
// was "unknown name" or "two plants share the stripped base". The group is
// labelled as a reviewed near-duplicate group, never as the cause.
func fleetNearDuplicateGroup(norm string) ([]string, bool) {
	key := strings.ToLower(strings.TrimSpace(norm))
	if len(key) < 4 {
		return nil, false
	}
	for _, pd := range nepraxwalk.PossibleDuplicates() {
		hits := 0
		for _, n := range pd.Names {
			if strings.HasPrefix(strings.ToLower(nepraxwalk.Normalize(n)), key) {
				hits++
			}
		}
		if hits >= 2 {
			return append([]string(nil), pd.Names...), true
		}
	}
	return nil, false
}

// fleetWriteRefusal puts the envelope on stdout for a machine caller and on
// stderr for a human, so a --json consumer always has a parseable body even on
// a non-zero exit (the `which` precedent, which_test.go:211).
func fleetWriteRefusal(cmd *cobra.Command, flags *rootFlags, r fleetRefusal) error {
	payload, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		fleetWriteRefusalHuman(cmd.ErrOrStderr(), r)
		return nil
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(payload))
	return err
}
