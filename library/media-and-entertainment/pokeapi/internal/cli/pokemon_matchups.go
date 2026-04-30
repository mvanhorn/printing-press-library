package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/pokeapi/internal/poke"
)

// matchupReport is the JSON shape returned by `pokemon matchups`. Defensive
// shows what hits the Pokémon and how hard, offensive shows what each of
// its attacking types can hit super-effectively.
type matchupReport struct {
	Pokemon    string                  `json:"pokemon"`
	Types      []string                `json:"types"`
	Defensive  []poke.MultiplierBucket `json:"defensive"`
	Offensive  map[string][]string     `json:"offensive"`
	HasNoTypes bool                    `json:"has_no_types,omitempty"`
}

func newPokemonMatchupsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "matchups [name]",
		Short: "Type weakness/resistance/immunity profile + offensive coverage for one Pokémon",
		Long: `Renders a Pokémon's defensive type chart (4×, 2×, ½×, ¼×, 0× incoming) plus the
super-effective coverage from its own attacking types. Useful for battle planning,
weakness audits, and answering 'what beats X?' or 'what does X beat?'.`,
		Example: strings.Trim(`
  pokeapi-pp-cli pokemon matchups charizard --json
  pokeapi-pp-cli pokemon matchups magnezone --json --select defensive
  pokeapi-pp-cli pokemon matchups gengar --json --select offensive`, "\n"),
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
			f := poke.NewLiveFetcher(c)
			ctx := context.Background()
			report, err := buildMatchups(ctx, f, args[0])
			if err != nil {
				return err
			}
			out, err := json.Marshal(report)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}

// buildMatchups computes defensive + offensive matchup data using a Fetcher.
// Extracted for sharing with `team coverage` / `team gaps`.
func buildMatchups(ctx context.Context, f *poke.LiveFetcher, name string) (*matchupReport, error) {
	types, err := f.GetPokemonTypes(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("looking up %q: %w", name, err)
	}
	report := &matchupReport{
		Pokemon: strings.ToLower(name),
		Types:   types,
	}
	if len(types) == 0 {
		report.HasNoTypes = true
		return report, nil
	}
	rels, err := poke.CollectTypeRelations(ctx, f, types)
	if err != nil {
		return nil, fmt.Errorf("fetching type relations: %w", err)
	}
	report.Defensive = poke.BucketDefensive(poke.DefensiveProfile(rels))

	// offensive: for each attacking type the Pokémon has, list defending
	// types it hits for 2× via STAB.
	report.Offensive = make(map[string][]string, len(types))
	for i, atkType := range types {
		report.Offensive[atkType] = append([]string(nil), rels[i].DoubleDamageTo...)
		sort.Strings(report.Offensive[atkType])
	}
	return report, nil
}
