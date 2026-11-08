// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command. pp:data-source local.

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/store"
	"github.com/spf13/cobra"
)

type digestEvent struct {
	Time      string `json:"time"`
	ConsoleID string `json:"console_id,omitempty"`
	Category  string `json:"category"`
	Summary   string `json:"summary"`
	EntityID  string `json:"entity_id,omitempty"`
	// Observed says how exact Time is. "at-sync" means the change happened somewhere
	// between two syncs and Time is the later one; "reported" is the API's own stamp.
	Observed string `json:"observed"`
}

type digestView struct {
	Since string `json:"since"`
	// Window echoes what the user asked for; WindowSeconds is the machine-readable form.
	Window        string         `json:"window"`
	WindowSeconds int64          `json:"window_seconds"`
	Note          string         `json:"note,omitempty"`
	Events        []digestEvent  `json:"events"`
	Counts        map[string]int `json:"counts"`
	Snapshots     int            `json:"snapshots_in_window"`
	Truncated     bool           `json:"truncated,omitempty"`
	Notes         []string       `json:"notes"`
}

var digestNotes = []string{
	"State changes are seen at sync points, so a flap that recovers between two syncs is missed. Site Manager's lastConnectionStateChange partly offsets this for hosts.",
	"ISP dips are listed beside other events by time only; being close in time does not make one the cause of another.",
}

// digestStateChanges compares state fields across a pair of snapshots. Only stateful
// types appear here; config differences come from computeDrift.
func digestStateChanges(console, at string, from, to map[string]store.SnapshotRow) []digestEvent {
	out := make([]digestEvent, 0)
	for key, after := range to {
		var label, field, category string
		switch after.ResourceType {
		case rtDevices:
			label, field, category = "device", "state", "device-state"
		case rtCameras:
			label, field, category = "camera", "state", "camera-state"
		default:
			continue
		}
		before, existed := from[key]
		if !existed {
			continue
		}
		b, a := decodeObject(before.Data), decodeObject(after.Data)
		bs, _ := b[field].(string)
		as, _ := a[field].(string)
		if bs == as {
			continue
		}
		name, _ := a["name"].(string)
		out = append(out, digestEvent{Time: at, ConsoleID: console, Category: category, Summary: fmt.Sprintf("%s %s went from %s to %s", label, firstNonEmpty(name, after.EntityID), bs, as), EntityID: after.EntityID, Observed: "at-sync"})
	}
	return out
}

// ispDips reads Site Manager samples. It reports each bad sample as a fact and makes
// no claim about what caused it or what it caused.
func ispDips(rows []mirrorRow, consoleFilter string, cutoff time.Time, latencyMs, lossPct float64) []digestEvent {
	out := make([]digestEvent, 0)
	for _, row := range rows {
		host := row.str("hostId")
		if consoleFilter != "" && host != consoleFilter {
			continue
		}
		at, err := time.Parse(time.RFC3339, row.str("metricTime"))
		if err != nil || at.Before(cutoff) {
			continue
		}
		wan, _ := row.Data["wan"].(map[string]any)
		if wan == nil {
			continue
		}
		latency, _ := wan["avgLatency"].(float64)
		loss, _ := wan["packetLoss"].(float64)
		downtime, _ := wan["downtime"].(float64)
		parts := make([]string, 0, 3)
		if latency >= latencyMs {
			parts = append(parts, fmt.Sprintf("latency %.0f ms", latency))
		}
		if loss >= lossPct {
			parts = append(parts, fmt.Sprintf("packet loss %.1f%%", loss))
		}
		if downtime > 0 {
			parts = append(parts, fmt.Sprintf("downtime %.0f s", downtime))
		}
		if len(parts) == 0 {
			continue
		}
		out = append(out, digestEvent{Time: at.UTC().Format(time.RFC3339), ConsoleID: host, Category: "isp", Summary: "ISP sample: " + strings.Join(parts, ", "), Observed: "reported"})
	}
	return out
}

func loadDigest(ctx context.Context, db *store.Store, console string, cutoff time.Time, latencyMs, lossPct float64) (digestView, error) {
	view := digestView{Since: cutoff.UTC().Format(time.RFC3339), Events: make([]digestEvent, 0), Counts: map[string]int{}, Notes: digestNotes}
	snaps, err := db.ListSnapshots(ctx, console)
	if err != nil {
		return view, err
	}
	view.Note = syncCoverageNote(len(snaps))
	byConsole := map[string][]store.Snapshot{}
	for _, snap := range snaps {
		byConsole[snap.ConsoleID] = append(byConsole[snap.ConsoleID], snap)
	}
	journal, err := db.ListJournalEntries(ctx, console, time.Time{})
	if err != nil {
		return view, err
	}
	for consoleID, list := range byConsole {
		for i := 1; i < len(list); i++ {
			taken, err := time.Parse(time.RFC3339, list[i].TakenAt)
			if err != nil || taken.Before(cutoff) {
				continue
			}
			view.Snapshots++
			from, err := db.SnapshotRows(ctx, list[i-1].ID)
			if err != nil {
				return view, err
			}
			to, err := db.SnapshotRows(ctx, list[i].ID)
			if err != nil {
				return view, err
			}
			view.Events = append(view.Events, digestStateChanges(consoleID, list[i].TakenAt, from, to)...)
			for _, change := range computeDrift(consoleID, from, to, journal, list[i-1].TakenAt, list[i].TakenAt, driftResourceTypes) {
				view.Events = append(view.Events, digestEvent{Time: list[i].TakenAt, ConsoleID: consoleID, Category: "drift", Summary: fmt.Sprintf("%s %s %s (%s, %d fields)", change.Category, firstNonEmpty(change.Name, change.EntityID), change.Change, change.Attribution, len(change.Fields)), EntityID: change.EntityID, Observed: "at-sync"})
			}
		}
	}
	newcomers, err := db.FirstSeenSince(ctx, console, cutoff)
	if err != nil {
		return view, err
	}
	for _, row := range newcomers {
		n := newcomerRowFrom(row)
		view.Events = append(view.Events, digestEvent{Time: row.FirstSeenAt, ConsoleID: row.ConsoleID, Category: "newcomer", Summary: fmt.Sprintf("new %s %s", n.Kind, firstNonEmpty(n.Name, n.MAC, n.EntityID)), EntityID: n.EntityID, Observed: "at-sync"})
	}
	for _, entry := range journal {
		created, err := time.Parse(time.RFC3339Nano, entry.CreatedAt)
		if err != nil || created.Before(cutoff) {
			continue
		}
		view.Events = append(view.Events, digestEvent{Time: created.UTC().Format(time.RFC3339), ConsoleID: entry.ConsoleID, Category: "change", Summary: fmt.Sprintf("%s %s %s: %s", entry.Kind, entry.Method, entry.Path, entry.State), EntityID: entry.EntityID, Observed: "reported"})
	}
	hosts, err := loadMirrorRows(ctx, db, "", rtHosts)
	if err != nil {
		return view, err
	}
	for _, host := range hosts {
		if console != "" && host.ID != console {
			continue
		}
		at, err := time.Parse(time.RFC3339, host.str("lastConnectionStateChange"))
		if err != nil || at.Before(cutoff) {
			continue
		}
		view.Events = append(view.Events, digestEvent{Time: at.UTC().Format(time.RFC3339), ConsoleID: host.ID, Category: "host-state", Summary: fmt.Sprintf("host %s connection state last changed (now %s)", firstNonEmpty(nestedString(host.Data, "reportedState", "name"), host.ID), firstNonEmpty(nestedString(host.Data, "reportedState", "state"), "unknown")), EntityID: host.ID, Observed: "reported"})
	}
	samples, err := loadMirrorRows(ctx, db, "", rtISPSamples)
	if err != nil {
		return view, err
	}
	view.Events = append(view.Events, ispDips(samples, console, cutoff, latencyMs, lossPct)...)
	sort.SliceStable(view.Events, func(i, j int) bool {
		if view.Events[i].Time != view.Events[j].Time {
			return view.Events[i].Time < view.Events[j].Time
		}
		if view.Events[i].ConsoleID != view.Events[j].ConsoleID {
			return view.Events[i].ConsoleID < view.Events[j].ConsoleID
		}
		return view.Events[i].Summary < view.Events[j].Summary
	})
	for _, event := range view.Events {
		view.Counts[event.Category]++
	}
	return view, nil
}

func newNovelDigestCmd(flags *rootFlags) *cobra.Command {
	var dbFlag, sinceRaw, category string
	var limit int
	var latencyMs, lossPct float64
	cmd := &cobra.Command{
		Use:   "digest",
		Short: "One timeline, built from local syncs, of state changes, ISP dips, drift, newcomers and journaled changes.",
		Long: `Use this command for a combined timeline across all categories. Do NOT use it for
field-level config differences; use 'drift' instead. Do NOT use it for the full list of
new clients; use 'newcomer' instead.

Built from local snapshots, ISP samples and the change journal. Transitions are seen only
at sync points, so a flap between syncs is missed, and events side by side in time are not
claimed to be related.`,
		Example: strings.Trim(`
  unifi-pp-cli digest --since 7d --agent
  unifi-pp-cli digest --since 24h --category drift --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "digest")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			window, err := parseLooseDuration(sinceRaw)
			if err != nil {
				return usageErr(fmt.Errorf("--since %q: %w", sinceRaw, err))
			}
			cutoff := time.Now().Add(-window)
			dbPath := mirrorDBPath(dbFlag)
			if mirrorMissing(dbPath) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: unifi-pp-cli sync --console <id> --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), digestView{Since: cutoff.UTC().Format(time.RFC3339), Window: sinceRaw, WindowSeconds: int64(window.Seconds()), Note: noSyncNote, Events: make([]digestEvent, 0), Counts: map[string]int{}, Notes: digestNotes}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), noSyncNote)
				return nil
			}
			db, err := openMirror(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			view, err := loadDigest(ctx, db, store.ActiveConsole(), cutoff, latencyMs, lossPct)
			if err != nil {
				return err
			}
			view.Window, view.WindowSeconds = sinceRaw, int64(window.Seconds())
			if category != "" {
				filtered := make([]digestEvent, 0)
				for _, event := range view.Events {
					if event.Category == category {
						filtered = append(filtered, event)
					}
				}
				view.Events = filtered
			}
			if limit > 0 && len(view.Events) > limit {
				// Keep the newest: this answers "what happened since I looked".
				view.Events = view.Events[len(view.Events)-limit:]
				view.Truncated = true
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(view.Events) == 0 {
				if view.Note != "" {
					fmt.Fprintln(cmd.OutOrStdout(), view.Note)
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Nothing recorded in the last %s.\n", sinceRaw)
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "TIME\tCONSOLE\tCATEGORY\tSUMMARY")
			for _, event := range view.Events {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", event.Time, event.ConsoleID, event.Category, event.Summary)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&dbFlag, "db", "", "Local mirror path (default: the standard mirror)")
	cmd.Flags().StringVar(&sinceRaw, "since", "24h", "Window to look back over, e.g. 24h, 7d")
	cmd.Flags().StringVar(&category, "category", "", "Only this category: device-state, camera-state, host-state, drift, newcomer, change, isp")
	cmd.Flags().IntVar(&limit, "limit", 200, "Newest events to return; 0 for all")
	cmd.Flags().Float64Var(&latencyMs, "isp-latency-ms", 100, "Report ISP samples with average latency at or above this")
	cmd.Flags().Float64Var(&lossPct, "isp-loss-pct", 1, "Report ISP samples with packet loss at or above this")
	return cmd
}
