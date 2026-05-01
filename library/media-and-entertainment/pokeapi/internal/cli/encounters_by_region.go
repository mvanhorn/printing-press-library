package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type regionEncounterRow struct {
	Pokemon      string   `json:"pokemon"`
	LocationArea string   `json:"location_area"`
	Location     string   `json:"location"`
	Methods      []string `json:"methods"`
	Versions     []string `json:"versions"`
	MinLevel     int      `json:"min_level"`
	MaxLevel     int      `json:"max_level"`
	MaxChance    int      `json:"max_chance"`
}

type encountersByRegionReport struct {
	Region    string               `json:"region"`
	Locations int                  `json:"locations_scanned"`
	Areas     int                  `json:"areas_scanned"`
	Rows      []regionEncounterRow `json:"rows"`
	Total     int                  `json:"total"`
	Notes     []string             `json:"notes,omitempty"`
}

func newEncountersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "encounters",
		Short: "Encounter queries",
	}
	cmd.AddCommand(newEncountersByRegionCmd(flags))
	cmd.AddCommand(newEncountersByRegionCmd(flags))
	return cmd
}

func newEncountersByRegionCmd(flags *rootFlags) *cobra.Command {
	var (
		versionFilter string
		limit         int
	)
	cmd := &cobra.Command{
		Use:   "by-region [region]",
		Short: "Render the encounter table for an entire region (joins location ↔ area ↔ encounters ↔ species)",
		Long: `For each location in the region (kanto, johto, hoenn, sinnoh, unova, kalos, alola,
galar, hisui, paldea), fetches every location-area, then every encounter on each
area. Returns a flat table of (pokemon × location × method × version × level range).

This is impossible against the live API in fewer than ~40 calls per region; the CLI
issues all the calls in one run with in-process memoization.`,
		Example: strings.Trim(`
  pokeapi-pp-cli encounters by-region kanto --json
  pokeapi-pp-cli encounters by-region johto --version gold --json --limit 100`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			region := strings.ToLower(strings.TrimSpace(args[0]))
			report, err := buildRegionEncounters(c, region, versionFilter, limit)
			if err != nil {
				return err
			}
			b, err := json.Marshal(report)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
		},
	}
	cmd.Flags().StringVar(&versionFilter, "version", "", "Filter to a single game version (e.g. red, blue, gold, ruby)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum rows to return (default 50; pass 0 for no cap, but large regions can take a minute)")
	return cmd
}

func buildRegionEncounters(c clientLike, region, versionFilter string, limit int) (*encountersByRegionReport, error) {
	report := &encountersByRegionReport{Region: region}
	rawRegion, err := c.Get(fmt.Sprintf("/api/v2/region/%s/", region), nil)
	if err != nil {
		return nil, classifyAPIError(err)
	}
	var rdoc struct {
		Locations []struct {
			Name string `json:"name"`
		} `json:"locations"`
	}
	if err := json.Unmarshal(rawRegion, &rdoc); err != nil {
		return nil, fmt.Errorf("decoding region %q: %w", region, err)
	}
	report.Locations = len(rdoc.Locations)

	for _, loc := range rdoc.Locations {
		rawLoc, err := c.Get(fmt.Sprintf("/api/v2/location/%s/", loc.Name), nil)
		if err != nil {
			continue
		}
		var ldoc struct {
			Areas []struct {
				Name string `json:"name"`
			} `json:"areas"`
		}
		if err := json.Unmarshal(rawLoc, &ldoc); err != nil {
			continue
		}
		for _, area := range ldoc.Areas {
			report.Areas++
			rawArea, err := c.Get(fmt.Sprintf("/api/v2/location-area/%s/", area.Name), nil)
			if err != nil {
				continue
			}
			var adoc struct {
				PokemonEncounters []struct {
					Pokemon struct {
						Name string `json:"name"`
					} `json:"pokemon"`
					VersionDetails []struct {
						Version struct {
							Name string `json:"name"`
						} `json:"version"`
						MaxChance        int `json:"max_chance"`
						EncounterDetails []struct {
							MinLevel int `json:"min_level"`
							MaxLevel int `json:"max_level"`
							Method   struct {
								Name string `json:"name"`
							} `json:"method"`
						} `json:"encounter_details"`
					} `json:"version_details"`
				} `json:"pokemon_encounters"`
			}
			if err := json.Unmarshal(rawArea, &adoc); err != nil {
				continue
			}
			for _, pe := range adoc.PokemonEncounters {
				row := regionEncounterRow{
					Pokemon:      pe.Pokemon.Name,
					LocationArea: area.Name,
					Location:     loc.Name,
				}
				methods := make(map[string]bool)
				versions := make(map[string]bool)
				for _, vd := range pe.VersionDetails {
					if versionFilter != "" && vd.Version.Name != versionFilter {
						continue
					}
					if vd.MaxChance > row.MaxChance {
						row.MaxChance = vd.MaxChance
					}
					versions[vd.Version.Name] = true
					for _, ed := range vd.EncounterDetails {
						methods[ed.Method.Name] = true
						if row.MinLevel == 0 || ed.MinLevel < row.MinLevel {
							row.MinLevel = ed.MinLevel
						}
						if ed.MaxLevel > row.MaxLevel {
							row.MaxLevel = ed.MaxLevel
						}
					}
				}
				if versionFilter != "" && len(versions) == 0 {
					continue
				}
				for m := range methods {
					row.Methods = append(row.Methods, m)
				}
				for v := range versions {
					row.Versions = append(row.Versions, v)
				}
				sort.Strings(row.Methods)
				sort.Strings(row.Versions)
				report.Rows = append(report.Rows, row)
				if limit > 0 && len(report.Rows) >= limit {
					goto done
				}
			}
		}
	}
done:
	report.Total = len(report.Rows)
	return report, nil
}
