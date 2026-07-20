// Copyright 2026 Richard Gill and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
// Novel command: config snapshot + diff. Capture the full PBX config graph to
// a named local snapshot, list snapshots, and compare any two snapshots to
// show added/removed/changed objects. Reads the local mirror; snapshots are
// JSON files under the data directory.

package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/3cx-xapi/internal/cliutil"
	"github.com/spf13/cobra"
)

// configSnapshotResources are the config entities captured in a snapshot.
var configSnapshotResources = []string{
	rtUsers, rtGroups, rtRingGroups, rtQueues, rtReceptionists, rtTrunks,
	"peers", rtInboundRules, rtOutboundRules, rtDidNumbers,
	"office-hours", "holidays", rtParkings,
}

type configSnapshot struct {
	Name        string                       `json:"name"`
	CapturedUTC string                       `json:"captured_utc"`
	Resources   map[string][]json.RawMessage `json:"resources"`
}

type resourceDiff struct {
	Resource string   `json:"resource"`
	Added    []string `json:"added"`
	Removed  []string `json:"removed"`
	Changed  []string `json:"changed"`
}

type diffResult struct {
	A            string         `json:"a"`
	B            string         `json:"b"`
	Diffs        []resourceDiff `json:"diffs"`
	TotalChanges int            `json:"total_changes"`
	Note         string         `json:"note,omitempty"`
}

func snapshotDir() (string, error) {
	base, err := cliutil.DataDir()
	if err != nil {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return "snapshots", nil
		}
		base = filepath.Join(home, ".local", "share", "3cx-xapi-pp-cli")
	}
	dir := filepath.Join(base, "snapshots")
	return dir, os.MkdirAll(dir, 0o700)
}

func snapshotPath(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name || strings.ContainsAny(name, `/\\`) {
		return "", usageErr(fmt.Errorf("invalid snapshot name %q: use a single file-safe name", name))
	}
	dir, err := snapshotDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".json"), nil
}

func writeSnapshot(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".snapshot-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// objectKey returns the stable identity of a config object for diffing.
func objectKey(raw json.RawMessage) string {
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return ""
	}
	if id := firstString(obj, "Id", "Number", "Name"); id != "" {
		return id
	}
	return ""
}

func objectHash(raw json.RawMessage) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:8])
}

// diffSnapshots is the pure comparison core: per resource type, compare object
// sets by key and content hash. Objects without a stable key are ignored.
func diffSnapshots(a, b configSnapshot) diffResult {
	res := diffResult{A: a.Name, B: b.Name, Diffs: []resourceDiff{}}
	seen := map[string]bool{}
	for _, rt := range configSnapshotResources {
		seen[rt] = true
		res.appendResourceDiff(rt, a.Resources[rt], b.Resources[rt])
	}
	// Cover any resource types present in either snapshot but not in the
	// canonical list (forward-compat with future snapshots).
	extra := map[string]bool{}
	for rt := range a.Resources {
		if !seen[rt] {
			extra[rt] = true
		}
	}
	for rt := range b.Resources {
		if !seen[rt] {
			extra[rt] = true
		}
	}
	extraSorted := make([]string, 0, len(extra))
	for rt := range extra {
		extraSorted = append(extraSorted, rt)
	}
	sort.Strings(extraSorted)
	for _, rt := range extraSorted {
		res.appendResourceDiff(rt, a.Resources[rt], b.Resources[rt])
	}
	for _, d := range res.Diffs {
		res.TotalChanges += len(d.Added) + len(d.Removed) + len(d.Changed)
	}
	return res
}

func (res *diffResult) appendResourceDiff(rt string, aObjs, bObjs []json.RawMessage) {
	aMap := map[string]string{}
	for _, o := range aObjs {
		if k := objectKey(o); k != "" {
			aMap[k] = objectHash(o)
		}
	}
	bMap := map[string]string{}
	for _, o := range bObjs {
		if k := objectKey(o); k != "" {
			bMap[k] = objectHash(o)
		}
	}
	d := resourceDiff{Resource: rt, Added: []string{}, Removed: []string{}, Changed: []string{}}
	for k, bh := range bMap {
		if ah, ok := aMap[k]; !ok {
			d.Added = append(d.Added, k)
		} else if ah != bh {
			d.Changed = append(d.Changed, k)
		}
	}
	for k := range aMap {
		if _, ok := bMap[k]; !ok {
			d.Removed = append(d.Removed, k)
		}
	}
	if len(d.Added)+len(d.Removed)+len(d.Changed) == 0 {
		return
	}
	sort.Strings(d.Added)
	sort.Strings(d.Removed)
	sort.Strings(d.Changed)
	res.Diffs = append(res.Diffs, d)
}

func loadSnapshot(name string) (configSnapshot, error) {
	var snap configSnapshot
	path, err := snapshotPath(name)
	if err != nil {
		return snap, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return snap, fmt.Errorf("snapshot %q not found (run: 3cx-xapi-pp-cli diff --save %s)", name, name)
	}
	if err := json.Unmarshal(data, &snap); err != nil {
		return snap, fmt.Errorf("snapshot %q is corrupt: %w", name, err)
	}
	return snap, nil
}

func newNovelDiffCmd(flags *rootFlags) *cobra.Command {
	var flagSave string
	var flagList bool
	var dbPath string
	cmd := &cobra.Command{
		Use:   "diff [snapshotA] [snapshotB]",
		Short: "Capture and compare PBX config snapshots (config drift detection)",
		Long: "Capture the full PBX config graph to a named local snapshot and compare any two\n" +
			"snapshots to see exactly what changed (added/removed/changed objects).\n\n" +
			"  diff --save <name>     capture a snapshot now\n" +
			"  diff --list            list saved snapshots\n" +
			"  diff <nameA> <nameB>   compare two snapshots\n\n" +
			"Use this command for config drift between two snapshots or two tenants. For broken\n" +
			"references right now use 'audit'; for live event activity use 'changed'.",
		Example:     "  3cx-xapi-pp-cli diff --save before\n  3cx-xapi-pp-cli diff before after --agent",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would capture or compare config snapshots")
				return nil
			}

			if flagList {
				dir, err := snapshotDir()
				if err != nil {
					return err
				}
				entries, _ := filepath.Glob(filepath.Join(dir, "*.json"))
				names := make([]string, 0, len(entries))
				for _, e := range entries {
					names = append(names, strings.TrimSuffix(filepath.Base(e), ".json"))
				}
				sort.Strings(names)
				if machineOut(cmd, flags) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"snapshots": names}, flags)
				}
				if len(names) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "no snapshots saved yet")
					return nil
				}
				for _, n := range names {
					fmt.Fprintln(cmd.OutOrStdout(), n)
				}
				return nil
			}

			if flagSave != "" {
				db, ok, err := openLocalMirror(cmd, flags, dbPath)
				if err != nil {
					return err
				}
				if !ok {
					return nil
				}
				defer db.Close()
				if !hintIfUnsynced(cmd, db, rtUsers) {
					hintIfStale(cmd, db, rtUsers, flags.maxAge)
				}
				snap := configSnapshot{
					Name:        flagSave,
					CapturedUTC: time.Now().UTC().Format(time.RFC3339),
					Resources:   map[string][]json.RawMessage{},
				}
				total := 0
				for _, rt := range configSnapshotResources {
					raws, err := db.List(rt, 0)
					if err != nil {
						return err
					}
					if raws == nil {
						raws = []json.RawMessage{}
					}
					snap.Resources[rt] = raws
					total += len(raws)
				}
				path, err := snapshotPath(flagSave)
				if err != nil {
					return err
				}
				out, _ := json.MarshalIndent(snap, "", "  ")
				if err := writeSnapshot(path, out); err != nil {
					return fmt.Errorf("writing snapshot: %w", err)
				}
				if machineOut(cmd, flags) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"snapshot": flagSave, "objects": total, "path": path, "captured_utc": snap.CapturedUTC,
					}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "saved snapshot %q (%d objects) to %s\n", flagSave, total, path)
				return nil
			}

			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("diff requires two snapshot names, e.g. 'diff before after', or use --save <name>"))
			}
			a, err := loadSnapshot(args[0])
			if err != nil {
				return notFoundErr(err)
			}
			b, err := loadSnapshot(args[1])
			if err != nil {
				return notFoundErr(err)
			}
			result := diffSnapshots(a, b)
			if result.TotalChanges == 0 {
				result.Note = "no differences between the two snapshots"
			}
			if machineOut(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Diff %s -> %s: %d change(s)\n", result.A, result.B, result.TotalChanges)
			for _, d := range result.Diffs {
				fmt.Fprintf(w, "  %s: +%d -%d ~%d\n", d.Resource, len(d.Added), len(d.Removed), len(d.Changed))
			}
			if result.Note != "" {
				fmt.Fprintln(w, result.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSave, "save", "", "Capture a snapshot now under this name")
	cmd.Flags().BoolVar(&flagList, "list", false, "List saved snapshots")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}
