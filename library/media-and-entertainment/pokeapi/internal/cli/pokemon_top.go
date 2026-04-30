package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type pokemonTopEntry struct {
	Name  string   `json:"name"`
	Stat  int      `json:"stat"`
	Types []string `json:"types"`
}

type pokemonTopReport struct {
	StatName   string            `json:"stat"`
	TypeFilter string            `json:"type_filter,omitempty"`
	Entries    []pokemonTopEntry `json:"entries"`
	Total      int               `json:"total"`
	Source     string            `json:"source"`
	Notes      []string          `json:"notes,omitempty"`
}

func newPokemonTopCmd(flags *rootFlags) *cobra.Command {
	var (
		stat       string
		typeFilter string
		limit      int
	)
	cmd := &cobra.Command{
		Use:   "top",
		Short: "Rank Pokémon by a base stat (attack, special-attack, speed, hp, defense, special-defense), optionally filtered by type",
		Long: `Iterates the local store's pokemon list (or live /pokemon/?limit=151 fallback)
and ranks each by the requested base stat. Combine --by special-attack with
--type ghost to answer 'who's the top special attacker among Ghost-types?'.

Source: prefers the local store. With an empty store, lists gen-1 (151 Pokémon)
via the live API.`,
		Example: strings.Trim(`
  pokeapi-pp-cli pokemon top --by attack --type fighting --limit 10 --json
  pokeapi-pp-cli pokemon top --by special-attack --limit 5 --json
  pokeapi-pp-cli pokemon top --by speed --type electric --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if stat == "" {
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
			report, err := rankByStat(ctx, c, stat, typeFilter, limit)
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
	cmd.Flags().StringVar(&stat, "by", "", "Base stat to rank by: hp, attack, defense, special-attack, special-defense, speed (required)")
	cmd.Flags().StringVar(&typeFilter, "type", "", "Restrict to Pokémon of this type (e.g. fire, ghost)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Number of top results to return (default 10)")
	return cmd
}

func rankByStat(_ context.Context, c clientLike, statName, typeFilter string, limit int) (*pokemonTopReport, error) {
	statName = strings.ToLower(strings.TrimSpace(statName))
	typeFilter = strings.ToLower(strings.TrimSpace(typeFilter))
	report := &pokemonTopReport{StatName: statName, TypeFilter: typeFilter}
	allowedStats := map[string]bool{"hp": true, "attack": true, "defense": true, "special-attack": true, "special-defense": true, "speed": true}
	if !allowedStats[statName] {
		return nil, fmt.Errorf("unknown stat %q; must be one of hp, attack, defense, special-attack, special-defense, speed", statName)
	}
	candidates, source, err := listCandidatesForRanking(c, typeFilter)
	if err != nil {
		return nil, err
	}
	report.Source = source
	if len(candidates) == 0 {
		report.Notes = append(report.Notes, "No candidates found. With an empty local store the fallback covers gen-1 (151) only — run 'pokeapi-pp-cli sync --resources pokemon' for full coverage.")
		return report, nil
	}
	for _, name := range candidates {
		entry, err := loadDamageActor(c, name)
		if err != nil {
			continue
		}
		if typeFilter != "" {
			ok := false
			for _, t := range entry.Types {
				if t == typeFilter {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}
		var v int
		switch statName {
		case "hp":
			v = entry.HP
		case "attack":
			v = entry.Attack
		case "defense":
			v = entry.Defense
		case "special-attack":
			v = entry.SpecialAttack
		case "special-defense":
			v = entry.SpecialDefense
		case "speed":
			v = entry.Speed
		}
		report.Entries = append(report.Entries, pokemonTopEntry{
			Name:  entry.Name,
			Stat:  v,
			Types: entry.Types,
		})
	}
	sort.SliceStable(report.Entries, func(i, j int) bool {
		if report.Entries[i].Stat != report.Entries[j].Stat {
			return report.Entries[i].Stat > report.Entries[j].Stat
		}
		return report.Entries[i].Name < report.Entries[j].Name
	})
	if limit > 0 && len(report.Entries) > limit {
		report.Entries = report.Entries[:limit]
	}
	report.Total = len(report.Entries)
	return report, nil
}

func listCandidatesForRanking(c clientLike, _ string) ([]string, string, error) {
	if names := tryListPokemonFromStore(); len(names) > 0 {
		return names, "local-store", nil
	}
	raw, err := c.Get("/api/v2/pokemon/", map[string]string{"limit": "151"})
	if err != nil {
		return nil, "live-failed", fmt.Errorf("listing pokemon: %w", err)
	}
	var doc struct {
		Results []struct {
			Name string `json:"name"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, "live-failed", err
	}
	out := make([]string, 0, len(doc.Results))
	for _, r := range doc.Results {
		out = append(out, r.Name)
	}
	return out, "live-gen1", nil
}

func tryListPokemonFromStore() []string {
	dbPath := defaultDBPath("pokeapi-pp-cli")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil
	}
	defer db.Close()
	rows, err := db.Query("SELECT id FROM resources WHERE resource_type='pokemon'")
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := make([]string, 0, 200)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		out = append(out, id)
	}
	return out
}
