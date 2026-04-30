// Package poke contains hand-written helpers shared by novel-feature commands.
// This is NOT a generator-emitted package — agents may add to it.
package poke

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// AllTypes is the canonical list of the 18 official Pokémon damage types,
// in the order Game Freak displays them on type charts. We use this as a
// stable enumeration when no live data is available.
var AllTypes = []string{
	"normal", "fighting", "flying", "poison", "ground", "rock",
	"bug", "ghost", "steel", "fire", "water", "grass",
	"electric", "psychic", "ice", "dragon", "dark", "fairy",
}

// TypeRelations is the four-way damage-multiplier table for a single
// defending type drawn from PokeAPI's /type/{name}/ payload.
type TypeRelations struct {
	DoubleDamageFrom []string // attacker types that hit this type for 2×
	HalfDamageFrom   []string // 0.5×
	NoDamageFrom     []string // 0×
	DoubleDamageTo   []string // attacker uses move of this type → 2× vs these defending types
	HalfDamageTo     []string // 0.5× vs these
	NoDamageTo       []string // 0× vs these (immune)
}

// DefenderRow is a per-defending-type score for one Pokémon (which can
// have one or two types). We multiply incoming multipliers across both.
type DefenderRow struct {
	Attacker   string  `json:"attacker"`
	Multiplier float64 `json:"multiplier"`
}

// Fetcher is the thin contract a command needs to look up live Pokémon
// data. Both the live HTTP client and a local-store reader can satisfy it.
type Fetcher interface {
	GetTypeRelations(ctx context.Context, typeName string) (TypeRelations, error)
	GetPokemonTypes(ctx context.Context, name string) ([]string, error)
}

// DefensiveProfile computes how each of the 18 attacker types damages a
// Pokémon with the supplied defending types. The multiplier table is the
// product of each defending type's incoming multipliers.
//
// Ground attacks Magnezone (steel + electric):
//
//	steel takes 2× from ground, electric takes 2× from ground → 4×.
func DefensiveProfile(rels []TypeRelations) map[string]float64 {
	out := make(map[string]float64, len(AllTypes))
	for _, atk := range AllTypes {
		out[atk] = 1.0
	}
	for _, r := range rels {
		for _, atk := range r.DoubleDamageFrom {
			out[atk] *= 2.0
		}
		for _, atk := range r.HalfDamageFrom {
			out[atk] *= 0.5
		}
		for _, atk := range r.NoDamageFrom {
			out[atk] = 0
		}
	}
	return out
}

// OffensiveCoverage returns, for each attacking type a Pokémon has, the set
// of defending types it hits for super-effective damage (2× or better). When
// the Pokémon has two types we union the coverage.
func OffensiveCoverage(rels []TypeRelations) map[string]bool {
	covered := make(map[string]bool)
	for _, r := range rels {
		for _, def := range r.DoubleDamageTo {
			covered[def] = true
		}
	}
	return covered
}

// MultiplierBucket groups attackers by their resolved damage multiplier.
// Returned in the order tools usually render: 4×, 2×, ½×, ¼×, 0×.
type MultiplierBucket struct {
	Multiplier string   `json:"multiplier"`
	Types      []string `json:"types"`
}

// BucketDefensive renders a defensive profile as deterministic, sorted
// buckets. Empty buckets are dropped.
func BucketDefensive(profile map[string]float64) []MultiplierBucket {
	groups := map[string][]string{
		"4x":  nil,
		"2x":  nil,
		"1/2": nil,
		"1/4": nil,
		"0":   nil,
	}
	for atk, mult := range profile {
		switch {
		case mult >= 4.0:
			groups["4x"] = append(groups["4x"], atk)
		case mult >= 2.0:
			groups["2x"] = append(groups["2x"], atk)
		case mult == 0:
			groups["0"] = append(groups["0"], atk)
		case mult <= 0.25:
			groups["1/4"] = append(groups["1/4"], atk)
		case mult <= 0.5:
			groups["1/2"] = append(groups["1/2"], atk)
		}
	}
	order := []string{"4x", "2x", "1/2", "1/4", "0"}
	out := make([]MultiplierBucket, 0, len(order))
	for _, k := range order {
		if len(groups[k]) == 0 {
			continue
		}
		sort.Strings(groups[k])
		out = append(out, MultiplierBucket{Multiplier: k, Types: groups[k]})
	}
	return out
}

// ParseTeam splits a comma- or space-separated team list into trimmed
// lowercased Pokémon slugs. Empty entries are dropped.
func ParseTeam(raw string) []string {
	if raw == "" {
		return nil
	}
	// Allow either commas or spaces as separators.
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.ToLower(strings.TrimSpace(f))
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

// ExtractResults peels the {meta, results} envelope returned by every
// generated CLI endpoint and unmarshals the inner payload. Returns an
// error if the envelope shape is unexpected.
func ExtractResults(raw json.RawMessage, dst any) error {
	var env struct {
		Results json.RawMessage `json:"results"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("decoding envelope: %w", err)
	}
	if len(env.Results) == 0 {
		return fmt.Errorf("envelope missing results field")
	}
	if err := json.Unmarshal(env.Results, dst); err != nil {
		return fmt.Errorf("decoding results: %w", err)
	}
	return nil
}
