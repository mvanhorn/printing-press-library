package nepraper

import "strings"

// Entity is a distribution licensee as NEPRA names it in a PER.
type Entity string

// The reporting roster. Note that this is TEN entities, not eleven: TESCO is
// excluded from the PER on the record (see TESCOExclusion), and BTPL appears in
// exactly one table of one report.
const (
	EntityPESCO     Entity = "PESCO"
	EntityIESCO     Entity = "IESCO"
	EntityGEPCO     Entity = "GEPCO"
	EntityFESCO     Entity = "FESCO"
	EntityLESCO     Entity = "LESCO"
	EntityMEPCO     Entity = "MEPCO"
	EntityQESCO     Entity = "QESCO"
	EntitySEPCO     Entity = "SEPCO"
	EntityHESCO     Entity = "HESCO"
	EntityKElectric Entity = "K-Electric"

	// EntityTESCO is Tribal Areas Electric Supply Company. It is NOT part of
	// the roster: NEPRA collects its data and then excludes it, in writing.
	EntityTESCO Entity = "TESCO"
	// EntityBTPL is Bulk Transmission Power Ltd, which appears only in the
	// FY2014-15 complaints table -- evidence that the roster is per-table, not
	// per-report.
	EntityBTPL Entity = "BTPL"

	// EntityWeightedAverage is the "W. Av:" summary row. It is a distinct
	// entity value so that it can never be mistaken for an 11th DISCO; it is
	// kept out of Report.Observations entirely.
	EntityWeightedAverage Entity = "W.Av"
)

// roster is the canonical ten, in NEPRA's own table order.
var roster = []Entity{
	EntityPESCO, EntityIESCO, EntityGEPCO, EntityFESCO, EntityLESCO,
	EntityMEPCO, EntityQESCO, EntitySEPCO, EntityHESCO, EntityKElectric,
}

// Roster returns the ten entities NEPRA actually evaluates in a PER, in report
// order. TESCO is not among them; see TESCOExclusion.
func Roster() []Entity {
	out := make([]Entity, len(roster))
	copy(out, roster)
	return out
}

// InRoster reports whether e is one of the ten evaluated entities.
func InRoster(e Entity) bool {
	for _, r := range roster {
		if r == e {
			return true
		}
	}
	return false
}

// TESCOExclusion is NEPRA's verbatim, on-the-record reason for leaving TESCO
// out of the PER, quoted from the FY2024-25 report. It is recorded as data so
// that a caller asking for TESCO gets an explanation instead of an empty row or
// -- far worse -- a zero.
const TESCOExclusion = "although the data has been obtained from TESCO, the electricity supply " +
	"to the large number of consumers remains un-metered. Furthermore, TESCO's distribution and " +
	"data recording systems are largely unreliable for most of the parameters, making data " +
	"unsuitable for inclusion in this report. Consequently, TESCO's data has not been incorporated."

// TESCOExclusionSource identifies where TESCOExclusion is stated.
const TESCOExclusionSource = "NEPRA Performance Evaluation Report, Distribution Companies, FY2024-25"

// TESCOExclusionStatedFY is the fiscal year whose report actually carries the
// [TESCOExclusion] wording. It is a SINGLE year because that is all the
// evidence supports, and the distinction matters: the FY2014-15 report both
// INCLUDES TESCO with 48 real published figures (its complaints table lists 12
// entities) and would, under an unscoped rule, have been made to assert that
// NEPRA excluded TESCO — quoting wording published a decade later. NEPRA's
// treatment of TESCO in the years between is simply not established here.
const TESCOExclusionStatedFY = "FY2024-25"

// ExclusionReason returns the on-the-record exclusion Value for an entity that
// NEPRA deliberately leaves out, and Absent() for entities it does evaluate.
//
// It asks only "is this entity excluded somewhere on the record?", so it does
// not know which report is being read. Use [ExclusionReasonInFY] when a
// specific report is in hand; asserting this quote against an arbitrary year
// back-dates it.
func ExclusionReason(e Entity) Value {
	if e == EntityTESCO {
		return Excluded(TESCOExclusion + " [" + TESCOExclusionSource + "]")
	}
	return Absent()
}

// ExclusionReasonInFY returns the exclusion Value only for a report that
// actually states it, and Absent() otherwise.
//
// Absent is the honest answer for an unattested year: it means "this build has
// no record of how that report treated the entity", which is different from
// both "the entity was excluded" and "the entity reported nothing".
func ExclusionReasonInFY(e Entity, reportFY string) Value {
	if e == EntityTESCO && reportFY == TESCOExclusionStatedFY {
		return Excluded(TESCOExclusion + " [" + TESCOExclusionSource + "]")
	}
	return Absent()
}

// entityAliases maps the normalised forms a PER's text layer produces onto the
// canonical Entity. Normalisation strips spaces, dots and case, so "K -
// Electric", "K-Electric" and "KElectric" all land here.
var entityAliases = map[string]Entity{
	"pesco":           EntityPESCO,
	"iesco":           EntityIESCO,
	"gepco":           EntityGEPCO,
	"fesco":           EntityFESCO,
	"lesco":           EntityLESCO,
	"mepco":           EntityMEPCO,
	"qesco":           EntityQESCO,
	"sepco":           EntitySEPCO,
	"hesco":           EntityHESCO,
	"kelectric":       EntityKElectric,
	"ke":              EntityKElectric,
	"kel":             EntityKElectric,
	"kelectriclt":     EntityKElectric,
	"tesco":           EntityTESCO,
	"btpl":            EntityBTPL,
	"wav":             EntityWeightedAverage,
	"wavg":            EntityWeightedAverage,
	"wave":            EntityWeightedAverage,
	"waverage":        EntityWeightedAverage,
	"wtdav":           EntityWeightedAverage,
	"weightedav":      EntityWeightedAverage,
	"weightedavg":     EntityWeightedAverage,
	"weightedaverage": EntityWeightedAverage,
	"total":           EntityWeightedAverage,
	"overall":         EntityWeightedAverage,
}

// weakAlias lists entity aliases that are also ordinary English words. They
// are honoured only as the first cell of a row.
var weakAlias = map[string]bool{"total": true, "overall": true, "wave": true}

// maxEntitySpans is how many consecutive spans LookupEntitySpans will join
// looking for an entity name. NEPRA draws "W. Av:" as two runs and, in some
// years, "K - Electric" as three.
const maxEntitySpans = 3

// LookupEntitySpans finds the leftmost entity name in a row, joining up to
// maxEntitySpans consecutive cells. It returns the entity, the index of its
// first span and the index one past its last.
//
// Joining matters: the weighted-average row is drawn as "W." + "Av:", and a
// single-span matcher silently misses it -- which would leave the summary row
// either unparsed or, worse, parsed as a DISCO.
func LookupEntitySpans(texts []string) (e Entity, start, end int, ok bool) {
	for i := range texts {
		for n := maxEntitySpans; n >= 1; n-- {
			if i+n > len(texts) {
				continue
			}
			if n > 1 && !joinableName(texts[i:i+n]) {
				continue
			}
			key := normalizeEntityToken(strings.Join(texts[i:i+n], ""))
			if weakAlias[key] && i > 0 {
				// "Total" is a summary-row name only at the start of a row.
				// Mid-row it is a column heading ("Total No. of Faults"), and
				// treating that as a row turns a header line into data.
				continue
			}
			if got, found := entityAliases[key]; found {
				return got, i, i + n, true
			}
		}
	}
	return "", 0, 0, false
}

// joinableName guards the multi-span join. Entity normalisation drops
// everything but letters, so joining "PESCO" with the data cell "37.15" would
// still normalise to "pesco" and swallow the row's first figure. A join is
// therefore only allowed across short, digit-free spans.
func joinableName(parts []string) bool {
	total := 0
	for _, p := range parts {
		if len(p) > 12 {
			return false
		}
		total += len(p)
		if strings.ContainsAny(p, "0123456789") {
			return false
		}
	}
	return total <= 24
}

// normalizeEntityToken reduces a raw span to the key used in entityAliases.
func normalizeEntityToken(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		}
	}
	return b.String()
}

// LookupEntity resolves a raw table cell to an Entity. ok is false for anything
// that is not an entity name, which is how prose lines are kept out of tables.
func LookupEntity(raw string) (Entity, bool) {
	key := normalizeEntityToken(raw)
	if key == "" {
		return "", false
	}
	e, ok := entityAliases[key]
	return e, ok
}
