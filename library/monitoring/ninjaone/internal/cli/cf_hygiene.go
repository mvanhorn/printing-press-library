// Copyright 2026 "Chris Carson" and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// pp:data-source live

type cfHygieneRow struct {
	Scope      string   `json:"scope"` // device|org
	EntityID   int64    `json:"entityId"`
	EntityName string   `json:"entityName"`
	Missing    []string `json:"missing"`
}

type cfHygieneView struct {
	Scope        string         `json:"scope"`
	Required     []string       `json:"required"`
	Rows         []cfHygieneRow `json:"rows"`
	Count        int            `json:"count"`
	ScannedPages int            `json:"scanned_pages"`
	MaxScanPages int            `json:"max_scan_pages"`
	Skipped      int            `json:"skipped_out_of_scope"`
	Note         string         `json:"note,omitempty"`
}

func newNovelCfHygieneCmd(flags *rootFlags) *cobra.Command {
	var (
		flagRequire  string
		flagScope    string
		flagLimit    int
		flagMaxPages int
	)

	cmd := &cobra.Command{
		Use:   "cf-hygiene",
		Short: "Find devices and organizations missing required custom-field values (asset tag, warranty, owner) fleet-wide.",
		Long: `Fetch scoped custom-field values and report which entities are missing any of the
required field names (--require). Scope can be device, org, or both.

Examples:
  ninjaone-pp-cli cf-hygiene --require "Asset Tag,Owner"
  ninjaone-pp-cli cf-hygiene --require warrantyExpiration --scope device
  ninjaone-pp-cli cf-hygiene --require assetTag --scope both --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitDryRunPreview(cmd, flags, "would fetch scoped custom-field values and report entities missing required fields")
			}

			required := parseCSVList(flagRequire)
			if len(required) == 0 {
				return usageErr(fmt.Errorf("--require is required (comma-separated list of custom-field names)"))
			}
			scope := strings.ToLower(strings.TrimSpace(flagScope))
			if scope == "" {
				scope = "both"
			}
			switch scope {
			case "device", "org", "both":
			default:
				return usageErr(fmt.Errorf("--scope must be one of device|org|both, got %q", flagScope))
			}

			maxPages := effectiveMaxScanPages(flagMaxPages)
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			orgs, err := fetchOrgs(ctx, c)
			if err != nil {
				return err
			}
			devices, _, err := fetchDevices(ctx, c, "", maxPages)
			if err != nil {
				return err
			}
			byID, _ := deviceOrgIndex(devices)

			cfRows, scanned, err := fetchScopedCustomFields(ctx, c, "", maxPages)
			if err != nil {
				return err
			}

			rows := make([]cfHygieneRow, 0)
			skipped := 0
			for _, r := range cfRows {
				// Determine scope label + entity id. Scoped report uses
				// scope+entityId; device report uses deviceId. Be defensive.
				var rowScope string
				var entityID int64
				switch strings.ToUpper(r.Scope) {
				case "NODE":
					rowScope, entityID = "device", r.EntityID
				case "ORGANIZATION":
					rowScope, entityID = "org", r.EntityID
				case "":
					if r.DeviceID != 0 {
						rowScope, entityID = "device", r.DeviceID
					} else {
						skipped++
						continue // location/end_user or unknown — skip
					}
				default:
					skipped++
					continue // LOCATION / END_USER — out of scope
				}

				if scope != "both" && scope != rowScope {
					continue
				}

				missing := missingRequiredFields(r.Fields, required)
				if len(missing) == 0 {
					continue
				}

				name := ""
				if rowScope == "device" {
					name = byID[entityID].bestName()
				} else {
					name = orgs[entityID]
				}
				rows = append(rows, cfHygieneRow{
					Scope:      rowScope,
					EntityID:   entityID,
					EntityName: name,
					Missing:    missing,
				})
			}

			sort.SliceStable(rows, func(i, j int) bool {
				if rows[i].Scope != rows[j].Scope {
					return rows[i].Scope < rows[j].Scope
				}
				return rows[i].EntityID < rows[j].EntityID
			})
			if n := boundLimit(len(rows), flagLimit); n < len(rows) {
				rows = rows[:n]
			}

			view := cfHygieneView{
				Scope:        scope,
				Required:     required,
				Rows:         rows,
				Count:        len(rows),
				ScannedPages: scanned,
				MaxScanPages: maxPages,
				Skipped:      skipped,
			}
			if len(rows) == 0 {
				view.Note = "no entities missing the required custom fields within max-scan-pages"
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			headers := []string{"SCOPE", "ENTITY", "NAME", "MISSING"}
			tableRows := make([][]string, 0, len(rows))
			for _, r := range rows {
				tableRows = append(tableRows, []string{r.Scope, strconv.FormatInt(r.EntityID, 10), r.EntityName, strings.Join(r.Missing, ", ")})
			}
			return flags.printTable(cmd, headers, tableRows)
		},
	}
	cmd.Flags().StringVar(&flagRequire, "require", "", "Comma-separated custom-field names that must be present (REQUIRED)")
	cmd.Flags().StringVar(&flagScope, "scope", "both", "Entity scope to check: device|org|both")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Maximum number of rows to return (0 = all)")
	cmd.Flags().IntVar(&flagMaxPages, "max-scan-pages", 5, "Maximum API pages to scan")
	return cmd
}
