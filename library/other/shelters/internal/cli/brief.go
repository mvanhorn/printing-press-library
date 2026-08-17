// Copyright 2026 Abe Diaz (@abe238) and contributors. Licensed under Apache-2.0. See LICENSE.
//
// The `brief` command: one-call situational awareness over the OpenShelters
// feed. Open count, breakdown by state, pet-friendly and accessible counts, and
// the capacity picture (computable + at/over). --markdown adds the gratitude +
// safety footer.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// briefData is the brief payload.
type briefData struct {
	OpenCount          int            `json:"open_count"`
	ByState            map[string]int `json:"by_state"`
	PetFriendlyCount   int            `json:"pet_friendly_count"`
	ADACount           int            `json:"ada_count"`
	WheelchairCount    int            `json:"wheelchair_count"`
	CapacityComputable int            `json:"capacity_computable_count"`
	AtCapacityCount    int            `json:"at_capacity_count"`
	ReportedFullCount  int            `json:"reported_full_count"`
	Summary            string         `json:"summary"`
}

// pp:data-source auto
func newNovelBriefCmd(flags *rootFlags) *cobra.Command {
	var flagState string
	var flagMarkdown bool
	var flagFixture string

	cmd := &cobra.Command{
		Use:   "brief",
		Short: "One-call shelter situational briefing (open count, pets, accessibility, capacity)",
		Long: "One command summarizing the shelter picture: how many are open, where, how many take pets, " +
			"how many are confirmed accessible, and the capacity situation. --markdown renders a human " +
			"briefing with a gratitude + safety footer. Useful even on a quiet day (the counts are simply low).",
		Example:     "  shelters-pp-cli brief\n  shelters-pp-cli brief --state TX --markdown",
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
			data := buildBrief(shelters)
			// --markdown is a human format: honor --quiet (suppress) and let
			// --json win (machine consumers get the envelope, not markdown).
			if flagMarkdown {
				if flags.quiet {
					return nil
				}
				if !flags.asJSON {
					fmt.Fprint(cmd.OutOrStdout(), renderBriefMarkdown(data))
					return nil
				}
			}
			return emitEnvelopeHuman(cmd, flags, source, data, func() string {
				return data.Summary + "\n"
			})
		},
	}
	cmd.Flags().StringVar(&flagState, "state", "", "Limit the briefing to a two-letter state/territory (e.g. TX)")
	cmd.Flags().BoolVar(&flagMarkdown, "markdown", false, "Render a human briefing with the gratitude + safety footer")
	cmd.Flags().StringVar(&flagFixture, "fixture", "", "Parse a saved feed JSON (path or - for stdin) instead of fetching live")
	return cmd
}

// buildBrief aggregates the feed into the briefing payload.
func buildBrief(shelters []Shelter) briefData {
	d := briefData{ByState: map[string]int{}}
	d.OpenCount = len(shelters)
	for _, s := range shelters {
		if s.State != "" {
			d.ByState[s.State]++
		}
		if allowsPets(s.PetAccommodations) {
			d.PetFriendlyCount++
		}
		if isYes(s.ADACompliant) {
			d.ADACount++
		}
		if isYes(s.WheelchairAccessible) {
			d.WheelchairCount++
		}
	}
	cap := buildCapacity(shelters)
	d.CapacityComputable = cap.ComputableCount
	d.AtCapacityCount = cap.AtCapacityCount
	d.ReportedFullCount = cap.ReportedFull

	d.Summary = fmt.Sprintf(
		"%d open shelter(s); %d take pets; %d ADA, %d wheelchair accessible; %d with computable capacity, %d at/over capacity.",
		d.OpenCount, d.PetFriendlyCount, d.ADACount, d.WheelchairCount, d.CapacityComputable, d.AtCapacityCount)
	return d
}

// renderBriefMarkdown renders the human briefing with the mandatory footer.
func renderBriefMarkdown(d briefData) string {
	var b strings.Builder
	fmt.Fprintln(&b, "# Shelter Situational Briefing")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "%s\n\n", d.Summary)

	fmt.Fprintf(&b, "## Open shelters (%d)\n\n", d.OpenCount)
	if d.OpenCount == 0 {
		fmt.Fprintln(&b, "No open shelters reported right now. This is normal when no disaster is active.")
	} else {
		states := make([]string, 0, len(d.ByState))
		for st := range d.ByState {
			states = append(states, st)
		}
		sort.Strings(states)
		for _, st := range states {
			fmt.Fprintf(&b, "- %s: %d\n", st, d.ByState[st])
		}
	}
	fmt.Fprintln(&b)

	fmt.Fprintln(&b, "## Access")
	fmt.Fprintf(&b, "- Pet-friendly: %d\n", d.PetFriendlyCount)
	fmt.Fprintf(&b, "- ADA compliant: %d\n", d.ADACount)
	fmt.Fprintf(&b, "- Wheelchair accessible: %d\n", d.WheelchairCount)
	fmt.Fprintln(&b)

	fmt.Fprintln(&b, "## Capacity")
	fmt.Fprintf(&b, "- Computable utilization: %d\n", d.CapacityComputable)
	fmt.Fprintf(&b, "- At/over capacity: %d\n", d.AtCapacityCount)
	fmt.Fprintf(&b, "- Reported FULL: %d\n", d.ReportedFullCount)
	fmt.Fprintln(&b)

	fmt.Fprintln(&b, "---")
	fmt.Fprintln(&b, sheltersGratitude)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, sheltersDisclaimer)
	return b.String()
}
