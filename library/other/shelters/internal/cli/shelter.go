// Copyright 2026 Abe Diaz (@abe238) and contributors. Licensed under Apache-2.0. See LICENSE.
//
// The `shelter <shelter_id>` command: full detail for one shelter, joined on the
// STABLE shelter_id (never objectid, which churns across snapshots).

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// pp:data-source auto
func newNovelShelterCmd(flags *rootFlags) *cobra.Command {
	var flagFixture string

	cmd := &cobra.Command{
		Use:   "shelter <shelter_id>",
		Short: "Full detail for one shelter, joined on the stable shelter_id",
		Long: "Full detail for a single shelter. Joins on the STABLE shelter_id (never objectid, which " +
			"changes between snapshots). Unreported fields come back as explicit null. Covers open shelters in the US and its territories only, from the union of the FEMA and American Red Cross feeds.",
		Example:     "  shelters-pp-cli shelter 368133",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("shelter: a shelter_id is required"))
			}
			id, err := strconv.Atoi(strings.TrimSpace(args[0]))
			if err != nil {
				return usageErr(fmt.Errorf("shelter: shelter_id must be an integer, got %q", args[0]))
			}
			feed, err := loadShelterFeed(cmd, flags, flagFixture)
			if err != nil {
				return err
			}
			s, ok := findShelter(feed.Shelters, id)
			if !ok {
				return notFoundErr(fmt.Errorf("shelter not found: %d (a shelter_id is a positive FEMA id; Red Cross-only shelters are listed without one, see 'shelters-pp-cli shelters')", id))
			}
			note := joinNotes(feed.Enrich.humanNote(), feed.RedCross.humanNote(), feed.Occupancy.humanNote(), feed.Hidden.humanNote())
			return emitEnvelopeHuman(cmd, flags, feed.Source, s, func() string {
				return renderShelterDetail(s, note)
			})
		},
	}
	cmd.Flags().StringVar(&flagFixture, "fixture", "", "Parse a saved feed JSON (path or - for stdin) instead of fetching live")
	return cmd
}

// findShelter returns the open shelter with the given stable FEMA shelter_id.
// A non-positive id never matches: Red Cross-only rows in the union carry
// shelter_id 0 (the RC feed has no FEMA id), so `shelter 0` must report
// not-found rather than returning the first such row.
func findShelter(shelters []Shelter, id int) (Shelter, bool) {
	if id <= 0 {
		return Shelter{}, false
	}
	for i := range shelters {
		if shelters[i].ShelterID == id {
			return shelters[i], true
		}
	}
	return Shelter{}, false
}

// renderShelterDetail renders the human single-shelter view. enrichNote is the
// (possibly empty) note explaining why the FEMA_NSS/0 fields below may be blank.
func renderShelterDetail(s Shelter, enrichNote string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s (shelter_id %d)\n", s.Name, s.ShelterID)
	addr := s.Address
	loc := strings.TrimSpace(strings.Join(nonEmpty(s.City, s.State, s.Zip), ", "))
	if addr != "" {
		fmt.Fprintf(&b, "  address    %s\n", addr)
	}
	if loc != "" {
		fmt.Fprintf(&b, "  location   %s\n", loc)
	}
	if s.CountyParish != "" {
		fmt.Fprintf(&b, "  county     %s\n", s.CountyParish)
	}
	fmt.Fprintf(&b, "  status     %s\n", dashIfEmpty(s.Status))
	fmt.Fprintf(&b, "  pets       %s\n", petLabel(s.PetAccommodations))
	if s.PetAccommodationsDesc != "" {
		fmt.Fprintf(&b, "  pet notes  %s\n", s.PetAccommodationsDesc)
	}
	fmt.Fprintf(&b, "  ada        %s\n", dashIfEmpty(s.ADACompliant))
	fmt.Fprintf(&b, "  wheelchair %s\n", dashIfEmpty(s.WheelchairAccessible))
	fmt.Fprintf(&b, "  population %s\n", intPtrStr(s.TotalPopulation))
	fmt.Fprintf(&b, "  evac cap   %s\n", intPtrStr(s.EvacuationCapacity))
	fmt.Fprintf(&b, "  post cap   %s\n", intPtrStr(s.PostImpactCapacity))
	if s.OrgName != "" {
		fmt.Fprintf(&b, "  org        %s\n", s.OrgName)
	}
	// FEMA_NSS/0 enrichment, shown only when reported.
	if s.IncidentName != "" {
		fmt.Fprintf(&b, "  incident   %s\n", s.IncidentName)
	}
	if s.ShelterOpenDate != "" {
		fmt.Fprintf(&b, "  opened     %s\n", s.ShelterOpenDate)
	}
	if s.ShelterClosedDate != "" {
		fmt.Fprintf(&b, "  closed     %s\n", s.ShelterClosedDate)
	}
	if s.FacilityType != "" {
		fmt.Fprintf(&b, "  facility   %s\n", s.FacilityType)
	}
	if s.GeneratorOnsite != "" {
		fmt.Fprintf(&b, "  generator  %s\n", s.GeneratorOnsite)
	}
	if s.SelfSufficientElectricity != "" {
		fmt.Fprintf(&b, "  self-elec  %s\n", s.SelfSufficientElectricity)
	}
	if hazard := hazardFlags(s); hazard != "" {
		fmt.Fprintf(&b, "  hazard     %s\n", hazard)
	}
	if pop := populationBreakdown(s); pop != "" {
		fmt.Fprintf(&b, "  pop break  %s\n", pop)
	}
	if s.OrgMainPhone != "" {
		fmt.Fprintf(&b, "  org phone  %s\n", s.OrgMainPhone)
	}
	if s.OrgHotlinePhone != "" {
		fmt.Fprintf(&b, "  hotline    %s\n", s.OrgHotlinePhone)
	}
	if s.Latitude != nil && s.Longitude != nil {
		fmt.Fprintf(&b, "  coords     %.5f, %.5f\n", *s.Latitude, *s.Longitude)
	} else {
		fmt.Fprintf(&b, "  coords     (not reported; geocode from address)\n")
	}
	if enrichNote != "" {
		fmt.Fprintf(&b, "\n%s\n", enrichNote)
	}
	return b.String()
}

// hazardFlags joins the reported floodplain / surge / pre-landfall flags into a
// compact phrase, omitting any the feed did not report (blank).
func hazardFlags(s Shelter) string {
	var parts []string
	if s.In100YrFloodplain != "" {
		parts = append(parts, "100yr-floodplain="+s.In100YrFloodplain)
	}
	if s.In500YrFloodplain != "" {
		parts = append(parts, "500yr-floodplain="+s.In500YrFloodplain)
	}
	if s.InSurgeSloshArea != "" {
		parts = append(parts, "surge-area="+s.InSurgeSloshArea)
	}
	if s.PreLandfallShelter != "" {
		parts = append(parts, "pre-landfall="+s.PreLandfallShelter)
	}
	return strings.Join(parts, ", ")
}

// populationBreakdown joins the reported sub-population counts (general / medical
// / other / pet), omitting any that are null.
func populationBreakdown(s Shelter) string {
	var parts []string
	if s.GeneralPopulation != nil {
		parts = append(parts, fmt.Sprintf("general=%d", *s.GeneralPopulation))
	}
	if s.MedicalNeedsPopulation != nil {
		parts = append(parts, fmt.Sprintf("medical=%d", *s.MedicalNeedsPopulation))
	}
	if s.OtherPopulation != nil {
		parts = append(parts, fmt.Sprintf("other=%d", *s.OtherPopulation))
	}
	if s.PetPopulation != nil {
		parts = append(parts, fmt.Sprintf("pet=%d", *s.PetPopulation))
	}
	return strings.Join(parts, ", ")
}

func nonEmpty(in ...string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}
