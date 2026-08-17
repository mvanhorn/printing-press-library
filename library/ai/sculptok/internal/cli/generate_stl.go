// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored (was a generator stub). Transcendence command:
// local image -> upload -> submit image-to-STL -> poll -> persist -> download.
//
// pp:data-source live

package cli

import (
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNovelGenerateStlCmd(flags *rootFlags) *cobra.Command {
	var widthMm, minThickness, maxThickness, scaleImage float64
	var invert, restoreFirst, noWait bool
	var batchDir, outDir, db string
	var pollInterval time.Duration

	cmd := &cobra.Command{
		Use:   "stl [image]",
		Short: "Convert a local image straight to a printable STL",
		Long: strings.Trim(`
Upload a local image (or pass a remote URL), submit an image-to-STL job, poll it
to completion, record the job locally, and print the STL URL. Cost: 3 credits.

Use this command for a printable mesh (lithophanes, relief plaques). For a depth
map use 'generate depthmap'; for a full 3D model use 'generate threed'.`, "\n"),
		Example: strings.Trim(`
  sculptok-pp-cli generate stl logo.png
  sculptok-pp-cli generate stl photo.jpg --width-mm 120 --max-thickness 5 --invert
  sculptok-pp-cli generate stl --batch ./photos --out ./stl`, "\n"),
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			// recorded mirrors the flags actually set into the persisted job
			// params (a string map), so STL jobs are searchable offline by their
			// parameters like depthmap/threed — not stored with params={}.
			recorded := map[string]string{}
			if cmd.Flags().Changed("width-mm") {
				body["width_mm"] = widthMm
				recorded["width_mm"] = strconv.FormatFloat(widthMm, 'f', -1, 64)
			}
			if cmd.Flags().Changed("min-thickness") {
				body["min_thickness"] = minThickness
				recorded["min_thickness"] = strconv.FormatFloat(minThickness, 'f', -1, 64)
			}
			if cmd.Flags().Changed("max-thickness") {
				body["max_thickness"] = maxThickness
				recorded["max_thickness"] = strconv.FormatFloat(maxThickness, 'f', -1, 64)
			}
			if cmd.Flags().Changed("scale-image") {
				body["scale_image"] = scaleImage
				recorded["scale_image"] = strconv.FormatFloat(scaleImage, 'f', -1, 64)
			}
			if cmd.Flags().Changed("invert") {
				body["invert"] = invert
				recorded["invert"] = strconv.FormatBool(invert)
			}
			return executeGenerate(cmd, flags, args, genCmdConfig{
				kind:         "stl",
				restoreFirst: restoreFirst,
				noWait:       noWait,
				batchDir:     batchDir,
				outDir:       outDir,
				db:           db,
				pollInterval: pollInterval,
				body:         body,
				recorded:     recorded,
			})
		},
	}
	cmd.Flags().Float64Var(&widthMm, "width-mm", 120, "Output model width in mm (40-240)")
	cmd.Flags().Float64Var(&minThickness, "min-thickness", 1.6, "Minimum thickness in mm for the brightest area (0.4-8)")
	cmd.Flags().Float64Var(&maxThickness, "max-thickness", 5.0, "Maximum thickness in mm for the darkest area (0.4-25)")
	cmd.Flags().Float64Var(&scaleImage, "scale-image", 50, "Image scale percent (0-100)")
	cmd.Flags().BoolVar(&invert, "invert", false, "Invert grayscale (black shallow, white deep)")
	cmd.Flags().BoolVar(&restoreFirst, "restore-first", false, "Run background removal + HD restore (2 credits) before the draw")
	cmd.Flags().BoolVar(&noWait, "no-wait", false, "Submit only; do not poll to completion")
	cmd.Flags().StringVar(&batchDir, "batch", "", "Generate for every image in this directory")
	cmd.Flags().StringVar(&outDir, "out", "", "Download result STL files into this directory")
	cmd.Flags().StringVar(&db, "db", "", "Local store path (default ~/.config/sculptok-pp-cli/sculptok.db)")
	cmd.Flags().DurationVar(&pollInterval, "poll-interval", 3*time.Second, "Status poll interval")
	return cmd
}
