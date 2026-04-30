package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type evolutionStep struct {
	Name        string           `json:"name"`
	EvolvesFrom string           `json:"evolves_from,omitempty"`
	Trigger     string           `json:"trigger,omitempty"`
	Conditions  map[string]any   `json:"conditions,omitempty"`
	Children    []*evolutionStep `json:"children,omitempty"`
}

type evolutionReport struct {
	Pokemon string         `json:"pokemon"`
	Species string         `json:"species"`
	ChainID int            `json:"chain_id"`
	Root    *evolutionStep `json:"root"`
}

func newPokemonEvolutionCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "evolution [name]",
		Short: "Render a Pokémon's full evolution chain (with triggers and conditions)",
		Long: `Resolves a Pokémon's species, fetches its evolution chain, and renders the full
tree (including branching evolutions like Eevee's nine forms) with trigger and
condition data inline.`,
		Example: strings.Trim(`
  pokeapi-pp-cli pokemon evolution eevee --json
  pokeapi-pp-cli pokemon evolution charmander --json --select root.name,root.children`, "\n"),
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
			ctx := context.Background()
			report, err := buildEvolution(ctx, c, args[0])
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

// buildEvolution resolves /pokemon/{name} → species → evolution_chain and
// returns a tree rooted at the chain's first species.
func buildEvolution(_ context.Context, c clientLike, name string) (*evolutionReport, error) {
	rawPokemon, err := c.Get(fmt.Sprintf("/api/v2/pokemon/%s/", name), nil)
	if err != nil {
		return nil, fmt.Errorf("fetching %q: %w", name, err)
	}
	var p struct {
		Species struct {
			Name string `json:"name"`
		} `json:"species"`
	}
	if err := json.Unmarshal(rawPokemon, &p); err != nil {
		return nil, fmt.Errorf("decoding pokemon %q: %w", name, err)
	}
	if p.Species.Name == "" {
		return nil, fmt.Errorf("pokemon %q has no species", name)
	}
	rawSpecies, err := c.Get(fmt.Sprintf("/api/v2/pokemon-species/%s/", p.Species.Name), nil)
	if err != nil {
		return nil, fmt.Errorf("fetching species %q: %w", p.Species.Name, err)
	}
	var sp struct {
		EvolutionChain struct {
			URL string `json:"url"`
		} `json:"evolution_chain"`
	}
	if err := json.Unmarshal(rawSpecies, &sp); err != nil {
		return nil, fmt.Errorf("decoding species %q: %w", p.Species.Name, err)
	}
	chainID := extractIDFromURL(sp.EvolutionChain.URL)
	if chainID == 0 {
		return nil, fmt.Errorf("species %q has no evolution chain", p.Species.Name)
	}
	rawChain, err := c.Get(fmt.Sprintf("/api/v2/evolution-chain/%d/", chainID), nil)
	if err != nil {
		return nil, fmt.Errorf("fetching evolution chain %d: %w", chainID, err)
	}
	var chain struct {
		ID    int          `json:"id"`
		Chain chainLinkRaw `json:"chain"`
	}
	if err := json.Unmarshal(rawChain, &chain); err != nil {
		return nil, fmt.Errorf("decoding chain: %w", err)
	}
	root := convertChainLink(chain.Chain, "")
	return &evolutionReport{
		Pokemon: strings.ToLower(name),
		Species: p.Species.Name,
		ChainID: chain.ID,
		Root:    root,
	}, nil
}

// chainLinkRaw mirrors the recursive evolution_chain.chain payload.
type chainLinkRaw struct {
	Species struct {
		Name string `json:"name"`
	} `json:"species"`
	EvolutionDetails []struct {
		Trigger struct {
			Name string `json:"name"`
		} `json:"trigger"`
		MinLevel              *int                   `json:"min_level"`
		Item                  *struct{ Name string } `json:"item"`
		HeldItem              *struct{ Name string } `json:"held_item"`
		MinHappiness          *int                   `json:"min_happiness"`
		MinAffection          *int                   `json:"min_affection"`
		MinBeauty             *int                   `json:"min_beauty"`
		KnownMove             *struct{ Name string } `json:"known_move"`
		KnownMoveType         *struct{ Name string } `json:"known_move_type"`
		Location              *struct{ Name string } `json:"location"`
		NeedsOverworldRain    bool                   `json:"needs_overworld_rain"`
		PartySpecies          *struct{ Name string } `json:"party_species"`
		PartyType             *struct{ Name string } `json:"party_type"`
		RelativePhysicalStats *int                   `json:"relative_physical_stats"`
		TimeOfDay             string                 `json:"time_of_day"`
		TradeSpecies          *struct{ Name string } `json:"trade_species"`
		TurnUpsideDown        bool                   `json:"turn_upside_down"`
		Gender                *int                   `json:"gender"`
	} `json:"evolution_details"`
	EvolvesTo []chainLinkRaw `json:"evolves_to"`
}

func convertChainLink(link chainLinkRaw, parent string) *evolutionStep {
	step := &evolutionStep{
		Name:        link.Species.Name,
		EvolvesFrom: parent,
	}
	if len(link.EvolutionDetails) > 0 {
		// Render the first evolution detail. Most species have one; rare cases
		// (Wurmple) have multiple — we surface the first and the agent can use
		// `pokeapi-pp-cli evolution-chain retrieve` for the full graph.
		d := link.EvolutionDetails[0]
		step.Trigger = d.Trigger.Name
		conds := map[string]any{}
		if d.MinLevel != nil {
			conds["min_level"] = *d.MinLevel
		}
		if d.Item != nil && d.Item.Name != "" {
			conds["item"] = d.Item.Name
		}
		if d.HeldItem != nil && d.HeldItem.Name != "" {
			conds["held_item"] = d.HeldItem.Name
		}
		if d.MinHappiness != nil {
			conds["min_happiness"] = *d.MinHappiness
		}
		if d.MinAffection != nil {
			conds["min_affection"] = *d.MinAffection
		}
		if d.MinBeauty != nil {
			conds["min_beauty"] = *d.MinBeauty
		}
		if d.KnownMove != nil && d.KnownMove.Name != "" {
			conds["known_move"] = d.KnownMove.Name
		}
		if d.KnownMoveType != nil && d.KnownMoveType.Name != "" {
			conds["known_move_type"] = d.KnownMoveType.Name
		}
		if d.Location != nil && d.Location.Name != "" {
			conds["location"] = d.Location.Name
		}
		if d.TimeOfDay != "" {
			conds["time_of_day"] = d.TimeOfDay
		}
		if d.NeedsOverworldRain {
			conds["needs_overworld_rain"] = true
		}
		if d.TurnUpsideDown {
			conds["turn_upside_down"] = true
		}
		if d.PartySpecies != nil && d.PartySpecies.Name != "" {
			conds["party_species"] = d.PartySpecies.Name
		}
		if d.PartyType != nil && d.PartyType.Name != "" {
			conds["party_type"] = d.PartyType.Name
		}
		if d.RelativePhysicalStats != nil {
			conds["relative_physical_stats"] = *d.RelativePhysicalStats
		}
		if d.TradeSpecies != nil && d.TradeSpecies.Name != "" {
			conds["trade_species"] = d.TradeSpecies.Name
		}
		if d.Gender != nil {
			conds["gender"] = *d.Gender
		}
		if len(conds) > 0 {
			step.Conditions = conds
		}
	}
	for _, child := range link.EvolvesTo {
		step.Children = append(step.Children, convertChainLink(child, link.Species.Name))
	}
	return step
}

// extractIDFromURL pulls the trailing integer ID off a PokéAPI resource URL
// like https://pokeapi.co/api/v2/evolution-chain/67/. Returns 0 on failure.
func extractIDFromURL(u string) int {
	u = strings.TrimRight(u, "/")
	idx := strings.LastIndex(u, "/")
	if idx < 0 {
		return 0
	}
	tail := u[idx+1:]
	var id int
	if _, err := fmt.Sscanf(tail, "%d", &id); err != nil {
		return 0
	}
	return id
}
