// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
// See .printing-press-patches/nepra-fca-series.json.
//
// pp:data-source live
// Supported strategies: auto, local, live, or computed. `live` matches the
// generated command this augments: there is no local store for the FCA sheet.
//
// WHY THIS FILE EXISTS. Transcendence row 9 approved
// `fca [--entity cppag|ke] [--cumulative] [--billing-month 2020-01]`, and the
// shipped `fca` had NO flags at all — it is a generated promoted command that
// returns the whole 48-row sheet and nothing else. verify-skill and the
// Sample Output Probe both caught it; the Phase 3 gate did not, because that
// gate resolves the command PATH and these are flags on a path that resolves.
//
// HOW IT ATTACHES. internal/cli/promoted_fca.go carries a "DO NOT EDIT"
// header and its command is NOT a novel scaffold, so addNovelCommandIfAbsent
// returns early on the name collision and cannot replace it. The hook below
// therefore finds the generated command and AUGMENTS IT IN PLACE: it adds the
// three flags and wraps RunE, so the generated Short, Example, annotations
// and the whole fetch path are preserved and a regen cannot drop the wiring.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/client"
)

// fcaEntity is one of the two entities the sheet publishes side by side.
type fcaEntity struct {
	ID string
	// Label is how the entity is named in prose.
	Label string
	// RequestedColumn and AllowedColumn are the sheet's own header strings,
	// VERBATIM, spacing included. "CPPA - G FCA Allowed (kWh)" carries spaces
	// around its hyphen and a unit suffix its sibling column does not, and
	// neither is normalised anywhere: these strings are the join to the
	// extracted rows and a tidied copy silently matches nothing.
	RequestedColumn string
	AllowedColumn   string
}

var fcaEntities = []fcaEntity{
	{ID: "cppag", Label: "CPPA-G", RequestedColumn: "CPPA - G FCA Requested", AllowedColumn: "CPPA - G FCA Allowed (kWh)"},
	{ID: "ke", Label: "K-Electric", RequestedColumn: "K-Electric FCA Requested", AllowedColumn: "K-Electric FCA Allowed"},
}

func fcaEntityByID(id string) (fcaEntity, bool) {
	for _, e := range fcaEntities {
		if strings.EqualFold(e.ID, id) {
			return e, true
		}
	}
	return fcaEntity{}, false
}

func fcaEntityIDs() []string {
	out := make([]string, 0, len(fcaEntities))
	for _, e := range fcaEntities {
		out = append(out, e.ID)
	}
	return out
}

// fcaMonths is the sheet's own month spelling, in FISCAL-FILE order as three
// letter abbreviations. Measured across all 48 rows: exactly these twelve.
var fcaMonths = map[string]int{
	"Jan": 1, "Feb": 2, "Mar": 3, "Apr": 4, "May": 5, "Jun": 6,
	"Jul": 7, "Aug": 8, "Sep": 9, "Oct": 10, "Nov": 11, "Dec": 12,
}

// fcaParseFigure reads one published FCA cell.
//
// THE PARENTHESES ARE THE WHOLE POINT. This sheet writes negatives in
// accounting notation — "(2.5935)" — and there are 49 such cells across the
// 48 rows. strconv.ParseFloat REJECTS that string, so any code that ignores
// the error reads a real negative adjustment as a zero, and any code that
// strips the parens without restoring the sign reads it as a positive. Both
// silently change the direction of a tariff adjustment.
//
// Returns ok=false for a genuinely empty cell, which is ABSENCE and must not
// become 0: the E.M.O Revised columns are empty on most rows.
func fcaParseFigure(raw string) (float64, bool, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, false, nil
	}
	negative := false
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		negative = true
		s = strings.TrimSuffix(strings.TrimPrefix(s, "("), ")")
	}
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false, fmt.Errorf("cell %q is neither a number nor an accounting negative: %w", raw, err)
	}
	if negative {
		v = -v
	}
	return v, true, nil
}

// fcaPublishedDecimals is the precision this sheet prints its figures to.
// Every cell in the corpus carries at most four decimal places.
const fcaPublishedDecimals = 4

// fcaRound snaps a DERIVED figure back to the source's published precision.
//
// It is applied ONLY to values this command computes — a difference or a sum —
// and NEVER to a published operand, which is parsed exactly as printed.
//
// WHY IT IS NOT A FUDGE. The operands are decimal values printed to four
// places; their exact difference and exact sum are therefore also
// representable at four places. What float64 cannot represent is the operands
// themselves, so 9.9095 - 9.8972 lands on 0.012299999999999756 and summing 48
// such differences printed 8.842499999999998 for a figure whose true value is
// 8.8425. Rounding to the published precision RESTORES the exact decimal
// answer rather than approximating it; leaving the dust in would publish a
// tariff figure no NEPRA document contains, in a CLI whose whole contract is
// that every number it emits is traceable to a published one.
func fcaRound(v float64) float64 {
	shift := math.Pow(10, fcaPublishedDecimals)
	return math.Round(v*shift) / shift
}

// fcaRow is one billing month for one entity.
type fcaRow struct {
	BillingMonth string `json:"billing_month"`
	Year         int    `json:"year"`
	Month        string `json:"month"`
	Entity       string `json:"entity"`
	// Requested and Allowed are nil when the published cell is EMPTY. An
	// absent adjustment is not a zero adjustment.
	Requested *float64 `json:"requested_rs_per_kwh"`
	Allowed   *float64 `json:"allowed_rs_per_kwh"`
	// Disallowance is requested minus allowed, and is nil unless BOTH sides
	// are published. It is never computed against a missing operand.
	Disallowance *float64 `json:"disallowance_rs_per_kwh"`
	// AllowedBelowRequested is nil on the same condition, for the same
	// reason. It is the month-level form of the headline finding.
	AllowedBelowRequested *bool `json:"allowed_below_requested"`
}

// fcaCumulative is the aggregation transcendence row 9 exists for.
type fcaCumulative struct {
	Entity string `json:"entity"`
	// Months is how many billing months contributed a figure to each sum,
	// carried so a total can never be read without its denominator.
	MonthsRequested int `json:"months_requested"`
	MonthsAllowed   int `json:"months_allowed"`
	MonthsBoth      int `json:"months_both"`
	// Requested / Allowed are nil when no month published a figure, so an
	// unmeasured total is never rendered as 0.00.
	Requested *float64 `json:"cumulative_requested_rs_per_kwh"`
	Allowed   *float64 `json:"cumulative_allowed_rs_per_kwh"`
	// Disallowance is the sum over the months where BOTH sides published,
	// which is why it is not simply Requested minus Allowed unless
	// MonthsRequested == MonthsAllowed == MonthsBoth.
	Disallowance          *float64 `json:"cumulative_disallowance_rs_per_kwh"`
	AllowedBelowRequested int      `json:"months_allowed_below_requested"`
	Basis                 string   `json:"basis"`
}

// fcaBuildRows turns the extracted sheet into per-entity month rows.
func fcaBuildRows(raw []map[string]any, entities []fcaEntity) ([]fcaRow, []string, error) {
	var warnings []string
	out := make([]fcaRow, 0, len(raw)*len(entities))
	for _, rec := range raw {
		year, month := fcaCell(rec, "Year"), fcaCell(rec, "Month")
		yr, err := strconv.Atoi(strings.TrimSpace(year))
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("row with Year %q is not a year and was skipped", year))
			continue
		}
		mn, ok := fcaMonths[strings.TrimSpace(month)]
		if !ok {
			warnings = append(warnings, fmt.Sprintf("row %s has month %q, which is not one of the twelve published spellings, and was skipped", year, month))
			continue
		}
		for _, e := range entities {
			row := fcaRow{
				BillingMonth: fmt.Sprintf("%04d-%02d", yr, mn),
				Year:         yr,
				Month:        strings.TrimSpace(month),
				Entity:       e.ID,
			}
			req, haveReq, err := fcaParseFigure(fcaCell(rec, e.RequestedColumn))
			if err != nil {
				return nil, warnings, fmt.Errorf("%s %s requested: %w", e.Label, row.BillingMonth, err)
			}
			alw, haveAlw, err := fcaParseFigure(fcaCell(rec, e.AllowedColumn))
			if err != nil {
				return nil, warnings, fmt.Errorf("%s %s allowed: %w", e.Label, row.BillingMonth, err)
			}
			if haveReq {
				v := req
				row.Requested = &v
			}
			if haveAlw {
				v := alw
				row.Allowed = &v
			}
			if haveReq && haveAlw {
				d := fcaRound(req - alw)
				row.Disallowance = &d
				b := alw < req
				row.AllowedBelowRequested = &b
			}
			out = append(out, row)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].BillingMonth != out[j].BillingMonth {
			return out[i].BillingMonth < out[j].BillingMonth
		}
		return out[i].Entity < out[j].Entity
	})
	return out, warnings, nil
}

func fcaCell(rec map[string]any, key string) string {
	if v, ok := rec[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
		if v != nil {
			return fmt.Sprint(v)
		}
	}
	return ""
}

// fcaBuildCumulative sums the series per entity.
//
// EACH SUM CARRIES ITS OWN DENOMINATOR. A cumulative disallowance quoted
// without the month count is unreadable: this window is 48 months, and the
// same number over 12 months would mean something completely different.
func fcaBuildCumulative(rows []fcaRow, entities []fcaEntity) []fcaCumulative {
	byEntity := map[string]*fcaCumulative{}
	order := []string{}
	for _, e := range entities {
		byEntity[e.ID] = &fcaCumulative{Entity: e.ID}
		order = append(order, e.ID)
	}
	var reqSum, alwSum, disSum map[string]float64
	reqSum, alwSum, disSum = map[string]float64{}, map[string]float64{}, map[string]float64{}

	for _, r := range rows {
		c, ok := byEntity[r.Entity]
		if !ok {
			continue
		}
		if r.Requested != nil {
			c.MonthsRequested++
			reqSum[r.Entity] += *r.Requested
		}
		if r.Allowed != nil {
			c.MonthsAllowed++
			alwSum[r.Entity] += *r.Allowed
		}
		if r.Disallowance != nil {
			c.MonthsBoth++
			disSum[r.Entity] += *r.Disallowance
		}
		if r.AllowedBelowRequested != nil && *r.AllowedBelowRequested {
			c.AllowedBelowRequested++
		}
	}

	out := make([]fcaCumulative, 0, len(order))
	for _, id := range order {
		c := byEntity[id]
		if c.MonthsRequested > 0 {
			v := fcaRound(reqSum[id])
			c.Requested = &v
		}
		if c.MonthsAllowed > 0 {
			v := fcaRound(alwSum[id])
			c.Allowed = &v
		}
		if c.MonthsBoth > 0 {
			v := fcaRound(disSum[id])
			c.Disallowance = &v
		}
		c.Basis = fmt.Sprintf(
			"summed over the months this sheet publishes, not over a calendar window: %d months carry a requested figure, %d an allowed figure, %d both. "+
				"The cumulative disallowance is summed only over the %d months publishing BOTH sides, so it is not requested minus allowed unless all three counts agree. "+
				"Accounting-parenthesised cells are negative adjustments and are summed with their sign.",
			c.MonthsRequested, c.MonthsAllowed, c.MonthsBoth, c.MonthsBoth)
		out = append(out, *c)
	}
	return out
}

// fcaFilteredSeries is the whole flag-driven path.
//
// It is called from the ONE patched line in the generated
// internal/cli/promoted_fca.go, which is also where the three flags are
// declared. That placement is deliberate: verify-skill attributes a flag to a
// command by finding the declaration and the command's `Use:` literal
// together, and with the flags in this file it reported
// `--entity is declared elsewhere but not on fca` — a false finding about a
// flag that works. Declaring them on the command itself makes the attribution
// true for the scanner and for a human reading promoted_fca.go.
//
// Returns handled=false when no flag was given, so the generated RunE
// continues untouched and default output is byte-identical to a build without
// this file.
func fcaFilteredSeries(c *cobra.Command, flags *rootFlags) (bool, error) {
	flagEntity, _ := c.Flags().GetString("entity")
	flagCumul, _ := c.Flags().GetBool("cumulative")
	flagMonth, _ := c.Flags().GetString("billing-month")
	if flagEntity == "" && !flagCumul && flagMonth == "" {
		return false, nil
	}
	err := func() error {
		if dryRunOK(flags) {
			return writeDryRun(c.OutOrStdout(), flags, "fca")
		}

		entities := fcaEntities
		if flagEntity != "" {
			e, ok := fcaEntityByID(flagEntity)
			if !ok {
				_ = c.Usage()
				return usageErr(fmt.Errorf("--entity %q is not published in this sheet; it carries exactly two: %s",
					flagEntity, strings.Join(fcaEntityIDs(), ", ")))
			}
			entities = []fcaEntity{e}
		}
		if flagMonth != "" {
			if !fcaValidMonthForm(flagMonth) {
				_ = c.Usage()
				return usageErr(fmt.Errorf("--billing-month %q is not a YYYY-MM billing month", flagMonth))
			}
		}

		raw, art, err := fcaFetchRows(c, flags)
		if err != nil {
			return err
		}
		rows, warnings, err := fcaBuildRows(raw, entities)
		if err != nil {
			return err
		}
		window := fcaWindow(rows)
		if flagMonth != "" {
			kept := make([]fcaRow, 0, len(rows))
			for _, r := range rows {
				if r.BillingMonth == flagMonth {
					kept = append(kept, r)
				}
			}
			if len(kept) == 0 {
				return notFoundErr(fmt.Errorf("this sheet does not publish billing month %s; it covers %s and is upstream-frozen at that window",
					flagMonth, window))
			}
			rows = kept
		}

		type envelope struct {
			Meta    map[string]any  `json:"meta"`
			Results []fcaRow        `json:"results"`
			Totals  []fcaCumulative `json:"cumulative,omitempty"`
		}
		meta := map[string]any{
			"source":           "live",
			"artifact":         art,
			"entities":         fcaEntityIDs(),
			"entity":           flagEntity,
			"billing_month":    flagMonth,
			"published_window": window,
			"rows":             len(rows),
			"unit":             "Rs/kWh",
			"refusals": []string{
				"CPPA-G and K-Electric figures are NEVER summed or averaged together: they are separate tariffs for separate consumer populations, published side by side in one sheet.",
				"No figure is emitted for a month this sheet does not publish. The window is upstream-frozen; an absent month is absent, not zero.",
			},
		}
		if len(warnings) > 0 {
			meta["warnings"] = warnings
		}
		env := envelope{Meta: meta, Results: rows}
		if flagCumul {
			env.Totals = fcaBuildCumulative(rows, entities)
		}
		return printJSONFiltered(c.OutOrStdout(), env, flags)
	}()
	return true, err
}

// fcaValidMonthForm checks the YYYY-MM shape without asserting the month is
// published; that is a separate, exit-3 finding.
func fcaValidMonthForm(s string) bool {
	if len(s) != 7 || s[4] != '-' {
		return false
	}
	y, err := strconv.Atoi(s[:4])
	if err != nil || y < 1900 {
		return false
	}
	m, err := strconv.Atoi(s[5:])
	return err == nil && m >= 1 && m <= 12
}

// fcaWindow renders the published window from the rows themselves rather than
// from a hardcoded constant, so it cannot drift from the sheet.
func fcaWindow(rows []fcaRow) string {
	if len(rows) == 0 {
		return "no published months"
	}
	lo, hi := rows[0].BillingMonth, rows[0].BillingMonth
	for _, r := range rows {
		if r.BillingMonth < lo {
			lo = r.BillingMonth
		}
		if r.BillingMonth > hi {
			hi = r.BillingMonth
		}
	}
	return lo + " .. " + hi
}

// fcaFetchRows performs the one GET and runs the body through the shared
// Excel-sheet extractor, which is the same path the generated command uses.
func fcaFetchRows(cmd *cobra.Command, flags *rootFlags) ([]map[string]any, nepraArtifact, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, nepraArtifact{}, err
	}
	// The same path and the same HTML-response header the generated command
	// uses. The header is load-bearing: without it the client treats the body
	// as JSON and the Excel sheet never reaches the extractor.
	path := resourceReadPaths["fca"]
	body, err := c.GetWithHeaders(cmd.Context(), path, map[string]string{},
		map[string]string{client.HTMLResponseHeader: "true"})
	if err != nil {
		return nil, nepraArtifact{}, classifyAPIError(cmd.OutOrStdout(), err, flags)
	}
	extracted, err := extractNepraTable(body)
	if err != nil {
		return nil, nepraArtifact{}, err
	}
	var rows []map[string]any
	if err := json.Unmarshal(extracted, &rows); err != nil {
		return nil, nepraArtifact{}, fmt.Errorf("the FCA sheet did not extract as rows: %w", err)
	}
	art := newNepraArtifact(c.RequestBaseURL()+path, body, extracted, c.LastContentType(), len(rows), 0)
	return rows, art, nil
}
