package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/cre-owner/internal/store"

	"github.com/spf13/cobra"
)

func newPortfolioCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var depth int

	cmd := &cobra.Command{
		Use:   "portfolio [entity-name]",
		Short: "Find all buildings owned by the same beneficial owner across multiple LLCs",
		Long: `Find all parcels owned by a given entity AND any related entities that share
officers or registered agents. This is the "hidden portfolio rollup" — it
pierces multiple LLCs to reveal the full portfolio behind a single beneficial
owner.`,
		Example: strings.Trim(`
  cre-owner-pp-cli portfolio "Lakefront Holdings LLC" --json
  cre-owner-pp-cli portfolio "Midwest Realty" --depth 3
  cre-owner-pp-cli portfolio "ABC Properties" --select id,address,owner`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("cre-owner-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			entityName := args[0]
			results, err := buildPortfolio(db, entityName, depth)
			if err != nil {
				return err
			}

			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&depth, "depth", 2, "Traversal depth for related entity discovery")
	return cmd
}

type portfolioResult struct {
	SeedEntity      string            `json:"seed_entity"`
	RelatedEntities []relatedEntity   `json:"related_entities"`
	TotalParcels    int               `json:"total_parcels"`
	Parcels         []json.RawMessage `json:"parcels"`
}

type relatedEntity struct {
	Name       string `json:"name"`
	ID         string `json:"id"`
	Connection string `json:"connection"`
}

func buildPortfolio(db *store.Store, entityName string, maxDepth int) (*portfolioResult, error) {
	// Step 1: Find matching entities by name
	seedEntities, err := findEntitiesByName(db, entityName)
	if err != nil {
		return nil, fmt.Errorf("searching entities: %w", err)
	}
	if len(seedEntities) == 0 {
		return nil, fmt.Errorf("no entities found matching %q — run 'sync' first to populate the local store", entityName)
	}

	// Step 2: Traverse the entity network to find related entities
	seen := map[string]bool{}
	var related []relatedEntity
	queue := make([]string, 0)

	for _, e := range seedEntities {
		id := extractStringField(e, "id", "name")
		if id != "" && !seen[id] {
			seen[id] = true
			queue = append(queue, id)
			related = append(related, relatedEntity{
				Name:       extractStringField(e, "name"),
				ID:         id,
				Connection: "seed",
			})
		}
	}

	for d := 1; d < maxDepth && len(queue) > 0; d++ {
		var nextQueue []string
		for _, entityID := range queue {
			// Find officers for this entity
			officers, err := findOfficersForEntity(db, entityID)
			if err != nil {
				continue
			}
			for _, officer := range officers {
				officerName := extractStringField(officer, "name", "officer_name")
				if officerName == "" {
					continue
				}
				// Find other entities with the same officer
				linkedEntities, err := findEntitiesByOfficer(db, officerName)
				if err != nil {
					continue
				}
				for _, le := range linkedEntities {
					leID := extractStringField(le, "id", "name")
					if leID != "" && !seen[leID] {
						seen[leID] = true
						nextQueue = append(nextQueue, leID)
						related = append(related, relatedEntity{
							Name:       extractStringField(le, "name"),
							ID:         leID,
							Connection: fmt.Sprintf("shared officer: %s", officerName),
						})
					}
				}
			}

			// Find entities sharing a registered agent
			agent := findRegisteredAgent(db, entityID)
			if agent != "" {
				linkedEntities, err := findEntitiesByAgent(db, agent)
				if err == nil {
					for _, le := range linkedEntities {
						leID := extractStringField(le, "id", "name")
						if leID != "" && !seen[leID] {
							seen[leID] = true
							nextQueue = append(nextQueue, leID)
							related = append(related, relatedEntity{
								Name:       extractStringField(le, "name"),
								ID:         leID,
								Connection: fmt.Sprintf("shared agent: %s", agent),
							})
						}
					}
				}
			}
		}
		queue = nextQueue
	}

	// Step 3: Find all parcels owned by any of these entities
	var allParcels []json.RawMessage
	parcelSeen := map[string]bool{}
	for _, ent := range related {
		parcels, err := findParcelsForOwner(db, ent.Name)
		if err != nil {
			continue
		}
		for _, p := range parcels {
			pid := extractStringField(p, "id")
			if pid != "" && parcelSeen[pid] {
				continue
			}
			if pid != "" {
				parcelSeen[pid] = true
			}
			raw, _ := json.Marshal(p)
			allParcels = append(allParcels, raw)
		}
	}

	return &portfolioResult{
		SeedEntity:      entityName,
		RelatedEntities: related,
		TotalParcels:    len(allParcels),
		Parcels:         allParcels,
	}, nil
}

func findEntitiesByName(db *store.Store, name string) ([]map[string]any, error) {
	rows, err := db.DB().Query(
		`SELECT data FROM resources WHERE resource_type = 'entities'
		 AND (LOWER(json_extract(data, '$.name')) LIKE LOWER(?) OR LOWER(json_extract(data, '$.entity_name')) LIKE LOWER(?))`,
		"%"+name+"%", "%"+name+"%",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJSONRows(rows)
}

func findOfficersForEntity(db *store.Store, entityID string) ([]map[string]any, error) {
	rows, err := db.DB().Query(
		`SELECT data FROM resources WHERE resource_type = 'entity_officers'
		 AND (json_extract(data, '$.entity_id') = ? OR json_extract(data, '$.entity_name') = ?)`,
		entityID, entityID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJSONRows(rows)
}

func findEntitiesByOfficer(db *store.Store, officerName string) ([]map[string]any, error) {
	rows, err := db.DB().Query(
		`SELECT r.data FROM resources r
		 WHERE r.resource_type = 'entities'
		 AND r.id IN (
		   SELECT COALESCE(json_extract(data, '$.entity_id'), json_extract(data, '$.entity_name'))
		   FROM resources
		   WHERE resource_type = 'entity_officers'
		   AND LOWER(json_extract(data, '$.name')) = LOWER(?)
		 )`,
		officerName,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJSONRows(rows)
}

func findRegisteredAgent(db *store.Store, entityID string) string {
	var data string
	err := db.DB().QueryRow(
		`SELECT data FROM resources WHERE resource_type = 'entities' AND id = ?`,
		entityID,
	).Scan(&data)
	if err != nil {
		return ""
	}
	var obj map[string]any
	if json.Unmarshal([]byte(data), &obj) != nil {
		return ""
	}
	for _, field := range []string{"registered_agent", "registeredAgent", "agent_name", "agentName"} {
		if v, ok := obj[field]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

func findEntitiesByAgent(db *store.Store, agentName string) ([]map[string]any, error) {
	rows, err := db.DB().Query(
		`SELECT data FROM resources WHERE resource_type = 'entities'
		 AND (LOWER(json_extract(data, '$.registered_agent')) = LOWER(?)
		   OR LOWER(json_extract(data, '$.registeredAgent')) = LOWER(?)
		   OR LOWER(json_extract(data, '$.agent_name')) = LOWER(?)
		   OR LOWER(json_extract(data, '$.agentName')) = LOWER(?))`,
		agentName, agentName, agentName, agentName,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJSONRows(rows)
}

func findParcelsForOwner(db *store.Store, ownerName string) ([]map[string]any, error) {
	rows, err := db.DB().Query(
		`SELECT data FROM resources WHERE resource_type = 'parcels'
		 AND (LOWER(json_extract(data, '$.owner')) LIKE LOWER(?)
		   OR LOWER(json_extract(data, '$.owner_name')) LIKE LOWER(?)
		   OR LOWER(json_extract(data, '$.ownerName')) LIKE LOWER(?))`,
		"%"+ownerName+"%", "%"+ownerName+"%", "%"+ownerName+"%",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJSONRows(rows)
}

func scanJSONRows(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]map[string]any, error) {
	var results []map[string]any
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			continue
		}
		var obj map[string]any
		if json.Unmarshal([]byte(data), &obj) == nil {
			results = append(results, obj)
		}
	}
	return results, rows.Err()
}

func extractStringField(obj map[string]any, fields ...string) string {
	for _, f := range fields {
		if v, ok := obj[f]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}
