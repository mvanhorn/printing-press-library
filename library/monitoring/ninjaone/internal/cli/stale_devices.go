// Copyright 2026 "Chris Carson" and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/monitoring/ninjaone/internal/cliutil"

	"github.com/spf13/cobra"
)

// pp:data-source live

type staleDeviceRow struct {
	OrganizationName string  `json:"organizationName"`
	DeviceID         int64   `json:"deviceId"`
	SystemName       string  `json:"systemName"`
	LastContact      string  `json:"lastContact"`
	DaysSinceContact float64 `json:"daysSinceContact"`
}

type staleDevicesView struct {
	Devices         []staleDeviceRow `json:"devices"`
	Count           int              `json:"count"`
	OfflineDays     int              `json:"offline_days"`
	ScannedDevices  int              `json:"scanned_devices"`
	ScannedPages    int              `json:"scanned_pages"`
	MaxScanPages    int              `json:"max_scan_pages"`
	Rebooted        bool             `json:"rebooted"`
	RebootSucceeded int              `json:"reboot_succeeded,omitempty"`
	RebootFailures  []sweepFailure   `json:"reboot_failures,omitempty"`
	Note            string           `json:"note,omitempty"`
}

func newNovelStaleDevicesCmd(flags *rootFlags) *cobra.Command {
	var (
		flagOfflineDays int
		flagOrg         string
		flagReboot      bool
		flagApply       bool
		flagLimit       int
		flagMaxPages    int
	)

	cmd := &cobra.Command{
		Use:   "stale-devices",
		Short: "List devices with no contact in N days across every organization, grouped by org.",
		Long: `Page the device fleet and report devices that are offline or whose last contact
is older than --offline-days, grouped by organization. With --reboot --apply,
issue a NORMAL reboot to each stale device (skipped without --apply).

Examples:
  ninjaone-pp-cli stale-devices
  ninjaone-pp-cli stale-devices --offline-days 14 --org Acme
  ninjaone-pp-cli stale-devices --reboot --apply`,
		// No mcp:read-only annotation: --reboot --apply POSTs device reboots,
		// so this command can mutate external state (matches patch-sweep/alert-clear).
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) || (flagReboot && cliutil.IsVerifyEnv()) {
				return emitDryRunPreview(cmd, flags, "would page the device fleet and report (and optionally reboot) devices stale beyond --offline-days")
			}

			if flagOfflineDays < 1 {
				flagOfflineDays = 30
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
			devices, pages, err := fetchDevices(ctx, c, "", maxPages)
			if err != nil {
				return err
			}

			now := time.Now()
			cutoffSecs := float64(flagOfflineDays) * 86400.0
			rows := make([]staleDeviceRow, 0)
			staleByID := make(map[int64]njDevice)
			for _, d := range devices {
				orgName := orgs[d.OrganizationID]
				if !orgMatches(flagOrg, d.OrganizationID, orgName) {
					continue
				}
				lc := d.lastContactSeconds()
				ageSecs := float64(now.Unix()) - lc
				isStale := d.Offline || (lc > 0 && ageSecs > cutoffSecs)
				if !isStale {
					continue
				}
				days := 0.0
				lcStr := ""
				if lc > 0 {
					days = ageSecs / 86400.0
					lcStr = time.Unix(int64(lc), 0).UTC().Format(time.RFC3339)
				}
				rows = append(rows, staleDeviceRow{
					OrganizationName: orgName,
					DeviceID:         d.ID,
					SystemName:       d.bestName(),
					LastContact:      lcStr,
					DaysSinceContact: float64(int(days*10)) / 10,
				})
				staleByID[d.ID] = d
			}

			sort.SliceStable(rows, func(i, j int) bool {
				if rows[i].OrganizationName != rows[j].OrganizationName {
					return rows[i].OrganizationName < rows[j].OrganizationName
				}
				return rows[i].DeviceID < rows[j].DeviceID
			})
			if n := boundLimit(len(rows), flagLimit); n < len(rows) {
				rows = rows[:n]
			}
			// Derive the reboot cohort from the sorted+truncated rows so the
			// devices acted on are exactly the devices displayed (alignment bug
			// fix: rows were sorted but a parallel slice was not).
			staleDevs := make([]njDevice, 0, len(rows))
			for _, r := range rows {
				staleDevs = append(staleDevs, staleByID[r.DeviceID])
			}

			view := staleDevicesView{
				Devices:        rows,
				Count:          len(rows),
				OfflineDays:    flagOfflineDays,
				ScannedDevices: len(devices),
				ScannedPages:   pages,
				MaxScanPages:   maxPages,
			}
			if len(rows) == 0 && pages >= maxPages {
				view.Note = "no stale devices within max-scan-pages; increase --max-scan-pages to scan deeper"
			}

			// Optional mutation: reboot stale devices.
			if flagReboot {
				if !flagApply {
					if view.Note == "" {
						view.Note = "reboot preview only; pass --apply to reboot the listed devices"
					}
				} else {
					view.RebootFailures = make([]sweepFailure, 0)
					for _, d := range staleDevs {
						path := fmt.Sprintf("/v2/device/%d/reboot/NORMAL", d.ID)
						if _, status, err := c.Post(ctx, path, map[string]any{}); err != nil || status >= 400 {
							view.RebootFailures = append(view.RebootFailures, sweepFailure{DeviceID: d.ID, Action: "reboot", Error: postErrString(status, err)})
						} else {
							view.RebootSucceeded++
						}
					}
					view.Rebooted = true
				}
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No stale devices found.")
				if view.Note != "" {
					fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				}
				return nil
			}
			headers := []string{"ORG", "DEVICE", "LAST CONTACT", "DAYS"}
			tableRows := make([][]string, 0, len(rows))
			for _, r := range rows {
				tableRows = append(tableRows, []string{r.OrganizationName, r.SystemName, r.LastContact, strconv.FormatFloat(r.DaysSinceContact, 'f', 1, 64)})
			}
			if err := flags.printTable(cmd, headers, tableRows); err != nil {
				return err
			}
			if view.Rebooted {
				fmt.Fprintf(cmd.OutOrStdout(), "Rebooted: %d  failures: %d\n", view.RebootSucceeded, len(view.RebootFailures))
			} else if view.Note != "" {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&flagOfflineDays, "offline-days", 30, "Treat a device as stale after this many days without contact")
	cmd.Flags().StringVar(&flagOrg, "org", "", "Filter to an organization by name substring or numeric id")
	cmd.Flags().BoolVar(&flagReboot, "reboot", false, "Reboot stale devices (requires --apply)")
	cmd.Flags().BoolVar(&flagApply, "apply", false, "Actually reboot (required with --reboot to mutate)")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Maximum number of devices to return/act on (0 = all)")
	cmd.Flags().IntVar(&flagMaxPages, "max-scan-pages", 5, "Maximum API pages to scan")
	return cmd
}
