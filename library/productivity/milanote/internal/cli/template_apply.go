// Copyright 2026 Beau Sterling and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/mvanhorn/printing-press-library/library/productivity/milanote/internal/milanote"
)

func newNovelTemplateApplyCmd(flags *rootFlags) *cobra.Command {
	var parent, name string

	cmd := &cobra.Command{
		Use:   "apply <kit.yaml>",
		Short: "Create a new board from a YAML kit made with 'template save'",
		Long: `Builds a fresh board from a kit: same headings, columns, notes, to-do lists
and sub-boards in the same relative positions, all with new ids. The new board
appears in the Unsorted panel of its parent (your Home by default).`,
		Example:     "  milanote-pp-cli template apply film-kit.yaml --parent 1Ab2Cd3EfGh4iJ --name 'New short' --dry-run",
		Annotations: writeAnnotations(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) && requiredInputWaived() {
				return writeDryRun(cmd.OutOrStdout(), flags, "template apply")
			}
			raw, err := readLocalFile(args[0])
			if err != nil {
				return usageErr(err)
			}
			var k kit
			if err := yaml.Unmarshal(raw, &k); err != nil {
				return usageErr(fmt.Errorf("reading kit: %w", err))
			}
			if k.Version != 1 {
				return usageErr(fmt.Errorf("%s is not a kit made by 'template save' (version %d)", args[0], k.Version))
			}
			title := strings.TrimSpace(name)
			if title == "" {
				title = k.Title
			}
			if title == "" {
				return usageErr(errors.New("--name is required: the kit has no title"))
			}
			total := countKit(k.Items)
			if dryRunOK(flags) {
				return emit(cmd, flags, map[string]any{"dry_run": true, "title": title, "elements": total}, func(w io.Writer) error {
					_, err := fmt.Fprintf(w, "dry-run: would create board %q with %d element(s)\n", title, total)
					return err
				})
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := mnClient(flags)
			if err != nil {
				return err
			}
			parentID := boardRef(parent)
			if parentID == "" {
				me, err := c.Me(ctx)
				if err != nil {
					return mnErr(err)
				}
				parentID = me.RootBoardID
			}
			payload, err := loadBoard(ctx, c, parentID)
			if err != nil {
				return err
			}
			w, err := c.Writer(ctx)
			if err != nil {
				return mnErr(err)
			}
			defer w.Close(ctx)
			id, err := w.Create(ctx, parentID, milanote.TypeBoard, milanote.Unsorted(payload, parentID), milanote.BoardContent(title), "")
			if err != nil {
				return mnErr(err)
			}
			if err := applyKitItems(ctx, w, id, id, k.Items); err != nil {
				return mnErr(fmt.Errorf("board %s created but filling it failed: %w", id, err))
			}
			return emit(cmd, flags, map[string]any{"board": id, "board_url": boardURL(id), "title": title, "parent": parentID, "elements": total}, func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "%s created %q with %d element(s): %s\n", green("✓"), title, total, boardURL(id))
				return err
			})
		},
	}
	cmd.Flags().StringVar(&parent, "parent", "", "Board to put the new board in (default: your Home)")
	cmd.Flags().StringVar(&name, "name", "", "Title of the new board (default: the kit's title)")
	cmd.Annotations["pp:verified-by-proof"] = "milanote-lifecycle.md"
	return cmd
}

func countKit(items []kitItem) int {
	n := len(items)
	for _, it := range items {
		n += countKit(it.Items)
	}
	return n
}

func kitContent(it kitItem) (string, map[string]any, error) {
	var elType string
	var content map[string]any
	switch it.Type {
	case "heading":
		align := "left"
		if it.Level == 1 {
			align = "center"
		}
		elType, content = milanote.TypeCard, milanote.HeadingContent(it.Level, it.Text, align)
	case "note":
		elType, content = milanote.TypeCard, milanote.NoteContent(milanote.MarkdownDoc(it.Text), "")
	case "todo":
		tasks := make([]milanote.Task, len(it.Tasks))
		for i, t := range it.Tasks {
			tasks[i] = milanote.Task{Text: t.Text, Checked: t.Done}
		}
		if len(tasks) == 0 {
			tasks = []milanote.Task{{}}
		}
		elType, content = milanote.TypeTaskList, milanote.TaskListContent(it.Title, tasks)
	case "column":
		elType, content = milanote.TypeColumn, map[string]any{"title": it.Title}
	case "board":
		elType, content = milanote.TypeBoard, milanote.BoardContent(it.Title)
	case "link":
		elType, content = milanote.TypeLink, map[string]any{"url": it.URL}
	default:
		return "", nil, fmt.Errorf("unknown item type %q in kit", it.Type)
	}
	if it.Width > 0 {
		content["width"] = it.Width
	}
	if it.Color != "" && it.Type != "board" {
		if v, err := milanote.ColorValue(it.Color); err == nil && v != nil {
			content["color"] = v
		}
	}
	return elType, content, nil
}

// applyKitItems creates items on board channel. parent is where they sit:
// the board itself or, for a column's cards, the column.
func applyKitItems(ctx context.Context, w *milanote.Writer, channel, parent string, items []kitItem) error {
	inColumn := parent != channel
	score := 0.0
	unsorted := 0
	for _, it := range items {
		elType, content, err := kitContent(it)
		if err != nil {
			return err
		}
		score += milanote.ScoreStep
		var loc milanote.Location
		switch {
		case inColumn || it.Unsorted:
			loc = milanote.ListAt(parent, unsorted, score)
			unsorted++
		default:
			loc = milanote.CanvasAt(parent, it.X, it.Y, score)
		}
		id, err := w.Create(ctx, channel, elType, loc, content, "")
		if err != nil {
			return err
		}
		switch it.Type {
		case "column":
			if err := applyKitItems(ctx, w, channel, id, it.Items); err != nil {
				return err
			}
		case "board":
			if err := applyKitItems(ctx, w, id, id, it.Items); err != nil {
				return err
			}
		}
	}
	return nil
}
