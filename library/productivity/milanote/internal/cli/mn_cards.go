// Copyright 2026 Beau Sterling and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/milanote/internal/milanote"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		notes := mnGroup(root, flags, "notes", "Add and edit rich-text notes")
		addNovelCommandIfAbsent(notes, newNotesAddCmd(flags))
		addNovelCommandIfAbsent(notes, newNotesEditCmd(flags))
		headings := mnGroup(root, flags, "headings", "Add native heading cards")
		addNovelCommandIfAbsent(headings, newHeadingsAddCmd(flags))
		links := mnGroup(root, flags, "links", "Add link cards with previews")
		addNovelCommandIfAbsent(links, newLinksAddCmd(flags))
		columns := mnGroup(root, flags, "columns", "Add columns and fill them with cards")
		addNovelCommandIfAbsent(columns, newColumnsAddCmd(flags))
		todos := mnGroup(root, flags, "todos", "Add, tick and list to-do lists")
		addNovelCommandIfAbsent(todos, newTodosAddCmd(flags))
		addNovelCommandIfAbsent(todos, newTodosCheckCmd(flags))
	})
}

// mnGroup returns the named top-level command group, creating it if needed.
func mnGroup(root *cobra.Command, flags *rootFlags, name, short string) *cobra.Command {
	for _, c := range root.Commands() {
		if c.Name() == name {
			return c
		}
	}
	g := &cobra.Command{
		Use:         name,
		Short:       short,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:parent-group": "true", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	root.AddCommand(g)
	return g
}

func writeAnnotations() map[string]string {
	return map[string]string{"mcp:read-only": "false", "pp:data-source": "live"}
}

func readAnnotations(source string) map[string]string {
	return map[string]string{"mcp:read-only": "true", "pp:data-source": source}
}

func newNotesAddCmd(flags *rootFlags) *cobra.Command {
	var place placeFlags
	var color string
	cmd := &cobra.Command{
		Use:   "add <board> <markdown|-|@file>",
		Short: "Add a note written in Markdown (headings, lists, bold, links, callouts)",
		Long: `Add a note card. The text is Markdown: # and ## become Milanote headings,
- and 1. become real bullet and numbered lists, **bold**, *italic*, ~~strike~~,
` + "`code`" + ` and [links](url) become rich text, and "> [!NOTE] text" or
"> ⚠️ text" becomes a callout. Pass - to read stdin or @file.md to read a file.

Without placement flags the note goes below everything already on the board,
lined up with the leftmost card.`,
		Example: `  milanote-pp-cli notes add 1Ab2Cd3EfGh4iJ "## Scene 4\n- wide shot\n- **close-up** on hands"
  milanote-pp-cli notes add 1Ab2Cd3EfGh4iJ @beats.md --x 40 --y 10 --width 40
  milanote-pp-cli notes add 1Ab2Cd3EfGh4iJ "Remember the props" --after 1Ab2Cd3EfGh4iK --color yellow`,
		Annotations: writeAnnotations(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "notes add")
			}
			if len(args) < 2 {
				return needArg(cmd, "note text")
			}
			if err := place.validate(); err != nil {
				return err
			}
			text, err := readTextArg(cmd, args[1])
			if err != nil {
				return err
			}
			colorVal, err := milanote.ColorValue(color)
			if err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := mnClient(flags)
			if err != nil {
				return err
			}
			boardID := boardRef(args[0])
			payload, err := loadBoard(ctx, c, boardID)
			if err != nil {
				return err
			}
			loc, err := place.resolve(payload, boardID)
			if err != nil {
				return err
			}
			content := place.withWidth(milanote.NoteContent(milanote.MarkdownDoc(text), ""))
			if colorVal != nil {
				content["color"] = colorVal
			}
			w, err := c.Writer(ctx)
			if err != nil {
				return mnErr(err)
			}
			defer w.Close(ctx)
			id, err := w.Create(ctx, boardID, milanote.TypeCard, loc, content, "")
			if err != nil {
				return mnErr(err)
			}
			return printCreated(cmd, flags, createdRow(id, milanote.TypeCard, boardID, loc, content))
		},
	}
	place.bind(cmd)
	cmd.Flags().StringVar(&color, "color", "", "Card colour: "+strings.Join(milanote.ColorNames, ", "))
	return cmd
}

func newNotesEditCmd(flags *rootFlags) *cobra.Command {
	var appendText bool
	cmd := &cobra.Command{
		Use:   "edit <board> <note-id> <markdown|-|@file>",
		Short: "Replace (or --append to) a note's text, written in Markdown",
		Example: `  milanote-pp-cli notes edit 1Ab2Cd3EfGh4iJ 1Ab2Cd3EfGh4iK "## Scene 4 (revised)\n- wide shot"
  milanote-pp-cli notes edit 1Ab2Cd3EfGh4iJ 1Ab2Cd3EfGh4iK "- extra beat" --append`,
		Annotations: writeAnnotations(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "notes edit")
			}
			if len(args) < 3 {
				return needArg(cmd, "note id and text")
			}
			text, err := readTextArg(cmd, args[2])
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := mnClient(flags)
			if err != nil {
				return err
			}
			boardID := boardRef(args[0])
			payload, err := loadBoard(ctx, c, boardID)
			if err != nil {
				return err
			}
			el, err := milanote.Find(payload, args[1])
			if err != nil {
				return notFoundErr(fmt.Errorf("note %s: %w", args[1], err))
			}
			if el.ElementType != milanote.TypeCard {
				return usageErr(fmt.Errorf("%s is a %s; notes edit only changes notes", el.ID, strings.ToLower(el.ElementType)))
			}
			doc := milanote.MarkdownDoc(text)
			if appendText {
				if old, ok := el.Content["textContent"].(map[string]any); ok {
					merged := append(asAnyList(old["content"]), asAnyList(doc["content"])...)
					doc = map[string]any{"type": "doc", "content": merged}
				}
			}
			w, err := c.Writer(ctx)
			if err != nil {
				return mnErr(err)
			}
			defer w.Close(ctx)
			if err := w.Update(ctx, milanote.BoardOf(payload, el.ID), milanote.Change{ID: el.ID, Changes: map[string]any{"textContent": doc}}); err != nil {
				return mnErr(err)
			}
			el.Content["textContent"] = doc
			return emit(cmd, flags, rowOf(el, boardID), func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "%s note %s updated\n", green("✓"), el.ID)
				return err
			})
		},
	}
	cmd.Flags().BoolVar(&appendText, "append", false, "Add the text after the existing note text instead of replacing it")
	return cmd
}

func asAnyList(v any) []any {
	l, _ := v.([]any)
	return l
}

func newHeadingsAddCmd(flags *rootFlags) *cobra.Command {
	var place placeFlags
	var level int
	var align string
	cmd := &cobra.Command{
		Use:   "add <board> <text>",
		Short: "Add a native heading card (transparent, level 1 or 2)",
		Example: `  milanote-pp-cli headings add 1Ab2Cd3EfGh4iJ "Act One" --x 2 --y 2
  milanote-pp-cli headings add 1Ab2Cd3EfGh4iJ "Characters" --level 2 --align left`,
		Annotations: writeAnnotations(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "headings add")
			}
			if len(args) < 2 {
				return needArg(cmd, "heading text")
			}
			if level != 1 && level != 2 {
				return usageErr(errors.New("--level must be 1 or 2"))
			}
			switch align {
			case "left", "center", "right":
			default:
				return usageErr(fmt.Errorf("--align must be left, center or right (got %q)", align))
			}
			if err := place.validate(); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := mnClient(flags)
			if err != nil {
				return err
			}
			boardID := boardRef(args[0])
			payload, err := loadBoard(ctx, c, boardID)
			if err != nil {
				return err
			}
			loc, err := place.resolve(payload, boardID)
			if err != nil {
				return err
			}
			content := place.withWidth(milanote.HeadingContent(level, args[1], align))
			w, err := c.Writer(ctx)
			if err != nil {
				return mnErr(err)
			}
			defer w.Close(ctx)
			id, err := w.Create(ctx, boardID, milanote.TypeCard, loc, content, "")
			if err != nil {
				return mnErr(err)
			}
			return printCreated(cmd, flags, createdRow(id, milanote.TypeCard, boardID, loc, content))
		},
	}
	place.bind(cmd)
	cmd.Flags().IntVar(&level, "level", 1, "Heading size: 1 (large) or 2 (medium)")
	cmd.Flags().StringVar(&align, "align", "center", "Text alignment: left, center or right")
	return cmd
}

func newLinksAddCmd(flags *rootFlags) *cobra.Command {
	var place placeFlags
	cmd := &cobra.Command{
		Use:         "add <board> <url>",
		Short:       "Add a link card; Milanote fetches the page title, description and image",
		Example:     "  milanote-pp-cli links add 1Ab2Cd3EfGh4iJ https://example.com/reference --after 1Ab2Cd3EfGh4iK",
		Annotations: writeAnnotations(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "links add")
			}
			if len(args) < 2 {
				return needArg(cmd, "url")
			}
			link := strings.TrimSpace(args[1])
			if !strings.HasPrefix(link, "http://") && !strings.HasPrefix(link, "https://") {
				return usageErr(fmt.Errorf("url must start with http:// or https:// (got %q)", link))
			}
			if err := place.validate(); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := mnClient(flags)
			if err != nil {
				return err
			}
			boardID := boardRef(args[0])
			payload, err := loadBoard(ctx, c, boardID)
			if err != nil {
				return err
			}
			loc, err := place.resolve(payload, boardID)
			if err != nil {
				return err
			}
			w, err := c.Writer(ctx)
			if err != nil {
				return mnErr(err)
			}
			defer w.Close(ctx)
			id := w.NewID()
			var preview map[string]any
			body := map[string]any{"url": link, "elementId": id, "environmentFolder": "p", "userId": w.User().ID, "locale": "en-au"}
			if err := c.PostJSON(ctx, "POST", "/media-service/api/link", body, &preview); err != nil {
				return mnErr(fmt.Errorf("Milanote could not read %s: %w", link, err))
			}
			if err := w.CreateWithID(ctx, id, boardID, milanote.TypeLink, loc, place.withWidth(map[string]any{"url": nil}), ""); err != nil {
				return mnErr(err)
			}
			page, _ := preview["link"].(map[string]any)
			if page == nil {
				page = map[string]any{"url": link}
			}
			changes := map[string]any{"image": orEmpty(preview["image"]), "mediaType": orString(preview["mediaType"], "NO_MEDIA"),
				"link": page, "provider": orEmpty(preview["provider"])}
			if d, _ := preview["description"].(string); d != "" {
				changes["caption"] = milanote.Doc(milanote.Paragraph(d))
				changes["showCaption"] = true
			}
			elementType := orString(preview["elementType"], milanote.TypeLink)
			if err := w.SetType(ctx, boardID, id, elementType, changes); err != nil {
				return mnErr(err)
			}
			title, _ := page["title"].(string)
			row := createdRow(id, elementType, boardID, loc, map[string]any{"title": title, "url": link})
			return printCreated(cmd, flags, row)
		},
	}
	place.bind(cmd)
	return cmd
}

func orEmpty(v any) any {
	if v == nil {
		return map[string]any{}
	}
	return v
}

func orString(v any, def string) string {
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return def
}

func newColumnsAddCmd(flags *rootFlags) *cobra.Command {
	var place placeFlags
	var cards string
	cmd := &cobra.Command{
		Use:   "add <board> <title>",
		Short: "Add a column, optionally moving existing cards into it in order",
		Example: `  milanote-pp-cli columns add 1Ab2Cd3EfGh4iJ "Shot list" --x 2 --y 30
  milanote-pp-cli columns add 1Ab2Cd3EfGh4iJ "Done" --cards 1Ab2Cd3EfGh4iK,1Ab2Cd3EfGh4iL`,
		Annotations: writeAnnotations(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "columns add")
			}
			if len(args) < 2 {
				return needArg(cmd, "column title")
			}
			if err := place.validate(); err != nil {
				return err
			}
			if place.column != "" {
				return usageErr(errors.New("a column cannot go inside another column"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := mnClient(flags)
			if err != nil {
				return err
			}
			boardID := boardRef(args[0])
			payload, err := loadBoard(ctx, c, boardID)
			if err != nil {
				return err
			}
			var members []*milanote.Element
			for _, ref := range splitList(cards) {
				el, err := milanote.Find(payload, ref)
				if err != nil {
					return notFoundErr(fmt.Errorf("card %s: %w", ref, err))
				}
				if el.ElementType == milanote.TypeColumn {
					return usageErr(fmt.Errorf("%s is a column; columns cannot nest", el.ID))
				}
				members = append(members, el)
			}
			loc, err := place.resolve(payload, boardID)
			if err != nil {
				return err
			}
			w, err := c.Writer(ctx)
			if err != nil {
				return mnErr(err)
			}
			defer w.Close(ctx)
			content := place.withWidth(map[string]any{"title": args[1]})
			id, err := w.Create(ctx, boardID, milanote.TypeColumn, loc, content, "")
			if err != nil {
				return mnErr(err)
			}
			moves := make([]milanote.Move, len(members))
			for i, el := range members {
				moves[i] = milanote.Move{ID: el.ID, From: el.Location, To: milanote.ListAt(id, i, float64(i+1)*milanote.ScoreStep)}
			}
			if err := w.MoveMany(ctx, "DROP", moves); err != nil {
				return mnErr(fmt.Errorf("column %s created but moving cards failed: %w", id, err))
			}
			row := createdRow(id, milanote.TypeColumn, boardID, loc, content)
			return emit(cmd, flags, map[string]any{"column": row, "moved": len(moves)}, func(w io.Writer) error {
				if err := printCreated(cmd, flags, row); err != nil {
					return err
				}
				if len(moves) > 0 {
					_, err = fmt.Fprintf(w, "  moved %d card(s) into it\n", len(moves))
				}
				return err
			})
		},
	}
	place.bind(cmd)
	cmd.Flags().StringVar(&cards, "cards", "", "Comma-separated element ids to move into the column, top to bottom")
	return cmd
}

func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// parseTaskLines reads "- [ ] item" / "- [x] item" / plain lines as tasks.
func parseTaskLines(lines []string) []milanote.Task {
	var out []milanote.Task
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		t := milanote.Task{Text: line}
		for _, prefix := range []string{"- ", "* ", "+ "} {
			t.Text = strings.TrimPrefix(t.Text, prefix)
		}
		switch {
		case strings.HasPrefix(t.Text, "[ ] "):
			t.Text = t.Text[4:]
		case strings.HasPrefix(t.Text, "[x] "), strings.HasPrefix(t.Text, "[X] "):
			t.Text, t.Checked = t.Text[4:], true
		}
		out = append(out, t)
	}
	return out
}

func newTodosAddCmd(flags *rootFlags) *cobra.Command {
	var place placeFlags
	var title, file string
	cmd := &cobra.Command{
		Use:   "add <board> [item...]",
		Short: "Add a native to-do list (real checkboxes) from items or '- [ ]' lines",
		Example: `  milanote-pp-cli todos add 1Ab2Cd3EfGh4iJ "Book location" "Cast extras" "[x] Write script"
  milanote-pp-cli todos add 1Ab2Cd3EfGh4iJ --file prep.md --title "Prep" --after 1Ab2Cd3EfGh4iK`,
		Annotations: writeAnnotations(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "todos add")
			}
			lines := args[1:]
			if file != "" {
				data, err := readTextArg(cmd, "@"+file)
				if file == "-" {
					data, err = readTextArg(cmd, "-")
				}
				if err != nil {
					return err
				}
				lines = append(lines, strings.Split(data, "\n")...)
			}
			tasks := parseTaskLines(lines)
			if len(tasks) == 0 {
				return needArg(cmd, "at least one to-do item (arguments or --file)")
			}
			if err := place.validate(); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := mnClient(flags)
			if err != nil {
				return err
			}
			boardID := boardRef(args[0])
			payload, err := loadBoard(ctx, c, boardID)
			if err != nil {
				return err
			}
			loc, err := place.resolve(payload, boardID)
			if err != nil {
				return err
			}
			content := place.withWidth(milanote.TaskListContent(title, tasks))
			w, err := c.Writer(ctx)
			if err != nil {
				return mnErr(err)
			}
			defer w.Close(ctx)
			id, err := w.Create(ctx, boardID, milanote.TypeTaskList, loc, content, "")
			if err != nil {
				return mnErr(err)
			}
			return printCreated(cmd, flags, createdRow(id, milanote.TypeTaskList, boardID, loc, content))
		},
	}
	place.bind(cmd)
	cmd.Flags().StringVar(&title, "title", "", "Title shown above the list")
	cmd.Flags().StringVar(&file, "file", "", "Read items from a file ('- [ ] item' lines or plain lines; - for stdin)")
	return cmd
}

func newTodosCheckCmd(flags *rootFlags) *cobra.Command {
	var uncheck, all bool
	var indexes string
	cmd := &cobra.Command{
		Use:   "check <board> <todo-list-id> [text...]",
		Short: "Tick (or --uncheck) to-do items by text match, --index or --all",
		Example: `  milanote-pp-cli todos check 1Ab2Cd3EfGh4iJ 1Ab2Cd3EfGh4iK "book location"
  milanote-pp-cli todos check 1Ab2Cd3EfGh4iJ 1Ab2Cd3EfGh4iK --index 1,3
  milanote-pp-cli todos check 1Ab2Cd3EfGh4iJ 1Ab2Cd3EfGh4iK --all --uncheck`,
		Annotations: writeAnnotations(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "todos check")
			}
			if len(args) < 2 {
				return needArg(cmd, "to-do list id")
			}
			matches := args[2:]
			idx := map[int]bool{}
			for _, s := range splitList(indexes) {
				n, err := strconv.Atoi(s)
				if err != nil || n < 1 {
					return usageErr(fmt.Errorf("--index takes 1-based numbers, got %q", s))
				}
				idx[n-1] = true
			}
			if !all && len(matches) == 0 && len(idx) == 0 {
				return usageErr(errors.New("say which items: give text to match, --index, or --all"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := mnClient(flags)
			if err != nil {
				return err
			}
			boardID := boardRef(args[0])
			payload, err := loadBoard(ctx, c, boardID)
			if err != nil {
				return err
			}
			el, err := milanote.Find(payload, args[1])
			if err != nil {
				return notFoundErr(fmt.Errorf("to-do list %s: %w", args[1], err))
			}
			if el.ElementType != milanote.TypeTaskList {
				return usageErr(fmt.Errorf("%s is a %s, not a to-do list", el.ID, strings.ToLower(el.ElementType)))
			}
			doc, _ := el.Content["textContent"].(map[string]any)
			matched := 0
			newDoc, changed := milanote.SetTaskChecked(doc, !uncheck, func(i int, t milanote.Task) bool {
				hit := all || idx[i]
				for _, m := range matches {
					if strings.Contains(strings.ToLower(t.Text), strings.ToLower(m)) || t.ID == m {
						hit = true
					}
				}
				if hit {
					matched++
				}
				return hit
			})
			if matched == 0 {
				return notFoundErr(errors.New("no to-do item matched"))
			}
			if changed > 0 {
				w, err := c.Writer(ctx)
				if err != nil {
					return mnErr(err)
				}
				defer w.Close(ctx)
				if err := w.Update(ctx, milanote.BoardOf(payload, el.ID), milanote.Change{ID: el.ID, Changes: map[string]any{"textContent": newDoc}}); err != nil {
					return mnErr(err)
				}
			}
			el.Content["textContent"] = newDoc
			result := map[string]any{"id": el.ID, "matched": matched, "changed": changed, "checked": !uncheck, "tasks": el.Tasks()}
			return emit(cmd, flags, result, func(w io.Writer) error {
				verb := "ticked"
				if uncheck {
					verb = "unticked"
				}
				fmt.Fprintf(w, "%s %s %d item(s) (%d already were)\n", green("✓"), verb, changed, matched-changed)
				for _, t := range el.Tasks() {
					box := "[ ]"
					if t.Checked {
						box = "[x]"
					}
					fmt.Fprintf(w, "  %s %s\n", box, t.Text)
				}
				return nil
			})
		},
	}
	cmd.Flags().BoolVar(&uncheck, "uncheck", false, "Untick instead of tick")
	cmd.Flags().BoolVar(&all, "all", false, "Apply to every item in the list")
	cmd.Flags().StringVar(&indexes, "index", "", "Comma-separated 1-based item numbers")
	return cmd
}

// readLocalFile reads a file the user named, capped at 64MB.
func readLocalFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(bufio.NewReader(f), 64<<20))
}
