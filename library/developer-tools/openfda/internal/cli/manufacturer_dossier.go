package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/openfda/internal/store"
	"github.com/spf13/cobra"
)

func newManufacturerCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "manufacturer",
		Short: "Manufacturer-level intelligence across all FDA data",
	}

	cmd.AddCommand(newManufacturerDossierCmd(flags))
	return cmd
}

func newManufacturerDossierCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "dossier <firm_name>",
		Short: "Build a cross-domain dossier for a manufacturer/firm",
		Long: `Aggregate data from drug-recalls, device-recalls, food-recalls,
drug-events, device-events, and device-510k for a specific firm or manufacturer.
Produces a comprehensive view of a manufacturer's FDA footprint.`,
		Example: `  # Get dossier for a firm
  openfda-pp-cli manufacturer dossier "PFIZER"

  # JSON output
  openfda-pp-cli manufacturer dossier "JOHNSON" --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			firmName := strings.Join(args, " ")
			if dryRunOK(flags) {
				return nil
			}

			if dbPath == "" {
				dbPath = defaultDBPath("openfda-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'openfda-pp-cli sync' first.", err)
			}
			defer db.Close()

			firmUpper := strings.ToUpper(firmName)

			type categoryResult struct {
				Category    string                   `json:"category"`
				Count       int                      `json:"count"`
				RecentItems []map[string]interface{}  `json:"recent_items,omitempty"`
			}

			var categories []categoryResult

			// Drug recalls
			drugRecalls := queryDossierCategory(db, "drug-recalls", firmUpper,
				[]dossierSearchField{
					{jsonPath: "$.recalling_firm", matchType: "like"},
				},
				[]string{"recalling_firm", "product_description", "classification", "report_date", "status", "reason_for_recall"},
				5,
			)
			if drugRecalls.Count > 0 {
				categories = append(categories, drugRecalls)
			}

			// Device recalls
			deviceRecalls := queryDossierCategory(db, "device-recalls", firmUpper,
				[]dossierSearchField{
					{jsonPath: "$.recalling_firm", matchType: "like"},
				},
				[]string{"recalling_firm", "product_description", "classification", "report_date", "status"},
				5,
			)
			if deviceRecalls.Count > 0 {
				categories = append(categories, deviceRecalls)
			}

			// Food recalls
			foodRecalls := queryDossierCategory(db, "food-recalls", firmUpper,
				[]dossierSearchField{
					{jsonPath: "$.recalling_firm", matchType: "like"},
				},
				[]string{"recalling_firm", "product_description", "classification", "report_date", "status"},
				5,
			)
			if foodRecalls.Count > 0 {
				categories = append(categories, foodRecalls)
			}

			// Drug events (companynumber or via drug manufacturer)
			drugEvents := queryDossierCategoryEvents(db, "drug-events", firmUpper, 5)
			if drugEvents.Count > 0 {
				categories = append(categories, drugEvents)
			}

			// Device events
			deviceEvents := queryDossierCategoryDeviceEvents(db, "device-events", firmUpper, 5)
			if deviceEvents.Count > 0 {
				categories = append(categories, deviceEvents)
			}

			// Device 510k
			device510k := queryDossierCategory(db, "device-510k", firmUpper,
				[]dossierSearchField{
					{jsonPath: "$.applicant", matchType: "like"},
				},
				[]string{"applicant", "device_name", "decision_date", "decision_description", "clearance_type"},
				5,
			)
			if device510k.Count > 0 {
				categories = append(categories, device510k)
			}

			totalItems := 0
			for _, c := range categories {
				totalItems += c.Count
			}

			result := map[string]interface{}{
				"firm":       firmName,
				"total_items": totalItems,
				"categories":  categories,
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Manufacturer Dossier: %s\n", bold(firmName))
			fmt.Fprintf(cmd.OutOrStdout(), "Total items across all categories: %d\n\n", totalItems)

			if len(categories) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No records found for this manufacturer.")
				return nil
			}

			for _, cat := range categories {
				fmt.Fprintf(cmd.OutOrStdout(), "%s (%d records)\n", bold(cat.Category), cat.Count)
				if len(cat.RecentItems) > 0 {
					tw := newTabWriter(cmd.OutOrStdout())
					// Print headers from first item
					var headers []string
					for k := range cat.RecentItems[0] {
						headers = append(headers, k)
					}
					if len(headers) > 5 {
						headers = headers[:5]
					}
					fmt.Fprintln(tw, strings.Join(headers, "\t"))
					for _, item := range cat.RecentItems {
						var vals []string
						for _, h := range headers {
							v := fmt.Sprintf("%v", item[h])
							vals = append(vals, truncate(v, 30))
						}
						fmt.Fprintln(tw, strings.Join(vals, "\t"))
					}
					tw.Flush()
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

type dossierSearchField struct {
	jsonPath  string
	matchType string // "like" or "exact"
}

func queryDossierCategory(db *store.Store, resourceType, firmUpper string, fields []dossierSearchField, extractFields []string, limit int) struct {
	Category    string                   `json:"category"`
	Count       int                      `json:"count"`
	RecentItems []map[string]interface{} `json:"recent_items,omitempty"`
} {
	result := struct {
		Category    string                   `json:"category"`
		Count       int                      `json:"count"`
		RecentItems []map[string]interface{} `json:"recent_items,omitempty"`
	}{Category: resourceType}

	var conditions []string
	var queryArgs []interface{}
	queryArgs = append(queryArgs, resourceType)
	for _, f := range fields {
		conditions = append(conditions, fmt.Sprintf("UPPER(json_extract(r.data, '%s')) LIKE ?", f.jsonPath))
		queryArgs = append(queryArgs, "%"+firmUpper+"%")
	}

	where := strings.Join(conditions, " OR ")

	// Count
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM resources r WHERE r.resource_type = ? AND (%s)", where)
	var count int
	row, err := db.Query(countQuery, queryArgs...)
	if err == nil {
		if row.Next() {
			row.Scan(&count)
		}
		row.Close()
	}
	result.Count = count

	// Recent items
	dataQuery := fmt.Sprintf("SELECT r.data FROM resources r WHERE r.resource_type = ? AND (%s) ORDER BY r.updated_at DESC LIMIT ?", where)
	queryArgs = append(queryArgs, limit)
	rows, err := db.Query(dataQuery, queryArgs...)
	if err == nil {
		for rows.Next() {
			var dataStr string
			if err := rows.Scan(&dataStr); err != nil {
				continue
			}
			var full map[string]interface{}
			if err := json.Unmarshal([]byte(dataStr), &full); err != nil {
				continue
			}
			extracted := make(map[string]interface{})
			for _, field := range extractFields {
				if v, ok := full[field]; ok {
					extracted[field] = v
				}
			}
			result.RecentItems = append(result.RecentItems, extracted)
		}
		rows.Close()
	}

	return result
}

func queryDossierCategoryEvents(db *store.Store, resourceType, firmUpper string, limit int) struct {
	Category    string                   `json:"category"`
	Count       int                      `json:"count"`
	RecentItems []map[string]interface{} `json:"recent_items,omitempty"`
} {
	result := struct {
		Category    string                   `json:"category"`
		Count       int                      `json:"count"`
		RecentItems []map[string]interface{} `json:"recent_items,omitempty"`
	}{Category: resourceType}

	// Search in companynumb field (FDA's actual FAERS field name)
	countQuery := `SELECT COUNT(*) FROM resources r WHERE r.resource_type = ? AND UPPER(json_extract(r.data, '$.companynumb')) LIKE ?`
	var count int
	row, err := db.Query(countQuery, resourceType, "%"+firmUpper+"%")
	if err == nil {
		if row.Next() {
			row.Scan(&count)
		}
		row.Close()
	}
	result.Count = count

	dataQuery := `SELECT r.data FROM resources r WHERE r.resource_type = ? AND UPPER(json_extract(r.data, '$.companynumb')) LIKE ? ORDER BY r.updated_at DESC LIMIT ?`
	rows, err := db.Query(dataQuery, resourceType, "%"+firmUpper+"%", limit)
	if err == nil {
		for rows.Next() {
			var dataStr string
			if err := rows.Scan(&dataStr); err != nil {
				continue
			}
			var full map[string]interface{}
			if err := json.Unmarshal([]byte(dataStr), &full); err != nil {
				continue
			}
			extracted := map[string]interface{}{
				"safetyreportid": full["safetyreportid"],
				"receiptdate":    full["receiptdate"],
				"serious":        full["serious"],
			}
			result.RecentItems = append(result.RecentItems, extracted)
		}
		rows.Close()
	}

	return result
}

func queryDossierCategoryDeviceEvents(db *store.Store, resourceType, firmUpper string, limit int) struct {
	Category    string                   `json:"category"`
	Count       int                      `json:"count"`
	RecentItems []map[string]interface{} `json:"recent_items,omitempty"`
} {
	result := struct {
		Category    string                   `json:"category"`
		Count       int                      `json:"count"`
		RecentItems []map[string]interface{} `json:"recent_items,omitempty"`
	}{Category: resourceType}

	// Search in device array for manufacturer name
	countQuery := `
		SELECT COUNT(DISTINCT r.id)
		FROM resources r, json_each(json_extract(r.data, '$.device')) je
		WHERE r.resource_type = ?
		AND UPPER(json_extract(je.value, '$.manufacturer_d_name')) LIKE ?
	`
	var count int
	row, err := db.Query(countQuery, resourceType, "%"+firmUpper+"%")
	if err == nil {
		if row.Next() {
			row.Scan(&count)
		}
		row.Close()
	}
	result.Count = count

	dataQuery := `
		SELECT DISTINCT r.data
		FROM resources r, json_each(json_extract(r.data, '$.device')) je
		WHERE r.resource_type = ?
		AND UPPER(json_extract(je.value, '$.manufacturer_d_name')) LIKE ?
		ORDER BY r.updated_at DESC
		LIMIT ?
	`
	rows, err := db.Query(dataQuery, resourceType, "%"+firmUpper+"%", limit)
	if err == nil {
		for rows.Next() {
			var dataStr string
			if err := rows.Scan(&dataStr); err != nil {
				continue
			}
			var full map[string]interface{}
			if err := json.Unmarshal([]byte(dataStr), &full); err != nil {
				continue
			}
			extracted := map[string]interface{}{
				"mdr_report_key": full["mdr_report_key"],
				"date_received":  full["date_received"],
			}
			result.RecentItems = append(result.RecentItems, extracted)
		}
		rows.Close()
	}

	return result
}
