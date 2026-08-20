// Copyright 2026 Micah Baldwin and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored helpers shared by all catalog-backed commands.
package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/lightroom-classic/internal/lrcat"
)

// openCatalog resolves and opens the Lightroom catalog read-only, honoring
// the --catalog persistent flag and LIGHTROOM_CLASSIC_CATALOG.
func openCatalog(ctx context.Context, flags *rootFlags) (*lrcat.Catalog, error) {
	path, err := lrcat.Resolve(flags.catalog)
	if err != nil {
		return nil, err
	}
	return lrcat.Open(ctx, path)
}

// emitLrcat writes v as JSON (honoring --select/--compact/--csv/--quiet) when
// machine output is wanted, otherwise calls human to pretty-print.
func emitLrcat(cmd *cobra.Command, flags *rootFlags, v any, human func(w io.Writer)) error {
	w := cmd.OutOrStdout()
	if flags.asJSON || flags.csv || flags.quiet || flags.selectFields != "" || (!isTerminal(w) && !humanFriendly) {
		return printJSONFiltered(w, v, flags)
	}
	human(w)
	return nil
}

// ratingStr renders an optional rating for human tables.
func ratingStr(r *float64) string {
	if r == nil {
		return "-"
	}
	return fmt.Sprintf("%.0f★", *r)
}

// photoLine is the standard one-line human rendering of a catalog photo.
func photoLine(w io.Writer, p lrcat.Photo) {
	pick := " "
	switch p.Pick {
	case 1:
		pick = "⚑"
	case -1:
		pick = "✗"
	}
	exif := ""
	if p.Camera != "" {
		exif = "  " + p.Camera
	}
	if p.Aperture != "" {
		exif += "  " + p.Aperture
	}
	if p.Shutter != "" {
		exif += "  " + p.Shutter
	}
	if p.ISO != nil {
		exif += fmt.Sprintf("  ISO %.0f", *p.ISO)
	}
	fmt.Fprintf(w, "%s %-4s %-20s %s%s\n    %s\n", pick, ratingStr(p.Rating), p.CaptureTime, p.FileName, exif, p.Path)
}
