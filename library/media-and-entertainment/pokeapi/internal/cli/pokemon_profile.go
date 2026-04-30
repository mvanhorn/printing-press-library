package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/pokeapi/internal/poke"
)

// pokemonProfile is the agent-friendly summary surface for a single Pokémon.
// Combines core pokemon + species + types + abilities + stats + move count
// in one structured payload, replacing the typical 3-7 sequential API calls
// an agent would make to assemble the same view.
type pokemonProfile struct {
	Name        string             `json:"name"`
	ID          int                `json:"id"`
	Types       []string           `json:"types"`
	Stats       map[string]int     `json:"stats"`
	Abilities   []abilityRef       `json:"abilities"`
	HeightDM    int                `json:"height_decimeters"`
	WeightHG    int                `json:"weight_hectograms"`
	BaseXP      int                `json:"base_experience"`
	MoveCount   int                `json:"move_count"`
	SignatureID string             `json:"-"` // unused, reserved
	Species     *speciesProfileRef `json:"species,omitempty"`
}

type abilityRef struct {
	Name     string `json:"name"`
	IsHidden bool   `json:"is_hidden"`
	Slot     int    `json:"slot"`
}

type speciesProfileRef struct {
	Name           string `json:"name"`
	GrowthRate     string `json:"growth_rate,omitempty"`
	BaseHappiness  int    `json:"base_happiness"`
	CaptureRate    int    `json:"capture_rate"`
	IsLegendary    bool   `json:"is_legendary"`
	IsMythical     bool   `json:"is_mythical"`
	IsBaby         bool   `json:"is_baby"`
	HabitatName    string `json:"habitat,omitempty"`
	GenerationName string `json:"generation,omitempty"`
}

func newPokemonProfileCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile [name]",
		Short: "Agent-ready Pokémon profile (types, stats, abilities, species, move count) in one structured payload",
		Long: `Combines core pokemon, species, types, abilities, stats, and move count into a single
structured payload. Replaces the 3-7 sequential API calls an agent would otherwise make.`,
		Example: strings.Trim(`
  pokeapi-pp-cli pokemon profile pikachu --json
  pokeapi-pp-cli pokemon profile charizard --json --select name,types,stats
  pokeapi-pp-cli pokemon profile mewtwo --json --select name,species.is_legendary`, "\n"),
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
			profile, err := buildPokemonProfile(ctx, c, f, args[0])
			if err != nil {
				return err
			}
			out, err := json.Marshal(profile)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}

// buildPokemonProfile assembles a profile by chaining /pokemon/{name}/ then
// /pokemon-species/{name}/ (the species link inside the pokemon payload).
// Failures fetching the species are non-fatal — the profile drops the species
// block but still returns the core pokemon data.
func buildPokemonProfile(ctx context.Context, c clientLike, _ *poke.LiveFetcher, name string) (*pokemonProfile, error) {
	rawPokemon, err := c.Get(fmt.Sprintf("/api/v2/pokemon/%s/", name), nil)
	if err != nil {
		return nil, fmt.Errorf("fetching pokemon %q: %w", name, err)
	}
	var p struct {
		ID             int    `json:"id"`
		Name           string `json:"name"`
		BaseExperience int    `json:"base_experience"`
		Height         int    `json:"height"`
		Weight         int    `json:"weight"`
		Types          []struct {
			Slot int `json:"slot"`
			Type struct {
				Name string `json:"name"`
			} `json:"type"`
		} `json:"types"`
		Abilities []struct {
			IsHidden bool `json:"is_hidden"`
			Slot     int  `json:"slot"`
			Ability  struct {
				Name string `json:"name"`
			} `json:"ability"`
		} `json:"abilities"`
		Stats []struct {
			BaseStat int `json:"base_stat"`
			Stat     struct {
				Name string `json:"name"`
			} `json:"stat"`
		} `json:"stats"`
		Moves   []json.RawMessage `json:"moves"`
		Species struct {
			Name string `json:"name"`
		} `json:"species"`
	}
	if err := json.Unmarshal(rawPokemon, &p); err != nil {
		return nil, fmt.Errorf("decoding pokemon %q: %w", name, err)
	}
	prof := &pokemonProfile{
		Name:      p.Name,
		ID:        p.ID,
		HeightDM:  p.Height,
		WeightHG:  p.Weight,
		BaseXP:    p.BaseExperience,
		MoveCount: len(p.Moves),
		Types:     make([]string, 0, len(p.Types)),
		Stats:     make(map[string]int, len(p.Stats)),
		Abilities: make([]abilityRef, 0, len(p.Abilities)),
	}
	for _, t := range p.Types {
		prof.Types = append(prof.Types, t.Type.Name)
	}
	for _, s := range p.Stats {
		prof.Stats[s.Stat.Name] = s.BaseStat
	}
	for _, a := range p.Abilities {
		prof.Abilities = append(prof.Abilities, abilityRef{
			Name:     a.Ability.Name,
			IsHidden: a.IsHidden,
			Slot:     a.Slot,
		})
	}
	if p.Species.Name != "" {
		if sp, err := fetchSpeciesProfile(c, p.Species.Name); err == nil {
			prof.Species = sp
		}
	}
	return prof, nil
}

func fetchSpeciesProfile(c clientLike, name string) (*speciesProfileRef, error) {
	raw, err := c.Get(fmt.Sprintf("/api/v2/pokemon-species/%s/", name), nil)
	if err != nil {
		return nil, err
	}
	var s struct {
		Name          string                 `json:"name"`
		BaseHappiness int                    `json:"base_happiness"`
		CaptureRate   int                    `json:"capture_rate"`
		IsBaby        bool                   `json:"is_baby"`
		IsLegendary   bool                   `json:"is_legendary"`
		IsMythical    bool                   `json:"is_mythical"`
		GrowthRate    *struct{ Name string } `json:"growth_rate"`
		Habitat       *struct{ Name string } `json:"habitat"`
		Generation    *struct{ Name string } `json:"generation"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	out := &speciesProfileRef{
		Name:          s.Name,
		BaseHappiness: s.BaseHappiness,
		CaptureRate:   s.CaptureRate,
		IsLegendary:   s.IsLegendary,
		IsMythical:    s.IsMythical,
		IsBaby:        s.IsBaby,
	}
	if s.GrowthRate != nil {
		out.GrowthRate = s.GrowthRate.Name
	}
	if s.Habitat != nil {
		out.HabitatName = s.Habitat.Name
	}
	if s.Generation != nil {
		out.GenerationName = s.Generation.Name
	}
	return out, nil
}

// clientLike narrows the client surface to just Get for testability and so we
// can pass the *client.Client around without exporting it through poke.
type clientLike interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
}
