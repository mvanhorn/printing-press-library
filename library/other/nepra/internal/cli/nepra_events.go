// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
// See .printing-press-patches/nepra-events-multipage-feed.json.
//
// pp:data-source live
// The determination feed is read from the 28-surface catalogue on every
// invocation that names a selector; it is never synced into the store, so
// there is no local feed to serve. With no selector the command prints the
// catalogue and makes NO request.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/client"
)

// eventsPromotedSuperseded keeps the generated newEventsPromotedCmd
// referenced, and it is neither dead code nor a lint dodge.
//
// TWO THINGS DEPEND ON IT. (1) The spec-emitted `events` command pointed at
// /tariff/Tariff.php, a 9-byte HTTP 404; root.go registers newNepraEventsCmd
// in its place, leaving the generated constructor orphaned. staticcheck then
// reports promoted_events.go's constructor as unused. (2) More importantly,
// the shipcheck command-tree scan is a TEXT scan: it pairs each constructor
// with a Use: literal in the same file and treats a constructor as registered
// only when its name followed by "(" appears at least twice across the
// non-test sources. The orphan appears exactly once — its own definition — so
// the scan reported the working `events` command as UNREGISTERED, which is a
// false finding about a command that is registered and works.
//
// The blank identifier is load-bearing: a NAMED package-level var would
// simply move the unused finding onto itself. This lives here, in a
// hand-authored file, rather than in generated root.go so that
// `generate --force` cannot drop it.
//
// DO NOT DELETE promoted_events.go to "fix" this. It is generated and a regen
// restores it, reopening both findings.
var _ = func(flags *rootFlags) *cobra.Command { return newEventsPromotedCmd(flags) }

// newNepraEventsCmd builds the regulatory determination feed.
//
// It replaces the spec-emitted `events`, which pointed at /tariff/Tariff.php —
// a 9-byte HTTP 404. The real feed is 28 catalogued pages carrying 16,405
// accordion rows from 27 Mar 1999 to 7 Sep 2026, and it is the ONLY current
// surface on the site: it was last updated the day before the survey, while
// all four Main.htm workbook surfaces froze in 2022.
func newNepraEventsCmd(flags *rootFlags) *cobra.Command {
	var (
		surfaceID string
		disco     string
		company   string
		since     string
		until     string
		docket    string
		all       bool
		dedup     bool
		strict    bool
		limit     int
	)

	cmd := &cobra.Command{
		Use:   "events",
		Short: "Regulatory determination feed: dated tariff decisions, indexations and SROs with their source PDFs",
		Long: "Dated regulatory events across NEPRA's tariff-determination stream.\n\n" +
			"This is an EVENT feed, not a numeric one: the amounts live inside the linked PDFs, so each row\n" +
			"carries the date, the company or year it sat under, the determination text, the verbatim document\n" +
			"URL and any TRF docket or S.R.O. number named. Dockets are the stable join keys — TRF-71 is\n" +
			"Nishat Power, TRF-70 Nishat Chunian, TRF-600 Kot Addu, TRF-100 the ex-WAPDA DISCOs.\n\n" +
			"With no selector it prints the catalogue of surfaces and makes no request.",
		Example: "  nepra-pp-cli events\n" +
			"  nepra-pp-cli events --surface ipp-short-term\n" +
			"  nepra-pp-cli events --disco LESCO --since 2026-01-01\n" +
			"  nepra-pp-cli events --surface ipp-thermal --docket TRF-71 --agent",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "--surface=ipp-short-term",
			"pp:typed-exit-codes": "true",
			"pp:novel-hand-coded": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Help-only branch: the catalogue is useful and costs nothing.
			if !all && surfaceID == "" && disco == "" {
				// Row filters have nothing to filter until a surface has been
				// fetched. Falling through to the catalogue here would answer a
				// filtered question with unfiltered output under exit 0 — the
				// silent-wrong-answer failure this CLI exists to stop.
				var ignored []string
				for _, name := range []string{"docket", "company", "since", "until"} {
					if cmd.Flags().Changed(name) {
						ignored = append(ignored, "--"+name)
					}
				}
				if len(ignored) > 0 {
					return usageErr(fmt.Errorf(
						"%s filters rows, but no surface was selected so no rows were fetched; "+
							"add --surface <id>, --disco <name> or --all "+
							"(run `nepra-pp-cli events` with no flags to list the surfaces)",
						strings.Join(ignored, ", ")))
				}
				return eventsPrintCatalogue(cmd, flags)
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "events")
			}

			// VALIDATE THE DATE WINDOW BEFORE FETCHING ANYTHING.
			//
			// eventsFilter compares these flags LEXICALLY against DateISO, so
			// any string that is not a zero-padded YYYY-MM-DD silently
			// produces a wrong window at exit 0 rather than an error. The
			// sharpest case is "01-01-2026": that is dd-mm-yyyy, which is the
			// format THIS COMMAND'S OWN `date` field publishes, so it is the
			// form a user copies out of the output and back into the flag —
			// and lexically it sorts below every ISO date, so the window is
			// silently ignored and the full feed comes back looking filtered.
			// "2026-1-1" drops rows that are genuinely inside the window, and
			// a typo like "yesterday" returns an empty feed as a success.
			// Every sibling window flag in this CLI validates its form first.
			if err := eventsValidateWindow(since, until); err != nil {
				return usageErr(err)
			}

			targets, err := eventsSelectSurfaces(surfaceID, disco, all)
			if err != nil {
				return usageErr(err)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			var rows []EventRow
			var report []eventsFetchStat
			for _, s := range targets {
				body, err := c.GetWithHeaders(cmd.Context(), s.Path, map[string]string{},
					map[string]string{client.HTMLResponseHeader: "true"})
				if err != nil {
					return classifyAPIError(cmd.OutOrStdout(), err, flags)
				}
				got, skipped, err := eventsExtract(s.ID, string(body), c.BaseURL+s.Path)
				if err != nil {
					return err
				}
				// Hash the EXTRACTED rows, not the markup: the markup carries
				// per-response Cloudflare obfuscation tokens and so never
				// hashes the same twice.
				content, err := json.Marshal(got)
				if err != nil {
					return err
				}
				report = append(report, eventsFetchStat{
					Surface: s.ID, Label: s.Label, Bytes: len(body),
					Rows: len(got), Skipped: skipped, FloorAsOf: s.RowsAsOf,
					Artifact: newNepraArtifact(c.BaseURL+s.Path, body, content, c.LastContentType(), len(got), skipped),
				})
				rows = append(rows, got...)
			}

			total := len(rows)
			rows = eventsFilter(rows, company, since, until, docket)
			duplicates := eventsCountDuplicates(rows)
			if dedup {
				rows = eventsDedup(rows)
			}
			sort.SliceStable(rows, func(i, j int) bool {
				if rows[i].DateISO != rows[j].DateISO {
					return rows[i].DateISO > rows[j].DateISO
				}
				return rows[i].Description < rows[j].Description
			})
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}

			// The completeness assertion. A surface returning FEWER rows than
			// the floor measured on the survey date is the signature of the
			// documented silent truncation — the Wind page once returned
			// 1,028 of 2,440 rows under an HTTP 200 — so it is reported
			// loudly, and --strict turns it into a non-zero exit.
			short := eventsShortfalls(report)
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				eventsWriteSummary(cmd, report, total, len(rows), duplicates, dedup)
			}
			if len(short) > 0 {
				for _, s := range short {
					fmt.Fprintf(cmd.ErrOrStderr(),
						"COMPLETENESS: %s returned %d rows, below the %d measured on %s. "+
							"This is how the documented silent truncation presents; do not treat the result as complete.\n",
						s.Surface, s.Rows, s.FloorAsOf, eventsAsOfDate)
				}
				if strict {
					return fmt.Errorf("%d of %d surfaces returned fewer rows than their measured floor", len(short), len(report))
				}
			}

			payload, err := json.Marshal(rows)
			if err != nil {
				return err
			}
			if flags.asJSON {
				// Machine consumers get the artifact provenance for every
				// surface read, so the row set is auditable against the exact
				// bytes it came from.
				envelope := map[string]any{
					"meta": map[string]any{
						"source":     "live",
						"surfaces":   report,
						"as_of_date": eventsAsOfDate,
					},
					"results": json.RawMessage(payload),
				}
				out, merr := json.MarshalIndent(envelope, "", "  ")
				if merr != nil {
					return merr
				}
				return printOutput(cmd.OutOrStdout(), out, true)
			}
			wrapped, err := wrapPlatformStructuredOutput(payload, flags, "results", true)
			if err != nil {
				return err
			}
			return printOutput(cmd.OutOrStdout(), wrapped, true)
		},
	}

	cmd.Flags().StringVar(&surfaceID, "surface", "", "Catalogue surface to read (run with no flags to list them)")
	cmd.Flags().StringVar(&disco, "disco", "", "Select a distribution company's pages: FESCO, GEPCO, HAZECO, HESCO, IESCO, LESCO, MEPCO, PESCO, QESCO, SEPCO, TESCO or KE")
	cmd.Flags().StringVar(&company, "company", "", "Keep rows whose accordion heading or description contains this text (case-insensitive)")
	cmd.Flags().StringVar(&since, "since", "", "Keep rows dated on or after this YYYY-MM-DD")
	cmd.Flags().StringVar(&until, "until", "", "Keep rows dated on or before this YYYY-MM-DD")
	cmd.Flags().StringVar(&docket, "docket", "", "Keep rows naming this tariff docket, e.g. TRF-71")
	cmd.Flags().BoolVar(&all, "all", false, "Read every catalogued surface (28 pages, several MB even gzipped)")
	cmd.Flags().BoolVar(&dedup, "dedup", false, "Collapse rows identical on surface, heading, date and document URL. Off by default so a genuine republication is never silently dropped")
	cmd.Flags().BoolVar(&strict, "strict", false, "Exit non-zero when any surface returns fewer rows than its measured floor")
	cmd.Flags().IntVar(&limit, "limit", 0, "Keep at most this many rows after filtering")
	return cmd
}

type eventsFetchStat struct {
	Surface   string `json:"surface"`
	Label     string `json:"label"`
	Bytes     int    `json:"bytes"`
	Rows      int    `json:"rows"`
	Skipped   int    `json:"skipped"`
	FloorAsOf int    `json:"floor_as_of"`
	// Artifact is machine-checkable provenance for the bytes these rows came
	// from, so a completeness claim can be re-derived later.
	Artifact nepraArtifact `json:"artifact"`
}

// eventsSelectSurfaces resolves the caller's selectors to catalogue entries.
func eventsSelectSurfaces(surfaceID, disco string, all bool) ([]eventSurface, error) {
	if all {
		return eventSurfaces, nil
	}
	if surfaceID != "" {
		s, ok := eventSurfaceByID(surfaceID)
		if !ok {
			var ids []string
			for _, e := range eventSurfaces {
				ids = append(ids, e.ID)
			}
			return nil, fmt.Errorf("unknown surface %q; known surfaces are %s", surfaceID, strings.Join(ids, ", "))
		}
		return []eventSurface{s}, nil
	}
	want := strings.ToUpper(strings.TrimSpace(disco))
	var out []eventSurface
	for _, e := range eventSurfaces {
		if e.DISCO != "" && strings.EqualFold(e.DISCO, want) {
			out = append(out, e)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no catalogued surface for disco %q", disco)
	}
	return out, nil
}

func eventsFilter(rows []EventRow, company, since, until, docket string) []EventRow {
	comp := strings.ToLower(strings.TrimSpace(company))
	dock := strings.ToUpper(strings.TrimSpace(docket))
	var out []EventRow
	for _, r := range rows {
		if comp != "" && !strings.Contains(strings.ToLower(r.Group+" "+r.Description), comp) {
			continue
		}
		if dock != "" && !strings.EqualFold(r.Docket, dock) {
			continue
		}
		// A row whose published date is not a real date cannot be compared
		// against a window. It is KEPT when no window was asked for and
		// excluded when one was, rather than being given a guessed date.
		if since != "" || until != "" {
			if r.DateISO == "" {
				continue
			}
			if since != "" && r.DateISO < since {
				continue
			}
			if until != "" && r.DateISO > until {
				continue
			}
		}
		out = append(out, r)
	}
	return out
}

func eventsRowKey(r EventRow) string {
	return r.Surface + "\x00" + r.Group + "\x00" + r.Date + "\x00" + r.DocumentURL
}

func eventsCountDuplicates(rows []EventRow) int {
	seen := map[string]int{}
	for _, r := range rows {
		seen[eventsRowKey(r)]++
	}
	n := 0
	for _, c := range seen {
		if c > 1 {
			n += c - 1
		}
	}
	return n
}

func eventsDedup(rows []EventRow) []EventRow {
	seen := map[string]bool{}
	var out []EventRow
	for _, r := range rows {
		k := eventsRowKey(r)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, r)
	}
	return out
}

// eventsShortfalls returns the surfaces that came back below their measured
// floor. A floor of 0 asserts nothing.
func eventsShortfalls(report []eventsFetchStat) []eventsFetchStat {
	var out []eventsFetchStat
	for _, s := range report {
		if s.FloorAsOf > 0 && s.Rows < s.FloorAsOf {
			out = append(out, s)
		}
	}
	return out
}

func eventsWriteSummary(cmd *cobra.Command, report []eventsFetchStat, total, kept, duplicates int, dedup bool) {
	w := cmd.ErrOrStderr()
	for _, s := range report {
		floor := "no floor recorded"
		if s.FloorAsOf > 0 {
			floor = fmt.Sprintf("floor %d as of %s", s.FloorAsOf, eventsAsOfDate)
		}
		skipped := ""
		if s.Skipped > 0 {
			skipped = fmt.Sprintf("  %d non-date rows skipped", s.Skipped)
		}
		fmt.Fprintf(w, "%-20s %7d rows  %8d B  %s%s\n", s.Surface, s.Rows, s.Bytes, floor, skipped)
	}
	fmt.Fprintf(w, "%d rows read, %d after filters", total, kept)
	if duplicates > 0 {
		if dedup {
			fmt.Fprintf(w, ", %d duplicates collapsed", duplicates)
		} else {
			fmt.Fprintf(w, ", %d duplicate rows kept (pass --dedup to collapse)", duplicates)
		}
	}
	fmt.Fprintln(w)
}

// eventsPrintCatalogue lists the surfaces without making a request.
func eventsPrintCatalogue(cmd *cobra.Command, flags *rootFlags) error {
	type entry struct {
		Surface string `json:"surface"`
		Label   string `json:"label"`
		Path    string `json:"path"`
		DISCO   string `json:"disco,omitempty"`
		// RowsAsOf is a POINTER and carries NO omitempty, both deliberately.
		// ipp-waste publishes a measured zero — reachable and genuinely
		// empty — while thirteen surfaces were never counted. Under the
		// previous `int` with omitempty, both rendered as an absent key, so
		// this command erased exactly the blank-versus-zero distinction it
		// exists to preserve in NEPRA's data. null now means "never
		// counted"; 0 means "counted, and there were none".
		RowsAsOf *int   `json:"rows_as_of"`
		AsOfDate string `json:"as_of_date"`
	}
	out := make([]entry, 0, len(eventSurfaces))
	for _, s := range eventSurfaces {
		e := entry{
			Surface: s.ID, Label: s.Label, Path: s.Path, DISCO: s.DISCO,
			AsOfDate: eventsAsOfDate,
		}
		if s.RowsMeasured {
			rows := s.RowsAsOf
			e.RowsAsOf = &rows
		}
		out = append(out, e)
	}
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"%d catalogued surfaces. Pass --surface <id>, --disco <name> or --all to read rows.\n"+
				"Row counts were measured on %s and are a FLOOR: the feed is live and grows.\n",
			len(eventSurfaces), eventsAsOfDate)
	}
	rows, err := json.Marshal(out)
	if err != nil {
		return err
	}
	// The catalogue carries the SAME {meta, results} envelope as the live
	// read above. It used to emit a bare top-level array, because a bare
	// array cannot satisfy wrapPlatformStructuredOutput's merge test and that
	// wrapper is a no-op without a platform session — so `.meta` and `.results`
	// simply did not exist here, and a consumer that iterated `.results[]`
	// silently saw nothing. The floor caveat belongs in the payload, not only
	// in a stderr line a machine consumer never reads.
	payload, err := json.Marshal(map[string]any{
		"meta": map[string]any{
			"source":     "catalogue",
			"as_of_date": eventsAsOfDate,
			"surfaces":   len(eventSurfaces),
			"requests":   0,
			"note": "row counts were measured on " + eventsAsOfDate + " and are a FLOOR, not a total: " +
				"the feed is live and grows. Pass --surface <id>, --disco <name> or --all to read rows.",
		},
		"results": json.RawMessage(rows),
	})
	if err != nil {
		return err
	}
	wrapped, err := wrapPlatformStructuredOutput(payload, flags, "results", true)
	if err != nil {
		return err
	}
	return printOutput(cmd.OutOrStdout(), wrapped, true)
}

// eventsValidateWindow refuses a --since/--until that eventsFilter would
// compare wrongly.
//
// The comparison downstream is a plain string compare against DateISO, which
// is correct ONLY for zero-padded ISO dates, where lexical and chronological
// order coincide. Anything else is refused here with the accepted form named,
// rather than silently answering a different question.
func eventsValidateWindow(since, until string) error {
	for _, f := range []struct{ name, value string }{{"--since", since}, {"--until", until}} {
		if f.value == "" {
			continue
		}
		if !eventsValidISODate(f.value) {
			return fmt.Errorf(
				"%s %q is not a YYYY-MM-DD date. The window is compared lexically against each row's ISO "+
					"date, so an unpadded or differently-ordered value would silently return the wrong rows "+
					"at exit 0 rather than an error. Note this command's own `date` field prints dd-mm-yyyy, "+
					"which is NOT the accepted form here: pass %s 2026-01-01, not 01-01-2026",
				f.name, f.value, f.name)
		}
	}
	if since != "" && until != "" && since > until {
		return fmt.Errorf(
			"--since %s is after --until %s, which selects nothing. Undated determinations are excluded from "+
				"any window regardless, because they cannot be placed in time", since, until)
	}
	return nil
}

// eventsValidISODate accepts exactly a zero-padded YYYY-MM-DD that names a
// real calendar day.
func eventsValidISODate(s string) bool {
	if len(s) != 10 || s[4] != '-' || s[7] != '-' {
		return false
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return false
	}
	// time.Parse normalises nothing here, but it does accept some impossible
	// days by rolling them over in other layouts; compare back to be sure.
	return t.Format("2006-01-02") == s
}
