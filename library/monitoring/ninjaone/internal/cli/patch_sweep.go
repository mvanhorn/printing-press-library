// Copyright 2026 "Chris Carson" and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/monitoring/ninjaone/internal/cliutil"

	"github.com/spf13/cobra"
)

// pp:data-source live

type sweepFailure struct {
	DeviceID int64  `json:"deviceId"`
	Action   string `json:"action"`
	Error    string `json:"error"`
}

type patchSweepView struct {
	Df             string         `json:"df"`
	PatchType      string         `json:"patchType"`
	Applied        bool           `json:"applied"`
	CohortCount    int            `json:"cohortCount"`
	DeviceIDs      []int64        `json:"deviceIds"`
	ScanRequested  bool           `json:"scanRequested"`
	ApplyRequested bool           `json:"applyRequested"`
	Succeeded      int            `json:"succeeded"`
	FetchFailures  []string       `json:"fetch_failures"`
	Failures       []sweepFailure `json:"failures"`
	ScannedPages   int            `json:"scanned_pages"`
	MaxScanPages   int            `json:"max_scan_pages"`
	Note           string         `json:"note,omitempty"`
}

func newNovelPatchSweepCmd(flags *rootFlags) *cobra.Command {
	var (
		flagDf       string
		flagType     string
		flagScan     bool
		flagApply    bool
		flagLimit    int
		flagMaxPages int
	)

	cmd := &cobra.Command{
		Use:   "patch-sweep",
		Short: "Scan and apply patches across a device cohort resolved from a device filter.",
		Long: `Resolve a device cohort from a device filter (--df) and, by default, preview it.
With --scan, POST a patch scan to each device; with --apply, POST a patch apply.
Mutations require --apply and are skipped in verify/dry-run mode.

Examples:
  ninjaone-pp-cli patch-sweep --df "status eq APPROVED"            # preview cohort only
  ninjaone-pp-cli patch-sweep --df "org = 5" --scan --apply        # scan then apply
  ninjaone-pp-cli patch-sweep --df "org = 5" --type both --apply`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return emitDryRunPreview(cmd, flags, "would resolve a device cohort from --df and scan/apply patches when --apply is set")
			}

			if strings.TrimSpace(flagDf) == "" {
				return usageErr(fmt.Errorf("--df is required (device filter selecting the cohort)"))
			}
			ptype := strings.ToLower(strings.TrimSpace(flagType))
			if ptype == "" {
				ptype = "os"
			}
			switch ptype {
			case "os", "software", "both":
			default:
				return usageErr(fmt.Errorf("--type must be one of os|software|both, got %q", flagType))
			}

			maxPages := effectiveMaxScanPages(flagMaxPages)
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			devices, scanned, err := fetchDevices(ctx, c, flagDf, maxPages)
			if err != nil {
				return err
			}
			if n := boundLimit(len(devices), flagLimit); n < len(devices) {
				devices = devices[:n]
			}

			ids := make([]int64, 0, len(devices))
			for _, d := range devices {
				ids = append(ids, d.ID)
			}

			view := patchSweepView{
				Df:             flagDf,
				PatchType:      ptype,
				CohortCount:    len(devices),
				DeviceIDs:      ids,
				ScanRequested:  flagScan,
				ApplyRequested: flagApply,
				FetchFailures:  make([]string, 0),
				Failures:       make([]sweepFailure, 0),
				ScannedPages:   scanned,
				MaxScanPages:   maxPages,
			}

			// Default = dry preview: print cohort, do not mutate.
			if !flagScan && !flagApply {
				view.Note = "preview only; pass --scan and/or --apply to mutate"
				return emitSweepView(cmd, flags, view)
			}

			types := []string{}
			if ptype == "os" || ptype == "both" {
				types = append(types, "os")
			}
			if ptype == "software" || ptype == "both" {
				types = append(types, "software")
			}

			for _, d := range devices {
				for _, t := range types {
					if flagScan {
						path := fmt.Sprintf("/v2/device/%d/patch/%s/scan", d.ID, t)
						if _, status, err := c.Post(ctx, path, map[string]any{}); err != nil || status >= 400 {
							view.Failures = append(view.Failures, sweepFailure{DeviceID: d.ID, Action: t + "/scan", Error: postErrString(status, err)})
						} else {
							view.Succeeded++
						}
					}
					if flagApply {
						path := fmt.Sprintf("/v2/device/%d/patch/%s/apply", d.ID, t)
						if _, status, err := c.Post(ctx, path, map[string]any{}); err != nil || status >= 400 {
							view.Failures = append(view.Failures, sweepFailure{DeviceID: d.ID, Action: t + "/apply", Error: postErrString(status, err)})
						} else {
							view.Succeeded++
						}
					}
				}
			}
			view.Applied = flagApply

			return emitSweepView(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&flagDf, "df", "", "Device filter selecting the cohort (REQUIRED)")
	cmd.Flags().StringVar(&flagType, "type", "os", "Patch type: os|software|both")
	cmd.Flags().BoolVar(&flagScan, "scan", false, "POST a patch scan to each device in the cohort")
	cmd.Flags().BoolVar(&flagApply, "apply", false, "POST a patch apply to each device (required to mutate)")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Maximum number of devices to act on (0 = all)")
	cmd.Flags().IntVar(&flagMaxPages, "max-scan-pages", 5, "Maximum API pages to scan resolving the cohort")
	return cmd
}

func postErrString(status int, err error) string {
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("HTTP %d", status)
}

func emitSweepView(cmd *cobra.Command, flags *rootFlags, view patchSweepView) error {
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		return printJSONFiltered(cmd.OutOrStdout(), view, flags)
	}
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Cohort: %d device(s) matching df=%q (type=%s)\n", view.CohortCount, view.Df, view.PatchType)
	if view.Note != "" {
		fmt.Fprintln(w, view.Note)
		return nil
	}
	fmt.Fprintf(w, "Succeeded: %d  Failures: %d\n", view.Succeeded, len(view.Failures))
	for _, f := range view.Failures {
		fmt.Fprintf(w, "  FAIL device %d %s: %s\n", f.DeviceID, f.Action, f.Error)
	}
	return nil
}
