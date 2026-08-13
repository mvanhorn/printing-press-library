// Copyright 2026 Ryan Gravette and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: diff the current catalog against the last local snapshot.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
)

type catalogEntry struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Price    int    `json:"price"`
	Modified string `json:"modified"`
}

type catalogChange struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Field string `json:"field"`
	Old   string `json:"old"`
	New   string `json:"new"`
}

type catalogDiffView struct {
	Baseline bool            `json:"baseline"`
	Added    []catalogEntry  `json:"added"`
	Removed  []catalogEntry  `json:"removed"`
	Changed  []catalogChange `json:"changed"`
	Total    int             `json:"total_current"`
	Note     string          `json:"note,omitempty"`
}

func catalogSnapshotPath() string {
	return filepath.Join(filepath.Dir(defaultDBPath("agilix-dawn-pp-cli")), "catalog-snapshot.json")
}

// pp:data-source computed
func newNovelCatalogDiffCmd(flags *rootFlags) *cobra.Command {
	var flagNoSave bool
	cmd := &cobra.Command{
		Use:         "diff",
		Short:       "Show what changed in the catalog since the last local sync (new/removed courses, price/status/title changes).",
		Long:        "Show what changed in the catalog since the last local snapshot (new/removed courses, price/status/title changes).\n\nUse to see what changed in the catalog since last run. The first run records a baseline.",
		Example:     "  agilix-dawn-pp-cli catalog diff --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch the catalog and diff against the local snapshot")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			matches, total, err := fetchAllSearch(ctx, c, "concept", map[string]any{
				"query":   "",
				"include": []string{"id", "title", "status", "price", "modified"},
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			// A cap-hit here would write a partial snapshot and mis-report
			// courses beyond the cap as Removed on the next run — warn loudly.
			warnTruncated(cmd.ErrOrStderr(), "catalog", len(matches), total)
			current := map[string]catalogEntry{}
			for _, m := range matches {
				var e catalogEntry
				if json.Unmarshal(m, &e) == nil && e.ID != "" {
					current[e.ID] = e
				}
			}
			view := catalogDiffView{Total: len(current)}
			snapPath := catalogSnapshotPath()

			// A transiently empty catalog fetch must not overwrite a good
			// snapshot (which would report everything removed next run).
			if len(current) == 0 {
				view.Note = "catalog fetch returned 0 courses; snapshot left unchanged"
				fmt.Fprintln(cmd.ErrOrStderr(), "warning: "+view.Note)
				return emitCatalogDiff(cmd, flags, view)
			}

			// #nosec G304 -- snapPath is program-derived (defaultDBPath dir + constant filename); no user input flows into it.
			prevRaw, readErr := os.ReadFile(snapPath)
			if readErr != nil {
				// First run — record a baseline.
				view.Baseline = true
				view.Note = fmt.Sprintf("baseline recorded (%d courses); re-run later to see changes", len(current))
				if !flagNoSave {
					if err := writeCatalogSnapshot(snapPath, current); err != nil {
						return err
					}
				}
				return emitCatalogDiff(cmd, flags, view)
			}

			var prev map[string]catalogEntry
			if err := json.Unmarshal(prevRaw, &prev); err != nil {
				// A corrupt snapshot must not silently masquerade as an empty
				// baseline (which would flag the whole catalog as "Added").
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: local snapshot at %s is unreadable (%v); recording a fresh baseline\n",
					snapPath, err)
				view.Baseline = true
				view.Note = fmt.Sprintf("previous snapshot was corrupt; baseline reset (%d courses)", len(current))
				if !flagNoSave {
					if werr := writeCatalogSnapshot(snapPath, current); werr != nil {
						return werr
					}
				}
				return emitCatalogDiff(cmd, flags, view)
			}
			for id, e := range current {
				old, ok := prev[id]
				if !ok {
					view.Added = append(view.Added, e)
					continue
				}
				if old.Title != e.Title {
					view.Changed = append(view.Changed, catalogChange{id, e.Title, "title", old.Title, e.Title})
				}
				if old.Status != e.Status {
					view.Changed = append(view.Changed, catalogChange{id, e.Title, "status", old.Status, e.Status})
				}
				if old.Price != e.Price {
					view.Changed = append(view.Changed, catalogChange{id, e.Title, "price",
						fmt.Sprintf("%d", old.Price), fmt.Sprintf("%d", e.Price)})
				}
			}
			for id, e := range prev {
				if _, ok := current[id]; !ok {
					view.Removed = append(view.Removed, e)
				}
			}
			sort.Slice(view.Added, func(i, j int) bool { return view.Added[i].ID < view.Added[j].ID })
			sort.Slice(view.Removed, func(i, j int) bool { return view.Removed[i].ID < view.Removed[j].ID })
			sort.Slice(view.Changed, func(i, j int) bool {
				if view.Changed[i].ID != view.Changed[j].ID {
					return view.Changed[i].ID < view.Changed[j].ID
				}
				return view.Changed[i].Field < view.Changed[j].Field
			})
			if !flagNoSave {
				if err := writeCatalogSnapshot(snapPath, current); err != nil {
					return err
				}
			}
			return emitCatalogDiff(cmd, flags, view)
		},
	}
	cmd.Flags().BoolVar(&flagNoSave, "no-save", false, "Do not update the local snapshot after diffing")
	return cmd
}

func writeCatalogSnapshot(path string, m map[string]catalogEntry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("creating snapshot dir: %w", err)
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	// Write atomically: a temp file in the same dir renamed into place, so an
	// interrupt cannot leave a truncated snapshot behind.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".catalog-snapshot-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp snapshot: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("writing temp snapshot: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("closing temp snapshot: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("finalizing snapshot: %w", err)
	}
	return nil
}

func emitCatalogDiff(cmd *cobra.Command, flags *rootFlags, view catalogDiffView) error {
	if flags.asJSON {
		return flags.printJSON(cmd, view)
	}
	w := cmd.OutOrStdout()
	if view.Baseline {
		fmt.Fprintln(w, view.Note)
		return nil
	}
	if len(view.Added) == 0 && len(view.Removed) == 0 && len(view.Changed) == 0 {
		fmt.Fprintf(w, "no catalog changes (%d courses)\n", view.Total)
		return nil
	}
	for _, e := range view.Added {
		fmt.Fprintf(w, "+ added    %s  %s\n", e.ID, e.Title)
	}
	for _, e := range view.Removed {
		fmt.Fprintf(w, "- removed  %s  %s\n", e.ID, e.Title)
	}
	for _, ch := range view.Changed {
		fmt.Fprintf(w, "~ changed  %s  %s: %s → %s  (%s)\n", ch.ID, ch.Field, ch.Old, ch.New, ch.Title)
	}
	return nil
}
