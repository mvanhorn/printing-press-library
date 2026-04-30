package cli

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type diffLearnsetReport struct {
	Form1  string     `json:"form1"`
	Form2  string     `json:"form2"`
	Only1  []string   `json:"only_in_form1"`
	Only2  []string   `json:"only_in_form2"`
	Shared []string   `json:"shared"`
	Counts diffCounts `json:"counts"`
}

type diffCounts struct {
	Only1  int `json:"only_in_form1"`
	Only2  int `json:"only_in_form2"`
	Shared int `json:"shared"`
}

func newPokemonDiffLearnsetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff-learnset [form1] [form2]",
		Short: "Compare two Pokémon (often forms or megas) and surface their move-learnset differences",
		Long: `Fetches the full learnset for two Pokémon and reports moves only the first knows,
moves only the second knows, and shared moves. Useful for choosing between forms
('charizard vs charizard-mega-x') or comparing regional variants ('vulpix vs
vulpix-alola').`,
		Example: strings.Trim(`
  pokeapi-pp-cli pokemon diff-learnset charizard charizard-mega-x --json
  pokeapi-pp-cli pokemon diff-learnset vulpix vulpix-alola --json --select only_in_form1,only_in_form2`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
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
			ms1, err := buildPokemonMoves(ctx, c, args[0], movesFilters{})
			if err != nil {
				return err
			}
			ms2, err := buildPokemonMoves(ctx, c, args[1], movesFilters{})
			if err != nil {
				return err
			}
			s1 := uniqMoves(ms1.Moves)
			s2 := uniqMoves(ms2.Moves)
			report := &diffLearnsetReport{
				Form1:  strings.ToLower(args[0]),
				Form2:  strings.ToLower(args[1]),
				Only1:  diffStrings(s1, s2),
				Only2:  diffStrings(s2, s1),
				Shared: intersectStrings(s1, s2),
			}
			report.Counts.Only1 = len(report.Only1)
			report.Counts.Only2 = len(report.Only2)
			report.Counts.Shared = len(report.Shared)
			b, err := json.Marshal(report)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
		},
	}
	return cmd
}

func uniqMoves(moves []pokemonMove) []string {
	seen := make(map[string]bool, len(moves))
	out := make([]string, 0, len(moves))
	for _, m := range moves {
		if !seen[m.Move] {
			seen[m.Move] = true
			out = append(out, m.Move)
		}
	}
	sort.Strings(out)
	return out
}

func diffStrings(a, b []string) []string {
	bset := make(map[string]bool, len(b))
	for _, s := range b {
		bset[s] = true
	}
	out := make([]string, 0)
	for _, s := range a {
		if !bset[s] {
			out = append(out, s)
		}
	}
	return out
}

func intersectStrings(a, b []string) []string {
	bset := make(map[string]bool, len(b))
	for _, s := range b {
		bset[s] = true
	}
	out := make([]string, 0)
	for _, s := range a {
		if bset[s] {
			out = append(out, s)
		}
	}
	return out
}
