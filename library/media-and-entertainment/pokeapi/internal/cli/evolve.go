package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type evolveIntoReport struct {
	Target string           `json:"target"`
	Paths  []evolveIntoPath `json:"paths"`
	Total  int              `json:"total"`
	Source string           `json:"source"`
	Notes  []string         `json:"notes,omitempty"`
}

type evolveIntoPath struct {
	From       string         `json:"from"`
	Trigger    string         `json:"trigger,omitempty"`
	Conditions map[string]any `json:"conditions,omitempty"`
}

func newEvolveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "evolve",
		Short: "Evolution-direction queries (e.g. 'how do I get an Umbreon?')",
	}
	cmd.AddCommand(newEvolveIntoCmd(flags))
	cmd.AddCommand(newEvolveIntoCmd(flags))
	return cmd
}

func newEvolveIntoCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "into [target]",
		Short: "Reverse-lookup the species and conditions that evolve into the target Pokémon",
		Long: `Given a target Pokémon (e.g. umbreon, gardevoir, garchomp), search every
evolution chain for branches whose result matches the target. Returns the source
species, the trigger (level-up, use-item, trade, ...), and the required conditions
(item, level, friendship, time-of-day, held-item, ...).

Source: prefers the local store (run 'pokeapi-pp-cli sync --resources evolution-chain').
Falls back to a paginated live API scan, which can take a moment.`,
		Example: strings.Trim(`
  pokeapi-pp-cli evolve into umbreon --json
  pokeapi-pp-cli evolve into garchomp --json --select paths`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			target := strings.ToLower(strings.TrimSpace(args[0]))
			report, err := findEvolveInto(flags, target)
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
	return cmd
}

// findEvolveInto walks the evolution chain that contains the target species
// and surfaces every branch producing it. Fast path: resolve target → species
// → chain ID directly; only scan that single chain. Falls back to a full scan
// of all chains if the species lookup fails (e.g., target is a Pokémon form,
// not a base species).
func findEvolveInto(flags *rootFlags, target string) (*evolveIntoReport, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	report := &evolveIntoReport{Target: target, Source: "live"}

	// Fast path: ask species directly which chain it belongs to.
	if rawSp, err := c.Get(fmt.Sprintf("/api/v2/pokemon-species/%s/", target), nil); err == nil {
		var sp struct {
			EvolutionChain struct {
				URL string `json:"url"`
			} `json:"evolution_chain"`
		}
		if json.Unmarshal(rawSp, &sp) == nil {
			if id := extractIDFromURL(sp.EvolutionChain.URL); id > 0 {
				if rawChain, err := c.Get(fmt.Sprintf("/api/v2/evolution-chain/%d/", id), nil); err == nil {
					var doc struct {
						Chain chainLinkRaw `json:"chain"`
					}
					if json.Unmarshal(rawChain, &doc) == nil {
						walkChainForTarget(doc.Chain, target, report)
						report.Total = len(report.Paths)
						report.Notes = append(report.Notes, fmt.Sprintf("Resolved chain via species lookup (chain %d).", id))
						if report.Total == 0 {
							report.Notes = append(report.Notes, fmt.Sprintf("%q is in a chain but no parent evolves into it — likely the chain root.", target))
						}
						return report, nil
					}
				}
			}
		}
	}
	// Fallback: scan every chain.
	chainIDs, err := listAllChainIDs(c)
	if err != nil {
		return nil, err
	}
	report.Notes = append(report.Notes, fmt.Sprintf("Species lookup failed; scanned %d evolution chains via live API.", len(chainIDs)))
	for _, id := range chainIDs {
		raw, err := c.Get(fmt.Sprintf("/api/v2/evolution-chain/%d/", id), nil)
		if err != nil {
			continue
		}
		var doc struct {
			Chain chainLinkRaw `json:"chain"`
		}
		if json.Unmarshal(raw, &doc) != nil {
			continue
		}
		walkChainForTarget(doc.Chain, target, report)
	}
	report.Total = len(report.Paths)
	if report.Total == 0 {
		report.Notes = append(report.Notes, fmt.Sprintf("No evolution path produces %q. Either it is a base form, has no evolutions, or PokeAPI lacks the chain.", target))
	}
	return report, nil
}

func walkChainForTarget(link chainLinkRaw, target string, report *evolveIntoReport) {
	for _, child := range link.EvolvesTo {
		if child.Species.Name == target {
			path := evolveIntoPath{From: link.Species.Name}
			if len(child.EvolutionDetails) > 0 {
				d := child.EvolutionDetails[0]
				path.Trigger = d.Trigger.Name
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
				if d.TimeOfDay != "" {
					conds["time_of_day"] = d.TimeOfDay
				}
				if d.KnownMove != nil && d.KnownMove.Name != "" {
					conds["known_move"] = d.KnownMove.Name
				}
				if d.Location != nil && d.Location.Name != "" {
					conds["location"] = d.Location.Name
				}
				if len(conds) > 0 {
					path.Conditions = conds
				}
			}
			report.Paths = append(report.Paths, path)
		}
		walkChainForTarget(child, target, report)
	}
}

func listAllChainIDs(c clientLike) ([]int, error) {
	// Fast path: if local store has evolution_chain ids, use them.
	if ids := tryListChainIDsFromStore(); len(ids) > 0 {
		return ids, nil
	}
	raw, err := c.Get("/api/v2/evolution-chain/", map[string]string{"limit": "1000"})
	if err != nil {
		return nil, fmt.Errorf("listing evolution chains: %w", err)
	}
	var doc struct {
		Results []struct {
			URL string `json:"url"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("decoding chain list: %w", err)
	}
	out := make([]int, 0, len(doc.Results))
	for _, r := range doc.Results {
		if id := extractIDFromURL(r.URL); id > 0 {
			out = append(out, id)
		}
	}
	return out, nil
}

func tryListChainIDsFromStore() []int {
	dbPath := defaultDBPath("pokeapi-pp-cli")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil
	}
	defer db.Close()
	rows, err := db.Query("SELECT id FROM resources WHERE resource_type='evolution-chain'")
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := make([]int, 0, 64)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(id, "%d", &n); err == nil {
			out = append(out, n)
		}
	}
	return out
}
