package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/cre-owner/internal/store"
	"github.com/spf13/cobra"
)

func newPackageCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "package [entity-name]",
		Short: "Generate a portfolio dossier for a target owner",
		Long: `Aggregate all available data about an entity into a structured portfolio
dossier: property count, total assessed value, tax exposure, entity status,
officers, and contacts.

Searches the local store for the entity by name, then gathers all related
parcels, tax records, contacts, and entity chain data.

Data comes from the local SQLite store. Run 'sync' first to populate.`,
		Example: strings.Trim(`
  cre-owner-pp-cli package "Acme Properties LLC"
  cre-owner-pp-cli package "Smith Holdings" --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			entityName := args[0]

			if dbPath == "" {
				dbPath = defaultDBPath("cre-owner-pp-cli")
			}
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			db := s.DB()

			// Find the entity by name.
			var entityInfo map[string]any
			entityRows, err := db.Query(`
				SELECT data FROM resources
				WHERE resource_type = 'entities'
				  AND LOWER(json_extract(data, '$.name')) LIKE LOWER(?)`,
				"%"+entityName+"%",
			)
			if err != nil {
				return fmt.Errorf("querying entities: %w", err)
			}
			defer entityRows.Close()

			if entityRows.Next() {
				var data string
				if err := entityRows.Scan(&data); err != nil {
					return fmt.Errorf("scanning entity: %w", err)
				}
				if err := json.Unmarshal([]byte(data), &entityInfo); err != nil {
					return fmt.Errorf("parsing entity: %w", err)
				}
			}
			entityRows.Close()

			if entityInfo == nil {
				return fmt.Errorf("entity %q not found in local store; run 'sync' first", entityName)
			}

			resolvedName, _ := entityInfo["name"].(string)
			entityID, _ := entityInfo["entity_id"].(string)

			// Find all parcels owned by this entity.
			parcelRows, err := db.Query(`
				SELECT data FROM resources
				WHERE resource_type = 'parcels'
				  AND LOWER(json_extract(data, '$.owner_name')) LIKE LOWER(?)`,
				"%"+resolvedName+"%",
			)
			if err != nil {
				return fmt.Errorf("querying parcels: %w", err)
			}
			defer parcelRows.Close()

			var properties []map[string]any
			var totalAssessed, totalSqft float64
			var propertyTypes = map[string]int{}
			var markets = map[string]int{}

			for parcelRows.Next() {
				var data string
				if err := parcelRows.Scan(&data); err != nil {
					continue
				}
				var parcel map[string]any
				if err := json.Unmarshal([]byte(data), &parcel); err != nil {
					continue
				}
				properties = append(properties, parcel)
				if v, ok := parcel["assessed_value"].(float64); ok {
					totalAssessed += v
				}
				if v, ok := parcel["sqft"].(float64); ok {
					totalSqft += v
				}
				if pt, ok := parcel["property_type"].(string); ok && pt != "" {
					propertyTypes[pt]++
				}
				if m, ok := parcel["market"].(string); ok && m != "" {
					markets[m]++
				}
			}
			parcelRows.Close()

			// Get tax records for the entity's properties.
			taxRows, err := db.Query(`
				SELECT data FROM resources
				WHERE resource_type = 'tax_records'
				  AND LOWER(json_extract(data, '$.owner_name')) LIKE LOWER(?)`,
				"%"+resolvedName+"%",
			)
			if err != nil {
				return fmt.Errorf("querying tax records: %w", err)
			}
			defer taxRows.Close()

			var taxRecords []map[string]any
			var totalTaxDue float64
			var delinquentCount int
			for taxRows.Next() {
				var data string
				if err := taxRows.Scan(&data); err != nil {
					continue
				}
				var rec map[string]any
				if err := json.Unmarshal([]byte(data), &rec); err != nil {
					continue
				}
				taxRecords = append(taxRecords, rec)
				if v, ok := rec["amount_due"].(float64); ok {
					totalTaxDue += v
				}
				if status, ok := rec["status"].(string); ok && strings.EqualFold(status, "delinquent") {
					delinquentCount++
				}
			}
			taxRows.Close()

			// Get contacts associated with this entity.
			var contacts []map[string]any
			contactQuery := `
				SELECT data FROM resources
				WHERE resource_type = 'contacts'
				  AND (LOWER(json_extract(data, '$.owner_id')) LIKE LOWER(?)
				    OR LOWER(json_extract(data, '$.entity_id')) LIKE LOWER(?))`

			contactArgs := []any{"%" + resolvedName + "%"}
			if entityID != "" {
				contactArgs = append(contactArgs, "%"+entityID+"%")
			} else {
				contactArgs = append(contactArgs, "%"+resolvedName+"%")
			}

			contactRows, err := db.Query(contactQuery, contactArgs...)
			if err != nil {
				return fmt.Errorf("querying contacts: %w", err)
			}
			defer contactRows.Close()

			for contactRows.Next() {
				var data string
				if err := contactRows.Scan(&data); err != nil {
					continue
				}
				var contact map[string]any
				if err := json.Unmarshal([]byte(data), &contact); err != nil {
					continue
				}
				contacts = append(contacts, contact)
			}
			contactRows.Close()

			// Get officers for this entity.
			var officers []map[string]any
			officerRows, err := db.Query(`
				SELECT data FROM resources
				WHERE resource_type = 'entity_officers'
				  AND (LOWER(json_extract(data, '$.entity_id')) LIKE LOWER(?)
				    OR LOWER(json_extract(data, '$.entity_name')) LIKE LOWER(?))`,
				"%"+entityID+"%",
				"%"+resolvedName+"%",
			)
			if err != nil {
				return fmt.Errorf("querying officers: %w", err)
			}
			defer officerRows.Close()

			for officerRows.Next() {
				var data string
				if err := officerRows.Scan(&data); err != nil {
					continue
				}
				var officer map[string]any
				if err := json.Unmarshal([]byte(data), &officer); err != nil {
					continue
				}
				officers = append(officers, officer)
			}
			officerRows.Close()

			// Build the dossier.
			dossier := map[string]any{
				"entity_info": entityInfo,
				"portfolio_summary": map[string]any{
					"property_count":       len(properties),
					"total_assessed_value": math.Round(totalAssessed*100) / 100,
					"total_sqft":           totalSqft,
					"property_types":       propertyTypes,
					"markets":              markets,
				},
				"properties": properties,
				"tax_exposure": map[string]any{
					"total_tax_due":    math.Round(totalTaxDue*100) / 100,
					"delinquent_count": delinquentCount,
					"tax_records":      taxRecords,
				},
				"contacts":     contacts,
				"entity_chain": officers,
			}

			return printJSONFiltered(cmd.OutOrStdout(), dossier, flags)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/cre-owner-pp-cli/data.db)")

	return cmd
}
