package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/pokeapi/internal/poke"
)

type damageReport struct {
	Attacker        string   `json:"attacker"`
	Defender        string   `json:"defender"`
	Move            string   `json:"move"`
	MovePower       int      `json:"move_power"`
	MoveType        string   `json:"move_type"`
	MoveDamageClass string   `json:"move_damage_class"`
	AttackerLevel   int      `json:"attacker_level"`
	DefenderLevel   int      `json:"defender_level"`
	STAB            float64  `json:"stab"`
	TypeMultiplier  float64  `json:"type_multiplier"`
	MinDamage       int      `json:"min_damage"`
	MaxDamage       int      `json:"max_damage"`
	DefenderHP      int      `json:"defender_hp_at_level"`
	MinPercentage   float64  `json:"min_pct_of_defender_hp"`
	MaxPercentage   float64  `json:"max_pct_of_defender_hp"`
	IsKO            bool     `json:"is_guaranteed_ko"`
	Notes           []string `json:"notes,omitempty"`
}

func newDamageCmd(flags *rootFlags) *cobra.Command {
	var (
		atkLevel int
		defLevel int
	)
	cmd := &cobra.Command{
		Use:   "damage [attacker] [defender] [move]",
		Short: "Compute expected damage range for a move (Smogon-calc-style; STAB + type effectiveness + level + base stats)",
		Long: `Computes a damage range using the official damage formula plus the type chart:

  base = floor(((2*level/5 + 2) * power * Attack/Defense) / 50) + 2
  total = base * STAB * type_multiplier * random(0.85, 1.0)

Uses base stats only — no IVs, EVs, items, abilities, weather, or status. Produces
the same shape Smogon's damage calculator does for the basic case. For full battle
mechanics use Pokémon Showdown.`,
		Example: strings.Trim(`
  pokeapi-pp-cli damage charizard blastoise hydro-pump --json
  pokeapi-pp-cli damage gengar mewtwo shadow-ball --level1 100 --level2 100 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 3 {
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
			report, err := computeDamage(ctx, c, f, args[0], args[1], args[2], atkLevel, defLevel)
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
	cmd.Flags().IntVar(&atkLevel, "level1", 50, "Attacker level (default 50)")
	cmd.Flags().IntVar(&defLevel, "level2", 50, "Defender level (default 50)")
	return cmd
}

func computeDamage(ctx context.Context, c clientLike, f *poke.LiveFetcher, attackerName, defenderName, moveName string, atkLevel, defLevel int) (*damageReport, error) {
	if atkLevel <= 0 {
		atkLevel = 50
	}
	if defLevel <= 0 {
		defLevel = 50
	}
	atk, err := loadDamageActor(c, attackerName)
	if err != nil {
		return nil, fmt.Errorf("attacker %q: %w", attackerName, err)
	}
	def, err := loadDamageActor(c, defenderName)
	if err != nil {
		return nil, fmt.Errorf("defender %q: %w", defenderName, err)
	}
	mv, err := loadMove(c, moveName)
	if err != nil {
		return nil, fmt.Errorf("move %q: %w", moveName, err)
	}
	report := &damageReport{
		Attacker:        atk.Name,
		Defender:        def.Name,
		Move:            mv.Name,
		MovePower:       mv.Power,
		MoveType:        mv.Type,
		MoveDamageClass: mv.DamageClass,
		AttackerLevel:   atkLevel,
		DefenderLevel:   defLevel,
		STAB:            1.0,
		TypeMultiplier:  1.0,
	}
	if mv.Power == 0 || mv.DamageClass == "status" {
		report.Notes = append(report.Notes, fmt.Sprintf("Move %q has no fixed power (status or variable-power); damage range is 0.", mv.Name))
		return report, nil
	}
	// STAB: 1.5 if move type matches one of attacker's types.
	for _, t := range atk.Types {
		if t == mv.Type {
			report.STAB = 1.5
			break
		}
	}
	// Type multiplier: product of move type's relations vs each defender type.
	rels, err := f.GetTypeRelations(ctx, mv.Type)
	if err != nil {
		return nil, fmt.Errorf("type relations for %q: %w", mv.Type, err)
	}
	for _, dt := range def.Types {
		switch {
		case stringInSlice(dt, rels.NoDamageTo):
			report.TypeMultiplier = 0
		case stringInSlice(dt, rels.DoubleDamageTo):
			report.TypeMultiplier *= 2.0
		case stringInSlice(dt, rels.HalfDamageTo):
			report.TypeMultiplier *= 0.5
		}
	}
	// Choose attack/defense stat by damage class.
	var atkStat, defStat int
	if mv.DamageClass == "physical" {
		atkStat = atk.Attack
		defStat = def.Defense
	} else {
		atkStat = atk.SpecialAttack
		defStat = def.SpecialDefense
	}
	if defStat == 0 {
		defStat = 1 // avoid div by zero
	}
	// Approximate stat at given level: stat * (level/50) for non-HP stats. PokeAPI ships
	// base stats; without IVs/EVs, level scaling is roughly linear in agent contexts.
	atkAtLevel := scaleStatToLevel(atkStat, atkLevel)
	defAtLevel := scaleStatToLevel(defStat, defLevel)
	// Damage formula:
	// base = floor( ((2*lvl/5 + 2) * power * atk/def) / 50 ) + 2
	base := ((2*atkLevel/5+2)*mv.Power*atkAtLevel/defAtLevel)/50 + 2
	if base < 1 {
		base = 1
	}
	total := float64(base) * report.STAB * report.TypeMultiplier
	report.MinDamage = int(total * 0.85)
	report.MaxDamage = int(total)
	// Defender HP at level
	hpStat := scaleHPToLevel(def.HP, defLevel)
	report.DefenderHP = hpStat
	if hpStat > 0 {
		report.MinPercentage = float64(report.MinDamage) / float64(hpStat) * 100
		report.MaxPercentage = float64(report.MaxDamage) / float64(hpStat) * 100
	}
	report.IsKO = report.MinDamage >= report.DefenderHP
	if report.TypeMultiplier == 0 {
		report.Notes = append(report.Notes, "Move has no effect on defender (immunity).")
	} else if report.TypeMultiplier >= 2.0 {
		report.Notes = append(report.Notes, "Super effective.")
	} else if report.TypeMultiplier <= 0.5 {
		report.Notes = append(report.Notes, "Not very effective.")
	}
	if report.STAB > 1.0 {
		report.Notes = append(report.Notes, "Same-Type Attack Bonus (STAB) applied.")
	}
	return report, nil
}

type damageActor struct {
	Name           string
	Types          []string
	HP             int
	Attack         int
	Defense        int
	SpecialAttack  int
	SpecialDefense int
	Speed          int
}

func loadDamageActor(c clientLike, name string) (*damageActor, error) {
	raw, err := c.Get(fmt.Sprintf("/api/v2/pokemon/%s/", strings.ToLower(strings.TrimSpace(name))), nil)
	if err != nil {
		return nil, err
	}
	var p struct {
		Name  string `json:"name"`
		Types []struct {
			Type struct{ Name string } `json:"type"`
		} `json:"types"`
		Stats []struct {
			BaseStat int                   `json:"base_stat"`
			Stat     struct{ Name string } `json:"stat"`
		} `json:"stats"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	a := &damageActor{Name: p.Name}
	for _, t := range p.Types {
		a.Types = append(a.Types, t.Type.Name)
	}
	for _, s := range p.Stats {
		switch s.Stat.Name {
		case "hp":
			a.HP = s.BaseStat
		case "attack":
			a.Attack = s.BaseStat
		case "defense":
			a.Defense = s.BaseStat
		case "special-attack":
			a.SpecialAttack = s.BaseStat
		case "special-defense":
			a.SpecialDefense = s.BaseStat
		case "speed":
			a.Speed = s.BaseStat
		}
	}
	return a, nil
}

type moveData struct {
	Name        string
	Power       int
	Type        string
	DamageClass string
	Accuracy    int
	PP          int
	Ailment     string
}

func loadMove(c clientLike, name string) (*moveData, error) {
	raw, err := c.Get(fmt.Sprintf("/api/v2/move/%s/", strings.ToLower(strings.TrimSpace(name))), nil)
	if err != nil {
		return nil, err
	}
	var m struct {
		Name        string                `json:"name"`
		Power       *int                  `json:"power"`
		Accuracy    *int                  `json:"accuracy"`
		PP          *int                  `json:"pp"`
		Type        struct{ Name string } `json:"type"`
		DamageClass struct{ Name string } `json:"damage_class"`
		Meta        *struct {
			Ailment *struct{ Name string } `json:"ailment"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	out := &moveData{
		Name:        m.Name,
		Type:        m.Type.Name,
		DamageClass: m.DamageClass.Name,
	}
	if m.Power != nil {
		out.Power = *m.Power
	}
	if m.Accuracy != nil {
		out.Accuracy = *m.Accuracy
	}
	if m.PP != nil {
		out.PP = *m.PP
	}
	if m.Meta != nil && m.Meta.Ailment != nil {
		out.Ailment = m.Meta.Ailment.Name
	}
	return out, nil
}

func stringInSlice(s string, ss []string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// scaleStatToLevel converts a base stat to its approximate value at a given
// level, ignoring IVs/EVs and natures. Mainline formula at L=50 with 0 IV/EV
// is roughly: stat = (2*base * 50 / 100) + 5 = base + 5. We approximate with
// (base * level / 50) + 5 to scale linearly.
func scaleStatToLevel(base, level int) int {
	if base <= 0 {
		return 0
	}
	return (base*level)/50 + 5
}

// scaleHPToLevel does the same for HP, which has a +10 floor (HP = base + lvl + 10
// in the simplified formula).
func scaleHPToLevel(base, level int) int {
	if base <= 0 {
		return 0
	}
	return (base*level)/50 + level + 10
}
