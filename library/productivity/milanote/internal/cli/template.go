// Copyright 2026 Beau Sterling and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"io"
	"math"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/mvanhorn/printing-press-library/library/productivity/milanote/internal/milanote"
)

// kit is a board structure saved as YAML.
type kit struct {
	Version int       `yaml:"version"`
	Title   string    `yaml:"title"`
	Items   []kitItem `yaml:"items"`
}

// kitItem is one card, column or sub-board in a kit.
type kitItem struct {
	Type     string    `yaml:"type"` // heading, note, todo, column, board, link
	X        float64   `yaml:"x"`
	Y        float64   `yaml:"y"`
	Unsorted bool      `yaml:"unsorted,omitempty"`
	Width    float64   `yaml:"width,omitempty"`
	Color    string    `yaml:"color,omitempty"`
	Level    int       `yaml:"level,omitempty"`
	Title    string    `yaml:"title,omitempty"`
	Text     string    `yaml:"text,omitempty"`
	URL      string    `yaml:"url,omitempty"`
	Tasks    []kitTask `yaml:"tasks,omitempty"`
	Items    []kitItem `yaml:"items,omitempty"`
}

type kitTask struct {
	Text string `yaml:"text"`
	Done bool   `yaml:"done,omitempty"`
}

func newNovelTemplateCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "template",
		Short:       "Save a board's structure as a YAML kit and stamp out fresh copies",
		Example:     "  milanote-pp-cli template apply film-kit.yaml --parent 1Ab2Cd3EfGh4iJ --name 'New short' --dry-run",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:parent-group": "true", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelTemplateApplyCmd(flags))
	addNovelCommandIfAbsent(cmd, newTemplateSaveCmd(flags))
	return cmd
}

func newTemplateSaveCmd(flags *rootFlags) *cobra.Command {
	var out string
	var noContent bool
	cmd := &cobra.Command{
		Use:   "save <board>",
		Short: "Save a board's headings, columns, notes, to-dos and sub-boards as a YAML kit",
		Long: `Saves relative positions and structure. With --no-content, headings and
column titles stay but note text and to-do items are left blank, so the kit is
just the skeleton. Images and other uploaded files are not copied into a kit;
use 'boards duplicate' for an exact copy.`,
		Example:     "  milanote-pp-cli template save 1Ab2Cd3EfGh4iJ --out film-kit.yaml --no-content",
		Annotations: readAnnotations("live"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "template save")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := mnClient(flags)
			if err != nil {
				return err
			}
			root := boardRef(args[0])
			boards := map[string]*milanote.TreeBoard{}
			if err := c.Walk(ctx, root, -1, func(b *milanote.TreeBoard) error {
				boards[b.ID] = b
				return nil
			}); err != nil {
				return mnErr(err)
			}
			if boards[root] == nil {
				return notFoundErr(fmt.Errorf("board %s not found", root))
			}
			skipped := 0
			k := kit{Version: 1, Title: boards[root].Title, Items: kitItems(boards, root, noContent, &skipped)}
			data, err := yaml.Marshal(k)
			if err != nil {
				return err
			}
			if out == "" {
				_, err := cmd.OutOrStdout().Write(data)
				return err
			}
			if err := os.WriteFile(out, data, 0o600); err != nil {
				return usageErr(err)
			}
			return emit(cmd, flags, map[string]any{"out": out, "boards": len(boards), "skipped_uploads": skipped}, func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "%s saved %d board(s) to %s (%d uploaded file(s) left out)\n", green("✓"), len(boards), out, skipped)
				return err
			})
		},
	}
	cmd.Flags().StringVar(&out, "out", "", "File to write (default: print the YAML)")
	cmd.Flags().BoolVar(&noContent, "no-content", false, "Keep the structure but blank the note text and to-do items")
	return cmd
}

func kitItems(boards map[string]*milanote.TreeBoard, id string, noContent bool, skipped *int) []kitItem {
	tb := boards[id]
	if tb == nil {
		return nil
	}
	p := tb.Payload
	canvas := p.Children(id, milanote.SectionCanvas)
	minX, minY := math.Inf(1), math.Inf(1)
	for _, el := range canvas {
		minX, minY = math.Min(minX, el.Location.X()), math.Min(minY, el.Location.Y())
	}
	items := []kitItem{}
	for _, el := range canvas {
		if it, ok := kitItemOf(boards, p, el, noContent, skipped); ok {
			it.X, it.Y = el.Location.X()-minX+2, el.Location.Y()-minY+2
			items = append(items, it)
		}
	}
	for _, el := range p.Children(id, milanote.SectionInbox) {
		if it, ok := kitItemOf(boards, p, el, noContent, skipped); ok {
			it.Unsorted = true
			items = append(items, it)
		}
	}
	return items
}

func kitItemOf(boards map[string]*milanote.TreeBoard, p *milanote.BoardPayload, el *milanote.Element, noContent bool, skipped *int) (kitItem, bool) {
	it := kitItem{Width: el.Width(), Color: el.Color()}
	switch el.ElementType {
	case milanote.TypeBoard:
		it.Type, it.Title, it.Width = "board", el.Title(), 0
		it.Items = kitItems(boards, el.ID, noContent, skipped)
	case milanote.TypeColumn:
		it.Type, it.Title = "column", el.Title()
		for _, kid := range p.Children(el.ID, milanote.SectionInbox) {
			if child, ok := kitItemOf(boards, p, kid, noContent, skipped); ok {
				it.Items = append(it.Items, child)
			}
		}
	case milanote.TypeTaskList:
		it.Type, it.Title = "todo", el.Title()
		if !noContent {
			for _, t := range el.Tasks() {
				it.Tasks = append(it.Tasks, kitTask{Text: t.Text, Done: t.Checked})
			}
		}
	case milanote.TypeLink:
		it.Type = "link"
		it.URL, _ = el.Content["url"].(string)
	case milanote.TypeCard:
		md := milanote.ElementMarkdown(el)
		if kind := milanote.ChunkKindOf(el); kind == milanote.ChunkHeading {
			chunks := milanote.ChunkMarkdown(md)
			it.Type = "heading"
			if len(chunks) > 0 {
				it.Level, it.Text = chunks[0].Level, chunks[0].Text
			}
			return it, true
		}
		it.Type = "note"
		if !noContent {
			it.Text = md
		}
	default:
		if el.ElementType == milanote.TypeImage || el.ElementType == milanote.TypeFile {
			*skipped++
		}
		return it, false
	}
	return it, true
}
