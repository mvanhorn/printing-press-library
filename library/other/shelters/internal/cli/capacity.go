// Copyright 2026 Abe Diaz (@abe238) and contributors. Licensed under Apache-2.0. See LICENSE.
//
// The `capacity` command answers "which shelters are at capacity?" honestly.
// Utilization is computed ONLY where total_population and a capacity both exist;
// the denominator is evacuation_capacity by default (pre-hazard, 20 sqft/person)
// and falls back to post_impact_capacity (40 sqft/person) with an explicit
// label. A shelter the feed reports as FULL is surfaced even when its numbers
// are missing, but FULL is treated as "reported", not asserted as ground truth.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

const (
	atCapacityPct   = 100.0
	nearCapacityPct = 90.0
)

// capacityRow is one shelter's capacity view.
type capacityRow struct {
	Shelter
	UtilizationPct *float64 `json:"utilization_pct"`
	CapacityBasis  string   `json:"capacity_basis"`
	AtCapacity     bool     `json:"at_capacity"`
	NearCapacity   bool     `json:"near_capacity"`
	ReportedFull   bool     `json:"reported_full"`
}

// capacityData is the capacity command payload.
type capacityData struct {
	ComputableCount int           `json:"computable_count"`
	UnknownCount    int           `json:"unknown_count"`
	AtCapacityCount int           `json:"at_capacity_count"`
	ReportedFull    int           `json:"reported_full_count"`
	Note            string        `json:"note"`
	Shelters        []capacityRow `json:"shelters"`
}

// pp:data-source auto
func newNovelCapacityCmd(flags *rootFlags) *cobra.Command {
	var flagState string
	var flagFixture string

	cmd := &cobra.Command{
		Use:   "capacity",
		Short: "Which open shelters are at or near capacity (honest about unknowns)",
		Long: "Report shelter utilization. Utilization % is computed only where both the current " +
			"population and a capacity are reported; the denominator is evacuation_capacity by default " +
			"and falls back to post_impact_capacity with a label. Shelters with missing numbers are " +
			"counted as unknown, never assumed full or empty. A shelter the feed marks FULL is surfaced " +
			"as reported-full.\n\nUse case: \"which shelter is at capacity?\"",
		Example:     "  shelters-pp-cli capacity\n  shelters-pp-cli capacity --state TX --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			source, shelters, err := loadShelterFeed(cmd, flags, flagFixture)
			if err != nil {
				return err
			}
			shelters = shelterFilter{state: flagState}.apply(shelters)
			data := buildCapacity(shelters)
			return emitEnvelopeHuman(cmd, flags, source, data, func() string {
				return renderCapacity(data)
			})
		},
	}
	cmd.Flags().StringVar(&flagState, "state", "", "Filter to a two-letter state/territory (e.g. TX)")
	cmd.Flags().StringVar(&flagFixture, "fixture", "", "Parse a saved feed JSON (path or - for stdin) instead of fetching live")
	return cmd
}

// buildCapacity computes the capacity view. Rows are sorted so computable rows
// come first (highest utilization first), then unknowns.
func buildCapacity(shelters []Shelter) capacityData {
	var d capacityData
	rows := make([]capacityRow, 0, len(shelters))
	for _, s := range shelters {
		row := capacityRow{Shelter: s}
		row.ReportedFull = s.Status == "FULL"
		denom, basis := capacityDenominator(s)
		if s.TotalPopulation != nil && denom != nil && *denom > 0 {
			pct := float64(*s.TotalPopulation) / float64(*denom) * 100
			pct = round1(pct)
			row.UtilizationPct = &pct
			row.CapacityBasis = basis
			row.AtCapacity = pct >= atCapacityPct
			row.NearCapacity = pct >= nearCapacityPct && pct < atCapacityPct
			d.ComputableCount++
		} else {
			d.UnknownCount++
		}
		if row.ReportedFull {
			row.AtCapacity = true
			d.ReportedFull++
		}
		if row.AtCapacity {
			d.AtCapacityCount++
		}
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		ui, uj := rows[i].UtilizationPct, rows[j].UtilizationPct
		switch {
		case ui != nil && uj != nil:
			return *ui > *uj
		case ui != nil:
			return true
		case uj != nil:
			return false
		default:
			return false
		}
	})
	d.Shelters = rows
	d.Note = "Utilization is computed only where population and a capacity are both reported. " +
		"The denominator is evacuation_capacity (20 sqft/person) unless noted as post_impact (40 sqft/person). " +
		"A FULL status is what the feed reported and may lag reality."
	return d
}

// capacityDenominator returns the capacity to divide by and a human label for
// the basis. evacuation_capacity is primary; post_impact_capacity is the
// labeled fallback. Returns (nil, "") when neither is reported.
func capacityDenominator(s Shelter) (*int, string) {
	if s.EvacuationCapacity != nil {
		return s.EvacuationCapacity, "evacuation_capacity (20 sqft/person)"
	}
	if s.PostImpactCapacity != nil {
		return s.PostImpactCapacity, "post_impact_capacity (40 sqft/person)"
	}
	return nil, ""
}

func renderCapacity(d capacityData) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Capacity: %d computable, %d unknown, %d at/over capacity",
		d.ComputableCount, d.UnknownCount, d.AtCapacityCount)
	if d.ReportedFull > 0 {
		fmt.Fprintf(&b, " (%d reported FULL)", d.ReportedFull)
	}
	fmt.Fprintln(&b)
	for _, r := range d.Shelters {
		loc := strings.TrimSpace(r.City + ", " + r.State)
		util := "unknown"
		if r.UtilizationPct != nil {
			util = fmt.Sprintf("%.1f%% of %s", *r.UtilizationPct, r.CapacityBasis)
		}
		flag := ""
		switch {
		case r.AtCapacity:
			flag = "  [AT CAPACITY]"
		case r.NearCapacity:
			flag = "  [near capacity]"
		}
		if r.ReportedFull {
			flag += "  [reported FULL]"
		}
		fmt.Fprintf(&b, "- %s (id %d) -- %s\n", r.Name, r.ShelterID, loc)
		fmt.Fprintf(&b, "      pop/cap %s | utilization %s%s\n", popCapStr(r.Shelter), util, flag)
	}
	fmt.Fprintf(&b, "\n%s\n", d.Note)
	return b.String()
}
