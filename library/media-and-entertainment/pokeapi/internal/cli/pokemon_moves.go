package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type pokemonMove struct {
	Move           string `json:"move"`
	LearnMethod    string `json:"learn_method"`
	VersionGroup   string `json:"version_group"`
	LevelLearnedAt int    `json:"level_learned_at"`
}

type pokemonMovesReport struct {
	Pokemon string        `json:"pokemon"`
	Filters movesFilters  `json:"filters"`
	Moves   []pokemonMove `json:"moves"`
	Total   int           `json:"total"`
}

type movesFilters struct {
	Method       string `json:"method,omitempty"`
	VersionGroup string `json:"version_group,omitempty"`
	MaxLevel     int    `json:"max_level,omitempty"`
	MinLevel     int    `json:"min_level,omitempty"`
}

func newPokemonMovesCmd(flags *rootFlags) *cobra.Command {
	var (
		method       string
		versionGroup string
		maxLevel     int
		minLevel     int
	)
	cmd := &cobra.Command{
		Use:   "moves [name]",
		Short: "List a Pokémon's moves filtered by learn method, version group, and level",
		Long: `Pulls the full learnset for a Pokémon and filters by --method (e.g. level-up,
machine, egg, tutor), --version-group (e.g. red-blue, scarlet-violet), and a level
range. The --max-level/--min-level filters apply only to level-up moves; they're
ignored for other learn methods.`,
		Example: strings.Trim(`
  pokeapi-pp-cli pokemon moves bulbasaur --method level-up --version-group red-blue --json
  pokeapi-pp-cli pokemon moves charizard --method egg --json
  pokeapi-pp-cli pokemon moves pikachu --method level-up --max-level 30 --json`, "\n"),
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
			report, err := buildPokemonMoves(ctx, c, args[0], movesFilters{
				Method:       method,
				VersionGroup: versionGroup,
				MaxLevel:     maxLevel,
				MinLevel:     minLevel,
			})
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
	cmd.Flags().StringVar(&method, "method", "", "Filter to a single learn method (e.g. level-up, machine, egg, tutor)")
	cmd.Flags().StringVar(&versionGroup, "version-group", "", "Filter to one version group (e.g. red-blue, scarlet-violet)")
	cmd.Flags().IntVar(&maxLevel, "max-level", 0, "Maximum level for level-up moves (0 = no cap)")
	cmd.Flags().IntVar(&minLevel, "min-level", 0, "Minimum level for level-up moves (0 = no floor)")
	return cmd
}

func buildPokemonMoves(_ context.Context, c clientLike, name string, filters movesFilters) (*pokemonMovesReport, error) {
	raw, err := c.Get(fmt.Sprintf("/api/v2/pokemon/%s/", name), nil)
	if err != nil {
		return nil, fmt.Errorf("fetching %q: %w", name, err)
	}
	var doc struct {
		Moves []struct {
			Move struct {
				Name string `json:"name"`
			} `json:"move"`
			VersionGroupDetails []struct {
				LevelLearnedAt  int `json:"level_learned_at"`
				MoveLearnMethod struct {
					Name string `json:"name"`
				} `json:"move_learn_method"`
				VersionGroup struct {
					Name string `json:"name"`
				} `json:"version_group"`
			} `json:"version_group_details"`
		} `json:"moves"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("decoding %q: %w", name, err)
	}
	out := &pokemonMovesReport{
		Pokemon: strings.ToLower(name),
		Filters: filters,
	}
	for _, m := range doc.Moves {
		for _, vg := range m.VersionGroupDetails {
			if filters.Method != "" && vg.MoveLearnMethod.Name != filters.Method {
				continue
			}
			if filters.VersionGroup != "" && vg.VersionGroup.Name != filters.VersionGroup {
				continue
			}
			if vg.MoveLearnMethod.Name == "level-up" {
				if filters.MaxLevel > 0 && vg.LevelLearnedAt > filters.MaxLevel {
					continue
				}
				if filters.MinLevel > 0 && vg.LevelLearnedAt < filters.MinLevel {
					continue
				}
			}
			out.Moves = append(out.Moves, pokemonMove{
				Move:           m.Move.Name,
				LearnMethod:    vg.MoveLearnMethod.Name,
				VersionGroup:   vg.VersionGroup.Name,
				LevelLearnedAt: vg.LevelLearnedAt,
			})
		}
	}
	sort.Slice(out.Moves, func(i, j int) bool {
		if out.Moves[i].LearnMethod != out.Moves[j].LearnMethod {
			return out.Moves[i].LearnMethod < out.Moves[j].LearnMethod
		}
		if out.Moves[i].LevelLearnedAt != out.Moves[j].LevelLearnedAt {
			return out.Moves[i].LevelLearnedAt < out.Moves[j].LevelLearnedAt
		}
		return out.Moves[i].Move < out.Moves[j].Move
	})
	out.Total = len(out.Moves)
	return out, nil
}
