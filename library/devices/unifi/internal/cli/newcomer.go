// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command. pp:data-source local.

package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/store"
	"github.com/spf13/cobra"
)

func parseLooseDuration(raw string) (time.Duration, error) { return cliutil.ParseDurationLoose(raw) }

// noSyncNote and baselineOnlyNote exist because an empty answer from a store that has
// never synced reads as an all clear, which is the one thing it cannot say.
const (
	noSyncNote       = "no sync has run yet, so this is not an all clear; run: unifi-pp-cli sync"
	baselineOnlyNote = "only one sync so far, so nothing can be compared yet; run unifi-pp-cli sync again later"
)

func syncCoverageNote(snapshots int) string {
	switch snapshots {
	case 0:
		return noSyncNote
	case 1:
		return baselineOnlyNote
	}
	return ""
}

type newcomerRow struct {
	ConsoleID   string `json:"console_id"`
	Kind        string `json:"kind"`
	EntityID    string `json:"entity_id"`
	SiteID      string `json:"site_id,omitempty"`
	Name        string `json:"name,omitempty"`
	MAC         string `json:"mac,omitempty"`
	IP          string `json:"ip,omitempty"`
	FirstSeenAt string `json:"first_seen_at"`
	LastSeenAt  string `json:"last_seen_at"`
	// RandomizedMAC flags the locally-administered bit: phones rotate these, so one
	// device can show up as several newcomers.
	RandomizedMAC bool `json:"randomized_mac,omitempty"`
}

type newcomerView struct {
	// Window echoes what the user asked for; WindowSeconds is the machine-readable form.
	Window        string        `json:"window"`
	WindowSeconds int64         `json:"window_seconds"`
	Note          string        `json:"note,omitempty"`
	Newcomers     []newcomerRow `json:"newcomers"`
	Truncated     bool          `json:"truncated,omitempty"`
	SnapshotCount int           `json:"snapshot_count"`
	Notes         []string      `json:"notes"`
}

var newcomerNotes = []string{
	"First-seen is local: only clients present at a sync are seen, so a brief visit between syncs is missed.",
	"Entities in a console's first snapshot are the baseline and are never reported as new.",
	"MAC randomization can inflate results; randomized_mac marks addresses likely to rotate.",
}

func macIsRandomized(mac string) bool {
	if len(mac) < 2 {
		return false
	}
	first, err := strconv.ParseUint(mac[:2], 16, 8)
	if err != nil {
		return false
	}
	return first&0x02 != 0
}

func newcomerKind(resourceType string) string {
	switch resourceType {
	case rtClients:
		return "client"
	case rtDevices:
		return "device"
	case rtCameras:
		return "camera"
	}
	return resourceType
}

func newcomerRowFrom(row store.FirstSeenRow) newcomerRow {
	entity, site := row.EntityID, ""
	if i := strings.LastIndexByte(entity, '/'); i >= 0 {
		site, entity = entity[:i], entity[i+1:]
	}
	return newcomerRow{ConsoleID: row.ConsoleID, Kind: newcomerKind(row.ResourceType), EntityID: entity, SiteID: site, Name: row.Label, MAC: row.MAC, IP: row.IP, FirstSeenAt: row.FirstSeenAt, LastSeenAt: row.LastSeenAt, RandomizedMAC: macIsRandomized(row.MAC)}
}

func loadNewcomers(ctx context.Context, db *store.Store, console string, window time.Duration, kind string, now time.Time) (newcomerView, error) {
	view := newcomerView{WindowSeconds: int64(window.Seconds()), Newcomers: make([]newcomerRow, 0), Notes: newcomerNotes}
	rows, err := db.FirstSeenSince(ctx, console, now.Add(-window))
	if err != nil {
		return view, err
	}
	for _, row := range rows {
		out := newcomerRowFrom(row)
		if kind != "" && kind != out.Kind {
			continue
		}
		view.Newcomers = append(view.Newcomers, out)
	}
	snaps, err := db.ListSnapshots(ctx, console)
	if err != nil {
		return view, err
	}
	view.SnapshotCount = len(snaps)
	view.Note = syncCoverageNote(len(snaps))
	if len(snaps) == 0 {
		view.Notes = append([]string{"No snapshots yet; each successful sync takes one."}, newcomerNotes...)
	} else if len(snaps) == 1 {
		view.Notes = append([]string{"Only the baseline snapshot exists, so nothing can be new yet. Sync again later."}, newcomerNotes...)
	}
	return view, nil
}

func newNovelNewcomerCmd(flags *rootFlags) *cobra.Command {
	var dbFlag, sinceRaw, kind string
	var limit int
	cmd := &cobra.Command{
		Use:   "newcomer",
		Short: "List clients, devices and cameras first seen at a sync in a window.",
		Long: `Use this command to list clients and devices first seen in a window. Do NOT use it for
config changes or a general timeline; use 'digest' instead.

The API has no first-seen field, so this is computed from local history. Only what
was present at a sync is seen, and randomized MACs can inflate the count.`,
		Example: strings.Trim(`
  unifi-pp-cli newcomer --since 24h --agent
  unifi-pp-cli newcomer --since 7d --kind client --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "newcomer")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			window, err := parseLooseDuration(sinceRaw)
			if err != nil {
				return usageErr(fmt.Errorf("--since %q: %w", sinceRaw, err))
			}
			if kind != "" && kind != "client" && kind != "device" && kind != "camera" {
				return usageErr(fmt.Errorf("--kind %q must be client, device or camera", kind))
			}
			dbPath := mirrorDBPath(dbFlag)
			if mirrorMissing(dbPath) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: unifi-pp-cli sync --console <id> --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), newcomerView{Window: sinceRaw, WindowSeconds: int64(window.Seconds()), Note: noSyncNote, Newcomers: make([]newcomerRow, 0), Notes: newcomerNotes}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), noSyncNote)
				return nil
			}
			db, err := openMirror(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			view, err := loadNewcomers(ctx, db, store.ActiveConsole(), window, kind, time.Now())
			if err != nil {
				return err
			}
			view.Window = sinceRaw
			if limit > 0 && len(view.Newcomers) > limit {
				// Keep the newest, as digest does: dropping them hides the arrivals an
				// operator is most likely checking for, and the flag says rows are gone.
				view.Newcomers = view.Newcomers[len(view.Newcomers)-limit:]
				view.Truncated = true
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(view.Newcomers) == 0 {
				if view.Note != "" {
					fmt.Fprintln(cmd.OutOrStdout(), view.Note)
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "No newcomers in the last %s.\n", sinceRaw)
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "FIRST SEEN\tKIND\tNAME\tMAC\tIP\tCONSOLE")
			for _, n := range view.Newcomers {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", n.FirstSeenAt, n.Kind, firstNonEmpty(n.Name, n.EntityID), n.MAC, n.IP, n.ConsoleID)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&dbFlag, "db", "", "Local mirror path (default: the standard mirror)")
	cmd.Flags().StringVar(&sinceRaw, "since", "24h", "Window to look back over, e.g. 24h, 7d")
	cmd.Flags().StringVar(&kind, "kind", "", "Only this kind: client, device or camera")
	cmd.Flags().IntVar(&limit, "limit", 200, "Maximum newcomers to return; 0 for all")
	return cmd
}
