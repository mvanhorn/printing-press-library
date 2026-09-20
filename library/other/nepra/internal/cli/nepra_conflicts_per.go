// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
//
// The PER derivation leg: re-derive same-key conflicts and adjacent-period
// breaks from a Performance Evaluation Report PDF.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraper"
)

// conflictsFetchPER downloads one PER and returns the PDF bytes with their
// provenance.
//
// It deliberately does NOT route through the `reliability` command or
// resourceDetailPaths["reliability"]. That template is hard-coded to
// /Standards/{path}, and the FY2022-23 and FY2024-25 reports live under
// /M&E/PER/Distribution/ — so the flagship FY2024-25 conflict is literally
// unreachable through it. nepraper's source table holds the real path for each
// year, including FY2020-21's trailing encoded space, which is load-bearing:
// the same URL without it returns 404.
//
// The client concatenates BaseURL and path (it does not re-escape), so an
// already-encoded NEPRA path passes through intact.
func conflictsFetchPER(ctx context.Context, c *client.Client, fy string) (raw []byte, url, contentType string, err error) {
	src, ok := nepraper.SourceFor(fy)
	if !ok {
		return nil, "", "", notFoundErr(fmt.Errorf(
			"no source record for %s; this build's PER table holds %s",
			nepraper.NormalizeFY(fy), strings.Join(conflictsPERYears(), ", ")))
	}
	if src.Availability != nepraper.AvailabilityPublished {
		// Refuse rather than fetch-and-report-empty. AvailabilityFor carries
		// NEPRA's own wording plus the URL that 404s.
		return nil, src.URL(), "", notFoundErr(fmt.Errorf(
			"%s cannot be obtained: %s", src.FY, nepraper.AvailabilityFor(src.FY).Reason))
	}

	body, err := c.GetWithHeaders(ctx, src.URLPath, nil,
		map[string]string{client.BinaryResponseHeader: "true"})
	if err != nil {
		return nil, src.URL(), "", err
	}
	decoded, ct, ok := client.UnwrapBinaryResponse(body)
	if !ok {
		return nil, src.URL(), "", fmt.Errorf(
			"%s: response was not the base64 binary envelope this build asks for (%d bytes); "+
				"the PDF cannot be read and no figure is derived from a partial body", src.FY, len(body))
	}
	if ct == "" {
		ct = c.LastContentType()
	}
	return decoded, src.URL(), ct, nil
}

// conflictsPERYears lists the fiscal years this build has a PER record for,
// published or not.
func conflictsPERYears() []string {
	src := nepraper.Sources()
	out := make([]string, 0, len(src))
	for _, s := range src {
		label := s.FY
		if s.Availability != nepraper.AvailabilityPublished {
			label += " (" + s.Availability.String() + ")"
		}
		out = append(out, label)
	}
	return out
}

// conflictsPERReport extracts and parses one PER.
//
// The caller's fiscal year is never overridden by the document's: a
// disagreement between the two is recorded in Report.DetectedFY and as a Note,
// and surfaced by the recompute block.
func conflictsPERReport(raw []byte, fy string) (*nepraper.Report, error) {
	doc, err := nepraper.ExtractText(raw)
	if err != nil {
		return nil, fmt.Errorf("extracting %s text layer: %w", nepraper.NormalizeFY(fy), err)
	}
	return nepraper.ParseReliability(doc, fy)
}

// perConflictEntries turns nepraper's own conflicts into ledger entries.
//
// nepraper.Conflicts is the whole engine here; this function only projects it.
// It does NOT marshal nepraper.Conflict, Key or Observation directly: none of
// them carries json tags, so they would serialise in Go field case and leak
// the entire Observation including Extras.
func perConflictEntries(r *nepraper.Report, reports ...*nepraper.Report) []conflictEntry {
	var cs []nepraper.Conflict
	if len(reports) > 0 {
		cs = nepraper.ConflictsAcross(append([]*nepraper.Report{r}, reports...)...)
	} else {
		cs = nepraper.Conflicts(r)
	}
	out := make([]conflictEntry, 0, len(cs))
	for _, c := range cs {
		key := conflictKey{
			PeriodFY: c.Key.PeriodFY,
			Entity:   string(c.Key.Entity),
			Metric:   string(c.Key.Metric),
		}
		aVal, bVal := perConflictFieldValues(c)
		a := perSide(c.A, aVal, c.A.Prov.Variant)
		b := perSide(c.B, bVal, c.B.Prov.Variant)
		disc := perDiscriminatorFor(c)
		var ratio *float64
		if v, ok := c.RatioValue(); ok {
			ratio = fptr(v)
		}
		var absDiff *float64
		if a.Value != nil && b.Value != nil {
			absDiff = fptr(c.AbsDiff)
		}
		field := c.Field
		if field == "" {
			field = "value"
		}
		out = append(out, conflictEntry{
			ID:                    conflictEntryID(conflictSurfacePER, conflictKindConflict, key, disc),
			Surface:               conflictSurfacePER,
			Kind:                  conflictKindConflict,
			Class:                 c.Class.String(),
			SameKey:               true,
			Key:                   key,
			A:                     a,
			B:                     b,
			Ratio:                 ratio,
			AbsDiff:               absDiff,
			Direction:             conflictDirectionFor(key),
			Reconciled:            false,
			Computable:            conflictVerified,
			Derivation:            "recomputed live from the published PDF by nepraper.Conflicts",
			ReproducedByThisBuild: true,
			RecomputeNeeds:        []string{nepraper.NormalizeFY(c.A.ReportFY)},
			Registry:              conflictRegistryFor(key),
			Note: "the two sides disagree on the published " + field + "; both reach the caller with their " +
				"own table and page and neither is preferred",
		})
	}
	return out
}

// perConflictFieldValues picks the two Values a conflict is actually about.
// The three published quantities — the DISCO's figure, NEPRA's target and the
// breach verdict — are different quantities and must never be compared with
// each other, but a same-key contradiction within any one of them is real.
func perConflictFieldValues(c nepraper.Conflict) (nepraper.Value, nepraper.Value) {
	switch c.Field {
	case "target":
		return c.A.Target, c.B.Target
	case "breach":
		return c.A.Breach, c.B.Breach
	default:
		return c.A.Value, c.B.Value
	}
}

// perSide projects one observation into a side, keeping the null discipline:
// a non-numeric Value contributes NO `value` key.
func perSide(o nepraper.Observation, v nepraper.Value, variant string) conflictSide {
	side := conflictSide{
		Raw:       v.Raw,
		ValueKind: v.Kind.String(),
		ReportFY:  o.ReportFY,
		Table:     o.Prov.TableLabel,
		Page:      o.Prov.Page,
		Variant:   variant,
		URL:       conflictPERURL(o.ReportFY),
		Basis:     v.Reason,
	}
	if n, ok := v.Float(); ok {
		side.Value = fptr(n)
	}
	if v.Label != "" && side.Basis == "" {
		side.Basis = "canonical label " + v.Label
	}
	return side
}

// perDiscriminatorFor keeps two conflicts on one key distinguishable by the
// tables that printed them, so an id never collides.
func perDiscriminatorFor(c nepraper.Conflict) string {
	return conflictPERDiscriminator(c.Field, c.A.Prov.TableLabel, c.B.Prov.TableLabel)
}

// perBreakSkip records a group that was NOT turned into a series, and why.
// Skips are reported rather than silently dropped.
type perBreakSkip struct {
	Group  string `json:"group"`
	Reason string `json:"reason"`
}

// perBreaks finds adjacent-period steps at or above minRatio.
//
// Grouping is by (Entity, Metric, TableLabel) and only columns whose Role is
// RoleFiscalYear are read. Both halves matter:
//
//   - Grouping by TABLE is what keeps a break inside ONE published series.
//     Comparing across tables would silently manufacture the very same-key
//     conflicts the conflict leg already reports — the FY2024-25 MEPCO 3x pair
//     would reappear here as a "break" between Table 6 and Table 18.
//   - Requiring RoleFiscalYear is what keeps a target or a breach column out
//     of the series. A headline table's single reported-figure column yields a
//     one-point series and therefore no step, which is correct.
//
// A group whose columns publish the same period twice is REFUSED rather than
// resolved: picking one of the two would be exactly the reconciliation this
// command exists to refuse.
func perBreaks(r *nepraper.Report, minRatio float64) ([]conflictEntry, []perBreakSkip) {
	if r == nil {
		return nil, nil
	}
	fyColumns := perFiscalYearColumns(r)

	type groupKey struct{ entity, metric, table string }
	groups := map[groupKey][]nepraper.Observation{}
	var order []groupKey
	for _, o := range r.Observations {
		cols, ok := fyColumns[o.Prov.TableLabel]
		if !ok || !cols[o.Prov.ColumnIndex] {
			continue
		}
		if nepraper.NormalizeFY(o.PeriodFY) == "" {
			continue
		}
		k := groupKey{string(o.Entity), string(o.Metric), o.Prov.TableLabel}
		if _, seen := groups[k]; !seen {
			order = append(order, k)
		}
		groups[k] = append(groups[k], o)
	}

	var entries []conflictEntry
	var skips []perBreakSkip
	for _, k := range order {
		obs := groups[k]
		label := k.entity + "/" + k.metric + " in " + k.table
		seen := map[string]bool{}
		duplicate := false
		for _, o := range obs {
			fy := nepraper.NormalizeFY(o.PeriodFY)
			if seen[fy] {
				duplicate = true
				break
			}
			seen[fy] = true
		}
		if duplicate {
			skips = append(skips, perBreakSkip{Group: label, Reason: "the table publishes the same period " +
				"more than once for this key, so an adjacent-period series cannot be built without " +
				"choosing between two published figures; that choice is refused"})
			continue
		}
		if len(obs) < 2 {
			continue
		}
		sort.SliceStable(obs, func(i, j int) bool {
			return nepraper.NormalizeFY(obs[i].PeriodFY) < nepraper.NormalizeFY(obs[j].PeriodFY)
		})
		for i := 0; i+1 < len(obs); i++ {
			e, ok := perBreakEntry(obs[i], obs[i+1], minRatio)
			if ok {
				entries = append(entries, e)
			}
		}
	}
	return entries, skips
}

// perFiscalYearColumns maps a table label to the set of its column indexes
// whose Role is RoleFiscalYear.
func perFiscalYearColumns(r *nepraper.Report) map[string]map[int]bool {
	out := map[string]map[int]bool{}
	for _, t := range r.Tables {
		for _, c := range t.Columns {
			if c.Role != nepraper.RoleFiscalYear {
				continue
			}
			if out[t.Label] == nil {
				out[t.Label] = map[int]bool{}
			}
			out[t.Label][c.Index] = true
		}
	}
	return out
}

// perBreakEntry builds one break entry when the step reaches minRatio.
//
// The ratio is max/min so it is always >= 1 and direction-free: a break is a
// step, and which way it went is in the two sides.
func perBreakEntry(from, to nepraper.Observation, minRatio float64) (conflictEntry, bool) {
	a, aok := from.Value.Float()
	b, bok := to.Value.Float()
	if !aok || !bok || a == 0 || b == 0 {
		// A zero endpoint has no ratio at all. It is not a break at any
		// threshold, and it is not reported as one.
		return conflictEntry{}, false
	}
	hi, lo := math.Abs(a), math.Abs(b)
	if lo > hi {
		hi, lo = lo, hi
	}
	ratio := hi / lo
	if ratio < minRatio {
		return conflictEntry{}, false
	}
	key := conflictKey{
		PeriodFY: nepraper.NormalizeFY(to.PeriodFY),
		Entity:   string(to.Entity),
		Metric:   string(to.Metric),
	}
	fromFY := nepraper.NormalizeFY(from.PeriodFY)
	seen := ratio
	return conflictEntry{
		ID:      conflictEntryID(conflictSurfacePER, conflictKindBreak, key, "from-"+fromFY),
		Surface: conflictSurfacePER,
		Kind:    conflictKindBreak,
		SameKey: false,
		Key:     key,
		A: conflictSide{
			Raw: from.Value.Raw, Value: fptr(a), ValueKind: from.Value.Kind.String(),
			ReportFY: from.ReportFY, Table: from.Prov.TableLabel, Page: from.Prov.Page,
			Variant: from.Prov.Variant, URL: conflictPERURL(from.ReportFY),
			Basis: "the " + fromFY + " column of the same published series",
		},
		B: conflictSide{
			Raw: to.Value.Raw, Value: fptr(b), ValueKind: to.Value.Kind.String(),
			ReportFY: to.ReportFY, Table: to.Prov.TableLabel, Page: to.Prov.Page,
			Variant: to.Prov.Variant, URL: conflictPERURL(to.ReportFY),
			Basis: "the " + key.PeriodFY + " column of the same published series",
		},
		Ratio:                 conflictRatio(fptr(b), fptr(a)),
		AbsDiff:               conflictAbsDiff(fptr(b), fptr(a)),
		Direction:             "unknowable",
		Reconciled:            false,
		Computable:            conflictVerified,
		Derivation:            "recomputed live from the published PDF",
		ReproducedByThisBuild: true,
		RecomputeNeeds:        []string{nepraper.NormalizeFY(to.ReportFY)},
		BreakRatioSeen:        &seen,
		Note: fmt.Sprintf("a %.2fx step between two adjacent periods of one published series, at the "+
			"threshold this run declared", ratio),
	}, true
}

// perContentHash is the stable half of a PER artifact's provenance: a hash
// over the EXTRACTED observations rather than the bytes.
//
// A raw byte hash of anything on this site is useless as a document identity —
// two identical fetches of one page seconds apart returned the same 74,431
// bytes with different SHA-256s because Cloudflare rewrites every
// data-cfemail token per response — so the artifact carries both and only this
// one is comparable across fetches.
func perContentHash(r *nepraper.Report) ([]byte, error) {
	type wire struct {
		ReportFY, PeriodFY, Entity, Metric, Raw, Table, Variant string
		Page                                                    int
	}
	rows := make([]wire, 0, len(r.Observations))
	for _, o := range r.Observations {
		rows = append(rows, wire{
			ReportFY: o.ReportFY, PeriodFY: o.PeriodFY, Entity: string(o.Entity),
			Metric: string(o.Metric), Raw: o.Value.Raw, Table: o.Prov.TableLabel,
			Variant: o.Prov.Variant, Page: o.Prov.Page,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Table != rows[j].Table {
			return rows[i].Table < rows[j].Table
		}
		if rows[i].Entity != rows[j].Entity {
			return rows[i].Entity < rows[j].Entity
		}
		if rows[i].Metric != rows[j].Metric {
			return rows[i].Metric < rows[j].Metric
		}
		return rows[i].PeriodFY < rows[j].PeriodFY
	})
	return json.Marshal(rows)
}
