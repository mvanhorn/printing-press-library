// Copyright 2026 Beau Sterling and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/milanote/internal/milanote"
)

func newNovelOutlineCmd(flags *rootFlags) *cobra.Command {
	var depth int
	var out string

	cmd := &cobra.Command{
		Use:         "outline <board>",
		Short:       "Get a board and its sub-boards back as Markdown in canvas reading order, with headings, lists and checkboxes intact.",
		Example:     "  milanote-pp-cli outline 1Ab2Cd3EfGh4iJ --depth 2 --out board.md",
		Annotations: readAnnotations("live"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "outline")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := mnClient(flags)
			if err != nil {
				return err
			}
			boards := map[string]*milanote.TreeBoard{}
			rootID := boardRef(args[0])
			if err := c.Walk(ctx, rootID, depth, func(b *milanote.TreeBoard) error {
				boards[b.ID] = b
				return nil
			}); err != nil {
				return mnErr(err)
			}
			if boards[rootID] == nil {
				return notFoundErr(fmt.Errorf("board %s not found", rootID))
			}
			var b strings.Builder
			renderBoard(&b, boards, rootID, 1)
			md := strings.TrimSpace(b.String()) + "\n"
			if out != "" && !dryRunOK(flags) {
				if err := os.WriteFile(out, []byte(md), 0o644); err != nil {
					return usageErr(err)
				}
			}
			result := map[string]any{"board": rootID, "boards": len(boards), "markdown": md}
			if out != "" {
				result = map[string]any{"board": rootID, "boards": len(boards), "out": out, "bytes": len(md)}
			}
			return emit(cmd, flags, result, func(w io.Writer) error {
				if out != "" {
					_, err := fmt.Fprintf(w, "%s wrote %d board(s) to %s\n", green("✓"), len(boards), out)
					return err
				}
				_, err := io.WriteString(w, md)
				return err
			})
		},
	}
	cmd.Flags().IntVar(&depth, "depth", -1, "How many levels of sub-boards to include (-1 for all, 0 for this board only)")
	cmd.Flags().StringVar(&out, "out", "", "Write to this file instead of the terminal")
	cmd.Annotations["pp:verified-by-proof"] = "milanote-lifecycle.md"
	return cmd
}

var mdHeadingLine = regexp.MustCompile(`^(#{1,6})\s`)

// shiftHeadings pushes a card's headings under the board title's level.
func shiftHeadings(md string, by int) string {
	lines := strings.Split(md, "\n")
	inFence := false
	for i, line := range lines {
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if m := mdHeadingLine.FindStringSubmatch(line); m != nil {
			level := len(m[1]) + by
			if level > 6 {
				level = 6
			}
			lines[i] = strings.Repeat("#", level) + line[len(m[1]):]
		}
	}
	return strings.Join(lines, "\n")
}

func renderBoard(b *strings.Builder, boards map[string]*milanote.TreeBoard, id string, level int) {
	tb := boards[id]
	if tb == nil {
		return
	}
	lvl := level
	if lvl > 6 {
		lvl = 6
	}
	fmt.Fprintf(b, "%s %s\n\n", strings.Repeat("#", lvl), strings.TrimSpace(tb.Title))
	p := tb.Payload
	for _, el := range p.Children(id, milanote.SectionCanvas) {
		renderElement(b, boards, p, el, level)
	}
	unsorted := p.Children(id, milanote.SectionInbox)
	if len(unsorted) > 0 {
		for _, el := range unsorted {
			renderElement(b, boards, p, el, level)
		}
	}
}

func renderElement(b *strings.Builder, boards map[string]*milanote.TreeBoard, p *milanote.BoardPayload, el *milanote.Element, level int) {
	switch el.ElementType {
	case milanote.TypeBoard:
		if boards[el.ID] != nil {
			renderBoard(b, boards, el.ID, level+1)
			return
		}
		fmt.Fprintf(b, "- [%s](%s)\n\n", el.Title(), boardURL(el.ID))
	case milanote.TypeColumn:
		if title := strings.TrimSpace(el.Title()); title != "" {
			fmt.Fprintf(b, "**%s**\n\n", title)
		}
		for _, kid := range p.Children(el.ID, milanote.SectionInbox) {
			renderElement(b, boards, p, kid, level)
		}
	case milanote.TypeCard, milanote.TypeTaskList:
		if el.ElementType == milanote.TypeTaskList {
			if title := strings.TrimSpace(el.Title()); title != "" {
				fmt.Fprintf(b, "**%s**\n\n", title)
			}
		}
		if md := strings.TrimSpace(shiftHeadings(milanote.ElementMarkdown(el), level)); md != "" {
			b.WriteString(md + "\n\n")
		}
	case milanote.TypeLink:
		title, _ := el.Content["title"].(string)
		u, _ := el.Content["url"].(string)
		if title == "" {
			title = u
		}
		if u != "" {
			fmt.Fprintf(b, "[%s](%s)\n\n", title, u)
		}
	case milanote.TypeImage:
		img, _ := el.Content["image"].(map[string]any)
		src, _ := img["original"].(string)
		if src != "" {
			fmt.Fprintf(b, "![%s](%s)\n\n", el.Text(), src)
		}
	}
}
