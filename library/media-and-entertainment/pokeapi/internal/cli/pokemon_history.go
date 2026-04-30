package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type pokemonHistoryReport struct {
	Pokemon            string             `json:"pokemon"`
	IntroducedIn       string             `json:"introduced_in,omitempty"`
	CurrentTypes       []string           `json:"current_types"`
	CurrentAbilities   []string           `json:"current_abilities"`
	PastTypeChanges    []pastTypeEntry    `json:"past_type_changes,omitempty"`
	PastAbilityChanges []pastAbilityEntry `json:"past_ability_changes,omitempty"`
	PastStatChanges    []pastStatEntry    `json:"past_stat_changes,omitempty"`
}

type pastTypeEntry struct {
	UntilGeneration string   `json:"until_generation"`
	Types           []string `json:"types"`
}

type pastAbilityEntry struct {
	UntilGeneration string   `json:"until_generation"`
	Abilities       []string `json:"abilities"`
}

type pastStatEntry struct {
	UntilGeneration string         `json:"until_generation"`
	Stats           map[string]int `json:"stats"`
}

func newPokemonHistoryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history [name]",
		Short: "Cross-generation timeline of a Pokémon: type changes, stat changes, ability changes",
		Long: `Stitches together pokemon-species generation, current pokemon types/abilities/stats,
and the past_types/past_abilities/past_stats records into a single timeline. Answers
questions like 'was Clefairy always a Fairy type?' (no — Fairy added in gen 6) or
'has Pikachu's stat spread changed?' in one command.`,
		Example: strings.Trim(`
  pokeapi-pp-cli pokemon history clefairy --json
  pokeapi-pp-cli pokemon history butterfree --json --select past_type_changes`, "\n"),
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
			name := strings.ToLower(strings.TrimSpace(args[0]))
			rawPokemon, err := c.Get(fmt.Sprintf("/api/v2/pokemon/%s/", name), nil)
			if err != nil {
				return classifyAPIError(err)
			}
			var p struct {
				Name  string `json:"name"`
				Types []struct {
					Type struct{ Name string } `json:"type"`
				} `json:"types"`
				Abilities []struct {
					Ability struct{ Name string } `json:"ability"`
				} `json:"abilities"`
				Species struct {
					Name string `json:"name"`
				} `json:"species"`
				PastTypes []struct {
					Generation struct{ Name string } `json:"generation"`
					Types      []struct {
						Type struct{ Name string } `json:"type"`
					} `json:"types"`
				} `json:"past_types"`
				PastAbilities []struct {
					Generation struct{ Name string } `json:"generation"`
					Abilities  []struct {
						Ability *struct{ Name string } `json:"ability"`
					} `json:"abilities"`
				} `json:"past_abilities"`
				PastStats []struct {
					Generation struct{ Name string } `json:"generation"`
					Stats      []struct {
						BaseStat int                   `json:"base_stat"`
						Stat     struct{ Name string } `json:"stat"`
					} `json:"stats"`
				} `json:"past_stats"`
			}
			if err := json.Unmarshal(rawPokemon, &p); err != nil {
				return fmt.Errorf("decoding %q: %w", name, err)
			}
			report := &pokemonHistoryReport{
				Pokemon:          p.Name,
				CurrentTypes:     extractTypes(p.Types),
				CurrentAbilities: extractAbilities(p.Abilities),
			}
			// Resolve introduced_in via species.generation
			if p.Species.Name != "" {
				if rawSp, err := c.Get(fmt.Sprintf("/api/v2/pokemon-species/%s/", p.Species.Name), nil); err == nil {
					var sp struct {
						Generation *struct{ Name string } `json:"generation"`
					}
					if json.Unmarshal(rawSp, &sp) == nil && sp.Generation != nil {
						report.IntroducedIn = sp.Generation.Name
					}
				}
			}
			for _, pt := range p.PastTypes {
				e := pastTypeEntry{UntilGeneration: pt.Generation.Name}
				for _, t := range pt.Types {
					e.Types = append(e.Types, t.Type.Name)
				}
				report.PastTypeChanges = append(report.PastTypeChanges, e)
			}
			for _, pa := range p.PastAbilities {
				e := pastAbilityEntry{UntilGeneration: pa.Generation.Name}
				for _, a := range pa.Abilities {
					if a.Ability != nil {
						e.Abilities = append(e.Abilities, a.Ability.Name)
					}
				}
				report.PastAbilityChanges = append(report.PastAbilityChanges, e)
			}
			for _, ps := range p.PastStats {
				e := pastStatEntry{UntilGeneration: ps.Generation.Name, Stats: make(map[string]int, len(ps.Stats))}
				for _, st := range ps.Stats {
					e.Stats[st.Stat.Name] = st.BaseStat
				}
				report.PastStatChanges = append(report.PastStatChanges, e)
			}
			sort.Slice(report.PastTypeChanges, func(i, j int) bool {
				return report.PastTypeChanges[i].UntilGeneration < report.PastTypeChanges[j].UntilGeneration
			})
			b, err := json.Marshal(report)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
		},
	}
	return cmd
}

func extractTypes(types []struct {
	Type struct{ Name string } `json:"type"`
}) []string {
	out := make([]string, 0, len(types))
	for _, t := range types {
		out = append(out, t.Type.Name)
	}
	return out
}

func extractAbilities(abs []struct {
	Ability struct{ Name string } `json:"ability"`
}) []string {
	out := make([]string, 0, len(abs))
	for _, a := range abs {
		out = append(out, a.Ability.Name)
	}
	return out
}
