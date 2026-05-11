package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/unify/internal/store"

	"github.com/spf13/cobra"
)

// newSchemaCmd is the parent for snapshot/diff. Snapshot writes a full
// objects+attributes+options dump to schema_snapshots; diff compares two
// snapshots.
func newSchemaCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Snapshot and diff the workspace schema (objects, attributes, options)",
		Long: `Unify has no server-side schema-change audit log. 'schema snapshot' writes
the current set of objects + attributes + options to a local snapshot row;
'schema diff' compares two snapshots and reports adds, removes, and type
changes per object.

Run 'unify-pp-cli sync --skip-records' first to make sure the local cache
of schema rows is up to date.`,
	}
	cmd.AddCommand(newSchemaSnapshotCmd(flags))
	cmd.AddCommand(newSchemaDiffCmd(flags))
	cmd.AddCommand(newSchemaListCmd(flags))
	return cmd
}

func newSchemaSnapshotCmd(flags *rootFlags) *cobra.Command {
	var dbPath, label string
	cmd := &cobra.Command{
		Use:         "snapshot",
		Short:       "Capture the current schema as a labelled snapshot",
		Example:     "  unify-pp-cli schema snapshot --label friday",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			s, err := store.Open(ctx, dbPath)
			if err != nil {
				return apiErr(err)
			}
			defer s.Close()
			id, err := s.Snapshot(ctx, label)
			if err != nil {
				return apiErr(err)
			}
			out := map[string]any{"snapshot_id": id, "label": label}
			blob, _ := json.MarshalIndent(out, "", "  ")
			return printOutputWithFlags(cmd.OutOrStdout(), blob, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to SQLite store")
	cmd.Flags().StringVar(&label, "label", "", "Optional snapshot label (e.g. 'friday')")
	return cmd
}

func newSchemaListCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:         "list",
		Aliases:     []string{"ls"},
		Short:       "List recent schema snapshots",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  unify-pp-cli schema list --limit 10 --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			s, err := store.Open(ctx, dbPath)
			if err != nil {
				return apiErr(err)
			}
			defer s.Close()
			if limit <= 0 {
				limit = 20
			}
			snaps, err := s.LatestSnapshots(ctx, limit)
			if err != nil {
				return apiErr(err)
			}
			view := make([]map[string]any, 0, len(snaps))
			for _, s := range snaps {
				view = append(view, map[string]any{
					"id":       s.ID,
					"label":    s.Label,
					"taken_at": s.TakenAt,
					"age":      secondsAgo(s.TakenAt),
				})
			}
			blob, _ := json.MarshalIndent(view, "", "  ")
			return printOutputWithFlags(cmd.OutOrStdout(), blob, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to SQLite store")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max snapshots to return")
	return cmd
}

func newSchemaDiffCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var fromID, toID int64
	var since string
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Diff two schema snapshots (default: latest two)",
		Long: `Reports per-object adds, removes, and type changes between two snapshots.

With no --from / --to flags, the two most-recent snapshots are diffed
(useful for "what changed since I last ran snapshot"). Use --since <duration>
to pick the oldest 'from' snapshot taken at least that long ago (e.g.
--since 1d for "since yesterday", --since 24h, --since 1w).`,
		Example: strings.Trim(`
  unify-pp-cli schema diff
  unify-pp-cli schema diff --from 12 --to 17 --agent
  unify-pp-cli schema diff --since 1d --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			s, err := store.Open(ctx, dbPath)
			if err != nil {
				return apiErr(err)
			}
			defer s.Close()

			if since != "" && fromID == 0 {
				d, err := parseHumanDuration(since)
				if err != nil {
					return usageErr(fmt.Errorf("--since: %w", err))
				}
				cutoff := time.Now().Add(-d).Unix()
				// Pick the most recent snapshot at-or-before cutoff.
				row := s.DB.QueryRowContext(ctx, `SELECT id FROM schema_snapshots WHERE taken_at <= ? ORDER BY taken_at DESC, id DESC LIMIT 1`, cutoff)
				if scanErr := row.Scan(&fromID); scanErr != nil {
					return apiErr(fmt.Errorf("--since %s: no snapshot found at or before that time. Run 'unify-pp-cli schema snapshot' more often.", since))
				}
			}

			from, to, err := resolveDiffSnapshots(ctx, s, fromID, toID)
			if err != nil {
				return apiErr(err)
			}
			diff, err := computeSchemaDiff(from, to)
			if err != nil {
				return apiErr(err)
			}
			diff["from_id"] = from.ID
			diff["to_id"] = to.ID
			blob, _ := json.MarshalIndent(diff, "", "  ")
			return printOutputWithFlags(cmd.OutOrStdout(), blob, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to SQLite store")
	cmd.Flags().Int64Var(&fromID, "from", 0, "Older snapshot id (default = next-to-latest)")
	cmd.Flags().Int64Var(&toID, "to", 0, "Newer snapshot id (default = latest)")
	cmd.Flags().StringVar(&since, "since", "", "Pick the oldest 'from' snapshot taken at least this long ago (e.g. 1d, 24h, 1w)")
	return cmd
}

func resolveDiffSnapshots(ctx context.Context, s *store.Store, fromID, toID int64) (*store.SnapshotRow, *store.SnapshotRow, error) {
	if fromID == 0 || toID == 0 {
		snaps, err := s.LatestSnapshots(ctx, 2)
		if err != nil {
			return nil, nil, err
		}
		if len(snaps) < 2 {
			return nil, nil, fmt.Errorf("need at least 2 snapshots to diff (have %d). Run 'schema snapshot' more than once.", len(snaps))
		}
		// snaps[0] is newest.
		if toID == 0 {
			toID = snaps[0].ID
		}
		if fromID == 0 {
			fromID = snaps[1].ID
		}
	}
	from, err := s.GetSnapshot(ctx, fromID)
	if err != nil {
		return nil, nil, err
	}
	to, err := s.GetSnapshot(ctx, toID)
	if err != nil {
		return nil, nil, err
	}
	return from, to, nil
}

type snapshotDoc struct {
	Objects []store.Object               `json:"objects"`
	Attrs   map[string][]store.Attribute `json:"attrs"`
}

func parseSnapshot(s *store.SnapshotRow) (*snapshotDoc, error) {
	var doc snapshotDoc
	if err := json.Unmarshal(s.Data, &doc); err != nil {
		return nil, fmt.Errorf("parse snapshot %d: %w", s.ID, err)
	}
	if doc.Attrs == nil {
		doc.Attrs = map[string][]store.Attribute{}
	}
	return &doc, nil
}

func computeSchemaDiff(from, to *store.SnapshotRow) (map[string]any, error) {
	a, err := parseSnapshot(from)
	if err != nil {
		return nil, err
	}
	b, err := parseSnapshot(to)
	if err != nil {
		return nil, err
	}

	objA := objectSet(a.Objects)
	objB := objectSet(b.Objects)

	addedObjects := setDiff(objB, objA)
	removedObjects := setDiff(objA, objB)

	type attrChange struct {
		Object   string `json:"object"`
		Name     string `json:"name"`
		FromType string `json:"from_type,omitempty"`
		ToType   string `json:"to_type,omitempty"`
	}
	addedAttrs := []attrChange{}
	removedAttrs := []attrChange{}
	typeChanges := []attrChange{}

	// Look at every object present in either snapshot.
	objs := unionSet(objA, objB)
	sort.Strings(objs)
	for _, o := range objs {
		fromMap := attrMap(a.Attrs[o])
		toMap := attrMap(b.Attrs[o])
		for name, at := range toMap {
			if _, ok := fromMap[name]; !ok {
				addedAttrs = append(addedAttrs, attrChange{Object: o, Name: name, ToType: at.Type})
				continue
			}
			if fromMap[name].Type != at.Type {
				typeChanges = append(typeChanges, attrChange{Object: o, Name: name, FromType: fromMap[name].Type, ToType: at.Type})
			}
		}
		for name, at := range fromMap {
			if _, ok := toMap[name]; !ok {
				removedAttrs = append(removedAttrs, attrChange{Object: o, Name: name, FromType: at.Type})
			}
		}
	}

	return map[string]any{
		"added_objects":   addedObjects,
		"removed_objects": removedObjects,
		"added_attrs":     addedAttrs,
		"removed_attrs":   removedAttrs,
		"type_changes":    typeChanges,
		"summary": map[string]int{
			"added_objects":   len(addedObjects),
			"removed_objects": len(removedObjects),
			"added_attrs":     len(addedAttrs),
			"removed_attrs":   len(removedAttrs),
			"type_changes":    len(typeChanges),
		},
	}, nil
}

func objectSet(objs []store.Object) map[string]struct{} {
	m := map[string]struct{}{}
	for _, o := range objs {
		m[o.APIName] = struct{}{}
	}
	return m
}

func setDiff(a, b map[string]struct{}) []string {
	out := []string{}
	for k := range a {
		if _, ok := b[k]; !ok {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func unionSet(a, b map[string]struct{}) []string {
	out := map[string]struct{}{}
	for k := range a {
		out[k] = struct{}{}
	}
	for k := range b {
		out[k] = struct{}{}
	}
	res := make([]string, 0, len(out))
	for k := range out {
		res = append(res, k)
	}
	return res
}

func attrMap(as []store.Attribute) map[string]store.Attribute {
	m := map[string]store.Attribute{}
	for _, a := range as {
		m[a.APIName] = a
	}
	return m
}
