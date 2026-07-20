// Copyright 2026 Micah Baldwin and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: criteria photo search over the local .lrcat catalog.
// Replaces the generated HTTP-backed command — this CLI is local-sqlite.
package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/lightroom-classic/internal/lrcat"
)

func newPhotosPromotedCmd(flags *rootFlags) *cobra.Command {
	var f lrcat.FindFilters
	var flagFormat string

	cmd := &cobra.Command{
		Use:     "photos",
		Aliases: []string{"find"},
		Short:   "Search photos by date, rating, pick, label, keyword, collection, camera, lens, and EXIF criteria",
		Long: "Criteria search over the catalog. Returns matching photo rows with resolved file paths and\n" +
			"human-readable EXIF (f-stop, shutter fraction). Use 'pick-of-day' when you need exactly one image per day.",
		Example: strings.Trim(`
  lightroom-classic-pp-cli photos --rating ">=4" --since 2026-06-01 --json
  lightroom-classic-pp-cli photos --date 2026-07-12 --json --select id,capture_time,path,camera
  lightroom-classic-pp-cli photos --picked --collection "100 Faces" --json
  lightroom-classic-pp-cli photos --camera leica --iso ">=1600" --limit 20`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--date=2026-07-12"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would search photos in the local catalog")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cat, err := openCatalog(ctx, flags)
			if err != nil {
				return err
			}
			defer cat.Close()
			f.Format = flagFormat
			if f.Limit <= 0 {
				f.Limit = 100 // keep the footer consistent with FindPhotos' default
			}
			photos, err := cat.FindPhotos(ctx, f)
			if err != nil {
				if strings.Contains(err.Error(), "invalid comparison") {
					_ = cmd.Usage()
					return usageErr(err)
				}
				return err
			}
			return emitLrcat(cmd, flags, photos, func(w io.Writer) {
				for _, p := range photos {
					photoLine(w, p)
				}
				fmt.Fprintf(w, "%d photos (limit %d)\n", len(photos), f.Limit)
			})
		},
	}
	cmd.Flags().StringVar(&f.Since, "since", "", "Earliest capture date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&f.Until, "until", "", "Latest capture date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&f.Date, "date", "", "Exact capture day (YYYY-MM-DD)")
	cmd.Flags().StringVar(&f.Rating, "rating", "", "Rating filter: '>=4', '5', '>0', or 'unrated'")
	cmd.Flags().BoolVar(&f.Picked, "picked", false, "Only flagged picks")
	cmd.Flags().BoolVar(&f.Rejected, "rejected", false, "Only rejected photos")
	cmd.Flags().StringVar(&f.Label, "label", "", "Color label, e.g. Red")
	cmd.Flags().StringVar(&f.Keyword, "keyword", "", "Keyword name (exact, case-insensitive)")
	cmd.Flags().StringVar(&f.Collection, "collection", "", "Collection name (substring match)")
	cmd.Flags().StringVar(&f.Camera, "camera", "", "Camera model substring, e.g. leica")
	cmd.Flags().StringVar(&f.Lens, "lens", "", "Lens substring")
	cmd.Flags().StringVar(&f.ISO, "iso", "", "ISO filter, e.g. '>=1600'")
	cmd.Flags().StringVar(&flagFormat, "format", "", "File format, e.g. RAW, JPG, DNG")
	cmd.Flags().IntVar(&f.Limit, "limit", 100, "Max photos returned")
	return cmd
}
