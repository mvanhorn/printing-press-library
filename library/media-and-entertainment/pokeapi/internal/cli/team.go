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

func newTeamCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "team",
		Short: "Team analysis: coverage, gaps, and partner suggestions",
	}
	cmd.AddCommand(newTeamCoverageCmd(flags))
	cmd.AddCommand(newTeamGapsCmd(flags))
	cmd.AddCommand(newTeamSuggestCmd(flags))
	return cmd
}

type teamMember struct {
	Name      string                  `json:"name"`
	Types     []string                `json:"types"`
	Defensive []poke.MultiplierBucket `json:"defensive"`
	Offensive map[string][]string     `json:"offensive"`
}

type teamCoverageReport struct {
	Team             []teamMember        `json:"team"`
	OffensiveTypes   map[string][]string `json:"offensive_coverage"` // attacker type → defenders hit 2×, deduped across team
	SharedWeaknesses map[string][]string `json:"shared_weaknesses"`  // attacker type → list of team members weak to it
	Resistances      map[string][]string `json:"shared_resistances"` // attacker type → members that resist
	Immunities       map[string][]string `json:"shared_immunities"`  // attacker type → members immune
	UncoveredTypes   []string            `json:"uncovered_types"`    // 18 types your team can't hit super-effectively
	Notes            []string            `json:"notes,omitempty"`
}

func newTeamCoverageCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "coverage [members]",
		Short: "Type analysis for a team: shared weaknesses, resistances, immunities, and offensive coverage",
		Long: `Accepts a comma- or space-separated list of Pokémon and computes:
  - shared_weaknesses: attacker types your team is collectively weak to
  - shared_resistances/immunities: attacker types your team handles well
  - offensive_coverage: defenders your team can hit super-effectively
  - uncovered_types: defenders no team member can dent`,
		Example: strings.Trim(`
  pokeapi-pp-cli team coverage pikachu,charizard,blastoise --json
  pokeapi-pp-cli team coverage 'gengar dragapult tyranitar' --json --select shared_weaknesses,uncovered_types`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			team := poke.ParseTeam(strings.Join(args, ","))
			if len(team) == 0 {
				return fmt.Errorf("no team members provided; pass a comma-separated list like 'pikachu,charizard,blastoise'")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			f := poke.NewLiveFetcher(c)
			ctx := context.Background()
			report, err := buildTeamCoverage(ctx, f, team)
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

func buildTeamCoverage(ctx context.Context, f *poke.LiveFetcher, team []string) (*teamCoverageReport, error) {
	report := &teamCoverageReport{
		OffensiveTypes:   make(map[string][]string),
		SharedWeaknesses: make(map[string][]string),
		Resistances:      make(map[string][]string),
		Immunities:       make(map[string][]string),
	}
	covered := make(map[string]bool, 18)
	for _, name := range team {
		report_, err := buildMatchups(ctx, f, name)
		if err != nil {
			report.Notes = append(report.Notes, fmt.Sprintf("skipping %q: %v", name, err))
			continue
		}
		member := teamMember{
			Name:      report_.Pokemon,
			Types:     report_.Types,
			Defensive: report_.Defensive,
			Offensive: report_.Offensive,
		}
		report.Team = append(report.Team, member)
		// shared weaknesses, resistances, immunities by bucket
		for _, b := range report_.Defensive {
			for _, atk := range b.Types {
				switch b.Multiplier {
				case "4x", "2x":
					report.SharedWeaknesses[atk] = append(report.SharedWeaknesses[atk], name)
				case "1/2", "1/4":
					report.Resistances[atk] = append(report.Resistances[atk], name)
				case "0":
					report.Immunities[atk] = append(report.Immunities[atk], name)
				}
			}
		}
		// offensive coverage: union all 2× targets across attacking types
		for atkType, defs := range report_.Offensive {
			report.OffensiveTypes[atkType] = unionStrings(report.OffensiveTypes[atkType], defs)
			for _, d := range defs {
				covered[d] = true
			}
		}
	}
	report.UncoveredTypes = make([]string, 0, 18)
	for _, t := range poke.AllTypes {
		if !covered[t] {
			report.UncoveredTypes = append(report.UncoveredTypes, t)
		}
	}
	sort.Strings(report.UncoveredTypes)
	for k := range report.SharedWeaknesses {
		sort.Strings(report.SharedWeaknesses[k])
	}
	for k := range report.Resistances {
		sort.Strings(report.Resistances[k])
	}
	for k := range report.Immunities {
		sort.Strings(report.Immunities[k])
	}
	for k := range report.OffensiveTypes {
		sort.Strings(report.OffensiveTypes[k])
	}
	return report, nil
}

func unionStrings(a, b []string) []string {
	seen := make(map[string]bool, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, s := range a {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, s := range b {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// teamGapsReport is the structured output of `team gaps`.
type teamGapsReport struct {
	Team           []string            `json:"team"`
	OffensiveGaps  []string            `json:"offensive_gaps"`            // defenders no team member hits 2×
	DefensiveGaps  []string            `json:"defensive_gaps"`            // attackers no team member resists
	DoubleExposure map[string][]string `json:"double_exposure,omitempty"` // attackers ≥2 team members are weak to
}

func newTeamGapsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gaps [members]",
		Short: "Surface offensive and defensive gaps in a team's typing (which types are uncovered or doubly exposed)",
		Long: `For a team:
  - offensive_gaps: defending types no member hits for 2×
  - defensive_gaps: attacking types no member resists or is immune to
  - double_exposure: attackers ≥2 members are weak to (the dangerous shared weaknesses)`,
		Example: strings.Trim(`
  pokeapi-pp-cli team gaps pikachu,charizard,blastoise --json
  pokeapi-pp-cli team gaps 'mewtwo machamp salamence' --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			team := poke.ParseTeam(strings.Join(args, ","))
			if len(team) == 0 {
				return fmt.Errorf("no team members provided")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			f := poke.NewLiveFetcher(c)
			ctx := context.Background()
			cov, err := buildTeamCoverage(ctx, f, team)
			if err != nil {
				return err
			}
			out := &teamGapsReport{
				Team:           team,
				OffensiveGaps:  cov.UncoveredTypes,
				DoubleExposure: make(map[string][]string),
			}
			// defensive_gaps: attackers nobody resists or is immune to
			handled := make(map[string]bool)
			for atk := range cov.Resistances {
				handled[atk] = true
			}
			for atk := range cov.Immunities {
				handled[atk] = true
			}
			for _, atk := range poke.AllTypes {
				if !handled[atk] {
					out.DefensiveGaps = append(out.DefensiveGaps, atk)
				}
			}
			sort.Strings(out.DefensiveGaps)
			// double_exposure: weakness shared by ≥ 2 members
			for atk, members := range cov.SharedWeaknesses {
				if len(members) >= 2 {
					sort.Strings(members)
					out.DoubleExposure[atk] = members
				}
			}
			b, err := json.Marshal(out)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
		},
	}
	return cmd
}

// teamSuggestCandidate scores a candidate Pokémon by how many of the team's
// gaps it fills. The score equals: gap-types it covers offensively + types
// it resists that the team currently doesn't.
type teamSuggestCandidate struct {
	Name             string   `json:"name"`
	Types            []string `json:"types"`
	Score            int      `json:"score"`
	OffensiveCovered []string `json:"offensive_covered"`
	DefensiveCovered []string `json:"defensive_covered"`
}

type teamSuggestReport struct {
	Team       []string               `json:"team"`
	Candidates []teamSuggestCandidate `json:"candidates"`
	Slots      int                    `json:"slots"`
	Source     string                 `json:"source"`
	Notes      []string               `json:"notes,omitempty"`
}

func newTeamSuggestCmd(flags *rootFlags) *cobra.Command {
	var (
		slots int
		limit int
	)
	cmd := &cobra.Command{
		Use:   "suggest [members]",
		Short: "Score remaining Pokémon by how well they fill the team's typing gaps",
		Long: `Computes the team's offensive and defensive gaps, then scores candidate Pokémon
by how many gaps each fills. Returns the top candidates by score.

Scope: type-coverage-only — does not consider Smogon usage stats, item synergies,
or role coverage. For meta-game team-building, complement this with Pikalytics or
Smogon's strategy dex.

Source: prefers the local store (run sync first); falls back to a live API list if
the store is empty. With many candidates this can take a moment.`,
		Example: strings.Trim(`
  pokeapi-pp-cli team suggest pikachu,charizard --slots 6 --limit 10 --json
  pokeapi-pp-cli team suggest 'gengar tyranitar' --json --select candidates.name,candidates.score`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			team := poke.ParseTeam(strings.Join(args, ","))
			if len(team) == 0 {
				return fmt.Errorf("no team members provided")
			}
			cl, err := flags.newClient()
			if err != nil {
				return err
			}
			f := poke.NewLiveFetcher(cl)
			ctx := context.Background()
			cov, err := buildTeamCoverage(ctx, f, team)
			if err != nil {
				return err
			}
			gaps := offensiveGapsFromCoverage(cov)
			defGaps := defensiveGapsFromCoverage(cov)

			report := &teamSuggestReport{
				Team:   team,
				Slots:  slots,
				Source: "live",
			}
			candidates, source, notes, err := loadTeamSuggestCandidates(ctx, f, cl, team)
			report.Source = source
			report.Notes = append(report.Notes, notes...)
			if err != nil {
				return err
			}
			scored := make([]teamSuggestCandidate, 0, len(candidates))
			for _, cand := range candidates {
				rels, err := poke.CollectTypeRelations(ctx, f, cand.Types)
				if err != nil {
					continue
				}
				offCov := poke.OffensiveCoverage(rels)
				defProf := poke.DefensiveProfile(rels)
				offHits := []string{}
				for gap := range gaps {
					if offCov[gap] {
						offHits = append(offHits, gap)
					}
				}
				defHits := []string{}
				for gap := range defGaps {
					if defProf[gap] < 1.0 { // resistance or immunity
						defHits = append(defHits, gap)
					}
				}
				sort.Strings(offHits)
				sort.Strings(defHits)
				score := len(offHits) + len(defHits)
				if score == 0 {
					continue
				}
				scored = append(scored, teamSuggestCandidate{
					Name:             cand.Name,
					Types:            cand.Types,
					Score:            score,
					OffensiveCovered: offHits,
					DefensiveCovered: defHits,
				})
			}
			sort.SliceStable(scored, func(i, j int) bool {
				if scored[i].Score != scored[j].Score {
					return scored[i].Score > scored[j].Score
				}
				return scored[i].Name < scored[j].Name
			})
			if limit > 0 && len(scored) > limit {
				scored = scored[:limit]
			}
			report.Candidates = scored
			b, err := json.Marshal(report)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
		},
	}
	cmd.Flags().IntVar(&slots, "slots", 6, "Total team size to aim for (informational; does not constrain candidates)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum candidates to return (sorted by score descending)")
	return cmd
}

func offensiveGapsFromCoverage(cov *teamCoverageReport) map[string]bool {
	out := make(map[string]bool, len(cov.UncoveredTypes))
	for _, t := range cov.UncoveredTypes {
		out[t] = true
	}
	return out
}

func defensiveGapsFromCoverage(cov *teamCoverageReport) map[string]bool {
	handled := make(map[string]bool)
	for atk := range cov.Resistances {
		handled[atk] = true
	}
	for atk := range cov.Immunities {
		handled[atk] = true
	}
	out := make(map[string]bool)
	for _, t := range poke.AllTypes {
		if !handled[t] {
			out[t] = true
		}
	}
	return out
}

// teamSuggestCandidateRaw is the cheap bag of names+types we feed to the
// scorer. Loaded from the local store first; live API listing is the fallback.
type teamSuggestCandidateRaw struct {
	Name  string
	Types []string
}

// loadTeamSuggestCandidates lists pokemon via the live API and resolves each
// candidate's types via the LiveFetcher's in-process memo. Skips team members.
//
// We list gen-1 (limit=151) by default to keep network traffic and runtime
// reasonable; the local-store path is intentionally not used because the store
// only caches list-endpoint summaries (just name + url), so we'd still have to
// call /pokemon/{name}/ for each candidate.
func loadTeamSuggestCandidates(ctx context.Context, f *poke.LiveFetcher, c clientGetter, team []string) ([]teamSuggestCandidateRaw, string, []string, error) {
	teamSet := make(map[string]bool, len(team))
	for _, m := range team {
		teamSet[strings.ToLower(m)] = true
	}
	var notes []string
	raw, err := c.Get("/api/v2/pokemon/", map[string]string{"limit": "151"})
	if err != nil {
		return nil, "live-failed", notes, fmt.Errorf("listing candidates: %w", err)
	}
	var doc struct {
		Results []struct {
			Name string `json:"name"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, "live-failed", notes, fmt.Errorf("decoding candidate list: %w", err)
	}
	candidates := make([]teamSuggestCandidateRaw, 0, len(doc.Results))
	for _, r := range doc.Results {
		if teamSet[strings.ToLower(r.Name)] {
			continue
		}
		types, err := f.GetPokemonTypes(ctx, r.Name)
		if err != nil {
			continue
		}
		candidates = append(candidates, teamSuggestCandidateRaw{Name: r.Name, Types: types})
	}
	notes = append(notes, "Scoring against gen-1 candidates (151). For broader candidate pool, expand the spec or use 'team suggest' with --json + composition.")
	return candidates, "live-gen1", notes, nil
}

type clientGetter interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
}
