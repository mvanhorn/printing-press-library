// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
// See .printing-press-patches/nepra-coverage-ledger.json.
//
// pp:data-source computed
// Supported strategies: auto, local, live, or computed. `computed` is the
// DEFAULT path and this annotation is about the default: coverage joins three
// in-code catalogues and makes NO request. There is no local store to read
// and nothing here is fetched, so neither `local` nor `live` describes it.
//
// WHY THIS COMMAND EXISTS. Absorb manifest rows 2 and 3 were approved against
// the command path `nepra-pp-cli coverage`, and the path resolved to nothing,
// so the Phase 3 completion gate failed on both. The parity target for row 2
// was ebillpakistan.pk's NEPRA_SOURCE object — a hand-maintained JavaScript
// literal saying where each tariff figure came from — and for row 3 its
// scripts/tariff-staleness.mjs, which exits 1 past STALE_AFTER_DAYS=90 for
// one surface. This is the beat-it version of both: provenance is MEASURED
// per artifact rather than retyped as prose, and the staleness gate covers
// EVERY surface this CLI can serve instead of one.
//
// IT IS NOT A SECOND `verify` AND NOT A SECOND `sources`. verify FETCHES and
// gates one surface family against byte floors, decoys and the column
// fingerprint. sources enumerates DOCUMENTS so a caller can discover the
// argument another command needs. coverage answers a third question that
// neither does — "for everything this CLI serves, what do we know, how old is
// that knowledge, and which command answers it" — over all FOUR catalogues
// at once, offline.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newNepraCoverageCmd(flags))
	})
}

// coverageStaleAfterDefault matches verify's window and ebillpakistan.pk's
// own STALE_AFTER_DAYS, so the two gates cannot disagree by default.
const coverageStaleAfterDefault = verifyStaleAfterDefault

// coverageRow is one surface this CLI can serve.
//
// NO FIELD THAT CARRIES A MEASUREMENT USES omitempty. The generated
// compactListFields drops any key present on fewer than ~80% of rows, and
// only 34 of the 63 catalogued documents carry a measured byte count — well
// under that line — so an omitempty bytes field would vanish from --compact
// and --agent output entirely and take the evidence with it.
type coverageRow struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	// Path is NEPRA's own encoding, byte for byte. It is the join key across
	// the four catalogues: the ids are NOT unique across them (verify and
	// sources both call the FY2023-24 workbook "gen-2023-24" while carrying
	// different byte floors), so joining on id would silently fuse two
	// different claims into one row.
	Path     string `json:"path"`
	Label    string `json:"label"`
	Coverage string `json:"coverage"`
	// State is the catalogue's own verdict: published, reachable,
	// unavailable, forbidden, decoy, indexed_unmeasured or reference.
	State string `json:"state"`
	// Catalogues names every in-code catalogue that carries this path, so a
	// disagreement between them is visible rather than resolved silently.
	Catalogues []string `json:"catalogues"`
	// ServedBy is the command that actually answers for this surface. A
	// surface nothing serves is a finding, not a blank.
	ServedBy string `json:"served_by"`

	// BytesAsOf is the decoded length when it was measured, nil when it never
	// was. nil is a DIFFERENT fact from zero.
	BytesAsOf *int `json:"bytes_as_of"`
	// ByteFloor is the length below which a 200 is not data, and it is a
	// POINTER for the same reason BytesAsOf is: nil means NO FLOOR WAS
	// DECLARED, which is a different fact from a declared floor of zero.
	// Emitting 0 for both made 72 of the ledger's rows claim a floor they do
	// not have, while the sibling bytes_as_of on the same row correctly said
	// null — an inconsistency an output review picked up.
	//
	// FloorOwner names which catalogue the floor came from, because verify's
	// per-year floors (426,282..516,219) and sources' blanket 400,000 are
	// both real and are not the same claim.
	ByteFloor  *int   `json:"byte_floor"`
	FloorOwner string `json:"floor_owner"`
	// RowFloor is the extracted-row floor, nil when none was declared. Note
	// a floor of 0 IS declared for the two register pages and the one
	// determination page that are published, reachable and genuinely empty.
	RowFloor *int `json:"row_floor"`
	// HTTPStatusAsOf is populated ONLY where a non-2xx was actually
	// observed. It is NEVER set on a success: the generated client returns
	// the body without the status code, so recording "200" would be
	// asserting a value this build never saw. See meta.provenance_note.
	HTTPStatusAsOf *int `json:"http_status_as_of"`

	// MeasuredAt is when the numbers on this row were measured, and AgeDays
	// is how old that is. AgeDays is nil when MeasuredAt is unreadable,
	// because an unparseable date silently becoming "0 days old" would read
	// as perfectly fresh.
	MeasuredAt string `json:"measured_at"`
	AgeDays    *int   `json:"age_days"`
	// Stale is nil when the age could not be computed. A surface whose age
	// is unknown is NOT reported as fresh.
	Stale *bool `json:"stale"`
}

// coverageProvenance is the per-artifact provenance contract, emitted under
// --provenance.
//
// This is row 2's deliverable and it is a DECLARATION of what a fetch of this
// surface records, not a fabricated record of a fetch that did not happen.
// coverage makes no request, so the fields it would populate are named with
// their source rather than filled with invented values.
type coverageProvenance struct {
	Path string `json:"path"`
	// URL is the absolute URL a fetch would record. It is built from the
	// client's own base, so it cannot drift from what the fetch would use.
	URL string `json:"url"`
	// RecordsFields is what nepraArtifact captures on every fetch of this
	// surface, in the order it captures them.
	RecordsFields []string `json:"records_fields"`
	// OmitsFields is what this CLI deliberately does NOT record, each with
	// the reason. An absent field with a stated reason is a different thing
	// from an oversight.
	OmitsFields map[string]string `json:"omits_fields"`
	// ComparableAcrossFetches names the ONE hash that may be diffed between
	// two fetches, and why the other may not.
	ComparableAcrossFetches string `json:"comparable_across_fetches"`
	NotComparableReason     string `json:"not_comparable_reason"`
	// MeasuredAt / BytesAsOf are the last recorded measurement for this
	// surface, or nil. They are evidence, not a live reading.
	MeasuredAt string `json:"measured_at"`
	BytesAsOf  *int   `json:"bytes_as_of"`
}

type coverageReport struct {
	Meta    coverageMeta  `json:"meta"`
	Results []coverageRow `json:"results"`
}

type coverageMeta struct {
	Source string `json:"source"`
	// Requests is always 0 and is emitted so a caller can see that, rather
	// than infer it.
	Requests       int                  `json:"requests"`
	CheckedAt      string               `json:"checked_at"`
	WindowDays     int                  `json:"window_days"`
	Gated          bool                 `json:"gated"`
	Surfaces       int                  `json:"surfaces"`
	SurfacesStale  int                  `json:"surfaces_stale"`
	SurfacesUnaged int                  `json:"surfaces_unaged"`
	SurfacesServed int                  `json:"surfaces_served"`
	Verdict        string               `json:"verdict"`
	CataloguesJoin string               `json:"catalogues_joined"`
	ProvenanceNote string               `json:"provenance_note"`
	StalenessNote  string               `json:"staleness_note"`
	DeclaredLimits int                  `json:"declared_limits"`
	Provenance     []coverageProvenance `json:"provenance,omitempty"`
	FloorDisagreed []string             `json:"floor_disagreements,omitempty"`
}

// coverageBuild joins the three in-code catalogues into one ledger.
//
// THE JOIN KEY IS THE PATH, NEVER THE ID. verify and sources both name the
// FY2023-24 workbook "gen-2023-24" while carrying different byte floors
// (verify's measured 493,187 against sources' blanket 400,000), so an id join
// fuses two different claims and silently drops one. Joining on the path
// keeps both and records which catalogue owns the floor.
//
// now is a parameter so the gate's boundary behaviour is testable without
// waiting 90 days.
func coverageBuild(now time.Time, windowDays int) ([]coverageRow, []string) {
	byPath := map[string]*coverageRow{}
	order := []string{}

	add := func(path string) *coverageRow {
		if r, ok := byPath[path]; ok {
			return r
		}
		r := &coverageRow{Path: path}
		byPath[path] = r
		order = append(order, path)
		return r
	}
	note := func(r *coverageRow, cat string) {
		for _, c := range r.Catalogues {
			if c == cat {
				return
			}
		}
		r.Catalogues = append(r.Catalogues, cat)
	}

	var disagreements []string

	// sources: the widest catalogue, and the only one carrying a coverage
	// window and an observed non-2xx status per row.
	for _, d := range sourcesDocs() {
		if d.Path == "" {
			continue
		}
		r := add(d.Path)
		note(r, "sources")
		r.ID, r.Kind, r.Label = d.ID, d.Kind, d.Label
		r.Coverage, r.State = d.Coverage, d.State
		r.BytesAsOf = d.BytesAsOf
		r.HTTPStatusAsOf = d.HTTPStatusAsOf
		r.MeasuredAt = sourcesAsOfDate
		if d.ByteFloor > 0 {
			bf := d.ByteFloor
			r.ByteFloor, r.FloorOwner = &bf, "sources"
		}
	}

	// verify: per-surface byte floors measured per fiscal year. Where it
	// overlaps sources, its floor is the more specific claim and wins — but
	// the disagreement is recorded rather than swallowed.
	for _, s := range verifySurfaces {
		r := add(s.Path)
		note(r, "verify")
		if r.ID == "" {
			r.ID, r.Kind, r.Label = s.ID, s.Kind, s.Label
		}
		if r.Coverage == "" {
			r.Coverage = s.FY
		}
		if r.State == "" {
			r.State = "published"
		}
		if s.ByteFloor > 0 {
			if r.ByteFloor != nil && *r.ByteFloor != s.ByteFloor {
				disagreements = append(disagreements, fmt.Sprintf(
					"%s: sources carries byte_floor %d and verify carries %d; verify's is per-fiscal-year and measured, so it owns the row",
					s.ID, *r.ByteFloor, s.ByteFloor))
			}
			bf := s.ByteFloor
			r.ByteFloor, r.FloorOwner = &bf, "verify"
		}
		if s.RowFloor > 0 {
			rf := s.RowFloor
			r.RowFloor = &rf
		}
		if s.ByteFloorAsOf != "" {
			r.MeasuredAt = s.ByteFloorAsOf
		}
	}

	// licence: the register's 21 key/value pages. Added after an output review
	// found coverage claiming to cover "every surface this CLI can serve"
	// while omitting them entirely — 83 rows against a real 104, and
	// served_by never naming `licence`. A ledger that silently excludes a
	// whole command family is exactly the failure it exists to prevent.
	for _, l := range licenceSurfaces {
		r := add(l.Path)
		note(r, "licence")
		if r.ID == "" {
			r.ID, r.Kind, r.Label = l.ID, "licence", l.Label
		}
		if r.State == "" {
			r.State = "published"
		}
		if r.Coverage == "" {
			r.Coverage = "generation licence register, open-ended"
		}
		// EntitiesAsOf 0 is a MEASURED zero for the Sindh and Net-Metering
		// pages, so it is recorded as a floor of 0 that asserts nothing
		// rather than being dropped.
		// A measured zero IS a floor: the Sindh and Net-Metering pages are
		// published, reachable and genuinely empty.
		rf := l.EntitiesAsOf
		r.RowFloor = &rf
		if r.BytesAsOf == nil && l.BytesAsOf > 0 {
			b := l.BytesAsOf
			r.BytesAsOf = &b
		}
		if r.MeasuredAt == "" {
			r.MeasuredAt = licenceAsOfDate
		}
	}

	// events: the only CURRENT surface family on the site, and the only one
	// whose floor is a row count rather than a byte count.
	for _, e := range eventSurfaces {
		r := add(e.Path)
		note(r, "events")
		if r.ID == "" {
			r.ID, r.Kind, r.Label = e.ID, "event", e.Label
		}
		if r.State == "" {
			r.State = "published"
		}
		if r.Coverage == "" {
			r.Coverage = "determination feed, open-ended"
		}
		// RowsAsOf 0 is a MEASURED zero for ipp-waste, not an unmeasured
		// one, so it is recorded as a floor of 0 that asserts nothing rather
		// than being dropped.
		if e.RowsMeasured {
			rf := e.RowsAsOf
			r.RowFloor = &rf
		}
		if r.MeasuredAt == "" {
			r.MeasuredAt = eventsAsOfDate
		}
	}

	out := make([]coverageRow, 0, len(order))
	for _, p := range order {
		r := byPath[p]
		r.ServedBy = coverageServedBy(*r)
		coverageApplyAge(r, now, windowDays)
		if r.Catalogues == nil {
			r.Catalogues = []string{}
		}
		out = append(out, *r)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].ID < out[j].ID
	})
	return out, disagreements
}

// coverageServedBy names the command that answers for a surface.
//
// It is derived from the kind rather than hand-listed per row so a new
// catalogue entry cannot silently arrive with no command behind it: an
// unrecognised kind returns the empty string and is counted in
// meta.surfaces_served, which is the finding.
func coverageServedBy(r coverageRow) string {
	// `sources` is appended to every row it catalogues, not listed per kind.
	// It is the ENUMERATOR: every other command in this CLI takes an argument
	// the caller can only supply if they already know it, and sources is
	// where that argument comes from. Omitting it made the ledger report
	// that the command serving the whole document catalogue served nothing —
	// found by TestCoverageCoversEveryCommandFamily.
	enumerated := false
	for _, c := range r.Catalogues {
		if c == "sources" {
			enumerated = true
		}
	}
	base := ""
	switch r.Kind {
	case "gen":
		base = "gen, generation plants, generation monthly, capacity, verify"
	case "per":
		base = "disco, conflicts, reliability"
	case "fca":
		base = "fca"
	case "tariff":
		base = "tariff, events"
	case "sir":
		base = "sir"
	case "event":
		base = "events"
	case "licence":
		base = "licence"
	case verifyKindSheet:
		base = "fca, hydel, quarterly, sro, verify"
	case verifyKindWorkbook:
		base = "gen, generation plants, generation monthly, capacity, verify"
	default:
		return ""
	}
	if enumerated {
		base += ", sources"
	}
	return base
}

// coverageBuildProvenance declares what a fetch of each surface records.
func coverageBuildProvenance(rows []coverageRow, baseURL string) []coverageProvenance {
	out := make([]coverageProvenance, 0, len(rows))
	for _, r := range rows {
		out = append(out, coverageProvenance{
			Path:          r.Path,
			URL:           baseURL + r.Path,
			RecordsFields: []string{"url", "bytes", "sha256_raw", "sha256_content", "fetched_at", "content_type", "rows", "skipped_rows"},
			OmitsFields: map[string]string{
				"http_status": "the generated client returns the body without the status code, so recording \"200\" would assert a value never observed. A non-2xx that WAS observed is carried per row as http_status_as_of.",
			},
			ComparableAcrossFetches: "sha256_content",
			NotComparableReason:     "sha256_raw is over the bytes this fetch received and MUST NOT be diffed: Cloudflare's email-obfuscation filter rewrites every data-cfemail token per response, so two identical fetches of one page seconds apart returned the same 74,431 bytes with different hashes. The same page is also 74,431 vs 74,792 bytes depending on the request's Accept header, and every text/html body on this host carries a ~361-byte injected RUM beacon.",
			MeasuredAt:              r.MeasuredAt,
			BytesAsOf:               r.BytesAsOf,
		})
	}
	return out
}

func newNepraCoverageCmd(flags *rootFlags) *cobra.Command {
	var (
		flagProvenance bool
		flagCheckStale bool
		flagStaleAfter int
		flagKind       string
	)

	cmd := &cobra.Command{
		Use:   "coverage",
		Short: "What this CLI can serve, where each figure came from, and how old that knowledge is",
		Long: "The coverage ledger: every surface this CLI can serve, joined from the four catalogues it\n" +
			"ships, with the command that answers for each and the age of every measurement.\n\n" +
			"MAKES NO REQUEST, ever. It reads the in-code catalogues, so it is safe in a cron and costs\n" +
			"NEPRA nothing. `verify` is the command that fetches and gates; `sources` is the command that\n" +
			"enumerates documents so you can discover an argument. This one answers neither of those.\n\n" +
			"--provenance declares what a fetch of each surface RECORDS, field by field, including what it\n" +
			"deliberately does not record and why. http_status is absent on purpose: the client returns the\n" +
			"body without the status code, so a recorded \"200\" would be a value never observed. Only\n" +
			"sha256_content is comparable between two fetches — sha256_raw changes on every response\n" +
			"because Cloudflare rewrites the page's email-obfuscation tokens each time.\n\n" +
			"--check-stale exits non-zero when ANY surface is older than the window, which is the whole\n" +
			"point: the staleness signal applies to every surface, not just one. A surface whose age cannot\n" +
			"be computed is reported as unaged and is NEVER counted as fresh.\n\n" +
			"Exit codes:\n" +
			"  0 the ledger printed, and under --check-stale nothing is past the window\n" +
			"  2 a usage refusal\n" +
			"  6 --check-stale found at least one surface past the window",
		Example: "  nepra-pp-cli coverage\n" +
			"  nepra-pp-cli coverage --kind gen --json\n" +
			"  nepra-pp-cli coverage --provenance --json\n" +
			"  nepra-pp-cli coverage --check-stale --stale-after 120",
		Annotations: map[string]string{
			// Reads in-code catalogues only. No request, no write.
			"mcp:read-only":       "true",
			"pp:happy-args":       "--kind=gen",
			"pp:typed-exit-codes": "0,2,6",
			"pp:novel-hand-coded": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "coverage")
			}
			if len(args) > 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("coverage takes no positional arguments; got %q", args[0]))
			}
			if flagStaleAfter < 0 {
				return usageErr(fmt.Errorf("--stale-after must not be negative; got %d", flagStaleAfter))
			}
			window := flagStaleAfter
			if !cmd.Flags().Changed("stale-after") {
				window = coverageStaleAfterDefault
			}

			rows, disagreements := coverageBuild(time.Now().UTC(), window)
			if flagKind != "" {
				kinds := map[string]bool{}
				for _, r := range rows {
					kinds[r.Kind] = true
				}
				if !kinds[flagKind] {
					known := make([]string, 0, len(kinds))
					for k := range kinds {
						known = append(known, k)
					}
					sort.Strings(known)
					return usageErr(fmt.Errorf("--kind %q is not a catalogued kind; this build carries: %s",
						flagKind, strings.Join(known, ", ")))
				}
				kept := make([]coverageRow, 0, len(rows))
				for _, r := range rows {
					if r.Kind == flagKind {
						kept = append(kept, r)
					}
				}
				rows = kept
			}

			stale, unaged, served := 0, 0, 0
			for _, r := range rows {
				switch {
				case r.Stale == nil:
					unaged++
				case *r.Stale:
					stale++
				}
				if r.ServedBy != "" {
					served++
				}
			}

			meta := coverageMeta{
				Source:         "catalogue",
				Requests:       0,
				CheckedAt:      time.Now().UTC().Format(time.RFC3339),
				WindowDays:     window,
				Gated:          flagCheckStale,
				Surfaces:       len(rows),
				SurfacesStale:  stale,
				SurfacesUnaged: unaged,
				SurfacesServed: served,
				CataloguesJoin: "sources, verify, licence, events — joined on the request PATH, never on the id: verify and sources both call the FY2023-24 workbook \"gen-2023-24\" while carrying different byte floors, so an id join would fuse two different claims and drop one",
				ProvenanceNote: "http_status is never recorded on a success. The generated client returns the body without the status code, so a recorded \"200\" would assert a value this build never observed; a non-2xx that WAS observed is carried per row as http_status_as_of.",
				StalenessNote:  "age is measured from the date each catalogue recorded its numbers, not from the document's publication date. A surface whose age cannot be computed is reported as unaged and is never counted as fresh.",
				DeclaredLimits: len(nepraScopeLimits),
				FloorDisagreed: disagreements,
			}
			meta.Verdict = verifyPass
			if stale > 0 {
				meta.Verdict = verifyFail
			}
			if flagProvenance {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				meta.Provenance = coverageBuildProvenance(rows, c.RequestBaseURL())
			}

			report := coverageReport{Meta: meta, Results: rows}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFiltered(cmd.OutOrStdout(), report, flags); err != nil {
					return err
				}
			} else {
				coverageWriteHuman(cmd, report)
			}

			if flagCheckStale && stale > 0 {
				return gateErr(fmt.Errorf("%d of %d surfaces are more than %d days old; the oldest measurement is %s",
					stale, len(rows), window, coverageOldest(rows)))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&flagProvenance, "provenance", false, "Declare what a fetch of each surface "+
		"records, field by field, plus what it deliberately does not record and why. Makes no request")
	cmd.Flags().BoolVar(&flagCheckStale, "check-stale", false, "Exit 6 when ANY surface is older than "+
		"the window. The ages print either way; this flag only changes the exit code, which is what makes "+
		"the command usable as a cron gate")
	cmd.Flags().IntVar(&flagStaleAfter, "stale-after", coverageStaleAfterDefault, "Staleness window in days")
	cmd.Flags().StringVar(&flagKind, "kind", "", "Restrict the ledger to one catalogued kind. An "+
		"unrecognised kind exits 2 and lists the kinds this build carries")
	return cmd
}

// coverageOldest names the oldest measurement date among the rows, for the
// gate message. It reports a date rather than a count so the reader knows
// what to go and re-measure.
func coverageOldest(rows []coverageRow) string {
	oldest := ""
	for _, r := range rows {
		if r.AgeDays == nil {
			continue
		}
		if oldest == "" || r.MeasuredAt < oldest {
			oldest = r.MeasuredAt
		}
	}
	if oldest == "" {
		return "unknown (no row carries a readable measurement date)"
	}
	return oldest
}

func coverageWriteHuman(cmd *cobra.Command, rep coverageReport) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "%d surfaces  %d stale  %d unaged  window %d days  verdict %s  (no request made)\n",
		rep.Meta.Surfaces, rep.Meta.SurfacesStale, rep.Meta.SurfacesUnaged, rep.Meta.WindowDays, rep.Meta.Verdict)
	fmt.Fprintf(out, "joined from: %s\n\n", rep.Meta.CataloguesJoin)
	for _, r := range rep.Results {
		age := "unaged"
		if r.AgeDays != nil {
			age = fmt.Sprintf("%dd", *r.AgeDays)
		}
		flag := " "
		if r.Stale != nil && *r.Stale {
			flag = "*"
		}
		fmt.Fprintf(out, "%s %-22s %-10s %-14s %-6s %s\n", flag, r.ID, r.Kind, r.State, age, r.ServedBy)
	}
	if len(rep.Meta.FloorDisagreed) > 0 {
		fmt.Fprintf(out, "\nfloor disagreements between catalogues (both kept, verify owns the row):\n")
		for _, d := range rep.Meta.FloorDisagreed {
			fmt.Fprintf(out, "  %s\n", d)
		}
	}
	fmt.Fprintf(out, "\nprovenance: %s\n", rep.Meta.ProvenanceNote)
	fmt.Fprintf(out, "staleness:  %s\n", rep.Meta.StalenessNote)
}

// coverageApplyAge sets AgeDays and Stale from the row's measurement date.
//
// IT IS A SEPARATE FUNCTION SO THE UNAGED CASE IS REACHABLE FROM A TEST.
// Every row in the shipped catalogues carries a readable date, so an
// invariant checked only by walking the real rows asserts nothing about the
// branch that matters — the same vacuous shape as a test whose demonstration
// sits in a condition that can never fire.
//
// THE ASYMMETRY IS THE POINT. When the date cannot be read the age is
// UNKNOWN, and both fields stay nil: nil means "not known", false would mean
// "checked, and fresh". Letting an unreadable date become 0 days old is
// precisely the failure a staleness gate exists to prevent, because it
// reports the least-known surface as the freshest one.
func coverageApplyAge(r *coverageRow, now time.Time, windowDays int) {
	days, ok := verifyDaysBetween(r.MeasuredAt, now)
	if !ok {
		r.AgeDays = nil
		r.Stale = nil
		return
	}
	d := days
	r.AgeDays = &d
	stale := days > windowDays
	r.Stale = &stale
}
