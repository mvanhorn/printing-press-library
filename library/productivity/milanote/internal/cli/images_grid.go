// Copyright 2026 Beau Sterling and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/milanote/internal/milanote"
)

var imageExts = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true}

// naturalLess sorts frame2 before frame10.
func naturalLess(a, b string) bool {
	ra, rb := []rune(strings.ToLower(a)), []rune(strings.ToLower(b))
	i, j := 0, 0
	for i < len(ra) && j < len(rb) {
		if unicode.IsDigit(ra[i]) && unicode.IsDigit(rb[j]) {
			si := i
			for i < len(ra) && unicode.IsDigit(ra[i]) {
				i++
			}
			sj := j
			for j < len(rb) && unicode.IsDigit(rb[j]) {
				j++
			}
			na, _ := strconv.Atoi(string(ra[si:i]))
			nb, _ := strconv.Atoi(string(rb[sj:j]))
			if na != nb {
				return na < nb
			}
			continue
		}
		if ra[i] != rb[j] {
			return ra[i] < rb[j]
		}
		i++
		j++
	}
	return len(ra)-i < len(rb)-j
}

func collectImages(args []string) ([]string, error) {
	var out []string
	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			out = append(out, arg)
			continue
		}
		entries, err := os.ReadDir(arg)
		if err != nil {
			return nil, err
		}
		var names []string
		for _, e := range entries {
			if !e.IsDir() && imageExts[strings.ToLower(filepath.Ext(e.Name()))] {
				names = append(names, e.Name())
			}
		}
		sort.Slice(names, func(i, j int) bool { return naturalLess(names[i], names[j]) })
		for _, n := range names {
			out = append(out, filepath.Join(arg, n))
		}
	}
	return out, nil
}

func newNovelImagesGridCmd(flags *rootFlags) *cobra.Command {
	var board, captionFrom string
	var cols int
	var gutter, width float64
	var number bool

	cmd := &cobra.Command{
		Use:   "grid <dir|files...>",
		Short: "Upload a folder of frames as an evenly spaced, numbered grid with optional captions.",
		Long: `Uploads images in natural filename order (frame2 before frame10) and places
them left to right on an even grid. Every file is checked before the first
upload, so one bad file cannot leave half a storyboard behind.`,
		Example: `  milanote-pp-cli images grid ./frames --board 1Ab2Cd3EfGh4iJ --cols 3 --number
  milanote-pp-cli images grid a.png b.png --board 1Ab2Cd3EfGh4iJ --caption-from filename`,
		Annotations: writeAnnotations(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) && requiredInputWaived() {
				return writeDryRun(cmd.OutOrStdout(), flags, "images grid")
			}
			if board == "" {
				return usageErr(errors.New("--board is required"))
			}
			if cols < 1 || gutter < 0 || width <= 0 {
				return usageErr(errors.New("--cols and --width must be positive and --gutter must not be negative"))
			}
			if captionFrom != "" && captionFrom != "filename" && captionFrom != "none" {
				return usageErr(fmt.Errorf("--caption-from must be filename or none, not %q", captionFrom))
			}
			files, err := collectImages(args)
			if err != nil {
				return usageErr(err)
			}
			if len(files) == 0 {
				return usageErr(errors.New("no PNG, JPEG, GIF or WebP images found"))
			}
			type frame struct {
				path    string
				data    []byte
				info    milanote.ImageInfo
				caption string
				imgH    float64
				capH    float64
				capDoc  map[string]any
			}
			frames := make([]frame, len(files))
			heights := make([]float64, len(files))
			for i, path := range files {
				data, err := readLocalFile(path)
				if err != nil {
					return usageErr(err)
				}
				info, err := milanote.SniffImage(data)
				if err != nil {
					return usageErr(fmt.Errorf("%s: %w", path, err))
				}
				f := frame{path: path, data: data, info: info, imgH: milanote.ImageHeight(info.Width, info.Height, width)}
				parts := []string{}
				if number {
					parts = append(parts, fmt.Sprintf("**%02d**", i+1))
				}
				if captionFrom == "filename" {
					parts = append(parts, strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
				}
				f.caption = strings.Join(parts, " ")
				heights[i] = f.imgH
				if f.caption != "" {
					f.capDoc = milanote.MarkdownDoc(f.caption)
					f.capH = milanote.NoteHeight(f.capDoc, width)
					heights[i] += 1 + f.capH
				}
				frames[i] = f
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := mnClient(flags)
			if err != nil {
				return err
			}
			boardID := boardRef(board)
			payload, err := loadBoard(ctx, c, boardID)
			if err != nil {
				return err
			}
			x, y := milanote.FreeOrigin(milanote.CanvasBoxes(payload, boardID), gutter*2)
			slots := milanote.Grid{X: x, Y: y, Cols: cols, ColW: width, Gutter: gutter}.Rows(heights)
			rows := make([]map[string]any, len(frames))
			for i, f := range frames {
				rows[i] = map[string]any{"file": f.path, "x": slots[i].X, "y": slots[i].Y, "width": f.info.Width, "height": f.info.Height, "caption": f.caption}
			}
			if !dryRunOK(flags) {
				w, err := c.Writer(ctx)
				if err != nil {
					return mnErr(err)
				}
				defer w.Close(ctx)
				score := milanote.MaxScore(payload, boardID)
				for i, f := range frames {
					score += milanote.ScoreStep
					fmt.Fprintf(cmd.ErrOrStderr(), "uploading %d/%d %s\n", i+1, len(frames), filepath.Base(f.path))
					id, _, err := c.AddImage(ctx, w, boardID, milanote.CanvasAt(boardID, slots[i].X, slots[i].Y, score), width, f.path, f.data)
					if err != nil {
						return mnErr(fmt.Errorf("%s (frame %d of %d): %w", f.path, i+1, len(frames), err))
					}
					rows[i]["id"] = id
					if f.capDoc != nil {
						score += milanote.ScoreStep
						content := milanote.NoteContent(f.capDoc, "")
						if width != milanote.CardWidth {
							content["width"] = milanote.Snap(width)
						}
						capID, err := w.Create(ctx, boardID, milanote.TypeCard, milanote.CanvasAt(boardID, slots[i].X, slots[i].Y+f.imgH+1, score), content, "")
						if err != nil {
							return mnErr(fmt.Errorf("caption for %s: %w", f.path, err))
						}
						rows[i]["caption_id"] = capID
					}
				}
			}
			return emit(cmd, flags, map[string]any{"board": boardID, "board_url": boardURL(boardID), "images": rows, "dry_run": dryRunOK(flags)}, func(w io.Writer) error {
				verb := "uploaded"
				if dryRunOK(flags) {
					verb = "would upload"
				}
				_, err := fmt.Fprintf(w, "%s %s %d image(s) on a %d-column grid: %s\n", green("✓"), verb, len(frames), cols, boardURL(boardID))
				return err
			})
		},
	}
	cmd.Flags().StringVar(&board, "board", "", "Board id or URL to add the images to")
	cmd.Flags().IntVar(&cols, "cols", 3, "Number of columns")
	cmd.Flags().Float64Var(&gutter, "gutter", milanote.DefaultGutter, "Gap between cells in grid units (1 unit = 9px)")
	cmd.Flags().Float64Var(&width, "width", milanote.CardWidth, "Image width in grid units")
	cmd.Flags().BoolVar(&number, "number", false, "Add a numbered caption under each image")
	cmd.Flags().StringVar(&captionFrom, "caption-from", "none", "Caption source: filename or none")
	cmd.Annotations["pp:verified-by-proof"] = "milanote-lifecycle.md"
	return cmd
}

func newImagesAddCmd(flags *rootFlags) *cobra.Command {
	var place placeFlags
	cmd := &cobra.Command{
		Use:         "add <board> <file>",
		Short:       "Upload one image and place it on a board",
		Example:     "  milanote-pp-cli images add 1Ab2Cd3EfGh4iJ hero.png --x 40 --y 10 --width 50",
		Annotations: writeAnnotations(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "images add")
			}
			if len(args) < 2 {
				return needArg(cmd, "image file")
			}
			if err := place.validate(); err != nil {
				return err
			}
			data, err := readLocalFile(args[1])
			if err != nil {
				return usageErr(err)
			}
			if _, err := milanote.SniffImage(data); err != nil {
				return usageErr(fmt.Errorf("%s: %w", args[1], err))
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
			id, info, err := c.AddImage(ctx, w, boardID, loc, place.width, args[1], data)
			if err != nil {
				return mnErr(err)
			}
			content := map[string]any{"image": map[string]any{"width": info.Width, "height": info.Height}}
			if place.width > 0 {
				content["width"] = milanote.Snap(place.width)
			}
			return printCreated(cmd, flags, createdRow(id, milanote.TypeImage, boardID, loc, content))
		},
	}
	place.bind(cmd)
	return cmd
}
