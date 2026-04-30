package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type abilityHolder struct {
	Pokemon  string `json:"pokemon"`
	IsHidden bool   `json:"is_hidden"`
	Slot     int    `json:"slot"`
}

type byAbilityReport struct {
	Ability string          `json:"ability"`
	Total   int             `json:"total"`
	Holders []abilityHolder `json:"holders"`
	Effect  string          `json:"effect,omitempty"`
}

func newPokemonByAbilityCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "by-ability [ability]",
		Aliases: []string{"with-ability"},
		Short:   "Find every Pokémon with a given ability",
		Long: `PokéAPI exposes /ability/{name}/ with a 'pokemon' array of every species that
can have the ability — so this command does one API call rather than scanning the
whole Pokédex. Each entry tags whether the ability is regular or hidden.`,
		Example: strings.Trim(`
  pokeapi-pp-cli pokemon by-ability levitate --json
  pokeapi-pp-cli pokemon by-ability wonder-guard --json --select holders
  pokeapi-pp-cli pokemon by-ability intimidate --json --select holders.pokemon`, "\n"),
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
			ability := strings.ToLower(strings.TrimSpace(args[0]))
			raw, err := c.Get(fmt.Sprintf("/api/v2/ability/%s/", ability), nil)
			if err != nil {
				return classifyAPIError(err)
			}
			var doc struct {
				Name    string `json:"name"`
				Pokemon []struct {
					IsHidden bool `json:"is_hidden"`
					Slot     int  `json:"slot"`
					Pokemon  struct {
						Name string `json:"name"`
					} `json:"pokemon"`
				} `json:"pokemon"`
				EffectEntries []struct {
					ShortEffect string `json:"short_effect"`
					Language    struct {
						Name string `json:"name"`
					} `json:"language"`
				} `json:"effect_entries"`
			}
			if err := json.Unmarshal(raw, &doc); err != nil {
				return fmt.Errorf("decoding ability %q: %w", ability, err)
			}
			report := &byAbilityReport{
				Ability: doc.Name,
				Total:   len(doc.Pokemon),
				Holders: make([]abilityHolder, 0, len(doc.Pokemon)),
			}
			for _, p := range doc.Pokemon {
				report.Holders = append(report.Holders, abilityHolder{
					Pokemon:  p.Pokemon.Name,
					IsHidden: p.IsHidden,
					Slot:     p.Slot,
				})
			}
			sort.Slice(report.Holders, func(i, j int) bool {
				return report.Holders[i].Pokemon < report.Holders[j].Pokemon
			})
			for _, e := range doc.EffectEntries {
				if e.Language.Name == "en" {
					report.Effect = strings.TrimSpace(e.ShortEffect)
					break
				}
			}
			b, err := json.Marshal(report)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
		},
	}
	return cmd
}
