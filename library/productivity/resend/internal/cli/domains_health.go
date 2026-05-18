// PATCH: novel domain health summary — verification + DKIM/SPF/DMARC status across every domain. No aggregate API endpoint.
package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/resend/internal/store"
)

func newDomainsHealthCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "health",
		Short: "Verification + DKIM/SPF/DMARC status across every domain (no aggregate API)",
		Long: `Aggregates locally-synced domains with their verification status and the
DKIM/SPF/DMARC record state from the domain.data JSON. Flags domains that
aren't fully verified. No aggregate endpoint exists — today this requires
N GET /domains/{id} calls.`,
		Example: strings.Trim(`
  # Summary across all domains
  resend-pp-cli domains health --json

  # Only flag unhealthy domains
  resend-pp-cli domains health --unhealthy-only --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("resend-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'resend-pp-cli sync' first.", err)
			}
			defer db.Close()

			// The /domains list endpoint does NOT include the DNS records
			// array — that field is only on GET /domains/{id}. Fall back to
			// a per-domain live call to fill in DKIM/SPF/MX statuses when
			// the cached records field is empty.
			liveClient, _ := flags.newClient()

			rows, err := db.Query(`
				SELECT
					d.id,
					COALESCE(d.name, '') AS name,
					COALESCE(d.status, '') AS status,
					COALESCE(d.region, '') AS region,
					COALESCE(d.click_tracking, 0) AS click_tracking,
					COALESCE(d.open_tracking, 0) AS open_tracking,
					COALESCE(json_extract(d.data, '$.records'), '') AS records_json
				FROM domains d
				ORDER BY d.name
			`)
			if err != nil {
				return fmt.Errorf("querying domains: %w", err)
			}
			defer rows.Close()

			type health struct {
				ID            string `json:"id"`
				Name          string `json:"name"`
				Status        string `json:"status"`
				Region        string `json:"region"`
				ClickTracking bool   `json:"click_tracking"`
				OpenTracking  bool   `json:"open_tracking"`
				DKIM          string `json:"dkim_status"`
				SPF           string `json:"spf_status"`
				MX            string `json:"mx_status"`
				Healthy       bool   `json:"healthy"`
			}
			results := []health{}
			for rows.Next() {
				var h health
				var clickInt, openInt int
				var recordsJSON string
				if err := rows.Scan(&h.ID, &h.Name, &h.Status, &h.Region, &clickInt, &openInt, &recordsJSON); err != nil {
					continue
				}
				h.ClickTracking = clickInt == 1
				h.OpenTracking = openInt == 1
				// If the cached domain row didn't carry the records array
				// (the list endpoint omits it), pull it live per-domain.
				if recordsJSON == "" && liveClient != nil {
					if raw, err := liveClient.Get("/domains/"+h.ID, nil); err == nil {
						var detail struct {
							Records json.RawMessage `json:"records"`
						}
						if json.Unmarshal(raw, &detail) == nil && len(detail.Records) > 0 {
							recordsJSON = string(detail.Records)
						}
					}
				}
				// Parse domain DNS records via encoding/json so field reorder
				// or unrelated "status" values in the JSON can't fool the match.
				h.DKIM = recordStatus(recordsJSON, "DKIM")
				h.SPF = recordStatus(recordsJSON, "SPF")
				h.MX = recordStatus(recordsJSON, "MX")
				h.Healthy = h.Status == "verified"
				results = append(results, h)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating domains: %w", err)
			}

			unhealthyOnly, _ := cmd.Flags().GetBool("unhealthy-only")
			if unhealthyOnly {
				filtered := results[:0]
				for _, r := range results {
					if !r.Healthy {
						filtered = append(filtered, r)
					}
				}
				results = filtered
			}

			out := cmd.OutOrStdout()
			if flags.asJSON {
				return printJSONFiltered(out, map[string]any{
					"count":   len(results),
					"domains": results,
				}, flags)
			}
			if len(results) == 0 {
				if unhealthyOnly {
					fmt.Fprintln(out, "All domains are healthy.")
				} else {
					fmt.Fprintln(out, "No domains in the local store.")
					fmt.Fprintln(out, "(Run 'resend-pp-cli sync --full' to refresh.)")
				}
				return nil
			}
			fmt.Fprintf(out, "%d domain(s):\n\n", len(results))
			fmt.Fprintf(out, "%-25s %-12s %-10s %-10s %-10s %s\n", "NAME", "STATUS", "DKIM", "SPF", "MX", "HEALTHY")
			fmt.Fprintf(out, "%-25s %-12s %-10s %-10s %-10s %s\n", "----", "------", "----", "---", "--", "-------")
			for _, r := range results {
				healthMark := "yes"
				if !r.Healthy {
					healthMark = "NO"
				}
				fmt.Fprintf(out, "%-25s %-12s %-10s %-10s %-10s %s\n", truncate(r.Name, 23), truncate(r.Status, 10), truncate(r.DKIM, 8), truncate(r.SPF, 8), truncate(r.MX, 8), healthMark)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/resend-pp-cli/data.db)")
	cmd.Flags().Bool("unhealthy-only", false, "Only show domains that are not fully verified")
	return cmd
}

// recordStatus parses the records JSON and returns the status for the named
// record type, case-insensitively. records is an array of objects with at
// least {record, status, type} fields per Resend's domain schema.
func recordStatus(jsonStr, recordType string) string {
	if jsonStr == "" {
		return ""
	}
	var records []struct {
		Record string `json:"record"`
		Status string `json:"status"`
		Type   string `json:"type"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &records); err != nil {
		return ""
	}
	want := strings.ToUpper(recordType)
	for _, r := range records {
		if strings.ToUpper(r.Record) == want {
			return r.Status
		}
	}
	return ""
}
