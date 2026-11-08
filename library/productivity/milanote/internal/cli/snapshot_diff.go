// Copyright 2026 Beau Sterling and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/milanote/internal/milanote"
)

type snapshotChange struct {
	Kind   string `json:"kind"` // added, removed, moved, edited, ticked, unticked, resized, recoloured
	ID     string `json:"id"`
	Type   string `json:"type"`
	Text   string `json:"text,omitempty"`
	Detail string `json:"detail,omitempty"`
}

func newNovelSnapshotDiffCmd(flags *rootFlags) *cobra.Command {
	var from, to string

	cmd := &cobra.Command{
		Use:   "diff <board>",
		Short: "See which cards were added, removed, moved, edited or ticked since a snapshot.",
		Example: `  milanote-pp-cli snapshot diff 1Ab2Cd3EfGh4iJ --json
  milanote-pp-cli snapshot diff 1Ab2Cd3EfGh4iJ --from before-rewrite --to latest`,
		Annotations: readAnnotations("live"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "snapshot diff")
			}
			boardID := boardRef(args[0])
			base, err := resolveSnapshot(boardID, from)
			if err != nil {
				return err
			}
			var head *boardSnapshot
			if to == "" || to == "live" {
				if head, err = captureBoard(cmd, flags, boardID); err != nil {
					return err
				}
				head.ID = "live"
			} else if head, err = resolveSnapshot(boardID, to); err != nil {
				return err
			}
			changes := diffSnapshots(base, head)
			result := map[string]any{"board": boardID, "from": base.ID, "to": head.ID, "changes": changes}
			return emit(cmd, flags, result, func(w io.Writer) error {
				if len(changes) == 0 {
					_, err := fmt.Fprintf(w, "No changes between %s and %s.\n", base.ID, head.ID)
					return err
				}
				fmt.Fprintf(w, "%d change(s) between %s and %s:\n", len(changes), base.ID, head.ID)
				for _, ch := range changes {
					line := fmt.Sprintf("  %-10s %s", ch.Kind, ch.Text)
					if ch.Detail != "" {
						line += "  (" + ch.Detail + ")"
					}
					if _, err := fmt.Fprintln(w, line); err != nil {
						return err
					}
				}
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&from, "from", "latest", "Snapshot to compare from (id, name or latest)")
	cmd.Flags().StringVar(&to, "to", "live", "Snapshot to compare to (id, name, latest or live)")
	cmd.Annotations["pp:verified-by-proof"] = "milanote-lifecycle.md"
	return cmd
}

func snapElements(s *boardSnapshot) map[string]*milanote.Element {
	out := map[string]*milanote.Element{}
	for id, el := range s.Elements {
		if id == s.Board || el.Location.Section == milanote.SectionTrash {
			continue
		}
		out[id] = el
	}
	return out
}

func taskText(el *milanote.Element) string {
	var parts []string
	for _, t := range el.Tasks() {
		parts = append(parts, t.Text)
	}
	return strings.Join(parts, "\n")
}

func diffSnapshots(a, b *boardSnapshot) []snapshotChange {
	before, after := snapElements(a), snapElements(b)
	out := []snapshotChange{}
	add := func(kind string, el *milanote.Element, detail string) {
		out = append(out, snapshotChange{Kind: kind, ID: el.ID, Type: strings.ToLower(el.ElementType), Text: preview(el.Text()), Detail: detail})
	}
	for id, old := range before {
		cur := after[id]
		if cur == nil {
			add("removed", old, "")
			continue
		}
		ol, cl := old.Location, cur.Location
		switch {
		case ol.ParentID != cl.ParentID || ol.Section != cl.Section:
			add("moved", cur, "to another column or panel")
		case ol.X() != cl.X() || ol.Y() != cl.Y() || ol.Index() != cl.Index():
			add("moved", cur, fmt.Sprintf("%g,%g to %g,%g", ol.X(), ol.Y(), cl.X(), cl.Y()))
		}
		if old.ElementType == milanote.TypeTaskList {
			if taskText(old) != taskText(cur) {
				add("edited", cur, "")
			}
			was := map[string]bool{}
			for _, t := range old.Tasks() {
				was[t.ID] = t.Checked
			}
			for _, t := range cur.Tasks() {
				if checked, ok := was[t.ID]; ok && checked != t.Checked {
					kind := "unticked"
					if t.Checked {
						kind = "ticked"
					}
					out = append(out, snapshotChange{Kind: kind, ID: cur.ID, Type: "task_list", Text: preview(t.Text)})
				}
			}
		} else if milanote.ElementMarkdown(old) != milanote.ElementMarkdown(cur) || old.Title() != cur.Title() {
			add("edited", cur, "")
		}
		if old.Width() != cur.Width() {
			add("resized", cur, fmt.Sprintf("width %g to %g", old.Width(), cur.Width()))
		}
		if old.Color() != cur.Color() {
			add("recoloured", cur, old.Color()+" to "+cur.Color())
		}
	}
	for id, cur := range after {
		if before[id] == nil {
			add("added", cur, "")
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Text < out[j].Text
	})
	return out
}
