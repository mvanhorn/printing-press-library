// Copyright 2026 Abe Diaz (@abe238) and contributors. Licensed under Apache-2.0. See LICENSE.
//
// The `shelters` (alias `list`) command: open shelters from the FEMA
// OpenShelters feed, flattened and filterable. Replaces the generic generated
// list because the feed nests fields under each feature's attributes object and
// the high-value access is by pets / accessibility / state, not raw ArcGIS rows.

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// shelterFilter is the shared filter set for list/near/capacity/brief. Empty
// fields and false flags are no-ops. The county/generator filters read FEMA_NSS/0
// enrichment fields, so they match nothing when enrichment was skipped or failed
// (honest: we cannot filter on data we do not have).
type shelterFilter struct {
	state      string
	pets       bool
	ada        bool
	wheelchair bool
	org        string
	status     string
	county     string
	generator  bool
}

// apply returns the subset of shelters matching every active filter.
func (sf shelterFilter) apply(in []Shelter) []Shelter {
	state := normCode(sf.state)
	status := normCode(sf.status)
	org := strings.ToLower(strings.TrimSpace(sf.org))
	county := strings.ToLower(strings.TrimSpace(sf.county))
	out := make([]Shelter, 0, len(in))
	for _, s := range in {
		if state != "" && s.State != state {
			continue
		}
		if status != "" && s.Status != status {
			continue
		}
		if sf.pets && !allowsPets(s.PetAccommodations) {
			continue
		}
		if sf.ada && !isYes(s.ADACompliant) {
			continue
		}
		if sf.wheelchair && !isYes(s.WheelchairAccessible) {
			continue
		}
		if org != "" && !strings.Contains(strings.ToLower(s.OrgName), org) {
			continue
		}
		if county != "" && !strings.Contains(strings.ToLower(s.CountyParish), county) {
			continue
		}
		if sf.generator && !isYes(s.GeneratorOnsite) {
			continue
		}
		out = append(out, s)
	}
	return out
}

// shelterListData is the list command payload.
type shelterListData struct {
	Count      int         `json:"count"`
	Shelters   []Shelter   `json:"shelters"`
	Enrichment enrichState `json:"enrichment"`
	RedCross   enrichState `json:"red_cross"`
	Occupancy  enrichState `json:"occupancy"`
	Hidden     enrichState `json:"hidden_filter"`
}

// pp:data-source auto
func newSheltersPromotedCmd(flags *rootFlags) *cobra.Command {
	var sf shelterFilter
	var flagLimit int
	var flagFixture string

	cmd := &cobra.Command{
		Use:     "shelters",
		Aliases: []string{"list"},
		Short:   "List open shelters from the union of FEMA OpenShelters and the American Red Cross feed (US only)",
		Long: "List open shelters from the union of FEMA's National Shelter System OpenShelters feed and the " +
			"American Red Cross feed (deduped, each tagged with a source field: fema, redcross, or fema+redcross), flattened " +
			"and filterable by state, pet policy, ADA / wheelchair accessibility, managing org, and " +
			"status. Returns few or none when no disaster is active (quiet baseline); counts spike " +
			"during named events. Coverage is the United States and its territories only, not other countries. " +
			"Data is reported roughly twice a day and may lag reality.",
		Example: "  shelters-pp-cli shelters --state TX\n" +
			"  shelters-pp-cli shelters --pets --json\n" +
			"  shelters-pp-cli list --ada --wheelchair",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			feed, err := loadShelterFeed(cmd, flags, flagFixture)
			if err != nil {
				return err
			}
			shelters := sf.apply(feed.Shelters)
			if flagLimit > 0 && len(shelters) > flagLimit {
				shelters = shelters[:flagLimit]
			}
			data := shelterListData{Count: len(shelters), Shelters: shelters, Enrichment: feed.Enrich, RedCross: feed.RedCross, Occupancy: feed.Occupancy, Hidden: feed.Hidden}
			return emitEnvelopeHuman(cmd, flags, feed.Source, data, func() string {
				return renderShelterTable(shelters, feed.Enrich, feed.RedCross, feed.Occupancy, feed.Hidden)
			})
		},
	}
	cmd.Flags().StringVar(&sf.state, "state", "", "Filter to a two-letter state/territory (e.g. TX)")
	cmd.Flags().BoolVar(&sf.pets, "pets", false, "Only shelters that allow pets (pet code COHABIT or ONSITE)")
	cmd.Flags().BoolVar(&sf.ada, "ada", false, "Only shelters confirmed ADA compliant")
	cmd.Flags().BoolVar(&sf.wheelchair, "wheelchair", false, "Only shelters confirmed wheelchair accessible")
	cmd.Flags().StringVar(&sf.org, "org", "", "Filter by managing organization name (substring, case-insensitive)")
	cmd.Flags().StringVar(&sf.status, "status", "", "Filter by status (e.g. OPEN). The live feed is all OPEN")
	cmd.Flags().StringVar(&sf.county, "county", "", "Filter by county/parish (substring, case-insensitive; needs FEMA_NSS/0 enrichment)")
	cmd.Flags().BoolVar(&sf.generator, "generator", false, "Only shelters with a confirmed onsite generator (needs FEMA_NSS/0 enrichment)")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Cap the number of shelters returned (0 = no cap)")
	cmd.Flags().StringVar(&flagFixture, "fixture", "", "Parse a saved feed JSON (path or - for stdin) instead of fetching live")
	return cmd
}

// renderShelterTable renders the human listing.
func renderShelterTable(shelters []Shelter, enrich, redCross, occupancy, hidden enrichState) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Open shelters: %d\n", len(shelters))
	if len(shelters) == 0 {
		fmt.Fprintln(&b, "  (none reported right now; this is normal when no disaster is active)")
		appendFeedNotes(&b, enrich, redCross, occupancy, hidden)
		return b.String()
	}
	for _, s := range shelters {
		loc := s.City
		if s.State != "" {
			loc = strings.TrimSpace(loc + ", " + s.State)
		}
		if s.CountyParish != "" {
			loc = strings.TrimSpace(loc + " (" + s.CountyParish + ")")
		}
		fmt.Fprintf(&b, "- %s (id %d) -- %s%s\n", s.Name, s.ShelterID, loc, sourceTag(s.Source))
		fmt.Fprintf(&b, "    status %s | pets %s | ada %s | wheelchair %s | pop/cap %s\n",
			dashIfEmpty(s.Status), petLabel(s.PetAccommodations),
			dashIfEmpty(s.ADACompliant), dashIfEmpty(s.WheelchairAccessible), popCapStr(s))
		if s.IncidentName != "" {
			fmt.Fprintf(&b, "    incident %s\n", s.IncidentName)
		}
	}
	appendFeedNotes(&b, enrich, redCross, occupancy, hidden)
	return b.String()
}

// sourceTag renders the per-shelter provenance for the human listing, e.g.
// " [src: redcross]". "fema" is the default spine, shown without a tag to keep
// the common case uncluttered.
func sourceTag(src string) string {
	if src == "" || src == "fema" {
		return ""
	}
	return " [src: " + src + "]"
}

// appendFeedNotes appends the side-feed notes that warrant a human mention (i.e.
// when a feed was skipped or failed); clean successes stay quiet.
func appendFeedNotes(b *strings.Builder, states ...enrichState) {
	for _, st := range states {
		if note := st.humanNote(); note != "" {
			fmt.Fprintf(b, "\n%s\n", note)
		}
	}
}

// petLabel renders the pet code in plain words for humans.
func petLabel(code string) string {
	switch normCode(code) {
	case "COHABIT":
		return "yes (with owner)"
	case "ONSITE":
		return "yes (onsite)"
	case "NONE":
		return "no"
	case "":
		return "-"
	default:
		return strings.ToLower(code)
	}
}

func dashIfEmpty(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func intPtrStr(p *int) string {
	if p == nil {
		return "-"
	}
	return strconv.Itoa(*p)
}

// popCapStr shows "pop/evac" (or post-impact when evac is null), or "-" when
// population is unknown.
func popCapStr(s Shelter) string {
	if s.TotalPopulation == nil {
		return "-"
	}
	cap := s.EvacuationCapacity
	if cap == nil {
		cap = s.PostImpactCapacity
	}
	if cap == nil {
		return strconv.Itoa(*s.TotalPopulation) + "/?"
	}
	return strconv.Itoa(*s.TotalPopulation) + "/" + strconv.Itoa(*cap)
}
