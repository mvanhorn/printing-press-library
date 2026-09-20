package nepraper

// This file holds the corpus facts that are DATA, not code: which reports
// exist, which figures must never be parsed as numbers, and which known
// artefacts a caller should be warned about. Encoding them as tables rather
// than as branches inside the parser keeps them auditable and keeps the parser
// free of year-specific special cases.

import "sort"

// Availability says whether a fiscal year's PER can be obtained at all.
type Availability int

const (
	// AvailabilityUnknown means this package has no record for the year.
	AvailabilityUnknown Availability = iota
	// AvailabilityPublished means the PDF was fetched and parsed.
	AvailabilityPublished
	// AvailabilityUnavailable means the report is referenced by NEPRA but
	// cannot be retrieved -- the link is on NEPRA's index and returns 404.
	// This is emphatically NOT "the report says nothing"; the difference
	// matters when a caller reasons about a gap in a time series.
	AvailabilityUnavailable
)

func (a Availability) String() string {
	switch a {
	case AvailabilityPublished:
		return "published"
	case AvailabilityUnavailable:
		return "unavailable"
	}
	return "unknown"
}

// SourceRecord is one PER PDF as this package found it.
//
// URLPath is stored exactly as NEPRA serves it. Do not "clean" it: the
// FY2020-21 path really does end in "Companies%20.pdf" with a trailing encoded
// space, and the same URL without that %20 returns 404. Filenames are also
// unconstructible across years -- "PER DISCOs 2018-19", "PER DISCOs 2019-20
// updated", "NEPRA PER 2021 Distribution Companies ", "PER-DISCO FY 2021-22
// final", "PER 2022-23 - DSICOs" (NEPRA's own typo) -- so the paths are a
// table, not a pattern.
type SourceRecord struct {
	FY           string
	URLPath      string
	Bytes        int
	Availability Availability
	// Pages is the page count read from the PDF page tree. file(1) reports
	// "1 pages" for six of these seven files; it is wrong and must not be
	// trusted.
	Pages int
	// Creator is the /Creator entry in the document info dictionary.
	Creator string
	// HasTextLayer records that the PDF carries a real text layer and needs no
	// OCR. This held for all seven retrievable reports.
	HasTextLayer bool
	// LowTextPages lists 1-indexed pages with fewer than 20 characters of
	// text -- covers, essentially.
	LowTextPages []int
	Note         string
}

const nepraHost = "https://nepra.org.pk"

// sources is the measured corpus. Byte counts, page counts, creator strings and
// low-text pages were verified by fetching and extracting each file.
var sources = []SourceRecord{
	{
		FY: "FY2014-15", URLPath: "/Standards/PER%20DISCOs%20and%20KE%20for%202014-15.pdf",
		Bytes: 1022593, Availability: AvailabilityPublished, Pages: 27,
		Creator: "Nitro Pro 8", HasTextLayer: true,
		Note: "SAIFI/SAIDI/T&D/recovery figures are embedded Excel chart images with no text " +
			"layer; only the 12-entity complaints table (Table 2) is machine-readable",
	},
	{
		FY: "FY2018-19", URLPath: "/Standards/2020/PER%20DISCOs%202018-19.pdf",
		Bytes: 3024893, Availability: AvailabilityPublished, Pages: 28,
		Creator: "Nitro Pro 8", HasTextLayer: true,
	},
	{
		FY: "FY2019-20", URLPath: "/Standards/2021/PER%20DISCOs%202019-20%20updated.pdf",
		Bytes: 3026235, Availability: AvailabilityPublished, Pages: 26,
		Creator: "Nitro Pro 8", HasTextLayer: true,
	},
	{
		FY: "FY2020-21", URLPath: "/Standards/2022/NEPRA%20PER%202021%20Distribution%20Companies%20.pdf",
		Bytes: 2277838, Availability: AvailabilityPublished, Pages: 29,
		Creator: "Nitro Pro 8", HasTextLayer: true, LowTextPages: []int{1},
		Note: "URL ends in a trailing encoded space before .pdf; without it NEPRA returns 404",
	},
	{
		FY: "FY2021-22", URLPath: "/Standards/2023/PER-DISCO%20FY%202021-22%20final.pdf",
		Bytes: 2435038, Availability: AvailabilityPublished, Pages: 39,
		Creator: "Nitro Pro 8", HasTextLayer: true,
	},
	{
		FY: "FY2022-23", URLPath: "/M&E/PER/Distribution/PER%202022-23%20-%20DSICOs.pdf",
		Bytes: 2483696, Availability: AvailabilityPublished, Pages: 43,
		Creator: "Nitro Pro 8", HasTextLayer: true,
		Note: "NEPRA's own filename misspells DISCOs as DSICOs",
	},
	{
		FY: "FY2023-24", URLPath: "/M&E/PER/Distribution/PER%20DISCOs%202023-24.pdf",
		Bytes: 0, Availability: AvailabilityUnavailable,
		Note: "linked from NEPRA's Performance Reports index but returns HTTP 404 " +
			"(9-byte text/html body); record as UNAVAILABLE, which is not the same as no data",
	},
	{
		FY: "FY2024-25", URLPath: "/M&E/PER/Distribution/2026/PER%202024-25%20Distribution%20Companies.pdf",
		Bytes: 1563742, Availability: AvailabilityPublished, Pages: 35,
		Creator: "", HasTextLayer: true,
		Note: "re-processed by iLovePDF, so /Creator is absent and /Producer is iLovePDF; " +
			"several tables draw every glyph of a cell at one pen position with zero advance " +
			"width, so cells must be read in content-stream order, not by x",
	},
}

// Sources returns the measured source records, oldest first.
func Sources() []SourceRecord {
	out := make([]SourceRecord, len(sources))
	copy(out, sources)
	sort.SliceStable(out, func(i, j int) bool { return out[i].FY < out[j].FY })
	return out
}

// SourceFor returns the record for a fiscal year.
func SourceFor(fy string) (SourceRecord, bool) {
	want := NormalizeFY(fy)
	for _, s := range sources {
		if s.FY == want {
			return s, true
		}
	}
	return SourceRecord{}, false
}

// AvailabilityFor answers "can this year's report be obtained" as a Value, so
// that an unavailable year flows through the same typed channel as a missing
// cell instead of becoming a zero.
// The three answers are materially different states and used to be separated
// only by the prose in Reason:
//
//	published    -> Absent with a reason saying the DOCUMENT is fine and any
//	                missing figure is a gap in the report, not in the fetch
//	unavailable   -> Unavailable, the document itself could not be obtained
//	not in table  -> Unavailable, but because this BUILD has no record of the
//	                year at all, which is not a claim about NEPRA
//
// A caller that must branch on the distinction should read
// [SourceFor](fy).Availability directly; this function exists so the answer
// can travel through the same typed channel as a missing cell.
func AvailabilityFor(fy string) Value {
	s, ok := SourceFor(fy)
	if !ok {
		return Unavailable("no record for " + NormalizeFY(fy) +
			" in this package's source table; this is the absence of a RECORD, not a statement that NEPRA published nothing")
	}
	switch s.Availability {
	case AvailabilityUnavailable:
		return Unavailable(s.FY + ": " + s.Note + " [" + nepraHost + s.URLPath + "]")
	case AvailabilityPublished:
		return Value{Kind: KindAbsent, Reason: s.FY +
			" was published and is obtainable, so there is no availability problem; a figure missing for this year is missing from the REPORT"}
	}
	return Unavailable("availability of " + s.FY + " is unknown")
}

// URL returns the absolute URL for a source record, with NEPRA's own encoding
// preserved byte for byte.
func (s SourceRecord) URL() string { return nepraHost + s.URLPath }

// UnverifiedFigure is a figure that exists in a NEPRA source in a form that
// cannot honestly be read as a number.
//
// The SAIDI values for FY2010-11 through FY2013-14 survive only as data labels
// on an Excel chart that was truncated when it was pasted into the report: the
// text layer holds "19,535." and the remaining digits are simply not in the
// file. Parsing that string as 19535 or 19.535 would be fabrication, so it is
// recorded here as KindUnverified and nothing else.
type UnverifiedFigure struct {
	PeriodFY string
	// Entity is EMPTY for every recorded figure, and that is deliberate
	// rather than an omission: these are data labels on a multi-year
	// SYSTEM-WIDE comparison chart, so there is no per-DISCO attribution to
	// record. An empty Entity here therefore means "not entity-specific", not
	// "we lost the entity" — read [UnverifiedFigure.EntityAttributed] rather
	// than testing Entity against "" or passing it to InRoster, which would
	// report false and look like the blank roster row this package works to
	// prevent everywhere else.
	Entity   Entity
	Metric   Metric
	RawLabel string
	Reason   string
	Source   string
}

// EntityAttributed reports whether this figure belongs to a named DISCO. It is
// false for a system-wide chart label, which is a property of the source, not
// a gap in the record.
func (u UnverifiedFigure) EntityAttributed() bool { return u.Entity != "" }

// UnverifiedGapFYs are fiscal years whose SAIDI is believed to exist ONLY as a
// truncated chart label, but whose raw label this build has not captured.
//
// FY2013-14 is listed because the corpus survey describes the truncated-label
// range as FY2010-11..FY2013-14 while only three labels were actually read off
// the charts. The missing one is recorded as a KNOWN GAP rather than
// interpolated or invented: a fabricated "19,535."-shaped string would be
// indistinguishable from a real reading. Anyone adding it must read it from
// the chart.
var UnverifiedGapFYs = []string{"FY2013-14"}

// knownUnverified is recorded as data because it is a property of the corpus,
// not of the parser. Only figures whose raw label was actually READ are here;
// see [UnverifiedGapFYs] for years believed truncated but not captured.
var knownUnverified = []UnverifiedFigure{
	{PeriodFY: "FY2010-11", Metric: MetricSAIDI, RawLabel: "19,535.",
		Reason: "truncated Excel chart data label; trailing digits absent from the text layer",
		Source: "NEPRA PER, early-year SAIDI comparison chart"},
	{PeriodFY: "FY2011-12", Metric: MetricSAIDI, RawLabel: "28,189.",
		Reason: "truncated Excel chart data label; trailing digits absent from the text layer",
		Source: "NEPRA PER, early-year SAIDI comparison chart"},
	{PeriodFY: "FY2012-13", Metric: MetricSAIDI, RawLabel: "15,896.",
		Reason: "truncated Excel chart data label; trailing digits absent from the text layer",
		Source: "NEPRA PER, early-year SAIDI comparison chart"},
}

// KnownUnverified returns the recorded truncated figures as typed Values.
// Every returned Value has Kind KindUnverified; none has a Num.
func KnownUnverified() []UnverifiedFigure {
	out := make([]UnverifiedFigure, len(knownUnverified))
	copy(out, knownUnverified)
	return out
}

// UnverifiedValue turns a recorded truncated figure into a Value.
func (u UnverifiedFigure) UnverifiedValue() Value {
	return Unverified(u.RawLabel, u.Reason+" ["+u.Source+"]")
}

// KnownArtifact is a documented, reproducible inconsistency in the corpus.
//
// Direction records whether the package can say which of two figures is the
// erroneous one. For the FY2024-25 MEPCO 3x pair the answer is no: both figures
// are triple-attested by their own chart and there is no tie-breaker in the
// document. For the ~1000x MEPCO SAIDI pair the direction is inferred from the
// surrounding series and is marked as inferred, not proven.
type KnownArtifact struct {
	Key         Key
	Description string
	Ratio       float64
	Direction   string // "unknowable" or "inferred: ..."
}

var knownArtifacts = []KnownArtifact{
	{
		Key: Key{PeriodFY: "FY2024-25", Entity: EntityMEPCO, Metric: MetricSAIDI},
		Description: "3547.00 in Table 06 (PDF page 15) vs 1182.56 in Table 18 (PDF page 29); " +
			"each is internally consistent with its own chart",
		Ratio: 2.9994249, Direction: "unknowable",
	},
	{
		Key: Key{PeriodFY: "FY2024-25", Entity: EntityMEPCO, Metric: MetricSAIFI},
		Description: "30.67 in Table 05 (PDF page 13) vs 10.23 in Table 17 (PDF page 28); " +
			"each is internally consistent with its own chart",
		Ratio: 2.9980450, Direction: "unknowable",
	},
	{
		Key: Key{PeriodFY: "FY2024-25", Entity: EntityLESCO, Metric: MetricSAIFI},
		Description: "28.16 in Table 05 (PDF page 13) vs 28.61 in Table 17 (PDF page 28); " +
			"a digit transposition, but the document does not say which way round",
		Ratio: 0.9842712, Direction: "unknowable",
	},
	{
		Key: Key{PeriodFY: "FY2020-21", Entity: EntityMEPCO, Metric: MetricSAIDI},
		Description: "39733 in the FY2020-21 report's Table 6 vs 39.733 in FY2024-25's " +
			"Table 18 comparison column",
		Ratio: 1000,
		Direction: "inferred: the surrounding MEPCO SAIDI series is in the thousands of " +
			"minutes, so 39733 is the likelier reading -- inferred, NOT proven by the documents",
	},
}

// KnownArtifacts returns the documented inconsistencies this package was built
// against, for use as a regression baseline.
func KnownArtifacts() []KnownArtifact {
	out := make([]KnownArtifact, len(knownArtifacts))
	copy(out, knownArtifacts)
	return out
}
