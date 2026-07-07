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
			"changes between snapshots). Unreported fields come back as explicit null.",
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
			source, shelters, err := loadShelterFeed(cmd, flags, flagFixture)
			if err != nil {
				return err
			}
			for i := range shelters {
				if shelters[i].ShelterID == id {
					s := shelters[i]
					return emitEnvelopeHuman(cmd, flags, source, s, func() string {
						return renderShelterDetail(s)
					})
				}
			}
			return notFoundErr(fmt.Errorf("shelter not found: %d (run 'shelters-pp-cli shelters' to list open shelters)", id))
		},
	}
	cmd.Flags().StringVar(&flagFixture, "fixture", "", "Parse a saved feed JSON (path or - for stdin) instead of fetching live")
	return cmd
}

// renderShelterDetail renders the human single-shelter view.
func renderShelterDetail(s Shelter) string {
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
	fmt.Fprintf(&b, "  status     %s\n", dashIfEmpty(s.Status))
	fmt.Fprintf(&b, "  pets       %s\n", petLabel(s.PetAccommodations))
	fmt.Fprintf(&b, "  ada        %s\n", dashIfEmpty(s.ADACompliant))
	fmt.Fprintf(&b, "  wheelchair %s\n", dashIfEmpty(s.WheelchairAccessible))
	fmt.Fprintf(&b, "  population %s\n", intPtrStr(s.TotalPopulation))
	fmt.Fprintf(&b, "  evac cap   %s\n", intPtrStr(s.EvacuationCapacity))
	fmt.Fprintf(&b, "  post cap   %s\n", intPtrStr(s.PostImpactCapacity))
	if s.OrgName != "" {
		fmt.Fprintf(&b, "  org        %s\n", s.OrgName)
	}
	if s.Latitude != nil && s.Longitude != nil {
		fmt.Fprintf(&b, "  coords     %.5f, %.5f\n", *s.Latitude, *s.Longitude)
	} else {
		fmt.Fprintf(&b, "  coords     (not reported; geocode from address)\n")
	}
	return b.String()
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
