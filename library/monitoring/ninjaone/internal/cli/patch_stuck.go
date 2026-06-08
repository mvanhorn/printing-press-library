// Copyright 2026 "Chris Carson" and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/monitoring/ninjaone/internal/store"

	"github.com/spf13/cobra"
)

// pp:data-source live

type stuckPatch struct {
	DeviceID  int64  `json:"deviceId"`
	KBNumber  string `json:"kbNumber"`
	Cycles    int    `json:"cycles"`
	FirstSeen string `json:"firstSeen"`
	LastSeen  string `json:"lastSeen"`
}

type patchStuckView struct {
	Stuck          []stuckPatch `json:"stuck"`
	Count          int          `json:"count"`
	Cycles         int          `json:"cycles"`
	PatchType      string       `json:"patchType"`
	SnapshotsTaken int          `json:"snapshots_taken"`
	Note           string       `json:"note,omitempty"`
}

func newNovelPatchStuckCmd(flags *rootFlags) *cobra.Command {
	var (
		flagCycles int
		flagType   string
		flagLimit  int
	)

	cmd := &cobra.Command{
		Use:   "patch-stuck",
		Short: "Surface KBs that have been failing or pending across multiple consecutive syncs, not just at this moment.",
		Long: `Record a snapshot of currently failing/pending patches into a local history table,
then report patches (deviceId+KB) that have appeared in at least --cycles distinct
snapshots. With no prior history it records one snapshot and returns an empty list.

Examples:
  ninjaone-pp-cli patch-stuck
  ninjaone-pp-cli patch-stuck --cycles 5 --type software
  ninjaone-pp-cli patch-stuck --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitDryRunPreview(cmd, flags, "would snapshot currently failing patches into local history and report KBs stuck across multiple cycles")
			}

			if flagCycles < 1 {
				flagCycles = 3
			}
			ptype := strings.ToLower(strings.TrimSpace(flagType))
			if ptype == "" {
				ptype = "os"
			}
			path := "/v2/queries/os-patches"
			if ptype == "software" {
				path = "/v2/queries/software-patches"
			} else if ptype != "os" {
				return usageErr(fmt.Errorf("--type must be os or software, got %q", flagType))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			patches, _, err := fetchPatches(ctx, c, path, "FAILED", "", effectiveMaxScanPages(5))
			if err != nil {
				return err
			}

			db, err := store.OpenWithContext(ctx, defaultDBPath("ninjaone-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()

			runID := strconv.FormatInt(time.Now().Unix(), 10)
			now := time.Now().Unix()
			kind := "patch:" + ptype
			snapshots := 0
			for _, p := range patches {
				key := fmt.Sprintf("%d:%s", p.DeviceID, p.KBNumber)
				if err := db.RecordHistory(ctx, kind, key, runID, now, map[string]any{
					"status": p.Status, "severity": p.Severity, "name": p.Name,
				}); err != nil {
					return err
				}
				snapshots++
			}

			entities, err := db.EntitiesWithMinRuns(ctx, kind, flagCycles)
			if err != nil {
				return err
			}

			stuck := make([]stuckPatch, 0, len(entities))
			for _, e := range entities {
				devID, kb := splitEntityKey(e.EntityKey)
				stuck = append(stuck, stuckPatch{
					DeviceID:  devID,
					KBNumber:  kb,
					Cycles:    e.Count,
					FirstSeen: isoFromEpoch(e.FirstSeen),
					LastSeen:  isoFromEpoch(e.LastSeen),
				})
			}
			if n := boundLimit(len(stuck), flagLimit); n < len(stuck) {
				stuck = stuck[:n]
			}

			view := patchStuckView{
				Stuck:          stuck,
				Count:          len(stuck),
				Cycles:         flagCycles,
				PatchType:      ptype,
				SnapshotsTaken: snapshots,
			}
			if len(stuck) == 0 {
				view.Note = fmt.Sprintf("recorded a snapshot of %d failing patch(es); no KB has yet appeared in >= %d distinct syncs — run again over time to build history", snapshots, flagCycles)
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(stuck) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			headers := []string{"DEVICE", "KB", "CYCLES", "FIRST", "LAST"}
			rows := make([][]string, 0, len(stuck))
			for _, s := range stuck {
				rows = append(rows, []string{strconv.FormatInt(s.DeviceID, 10), s.KBNumber, strconv.Itoa(s.Cycles), s.FirstSeen, s.LastSeen})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}
	cmd.Flags().IntVar(&flagCycles, "cycles", 3, "Minimum distinct sync snapshots a KB must appear in")
	cmd.Flags().StringVar(&flagType, "type", "os", "Patch type: os|software")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Maximum number of stuck patches to return (0 = all)")
	return cmd
}

func splitEntityKey(key string) (int64, string) {
	idx := strings.Index(key, ":")
	if idx < 0 {
		return 0, key
	}
	id, _ := strconv.ParseInt(key[:idx], 10, 64)
	return id, key[idx+1:]
}
