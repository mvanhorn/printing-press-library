package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type pokemonFormSummary struct {
	Name      string         `json:"name"`
	IsDefault bool           `json:"is_default"`
	Types     []string       `json:"types"`
	Stats     map[string]int `json:"stats"`
	Abilities []string       `json:"abilities"`
}

type pokemonFormsReport struct {
	Species string               `json:"species"`
	Forms   []pokemonFormSummary `json:"forms"`
	Total   int                  `json:"total"`
}

func newPokemonFormsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "forms [species]",
		Short: "List every form for a species (regional forms, megas, gigantamax) with type/stat/ability deltas",
		Long: `PokéAPI tracks each form (Vulpix, Alolan Vulpix; Charizard, Mega-Charizard-X,
Mega-Charizard-Y, Gigantamax-Charizard) as a separate /pokemon/{name}/ entry but
groups them under one /pokemon-species/{name}/. This command resolves the species
and renders type/stat/ability for every form side-by-side.`,
		Example: strings.Trim(`
  pokeapi-pp-cli pokemon forms vulpix --json
  pokeapi-pp-cli pokemon forms charizard --json --select forms.name,forms.types,forms.stats`, "\n"),
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
			speciesArg := strings.ToLower(strings.TrimSpace(args[0]))
			raw, err := c.Get(fmt.Sprintf("/api/v2/pokemon-species/%s/", speciesArg), nil)
			if err != nil {
				return classifyAPIError(err)
			}
			var doc struct {
				Name      string `json:"name"`
				Varieties []struct {
					IsDefault bool `json:"is_default"`
					Pokemon   struct {
						Name string `json:"name"`
					} `json:"pokemon"`
				} `json:"varieties"`
			}
			if err := json.Unmarshal(raw, &doc); err != nil {
				return fmt.Errorf("decoding species %q: %w", speciesArg, err)
			}
			report := &pokemonFormsReport{
				Species: doc.Name,
				Forms:   make([]pokemonFormSummary, 0, len(doc.Varieties)),
			}
			for _, v := range doc.Varieties {
				summary, err := fetchFormSummary(c, v.Pokemon.Name, v.IsDefault)
				if err != nil {
					continue
				}
				report.Forms = append(report.Forms, *summary)
			}
			report.Total = len(report.Forms)
			b, err := json.Marshal(report)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
		},
	}
	return cmd
}

func fetchFormSummary(c clientLike, name string, isDefault bool) (*pokemonFormSummary, error) {
	raw, err := c.Get(fmt.Sprintf("/api/v2/pokemon/%s/", name), nil)
	if err != nil {
		return nil, err
	}
	var p struct {
		Name  string `json:"name"`
		Types []struct {
			Type struct {
				Name string `json:"name"`
			} `json:"type"`
		} `json:"types"`
		Abilities []struct {
			Ability struct {
				Name string `json:"name"`
			} `json:"ability"`
		} `json:"abilities"`
		Stats []struct {
			BaseStat int `json:"base_stat"`
			Stat     struct {
				Name string `json:"name"`
			} `json:"stat"`
		} `json:"stats"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	out := &pokemonFormSummary{
		Name:      p.Name,
		IsDefault: isDefault,
		Types:     make([]string, 0, len(p.Types)),
		Stats:     make(map[string]int, len(p.Stats)),
		Abilities: make([]string, 0, len(p.Abilities)),
	}
	for _, t := range p.Types {
		out.Types = append(out.Types, t.Type.Name)
	}
	for _, a := range p.Abilities {
		out.Abilities = append(out.Abilities, a.Ability.Name)
	}
	for _, s := range p.Stats {
		out.Stats[s.Stat.Name] = s.BaseStat
	}
	return out, nil
}
