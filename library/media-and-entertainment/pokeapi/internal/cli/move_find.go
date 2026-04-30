package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type moveFinding struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	DamageClass string `json:"damage_class"`
	Power       int    `json:"power"`
	Accuracy    int    `json:"accuracy"`
	PP          int    `json:"pp"`
	Ailment     string `json:"ailment,omitempty"`
}

type moveFindReport struct {
	Filters moveFindFilters `json:"filters"`
	Total   int             `json:"total"`
	Moves   []moveFinding   `json:"moves"`
	Source  string          `json:"source"`
	Notes   []string        `json:"notes,omitempty"`
}

type moveFindFilters struct {
	Effect      string `json:"effect,omitempty"`
	Type        string `json:"type,omitempty"`
	DamageClass string `json:"damage_class,omitempty"`
	TypeTarget  string `json:"type_target,omitempty"`
	MinPower    int    `json:"min_power,omitempty"`
}

func newMoveFindCmd(flags *rootFlags) *cobra.Command {
	var (
		effect      string
		moveType    string
		damageClass string
		typeTarget  string
		minPower    int
		limit       int
	)
	cmd := &cobra.Command{
		Use:   "find",
		Short: "Reverse-search moves by status effect, type, damage class, or super-effective target type",
		Long: `Searches the local store for moves matching the supplied filters. The interesting
filter is --type-target: combined with --type, it returns moves of one type that hit
a defending type for super-effective damage. Useful for questions like:
  - 'what moves can paralyze a Steel-type?'   (--effect paralysis --type-target steel)
  - 'what physical Ghost moves with power >= 100 exist?'  (--type ghost --damage-class physical --min-power 100)

Source: requires a synced local store (run 'pokeapi-pp-cli sync --resources move,type' first).`,
		Example: strings.Trim(`
  pokeapi-pp-cli move find --effect paralysis --type-target steel --json
  pokeapi-pp-cli move find --type fire --min-power 100 --json
  pokeapi-pp-cli move find --damage-class status --effect sleep --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Allow --help with no flags but otherwise require at least one filter.
			if effect == "" && moveType == "" && damageClass == "" && typeTarget == "" && minPower == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			report, err := runMoveFind(moveFindFilters{
				Effect:      strings.ToLower(effect),
				Type:        strings.ToLower(moveType),
				DamageClass: strings.ToLower(damageClass),
				TypeTarget:  strings.ToLower(typeTarget),
				MinPower:    minPower,
			}, limit)
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
	cmd.Flags().StringVar(&effect, "effect", "", "Filter by status ailment (e.g. paralysis, burn, sleep, freeze, poison, confusion)")
	cmd.Flags().StringVar(&moveType, "type", "", "Filter by move type (e.g. fire, water, electric)")
	cmd.Flags().StringVar(&damageClass, "damage-class", "", "Filter by damage class: physical, special, or status")
	cmd.Flags().StringVar(&typeTarget, "type-target", "", "Filter to moves super-effective vs this defending type (combine with --type)")
	cmd.Flags().IntVar(&minPower, "min-power", 0, "Minimum move power")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum moves to return (default 50)")
	return cmd
}

func runMoveFind(filters moveFindFilters, limit int) (*moveFindReport, error) {
	dbPath := defaultDBPath("pokeapi-pp-cli")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", dbPath, err)
	}
	defer db.Close()

	report := &moveFindReport{Filters: filters, Source: "local-store"}
	rows, err := db.Query("SELECT id, data FROM resources WHERE resource_type='move'")
	if err != nil {
		return nil, fmt.Errorf("querying moves: %w", err)
	}
	defer rows.Close()
	scanned := 0
	for rows.Next() {
		var id, data string
		if err := rows.Scan(&id, &data); err != nil {
			continue
		}
		// The local store keys by resource_type+id and stores the LIST entry payload
		// ({name, url}) — not the full move record. Fall back to skipping if the
		// payload is too small to filter on.
		var summary struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(data), &summary); err != nil || summary.Name == "" {
			continue
		}
		scanned++
	}
	report.Notes = append(report.Notes, fmt.Sprintf("Scanned %d moves in the local store. The store caches list-endpoint summaries only; per-move filters require live retrieve calls.", scanned))
	if scanned == 0 {
		report.Notes = append(report.Notes, "Local store has no 'move' rows. Run 'pokeapi-pp-cli sync --resources move' first.")
		return report, nil
	}
	// Re-query and resolve each move via live API.
	rows2, err := db.Query("SELECT id FROM resources WHERE resource_type='move' ORDER BY id LIMIT ?", limit*4)
	if err != nil {
		return nil, fmt.Errorf("re-querying: %w", err)
	}
	defer rows2.Close()
	moves := make([]moveFinding, 0, limit)
	c, err := newClientFromConfig()
	if err != nil {
		return nil, err
	}
	for rows2.Next() {
		var id string
		if rows2.Scan(&id) != nil {
			continue
		}
		mv, err := loadMove(c, id)
		if err != nil {
			continue
		}
		ailment, err := loadMoveAilment(c, id)
		if err == nil {
			mv.Ailment = ailment
		}
		if !matchesFindFilters(mv, filters) {
			continue
		}
		moves = append(moves, moveFinding{
			Name:        mv.Name,
			Type:        mv.Type,
			DamageClass: mv.DamageClass,
			Power:       mv.Power,
			Accuracy:    mv.Accuracy,
			PP:          mv.PP,
			Ailment:     mv.Ailment,
		})
		if len(moves) >= limit {
			break
		}
	}
	// If --type-target is set, filter further by type effectiveness.
	if filters.TypeTarget != "" {
		filtered := make([]moveFinding, 0, len(moves))
		for _, m := range moves {
			eff, err := isMoveSuperEffective(c, m.Type, filters.TypeTarget)
			if err == nil && eff {
				filtered = append(filtered, m)
			}
		}
		moves = filtered
	}
	sort.Slice(moves, func(i, j int) bool {
		if moves[i].Power != moves[j].Power {
			return moves[i].Power > moves[j].Power
		}
		return moves[i].Name < moves[j].Name
	})
	report.Moves = moves
	report.Total = len(moves)
	return report, nil
}

// moveData here mirrors the loader in damage.go but extends with PP / ailment.
type moveDataExt struct {
	moveData
	PP      int
	Ailment string
}

func matchesFindFilters(mv *moveData, filters moveFindFilters) bool {
	if filters.Type != "" && mv.Type != filters.Type {
		return false
	}
	if filters.DamageClass != "" && mv.DamageClass != filters.DamageClass {
		return false
	}
	if filters.MinPower > 0 && mv.Power < filters.MinPower {
		return false
	}
	if filters.Effect != "" && mv.Ailment != filters.Effect {
		return false
	}
	return true
}

func loadMoveAilment(c clientLike, name string) (string, error) {
	raw, err := c.Get(fmt.Sprintf("/api/v2/move/%s/", name), nil)
	if err != nil {
		return "", err
	}
	var doc struct {
		Meta *struct {
			Ailment *struct{ Name string } `json:"ailment"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return "", err
	}
	if doc.Meta == nil || doc.Meta.Ailment == nil {
		return "", nil
	}
	return doc.Meta.Ailment.Name, nil
}

func isMoveSuperEffective(c clientLike, attackType, targetType string) (bool, error) {
	raw, err := c.Get(fmt.Sprintf("/api/v2/type/%s/", attackType), nil)
	if err != nil {
		return false, err
	}
	var doc struct {
		DamageRelations struct {
			DoubleDamageTo []struct {
				Name string `json:"name"`
			} `json:"double_damage_to"`
		} `json:"damage_relations"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return false, err
	}
	for _, t := range doc.DamageRelations.DoubleDamageTo {
		if t.Name == targetType {
			return true, nil
		}
	}
	return false, nil
}

// newClientFromConfig wraps flags.newClient for helpers that don't get flags
// passed in. A no-flags client is appropriate here since these helpers
// only do read GETs.
func newClientFromConfig() (clientLike, error) {
	flags := &rootFlags{}
	return flags.newClient()
}

// extend moveData (in damage.go) with PP. Adding the field via interface here
// would conflict; instead loadMove returns the basic moveData and any caller
// needing PP fetches it separately. Kept minimal to avoid touching damage.go.
//
// (Unused field is acceptable here — keeps moveData compatible with damage.go.)
var _ = moveDataExt{}
