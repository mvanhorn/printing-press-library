package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/cre-owner/internal/store"

	"github.com/spf13/cobra"
)

func newNetworkCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var depth int

	cmd := &cobra.Command{
		Use:   "network [entity-name]",
		Short: "Discover hidden partnerships by finding LLCs that share officers, registered agents, or mailing addresses",
		Long: `Map the co-ownership network for a given entity by finding other LLCs that
share officers, registered agents, or mailing addresses. Reveals hidden
partnerships and beneficial ownership structures that are not visible from
public filings alone.`,
		Example: strings.Trim(`
  cre-owner-pp-cli network "Lakefront Holdings LLC" --json
  cre-owner-pp-cli network "Midwest Realty" --depth 3
  cre-owner-pp-cli network "ABC Properties" --select name,connection`, "\n"),
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
			results, err := buildNetwork(db, entityName, depth)
			if err != nil {
				return err
			}

			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&depth, "depth", 2, "Network traversal depth")
	return cmd
}

type networkResult struct {
	SeedEntity string        `json:"seed_entity"`
	Nodes      []networkNode `json:"nodes"`
	TotalNodes int           `json:"total_nodes"`
}

type networkNode struct {
	Name       string            `json:"name"`
	ID         string            `json:"id"`
	Connection string            `json:"connection"`
	Depth      int               `json:"depth"`
	Properties []json.RawMessage `json:"properties,omitempty"`
}

func buildNetwork(db *store.Store, entityName string, maxDepth int) (*networkResult, error) {
	// Step 1: Find the seed entity
	seedEntities, err := findEntitiesByName(db, entityName)
	if err != nil {
		return nil, fmt.Errorf("searching entities: %w", err)
	}
	if len(seedEntities) == 0 {
		return nil, fmt.Errorf("no entities found matching %q — run 'sync' first to populate the local store", entityName)
	}

	seen := map[string]bool{}
	var nodes []networkNode

	type queueItem struct {
		id    string
		name  string
		depth int
	}
	queue := make([]queueItem, 0)

	// Seed the queue with matched entities
	for _, e := range seedEntities {
		id := extractStringField(e, "id", "name")
		name := extractStringField(e, "name", "entity_name")
		if id != "" && !seen[id] {
			seen[id] = true
			queue = append(queue, queueItem{id: id, name: name, depth: 0})

			// Find properties for the seed entity
			parcels := findParcelsRaw(db, name)
			nodes = append(nodes, networkNode{
				Name:       name,
				ID:         id,
				Connection: "seed",
				Depth:      0,
				Properties: parcels,
			})
		}
	}

	// BFS traversal
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current.depth >= maxDepth {
			continue
		}

		// Find officers for this entity
		officers, err := findOfficersForEntity(db, current.id)
		if err == nil {
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
					leName := extractStringField(le, "name", "entity_name")
					if leID != "" && !seen[leID] {
						seen[leID] = true
						queue = append(queue, queueItem{id: leID, name: leName, depth: current.depth + 1})
						parcels := findParcelsRaw(db, leName)
						nodes = append(nodes, networkNode{
							Name:       leName,
							ID:         leID,
							Connection: fmt.Sprintf("shared officer: %s (via %s)", officerName, current.name),
							Depth:      current.depth + 1,
							Properties: parcels,
						})
					}
				}
			}
		}

		// Find entities sharing a registered agent
		agent := findRegisteredAgent(db, current.id)
		if agent != "" {
			linkedEntities, err := findEntitiesByAgent(db, agent)
			if err == nil {
				for _, le := range linkedEntities {
					leID := extractStringField(le, "id", "name")
					leName := extractStringField(le, "name", "entity_name")
					if leID != "" && !seen[leID] {
						seen[leID] = true
						queue = append(queue, queueItem{id: leID, name: leName, depth: current.depth + 1})
						parcels := findParcelsRaw(db, leName)
						nodes = append(nodes, networkNode{
							Name:       leName,
							ID:         leID,
							Connection: fmt.Sprintf("shared agent: %s (via %s)", agent, current.name),
							Depth:      current.depth + 1,
							Properties: parcels,
						})
					}
				}
			}
		}

		// Find entities sharing a mailing address
		address := findMailingAddress(db, current.id)
		if address != "" {
			linkedEntities, err := findEntitiesByAddress(db, address)
			if err == nil {
				for _, le := range linkedEntities {
					leID := extractStringField(le, "id", "name")
					leName := extractStringField(le, "name", "entity_name")
					if leID != "" && !seen[leID] {
						seen[leID] = true
						queue = append(queue, queueItem{id: leID, name: leName, depth: current.depth + 1})
						parcels := findParcelsRaw(db, leName)
						nodes = append(nodes, networkNode{
							Name:       leName,
							ID:         leID,
							Connection: fmt.Sprintf("shared address: %s (via %s)", address, current.name),
							Depth:      current.depth + 1,
							Properties: parcels,
						})
					}
				}
			}
		}
	}

	return &networkResult{
		SeedEntity: entityName,
		Nodes:      nodes,
		TotalNodes: len(nodes),
	}, nil
}

func findParcelsRaw(db *store.Store, ownerName string) []json.RawMessage {
	parcels, err := findParcelsForOwner(db, ownerName)
	if err != nil || len(parcels) == 0 {
		return nil
	}
	var raw []json.RawMessage
	for _, p := range parcels {
		b, err := json.Marshal(p)
		if err == nil {
			raw = append(raw, b)
		}
	}
	return raw
}

func findMailingAddress(db *store.Store, entityID string) string {
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
	for _, field := range []string{"mailing_address", "mailingAddress", "address", "principal_address", "principalAddress"} {
		if v, ok := obj[field]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

func findEntitiesByAddress(db *store.Store, address string) ([]map[string]any, error) {
	rows, err := db.DB().Query(
		`SELECT data FROM resources WHERE resource_type = 'entities'
		 AND (LOWER(json_extract(data, '$.mailing_address')) = LOWER(?)
		   OR LOWER(json_extract(data, '$.mailingAddress')) = LOWER(?)
		   OR LOWER(json_extract(data, '$.address')) = LOWER(?)
		   OR LOWER(json_extract(data, '$.principal_address')) = LOWER(?)
		   OR LOWER(json_extract(data, '$.principalAddress')) = LOWER(?))`,
		address, address, address, address, address,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJSONRows(rows)
}
