// Copyright 2026 "Chris Carson" and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// pp:data-source live

type patchGapRow struct {
	OrganizationID   int64  `json:"organizationId"`
	OrganizationName string `json:"organizationName"`
	DeviceID         int64  `json:"deviceId"`
	SystemName       string `json:"systemName"`
	KBNumber         string `json:"kbNumber"`
	Name             string `json:"name"`
	Severity         string `json:"severity"`
	Status           string `json:"status"`
	PatchType        string `json:"patchType"`
}

type patchGapsView struct {
	Rows         []patchGapRow `json:"rows"`
	Count        int           `json:"count"`
	ScannedPages int           `json:"scanned_pages"`
	MaxScanPages int           `json:"max_scan_pages"`
	Note         string        `json:"note,omitempty"`
}

func newNovelPatchGapsCmd(flags *rootFlags) *cobra.Command {
	var (
		flagSeverity string
		flagOrg      string
		flagType     string
		flagLimit    int
		flagMaxPages int
	)

	cmd := &cobra.Command{
		Use:   "patch-gaps",
		Short: "See every device still missing or failing a patch across all your client organizations in one fleet-wide view.",
		Long: `Fetch failed/missing OS (and optionally software) patches fleet-wide, join each
device to its organization, and emit one row per gap grouped by org then device.

Examples:
  ninjaone-pp-cli patch-gaps
  ninjaone-pp-cli patch-gaps --type all --severity critical
  ninjaone-pp-cli patch-gaps --org "Acme" --limit 50 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitDryRunPreview(cmd, flags, "would fetch failed/missing patches fleet-wide, join devices to organizations, and report patch gaps")
			}

			ptype := strings.ToLower(strings.TrimSpace(flagType))
			if ptype == "" {
				ptype = "os"
			}
			switch ptype {
			case "os", "software", "all":
			default:
				return usageErr(fmt.Errorf("--type must be one of os|software|all, got %q", flagType))
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

			patches := make([]njPatch, 0)
			scanned := 0
			if ptype == "os" || ptype == "all" {
				p, pg, err := fetchPatches(ctx, c, "/v2/queries/os-patches", "FAILED", flagSeverity, maxPages)
				if err != nil {
					return err
				}
				patches = append(patches, p...)
				scanned += pg
			}
			if ptype == "software" || ptype == "all" {
				p, pg, err := fetchPatches(ctx, c, "/v2/queries/software-patches", "FAILED", flagSeverity, maxPages)
				if err != nil {
					return err
				}
				patches = append(patches, p...)
				scanned += pg
			}

			rows := make([]patchGapRow, 0, len(patches))
			for _, p := range patches {
				dev := byID[p.DeviceID]
				orgID := dev.OrganizationID
				orgName := orgs[orgID]
				if !orgMatches(flagOrg, orgID, orgName) {
					continue
				}
				// Severity is already applied server-side via the API `severity`
				// param in fetchPatches; do not re-filter client-side with exact
				// equality, which silently zeroes results when the API returns a
				// differently-cased/normalized severity label than the input.
				pt := p.Type
				if pt == "" {
					pt = "os"
				}
				rows = append(rows, patchGapRow{
					OrganizationID:   orgID,
					OrganizationName: orgName,
					DeviceID:         p.DeviceID,
					SystemName:       dev.bestName(),
					KBNumber:         p.KBNumber,
					Name:             p.Name,
					Severity:         p.Severity,
					Status:           p.Status,
					PatchType:        pt,
				})
			}

			sort.SliceStable(rows, func(i, j int) bool {
				if rows[i].OrganizationName != rows[j].OrganizationName {
					return rows[i].OrganizationName < rows[j].OrganizationName
				}
				if rows[i].DeviceID != rows[j].DeviceID {
					return rows[i].DeviceID < rows[j].DeviceID
				}
				return rows[i].KBNumber < rows[j].KBNumber
			})

			if n := boundLimit(len(rows), flagLimit); n < len(rows) {
				rows = rows[:n]
			}

			view := patchGapsView{
				Rows:         rows,
				Count:        len(rows),
				ScannedPages: scanned,
				MaxScanPages: maxPages,
			}
			if len(rows) == 0 && scanned >= maxPages {
				view.Note = "no patch gaps found within max-scan-pages; increase --max-scan-pages to scan deeper"
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No patch gaps found.")
				if view.Note != "" {
					fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				}
				return nil
			}
			headers := []string{"ORG", "DEVICE", "KB", "SEVERITY", "STATUS", "TYPE"}
			tableRows := make([][]string, 0, len(rows))
			for _, r := range rows {
				tableRows = append(tableRows, []string{r.OrganizationName, r.SystemName, r.KBNumber, r.Severity, r.Status, r.PatchType})
			}
			return flags.printTable(cmd, headers, tableRows)
		},
	}
	cmd.Flags().StringVar(&flagSeverity, "severity", "", "Filter to a patch severity (e.g. critical)")
	cmd.Flags().StringVar(&flagOrg, "org", "", "Filter to an organization by name substring or numeric id")
	cmd.Flags().StringVar(&flagType, "type", "os", "Patch type to scan: os|software|all")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Maximum number of gap rows to return (0 = all)")
	cmd.Flags().IntVar(&flagMaxPages, "max-scan-pages", 5, "Maximum API pages to scan per source")
	return cmd
}
