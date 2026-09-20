// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package nepraxwalk

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraparse"
)

// ErrAmbiguousToken is what a refused token matches with errors.Is. Use
// errors.As with *AmbiguousTokenError to read the competing candidates.
var ErrAmbiguousToken = errors.New("nepraxwalk: token is ambiguous")

// ErrUnknownToken means the token is in neither namespace this crosswalk
// knows. It is not a claim that the token is invalid on the PSX.
var ErrUnknownToken = errors.New("nepraxwalk: token not in crosswalk")

// AmbiguousTokenError refuses a token that has more than one defensible
// reading, and carries every reading so a caller can disambiguate.
//
// The canonical instance is "KEL": NEPRA writes "Kohinoor Energy Limited.
// (KEL)" for a 131 MW RFO plant, while KEL is K-Electric Limited's PSX symbol.
// Resolving it either way silently misattributes generation.
type AmbiguousTokenError struct {
	Token      string
	Reason     string
	Hazard     string
	Candidates []TokenCandidate
}

func (e *AmbiguousTokenError) Error() string {
	parts := make([]string, 0, len(e.Candidates))
	for _, c := range e.Candidates {
		switch c.Kind {
		case TokenPSXTicker:
			parts = append(parts, fmt.Sprintf("PSX symbol %s = %s", c.PSXTicker, c.ParentName))
		default:
			label := c.CanonicalName
			if c.ParentName != "" {
				label = fmt.Sprintf("%s (parent %s", c.CanonicalName, c.ParentName)
				if c.PSXTicker != "" {
					label += ", PSX " + c.PSXTicker
				}
				label += ")"
			}
			parts = append(parts, "NEPRA acronym for "+label)
		}
	}
	return fmt.Sprintf("nepraxwalk: token %q is ambiguous (%s): %s; disambiguate with a full plant name or an explicit PSX symbol",
		e.Token, e.Reason, strings.Join(parts, " vs "))
}

// Is makes errors.Is(err, ErrAmbiguousToken) true.
func (e *AmbiguousTokenError) Is(target error) bool { return target == ErrAmbiguousToken }

// TickerResolution is a token accepted in exactly one namespace.
type TickerResolution struct {
	// Token is the caller's token, uppercased.
	Token string
	// Kind says which namespace accepted it.
	Kind         TokenKind
	ParentName   string
	PSXTicker    string
	ListedStatus ListedStatus
	// Plants are the canonical plant names this token reaches, sorted. They
	// carry NO operating status: this type answers "which namespace is this
	// token in", not "what is running". Call [PlantsForParent] for capacity.
	Plants []string
	// NonOperatingPlants is the subset of Plants whose generation block was
	// DELICENSED or DECOMMISSIONED in the crosswalk's latest observed fiscal
	// year, named here so a caller cannot read Plants as a live fleet.
	NonOperatingPlants []string
	// Status is StatusAbsentFromDataset when the parent's fleet is known to be
	// outside these workbooks, in which case Plants is empty by design.
	Status ParentStatus
	// AbsenceNote is non-empty only for StatusAbsentFromDataset.
	AbsenceNote string
}

// ResolveTicker resolves a bare token that could be a PSX symbol or one of
// NEPRA's parenthetical plant acronyms.
//
// The two namespaces overlap, and every verified overlap is refused with an
// *AmbiguousTokenError (matching ErrAmbiguousToken):
//
//	KEL  - Kohinoor Energy Limited (PSX KOHE) vs PSX KEL = K-Electric Limited
//	AGL  - Attock Gen Limited (unlisted)      vs PSX AGL = Agritech Limited
//	APL  - Atlas Power Limited                vs PSX APL = Attock Petroleum
//	SPL  - Saif Power Limited (PSX SPWL)      vs PSX SPL = Sitara Peroxide
//	AEL  - Altern Energy Limited (PSX ALTN)   vs PSX AEL = AEL Textile
//	HEPL - Hawa Energy (wind) vs HNDS Energy (solar), both in FY2023-24
//
// Beyond that curated list, a token is refused whenever it reaches two or more
// plants that do not share one attributed parent, so a future data edit cannot
// quietly introduce a collision.
//
// An unknown token returns ErrUnknownToken. Nothing is guessed.
func ResolveTicker(token string) (TickerResolution, error) {
	ix := load()
	if ix.loadErr != nil {
		return TickerResolution{}, ix.loadErr
	}
	t := strings.ToUpper(strings.TrimSpace(token))
	if t == "" {
		return TickerResolution{}, fmt.Errorf("%w: empty token", ErrUnknownToken)
	}
	if a, ok := ix.ambiguous[t]; ok {
		return TickerResolution{}, &AmbiguousTokenError{
			Token:      t,
			Reason:     a.Reason,
			Hazard:     a.Hazard,
			Candidates: append([]TokenCandidate(nil), a.Candidates...),
		}
	}
	if cands, ok := derivedCollision(ix, t); ok {
		return TickerResolution{}, &AmbiguousTokenError{
			Token:      t,
			Reason:     "token reaches more than one plant without a single attributed parent (derived from crosswalk rows)",
			Hazard:     "attributing one plant's generation to another",
			Candidates: cands,
		}
	}

	// PSX symbol namespace first: it is the one callers mean when they pass a
	// ticker, and absent-parent records live there.
	if rows, ok := ix.byTicker[t]; ok {
		return resolutionFromRows(ix, t, TokenPSXTicker, rows), nil
	}
	if ab, ok := ix.absent[t]; ok {
		return TickerResolution{
			Token:        t,
			Kind:         TokenPSXTicker,
			ParentName:   ab.ParentName,
			PSXTicker:    ab.PSXTicker,
			ListedStatus: StatusListed,
			Status:       ab.Status,
			AbsenceNote:  ab.AbsenceNote,
		}, nil
	}
	if rows, ok := ix.acronyms[t]; ok {
		return resolutionFromRows(ix, t, TokenNEPRAAcronym, rows), nil
	}
	return TickerResolution{}, fmt.Errorf("%w: %q is neither a PSX symbol nor a NEPRA plant acronym in this crosswalk", ErrUnknownToken, t)
}

func resolutionFromRows(ix *index, token string, kind TokenKind, rows []int) TickerResolution {
	res := TickerResolution{Token: token, Kind: kind, Status: StatusHasPlants, ListedStatus: StatusUnknown}
	asOf := ix.tbl.LatestObservedFY
	for n, i := range rows {
		r := ix.tbl.Rows[i]
		res.Plants = append(res.Plants, r.CanonicalName)
		if op, observed := r.OperatingInFY(asOf); observed && !op {
			res.NonOperatingPlants = append(res.NonOperatingPlants, r.CanonicalName)
		}
		if n == 0 {
			res.ParentName, res.PSXTicker, res.ListedStatus = r.ParentName, r.PSXTicker, r.ListedStatus
		}
	}
	sort.Strings(res.Plants)
	sort.Strings(res.NonOperatingPlants)
	return res
}

// derivedCollision recomputes ambiguity from the rows themselves, independent
// of the curated ambiguous_tokens list, so a data edit that creates a new
// collision is caught rather than silently resolved. A token reaching several
// plants is fine only when every one of them is attributed to the same parent.
func derivedCollision(ix *index, token string) ([]TokenCandidate, bool) {
	rows := ix.acronyms[token]
	if len(rows) < 2 {
		// A single acronym row may still clash with a ticker on another parent.
		if len(rows) == 1 {
			if tr, ok := ix.byTicker[token]; ok && !sameParent(ix, append(append([]int(nil), rows...), tr...)) {
				return candidates(ix, rows, tr), true
			}
		}
		return nil, false
	}
	if sameParent(ix, rows) {
		return nil, false
	}
	return candidates(ix, rows, nil), true
}

func sameParent(ix *index, rows []int) bool {
	var parent string
	for n, i := range rows {
		p := ix.tbl.Rows[i].ParentName
		if p == "" {
			// Unattributed plants can never be grouped: "unknown" is not a
			// shared identity.
			return false
		}
		if n == 0 {
			parent = p
			continue
		}
		if p != parent {
			return false
		}
	}
	return true
}

func candidates(ix *index, acronymRows, tickerRows []int) []TokenCandidate {
	var out []TokenCandidate
	for _, i := range acronymRows {
		r := ix.tbl.Rows[i]
		out = append(out, TokenCandidate{
			Kind: TokenNEPRAAcronym, CanonicalName: r.CanonicalName,
			ParentName: r.ParentName, PSXTicker: r.PSXTicker,
			Note: "derived from crosswalk row " + r.CanonicalName,
		})
	}
	for _, i := range tickerRows {
		r := ix.tbl.Rows[i]
		out = append(out, TokenCandidate{
			Kind: TokenPSXTicker, CanonicalName: r.CanonicalName,
			ParentName: r.ParentName, PSXTicker: r.PSXTicker,
			Note: "derived from crosswalk row " + r.CanonicalName,
		})
	}
	return out
}

// PlantRef is a plant seen from its parent's side, as of one fiscal year.
//
// The operating fields are not decoration. Without them this struct answered
// "which plants does HUBC own?" with six entries that looked alike, one of
// which — the Hub Power Company's own 1,292 MW base plant — had been
// DELICENSED and generated nothing in the year being reported. An analyst
// summing the list got 15.4% more live capacity than exists.
type PlantRef struct {
	CanonicalName string     `json:"canonical_name"`
	ParentName    string     `json:"parent_name,omitempty"`
	PSXTicker     string     `json:"psx_ticker,omitempty"`
	Confidence    Confidence `json:"confidence"`
	ValidFromFY   string     `json:"valid_from_fy"`
	// ValidToFY is the PUBLICATION interval's end, not a shutdown date. See
	// [Row.IsOpenEnded]: every row in this crosswalk has an open interval,
	// delicensed plants included.
	ValidToFY string `json:"valid_to_fy,omitempty"`

	// ObservedInAsOfFY reports whether this plant appears in the workbook for
	// the enclosing [ParentPlants.AsOfFY]. When false, every field below is
	// unset and says nothing about the plant.
	ObservedInAsOfFY bool `json:"observed_in_as_of_fy"`
	// StatusName is the plant's generation-block status in AsOfFY:
	// "numeric" (generating), "delicensed" or "decommissioned".
	StatusName string `json:"status,omitempty"`
	// Operating is true only for a plant whose status is recorded AND
	// ordinary. StatusKnown distinguishes "not generating" from "we do not
	// know", so a false Operating is never silently a claim.
	Operating   bool `json:"operating"`
	StatusKnown bool `json:"status_known"`
	// InstalledCapacity is the AsOfFY published capacity, typed so that a
	// blank cell and an "Export to K.Electric" cell cannot be read as zero.
	// A non-operating plant KEEPS its capacity here — that is correct, and it
	// is why Operating must be read alongside it.
	InstalledCapacity nepraparse.Value `json:"installed_capacity"`
}

// ParentPlants is everything the crosswalk knows about one listed operator's
// presence in the generation workbooks, as of a stated fiscal year.
type ParentPlants struct {
	PSXTicker  string       `json:"psx_ticker"`
	ParentName string       `json:"parent_name,omitempty"`
	Status     ParentStatus `json:"status"`
	// AsOfFY is the fiscal year every operating status and capacity below is
	// read from: the crosswalk's latest observed year. It is stated because a
	// fleet summary with no as-of date is not interpretable.
	AsOfFY string `json:"as_of_fy,omitempty"`
	// Plants is empty when Status is StatusAbsentFromDataset. Read Status
	// before reading len(Plants).
	Plants []PlantRef `json:"plants,omitempty"`

	// The four counts partition len(Plants) by operating status in AsOfFY.
	OperatingPlants     int `json:"operating_plants"`
	NonOperatingPlants  int `json:"non_operating_plants"`
	StatusUnknownPlants int `json:"status_unknown_plants"`
	NotObservedPlants   int `json:"not_observed_plants"`

	// CapacityOperating is the fleet's generating capacity in AsOfFY.
	// CapacityNonOperating is published capacity that is NOT generating —
	// delicensed or decommissioned. They are reported separately because
	// adding them is the error this type exists to prevent, and each is an
	// [MWSum] so a parent with nothing observed cannot return a zero.
	CapacityOperating    MWSum `json:"capacity_operating"`
	CapacityNonOperating MWSum `json:"capacity_non_operating"`

	// AbsenceNote explains a StatusAbsentFromDataset result: what was searched
	// and why the absence is a scope limit rather than a measurement.
	AbsenceNote string `json:"absence_note,omitempty"`
	// Caveat carries any token hazard attached to this ticker.
	Caveat string `json:"caveat,omitempty"`
}

// PlantsForParent lists the plants attributed to one PSX symbol.
//
// The argument is interpreted as a PSX symbol only, never as a NEPRA acronym;
// pass a bare token to ResolveTicker instead, which will refuse it if it is
// ambiguous.
//
// The second return is false only when the symbol is unknown to the crosswalk.
// It is true, with Status == StatusAbsentFromDataset and no plants, for an
// operator that generates but whose fleet these workbooks do not cover — which
// is exactly K-Electric's case:
//
//	pp, ok := PlantsForParent("KEL")   // ok == true
//	pp.Status                          // StatusAbsentFromDataset
//	pp.Plants                          // empty, and AbsenceNote says why
//
// A caller must branch on Status. An empty Plants slice alone must never be
// rendered as "generated nothing".
//
// EVERY plant comes back with its operating status as of AsOfFY, and a caller
// summing capacity MUST use CapacityOperating rather than adding the plants up
// itself. HUBC is the worked example: six attributed plants, of which the Hub
// Power Company's own 1,292 MW base plant was DELICENSED in FY2023-24 and
// generated nothing. It is still correctly listed, with a correct 1,292 MW
// published capacity, and it belongs in CapacityNonOperating — never in a
// live-capacity total.
func PlantsForParent(ticker string) (ParentPlants, bool) {
	ix := load()
	if ix.loadErr != nil || ix.tbl == nil {
		return ParentPlants{}, false
	}
	t := strings.ToUpper(strings.TrimSpace(ticker))
	if t == "" {
		return ParentPlants{}, false
	}
	if ab, ok := ix.absent[t]; ok {
		return ParentPlants{
			PSXTicker:   ab.PSXTicker,
			ParentName:  ab.ParentName,
			Status:      ab.Status,
			AbsenceNote: ab.AbsenceNote,
			Caveat:      ab.Caveat,
		}, true
	}
	rows, ok := ix.byTicker[t]
	if !ok {
		return ParentPlants{}, false
	}
	asOf := ix.tbl.LatestObservedFY
	pp := ParentPlants{PSXTicker: t, Status: StatusHasPlants, AsOfFY: asOf}
	var opMW, nonOpMW float64
	var opN, nonOpN int
	for _, i := range rows {
		r := ix.tbl.Rows[i]
		if pp.ParentName == "" {
			pp.ParentName = r.ParentName
		}
		ref := PlantRef{
			CanonicalName: r.CanonicalName,
			ParentName:    r.ParentName,
			PSXTicker:     r.PSXTicker,
			Confidence:    r.Confidence,
			ValidFromFY:   r.ValidFromFY,
			ValidToFY:     r.ValidToFY,
		}
		if o, found := r.ObservedIn(asOf); found {
			ref.ObservedInAsOfFY = true
			ref.InstalledCapacity = o.Capacity()
			st, _ := o.BlockStatus()
			ref.StatusName = st.String()
			ref.Operating, ref.StatusKnown = o.Operating()
			mw, isNumber := ref.InstalledCapacity.Float64()
			switch {
			case !ref.StatusKnown:
				pp.StatusUnknownPlants++
			case ref.Operating:
				pp.OperatingPlants++
				if isNumber {
					opMW += mw
					opN++
				}
			default:
				pp.NonOperatingPlants++
				if isNumber {
					nonOpMW += mw
					nonOpN++
				}
			}
		} else {
			pp.NotObservedPlants++
		}
		pp.Plants = append(pp.Plants, ref)
	}
	// Unmeasured unless at least one plant was actually observed in AsOfFY.
	measurable := pp.OperatingPlants+pp.NonOperatingPlants+pp.StatusUnknownPlants > 0
	pp.CapacityOperating = MWSum{mw: opMW, measured: measurable, plants: opN}
	pp.CapacityNonOperating = MWSum{mw: nonOpMW, measured: measurable, plants: nonOpN}
	sort.Slice(pp.Plants, func(i, j int) bool { return pp.Plants[i].CanonicalName < pp.Plants[j].CanonicalName })
	if a, ok := ix.ambiguous[t]; ok {
		pp.Caveat = "token hazard: " + a.Hazard
	}
	return pp, true
}

// Tickers lists every PSX symbol the crosswalk attributes at least one plant
// to, sorted. Symbols recorded only as absent parents are excluded.
func Tickers() []string {
	ix := load()
	if ix.tbl == nil {
		return nil
	}
	out := make([]string, 0, len(ix.byTicker))
	for t := range ix.byTicker {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}
