// Copyright 2026 Beau Sterling and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/milanote/internal/milanote"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		boards := mnGroup(root, flags, "boards", "Boards: the canvases that hold notes, headings, images and sub-boards")
		for _, c := range []*cobra.Command{newBoardsListCmd(flags), newBoardsRecentCmd(flags), newBoardsGetCmd(flags),
			newBoardsCreateCmd(flags), newBoardsRenameCmd(flags), newBoardsDuplicateCmd(flags), newBoardsExportCmd(flags)} {
			addNovelCommandIfAbsent(boards, c)
		}
		share := mnGroup(root, flags, "share", "See and change who can open a board")
		for _, c := range []*cobra.Command{newShareGetCmd(flags), newShareInviteCmd(flags), newShareRemoveCmd(flags),
			newSharePublishCmd(flags), newShareUnpublishCmd(flags)} {
			addNovelCommandIfAbsent(share, c)
		}
		labels := mnGroup(root, flags, "labels", "List your labels")
		addNovelCommandIfAbsent(labels, newLabelsListCmd(flags))
		addNovelCommandIfAbsent(root, newUsageCmd(flags))
		addNovelCommandIfAbsent(root, newWhoamiCmd(flags))
	})
}

type boardRow struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Parent string `json:"parent_id,omitempty"`
	Depth  int    `json:"depth"`
	Path   string `json:"path"`
	URL    string `json:"board_url"`
}

// mnRun holds what nearly every Milanote command needs.
type mnRun struct {
	ctx    context.Context
	c      *milanote.Client
	cancel context.CancelFunc
}

func startRun(cmd *cobra.Command, flags *rootFlags) (*mnRun, error) {
	c, err := mnClient(flags)
	if err != nil {
		return nil, err
	}
	ctx, cancel := boundCtx(cmd.Context(), flags)
	return &mnRun{ctx: ctx, c: c, cancel: cancel}, nil
}

func (r *mnRun) writer() (*milanote.Writer, error) {
	w, err := r.c.Writer(r.ctx)
	return w, mnErr(err)
}

func newBoardsListCmd(flags *rootFlags) *cobra.Command {
	var parent string
	var depth int
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List your boards as a tree",
		Example:     "  milanote-pp-cli boards list --depth 2 --json",
		Annotations: readAnnotations("live"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "boards list")
			}
			run, err := startRun(cmd, flags)
			if err != nil {
				return err
			}
			defer run.cancel()
			root := boardRef(parent)
			if root == "" {
				me, err := run.c.Me(run.ctx)
				if err != nil {
					return mnErr(err)
				}
				root = me.RootBoardID
			}
			out := []boardRow{}
			if err := run.c.Walk(run.ctx, root, depth, func(b *milanote.TreeBoard) error {
				out = append(out, boardRow{ID: b.ID, Title: b.Title, Parent: b.ParentID, Depth: b.Depth, Path: b.PathString(), URL: boardURL(b.ID)})
				return nil
			}); err != nil {
				return mnErr(err)
			}
			out = treeOrder(out)
			return emit(cmd, flags, out, func(w io.Writer) error {
				for _, b := range out {
					if _, err := fmt.Fprintf(w, "%s%s  %s\n", strings.Repeat("  ", b.Depth), b.Title, b.ID); err != nil {
						return err
					}
				}
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&parent, "parent", "", "Start from this board (default: your Home)")
	cmd.Flags().IntVar(&depth, "depth", 1, "Levels below the start to include (-1 for all)")
	return cmd
}

// treeOrder puts each board directly after its parent. Walk visits level by
// level, which reads wrongly once the rows are indented.
func treeOrder(rows []boardRow) []boardRow {
	kids := map[string][]boardRow{}
	known := map[string]bool{}
	for _, r := range rows {
		known[r.ID] = true
		kids[r.Parent] = append(kids[r.Parent], r)
	}
	out := make([]boardRow, 0, len(rows))
	var visit func(r boardRow)
	visit = func(r boardRow) {
		out = append(out, r)
		for _, k := range kids[r.ID] {
			visit(k)
		}
	}
	for _, r := range rows {
		if !known[r.Parent] {
			visit(r)
		}
	}
	return out
}

func newBoardsRecentCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "recent",
		Short:       "Boards you viewed most recently",
		Example:     "  milanote-pp-cli boards recent --limit 10",
		Annotations: readAnnotations("live"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "boards recent")
			}
			run, err := startRun(cmd, flags)
			if err != nil {
				return err
			}
			defer run.cancel()
			entries, els, err := run.c.History(run.ctx)
			if err != nil {
				return mnErr(err)
			}
			sort.SliceStable(entries, func(i, j int) bool { return entries[i].LastView > entries[j].LastView })
			out := []map[string]any{}
			for _, e := range entries {
				if limit > 0 && len(out) >= limit {
					break
				}
				el := els[e.ElementID]
				if el == nil || el.ElementType != milanote.TypeBoard {
					continue
				}
				out = append(out, map[string]any{"id": e.ElementID, "title": el.Title(), "last_viewed": time.UnixMilli(e.LastView).UTC().Format(time.RFC3339),
					"view_count": e.ViewCount, "board_url": boardURL(e.ElementID)})
			}
			return emit(cmd, flags, out, func(w io.Writer) error {
				for _, r := range out {
					if _, err := fmt.Fprintf(w, "%v  %v\n", r["title"], r["id"]); err != nil {
						return err
					}
				}
				return nil
			})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "How many boards to show (0 for all)")
	return cmd
}

func newBoardsGetCmd(flags *rootFlags) *cobra.Command {
	var typeFilter string
	cmd := &cobra.Command{
		Use:         "get <board>",
		Short:       "Show a board's cards with type, position and text (accepts a board URL)",
		Example:     "  milanote-pp-cli boards get https://app.milanote.com/1Ab2Cd3EfGh4iJ --json --select id,type,text",
		Annotations: readAnnotations("live"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "boards get")
			}
			run, err := startRun(cmd, flags)
			if err != nil {
				return err
			}
			defer run.cancel()
			boardID := boardRef(args[0])
			payload, err := loadBoard(run.ctx, run.c, boardID)
			if err != nil {
				return err
			}
			scoped := milanote.Scope(payload, boardID)
			els := []*milanote.Element{}
			for id, el := range scoped.Elements {
				if id != boardID && el.Location.Section != milanote.SectionTrash {
					els = append(els, el)
				}
			}
			milanote.SortReading(els)
			out := []elementRow{}
			want := strings.ToLower(typeFilter)
			for _, el := range els {
				if want != "" && strings.ToLower(el.ElementType) != want {
					continue
				}
				out = append(out, rowOf(el, boardID))
			}
			return emit(cmd, flags, out, func(w io.Writer) error {
				tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
				fmt.Fprintln(tw, "ID\tTYPE\tWHERE\tTEXT")
				for _, r := range out {
					where := r.Section
					if r.X != nil {
						where = fmt.Sprintf("%g,%g", *r.X, *r.Y)
					}
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", r.ID, r.Type, where, preview(r.Text))
				}
				return tw.Flush()
			})
		},
	}
	cmd.Flags().StringVar(&typeFilter, "type", "", "Only this element type (card, task_list, image, column, board, link...)")
	return cmd
}

func newBoardsCreateCmd(flags *rootFlags) *cobra.Command {
	var parent string
	var x, y float64
	cmd := &cobra.Command{
		Use:         "create <title>",
		Short:       "Create a board, in your Home or inside another board",
		Example:     "  milanote-pp-cli boards create 'Location scout' --parent 1Ab2Cd3EfGh4iJ --x 40 --y 10",
		Annotations: writeAnnotations(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "boards create")
			}
			run, err := startRun(cmd, flags)
			if err != nil {
				return err
			}
			defer run.cancel()
			parentID := boardRef(parent)
			if parentID == "" {
				me, err := run.c.Me(run.ctx)
				if err != nil {
					return mnErr(err)
				}
				parentID = me.RootBoardID
			}
			payload, err := loadBoard(run.ctx, run.c, parentID)
			if err != nil {
				return err
			}
			loc := milanote.Unsorted(payload, parentID)
			if cmd.Flags().Changed("x") || cmd.Flags().Changed("y") {
				loc = milanote.CanvasAt(parentID, milanote.Snap(x), milanote.Snap(y), milanote.MaxScore(payload, parentID)+milanote.ScoreStep)
			}
			w, err := run.writer()
			if err != nil {
				return err
			}
			defer w.Close(run.ctx)
			content := milanote.BoardContent(args[0])
			id, err := w.Create(run.ctx, parentID, milanote.TypeBoard, loc, content, "")
			if err != nil {
				return mnErr(err)
			}
			return printCreated(cmd, flags, createdRow(id, milanote.TypeBoard, parentID, loc, content))
		},
	}
	cmd.Flags().StringVar(&parent, "parent", "", "Board to put it in (default: your Home)")
	cmd.Flags().Float64Var(&x, "x", 0, "Place the board tile on the parent's canvas at this x")
	cmd.Flags().Float64Var(&y, "y", 0, "Place the board tile on the parent's canvas at this y")
	return cmd
}

func boardElement(payload *milanote.BoardPayload, id string) (*milanote.Element, error) {
	el := payload.Elements[id]
	if el == nil {
		return nil, notFoundErr(fmt.Errorf("board %s not found", id))
	}
	return el, nil
}

func newBoardsRenameCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "rename <board> <title>",
		Short:       "Change a board's title",
		Example:     "  milanote-pp-cli boards rename 1Ab2Cd3EfGh4iJ 'Spring shoot, final'",
		Annotations: writeAnnotations(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "boards rename")
			}
			if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
				return needArg(cmd, "new title")
			}
			run, err := startRun(cmd, flags)
			if err != nil {
				return err
			}
			defer run.cancel()
			boardID := boardRef(args[0])
			payload, err := loadBoard(run.ctx, run.c, boardID)
			if err != nil {
				return err
			}
			el, err := boardElement(payload, boardID)
			if err != nil {
				return err
			}
			previous := el.Title()
			result := map[string]any{"board": boardID, "title": args[1], "previous_title": previous, "board_url": boardURL(boardID)}
			if previous != args[1] {
				w, err := run.writer()
				if err != nil {
					return err
				}
				defer w.Close(run.ctx)
				if err := w.UpdateBoard(run.ctx, el, map[string]any{"title": args[1]}); err != nil {
					return mnErr(err)
				}
			}
			return emit(cmd, flags, result, func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "%s renamed %q to %q\n", green("✓"), previous, args[1])
				return err
			})
		},
	}
	return cmd
}

func newBoardsDuplicateCmd(flags *rootFlags) *cobra.Command {
	var title, parent string
	cmd := &cobra.Command{
		Use:         "duplicate <board>",
		Short:       "Copy a board with everything on it, sub-boards included",
		Example:     "  milanote-pp-cli boards duplicate 1Ab2Cd3EfGh4iJ --title 'Spring shoot v2'",
		Annotations: writeAnnotations(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "boards duplicate")
			}
			run, err := startRun(cmd, flags)
			if err != nil {
				return err
			}
			defer run.cancel()
			boardID := boardRef(args[0])
			payload, err := loadBoard(run.ctx, run.c, boardID)
			if err != nil {
				return err
			}
			original, err := boardElement(payload, boardID)
			if err != nil {
				return err
			}
			parentID := boardRef(parent)
			if parentID == "" {
				parentID = original.Location.ParentID
			}
			if parentID == "" {
				return usageErr(errors.New("your Home cannot be duplicated"))
			}
			parentPayload, err := loadBoard(run.ctx, run.c, parentID)
			if err != nil {
				return err
			}
			content := map[string]any{}
			for _, k := range []string{"title", "icon", "color", "secondaryColor", "defaultColorPalette"} {
				content[k] = original.Content[k]
			}
			if strings.TrimSpace(title) != "" {
				content["title"] = title
			}
			loc := milanote.Unsorted(parentPayload, parentID)
			w, err := run.writer()
			if err != nil {
				return err
			}
			defer w.Close(run.ctx)
			id, err := w.Create(run.ctx, parentID, milanote.TypeBoard, loc, content, "")
			if err != nil {
				return mnErr(err)
			}
			if _, err := run.c.DuplicateBoard(run.ctx, boardID, id, loc); err != nil {
				return mnErr(fmt.Errorf("new board %s created but copying failed: %w", id, err))
			}
			return emit(cmd, flags, map[string]any{"board": id, "original": boardID, "title": content["title"], "parent": parentID, "board_url": boardURL(id)}, func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "%s copied to %s\n", green("✓"), boardURL(id))
				return err
			})
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "Title of the copy (default: same title)")
	cmd.Flags().StringVar(&parent, "parent", "", "Board to put the copy in (default: next to the original)")
	return cmd
}

func newBoardsExportCmd(flags *rootFlags) *cobra.Command {
	var format, out string
	cmd := &cobra.Command{
		Use:         "export <board>",
		Short:       "Export a board with Milanote's own exporter (pdf, png, word, markdown, text)",
		Example:     "  milanote-pp-cli boards export 1Ab2Cd3EfGh4iJ --format pdf --out board.pdf",
		Annotations: writeAnnotations(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if _, ok := milanote.ExportFormats[format]; !ok {
				return usageErr(fmt.Errorf("--format must be pdf, png, word, markdown or text, not %q", format))
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "boards export")
			}
			run, err := startRun(cmd, flags)
			if err != nil {
				return err
			}
			defer run.cancel()
			me, err := run.c.Me(run.ctx)
			if err != nil {
				return mnErr(err)
			}
			tz, _ := me.Timezone.(string)
			if tz == "" {
				tz = "UTC"
			}
			boardID := boardRef(args[0])
			link, err := run.c.ExportBoard(run.ctx, boardID, format, tz)
			if err != nil {
				return mnErr(err)
			}
			result := map[string]any{"board": boardID, "format": format, "file_url": link}
			if out != "" {
				data, err := run.c.Fetch(run.ctx, link)
				if err != nil {
					return mnErr(err)
				}
				if err := os.WriteFile(out, data, 0o600); err != nil {
					return usageErr(err)
				}
				result["out"], result["bytes"] = out, len(data)
			}
			return emit(cmd, flags, result, func(w io.Writer) error {
				if out != "" {
					_, err := fmt.Fprintf(w, "%s exported to %s\n", green("✓"), out)
					return err
				}
				_, err := fmt.Fprintln(w, link)
				return err
			})
		},
	}
	cmd.Flags().StringVar(&format, "format", "pdf", "pdf, png, word, markdown or text")
	cmd.Flags().StringVar(&out, "out", "", "Download the file here (default: print its link)")
	return cmd
}

func newShareGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "get <board>",
		Short:       "Show who can open a board and its read-only link",
		Example:     "  milanote-pp-cli share get 1Ab2Cd3EfGh4iJ --json",
		Annotations: readAnnotations("live"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "share get")
			}
			run, err := startRun(cmd, flags)
			if err != nil {
				return err
			}
			defer run.cancel()
			boardID := boardRef(args[0])
			payload, err := loadBoard(run.ctx, run.c, boardID)
			if err != nil {
				return err
			}
			el, err := boardElement(payload, boardID)
			if err != nil {
				return err
			}
			// While a read-only link is on, the ACL also holds an entry for the
			// link itself, which is not a user id the people lookup accepts.
			ids := make([]string, 0, len(el.ACL))
			for uid, bits := range el.ACL {
				if role := milanote.ACLRole(bits); role == "owner" || role == "editor" {
					ids = append(ids, uid)
				}
			}
			sort.Strings(ids)
			users, err := run.c.Users(run.ctx, ids)
			if err != nil {
				return mnErr(err)
			}
			people := []map[string]any{}
			for _, uid := range ids {
				name := ""
				if n, ok := users[uid]["name"].(map[string]any); ok {
					name, _ = n["displayName"].(string)
				}
				email, _ := users[uid]["email"].(string)
				people = append(people, map[string]any{"user_id": uid, "name": name, "email": email, "role": milanote.ACLRole(el.ACL[uid])})
			}
			link := ""
			if published, _ := el.Content["published"].(bool); published {
				if link, err = run.c.PublicLink(run.ctx, boardID); err != nil {
					return mnErr(err)
				}
			}
			result := map[string]any{"board": boardID, "people": people, "read_only_link": link}
			return emit(cmd, flags, result, func(w io.Writer) error {
				for _, p := range people {
					fmt.Fprintf(w, "%v  %v  %v\n", p["role"], p["name"], p["email"])
				}
				if link != "" {
					fmt.Fprintln(w, "read-only link:", link)
				}
				return nil
			})
		},
	}
	return cmd
}

var emailShape = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

func newShareInviteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "invite <board> <email>",
		Short:       "Invite someone to edit a board (Milanote emails them)",
		Example:     "  milanote-pp-cli share invite 1Ab2Cd3EfGh4iJ sam@example.com --dry-run",
		Annotations: writeAnnotations(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "share invite")
			}
			if len(args) < 2 || !emailShape.MatchString(args[1]) {
				return needArg(cmd, "a valid email address")
			}
			run, err := startRun(cmd, flags)
			if err != nil {
				return err
			}
			defer run.cancel()
			boardID := boardRef(args[0])
			if err := run.c.InviteEditor(run.ctx, boardID, strings.ToLower(args[1])); err != nil {
				return mnErr(err)
			}
			return emit(cmd, flags, map[string]any{"board": boardID, "invited": args[1], "role": "editor"}, func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "%s invited %s to edit %s\n", green("✓"), args[1], boardURL(boardID))
				return err
			})
		},
	}
	return cmd
}

func newShareRemoveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "remove <board> <user-id|email>",
		Short:       "Remove an editor from a board",
		Example:     "  milanote-pp-cli share remove 1Ab2Cd3EfGh4iJ sam@example.com --dry-run",
		Annotations: writeAnnotations(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "share remove")
			}
			if len(args) < 2 {
				return needArg(cmd, "user id or email")
			}
			run, err := startRun(cmd, flags)
			if err != nil {
				return err
			}
			defer run.cancel()
			boardID := boardRef(args[0])
			payload, err := loadBoard(run.ctx, run.c, boardID)
			if err != nil {
				return err
			}
			el, err := boardElement(payload, boardID)
			if err != nil {
				return err
			}
			target := args[1]
			var editors []string
			for uid, bits := range el.ACL {
				if milanote.ACLRole(bits) == "editor" {
					editors = append(editors, uid)
				}
			}
			if strings.Contains(target, "@") {
				users, err := run.c.Users(run.ctx, editors)
				if err != nil {
					return mnErr(err)
				}
				for uid, u := range users {
					if email, _ := u["email"].(string); strings.EqualFold(email, target) {
						target = uid
					}
				}
			}
			found := false
			for _, uid := range editors {
				found = found || uid == target
			}
			if !found {
				return notFoundErr(fmt.Errorf("%s is not an editor of this board; 'share get' lists them", args[1]))
			}
			if err := run.c.RemoveEditor(run.ctx, boardID, target); err != nil {
				return mnErr(err)
			}
			return emit(cmd, flags, map[string]any{"board": boardID, "removed": target}, func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "%s removed %s\n", green("✓"), args[1])
				return err
			})
		},
	}
	return cmd
}

func newSharePublishCmd(flags *rootFlags) *cobra.Command {
	var comments bool
	cmd := &cobra.Command{
		Use:         "publish <board>",
		Short:       "Turn on a read-only link anyone can open, and print it",
		Example:     "  milanote-pp-cli share publish 1Ab2Cd3EfGh4iJ --dry-run",
		Annotations: writeAnnotations(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "share publish")
			}
			run, err := startRun(cmd, flags)
			if err != nil {
				return err
			}
			defer run.cancel()
			boardID := boardRef(args[0])
			if err := run.c.Publish(run.ctx, boardID, comments); err != nil {
				return mnErr(err)
			}
			link, err := run.c.PublicLink(run.ctx, boardID)
			if err != nil {
				return mnErr(err)
			}
			if link == "" {
				return apiErr(errors.New("Milanote did not turn the read-only link on"))
			}
			return emit(cmd, flags, map[string]any{"board": boardID, "read_only_link": link, "comments": comments}, func(w io.Writer) error {
				_, err := fmt.Fprintln(w, link)
				return err
			})
		},
	}
	cmd.Flags().BoolVar(&comments, "allow-comments", false, "Let viewers of the link comment")
	return cmd
}

func newShareUnpublishCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "unpublish <board>",
		Short:       "Turn off a board's read-only link",
		Example:     "  milanote-pp-cli share unpublish 1Ab2Cd3EfGh4iJ --dry-run",
		Annotations: writeAnnotations(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "share unpublish")
			}
			run, err := startRun(cmd, flags)
			if err != nil {
				return err
			}
			defer run.cancel()
			boardID := boardRef(args[0])
			if err := run.c.Unpublish(run.ctx, boardID); err != nil {
				return mnErr(err)
			}
			return emit(cmd, flags, map[string]any{"board": boardID, "read_only_link": nil}, func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "%s read-only link is off\n", green("✓"))
				return err
			})
		},
	}
	return cmd
}

func newLabelsListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List your labels",
		Example:     "  milanote-pp-cli labels list --json",
		Annotations: readAnnotations("live"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "labels list")
			}
			run, err := startRun(cmd, flags)
			if err != nil {
				return err
			}
			defer run.cancel()
			labels, err := run.c.Labels(run.ctx)
			if err != nil {
				return mnErr(err)
			}
			if labels == nil {
				labels = []milanote.Label{}
			}
			return emit(cmd, flags, labels, func(w io.Writer) error {
				for _, l := range labels {
					if _, err := fmt.Fprintf(w, "%s  %s\n", l.Name, l.Color); err != nil {
						return err
					}
				}
				return nil
			})
		},
	}
	return cmd
}

func newUsageCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "usage",
		Short:       "Show card counts against your plan's limit",
		Example:     "  milanote-pp-cli usage --json",
		Annotations: readAnnotations("live"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "usage")
			}
			run, err := startRun(cmd, flags)
			if err != nil {
				return err
			}
			defer run.cancel()
			counts, err := run.c.Counts(run.ctx)
			if err != nil {
				return mnErr(err)
			}
			return emit(cmd, flags, counts, func(w io.Writer) error {
				keys := make([]string, 0, len(counts))
				for k := range counts {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					if _, err := fmt.Fprintf(w, "%s: %v\n", k, counts[k]); err != nil {
						return err
					}
				}
				return nil
			})
		},
	}
	return cmd
}

func newWhoamiCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "whoami",
		Short:       "Show who you are logged in as and your Home board",
		Example:     "  milanote-pp-cli whoami --json",
		Annotations: readAnnotations("live"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "whoami")
			}
			if flags.platformSession != nil {
				return flags.printJSON(cmd, platformIdentityReport(flags.platformSession))
			}
			run, err := startRun(cmd, flags)
			if err != nil {
				return err
			}
			defer run.cancel()
			me, err := run.c.Me(run.ctx)
			if err != nil {
				return mnErr(err)
			}
			result := map[string]any{"id": me.ID, "name": me.Name.DisplayName, "email": me.Email, "root_board": me.RootBoardID, "board_url": boardURL(me.RootBoardID)}
			return emit(cmd, flags, result, func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "%s (%s)\nHome: %s\n", me.Name.DisplayName, me.Email, boardURL(me.RootBoardID))
				return err
			})
		},
	}
	return cmd
}
