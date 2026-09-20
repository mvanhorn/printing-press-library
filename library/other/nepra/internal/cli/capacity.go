// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. The novel scaffold's RunE is implemented; `generate --force`
// preserves implemented bodies. The func name newNovelCapacityCmd is kept so
// root.go's existing registration keeps working, and the "pp:novel-scaffold"
// annotation is DELETED so preferImplementedNovelCommands does not treat this
// as a stub.
//
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraparse"
)

// newNovelCapacityCmd answers "what was installed capacity on 30 June <year>"
// with a LABELLED answer instead of a number.
//
// It partitions every published plant row into a four-member status enum —
// active / delicensed / decommissioned / listed_no_data — reports installed
// AND dependable megawatts per bucket, and prints a reconciliation ledger in
// which the figures this run derived from the bytes it fetched are marked
// `derived` while three published national figures are marked `unavailable`
// with NO numeric field at all. A consumer therefore cannot add a figure this
// CLI never measured into a total.
func newNovelCapacityCmd(flags *rootFlags) *cobra.Command {
	var (
		asOf   string
		flagBy string
		strict bool
	)

	cmd := &cobra.Command{
		Use:   "capacity",
		Short: "Installed and dependable capacity broken out by plant status and system",
		Long: "Installed and dependable capacity as at a 30-June balance-sheet date, with every megawatt LABELLED.\n\n" +
			"All 133 published FY2023-24 plant rows are partitioned into four states: active, delicensed,\n" +
			"decommissioned, and listed_no_data — a plant publishing a capacity with NO status token and NO\n" +
			"monthly data, whose operating status is genuinely unknown and whose megawatts therefore belong to\n" +
			"neither the operating nor the non-operating total. A non-operating plant KEEPS its published\n" +
			"capacity: Kotri Power Station publishes 174 MW installed / 120 MW dependable while all 26 of its\n" +
			"monthly cells read DELICENSED.\n\n" +
			"Only a 30-June date can be honoured: the workbook carries no in-year capacity series.\n" +
			"With no --as-of the command prints the dated catalogue and makes NO request.",
		Example: "  nepra-pp-cli capacity\n" +
			"  nepra-pp-cli capacity --as-of 2024-06-30\n" +
			"  nepra-pp-cli capacity --as-of 2024-06-30 --by technology --json\n" +
			"  nepra-pp-cli capacity --as-of 2021-06-30 --by system --agent\n" +
			"  nepra-pp-cli capacity --as-of 2024-06-30 --data-source local --strict",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "--as-of=2024-06-30",
			"pp:typed-exit-codes": "true",
			"pp:novel-hand-coded": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// (a) Help-only branch. The catalogue is useful, and it costs
			// nothing: no client is built and no request is made.
			if asOf == "" && !cmd.Flags().Changed("by") && !cmd.Flags().Changed("strict") {
				return capacityPrintCatalogue(cmd, flags)
			}
			// (b) Dry run, before any validation that could mask it.
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "capacity")
			}
			// (c) Usage errors.
			by, err := capacityNormaliseBy(flagBy)
			if err != nil {
				return usageErr(err)
			}
			if err := validateDataSourceStrategy(flags, "auto"); err != nil {
				return usageErr(err)
			}
			surface, err := capacityResolveAsOf(asOf)
			if err != nil {
				return err
			}

			// (d) The offline path is a DEGRADED answer that declares itself.
			if flags.dataSource == "local" {
				if !capacityLocalCanAnswer(surface.AsOf) {
					return usageErr(fmt.Errorf(
						"--data-source local can only answer --as-of %s: the embedded crosswalk observes FY2017-18 "+
							"and FY2023-24 only. Pass --data-source live or auto for the other five reachable years",
						strings.Join(capacityLocalAsOfDates, " or ")))
				}
				return capacityEmitLocal(cmd, flags, surface, by, strict, "")
			}

			// (e) Fetch.
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := capacityFYPath(surface.FiscalYear)
			body, err := c.GetWithHeaders(cmd.Context(), path, map[string]string{},
				map[string]string{client.HTMLResponseHeader: "true"})
			if err != nil {
				// auto degrades to the embedded crosswalk on a NETWORK
				// failure only, and says so in meta. An HTTP error is a fact
				// about the document and is never papered over.
				if flags.dataSource != "live" && isNetworkError(err) && capacityLocalCanAnswer(surface.AsOf) {
					return capacityEmitLocal(cmd, flags, surface, by, strict,
						"the live sheet was unreachable ("+err.Error()+"), so this answer comes from the embedded "+
							"crosswalk and is DEGRADED: see meta.status_enum_gap")
				}
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}

			// (f) Parse with a NON-EMPTY fy. This is the only defence against
			// `SIR Data 2025.htm`, which serves FY2023-24 under an HTTP 200
			// behind a plausible 2025 filename: a non-empty fy makes the
			// parser cross-check the caller's year against the in-document
			// header band and fail with ErrFiscalYearMismatch.
			w, err := nepraparse.ParseWorkbook(body, surface.FiscalYear)
			if err != nil {
				return fmt.Errorf("parsing the FY%s workbook fetched from %s: %w", surface.FiscalYear, c.BaseURL+path, err)
			}

			// (g) Group, assert, emit.
			rep, err := capacityGroupLive(w, by)
			if err != nil {
				return err
			}
			assertions := capacityAssertLive(w, len(body), surface, rep)

			results, err := json.Marshal(rep)
			if err != nil {
				return err
			}
			meta := capacityMeta(surface, by, "live", rep, assertions)
			meta["charset"] = w.Charset
			meta["charset_declared"] = w.CharsetDeclared
			meta["band_label_read_from_file"] = w.FiscalYear.BandLabel()
			// Parser warnings travel verbatim. A warning never means a value
			// was fabricated or dropped, and dropping the warnings would hide
			// every unknown vocabulary value the file carries.
			meta["warnings"] = capacityWarnings(w.Warnings)
			meta["artifact"] = newNepraArtifact(c.BaseURL+path, body, results, c.LastContentType(), len(w.Plants), 0)

			return capacityEmit(cmd, flags, meta, results, rep, assertions, strict)
		},
	}

	cmd.Flags().StringVar(&asOf, "as-of", "",
		"Balance-sheet date for the capacity stock, as YYYY-06-30. NEPRA publishes capacity once per fiscal year "+
			"with no in-year series, so only a 30-June date can be honoured. Reachable: 2018-06-30..2024-06-30. "+
			"Empty prints the catalogue and makes no request")
	cmd.Flags().StringVar(&flagBy, "by", "status",
		"Grouping: status (the four-member plant-status enum), system (CPPA-G scope vs the K-Electric limit), "+
			"technology (the 9 verbatim published technology strings, each split by status)")
	cmd.Flags().BoolVar(&strict, "strict", false,
		"Exit non-zero when a completeness assertion fails or a measured floor is missed. Off by default so the "+
			"numbers are still printed alongside the warning")
	return cmd
}

func capacityLocalCanAnswer(asOf string) bool {
	for _, d := range capacityLocalAsOfDates {
		if d == asOf {
			return true
		}
	}
	return false
}

// capacityWarnings normalises a nil warning slice to an empty array so the
// JSON key is always present and always a list.
func capacityWarnings(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

// capacityMeta builds the shared meta block.
func capacityMeta(surface capacityAsOfSurface, by, source string, rep capacityReport, a capacityAssertions) map[string]any {
	meta := map[string]any{
		"as_of":                surface.AsOf,
		"fiscal_year":          surface.FiscalYear,
		"by":                   by,
		"source":               source,
		"status_enum":          rep.StatusEnum,
		"status_enum_complete": rep.StatusEnumComplete,
		"assertions":           a,
	}
	if rep.StatusEnumGap != nil {
		meta["status_enum_gap"] = rep.StatusEnumGap
	}
	return meta
}

// capacityEmitLocal answers from the embedded crosswalk.
func capacityEmitLocal(cmd *cobra.Command, flags *rootFlags, surface capacityAsOfSurface, by string, strict bool, degraded string) error {
	rep, err := capacityGroupLocal(surface.FiscalYear, by)
	if err != nil {
		return notFoundErr(err)
	}
	assertions := capacityAssertLocal(surface, rep)
	results, err := json.Marshal(rep)
	if err != nil {
		return err
	}
	meta := capacityMeta(surface, by, "local:embedded-crosswalk", rep, assertions)
	meta["warnings"] = capacityWarnings(nil)
	if degraded != "" {
		meta["degraded_from"] = "live"
		meta["degraded_reason"] = degraded
	}
	return capacityEmit(cmd, flags, meta, results, rep, assertions, strict)
}

// capacityEmit writes the payload, then the human summary, then escalates
// under --strict. The payload is written FIRST and is never suppressed: a
// failed assertion is a reason to distrust a number, not a reason to withhold
// it.
func capacityEmit(cmd *cobra.Command, flags *rootFlags, meta map[string]any, results json.RawMessage,
	rep capacityReport, a capacityAssertions, strict bool) error {

	human := wantsHumanTable(cmd.OutOrStdout(), flags)
	if human {
		if err := capacityWriteHuman(cmd, meta, rep); err != nil {
			return err
		}
	} else {
		envelope, err := json.Marshal(map[string]any{"meta": meta, "results": json.RawMessage(results)})
		if err != nil {
			return err
		}
		wrapped, err := wrapPlatformStructuredOutput(envelope, flags, "results", true)
		if err != nil {
			return err
		}
		if err := printOutput(cmd.OutOrStdout(), wrapped, true); err != nil {
			return err
		}
	}

	if rep.StatusEnumGap != nil {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"DEGRADED: the %q status member is NOT derivable on this path. %s\n",
			rep.StatusEnumGap.MissingMember, rep.StatusEnumGap.Reason)
	}
	for _, f := range a.Failed {
		fmt.Fprintf(cmd.ErrOrStderr(), "COMPLETENESS: %s\n", f)
	}
	if strict && len(a.Failed) > 0 {
		return fmt.Errorf("%d completeness assertion(s) failed; the payload above was still written", len(a.Failed))
	}
	return nil
}

// capacityWriteHuman renders the bucket table and the reconciliation ledger.
//
// The table is written with a tabwriter rather than printAutoTable because
// printAutoTable derives its column order from Go MAP ITERATION ORDER for
// every field in the same priority tier (helpers.go prioritizeFields), so
// the same data prints its columns in a different order on different runs.
// A capacity report has to be diffable between runs, so the order is fixed
// here.
func capacityWriteHuman(cmd *cobra.Command, meta map[string]any, rep capacityReport) error {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Capacity as at %v (FY%v), by %v — source %v\n\n",
		meta["as_of"], meta["fiscal_year"], meta["by"], meta["source"])

	tw := newTabWriter(out)
	fmt.Fprintln(tw, "BUCKET\tPLANTS\tINSTALLED MW (n)\tDEPENDABLE MW (n)\tOPERATING\tNO CAPACITY NUMBER")
	for _, b := range rep.Buckets {
		capacityWriteBucketRow(tw, b, "")
		for _, n := range b.ByStatus {
			// A nested member with no plants in it is a true zero-row fact,
			// but printing nine empty sub-rows per technology buries the
			// answer. The JSON keeps every one of them.
			if n.Plants != nil && *n.Plants == 0 {
				continue
			}
			capacityWriteBucketRow(tw, n, "  - ")
		}
	}
	capacityWriteBucketRow(tw, capacityBucket{
		Key:                "TOTAL published",
		Plants:             capacityIntPtr(rep.Totals.PlantRows),
		Installed:          &rep.Totals.Installed,
		Dependable:         rep.Totals.Dependable,
		capacityCellCounts: &rep.Totals.capacityCellCounts,
	}, "")
	if err := tw.Flush(); err != nil {
		return err
	}

	fmt.Fprintf(out, "\nRECONCILIATION\n")
	rw := newTabWriter(out)
	fmt.Fprintln(rw, "LABEL\tSTATE\tINSTALLED MW\tDEPENDABLE MW\tPLANTS\tSOURCE")
	for _, r := range rep.Reconciliation {
		inst, dep, plants := "<unavailable>", "<unavailable>", "<unavailable>"
		if r.InstalledMW != nil {
			inst = fmt.Sprintf("%.2f", *r.InstalledMW)
		} else if r.PublishedAs != "" {
			inst = "published as " + r.PublishedAs
		}
		if r.DependableMW != nil {
			dep = fmt.Sprintf("%.2f", *r.DependableMW)
		}
		if r.Plants != nil {
			plants = fmt.Sprintf("%d", *r.Plants)
		}
		fmt.Fprintf(rw, "%s\t%s\t%s\t%s\t%s\t%s\n", r.Label, r.State, inst, dep, plants, r.Source)
	}
	if err := rw.Flush(); err != nil {
		return err
	}
	fmt.Fprint(out, "\nThe `unavailable` rows carry NEPRA's published string and NO number: this CLI cannot re-derive\n"+
		"them, so they must never be added into a total. Pass --json for the full reasons.\n")
	return nil
}

func capacityWriteBucketRow(w io.Writer, b capacityBucket, indent string) {
	// "" means the row is not a status member so the question does not
	// apply; "unknown" means it IS one and the source does not say.
	plants, operating, noNumber := "<absent>", "", "<absent>"
	if b.Plants != nil {
		plants = fmt.Sprintf("%d", *b.Plants)
	}
	if b.OperatingKnown != nil {
		operating = "unknown"
		if *b.OperatingKnown && b.Operating != nil {
			operating = "no"
			if *b.Operating {
				operating = "yes"
			}
		}
	}
	if b.capacityCellCounts != nil {
		noNumber = fmt.Sprintf("%d blank / %d status / %d unmodelled",
			b.NotReported, b.StatusCell, b.UnknownText)
	}
	key := indent + b.Key
	if b.State != "" {
		key += " (" + b.State + ")"
	}
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
		key, plants, capacityMWCell(b.Installed), capacityMWCell(b.Dependable), operating, noNumber)
}

// capacityMWCell renders a sum without ever printing a zero for an
// unmeasured one, and distinguishes an unmeasured sum from an absent column.
func capacityMWCell(c *capacityMW) string {
	if c == nil {
		return "<no such column>"
	}
	n, ok := c.Float64()
	if !ok {
		return "<not measured>"
	}
	return fmt.Sprintf("%.2f (%d)", n, c.Plants())
}

// capacityPrintCatalogue lists every dated surface and makes NO request.
func capacityPrintCatalogue(cmd *cobra.Command, flags *rootFlags) error {
	entries := make([]map[string]any, 0, len(capacityAsOfCatalogue))
	reachable := 0
	for _, s := range capacityAsOfCatalogue {
		e := map[string]any{
			"as_of":           s.AsOf,
			"fiscal_year":     s.FiscalYear,
			"state":           s.State,
			"floors_measured": s.FloorsMeasured,
			"offline":         s.Offline,
		}
		// A floor of 0 would assert something. A year with no fixture
		// carries NEITHER floor key.
		if s.FloorsMeasured {
			if s.PlantRowFloor > 0 {
				e["plant_row_floor"] = s.PlantRowFloor
			}
			if s.DecodedByteFloor > 0 {
				e["decoded_byte_floor"] = s.DecodedByteFloor
			}
		}
		if s.Reason != "" {
			e["reason"] = s.Reason
		}
		if s.State == "reachable" {
			reachable++
		}
		entries = append(entries, e)
	}

	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"%d dated surfaces, %d reachable. Pass --as-of YYYY-06-30 to read one.\n"+
				"Only a 30-June date can be honoured: the workbook publishes capacity once per fiscal year and\n"+
				"carries no in-year series. Floors are MEASURED lower bounds and exist for the %d years with a\n"+
				"committed fixture; the others carry no floor rather than a zero.\n",
			len(capacityAsOfCatalogue), reachable, capacityMeasuredFloors())
	}

	payload, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	envelope, err := json.Marshal(map[string]any{
		"meta": map[string]any{
			"source":       "catalogue",
			"request_made": false,
		},
		"results": json.RawMessage(payload),
	})
	if err != nil {
		return err
	}
	wrapped, err := wrapPlatformStructuredOutput(envelope, flags, "results", true)
	if err != nil {
		return err
	}
	return printOutput(cmd.OutOrStdout(), wrapped, true)
}

func capacityMeasuredFloors() int {
	n := 0
	for _, s := range capacityAsOfCatalogue {
		if s.FloorsMeasured {
			n++
		}
	}
	return n
}
