// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command. pp:data-source local.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/client"
	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/store"
	"github.com/spf13/cobra"
)

type driftField struct {
	Path   string `json:"path"`
	Before any    `json:"before"`
	After  any    `json:"after"`
	// SecretChanged says a secret differs between snapshots. Before and After then hold
	// only placeholders: the stored digests stay inside the store.
	SecretChanged bool `json:"secret_changed,omitempty"`
}

type driftChange struct {
	ConsoleID string `json:"console_id"`
	Category  string `json:"category"`
	EntityID  string `json:"entity_id"`
	Name      string `json:"name,omitempty"`
	Change    string `json:"change"`
	// Attribution is "attributed" when the journal holds a change to this entity in
	// the window, otherwise "out-of-band". Absence means someone or something else
	// changed it; it does not say who.
	Attribution string       `json:"attribution"`
	JournalIDs  []string     `json:"journal_ids,omitempty"`
	Fields      []driftField `json:"fields,omitempty"`
}

type driftWindow struct {
	ConsoleID string `json:"console_id"`
	From      int64  `json:"from_snapshot"`
	FromTime  string `json:"from_time"`
	To        int64  `json:"to_snapshot"`
	ToTime    string `json:"to_time"`
}

type driftView struct {
	Windows []driftWindow `json:"windows"`
	Changes []driftChange `json:"changes"`
	Note    string        `json:"note,omitempty"`
}

// diffFields returns field-level differences, sorted for stable output. Secret
// values never appear: drift output lands in terminals and agent transcripts.
func diffFields(prefix string, before, after any) []driftField {
	if reflect.DeepEqual(before, after) {
		return nil
	}
	beforeMap, beforeIsMap := before.(map[string]any)
	afterMap, afterIsMap := after.(map[string]any)
	if beforeIsMap && afterIsMap {
		keys := map[string]bool{}
		for k := range beforeMap {
			keys[k] = true
		}
		for k := range afterMap {
			keys[k] = true
		}
		names := make([]string, 0, len(keys))
		for k := range keys {
			names = append(names, k)
		}
		sort.Strings(names)
		out := make([]driftField, 0)
		for _, name := range names {
			child := name
			if prefix != "" {
				child = prefix + "." + name
			}
			// Decided here, not only at the leaf: walking into {"credentials":{"value":x}}
			// reaches a leaf named "value", which looks harmless and prints x.
			if client.IsSecretField(name) {
				if !reflect.DeepEqual(beforeMap[name], afterMap[name]) {
					out = append(out, driftField{Path: child, Before: "[scrubbed]", After: "[scrubbed]", SecretChanged: true})
				}
				continue
			}
			out = append(out, diffFields(child, beforeMap[name], afterMap[name])...)
		}
		return out
	}
	if client.IsSecretField(leafName(prefix)) {
		return []driftField{{Path: prefix, Before: "[scrubbed]", After: "[scrubbed]", SecretChanged: true}}
	}
	// An array of objects or a voucher code reaches here under a harmless name. Scrubbed
	// forms that match, on values that differ, mean only the secret inside changed.
	scrubbedBefore, scrubbedAfter := client.ScrubSecrets(before), client.ScrubSecrets(after)
	return []driftField{{Path: prefix, Before: scrubbedBefore, After: scrubbedAfter, SecretChanged: reflect.DeepEqual(scrubbedBefore, scrubbedAfter)}}
}

func leafName(path string) string {
	if i := strings.LastIndexByte(path, '.'); i >= 0 {
		return path[i+1:]
	}
	return path
}

func decodeObject(data json.RawMessage) map[string]any {
	var obj map[string]any
	if json.Unmarshal(data, &obj) != nil {
		return nil
	}
	// The sync-injected parent key and any stamps are bookkeeping, not config.
	delete(obj, "parent_id")
	delete(obj, "network_integration_v1_sites_id")
	delete(obj, store.ConsoleIDField)
	return obj
}

// computeDrift is pure. Given identical snapshots it returns an empty slice, never
// a fabricated change.
func computeDrift(console string, from, to map[string]store.SnapshotRow, journal []store.JournalEntry, fromTime, toTime string, categories map[string]string) []driftChange {
	byEntity := map[string][]string{}
	for _, e := range journal {
		if e.ConsoleID != console || e.EntityID == "" {
			continue
		}
		// Parsed, not compared as text: journal stamps carry nanoseconds and snapshot
		// stamps do not, and "12:00:00.5Z" sorts before "12:00:00Z" as a string.
		created, errCreated := time.Parse(time.RFC3339Nano, e.CreatedAt)
		start, errStart := time.Parse(time.RFC3339Nano, fromTime)
		end, errEnd := time.Parse(time.RFC3339Nano, toTime)
		if e.State == journalFailed || errCreated != nil || errStart != nil || errEnd != nil || !created.After(start) || created.After(end) {
			continue
		}
		byEntity[e.EntityID] = append(byEntity[e.EntityID], e.ID)
	}
	keys := map[string]bool{}
	for k, row := range from {
		if _, ok := categories[row.ResourceType]; ok {
			keys[k] = true
		}
	}
	for k, row := range to {
		if _, ok := categories[row.ResourceType]; ok {
			keys[k] = true
		}
	}
	ordered := make([]string, 0, len(keys))
	for k := range keys {
		ordered = append(ordered, k)
	}
	sort.Strings(ordered)
	changes := make([]driftChange, 0)
	for _, key := range ordered {
		before, hadBefore := from[key]
		after, hasAfter := to[key]
		row := after
		if !hasAfter {
			row = before
		}
		bare := row.EntityID
		if i := strings.LastIndexByte(bare, '/'); i >= 0 {
			bare = bare[i+1:]
		}
		change := driftChange{ConsoleID: console, Category: categories[row.ResourceType], EntityID: bare}
		switch {
		case !hadBefore:
			change.Change = "added"
			obj := decodeObject(after.Data)
			change.Name, _ = obj["name"].(string)
		case !hasAfter:
			change.Change = "removed"
			obj := decodeObject(before.Data)
			change.Name, _ = obj["name"].(string)
		default:
			b, a := decodeObject(before.Data), decodeObject(after.Data)
			fields := diffFields("", any(b), any(a))
			if len(fields) == 0 {
				continue
			}
			change.Change = "changed"
			change.Fields = fields
			change.Name, _ = a["name"].(string)
		}
		if ids := byEntity[bare]; len(ids) > 0 {
			change.Attribution = "attributed"
			change.JournalIDs = ids
		} else {
			change.Attribution = "out-of-band"
		}
		changes = append(changes, change)
	}
	return changes
}

// pickDriftSnapshots: default is the last two. --since moves "from" back to the
// newest snapshot taken at or before the cutoff, or the oldest if none is that old.
func pickDriftSnapshots(snaps []store.Snapshot, cutoff time.Time) (from, to store.Snapshot, ok bool) {
	if len(snaps) < 2 {
		return from, to, false
	}
	to = snaps[len(snaps)-1]
	from = snaps[len(snaps)-2]
	if !cutoff.IsZero() {
		from = snaps[0]
		for _, snap := range snaps[:len(snaps)-1] {
			taken, err := time.Parse(time.RFC3339, snap.TakenAt)
			if err == nil && !taken.After(cutoff) {
				from = snap
			}
		}
	}
	return from, to, true
}

func loadDrift(ctx context.Context, db *store.Store, consoleFilter string, cutoff time.Time) (driftView, error) {
	view := driftView{Windows: []driftWindow{}, Changes: []driftChange{}}
	snaps, err := db.ListSnapshots(ctx, consoleFilter)
	if err != nil {
		return view, err
	}
	byConsole := map[string][]store.Snapshot{}
	for _, snap := range snaps {
		byConsole[snap.ConsoleID] = append(byConsole[snap.ConsoleID], snap)
	}
	consoles := make([]string, 0, len(byConsole))
	for c := range byConsole {
		consoles = append(consoles, c)
	}
	sort.Strings(consoles)
	journal, err := db.ListJournalEntries(ctx, consoleFilter, time.Time{})
	if err != nil {
		return view, err
	}
	single := 0
	for _, c := range consoles {
		from, to, ok := pickDriftSnapshots(byConsole[c], cutoff)
		if !ok {
			single++
			continue
		}
		fromRows, err := db.SnapshotRows(ctx, from.ID)
		if err != nil {
			return view, err
		}
		toRows, err := db.SnapshotRows(ctx, to.ID)
		if err != nil {
			return view, err
		}
		view.Windows = append(view.Windows, driftWindow{ConsoleID: c, From: from.ID, FromTime: from.TakenAt, To: to.ID, ToTime: to.TakenAt})
		view.Changes = append(view.Changes, computeDrift(c, fromRows, toRows, journal, from.TakenAt, to.TakenAt, driftResourceTypes)...)
	}
	switch {
	case len(consoles) == 0:
		view.Note = "no snapshots yet; each successful sync takes one"
	case single > 0 && len(view.Windows) == 0:
		view.Note = "only one snapshot so far, so there is nothing to compare; sync again later"
	case single > 0:
		view.Note = fmt.Sprintf("%d console(s) have only one snapshot and were not compared", single)
	}
	return view, nil
}

func newNovelDriftCmd(flags *rootFlags) *cobra.Command {
	var dbFlag, sinceRaw string
	var limit int
	cmd := &cobra.Command{
		Use:   "drift",
		Short: "See field-level config changes between two synced snapshots, marked attributed (in the change journal) or out-of-band.",
		Long: `Use this command for field-level config changes between snapshots. Do NOT use it for
a cross-category timeline; use 'digest' instead.

Compares networks, firewall policies and zones, WiFi, DNS policies, ACL rules and
traffic matching lists between the two most recent snapshots of each console. A
change is attributed when the change journal holds a change to that entity in the
window, and out-of-band otherwise. Secret-named values are never shown.`,
		Example: strings.Trim(`
  unifi-pp-cli drift --agent
  unifi-pp-cli drift --since 7d --console EXAMPLE-CONSOLE-ID --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "drift")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cutoff, err := parseSinceFlag(sinceRaw, "--since", time.Now())
			if err != nil {
				return err
			}
			dbPath := mirrorDBPath(dbFlag)
			if mirrorMissing(dbPath) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: unifi-pp-cli sync --console <id> --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), driftView{Windows: []driftWindow{}, Changes: []driftChange{}, Note: "no local mirror; run sync first"}, flags)
				}
				return nil
			}
			db, err := openMirror(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			view, err := loadDrift(ctx, db, store.ActiveConsole(), cutoff)
			if err != nil {
				return err
			}
			if limit > 0 && len(view.Changes) > limit {
				view.Changes = view.Changes[:limit]
				view.Note = strings.TrimSpace(view.Note + " output capped by --limit")
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(view.Changes) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), firstNonEmpty(view.Note, "No config changes between snapshots."))
				return nil
			}
			for _, change := range view.Changes {
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s %q [%s]\n", change.Change, change.Category, change.EntityID, change.Name, change.Attribution)
				for _, f := range change.Fields {
					if f.SecretChanged {
						fmt.Fprintf(cmd.OutOrStdout(), "  %s: secret changed (value hidden)\n", f.Path)
						continue
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  %s: %v -> %v\n", f.Path, f.Before, f.After)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbFlag, "db", "", "Local mirror path (default: the standard mirror)")
	cmd.Flags().StringVar(&sinceRaw, "since", "", "Compare against the snapshot at or before this age, e.g. 7d (default: previous snapshot)")
	cmd.Flags().IntVar(&limit, "limit", 200, "Maximum changes to return; 0 for all")
	return cmd
}

func parseSinceFlag(raw, flagName string, now time.Time) (time.Time, error) {
	if raw == "" {
		return time.Time{}, nil
	}
	window, err := parseLooseDuration(raw)
	if err != nil {
		return time.Time{}, usageErr(fmt.Errorf("%s %q: %w", flagName, raw, err))
	}
	return now.Add(-window), nil
}
