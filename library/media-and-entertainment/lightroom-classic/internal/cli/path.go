// Copyright 2026 Micah Baldwin and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: resolve a photo to its absolute path on disk.
package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/lightroom-classic/internal/lrcat"
)

type resolvedPath struct {
	lrcat.Photo
	// Status is always present ("ok", "missing", or "unreadable") so
	// --compact/--select field filtering can never hide an access failure.
	Status    string `json:"status"`
	Exists    bool   `json:"exists"`
	StatError string `json:"stat_error,omitempty"`
}

func newPathCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "path [id-or-filename]",
		Short: "Resolve a photo to its absolute path on disk",
		Long:  "Looks up a photo by numeric catalog id or by filename and prints its resolved absolute path,\nwith an exists-on-disk check. Filenames may match multiple photos; all matches are returned.",
		Example: strings.Trim(`
  lightroom-classic-pp-cli path 12345 --json
  lightroom-classic-pp-cli path DSC01234.ARW`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "id-or-filename=1", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would resolve the photo's path from the local catalog")
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an id or filename argument is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cat, err := openCatalog(ctx, flags)
			if err != nil {
				return err
			}
			defer cat.Close()
			photos, err := cat.PhotoByIDOrName(ctx, args[0])
			if err != nil {
				return err
			}
			out := make([]resolvedPath, 0, len(photos))
			for _, p := range photos {
				r := resolvedPath{Photo: p}
				switch _, statErr := os.Stat(p.Path); {
				case statErr == nil:
					r.Exists = true
					r.Status = "ok"
				case os.IsNotExist(statErr):
					r.Status = "missing"
				default:
					// Permission or I/O errors are not evidence of deletion.
					r.Status = "unreadable"
					r.StatError = statErr.Error()
				}
				out = append(out, r)
			}
			return emitLrcat(cmd, flags, out, func(w io.Writer) {
				if len(out) == 0 {
					fmt.Fprintf(w, "no photo matching %q\n", args[0])
					return
				}
				for _, r := range out {
					mark := "✓"
					switch {
					case r.StatError != "":
						mark = "UNREADABLE (" + r.StatError + ")"
					case !r.Exists:
						mark = "MISSING"
					}
					fmt.Fprintf(w, "%s  %s\n", mark, r.Path)
				}
			})
		},
	}
	return cmd
}
