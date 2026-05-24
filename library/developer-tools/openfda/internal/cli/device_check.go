package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/openfda/internal/store"
	"github.com/spf13/cobra"
)

func newDeviceCheckCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var filePath string

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check device inventory against recalls and adverse events",
		Long: `Parse a CSV file containing device names or product codes and check
each against local device-recalls and device-events data.
Outputs a risk assessment per device.

CSV must have a header row with at least one of: product_code, device_name.`,
		Example: `  # Check devices from inventory file
  openfda-pp-cli device check --file inventory.csv

  # JSON output
  openfda-pp-cli device check --file devices.csv --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if filePath == "" {
				return usageErr(fmt.Errorf("--file is required"))
			}
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

			// Parse CSV
			f, err := os.Open(filePath)
			if err != nil {
				return fmt.Errorf("opening CSV file: %w", err)
			}
			defer f.Close()

			reader := csv.NewReader(f)
			headers, err := reader.Read()
			if err != nil {
				return fmt.Errorf("reading CSV header: %w", err)
			}

			// Find column indices
			productCodeIdx := -1
			deviceNameIdx := -1
			for i, h := range headers {
				h = strings.TrimSpace(strings.ToLower(h))
				switch h {
				case "product_code":
					productCodeIdx = i
				case "device_name":
					deviceNameIdx = i
				}
			}
			if productCodeIdx == -1 && deviceNameIdx == -1 {
				return usageErr(fmt.Errorf("CSV must have 'product_code' or 'device_name' column"))
			}

			type deviceRisk struct {
				DeviceName    string `json:"device_name"`
				ProductCode   string `json:"product_code,omitempty"`
				RecallCount   int    `json:"recall_count"`
				EventCount    int    `json:"event_count"`
				RiskLevel     string `json:"risk_level"`
				LatestRecall  string `json:"latest_recall,omitempty"`
			}

			var results []deviceRisk

			for {
				record, err := reader.Read()
				if err == io.EOF {
					break
				}
				if err != nil {
					continue
				}

				var deviceName, productCode string
				if deviceNameIdx >= 0 && deviceNameIdx < len(record) {
					deviceName = strings.TrimSpace(record[deviceNameIdx])
				}
				if productCodeIdx >= 0 && productCodeIdx < len(record) {
					productCode = strings.TrimSpace(record[productCodeIdx])
				}

				searchTerm := deviceName
				if searchTerm == "" {
					searchTerm = productCode
				}
				if searchTerm == "" {
					continue
				}

				searchUpper := strings.ToUpper(searchTerm)

				// Check device-recalls
				recallCount := 0
				latestRecall := ""
				recallQuery := `
					SELECT json_extract(r.data, '$.report_date')
					FROM resources r
					WHERE r.resource_type = 'device-recalls'
					AND (UPPER(json_extract(r.data, '$.product_description')) LIKE ?
					  OR UPPER(json_extract(r.data, '$.product_code')) LIKE ?)
					ORDER BY r.updated_at DESC
				`
				recallRows, err := db.Query(recallQuery, "%"+searchUpper+"%", "%"+searchUpper+"%")
				if err == nil {
					for recallRows.Next() {
						var rd string
						recallRows.Scan(&rd)
						recallCount++
						if latestRecall == "" {
							latestRecall = rd
						}
					}
					recallRows.Close()
				}

				// Check device-events — use a single query with OR to avoid
				// double-counting reports that match both brand_name and product_code.
				eventCount := 0
				if productCode != "" {
					pcUpper := strings.ToUpper(productCode)
					combinedQuery := `
						SELECT COUNT(DISTINCT r.id)
						FROM resources r, json_each(json_extract(r.data, '$.device')) je
						WHERE r.resource_type = 'device-events'
						AND (UPPER(json_extract(je.value, '$.brand_name')) LIKE ?
						  OR UPPER(json_extract(je.value, '$.device_report_product_code')) LIKE ?)
					`
					row, err := db.Query(combinedQuery, "%"+searchUpper+"%", "%"+pcUpper+"%")
					if err == nil {
						if row.Next() {
							row.Scan(&eventCount)
						}
						row.Close()
					}
				} else {
					eventQuery := `
						SELECT COUNT(DISTINCT r.id)
						FROM resources r, json_each(json_extract(r.data, '$.device')) je
						WHERE r.resource_type = 'device-events'
						AND UPPER(json_extract(je.value, '$.brand_name')) LIKE ?
					`
					row, err := db.Query(eventQuery, "%"+searchUpper+"%")
					if err == nil {
						if row.Next() {
							row.Scan(&eventCount)
						}
						row.Close()
					}
				}

				// Determine risk level
				riskLevel := "LOW"
				if recallCount > 0 && eventCount > 5 {
					riskLevel = "HIGH"
				} else if recallCount > 0 || eventCount > 2 {
					riskLevel = "MEDIUM"
				}

				results = append(results, deviceRisk{
					DeviceName:   deviceName,
					ProductCode:  productCode,
					RecallCount:  recallCount,
					EventCount:   eventCount,
					RiskLevel:    riskLevel,
					LatestRecall: latestRecall,
				})
			}

			// Summary
			highCount := 0
			medCount := 0
			for _, r := range results {
				switch r.RiskLevel {
				case "HIGH":
					highCount++
				case "MEDIUM":
					medCount++
				}
			}

			output := map[string]interface{}{
				"total_devices": len(results),
				"high_risk":     highCount,
				"medium_risk":   medCount,
				"low_risk":      len(results) - highCount - medCount,
				"devices":       results,
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(output)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Device Inventory Check\n")
			fmt.Fprintf(cmd.OutOrStdout(), "Devices: %d | High: %d | Medium: %d | Low: %d\n\n",
				len(results), highCount, medCount, len(results)-highCount-medCount)

			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No devices found in CSV.")
				return nil
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "DEVICE\tCODE\tRECALLS\tEVENTS\tRISK")
			for _, r := range results {
				fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%s\n",
					truncate(r.DeviceName, 30), r.ProductCode, r.RecallCount, r.EventCount, r.RiskLevel)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&filePath, "file", "", "Path to CSV inventory file (required)")

	return cmd
}
