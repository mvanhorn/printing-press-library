// Copyright 2026 Derick Ng and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: diff the two most recent zone-record snapshots.
// pp:data-source local

package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/dnsmadeeasy/internal/store"

	"github.com/spf13/cobra"
)

type driftEntry struct {
	Zone     string `json:"zone"`
	RecordID string `json:"record_id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	OldName  string `json:"old_name,omitempty"`
	OldType  string `json:"old_type,omitempty"`
	Old      string `json:"old_value,omitempty"`
	New      string `json:"new_value,omitempty"`
	OldTTL   int    `json:"old_ttl,omitempty"`
	NewTTL   int    `json:"new_ttl,omitempty"`
}

type driftView struct {
	FromBatch string       `json:"from_batch"`
	ToBatch   string       `json:"to_batch"`
	Added     []driftEntry `json:"added"`
	Removed   []driftEntry `json:"removed"`
	Changed   []driftEntry `json:"changed"`
	Note      string       `json:"note,omitempty"`
}

type snapRow struct {
	zone, name, typ, value string
	ttl                    int
}

func newNovelDriftCmd(flags *rootFlags) *cobra.Command {
	var flagZone string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "drift [zone]",
		Short: "Diff the two most recent record snapshots (added, changed, removed)",
		Long: strings.Trim(`
Compare the two most recent 'sync-records' snapshots and report which records
were added, changed, or removed. The API has no history; drift exists only
because each sync-records run snapshots the account locally.

Run 'dnsmadeeasy-pp-cli sync-records' at least twice to have something to diff.`, "\n"),
		Example:     "  dnsmadeeasy-pp-cli drift --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would diff the two most recent snapshots")
				return nil
			}
			if len(args) > 0 {
				flagZone = args[0]
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("dnsmadeeasy-pp-cli")
			}
			s, ok, err := openZoneMirror(ctx, cmd, dbPath)
			if err != nil {
				return err
			}
			if !ok {
				if flags.asJSON || flags.agent {
					return printJSONFiltered(cmd.OutOrStdout(), driftView{
						Added: []driftEntry{}, Removed: []driftEntry{}, Changed: []driftEntry{},
						Note: "no local record mirror yet; run 'dnsmadeeasy-pp-cli sync-records' first",
					}, flags)
				}
				return nil
			}
			defer s.Close()

			batches, err := recentBatches(ctx, s, 2)
			if err != nil {
				return fmt.Errorf("reading snapshots: %w", err)
			}
			view := driftView{Added: []driftEntry{}, Removed: []driftEntry{}, Changed: []driftEntry{}}
			if len(batches) < 2 {
				view.Note = "need at least 2 snapshots to compute drift; run 'dnsmadeeasy-pp-cli sync-records' again after changes"
				if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			view.ToBatch = batches[0]
			view.FromBatch = batches[1]

			newRows, err := snapshotRows(ctx, s, view.ToBatch, flagZone)
			if err != nil {
				return err
			}
			oldRows, err := snapshotRows(ctx, s, view.FromBatch, flagZone)
			if err != nil {
				return err
			}
			for id, nr := range newRows {
				or, existed := oldRows[id]
				if !existed {
					view.Added = append(view.Added, driftEntry{Zone: nr.zone, RecordID: id, Name: nr.name, Type: nr.typ, New: nr.value, NewTTL: nr.ttl})
					continue
				}
				if snapRowsDiffer(or, nr) {
					entry := driftEntry{Zone: nr.zone, RecordID: id, Name: nr.name, Type: nr.typ, Old: or.value, New: nr.value, OldTTL: or.ttl, NewTTL: nr.ttl}
					if or.name != nr.name {
						entry.OldName = or.name
					}
					if or.typ != nr.typ {
						entry.OldType = or.typ
					}
					view.Changed = append(view.Changed, entry)
				}
			}
			for id, or := range oldRows {
				if _, existed := newRows[id]; !existed {
					view.Removed = append(view.Removed, driftEntry{Zone: or.zone, RecordID: id, Name: or.name, Type: or.typ, Old: or.value, OldTTL: or.ttl})
				}
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "drift %s -> %s: %d added, %d changed, %d removed\n",
				view.FromBatch, view.ToBatch, len(view.Added), len(view.Changed), len(view.Removed))
			for _, e := range view.Added {
				fmt.Fprintf(cmd.OutOrStdout(), "  + %s %s %s -> %s\n", e.Zone, e.Name, e.Type, e.New)
			}
			for _, e := range view.Changed {
				changes := []string{}
				if e.OldName != "" {
					changes = append(changes, fmt.Sprintf("name %s -> %s", nameOrApex(e.OldName), nameOrApex(e.Name)))
				}
				if e.OldType != "" {
					changes = append(changes, fmt.Sprintf("type %s -> %s", e.OldType, e.Type))
				}
				if e.Old != e.New {
					changes = append(changes, fmt.Sprintf("value %s -> %s", e.Old, e.New))
				}
				if e.OldTTL != e.NewTTL {
					changes = append(changes, fmt.Sprintf("TTL %d -> %d", e.OldTTL, e.NewTTL))
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  ~ %s %s: %s\n", e.Zone, e.RecordID, strings.Join(changes, ", "))
			}
			for _, e := range view.Removed {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s %s %s (was %s)\n", e.Zone, e.Name, e.Type, e.Old)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local mirror database")
	return cmd
}

func snapRowsDiffer(old, current snapRow) bool {
	return old.zone != current.zone || old.name != current.name || old.typ != current.typ || old.value != current.value || old.ttl != current.ttl
}

// recentBatches returns up to n batch ids, most recent first.
func recentBatches(ctx context.Context, s *store.Store, n int) ([]string, error) {
	rows, err := s.DB().QueryContext(ctx, `SELECT batch_id FROM record_snapshots GROUP BY batch_id ORDER BY MAX(taken_at) DESC LIMIT ?`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var b string
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// snapshotRows returns a batch's records keyed by "domainId/recordId".
func snapshotRows(ctx context.Context, s *store.Store, batch, zoneFilter string) (map[string]snapRow, error) {
	q := `SELECT domain_id, record_id, domain_name, name, type, value, ttl FROM record_snapshots WHERE batch_id = ?`
	argv := []any{batch}
	if zoneFilter != "" {
		q += ` AND domain_name = ? COLLATE NOCASE`
		argv = append(argv, strings.TrimSuffix(zoneFilter, "."))
	}
	rows, err := s.DB().QueryContext(ctx, q, argv...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]snapRow{}
	for rows.Next() {
		var domID, recID, zone, name, typ, value string
		var ttl int
		if err := rows.Scan(&domID, &recID, &zone, &name, &typ, &value, &ttl); err != nil {
			return nil, err
		}
		out[domID+"/"+recID] = snapRow{zone: zone, name: name, typ: typ, value: value, ttl: ttl}
	}
	return out, rows.Err()
}
