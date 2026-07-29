// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored (was a generator stub). Transcendence command:
// local image -> upload -> submit 3D draw -> poll -> persist -> download.
//
// pp:data-source live

package cli

import (
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNovelGenerateThreedCmd(flags *rootFlags) *cobra.Command {
	var hdFix string
	var restoreFirst, noWait bool
	var batchDir, outDir, db string
	var pollInterval time.Duration

	cmd := &cobra.Command{
		Use:   "threed [image]",
		Short: "Generate a 3D model from a local image",
		Long: strings.Trim(`
Upload a local image (or pass a remote URL), submit a 3D draw at basic, standard,
or high precision, poll it to completion, record the job locally, and print the
result URLs. Cost: 10 credits.

Use this command for a full 3D model. For a depth map use 'generate depthmap';
for a printable mesh use 'generate stl'.`, "\n"),
		Example: strings.Trim(`
  sculptok-pp-cli generate threed bust.jpg
  sculptok-pp-cli generate threed bust.jpg --hd-fix high
  sculptok-pp-cli generate threed --batch ./photos`, "\n"),
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			if hdFix != "" {
				body["hd_fix"] = hdFix
			}
			return executeGenerate(cmd, flags, args, genCmdConfig{
				kind:         "threed",
				restoreFirst: restoreFirst,
				noWait:       noWait,
				batchDir:     batchDir,
				outDir:       outDir,
				db:           db,
				pollInterval: pollInterval,
				body:         body,
				recorded:     map[string]string{"hd_fix": hdFix},
			})
		},
	}
	cmd.Flags().StringVar(&hdFix, "hd-fix", "", "Precision: basic|standard|high (default basic)")
	cmd.Flags().BoolVar(&restoreFirst, "restore-first", false, "Run background removal + HD restore (2 credits) before the draw")
	cmd.Flags().BoolVar(&noWait, "no-wait", false, "Submit only; do not poll to completion")
	cmd.Flags().StringVar(&batchDir, "batch", "", "Generate for every image in this directory")
	cmd.Flags().StringVar(&outDir, "out", "", "Download result files into this directory")
	cmd.Flags().StringVar(&db, "db", "", "Local store path (default ~/.config/sculptok-pp-cli/sculptok.db)")
	cmd.Flags().DurationVar(&pollInterval, "poll-interval", 3*time.Second, "Status poll interval")
	return cmd
}
