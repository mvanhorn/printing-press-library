// Copyright 2026 Beau Sterling and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/milanote/internal/milanote"
)

type openTask struct {
	Board   string `json:"board_id"`
	Path    string `json:"path"`
	ListID  string `json:"list_id"`
	TaskID  string `json:"task_id"`
	Text    string `json:"text"`
	Command string `json:"tick_command"`
}

func newNovelTodosOpenCmd(flags *rootFlags) *cobra.Command {
	var board string
	var recursive bool

	cmd := &cobra.Command{
		Use:   "open",
		Short: "List every unchecked checkbox across your boards with where it lives, ready to tick off.",
		Example: `  milanote-pp-cli todos open --agent
  milanote-pp-cli todos open --board 1Ab2Cd3EfGh4iJ --recursive=false`,
		Annotations: readAnnotations("live"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "todos open")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := mnClient(flags)
			if err != nil {
				return err
			}
			root := boardRef(board)
			if root == "" {
				me, err := c.Me(ctx)
				if err != nil {
					return mnErr(err)
				}
				root = me.RootBoardID
			}
			depth := -1
			if !recursive {
				depth = 0
			}
			out := []openTask{}
			err = c.Walk(ctx, root, depth, func(b *milanote.TreeBoard) error {
				var lists []*milanote.Element
				for _, el := range b.Payload.Elements {
					if el.ElementType == milanote.TypeTaskList && el.Location.Section != milanote.SectionTrash {
						lists = append(lists, el)
					}
				}
				milanote.SortReading(lists)
				for _, list := range lists {
					for _, t := range list.Tasks() {
						if t.Checked {
							continue
						}
						out = append(out, openTask{Board: b.ID, Path: b.PathString(), ListID: list.ID, TaskID: t.ID, Text: t.Text,
							Command: fmt.Sprintf("milanote-pp-cli todos check %s %s %q", b.ID, list.ID, t.Text)})
					}
				}
				return nil
			})
			if err != nil {
				return mnErr(err)
			}
			return emit(cmd, flags, out, func(w io.Writer) error {
				if len(out) == 0 {
					_, err := fmt.Fprintln(w, "No open to-dos.")
					return err
				}
				tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
				fmt.Fprintln(tw, "BOARD\tTO-DO")
				for _, t := range out {
					fmt.Fprintf(tw, "%s\t%s\n", t.Path, strings.ReplaceAll(t.Text, "\n", " "))
				}
				return tw.Flush()
			})
		},
	}
	cmd.Flags().StringVar(&board, "board", "", "Start from this board (default: your whole Home)")
	cmd.Flags().BoolVar(&recursive, "recursive", true, "Include sub-boards")
	return cmd
}
