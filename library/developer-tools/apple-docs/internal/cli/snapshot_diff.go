// Copyright 2026 joseph-alvin-castillo. Licensed under Apache-2.0. See LICENSE.

// pp:data-source auto

package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/apple-docs/internal/applejson"

	"github.com/spf13/cobra"
)

func snapshotDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".cache", "apple-docs-pp-cli", "snapshots")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	return dir, nil
}

// ---- snapshot save ----

func newNovelSnapshotSaveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "save <framework>",
		Short: "Fetch and save a dated copy of the framework's index for later diffing",
		Long: strings.TrimSpace(`
Fetch /tutorials/data/index/<framework>.json and save a dated copy to
~/.cache/apple-docs-pp-cli/snapshots/<framework>-<YYYY-MM-DD>.json.

Run this right before WWDC each year (or before any release you'll want to
diff against). Then run 'snapshot diff' to see what changed.
`),
		Example: strings.Trim(`
  apple-docs-pp-cli snapshot save swiftui
  apple-docs-pp-cli snapshot save foundation
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch and save snapshot")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<framework> is required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			framework := strings.ToLower(strings.Trim(args[0], "/ "))
			raw, err := c.Get(cmd.Context(), "/tutorials/data/index/"+framework+".json", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			dir, err := snapshotDir()
			if err != nil {
				return err
			}
			date := time.Now().UTC().Format("2006-01-02")
			path := filepath.Join(dir, framework+"-"+date+".json")
			if err := os.WriteFile(path, raw, 0o600); err != nil {
				return err
			}
			return emitJSON(cmd, flags, map[string]any{
				"framework": framework,
				"date":      date,
				"path":      path,
				"bytes":     len(raw),
			})
		},
	}
	return cmd
}

// ---- snapshot list ----

func newNovelSnapshotListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List saved snapshots in ~/.cache/apple-docs-pp-cli/snapshots",
		Example: "  apple-docs-pp-cli snapshot list\n" +
			"  apple-docs-pp-cli snapshot list --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list saved snapshots")
				return nil
			}
			dir, err := snapshotDir()
			if err != nil {
				return err
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				return err
			}
			rows := make([]map[string]any, 0, len(entries))
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				name := e.Name()
				if !strings.HasSuffix(name, ".json") {
					continue
				}
				stem := strings.TrimSuffix(name, ".json")
				framework, date := splitSnapshotName(stem)
				info, _ := e.Info()
				row := map[string]any{
					"framework": framework,
					"date":      date,
					"path":      filepath.Join(dir, name),
				}
				if info != nil {
					row["bytes"] = info.Size()
				}
				rows = append(rows, row)
			}
			sort.SliceStable(rows, func(i, j int) bool {
				if rows[i]["framework"] != rows[j]["framework"] {
					return fmt.Sprint(rows[i]["framework"]) < fmt.Sprint(rows[j]["framework"])
				}
				return fmt.Sprint(rows[i]["date"]) < fmt.Sprint(rows[j]["date"])
			})
			return emitJSON(cmd, flags, map[string]any{"snapshots": rows, "count": len(rows)})
		},
	}
	return cmd
}

// ---- snapshot diff ----

func newNovelSnapshotDiffCmd(flags *rootFlags) *cobra.Command {
	var flagFrom string
	var flagTo string
	var flagClassify bool
	var flagRenameDist int

	cmd := &cobra.Command{
		Use:   "diff <framework>",
		Short: "Classify deltas between two saved framework-index snapshots",
		Long: strings.TrimSpace(`
Load two saved snapshots of a framework's index and classify every delta:
  added       — path is in the 'to' snapshot but not the 'from'
  removed     — path is in the 'from' snapshot but not the 'to'
  deprecated  — path is in both, with deprecated=true only in 'to'
  renamed*    — when --classify is set, a removed symbol whose path stem is
                Levenshtein-close (<= --rename-dist edits) to an added symbol
                in the same parent directory is paired up as a likely rename.

Snapshots must already exist on disk; create them with 'snapshot save'.

Use this command for a structured added/removed/deprecated/renamed delta
between two saved snapshots of one framework. Do NOT use it for the rolling
'what's new since last sync' feed; use 'updates' instead. Do NOT use it for a
single-version cliff report; use 'deprecation-cliff' instead.
`),
		Example: strings.Trim(`
  apple-docs-pp-cli snapshot diff swiftui --from 2025-06-09 --to 2026-05-28 --classify --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would diff snapshots")
				return nil
			}
			if len(args) == 0 || flagFrom == "" || flagTo == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<framework>, --from, and --to are required"))
			}
			framework := strings.ToLower(strings.Trim(args[0], "/ "))
			dir, err := snapshotDir()
			if err != nil {
				return err
			}
			fromPath := filepath.Join(dir, framework+"-"+flagFrom+".json")
			toPath := filepath.Join(dir, framework+"-"+flagTo+".json")
			fromIdx, err := loadSnapshot(fromPath)
			if err != nil {
				return err
			}
			toIdx, err := loadSnapshot(toPath)
			if err != nil {
				return err
			}
			res := diffSnapshots(fromIdx, toIdx, flagClassify, flagRenameDist)
			res.Framework = framework
			res.From = flagFrom
			res.To = flagTo
			return emitJSON(cmd, flags, res)
		},
	}
	cmd.Flags().StringVar(&flagFrom, "from", "", "Source snapshot date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&flagTo, "to", "", "Target snapshot date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&flagClassify, "classify", false, "Pair removed/added symbols by path-stem proximity as 'renamed'")
	cmd.Flags().IntVar(&flagRenameDist, "rename-dist", 4, "Maximum Levenshtein edit distance for rename pairing")
	return cmd
}

func splitSnapshotName(stem string) (framework, date string) {
	idx := strings.LastIndex(stem, "-20")
	if idx < 0 {
		return stem, ""
	}
	return stem[:idx], stem[idx+1:]
}

func loadSnapshot(path string) (*applejson.FrameworkIndex, error) {
	// Containment check: refuse paths that escape the snapshot dir. The
	// path is built from snapshotDir() + framework + date, but framework
	// is user input, so a value with embedded path separators or ".."
	// could traverse out of the cache. Cleaning + a prefix check on the
	// resolved snapshot dir keeps the read inside the trusted location.
	dir, err := snapshotDir()
	if err != nil {
		return nil, err
	}
	cleaned := filepath.Clean(path)
	if !strings.HasPrefix(cleaned, filepath.Clean(dir)+string(filepath.Separator)) {
		return nil, fmt.Errorf("snapshot path escapes snapshot directory: %s", path)
	}
	f, err := os.Open(cleaned) // #nosec G304 -- path validated above against snapshotDir()
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("snapshot not found: %s (run 'snapshot save' first)", cleaned)
		}
		return nil, err
	}
	defer f.Close()
	raw, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return applejson.ParseIndex(raw)
}

type snapshotDiffResult struct {
	Framework  string         `json:"framework"`
	From       string         `json:"from"`
	To         string         `json:"to"`
	Added      []symbolEntry  `json:"added,omitempty"`
	Removed    []symbolEntry  `json:"removed,omitempty"`
	Deprecated []symbolEntry  `json:"deprecated,omitempty"`
	Renamed    []renameEntry  `json:"renamed,omitempty"`
	Counts     map[string]int `json:"counts"`
}

type symbolEntry struct {
	Title string `json:"title"`
	Path  string `json:"path"`
	Type  string `json:"type,omitempty"`
}

type renameEntry struct {
	From symbolEntry `json:"from"`
	To   symbolEntry `json:"to"`
	Dist int         `json:"edit_dist"`
}

func diffSnapshots(from, to *applejson.FrameworkIndex, classify bool, renameDist int) *snapshotDiffResult {
	fromMap := map[string]*applejson.IndexNode{}
	toMap := map[string]*applejson.IndexNode{}
	from.WalkSwift(func(n *applejson.IndexNode) {
		if n.Path == "" {
			return
		}
		fromMap[n.Path] = n
	})
	to.WalkSwift(func(n *applejson.IndexNode) {
		if n.Path == "" {
			return
		}
		toMap[n.Path] = n
	})
	res := &snapshotDiffResult{Counts: map[string]int{}}
	for p, n := range toMap {
		if _, ok := fromMap[p]; !ok {
			res.Added = append(res.Added, symbolEntry{Title: n.Title, Path: p, Type: n.Type})
		}
	}
	for p, n := range fromMap {
		other, ok := toMap[p]
		if !ok {
			res.Removed = append(res.Removed, symbolEntry{Title: n.Title, Path: p, Type: n.Type})
			continue
		}
		if !n.Deprecated && other.Deprecated {
			res.Deprecated = append(res.Deprecated, symbolEntry{Title: other.Title, Path: p, Type: other.Type})
		}
	}
	sort.SliceStable(res.Added, func(i, j int) bool { return res.Added[i].Path < res.Added[j].Path })
	sort.SliceStable(res.Removed, func(i, j int) bool { return res.Removed[i].Path < res.Removed[j].Path })
	sort.SliceStable(res.Deprecated, func(i, j int) bool { return res.Deprecated[i].Path < res.Deprecated[j].Path })

	if classify {
		// Pair removed↔added by parent dir + stem similarity.
		addedByDir := map[string][]symbolEntry{}
		for _, a := range res.Added {
			addedByDir[parentDir(a.Path)] = append(addedByDir[parentDir(a.Path)], a)
		}
		used := map[string]bool{}
		var renames []renameEntry
		newRemoved := make([]symbolEntry, 0, len(res.Removed))
		for _, r := range res.Removed {
			dir := parentDir(r.Path)
			candidates := addedByDir[dir]
			rStem := applejson.PathStem(r.Path)
			bestIdx := -1
			bestDist := renameDist + 1
			for i, c := range candidates {
				if used[c.Path] {
					continue
				}
				cStem := applejson.PathStem(c.Path)
				if cStem == rStem {
					bestIdx = i
					bestDist = 0
					break
				}
				dist := applejson.Levenshtein(rStem, cStem)
				if dist <= renameDist && dist < bestDist {
					bestIdx = i
					bestDist = dist
				}
			}
			if bestIdx >= 0 {
				used[candidates[bestIdx].Path] = true
				renames = append(renames, renameEntry{From: r, To: candidates[bestIdx], Dist: bestDist})
			} else {
				newRemoved = append(newRemoved, r)
			}
		}
		// Filter out the now-claimed added entries.
		newAdded := res.Added[:0]
		for _, a := range res.Added {
			if !used[a.Path] {
				newAdded = append(newAdded, a)
			}
		}
		res.Added = newAdded
		res.Removed = newRemoved
		res.Renamed = renames
	}
	res.Counts["added"] = len(res.Added)
	res.Counts["removed"] = len(res.Removed)
	res.Counts["deprecated"] = len(res.Deprecated)
	res.Counts["renamed"] = len(res.Renamed)
	return res
}

func parentDir(p string) string {
	p = strings.TrimSuffix(p, "/")
	idx := strings.LastIndex(p, "/")
	if idx < 0 {
		return p
	}
	return p[:idx]
}
