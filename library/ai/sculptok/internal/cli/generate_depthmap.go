// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored (was a generator stub). Headline transcendence command:
// local image -> upload -> submit depth-map draw -> poll -> persist -> download.
//
// pp:data-source live

package cli

import (
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNovelGenerateDepthmapCmd(flags *rootFlags) *cobra.Command {
	var style, hdFix, optimalSize, extInfo, version, drawHD string
	var restoreFirst, noWait bool
	var batchDir, outDir, db string
	var pollInterval time.Duration

	cmd := &cobra.Command{
		Use:   "depthmap [image]",
		Short: "Turn a local image into SculptOK depth-map candidates in one command",
		Long: strings.Trim(`
Upload a local image (or pass a remote URL), submit a depth-map draw, poll it to
completion, record the job locally, and print the result URLs. Cost: 10 credits
(style=pro 2k 15; style=pro + draw-hd=4k 30).

Use this command for depth maps / 2.5D reliefs. For a printable mesh use
'generate stl'; for a full 3D model use 'generate threed'.`, "\n"),
		Example: strings.Trim(`
  sculptok-pp-cli generate depthmap photo.jpg
  sculptok-pp-cli generate depthmap portrait.jpg --style pro --draw-hd 4k
  sculptok-pp-cli generate depthmap logo.png --restore-first --out ./out
  sculptok-pp-cli generate depthmap --batch ./photos --style pro`, "\n"),
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			if style != "" {
				body["style"] = style
			}
			if hdFix != "" {
				body["hd_fix"] = hdFix
			}
			if optimalSize != "" {
				body["optimal_size"] = optimalSize
			}
			if extInfo != "" {
				body["extInfo"] = extInfo
			}
			if version != "" {
				body["version"] = version
			}
			if drawHD != "" {
				body["draw_hd"] = drawHD
			}
			return executeGenerate(cmd, flags, args, genCmdConfig{
				kind:         "depthmap",
				restoreFirst: restoreFirst,
				noWait:       noWait,
				batchDir:     batchDir,
				outDir:       outDir,
				db:           db,
				pollInterval: pollInterval,
				body:         body,
				recorded:     map[string]string{"style": style, "draw_hd": drawHD, "ext_info": extInfo, "hd_fix": hdFix},
				style:        style,
				drawHD:       drawHD,
			})
		},
	}
	cmd.Flags().StringVar(&style, "style", "", "Depth-map style: normal|portrait|sketch|pro (default normal)")
	cmd.Flags().StringVar(&hdFix, "hd-fix", "", "AI optimization: auto|manual (default manual)")
	cmd.Flags().StringVar(&optimalSize, "optimal-size", "", "Optimal output size: true|false (default true)")
	cmd.Flags().StringVar(&extInfo, "ext-info", "", "Bit depth: 8bit|16bit|exr (exr requires --style pro); default off")
	cmd.Flags().StringVar(&version, "version", "", "Model version (pro only): 1.0|1.5")
	cmd.Flags().StringVar(&drawHD, "draw-hd", "", "Resolution (pro only): 2k|4k (4k costs 30 credits)")
	cmd.Flags().BoolVar(&restoreFirst, "restore-first", false, "Run background removal + HD restore (2 credits) before the draw")
	cmd.Flags().BoolVar(&noWait, "no-wait", false, "Submit only; do not poll to completion")
	cmd.Flags().StringVar(&batchDir, "batch", "", "Generate for every image in this directory")
	cmd.Flags().StringVar(&outDir, "out", "", "Download result images into this directory")
	cmd.Flags().StringVar(&db, "db", "", "Local store path (default ~/.config/sculptok-pp-cli/sculptok.db)")
	cmd.Flags().DurationVar(&pollInterval, "poll-interval", 3*time.Second, "Status poll interval")
	return cmd
}
