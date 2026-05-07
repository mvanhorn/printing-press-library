package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

func newCrossoverCmd(flags *rootFlags) *cobra.Command {
	var (
		anchor   string
		radius   string // string so users can write "800m" or "1.2km"
		pair     string
		minTrust float64
	)
	cmd := &cobra.Command{
		Use:   "crossover",
		Short: "Find pairs where a high-trust restaurant sits within 200m of a Wikipedia-notable historic site or Atlas Obscura entry — food + culture in one walk.",
		Long: `Spatial cross-entity SQL: pairs places where the first member matches one
intent (default: food) and the second matches another (default: culture or
historic), within --pair-distance meters of each other AND within the
--radius meters of the anchor. Ranked by combined trust × proximity.`,
		Example: strings.Trim(`
  # Default food+culture pairing in Marais
  wanderlust-goat-pp-cli crossover --anchor "Marais, Paris" --radius 800m

  # Food + viewpoint pairing
  wanderlust-goat-pp-cli crossover --anchor "Asakusa, Tokyo" --radius 1km --pair food+viewpoint

  # Agent JSON for pipelines
  wanderlust-goat-pp-cli crossover --anchor "Bukchon, Seoul" --radius 1.2km --pair food+culture --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if anchor == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			res, err := resolveAnchor(ctx, anchor)
			if err != nil {
				return err
			}
			rmeters, err := parseDistance(radius)
			if err != nil {
				return err
			}
			intentA, intentB, err := parsePair(pair)
			if err != nil {
				return err
			}
			store, err := openGoatStore(cmd, flags)
			if err != nil {
				return err
			}
			defer store.Close()

			candidatesA, err := store.QueryRadius(ctx, res.Lat, res.Lng, float64(rmeters), intentA)
			if err != nil {
				return err
			}
			candidatesB, err := store.QueryRadius(ctx, res.Lat, res.Lng, float64(rmeters), intentB)
			if err != nil {
				return err
			}

			pairDist := 200.0 // brief: "within 200m"
			report := crossoverReport{
				Anchor: res, Radius: rmeters, Pair: pair, PairDistanceMeters: pairDist,
			}
			for _, a := range candidatesA {
				if a.Trust < minTrust {
					continue
				}
				for _, b := range candidatesB {
					if b.Trust < minTrust {
						continue
					}
					if a.ID == b.ID {
						continue
					}
					d := haversineMeters(a.Lat, a.Lng, b.Lat, b.Lng)
					if d > pairDist {
						continue
					}
					report.Pairs = append(report.Pairs, crossoverPair{
						A:              pairMember{Name: a.Name, NameLocal: a.NameLocal, Source: a.Source, Intent: a.Intent, Trust: a.Trust, Lat: a.Lat, Lng: a.Lng, WhySpecial: a.WhySpecial},
						B:              pairMember{Name: b.Name, NameLocal: b.NameLocal, Source: b.Source, Intent: b.Intent, Trust: b.Trust, Lat: b.Lat, Lng: b.Lng, WhySpecial: b.WhySpecial},
						DistanceMeters: d,
						CombinedTrust:  a.Trust + b.Trust,
					})
				}
			}
			if len(report.Pairs) == 0 {
				report.Note = "No qualifying pairs in the local store. Run 'sync-city <slug>' to populate places, or widen --radius."
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}
	cmd.Flags().StringVar(&anchor, "anchor", "", "Anchor address or \"lat,lng\".")
	cmd.Flags().StringVar(&radius, "radius", "800m", "Search radius (e.g. 800m, 1km, 1.2km).")
	cmd.Flags().StringVar(&pair, "pair", "food+culture", "Intent pair: food+culture, food+historic, food+viewpoint.")
	cmd.Flags().Float64Var(&minTrust, "min-trust", 0.85, "Minimum trust for both members (default 0.85).")
	return cmd
}

type crossoverReport struct {
	Anchor             AnchorResolution `json:"anchor"`
	Radius             int              `json:"radius_meters"`
	Pair               string           `json:"pair"`
	PairDistanceMeters float64          `json:"pair_distance_meters"`
	Pairs              []crossoverPair  `json:"pairs"`
	Note               string           `json:"note,omitempty"`
}

type crossoverPair struct {
	A              pairMember `json:"a"`
	B              pairMember `json:"b"`
	DistanceMeters float64    `json:"distance_meters"`
	CombinedTrust  float64    `json:"combined_trust"`
}

type pairMember struct {
	Name       string  `json:"name"`
	NameLocal  string  `json:"name_local,omitempty"`
	Source     string  `json:"source"`
	Intent     string  `json:"intent"`
	Trust      float64 `json:"trust"`
	Lat        float64 `json:"lat"`
	Lng        float64 `json:"lng"`
	WhySpecial string  `json:"why_special,omitempty"`
}

func parseDistance(s string) (int, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 800, nil
	}
	switch {
	case strings.HasSuffix(s, "km"):
		f, err := strconvParseFloat(strings.TrimSuffix(s, "km"))
		if err != nil {
			return 0, err
		}
		return int(f * 1000), nil
	case strings.HasSuffix(s, "m"):
		f, err := strconvParseFloat(strings.TrimSuffix(s, "m"))
		if err != nil {
			return 0, err
		}
		return int(f), nil
	}
	f, err := strconvParseFloat(s)
	if err != nil {
		return 0, err
	}
	return int(f), nil
}

func parsePair(s string) (string, string, error) {
	parts := strings.Split(s, "+")
	if len(parts) != 2 {
		return "", "", strErr("pair must be \"a+b\" (e.g. food+culture)")
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}
