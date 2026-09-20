// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
// See .printing-press-patches/nepra-licence-register.json.
//
// pp:data-source live
// Supported strategies: auto, local, live, or computed. `live` is the only
// honest value: the register is read from the site on every invocation and
// there is no local store for it.
//
// WHY THIS FILE EXISTS. Transcendence row 6 approved
// `licence [--search "Thar"] [--ticker HUBC] [--fuel Coal]` and the shipped
// command had no flags AND returned the wrong document: the spec pointed at
// /licensing/Generation%20IPPs.php, which is a 47,066-byte HUB page with zero
// accordions and zero tables, so html_extract's page mode returned the site
// NAVIGATION MENU — 50 chrome links, no licence row anywhere. root.go's own
// help already advertised "the licence register with gross capacity, plant
// type, fuel and the modification trail, searchable by name, fuel or listed
// operator", so the CLI was describing a capability it did not have.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/client"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		generated, rest, err := root.Find([]string{"licence"})
		if err != nil || generated == nil || generated == root || len(rest) != 0 || generated.Name() != "licence" {
			return
		}
		// AUGMENT IN PLACE rather than RemoveCommand + AddCommand.
		//
		// Replacement worked, but it left TWO files declaring `Use:
		// "licence"` — the generated one and this one — and verify-skill
		// attributes a flag to a command by finding the declaration
		// alongside that command's Use literal. It picked the generated
		// file and reported "--fuel is declared elsewhere but not on
		// licence", a false finding about a flag that works. Mutating the
		// generated command leaves exactly one Use literal, needs no
		// RemoveCommand, and cannot leave two `licence` children.
		//
		// The RunE is replaced OUTRIGHT, not wrapped: with no flags the
		// generated path returns the site navigation menu, which is the
		// defect this file exists to fix, so there is nothing to fall
		// through to.
		augmentLicenceCommand(generated, flags)
	})
}

// licenceFetchConcurrency bounds the parallel page reads.
//
// Six is deliberately modest against a regulator's site: it cuts the 21-page
// read from a measured 52 s to a few seconds while keeping this CLI's
// concurrent footprint on nepra.org.pk below what an ordinary browser opens
// for a single page. The root --rate-limit still applies on top, because the
// generated client paces itself.
const licenceFetchConcurrency = 6

// licenceAsOfDate is when every byte count and entity count in
// licenceSurfaces was measured, by fetching all 22 licensing/Generation*.php
// pages. A floor without a date is an assertion nobody can re-check.
const licenceAsOfDate = "2026-09-11"

// licenceSurface is one register page.
type licenceSurface struct {
	ID string
	// Path carries NEPRA's own encoding. The hub page's hrefs contain
	// LITERAL SPACES, so they cannot be used verbatim as request paths.
	Path  string
	Label string
	// EntitiesAsOf is the accordion count measured on licenceAsOfDate. It is
	// a FLOOR: the register grows.
	EntitiesAsOf int
	// BytesAsOf is the decoded length measured the same day.
	BytesAsOf int
	// InProbeScope records whether this page was inside the 18-page scope the
	// research probe counted as "the register" (335 entities). IGCs and
	// Distributed Generation were NOT, which is why 335 is a scoping decision
	// and not a measurement of the register.
	InProbeScope bool
	// Note carries anything a caller must know before quoting the page.
	Note string
}

// licenceSurfaces is the register catalogue.
//
// TWENTY-ONE pages carry the key/value entity schema, holding 379 accordions
// between them; two of those pages are genuinely empty with an HTTP 200
// (Sindh and Net-Metering). Generation Concurrences.php is DELIBERATELY ABSENT
// from this list: it uses the same accordion CSS but its keys are DATES
// ('23-09-2024'), i.e. the tariff-stream schema, so reading it with this
// shaper would produce 7 entities whose every field is empty.
var licenceSurfaces = []licenceSurface{
	{ID: "ipps-1994", Path: "/licensing/Generation%20IPPs%201994.php", Label: "IPPs 1994 policy", EntitiesAsOf: 15, BytesAsOf: 86436, InProbeScope: true,
		Note: "holds Fauji Kabirwala, whose key/value pairs are transposed upstream"},
	{ID: "ipps-1995-hydel", Path: "/licensing/Generation%20IPPs%201995%20Hydel.php", Label: "IPPs 1995 Hydel policy", EntitiesAsOf: 1, BytesAsOf: 49827, InProbeScope: true},
	{ID: "ipps-2002", Path: "/licensing/Generation%20IPPs%202002.php", Label: "IPPs 2002 policy", EntitiesAsOf: 29, BytesAsOf: 111708, InProbeScope: true},
	{ID: "ipps-2006-kpk", Path: "/licensing/Generation%20IPPs%202006%20kpk.php", Label: "IPPs 2006 KPK", EntitiesAsOf: 21, BytesAsOf: 92165, InProbeScope: true},
	{ID: "ipps-2006-punjab", Path: "/licensing/Generation%20IPPs%202006%20punjab.php", Label: "IPPs 2006 Punjab", EntitiesAsOf: 19, BytesAsOf: 88173, InProbeScope: true},
	{ID: "ipps-2007-balochistan", Path: "/licensing/Generation%20IPPs%202007%20Balochistan.php", Label: "IPPs 2007 Balochistan", EntitiesAsOf: 3, BytesAsOf: 53136, InProbeScope: true},
	{ID: "ipps-2015", Path: "/licensing/Generation%20IPPs%202015.php", Label: "IPPs 2015 policy", EntitiesAsOf: 10, BytesAsOf: 70485, InProbeScope: true},
	{ID: "ipps-re-2006", Path: "/licensing/Generation%20IPPs%20RE%202006.php", Label: "IPPs renewable 2006 policy", EntitiesAsOf: 122, BytesAsOf: 300806, InProbeScope: true,
		Note: "the largest page; holds Almoiz, whose capacity cell reads \"3s 6 MW\" and is refused"},
	{ID: "ipps-sindh", Path: "/licensing/Generation%20IPPs%20Sindh.php", Label: "IPPs Sindh", EntitiesAsOf: 0, BytesAsOf: 47627, InProbeScope: true,
		Note: "published, reachable and genuinely EMPTY: a measured zero, not a missing page"},
	{ID: "ipps-others", Path: "/licensing/Generation%20IPPs%20others.php", Label: "IPPs other policies", EntitiesAsOf: 9, BytesAsOf: 67825, InProbeScope: true},
	{ID: "ipps-short-term", Path: "/licensing/Generation%20IPPs%20short%20term.php", Label: "IPPs short term", EntitiesAsOf: 2, BytesAsOf: 49655, InProbeScope: true,
		Note: "both entities are licence-expired and publish NO capacity key at all"},
	{ID: "cpps", Path: "/licensing/Generation%20CPPs.php", Label: "Captive power plants", EntitiesAsOf: 65, BytesAsOf: 209681, InProbeScope: true},
	{ID: "gencos", Path: "/licensing/Generation%20GENCOs.php", Label: "GENCOs", EntitiesAsOf: 4, BytesAsOf: 56104, InProbeScope: true},
	{ID: "igcs", Path: "/licensing/Generation%20IGCs.php", Label: "Isolated generation companies", EntitiesAsOf: 9, BytesAsOf: 63028, InProbeScope: false,
		Note: "IDENTICAL key/value schema but OUTSIDE the research probe's 335-entity scope"},
	{ID: "k-electric", Path: "/licensing/Generation%20K-Electric.php", Label: "K-Electric generation", EntitiesAsOf: 1, BytesAsOf: 47501, InProbeScope: true,
		Note: "the accordion header misspells the entity as \"K-Elecric\" while the page title spells it correctly"},
	{ID: "ncpps", Path: "/licensing/Generation%20NCPPs.php", Label: "NCPPs", EntitiesAsOf: 10, BytesAsOf: 64785, InProbeScope: true},
	{ID: "npps", Path: "/licensing/Generation%20NPPs.php", Label: "NPPs", EntitiesAsOf: 5, BytesAsOf: 50684, InProbeScope: true},
	{ID: "spps", Path: "/licensing/Generation%20SPPs.php", Label: "SPPs", EntitiesAsOf: 18, BytesAsOf: 91978, InProbeScope: true},
	{ID: "wapda-hydel", Path: "/licensing/Generation%20WAPDA%20Hydel.php", Label: "WAPDA Hydel", EntitiesAsOf: 1, BytesAsOf: 44206, InProbeScope: true,
		Note: "its capacity KEY reads \"Gross Capacityy\"; exact matching alone drops its 17,367.96 MW"},
	{ID: "distributed-generation", Path: "/licensing/Generation%20Distributed%20Generation.php", Label: "Distributed generation", EntitiesAsOf: 35, BytesAsOf: 111883, InProbeScope: false,
		Note: "IDENTICAL key/value schema but OUTSIDE the research probe's 335-entity scope"},
	{ID: "net-metering", Path: "/licensing/Generation%20Netmetering.php", Label: "Net metering", EntitiesAsOf: 0, BytesAsOf: 48909, InProbeScope: false,
		Note: "published, reachable and genuinely EMPTY: a measured zero"},
}

// licenceExcludedSurfaces are pages that look like the register and are not.
var licenceExcludedSurfaces = []licenceSurface{
	{ID: "concurrences", Path: "/licensing/Generation%20Concurrences.php", Label: "Concurrences", EntitiesAsOf: 7, BytesAsOf: 55460,
		Note: "SAME accordion CSS, DIFFERENT schema: its key cells are DATES (\"23-09-2024\"), i.e. the tariff-stream " +
			"date/description/href shape. Reading it with the register shaper would yield 7 entities with every " +
			"field empty, so it is excluded rather than silently mis-parsed."},
	{ID: "hub", Path: "/licensing/Generation%20IPPs.php", Label: "IPP index (hub)", EntitiesAsOf: 0, BytesAsOf: 47066,
		Note: "the path the generated command used. An INDEX: zero accordions, zero tables. Page mode returns the " +
			"site navigation menu, which is what `licence` shipped as its answer."},
}

func licenceSurfaceByID(id string) (licenceSurface, bool) {
	for _, s := range licenceSurfaces {
		if strings.EqualFold(s.ID, id) {
			return s, true
		}
	}
	return licenceSurface{}, false
}

func licenceSurfaceIDs() []string {
	out := make([]string, 0, len(licenceSurfaces))
	for _, s := range licenceSurfaces {
		out = append(out, s.ID)
	}
	return out
}

// licenceEntitiesFloor is the measured total across the catalogue.
func licenceEntitiesFloor(inProbeScopeOnly bool) int {
	n := 0
	for _, s := range licenceSurfaces {
		if inProbeScopeOnly && !s.InProbeScope {
			continue
		}
		n += s.EntitiesAsOf
	}
	return n
}

type licenceReport struct {
	Meta    licenceMeta     `json:"meta"`
	Results []licenceEntity `json:"results"`
}

type licenceMeta struct {
	Source         string   `json:"source"`
	AsOfDate       string   `json:"as_of_date"`
	Surfaces       int      `json:"surfaces_read"`
	SurfacesFailed []string `json:"surfaces_failed,omitempty"`
	// SurfacesShort names pages that answered successfully but came back
	// with FEWER entities than were measured on as_of_date. Distinct from
	// SurfacesFailed: nothing errored, so without this the page is counted
	// as read and the short register is labelled source: live.
	SurfacesShort []string `json:"surfaces_short,omitempty"`
	// Entities is what this run read; EntitiesFloor is what was measured on
	// as_of_date. Coming back with fewer is the signature of a truncation.
	Entities      int `json:"entities"`
	EntitiesFloor int `json:"entities_floor"`
	// DistinctNames is below Entities whenever the register repeats a name.
	// It is emitted so nobody treats the name as a primary key.
	DistinctNames int `json:"distinct_names"`
	Matched       int `json:"matched"`

	Search string `json:"search,omitempty"`
	Fuel   string `json:"fuel,omitempty"`
	// FuelNearMisses are published fuel cells that CONTAIN the --fuel term
	// without carrying it as one of their own tokens. They are reported
	// rather than silently included or dropped.
	FuelNearMisses []licenceFuelNearMissRow `json:"fuel_near_misses,omitempty"`
	// FuelNearMissEntities and FuelNearMissMW total the rows above so the
	// SIZE of the exclusion is readable without summing it. A bare list of
	// strings understates what the caller is not being shown: on
	// `--fuel Coal` the near misses are 8 entities and over 1,600 MW,
	// including cells reading "Imported Coal" and "Indigenous Coal" that
	// most callers would want, alongside "Coal Water Slurry" and "Coke Oven
	// Gas / Blast Furnace Gas /Coal tar" that most would not. This tool
	// cannot know which, so it quantifies the choice rather than guessing.
	FuelNearMissEntities int      `json:"fuel_near_miss_entities,omitempty"`
	FuelNearMissMW       *float64 `json:"fuel_near_miss_mw,omitempty"`
	FuelVocabulary       []string `json:"fuel_vocabulary,omitempty"`

	// The FOUR capacity populations, kept separate for the same reason the
	// generation workbook's three are: merging them answers a different
	// question than the one asked.
	//
	// THEY PARTITION `matched`, NOT `entities`. Under a filter `entities` stays
	// register-wide (it is what was READ) while these four describe only the rows
	// returned, so a bare reading of "entities: 379" beside "4 + 0 + 0 + 0" looks
	// like a contradiction. CapacityScope below says which number they sum to.
	//
	//   parsed          a unit convertible to MW (MW, MWe, kW) — summable
	//   non_mw_unit     a REAL published figure in a unit that is not MW:
	//                   MWp, a solar peak DC rating with no NEPRA-published
	//                   conversion. Measured, not missing.
	//   unreadable      the cell does not parse as <number><unit>. NOTE most of these
	//                   DO carry a unit — "12:00 MW", "110 MW Gross ISO" — and it is
	//                   the NUMBER that is unreadable, so do not describe them as
	//                   unit-less.
	//   no_capacity_key the entity publishes no capacity key
	CapacityParsed         int `json:"entities_with_parsed_capacity"`
	CapacityNotConvertible int `json:"entities_with_non_mw_unit"`
	CapacityUnreadable     int `json:"entities_with_unreadable_capacity"`
	CapacityAbsent         int `json:"entities_with_no_capacity_key"`
	// CapacityScope names the count the four populations above sum to, so a
	// caller never has to guess whether they describe the register or the
	// filtered result.
	CapacityScope   string          `json:"capacity_population_scope"`
	UnitsPublished  map[string]int  `json:"units_published"`
	TransposedRows  int             `json:"entities_with_transposed_pairs"`
	NoCapacityTotal string          `json:"no_capacity_total"`
	Refusals        []string        `json:"refusals"`
	Artifacts       []nepraArtifact `json:"artifacts,omitempty"`
}

func augmentLicenceCommand(cmd *cobra.Command, flags *rootFlags) {

	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations["mcp:read-only"] = "true"
	cmd.Annotations["pp:happy-args"] = "--surface=wapda-hydel"
	cmd.Annotations["pp:typed-exit-codes"] = "0,2,3,5"
	cmd.Annotations["pp:novel-hand-coded"] = "true"

	cmd.Short = "The generation licence register: gross capacity, plant type, fuel and the modification trail"
	cmd.Long = "NEPRA's generation licence register, as rows.\n\n" +
		"21 pages carry the per-entity key/value schema and held 379 licensees when this build measured\n" +
		"them. The research probe scoped \"the register\" at 335 across 18 pages; Isolated Generation\n" +
		"Companies (9) and Distributed Generation (35) carry the IDENTICAL schema and were simply not in\n" +
		"that scope, so both counts are reported and neither is called \"the register\" on its own.\n\n" +
		"THE NAME IS NOT A KEY. 379 accordions carry fewer distinct names than entities — \"Hamza Sugar\n" +
		"Mills Limited\" appears three times — so nothing here dedupes on it and meta.distinct_names is\n" +
		"emitted so a caller cannot assume otherwise.\n\n" +
		"NO HEADLINE CAPACITY TOTAL IS EMITTED. NEPRA publishes no sum, and three cells cannot be read\n" +
		"without inventing a number: one reads \"12:00 MW\", one reads \"3s 6 MW\" on an entity whose own\n" +
		"plant-detail row says 36 MW, and one entity has its key/value pairs TRANSPOSED upstream so its\n" +
		"capacity cell publishes a technology string. Each is flagged and none is repaired.\n\n" +
		"Exit codes:\n" +
		"  0 the register printed\n" +
		"  2 a usage refusal, including --ticker (see below)\n" +
		"  3 an unknown --surface\n\n" +
		"THERE IS NO EXIT 5 AND THAT IS DELIBERATE. A page that fails to fetch or parse does NOT abort the\n" +
		"run: it is NAMED in meta.surfaces_failed, contributes no rows, and the command still exits 0 with\n" +
		"meta.surfaces_read below the catalogue count. A partial register is reported as partial rather\n" +
		"than as nothing, because one unreachable page discarding the other twenty loses more than it\n" +
		"protects. ALWAYS read surfaces_read and surfaces_failed before treating a row count as complete.\n\n" +
		"--ticker IS REFUSED, NOT SILENTLY WRONG. Transcendence row 6 approved it, but the crosswalk\n" +
		"carries generation-workbook spellings while the register publishes legal names: measured against\n" +
		"all 335 in-scope headers, only 28 resolve. `--ticker HUBC` would return three entities and\n" +
		"silently MISS \"Hub Power Company Limited\" (1,292 MW) and its Narowal sibling, while KAPCO, NPL\n" +
		"and NCPL return nothing at all. A flag that answers for 8% of the register while looking\n" +
		"complete is worse than no flag, so it exits 2. Use --search against the register's own names,\n" +
		"or the fleet command's --parent for the generation panel, where the crosswalk is the right\n" +
		"instrument. The limit is declared machine-readably in `doctor --scope`."

	cmd.Example = "  nepra-pp-cli licence\n" +
		"  nepra-pp-cli licence --surface wapda-hydel --json\n" +
		"  nepra-pp-cli licence --search Thar\n" +
		"  nepra-pp-cli licence --fuel Coal --json\n" +
		"  nepra-pp-cli licence --all --json"

	cmd.RunE = func(c *cobra.Command, args []string) error {
		flagSearch, _ := c.Flags().GetString("search")
		flagFuel, _ := c.Flags().GetString("fuel")
		flagTicker, _ := c.Flags().GetString("ticker")
		flagSurface, _ := c.Flags().GetString("surface")
		flagAll, _ := c.Flags().GetBool("all")
		flagStrict, _ := c.Flags().GetBool("strict")
		if dryRunOK(flags) {
			return writeDryRun(c.OutOrStdout(), flags, "licence")
		}
		if len(args) > 0 {
			_ = c.Usage()
			return usageErr(fmt.Errorf("licence takes no positional arguments; got %q", args[0]))
		}
		if flagTicker != "" {
			_ = c.Usage()
			return usageErr(fmt.Errorf(
				"--ticker is refused rather than answered incompletely. The embedded crosswalk carries "+
					"generation-workbook spellings (\"Hub Power Company (HUBCO)\", \"Nishat Power Ltd (NPL)\") "+
					"while this register publishes legal names (\"Hub Power Company Limited\", \"Nishat Power "+
					"Limited\"); measured against all 335 in-scope headers only 28 resolve. --ticker %s would "+
					"return a partial list that LOOKS complete: for HUBC it reaches three entities and misses "+
					"\"Hub Power Company Limited\" (1,292 MW) and \"Hub Power Generation Company (Pvt.) Limited "+
					"Narowal\", and KAPCO, NPL and NCPL each reach zero. Use `licence --search \"<company>\"` to "+
					"search the register's own names, or `fleet --parent %s` for the generation panel, where the "+
					"crosswalk is the right instrument. Closing this properly is a curated-alias data task, and "+
					"it is declared in `doctor --scope`",
				flagTicker, flagTicker))
		}
		if err := validateDataSourceStrategy(flags, "live"); err != nil {
			return usageErr(err)
		}
		if flagSurface != "" && flagAll {
			_ = c.Usage()
			return usageErr(fmt.Errorf("--surface and --all are mutually exclusive"))
		}

		// With no selector at all, print the catalogue and make NO
		// request: reading the whole register is 21 GETs.
		if flagSurface == "" && !flagAll && flagSearch == "" && flagFuel == "" {
			return licencePrintCatalogue(c, flags)
		}

		surfaces := licenceSurfaces
		if flagSurface != "" {
			s, ok := licenceSurfaceByID(flagSurface)
			if !ok {
				return notFoundErr(fmt.Errorf("--surface %q is not a catalogued register page; this build carries: %s",
					flagSurface, strings.Join(licenceSurfaceIDs(), ", ")))
			}
			surfaces = []licenceSurface{s}
		}

		// No shared client: each worker builds its own, because the
		// generated client is not safe for concurrent use (see the goroutine
		// below). A construction failure surfaces per page rather than once
		// up front, which is the same shape as a per-page fetch failure.
		ctx, cancel := boundCtx(c.Context(), flags)
		defer cancel()

		// FETCHED CONCURRENTLY, WITH THE RESULTS REASSEMBLED IN CATALOGUE
		// ORDER.
		//
		// Reading the register is 21 independent GETs of ~1.6 MB total.
		// Serially that MEASURED 52 seconds, which makes --search and --fuel
		// unusable for an agent and long enough that a caller cannot tell a
		// slow answer from a hung one. The work is purely I/O bound, so a
		// small bounded pool turns it into a few seconds.
		//
		// Results land in an INDEXED slice rather than being appended as
		// replies arrive, so row order is DETERMINISTIC and does not depend
		// on which page answered first. An order that varied per run would
		// make two otherwise-identical extracts diff against each other.
		results := licenceFetchAll(surfaces, licenceFetchConcurrency, func(sf licenceSurface) licenceFetch {
			// EACH WORKER GETS ITS OWN CLIENT. The generated client is NOT
			// safe for concurrent use: doInternal writes c.lastContentType
			// (internal/client/client.go:1250) which LastContentType() later
			// reads, so sharing one client across these goroutines is a data
			// race — confirmed by `go build -race`, which reported
			// simultaneous writes from two workers. Constructing one per page
			// is cheap next to the request itself.
			worker, cerr := flags.newClient()
			if cerr != nil {
				return licenceFetch{err: cerr}
			}
			// The HTML-response header is load-bearing: without it the
			// client's JSON guard rejects every page with "expected JSON,
			// API returned HTML instead of JSON" and the register reads as
			// 0 entities across 21 named failures.
			body, ferr := worker.GetWithHeaders(ctx, sf.Path, map[string]string{},
				map[string]string{client.HTMLResponseHeader: "true"})
			if ferr != nil {
				return licenceFetch{err: ferr}
			}
			got, perr := licenceParsePage(body, sf.ID, worker.RequestBaseURL()+sf.Path)
			if perr != nil {
				return licenceFetch{err: perr}
			}
			return licenceFetch{
				entities: got,
				artifact: newNepraArtifact(worker.RequestBaseURL()+sf.Path, body,
					[]byte(licenceContentKey(got)), worker.LastContentType(), len(got), 0),
				ok: true,
			}
		})

		entities, artifacts, failed := licenceAssemble(surfaces, results)
		shortSurfaces := licenceShortSurfaces(surfaces, results)

		meta := licenceMeta{
			Source:         "live",
			AsOfDate:       licenceAsOfDate,
			Surfaces:       len(surfaces) - len(failed),
			SurfacesFailed: failed,
			SurfacesShort:  shortSurfaces,
			Entities:       len(entities),
			Artifacts:      artifacts,
		}
		if flagSurface != "" {
			meta.EntitiesFloor = surfaces[0].EntitiesAsOf
		} else {
			meta.EntitiesFloor = licenceEntitiesFloor(false)
		}
		names := map[string]struct{}{}
		for _, e := range entities {
			names[e.Name] = struct{}{}
		}
		meta.DistinctNames = len(names)

		matched := entities
		if flagSearch != "" {
			kept := make([]licenceEntity, 0, len(matched))
			for _, e := range matched {
				if strings.Contains(strings.ToLower(e.Name), strings.ToLower(flagSearch)) {
					kept = append(kept, e)
				}
			}
			matched = kept
			meta.Search = flagSearch
		}
		if flagFuel != "" {
			kept := make([]licenceEntity, 0, len(matched))
			near := map[string]*licenceFuelNearMissRow{}
			for _, e := range matched {
				switch {
				case licenceFuelMatch(e.Fuel, flagFuel):
					kept = append(kept, e)
				case licenceFuelNearMiss(e.Fuel, flagFuel):
					row, ok := near[e.Fuel]
					if !ok {
						row = &licenceFuelNearMissRow{Fuel: e.Fuel}
						near[e.Fuel] = row
					}
					row.Entities++
					if e.GrossCapacityMW != nil {
						mw := row.MWOrZero() + *e.GrossCapacityMW
						row.MW = &mw
					}
				}
			}
			matched = kept
			meta.Fuel = flagFuel
			for _, row := range near {
				meta.FuelNearMisses = append(meta.FuelNearMisses, *row)
				meta.FuelNearMissEntities += row.Entities
			}
			sort.Slice(meta.FuelNearMisses, func(i, j int) bool {
				return meta.FuelNearMisses[i].Fuel < meta.FuelNearMisses[j].Fuel
			})
			meta.FuelNearMissMW = licenceNearMissMW(meta.FuelNearMisses)
			meta.FuelVocabulary = licenceFuelVocabulary(entities)
		}
		meta.Matched = len(matched)

		meta.UnitsPublished = map[string]int{}
		for _, e := range matched {
			switch {
			case e.GrossCapacityMW != nil:
				meta.CapacityParsed++
			case e.CapacityValue != nil:
				// A real published figure in a unit that does not
				// convert to MW. Counting it as unreadable would report
				// a measured value as missing.
				meta.CapacityNotConvertible++
			case e.GrossCapacityRaw == "":
				meta.CapacityAbsent++
			default:
				meta.CapacityUnreadable++
			}
			if e.CapacityUnit != "" {
				meta.UnitsPublished[e.CapacityUnit]++
			}
			if e.Transposed {
				meta.TransposedRows++
			}
		}
		meta.CapacityScope = fmt.Sprintf(
			"the four capacity populations describe the %d MATCHED rows in results[], not the %d entities "+
				"read from the register; they sum to matched exactly (%d + %d + %d + %d = %d). entities and "+
				"distinct_names are register-wide on purpose: they are facts about the SOURCE.",
			meta.Matched, meta.Entities,
			meta.CapacityParsed, meta.CapacityNotConvertible, meta.CapacityUnreadable, meta.CapacityAbsent,
			meta.CapacityParsed+meta.CapacityNotConvertible+meta.CapacityUnreadable+meta.CapacityAbsent)

		meta.NoCapacityTotal = "NEPRA publishes no capacity sum for this register and this command emits none. " +
			"Summing gross_capacity_mw yourself gives a PARTIAL total, and the four population counts above " +
			"are what make it readable — they partition the matched set exactly. THREE THINGS KEEP IT " +
			"PARTIAL. (1) The register publishes FOUR units in this one cell and only three convert: MW, MWe " +
			"(the same quantity) and kW (at 1000:1). MWp is a solar PEAK DC nameplate rating, a different " +
			"physical quantity from an AC megawatt with no NEPRA-published conversion, so those entities " +
			"carry capacity_value and capacity_unit but NO MW figure. (2) Some cells carry a unit but no " +
			"readable number — \"12:00 MW\", \"02:00 MW\", \"3s 6 MW\", \"110 MW Gross ISO\", " +
			"\"8.5 MW (based on alternators coupled with S.Ts)\" — and taking the leading digits would " +
			"invent a figure: the \"3s 6 MW\" entity's own plant-detail row reads 1x16MW + 1x20MW, i.e. " +
			"36 MW, so that guess is out by 12x. (3) One entity has its key/value pairs transposed upstream, " +
			"so its capacity cell publishes a technology string while its plant-type cell publishes the " +
			"megawatts; it is flagged and nothing is moved. Read entities_with_unreadable_capacity for the " +
			"count in THIS result rather than any number quoted in prose."
		meta.Refusals = []string{
			"No headline gross-capacity total. See no_capacity_total.",
			"MWp figures are NEVER folded into the MW total: a solar peak DC rating is a different quantity from an AC megawatt and NEPRA publishes no conversion between them.",
			"--ticker is refused: the crosswalk resolves only 28 of 335 register names. See the command's Long help and `doctor --scope`.",
			"Generation Concurrences.php is excluded: same accordion CSS, date-keyed schema.",
			"A transposed entity is FLAGGED, never repaired: its megawatts are published under the wrong key and moving them would invent an attribution.",
		}
		if matched == nil {
			matched = make([]licenceEntity, 0)
		}
		report := licenceReport{Meta: meta, Results: matched}
		if err := printJSONFiltered(c.OutOrStdout(), report, flags); err != nil {
			return err
		}
		// Warn and gate AFTER the payload is written, not before: the caller
		// still gets the rows and meta.surfaces_short naming exactly which
		// pages fell short, and the exit code still refuses under --strict.
		// Returning early would withhold the evidence for the refusal.
		for _, sfShort := range shortSurfaces {
			fmt.Fprintf(c.ErrOrStderr(),
				"COMPLETENESS: %s. This is how a silently truncated or empty 200 presents; "+
					"do not treat the register as complete.\n", sfShort)
		}
		if flagStrict && len(shortSurfaces) > 0 {
			return fmt.Errorf("%d of %d surfaces returned fewer entities than their measured floor",
				len(shortSurfaces), len(surfaces))
		}
		return nil
	}

	// The five flags are declared in internal/cli/promoted_licence.go, on the
	// command itself, so that verify-skill's flag-to-command attribution is
	// true; re-declaring them here would panic on a duplicate flag. Their
	// values are read through cmd.Flags(), which is the same registry those
	// declarations bind to.
}

// licenceContentKey builds the stable content string an artifact hashes.
//
// It is over the EXTRACTED entity identities and figures, not the markup, so
// the hash is comparable across fetches: these pages carry Cloudflare's
// rotating email-obfuscation tokens like every other page on this host.
func licenceContentKey(entities []licenceEntity) string {
	var b strings.Builder
	for _, e := range entities {
		b.WriteString(e.Name)
		b.WriteByte('|')
		b.WriteString(e.LicenceNo)
		b.WriteByte('|')
		b.WriteString(e.GrossCapacityRaw)
		b.WriteByte('\n')
	}
	return b.String()
}

func licencePrintCatalogue(cmd *cobra.Command, flags *rootFlags) error {
	type entry struct {
		Surface      string `json:"surface"`
		Label        string `json:"label"`
		Path         string `json:"path"`
		EntitiesAsOf int    `json:"entities_as_of"`
		BytesAsOf    int    `json:"bytes_as_of"`
		InProbeScope bool   `json:"in_probe_scope"`
		Note         string `json:"note,omitempty"`
	}
	type excluded struct {
		Surface string `json:"surface"`
		Path    string `json:"path"`
		Reason  string `json:"reason"`
	}
	rows := make([]entry, 0, len(licenceSurfaces))
	for _, s := range licenceSurfaces {
		rows = append(rows, entry{s.ID, s.Label, s.Path, s.EntitiesAsOf, s.BytesAsOf, s.InProbeScope, s.Note})
	}
	ex := make([]excluded, 0, len(licenceExcludedSurfaces))
	for _, s := range licenceExcludedSurfaces {
		ex = append(ex, excluded{s.ID, s.Path, s.Note})
	}
	payload := map[string]any{
		"meta": map[string]any{
			"source":                  "catalogue",
			"requests":                0,
			"as_of_date":              licenceAsOfDate,
			"surfaces":                len(licenceSurfaces),
			"entities_as_of":          licenceEntitiesFloor(false),
			"entities_in_probe_scope": licenceEntitiesFloor(true),
			"scope_note": "the research probe scoped \"the register\" at 335 entities over 18 pages. Isolated " +
				"Generation Companies (9) and Distributed Generation (35) carry the IDENTICAL key/value schema and " +
				"were simply outside that scope, so 379 is the measured total and 335 is a scoping decision. " +
				"Both are reported; neither is called \"the register\" on its own.",
			"excluded": ex,
		},
		"results": rows,
	}
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "%d register pages, %d licensees measured on %s (%d inside the probe's 335-entity scope). No request made.\n\n",
			len(licenceSurfaces), licenceEntitiesFloor(false), licenceAsOfDate, licenceEntitiesFloor(true))
		for _, s := range licenceSurfaces {
			scope := "     "
			if !s.InProbeScope {
				scope = "[new]"
			}
			fmt.Fprintf(out, "  %s %-24s %4d entities  %8d B  %s\n", scope, s.ID, s.EntitiesAsOf, s.BytesAsOf, s.Label)
		}
		fmt.Fprintf(out, "\nexcluded:\n")
		for _, s := range licenceExcludedSurfaces {
			fmt.Fprintf(out, "  %-24s %s\n", s.ID, s.Note)
		}
		fmt.Fprintf(out, "\nPass --search, --fuel, --surface <id> or --all to read rows.\n")
		return nil
	}
	return printJSONFiltered(cmd.OutOrStdout(), payload, flags)
}

// licenceFetch is one worker's result. It carries its own error so a failure
// travels with the page that failed rather than aborting the whole read.
type licenceFetch struct {
	entities []licenceEntity
	artifact nepraArtifact
	ok       bool
	err      error
}

// licenceAssemble reassembles the concurrent results into catalogue order.
//
// IT IS A SEPARATE FUNCTION SO THE ORDERING AND THE FAILURE ACCOUNTING ARE
// TESTABLE. Both were previously asserted only by a comment, and a mutation
// that reversed the result index survived the whole test suite: nothing
// checked that entity order follows the CATALOGUE rather than whichever page
// answered first. An order that varied per run would make two
// otherwise-identical extracts diff against each other for no reason.
//
// A page that failed is NAMED and counted, never silently dropped — a short
// register must not be indistinguishable from a small one.
func licenceAssemble(surfaces []licenceSurface, results []licenceFetch) ([]licenceEntity, []nepraArtifact, []string) {
	var (
		entities  []licenceEntity
		artifacts []nepraArtifact
		failed    []string
	)
	for i := range surfaces {
		if i >= len(results) {
			failed = append(failed, fmt.Sprintf("%s: no result recorded", surfaces[i].ID))
			continue
		}
		r := results[i]
		if !r.ok {
			reason := "no result recorded"
			if r.err != nil {
				reason = r.err.Error()
			}
			failed = append(failed, fmt.Sprintf("%s: %s", surfaces[i].ID, reason))
			continue
		}
		entities = append(entities, r.entities...)
		artifacts = append(artifacts, r.artifact)
	}
	return entities, artifacts, failed
}

// licenceShortSurfaces names every page that answered SUCCESSFULLY but came
// back with fewer entities than the count measured on licenceAsOfDate.
//
// WHY THIS IS SEPARATE FROM licenceAssemble'S FAILURE LIST. A page that fails
// to fetch is already named there. This is the other, quieter case, and it is
// the one this CLI exists to refuse: NEPRA answers HTTP 200 with a body that
// parses cleanly and yields nothing, or yields half the register. The parse
// succeeds, the surface is counted as read, and the run reports `source:
// live` over a short register with no error anywhere. That is precisely the
// shape the gzip defect produced across all seven HTML commands — see
// .printing-press-patches/nepra-decompress-content-encoding.json, where every
// one of them returned results:{} under an HTTP 200 with truthful live
// provenance. A measured shortfall must never be indistinguishable from a
// genuinely small register.
//
// The measured count is a FLOOR, not an equality: the register grows, so
// MORE entities than measured is normal and is not reported. A page carrying
// no measured floor likewise asserts nothing and is never reported short —
// that falls out of the comparison itself, since a parsed count is never
// negative and so can never fall below a floor of zero. An explicit
// EntitiesAsOf <= 0 guard was written here first and then removed: it could
// not change the outcome for any input, and the test written to cover it
// could not fail. Do not re-add it; use `!=` here and the floor becomes an
// equality, which is the mutation the tests below actually pin.
func licenceShortSurfaces(surfaces []licenceSurface, results []licenceFetch) []string {
	var short []string
	for i := range surfaces {
		// Missing results and failed fetches are licenceAssemble's to name;
		// reporting them here too would double-count one page.
		if i >= len(results) || !results[i].ok {
			continue
		}
		if got := len(results[i].entities); got < surfaces[i].EntitiesAsOf {
			short = append(short, fmt.Sprintf("%s: %d entities, below the %d measured on %s",
				surfaces[i].ID, got, surfaces[i].EntitiesAsOf, licenceAsOfDate))
		}
	}
	return short
}

// licenceFetchAll reads the given surfaces concurrently, bounded, and returns
// one result PER SURFACE AT THE SURFACE'S OWN INDEX.
//
// The index discipline is the whole contract and it is why this is a separate
// function taking an injectable fetcher: results are written to results[i]
// for surfaces[i], never appended as replies arrive, so both the row order
// and the failure attribution are properties of the catalogue rather than of
// the race. A mutation that wrote to results[len-1-i] survived an earlier
// test pass precisely because nothing could exercise this loop without a
// network, and under it surface i's ID would be reported alongside a
// different surface's error.
func licenceFetchAll(surfaces []licenceSurface, concurrency int, fetch func(licenceSurface) licenceFetch) []licenceFetch {
	if concurrency < 1 {
		concurrency = 1
	}
	results := make([]licenceFetch, len(surfaces))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i, sf := range surfaces {
		wg.Add(1)
		go func(i int, sf licenceSurface) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = fetch(sf)
		}(i, sf)
	}
	wg.Wait()
	return results
}

// licenceFuelNearMissRow is one published fuel cell that contains the queried
// term without carrying it as one of its own tokens, quantified.
type licenceFuelNearMissRow struct {
	Fuel     string `json:"fuel"`
	Entities int    `json:"entities"`
	// MW is nil when no entity in this group publishes an MW-convertible
	// capacity: an UNMEASURED group, not a zero-megawatt one.
	MW *float64 `json:"mw"`
}

// MWOrZero is an accumulator helper only, deliberately not named as a getter:
// a caller reading MW must see nil for an unmeasured group.
func (r licenceFuelNearMissRow) MWOrZero() float64 {
	if r.MW == nil {
		return 0
	}
	return *r.MW
}

// licenceNearMissMW totals the groups, returning nil when not one of them
// published a convertible capacity.
func licenceNearMissMW(rows []licenceFuelNearMissRow) *float64 {
	total := 0.0
	measured := false
	for _, r := range rows {
		if r.MW != nil {
			total += *r.MW
			measured = true
		}
	}
	if !measured {
		return nil
	}
	return &total
}
